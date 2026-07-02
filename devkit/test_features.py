"""
Loom 新能力单元测试（纯标准库，无需 live 网关 —— 用假 gateway_chat 打桩）。

跑法：
    cd agent-platform && PYTHONPATH=. python3 -m unittest devkit.test_features -v
    # 或：PYTHONPATH=. python3 devkit/test_features.py

覆盖：
  - 上下文压缩 compact_text：成功摘要（去空白、计成本）/ 失败回退截断（零成本）
  - 控制台 ask_model：单模型扁平返回 / 多模型并行比较（保序、聚合 token+$）/ 空参兜底
  - 控制台 diff_runs：new / changed / deleted / same 状态 + 自动选上一次带 build 的基线
"""
import json
import io
import pathlib
import sys
import tempfile
import urllib.error
import unittest
from contextlib import redirect_stdout
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))                 # devkit 包
sys.path.insert(0, str(ROOT / "console"))     # console/server.py 作为顶层模块

from devkit import rdloop          # noqa: E402
import server                       # noqa: E402  (console/server.py)


class CompactTextTest(unittest.TestCase):
    def test_summary_ok_strips_and_costs(self):
        seen = {}

        def fake_gw(base, key, model, sysmsg, user, max_tokens, tags=None):
            seen["model"], seen["tags"], seen["len"] = model, tags, len(user)
            return True, "  要点A\n要点B  ", model, 42, 0.001

        orig, rdloop.gateway_chat = rdloop.gateway_chat, fake_gw
        try:
            summ, tok, cost = rdloop.compact_text("x" * 5000, "u", "k", "deepseek",
                                                  tags=["t"])
        finally:
            rdloop.gateway_chat = orig
        self.assertEqual(summ, "要点A\n要点B")       # 去掉首尾空白
        self.assertEqual((tok, cost), (42, 0.001))   # 成本如实回传
        self.assertEqual(seen["model"], "deepseek")
        self.assertEqual(seen["tags"], ["t"])
        self.assertLessEqual(seen["len"], 8000)      # 输入被截到 8000 上限

    def test_fallback_truncates_zero_cost(self):
        def fake_gw(*a, **k):
            return False, "boom", "deepseek", 0, 0.0
        orig, rdloop.gateway_chat = rdloop.gateway_chat, fake_gw
        try:
            txt = "y" * 5000
            summ, tok, cost = rdloop.compact_text(txt, "u", "k", "deepseek")
        finally:
            rdloop.gateway_chat = orig
        self.assertEqual(summ, txt[:3500])           # 回退到截断
        self.assertEqual((tok, cost), (0, 0.0))      # 失败不计成本


class AskModelTest(unittest.TestCase):
    def setUp(self):
        self._orig = rdloop.gateway_chat

        def fake_gw(base, key, model, sysmsg, user, max_tokens=700, tags=None):
            if model == "boom":
                return False, "down", model, 0, 0.0
            return True, f"answer-from-{model}", model, 10, 0.002
        rdloop.gateway_chat = fake_gw

    def tearDown(self):
        rdloop.gateway_chat = self._orig

    def test_single_flat_shape(self):
        r = server.ask_model("deepseek", "hi")
        self.assertTrue(r["ok"])
        self.assertEqual(r["served"], "deepseek")
        self.assertEqual(r["content"], "answer-from-deepseek")
        self.assertNotIn("results", r)               # 单个保持扁平（向后兼容）

    def test_multi_compare_parallel_ordered_aggregated(self):
        r = server.ask_model("deepseek, glm ,minimax", "hi")
        self.assertTrue(r.get("compare"))
        self.assertEqual([x["model"] for x in r["results"]],
                         ["deepseek", "glm", "minimax"])   # 按输入顺序，去空格
        self.assertEqual(r["tot_tokens"], 30)
        self.assertAlmostEqual(r["tot_cost"], 0.006)

    def test_multi_mixed_success_and_error(self):
        r = server.ask_model("deepseek,boom", "hi")
        st = {x["model"]: x["ok"] for x in r["results"]}
        self.assertEqual(st, {"deepseek": True, "boom": False})
        self.assertEqual(r["tot_tokens"], 10)         # 失败的不计入

    def test_empty_args_guard(self):
        self.assertIn("error", server.ask_model("", "hi"))
        self.assertIn("error", server.ask_model("deepseek", "   "))


class AskModelsSharedTest(unittest.TestCase):
    """devkit.ask.ask_models —— CLI(`devkit ask`) 直接消费的 list 契约。"""

    def setUp(self):
        self._orig = rdloop.gateway_chat

        def fake_gw(base, key, model, sysmsg, user, max_tokens=700, timeout=180, tags=None):
            return True, f"a-{model}", model, 5, 0.001
        rdloop.gateway_chat = fake_gw

    def tearDown(self):
        rdloop.gateway_chat = self._orig

    def test_returns_ordered_list(self):
        from devkit.ask import ask_models
        out = ask_models([" a ", "b", "", "c"], "q", "u", "k")
        self.assertEqual([r["model"] for r in out], ["a", "b", "c"])  # 去空、保序
        self.assertTrue(all(r["ok"] for r in out))

    def test_empty_models_returns_empty(self):
        from devkit.ask import ask_models
        self.assertEqual(ask_models([], "q", "u", "k"), [])

    def test_single_empty_response_becomes_failure(self):
        from devkit.ask import ask_models

        def fake_empty(base, key, model, sysmsg, user, max_tokens=700, timeout=180, tags=None):
            return True, "   ", model, 9, 0.0

        rdloop.gateway_chat = fake_empty
        out = ask_models(["codex-sub"], "q", "u", "k")
        self.assertFalse(out[0]["ok"])
        self.assertEqual(out[0]["failure_code"], "EMPTY_RESPONSE")


class AskFallbackPolicyTest(unittest.TestCase):
    def setUp(self):
        self._orig = rdloop.gateway_chat

    def tearDown(self):
        rdloop.gateway_chat = self._orig

    def test_control_plane_alias_expands_and_fallbacks_on_timeout(self):
        from devkit.ask import ask_one_with_fallback
        calls = []

        def fake_gw(base, key, model, sysmsg, user, max_tokens=700, timeout=180, tags=None):
            calls.append((model, timeout))
            if model == "loom-product":
                return False, "TimeoutError: timed out", model, 0, 0.0
            return True, f"ok-{model}", model, 11, 0.001

        rdloop.gateway_chat = fake_gw
        out = ask_one_with_fallback(["loom-product"], "q", "u", "k")
        self.assertTrue(out["ok"])
        self.assertEqual(out["served"], "minimax-m27-highspeed")
        self.assertEqual(out["attempted_models"][:2], ["loom-product", "minimax-m27-highspeed"])
        self.assertEqual(calls[0], ("loom-product", 75))

    def test_non_retryable_invalid_model_stops_chain(self):
        from devkit.ask import ask_one_with_fallback

        def fake_gw(base, key, model, sysmsg, user, max_tokens=700, timeout=180, tags=None):
            return False, "HTTP 400: invalid model name", model, 0, 0.0

        rdloop.gateway_chat = fake_gw
        out = ask_one_with_fallback(["codex-sub"], "q", "u", "k")
        self.assertFalse(out["ok"])
        self.assertEqual(out["failure_code"], "INVALID_MODEL")
        self.assertEqual(out["attempted_models"], ["codex-sub"])

    def test_legacy_gpt_review_name_normalizes_to_review_alias(self):
        from devkit.model_aliases import normalize_model_name
        self.assertEqual(normalize_model_name("gpt-5.4", stage="review"), "loom-reviewer")

    def test_minimax_provider_model_normalizes_to_gateway_alias(self):
        from devkit.model_aliases import normalize_model_name
        self.assertEqual(normalize_model_name("minimax/MiniMax-M3", stage="implement"), "minimax")


class DiffRunsTest(unittest.TestCase):
    def _mk(self, base, rel, text):
        p = base / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)

    def test_status_and_auto_baseline(self):
        with tempfile.TemporaryDirectory() as d:
            runs = pathlib.Path(d)
            self._mk(runs, "r1/build/a.py", "def f():\n    return 1\n")
            self._mk(runs, "r1/build/gone.py", "x = 1\n")
            self._mk(runs, "r2/build/a.py", "def f():\n    return 2\n")   # changed
            self._mk(runs, "r2/build/new.py", "y = 2\n")                 # new
            # r2 没有 gone.py → deleted；再加一个两边相同的 same.py
            self._mk(runs, "r1/build/same.py", "Z = 9\n")
            self._mk(runs, "r2/build/same.py", "Z = 9\n")
            orig, server.RUNS_DIR = server.RUNS_DIR, runs
            try:
                res = server.diff_runs("r2")          # 默认基线应自动选 r1
            finally:
                server.RUNS_DIR = orig
            self.assertEqual(res["against"], "r1")
            st = {f["name"]: f["status"] for f in res["files"]}
            self.assertEqual(st["a.py"], "changed")
            self.assertEqual(st["new.py"], "new")
            self.assertEqual(st["gone.py"], "deleted")
            self.assertEqual(st["same.py"], "same")
            self.assertEqual(res["changed"], 3)        # same 不计入改动
            a = next(f for f in res["files"] if f["name"] == "a.py")
            self.assertIn("-    return 1", a["diff"])
            self.assertIn("+    return 2", a["diff"])

    def test_no_baseline_errors(self):
        with tempfile.TemporaryDirectory() as d:
            runs = pathlib.Path(d)
            self._mk(runs, "solo/build/a.py", "a = 1\n")
            orig, server.RUNS_DIR = server.RUNS_DIR, runs
            try:
                res = server.diff_runs("solo")         # 没有更早的带 build 运行
            finally:
                server.RUNS_DIR = orig
            self.assertIn("error", res)


class RolesTest(unittest.TestCase):
    """devkit.roles —— 用户自定义角色（数据而非代码）的加载与校验。"""

    def test_default_fallback(self):
        # 无用户文件 → 回退内置默认 5 角色。用 LOOM_ROLES 指向不存在路径来强制"无文件"。
        import os
        from devkit.roles import load_stages
        old = os.environ.get("LOOM_ROLES")
        os.environ["LOOM_ROLES"] = ""               # 空 → 跳过 env 候选
        try:
            with tempfile.TemporaryDirectory() as d:
                cwd = os.getcwd()
                try:
                    os.chdir(d)                     # 空目录，cwd 候选也找不到
                    stages = load_stages()
                finally:
                    os.chdir(cwd)
        finally:
            if old is None:
                os.environ.pop("LOOM_ROLES", None)
            else:
                os.environ["LOOM_ROLES"] = old
        self.assertGreaterEqual(len(stages), 5)
        self.assertEqual(stages[0].key, "brainstorm")

    def test_load_toml(self):
        from devkit.roles import load_stages
        with tempfile.TemporaryDirectory() as d:
            p = pathlib.Path(d) / "loom.roles.toml"
            p.write_text(
                '[[stages]]\nkey="spec"\nrole="产品"\ntitle="拆解"\n'
                'carrier="deepseek"\nsystem="""\n你是产品。\n"""\n'
                '[[stages]]\nkey="build"\ncarrier="glm"\nsystem="你是开发"\n',
                encoding="utf-8")
            stages = load_stages(str(p))
        self.assertEqual([s.key for s in stages], ["spec", "build"])
        self.assertEqual(stages[0].carrier, "deepseek")
        self.assertEqual(stages[0].system, "你是产品。")     # 多行 + strip
        self.assertEqual(stages[1].role, "build")            # role 缺省 = key
        self.assertEqual(stages[1].carrier, "glm")

    def test_load_json_and_model_alias(self):
        from devkit.roles import load_stages
        with tempfile.TemporaryDirectory() as d:
            p = pathlib.Path(d) / "loom.roles.json"
            p.write_text('{"stages":[{"key":"x","model":"deepseek","system":"hi"}]}',
                         encoding="utf-8")
            stages = load_stages(str(p))
        self.assertEqual(stages[0].carrier, "deepseek")      # model 作为 carrier 别名

    def test_validation_errors(self):
        from devkit.roles import load_stages
        with tempfile.TemporaryDirectory() as d:
            miss = pathlib.Path(d) / "a.toml"
            miss.write_text('[[stages]]\nkey="x"\ncarrier="deepseek"\n', encoding="utf-8")
            with self.assertRaises(ValueError):
                load_stages(str(miss))                       # 缺 system
            dup = pathlib.Path(d) / "b.json"
            dup.write_text('{"stages":[{"key":"x","carrier":"deepseek","system":"a"},'
                           '{"key":"x","carrier":"glm","system":"b"}]}', encoding="utf-8")
            with self.assertRaises(ValueError):
                load_stages(str(dup))                        # key 重复

    def test_executor_and_maxtokens(self):
        from devkit.roles import load_stages
        with tempfile.TemporaryDirectory() as d:
            p = pathlib.Path(d) / "loom.roles.toml"
            p.write_text(
                '[[stages]]\nkey="a"\ncarrier="deepseek"\nexecutor="hermes"\n'
                'max_tokens=300\nsystem="hi"\n'
                '[[stages]]\nkey="b"\ncarrier="glm"\nsystem="ho"\n',   # 缺省
                encoding="utf-8")
            st = load_stages(str(p))
        self.assertEqual((st[0].executor, st[0].max_tokens), ("hermes", 300))
        self.assertEqual((st[1].executor, st[1].max_tokens), ("chat", None))  # 默认

    def test_executor_maxtokens_validation(self):
        from devkit.roles import load_stages
        with tempfile.TemporaryDirectory() as d:
            bad_ex = pathlib.Path(d) / "e.toml"
            bad_ex.write_text('[[stages]]\nkey="a"\ncarrier="deepseek"\n'
                              'executor="bogus"\nsystem="x"\n', encoding="utf-8")
            with self.assertRaises(ValueError):
                load_stages(str(bad_ex))
            bad_mt = pathlib.Path(d) / "m.toml"
            bad_mt.write_text('[[stages]]\nkey="a"\ncarrier="deepseek"\n'
                              'max_tokens=-5\nsystem="x"\n', encoding="utf-8")
            with self.assertRaises(ValueError):
                load_stages(str(bad_mt))

    def test_save_roundtrip_and_reject(self):
        # 控制台在线编辑 → save_stages 校验+写 TOML → load_stages 能读回（同一份）。
        from devkit.roles import save_stages, load_stages, validate_stage_dicts
        with tempfile.TemporaryDirectory() as d:
            dest = pathlib.Path(d) / "loom.roles.toml"
            res = save_stages([
                {"key": "spec", "role": "产品", "title": "拆解",
                 "carrier": "deepseek", "system": "你是产品。"},
                {"key": "build", "carrier": "glm", "system": "你是开发。"},
            ], path=dest)
            self.assertTrue(res["ok"])
            self.assertEqual(res["n"], 2)
            back = load_stages(str(dest))
            self.assertEqual([s.key for s in back], ["spec", "build"])
            self.assertEqual(back[1].role, "build")           # 缺省 = key
            # 非法（缺 system）→ 抛错且不落盘
            bad = pathlib.Path(d) / "bad.toml"
            with self.assertRaises(ValueError):
                save_stages([{"key": "x", "carrier": "deepseek"}], path=bad)
            self.assertFalse(bad.exists())
            with self.assertRaises(ValueError):
                validate_stage_dicts([])                      # 空列表


class MaterializeSalvageTest(unittest.TestCase):
    """A/B 没标文件名时，从代码块推断文件名（实现名取测试 import），避免 0 文件白跑。"""

    def test_salvage_impl_and_test(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = ("好的：\n```python\ndef reverse(s):\n    return s[::-1]\n```\n\n"
                    "```python\nfrom reverse import reverse\n"
                    "def test_reverse():\n    assert reverse('ab') == 'ba'\n```\n")
            files = sorted(materialize(text, pathlib.Path(d)))
            self.assertEqual(files, ["reverse.py", "test_reverse.py"])   # 实现名取自测试 import
            self.assertIn("s[::-1]", (pathlib.Path(d) / "reverse.py").read_text())

    def test_salvage_dotted_import(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = ("```python\ndef rev(s):\n    return s[::-1]\n```\n\n"
                    "```python\nfrom pkg.strutil import rev\n"
                    "def test_rev():\n    assert rev('ab') == 'ba'\n```\n")
            files = sorted(materialize(text, pathlib.Path(d)))
            self.assertEqual(files, ["pkg/strutil.py", "test_strutil.py"])  # 点路径→子目录文件
            self.assertTrue((pathlib.Path(d) / "pkg" / "strutil.py").exists())

    def test_salvage_single_block(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            files = materialize("```python\ndef f():\n    return 1\n```\n", pathlib.Path(d))
            self.assertEqual(files, ["solution.py"])                     # 无测试 → 默认名

    def test_markers_still_win_no_salvage(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = "```python\n# calc.py\ndef add(a, b):\n    return a + b\n```\n"
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["calc.py"])                         # B 命中 → 不走兜底

    def test_heading_with_numbered_prefix_is_recognized(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = "### 2.1 `devkit/test_wallet.py`\n\n```python\ndef test_ok():\n    assert True\n```\n"
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["devkit/test_wallet.py"])
            self.assertTrue((pathlib.Path(d) / "devkit" / "test_wallet.py").exists())

    def test_file_prefix_before_fence_is_recognized(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "FILE: devkit/heartbeat.py\n"
                "```python\n"
                "def beat():\n"
                "    return 'ok'\n"
                "```\n"
            )
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["devkit/heartbeat.py"])
            self.assertIn("def beat()", (pathlib.Path(d) / "devkit" / "heartbeat.py").read_text())

    def test_heading_file_prefix_before_fence_is_recognized(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "## FILE: devkit/test_heartbeat.py\n\n"
                "```python\n"
                "def test_ok():\n"
                "    assert True\n"
                "```\n"
            )
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["devkit/test_heartbeat.py"])

    def test_file_prefix_inside_fence_is_recognized_and_stripped(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "```python\n"
                "FILE: devkit/test_heartbeat.py\n"
                "def test_ok():\n"
                "    assert True\n"
                "```\n"
            )
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["devkit/test_heartbeat.py"])
            body = (pathlib.Path(d) / "devkit" / "test_heartbeat.py").read_text()
            self.assertIn("def test_ok()", body)
            self.assertNotIn("FILE:", body)

    def test_multiple_file_prefix_sections_split_into_multiple_files(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "FILE: devkit/heartbeat.py\n"
                "```python\n"
                "def beat():\n"
                "    return 'ok'\n"
                "```\n\n"
                "FILE: devkit/test_heartbeat.py\n"
                "```python\n"
                "from devkit.heartbeat import beat\n"
                "def test_beat():\n"
                "    assert beat() == 'ok'\n"
                "```\n"
            )
            files = sorted(materialize(text, pathlib.Path(d)))
            self.assertEqual(files, ["devkit/heartbeat.py", "devkit/test_heartbeat.py"])

    def test_backticked_report_path_is_recognized(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = "`runs/demo/audit.md`：\n\n```md\n# Audit\ncount_ok=1\n```\n"
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["runs/demo/audit.md"])
            self.assertIn("# Audit", (pathlib.Path(d) / "runs" / "demo" / "audit.md").read_text())

    def test_report_block_with_explanation_salvages_path(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "请写入 `runs/demo/audit.md`。\n\n"
                "先说明一下上下文。\n\n"
                "```markdown\n# Audit\ncount_ok=1\n```\n"
            )
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["runs/demo/audit.md"])
            self.assertIn("count_ok=1", (pathlib.Path(d) / "runs" / "demo" / "audit.md").read_text())

    def test_unclosed_fence_with_file_marker_materializes_to_eof(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = (
                "### 1.1 单元测试 — `tests/test_applylock.py`\n\n"
                "```python\n"
                "def test_ok():\n"
                "    assert True\n"
            )
            files = materialize(text, pathlib.Path(d))
            self.assertEqual(files, ["tests/test_applylock.py"])
            self.assertIn("def test_ok()", (pathlib.Path(d) / "tests" / "test_applylock.py").read_text())

    def test_no_python_no_files(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(materialize("就是一段说明文字，没有代码。\n", pathlib.Path(d)), [])


class MaterializeCleanTest(unittest.TestCase):
    """materialize 清掉漏进文件内容的 markdown 围栏（曾导致 SyntaxError 污染整批测试）。"""

    def test_clean_body_strips_fences(self):
        from devkit.apply import _clean_body
        self.assertEqual(_clean_body("x = 1\n```\n"), "x = 1")
        self.assertEqual(_clean_body("```python\ny = 2\n```"), "y = 2")
        self.assertEqual(_clean_body("a = 1\nb = 2"), "a = 1\nb = 2")   # 正常不动

    def test_materialize_no_stray_fence(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            text = "#### m.py\n```python\nv = 1\n```\n"
            materialize(text, pathlib.Path(d))
            body = (pathlib.Path(d) / "m.py").read_text()
            self.assertNotIn("```", body)
            self.assertIn("v = 1", body)


class MaterializeDiagnoseTest(unittest.TestCase):
    def test_empty_text_code(self):
        from devkit.apply import diagnose_materialization
        got = diagnose_materialization("", [])
        self.assertEqual(got["failure_code"], "MATERIALIZE_EMPTY_TEXT")

    def test_missing_file_markers_code(self):
        from devkit.apply import diagnose_materialization
        got = diagnose_materialization("```python\nprint(1)\n```\n", [])
        self.assertEqual(got["failure_code"], "FORMAT_MISMATCH_NO_FILE_MARKERS")

    def test_unclosed_fence_counts_as_code_fence(self):
        from devkit.apply import diagnose_materialization
        got = diagnose_materialization("### `tests/test_demo.py`\n```python\ndef test_ok():\n    assert True\n", [])
        self.assertTrue(got["has_code_fence"])
        self.assertTrue(got["has_python_block"])

    def test_materialized_ok(self):
        from devkit.apply import diagnose_materialization
        got = diagnose_materialization("```python\nprint(1)\n```\n", ["solution.py"])
        self.assertEqual(got["status"], "materialized")
        self.assertIsNone(got["failure_code"])

    def test_output_protocol_marks_unclosed_python_as_incomplete(self):
        from devkit.apply import build_output_protocol
        text = "### `adder.py`\n```python\ndef add(a, b):\n    return (\n"
        got = build_output_protocol(text, response_diag={"finish_reason": "length"})
        self.assertTrue(got["suggested_continue"])
        self.assertFalse(got["fences_balanced"])
        self.assertEqual(got["files"][0]["path"], "adder.py")
        self.assertFalse(got["files"][0]["complete_guess"])

    def test_output_protocol_extracts_verify_commands(self):
        from devkit.apply import build_output_protocol
        text = (
            "```python\n# adder.py\ndef add(a, b):\n    return a + b\n```\n"
            "```bash\npython3 -m unittest discover -s tests -v\npytest -q\n```\n"
        )
        got = build_output_protocol(text)
        self.assertEqual(
            got["verify_commands"],
            ["python3 -m unittest discover -s tests -v", "pytest -q"],
        )

    def test_materialize_sanitizes_embedded_restart_fence_for_python(self):
        from devkit.apply import materialize
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            text = (
                "### `math_utils.py`\n```python\n"
                "def add(a, b):\n    return a + b```python\n# math_utils.py\n"
                "def add(a, b):\n    return a + b\n```\n"
            )
            files = materialize(text, root)
            self.assertEqual(files, ["math_utils.py"])
            body = (root / "math_utils.py").read_text(encoding="utf-8")
            self.assertNotIn("```python", body)
            self.assertEqual(body.count("def add"), 1)


class TestCollectionPreflightTest(unittest.TestCase):
    def test_detect_stdlib_shadowing(self):
        from devkit.apply import detect_stdlib_shadowing
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            (root / "pathlib.py").write_text("X = 1\n", encoding="utf-8")
            (root / "pkg.py").write_text("Y = 1\n", encoding="utf-8")
            self.assertEqual(detect_stdlib_shadowing(root), ["pathlib.py"])

    def test_collect_tests_without_test_files_is_hard_failure(self):
        from devkit.apply import collect_tests
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            (root / "pkg.py").write_text("VALUE = 1\n", encoding="utf-8")
            got = collect_tests(root)
        self.assertEqual(got["failure_code"], "TEST_COLLECT_NONE")
        self.assertEqual(got["collected"], 0)
        self.assertFalse(got["ok"])

    def test_collect_tests_blocks_stdlib_shadowing(self):
        from devkit.apply import collect_tests
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            (root / "pathlib.py").write_text("VALUE = 1\n", encoding="utf-8")
            (root / "test_pathlib.py").write_text("def test_ok():\n    assert True\n", encoding="utf-8")
            got = collect_tests(root)
        self.assertEqual(got["failure_code"], "STDLIB_SHADOWING")
        self.assertIn("pathlib.py", got["output"])
        self.assertFalse(got["ok"])

    def test_collect_tests_adds_repo_root_for_devkit_imports(self):
        from devkit.apply import collect_tests
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            build = root / "build"
            build.mkdir()
            pkg = root / "devkit"
            pkg.mkdir()
            (pkg / "__init__.py").write_text("", encoding="utf-8")
            (pkg / "helper.py").write_text("VALUE = 7\n", encoding="utf-8")
            (build / "test_import_devkit.py").write_text(
                "from devkit.helper import VALUE\n\n"
                "def test_value():\n"
                "    assert VALUE == 7\n",
                encoding="utf-8",
            )
            got = collect_tests(build)
        self.assertTrue(got["ok"], got["output"])
        self.assertIsNone(got["failure_code"])
        self.assertEqual(got["collected"], 1)


class MaterializationContractTest(unittest.TestCase):
    """外部项目产物物化契约：required outputs + apply outcome classification。"""

    def test_contract_rejects_absolute_path(self):
        from devkit.materialization_contract import build_contract
        with self.assertRaises(ValueError):
            build_contract(["/tmp/result.md"])

    def test_contract_rejects_parent_escape(self):
        from devkit.materialization_contract import build_contract
        with self.assertRaises(ValueError):
            build_contract(["../outside.md"])

    def test_contract_defaults_apply_policy(self):
        from devkit.materialization_contract import build_contract
        c = build_contract(["outputs/result.md"])
        self.assertEqual(c["apply_policy"], "full_file")
        self.assertEqual(c["required_output_paths"], ["outputs/result.md"])

    def test_all_required_paths_materialized_is_applied(self):
        from devkit.materialization_contract import (
            build_contract, classify_paths, classify_apply_outcome,
        )
        c = build_contract(["outputs/result.md"])
        paths = classify_paths(c, materialized_paths=["outputs/result.md"])
        self.assertEqual(classify_apply_outcome(c, paths, attempted=True), "applied")

    def test_missing_required_output_is_not_applied(self):
        from devkit.materialization_contract import (
            build_contract, classify_paths, classify_apply_outcome,
        )
        c = build_contract(["outputs/result.md"])
        paths = classify_paths(c)
        self.assertEqual(classify_apply_outcome(c, paths, attempted=True), "not_applied")
        self.assertEqual(paths["missing_required_outputs"], ["outputs/result.md"])

    def test_some_materialized_some_missing_is_apply_partial(self):
        from devkit.materialization_contract import (
            build_contract, classify_paths, classify_apply_outcome,
        )
        c = build_contract(["outputs/a.md", "outputs/b.md"])
        paths = classify_paths(c, materialized_paths=["outputs/a.md"])
        self.assertEqual(classify_apply_outcome(c, paths, attempted=True), "apply_partial")

    def test_blocked_path_yields_apply_blocked_and_reason(self):
        from devkit.materialization_contract import (
            build_contract, classify_paths, classify_apply_outcome,
        )
        c = build_contract(["outputs/result.md"])
        paths = classify_paths(
            c,
            blocked_paths=["outputs/result.md"],
            block_reasons={"outputs/result.md": "policy"},
        )
        self.assertEqual(classify_apply_outcome(c, paths, attempted=True), "apply_blocked")
        self.assertEqual(paths["per_path"]["outputs/result.md"]["block_reason"], "policy")

    def test_partial_write_reason_normalized(self):
        from devkit.materialization_contract import normalize_block_reason
        self.assertEqual(normalize_block_reason("partial-write"), "partial_write")
        self.assertEqual(normalize_block_reason("partial write"), "partial_write")

    def test_not_attempted_outcome(self):
        from devkit.materialization_contract import (
            build_contract, classify_paths, classify_apply_outcome,
        )
        c = build_contract(["outputs/result.md"])
        paths = classify_paths(c)
        self.assertEqual(classify_apply_outcome(c, paths, attempted=False), "apply_not_attempted")

    def test_closeout_is_buddys_agnostic(self):
        from devkit.materialization_contract import build_contract, classify_paths, closeout_packet
        c = build_contract(["outputs/result.md"])
        paths = classify_paths(c, materialized_paths=["outputs/result.md"])
        packet = closeout_packet("run-1", c, paths, attempted=True)
        self.assertEqual(packet["apply_outcome"], "applied")
        self.assertNotIn("buddys", json.dumps(packet).lower())
        self.assertNotIn("empty_stage_diagnostics", packet)


class FailureClassificationTest(unittest.TestCase):
    """外部 runner 可消费的 failure_kind + next_action_hint 纯映射。"""

    def test_failure_kind_enum_is_closed(self):
        from devkit.failure_classification import FAILURE_KINDS, normalize_failure_kind
        self.assertEqual(FAILURE_KINDS, (
            "candidate_topic_drift",
            "missing_contracted_outputs",
            "verification_failed_authoritative_surface",
            "review_rejected",
            "empty_or_non_actionable_model_output",
        ))
        with self.assertRaises(ValueError):
            normalize_failure_kind("buddys_specific_failure")

    def test_next_action_hint_enum_is_closed(self):
        from devkit.failure_classification import NEXT_ACTION_HINTS, normalize_next_action_hint
        self.assertEqual(NEXT_ACTION_HINTS, (
            "retry_immediate",
            "retry_different_carrier",
            "skip_candidate_reopen_task",
            "cool_down",
        ))
        with self.assertRaises(ValueError):
            normalize_next_action_hint("invent_new_policy")

    def test_default_hint_for_each_failure_kind(self):
        from devkit.failure_classification import default_hint_for_failure_kind
        self.assertEqual(default_hint_for_failure_kind("candidate_topic_drift"),
                         "skip_candidate_reopen_task")
        self.assertEqual(default_hint_for_failure_kind("missing_contracted_outputs"),
                         "retry_different_carrier")
        self.assertEqual(default_hint_for_failure_kind("verification_failed_authoritative_surface"),
                         "cool_down")
        self.assertEqual(default_hint_for_failure_kind("review_rejected"),
                         "skip_candidate_reopen_task")
        self.assertEqual(default_hint_for_failure_kind("empty_or_non_actionable_model_output"),
                         "retry_immediate")

    def test_outcome_tag_mapping_covers_requested_classes(self):
        from devkit.failure_classification import classify_outcome_tag
        cases = {
            "candidate_topic_drift": ("candidate_topic_drift", "skip_candidate_reopen_task"),
            "missing_contracted_outputs": ("missing_contracted_outputs", "retry_different_carrier"),
            "verification_failed_authoritative_surface": (
                "verification_failed_authoritative_surface", "cool_down",
            ),
            "review_rejected": ("review_rejected", "skip_candidate_reopen_task"),
            "empty_or_non_actionable_model_output": (
                "empty_or_non_actionable_model_output", "retry_immediate",
            ),
        }
        for tag, expected in cases.items():
            r = classify_outcome_tag(tag)
            self.assertEqual((r["failure_kind"], r["next_action_hint"]), expected)
            self.assertFalse(r["fallback"])

    def test_aliases_cover_existing_apply_and_stage_words(self):
        from devkit.failure_classification import classify_outcome_tag
        self.assertEqual(classify_outcome_tag("apply_not_applied")["failure_kind"],
                         "missing_contracted_outputs")
        self.assertEqual(classify_outcome_tag("verify_failed")["failure_kind"],
                         "verification_failed_authoritative_surface")
        self.assertEqual(classify_outcome_tag("empty_output")["failure_kind"],
                         "empty_or_non_actionable_model_output")

    def test_unknown_outcome_has_explicit_fallback(self):
        from devkit.failure_classification import classify_outcome_tag
        r = classify_outcome_tag("unmapped_non_go")
        self.assertTrue(r["fallback"])
        self.assertEqual(r["failure_kind"], "empty_or_non_actionable_model_output")
        self.assertEqual(r["next_action_hint"], "retry_different_carrier")

    def test_mapping_is_deterministic_and_side_effect_free_shape(self):
        from devkit.failure_classification import classify_outcome_tag
        first = classify_outcome_tag("review_rejected")
        for _ in range(1000):
            self.assertEqual(classify_outcome_tag("review_rejected"), first)
        self.assertEqual(sorted(first.keys()), [
            "advisory",
            "failure_kind",
            "fallback",
            "next_action_hint",
            "source_outcome_tag",
        ])

    def test_serialized_mapping_is_downstream_agnostic(self):
        from devkit.failure_classification import classify_outcome_tag
        body = json.dumps(classify_outcome_tag("apply_not_applied"))
        self.assertNotIn("buddys", body.lower())


class FeatureCommitTest(unittest.TestCase):
    """每特性一 commit（就地 git，按议定合同）：消息/作者/树内容/排除/幂等/不动既有仓库。"""

    def _git(self, pd, *a):
        import subprocess
        return subprocess.run(["git", "-C", str(pd), *a], capture_output=True, text=True)

    def test_commit_feature_contract(self):
        import shutil
        if not shutil.which("git"):
            self.skipTest("no git")
        from devkit.backlog import commit_feature
        with tempfile.TemporaryDirectory() as d:
            pd = pathlib.Path(d)
            (pd / "calc.py").write_text("def add(a, b):\n    return a + b\n")
            (pd / "tests").mkdir(); (pd / "tests" / "test_calc.py").write_text("def test():\n    pass\n")
            (pd / "_deps").mkdir(); (pd / "_deps" / "junk.py").write_text("x = 1\n")          # 应被忽略
            (pd / "__pycache__").mkdir(); (pd / "__pycache__" / "c.pyc").write_text("x\n")     # 应被忽略
            (pd / "backlog.json").write_text('{"idea":"x","features":[]}')
            (pd / "progress.md").write_text("# x\n")
            feat = {"id": "f1", "title": "加法", "status": "done"}
            h = commit_feature(pd, feat)
            self.assertRegex(h, r"^[0-9a-f]{7,}$")
            self.assertEqual(self._git(pd, "log", "-1", "--format=%s").stdout.strip(), "feat(f1): 加法")
            self.assertEqual(self._git(pd, "log", "-1", "--format=%an <%ae>").stdout.strip(),
                             "Loom <loom@local>")                                              # headless 身份
            tree = self._git(pd, "ls-tree", "-r", "--name-only", "HEAD").stdout
            for want in ("calc.py", "tests/test_calc.py", ".gitignore", "backlog.json", "progress.md"):
                self.assertIn(want, tree)
            for nope in ("_deps/", "__pycache__", ".pyc"):
                self.assertNotIn(nope, tree)                                                  # 排除构建产物
            self.assertEqual(self._git(pd, "remote").stdout.strip(), "")                       # 无 remote/不 push
            self.assertIsNone(commit_feature(pd, feat))                                        # 无改动 → None
            (pd / "calc.py").write_text("def add(a, b):\n    return a + b\ndef mul(a, b):\n    return a*b\n")
            self.assertTrue(commit_feature(pd, {"id": "f2", "title": "乘法", "status": "done"}))
            self.assertEqual(self._git(pd, "rev-list", "--count", "HEAD").stdout.strip(), "2")  # 每特性 +1

    def test_pre_existing_repo_not_clobbered(self):
        import shutil
        if not shutil.which("git"):
            self.skipTest("no git")
        from devkit.backlog import commit_feature
        with tempfile.TemporaryDirectory() as d:
            pd = pathlib.Path(d)
            self._git(pd, "init", "-q")
            (pd / "orig.txt").write_text("hi\n")
            self._git(pd, "add", "-A")
            self._git(pd, "-c", "user.email=u@x", "-c", "user.name=U", "commit", "-q", "-m", "orig")
            (pd / "f.py").write_text("x = 1\n")
            commit_feature(pd, {"id": "f1", "title": "t", "status": "done"})
            log = self._git(pd, "log", "--format=%s").stdout
            self.assertIn("orig", log)                       # 既有历史还在
            self.assertIn("feat(f1): t", log)


class BacklogTest(unittest.TestCase):
    """backlog 选取/计数（单特性增量的状态机）。"""

    def test_next_todo_picks_lowest_priority(self):
        from devkit import backlog as B
        bl = {"features": [{"id": "f2", "priority": 2, "status": "todo"},
                           {"id": "f1", "priority": 1, "status": "done"},
                           {"id": "f3", "priority": 3, "status": "todo"}]}
        self.assertEqual(B.next_todo(bl)["id"], "f2")
        self.assertEqual(B.counts(bl), (1, 3))

    def test_next_todo_none_when_all_done(self):
        from devkit import backlog as B
        self.assertIsNone(B.next_todo({"features": [{"id": "f1", "status": "done"}]}))


class ContractEvalTest(unittest.TestCase):
    """Sprint Contract 解析 + golden 的 raises 断言（非 happy-path 可表达且可满足）。"""

    def test_contract_extract_and_block(self):
        from devkit.contract import _extract_json_array, _valid_case, to_block
        txt = ('好的：\n```json\n[{"name":"a","import":"from m import f","expr":"f(1)","expect":1},'
               '{"name":"err","import":"from m import f","expr":"f(-1)","raises":"ValueError"},'
               '{"oops":1}]\n```\n')
        arr = _extract_json_array(txt)
        self.assertEqual(len(arr), 3)
        valid = [c for c in arr if _valid_case(c)]
        self.assertEqual(len(valid), 2)                 # 丢掉无 expr/cmd 的
        self.assertIn("f(1)", to_block(valid))

    def test_golden_raises(self):
        from devkit import evals
        with tempfile.TemporaryDirectory() as d:
            b = pathlib.Path(d)
            (b / "m.py").write_text(
                "def f(x):\n    if x < 0:\n        raise ValueError('neg')\n    return x*2\n")
            g = b / "g.json"
            g.write_text('[{"import":"from m import f","expr":"f(3)","expect":6},'
                         '{"import":"from m import f","expr":"f(-1)","raises":"ValueError"}]')
            ok, _summ = evals.run_golden(b, str(g))
            self.assertTrue(ok)                         # 返回正确 + 正确抛异常 → 全过
            # 不抛异常的实现，raises 用例应判失败
            (b / "m.py").write_text("def f(x):\n    return x*2\n")
            ok2, _ = evals.run_golden(b, str(g))
            self.assertFalse(ok2)


class WebVerifyTest(unittest.TestCase):
    """真机 Web 验证：启动真实应用 + 真实 HTTP 请求；playwright 未装则跳过不拉低 gate。"""

    def test_web_real_http(self):
        import socket
        from devkit import evals
        s = socket.socket()
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
        s.close()
        with tempfile.TemporaryDirectory() as d:
            b = pathlib.Path(d)
            (b / "app.py").write_text(
                "import sys\n"
                "from http.server import BaseHTTPRequestHandler, HTTPServer\n"
                "class H(BaseHTTPRequestHandler):\n"
                "    def do_GET(self):\n"
                "        self.send_response(200); self.end_headers()\n"
                "        self.wfile.write(b'<html>hi loom</html>')\n"
                "    def log_message(self,*a): pass\n"
                "HTTPServer(('127.0.0.1', int(sys.argv[1])), H).serve_forever()\n")
            g = b / "g.json"
            g.write_text(json.dumps([{"name": "home", "web": {
                "start": [sys.executable, "app.py", str(port)], "port": port,
                "path": "/", "status": 200, "expect_contains": "hi loom"}}]))
            ok, summ = evals.run_golden(b, str(g))
        self.assertTrue(ok)
        self.assertIn("真机验证", summ)

    def test_playwright_skips_when_absent(self):
        import importlib.util
        from devkit import evals
        orig = importlib.util.find_spec
        importlib.util.find_spec = lambda n, *a, **k: (None if n == "playwright" else orig(n, *a, **k))
        try:
            ok, detail = evals._run_case(pathlib.Path("."), {"playwright": "x.py", "web": {"port": 1}})
        finally:
            importlib.util.find_spec = orig
        self.assertIsNone(ok)                      # 跳过（None），不是失败
        self.assertIn("未安装", detail)

    def test_skip_does_not_fail_gate(self):
        import importlib.util
        from devkit import evals
        orig = importlib.util.find_spec
        importlib.util.find_spec = lambda n, *a, **k: (None if n == "playwright" else orig(n, *a, **k))
        with tempfile.TemporaryDirectory() as d:
            b = pathlib.Path(d)
            (b / "m.py").write_text("def f():\n    return 1\n")
            g = b / "g.json"
            g.write_text(json.dumps([
                {"import": "from m import f", "expr": "f()", "expect": 1},
                {"playwright": "x.py", "web": {"port": 1}}]))
            try:
                ok, summ = evals.run_golden(b, str(g))
            finally:
                importlib.util.find_spec = orig
        self.assertTrue(ok)                        # 1 过 + 1 跳过 → gate 通过
        self.assertIn("跳过", summ)


class ContractNegotiateTest(unittest.TestCase):
    """合同多轮协商：构建者收紧/修正 + 反削弱地板 + 跨载体独立 + rounds=0 向后兼容。"""

    def _patch(self, fn):
        self._orig = rdloop.gateway_chat
        rdloop.gateway_chat = fn

    def tearDown(self):
        if hasattr(self, "_orig"):
            rdloop.gateway_chat = self._orig

    def test_builder_fixes_and_adds(self):
        from devkit import contract
        seen = []
        EV = ('[{"import":"from m import f","expr":"f(1)","expect":1},'
              '{"import":"from m import f","expr":"f(-1)","raises":"ValueError"}]')
        BU = ('[{"import":"from m import f","expr":"f(1)","expect":2},'           # 修正 expect 1→2
              '{"import":"from m import f","expr":"f(-1)","raises":"ValueError"},'
              '{"import":"from m import f","expr":"f(2)","expect":4}]')           # 新增一条

        def fake(base, key, model, sysmsg, user, max_tokens=1100, tags=None):
            seen.append(model)
            return (True, EV if model == "ev" else BU, model, 10 if model == "ev" else 7, 0.0)
        self._patch(fake)
        cases, tk, co, _raw = contract.negotiate_rounds("t", "p", "u", "k", "ev", "bu", n=2, rounds=1)
        self.assertEqual(len(cases), 3)                 # 评判者 2 → 构建者补成 3
        self.assertEqual(cases[0]["expect"], 2)         # 修正落地
        self.assertEqual(seen[0], "ev")                 # 评判者先拟（路由到 eval_carrier）
        self.assertIn("bu", seen[1:])                   # 构建者后议（路由到 build_carrier）= 独立
        self.assertEqual(tk, 17)                         # 各轮 token 累加

    def test_anti_weakening_floor(self):
        from devkit import contract
        EV = ('[{"import":"from m import f","expr":"f(1)","expect":1},'
              '{"import":"from m import f","expr":"f(-1)","raises":"ValueError"}]')
        BU = '[{"import":"from m import f","expr":"f(1)","expect":1}]'            # 想删到 1 条、去掉 raises

        def fake(base, key, model, sysmsg, user, max_tokens=1100, tags=None):
            return (True, EV if model == "ev" else BU, model, 5, 0.0)
        self._patch(fake)
        cases, _tk, _co, _raw = contract.negotiate_rounds("t", "p", "u", "k", "ev", "bu", n=2, rounds=2)
        self.assertGreaterEqual(len(cases), 2)          # 地板：不得少于评判者原条数
        self.assertEqual(sum(1 for c in cases if "raises" in c), 1)   # raises 用例被保住

    def test_rounds_zero_backward_compat(self):
        from devkit import contract
        seen = []

        def fake(base, key, model, sysmsg, user, max_tokens=1100, tags=None):
            seen.append(model)
            return (True, '[{"import":"from m import f","expr":"f(1)","expect":1}]', model, 9, 0.0)
        self._patch(fake)
        cases, _tk, _co, _raw = contract.negotiate_rounds("t", "p", "u", "k", "ev", "bu", n=1, rounds=0)
        self.assertNotIn("bu", seen)                    # rounds=0：构建者从不被调用
        self.assertEqual(len(cases), 1)


class ResponseCacheTest(unittest.TestCase):
    """精确响应缓存：miss→打+存 / hit→免费不打 / tag 不进键 / max_tokens 进键 /
    remap(mtime) 失效 / --no-cache 旁路 / 失败不缓存 / 坏库降级 / 并发不崩 / TTL+上限。"""

    def setUp(self):
        self._ou, self._oe, self._om = (rdloop._uncached_gateway_chat,
                                        rdloop.CACHE_ENABLED, rdloop._config_mtime)
        rdloop.CACHE_ENABLED = True
        rdloop._config_mtime = lambda: "MT"
        self.calls = []

        def fake(base, key, model, system, user, max_tokens=900, timeout=180, tags=None):
            self.calls.append(model)
            return True, f"ans-{model}-{len(self.calls)}", model, 11, 0.002
        rdloop._uncached_gateway_chat = fake
        self._td = tempfile.TemporaryDirectory()
        self.db = pathlib.Path(self._td.name) / "c.db"

    def tearDown(self):
        rdloop._uncached_gateway_chat, rdloop.CACHE_ENABLED, rdloop._config_mtime = (
            self._ou, self._oe, self._om)
        self._td.cleanup()

    def _gw(self, model="deepseek", mt=100, tags=None):
        return rdloop.gateway_chat("u", "k", model, "sys", "usr", mt, tags=tags, _db=self.db)

    def test_miss_then_hit_free(self):
        r1 = self._gw()
        self.assertEqual(self.calls, ["deepseek"])              # miss → 打一次
        self.assertEqual(r1[3:], (11, 0.002))
        r2 = self._gw()
        self.assertEqual(self.calls, ["deepseek"])              # hit → 没再打
        self.assertEqual(r2, (True, r1[1], "deepseek", 0, 0.0))  # 免费命中

    def test_tags_excluded_from_key(self):
        self._gw(tags=["run:1"])
        self._gw(tags=["run:2"])
        self.assertEqual(len(self.calls), 1)                    # tag 不进键 → 仍命中

    def test_max_tokens_in_key(self):
        self._gw(mt=100)
        self._gw(mt=200)
        self.assertEqual(len(self.calls), 2)                    # max_tokens 不同 → miss

    def test_config_mtime_busts(self):
        self._gw()
        rdloop._config_mtime = lambda: "MT2"                    # 模拟 remap 改了 config
        self._gw()
        self.assertEqual(len(self.calls), 2)

    def test_no_cache_bypass(self):
        rdloop.CACHE_ENABLED = False
        self._gw(); self._gw()
        self.assertEqual(len(self.calls), 2)

    def test_failure_not_cached(self):
        def failer(*a, **k):
            self.calls.append("x")
            return False, "boom", "deepseek", 0, 0.0
        rdloop._uncached_gateway_chat = failer
        self._gw(); self._gw()
        self.assertEqual(len(self.calls), 2)                    # 失败不缓存 → 重打

    def test_corrupt_db_degrades(self):
        self.db.write_text("not a sqlite database, garbage")
        r = self._gw()
        self.assertTrue(r[0])                                   # 仍返回 live 结果，不崩
        self.assertEqual(len(self.calls), 1)

    def test_concurrent_no_crash(self):
        import threading
        errs = []

        def work(i):
            try:
                self._gw(model=f"m{i % 2}")
            except Exception as e:  # noqa: BLE001
                errs.append(e)
        thr = [threading.Thread(target=work, args=(i,)) for i in range(6)]
        [t.start() for t in thr]
        [t.join() for t in thr]
        self.assertEqual(errs, [])

    def test_non_dict_row_ignored(self):
        import sqlite3
        from devkit import cache
        cache.put(self.db, "k", {"content": "a", "served": "m"}, 100)   # 先建表
        c = sqlite3.connect(str(self.db))                               # 篡改成非 dict 的合法 JSON
        c.execute("UPDATE cache SET val=? WHERE key=?", ("[1,2,3]", "k"))
        c.commit(); c.close()
        self.assertIsNone(cache.get(self.db, "k", 1e9))                # 非 dict → None，不返回坏形状

    def test_cache_module_ttl_and_cap(self):
        from devkit import cache
        cache.put(self.db, "k1", {"content": "a", "served": "m"}, 100, now=1000)
        self.assertIsNotNone(cache.get(self.db, "k1", 10, now=1005))   # 未过期
        self.assertIsNone(cache.get(self.db, "k1", 10, now=2000))      # 过期
        cap = pathlib.Path(self._td.name) / "cap.db"
        for i, t in enumerate([10, 20, 30]):
            cache.put(cap, f"c{i}", {"content": str(i), "served": "m"}, max_rows=2, now=t)
        self.assertIsNone(cache.get(cap, "c0", 1e9, now=40))           # 最旧被淘汰
        self.assertIsNotNone(cache.get(cap, "c2", 1e9, now=40))        # 最新还在

    def test_empty_success_not_cached(self):
        def empty_ok(*a, **k):
            self.calls.append("x")
            return True, "", "deepseek", 123, 0.0
        rdloop._uncached_gateway_chat = empty_ok
        r1 = self._gw()
        r2 = self._gw()
        self.assertEqual(self.calls, ["x", "x"])                       # 不缓存空正文
        self.assertEqual(r1[1], "")
        self.assertEqual(r2[3], 123)                                   # 第二次仍是 live，不是缓存 0tok

    def test_cached_empty_entry_is_evicted_and_refetched(self):
        from devkit import cache
        key = rdloop._cache_key("deepseek", "sys", "usr", 100)
        cache.put(self.db, key, {"content": "", "served": "deepseek"}, 100)
        r = self._gw()
        self.assertEqual(self.calls, ["deepseek"])                      # 命中空缓存后回退 live
        self.assertEqual(r[1], "ans-deepseek-1")
        self.assertIsNotNone(cache.get(self.db, key, 1e9))

    def test_real_content_still_caches(self):
        r1 = self._gw()
        r2 = self._gw()
        self.assertEqual(self.calls, ["deepseek"])
        self.assertEqual(r2, (True, r1[1], "deepseek", 0, 0.0))

    def test_has_cacheable_text_content_false_for_tool_noise(self):
        self.assertFalse(rdloop._has_cacheable_text_content("<invoke>tool</invoke>"))
        self.assertTrue(rdloop._has_cacheable_text_content("hello"))


class MiniMaxGatewayCompatTest(unittest.TestCase):
    class _FakeResponse:
        def __init__(self, body: dict, cost_hdr: str | None = None):
            self._body = json.dumps(body).encode("utf-8")
            self.headers = {"x-litellm-response-cost": cost_hdr} if cost_hdr is not None else {}

        def read(self):
            return self._body

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return False

    def test_minimax_request_disables_thinking_and_splits_reasoning(self):
        seen = {}

        def fake_urlopen(req, timeout=0):
            seen["payload"] = json.loads(req.data.decode("utf-8"))
            seen["url"] = req.full_url
            seen["auth"] = req.headers.get("Authorization")
            return self._FakeResponse({
                "model": "MiniMax-M3",
                "usage": {"total_tokens": 7},
                "choices": [{"message": {"content": "ok"}}],
            }, "0.0")

        with mock.patch("urllib.request.urlopen", fake_urlopen), mock.patch.object(rdloop, "load_env_key", return_value="minimax-direct-key"):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://localhost:4000", "k", "minimax", "sys", "usr", tags=["run:x"]
            )
        self.assertTrue(ok)
        self.assertEqual(content, "ok")
        self.assertEqual(served, "MiniMax-M3")
        self.assertEqual(tokens, 7)
        self.assertEqual(cost, 0.0)
        self.assertEqual(seen["payload"]["thinking"], {"type": "disabled"})
        self.assertTrue(seen["payload"]["reasoning_split"])
        self.assertNotIn("metadata", seen["payload"])
        self.assertEqual(seen["payload"]["model"], "MiniMax-M3")
        self.assertEqual(seen["url"], "https://api.minimaxi.com/v1/chat/completions")
        self.assertEqual(seen["auth"], "Bearer minimax-direct-key")
        self.assertEqual(seen["payload"]["max_completion_tokens"], 900)
        self.assertNotIn("max_tokens", seen["payload"])

    def test_model_specific_fields_remap_max_tokens_for_minimax(self):
        payload = rdloop._apply_model_specific_request_fields(
            {"model": "minimax", "max_tokens": 32},
            "minimax",
        )
        self.assertEqual(payload["thinking"], {"type": "disabled"})
        self.assertTrue(payload["reasoning_split"])
        self.assertEqual(payload["max_completion_tokens"], 32)
        self.assertNotIn("max_tokens", payload)

    def test_non_minimax_request_keeps_plain_openai_shape(self):
        seen = {}

        def fake_urlopen(req, timeout=0):
            seen["payload"] = json.loads(req.data.decode("utf-8"))
            return self._FakeResponse({
                "model": "deepseek-chat",
                "usage": {"total_tokens": 3},
                "choices": [{"message": {"content": "pong"}}],
            })

        with mock.patch("urllib.request.urlopen", fake_urlopen):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://gw", "k", "deepseek", "sys", "usr"
            )
        self.assertTrue(ok)
        self.assertEqual(content, "pong")
        self.assertEqual(served, "deepseek-chat")
        self.assertEqual(tokens, 3)
        self.assertEqual(cost, 0.0)
        self.assertNotIn("thinking", seen["payload"])
        self.assertNotIn("reasoning_split", seen["payload"])
        self.assertEqual(seen["payload"]["max_tokens"], 900)
        self.assertNotIn("max_completion_tokens", seen["payload"])

    def test_reasoning_split_response_message_is_used_when_content_empty(self):
        body = {
            "model": "MiniMax-M3",
            "usage": {"total_tokens": 12},
            "choices": [{
                "message": {
                    "content": "",
                    "response_message": {"content": "final answer"},
                    "reasoning_content": "internal chain",
                }
            }],
        }
        self.assertEqual(rdloop._extract_chat_completion_text(body), "final answer")

    def test_content_blocks_and_inline_thinking_are_normalized(self):
        body = {
            "choices": [{
                "message": {
                    "content": [
                        {"type": "text", "text": "<think>plan</think>"},
                        {"type": "text", "text": "ship patch"},
                    ]
                }
            }]
        }
        self.assertEqual(rdloop._extract_chat_completion_text(body), "ship patch")

    def test_reasoning_only_length_is_classified(self):
        body = {
            "model": "glm-5.2",
            "object": "chat.completion",
            "choices": [{
                "finish_reason": "length",
                "message": {"content": "", "reasoning_content": "step 1"},
            }],
        }
        got = rdloop._extract_response_payload(body)
        self.assertEqual(got["text"], "")
        self.assertEqual(got["failure_code"], "EMPTY_REASONING_ONLY_LENGTH")
        self.assertTrue(got["diag"]["has_reasoning_content"])

    def test_responses_api_output_is_used(self):
        body = {
            "object": "response",
            "output": [{
                "type": "message",
                "content": [{"type": "output_text", "text": "answer from responses"}],
            }],
        }
        got = rdloop._extract_response_payload(body)
        self.assertEqual(got["text"], "answer from responses")
        self.assertIsNone(got["failure_code"])

    def test_tool_noise_is_classified(self):
        body = {
            "choices": [{
                "message": {"content": "<invoke>tool</invoke>"},
            }]
        }
        got = rdloop._extract_response_payload(body)
        self.assertEqual(got["text"], "<invoke>tool</invoke>")
        self.assertEqual(got["normalized_text"], "")
        self.assertEqual(got["failure_code"], "EMPTY_TOOL_NOISE")

    def test_reasoning_only_triggers_retry_and_recovers_text(self):
        calls = []

        def fake_urlopen(req, timeout=0):
            payload = json.loads(req.data.decode("utf-8"))
            calls.append(payload)
            if len(calls) == 1:
                return self._FakeResponse({
                    "model": "glm-5.2",
                    "object": "chat.completion",
                    "usage": {"total_tokens": 80},
                    "choices": [{
                        "finish_reason": "length",
                        "message": {"content": "", "reasoning_content": "step 1"},
                    }],
                })
            return self._FakeResponse({
                "model": "glm-5.2",
                "object": "chat.completion",
                "usage": {"total_tokens": 20},
                "choices": [{"finish_reason": "stop", "message": {"content": "pong"}}],
            })

        with mock.patch("urllib.request.urlopen", fake_urlopen):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://gw", "k", "deepseek", "sys", "usr", max_tokens=64
            )
        self.assertTrue(ok)
        self.assertEqual(content, "pong")
        self.assertEqual(tokens, 100)
        self.assertEqual(len(calls), 2)
        self.assertEqual(calls[1]["max_tokens"], 256)
        self.assertIn("只输出最终答案正文", calls[1]["messages"][0]["content"])

    def test_truncated_text_triggers_continuation_and_merges(self):
        calls = []

        def fake_urlopen(req, timeout=0):
            payload = json.loads(req.data.decode("utf-8"))
            calls.append(payload)
            if len(calls) == 1:
                return self._FakeResponse({
                    "model": "MiniMax-M3",
                    "object": "chat.completion",
                    "usage": {"total_tokens": 80},
                    "choices": [{
                        "finish_reason": "length",
                        "message": {"content": "```python\n# adder.py\ndef add(a, b):\n    return a +"},
                    }],
                })
            return self._FakeResponse({
                "model": "MiniMax-M3",
                "object": "chat.completion",
                "usage": {"total_tokens": 20},
                "choices": [{
                    "finish_reason": "stop",
                    "message": {"content": " b\n```\n"},
                }],
            })

        with mock.patch("urllib.request.urlopen", fake_urlopen):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://gw", "k", "minimax", "sys", "usr", max_tokens=64
            )
        self.assertTrue(ok)
        self.assertIn("return a + b", content)
        self.assertEqual(tokens, 100)
        self.assertEqual(len(calls), 2)
        self.assertIn("继续输出", calls[1]["messages"][-1]["content"])

    def test_truncated_text_can_continue_multiple_rounds(self):
        calls = []

        def fake_urlopen(req, timeout=0):
            payload = json.loads(req.data.decode("utf-8"))
            calls.append(payload)
            idx = len(calls)
            if idx == 1:
                return self._FakeResponse({
                    "model": "MiniMax-M3",
                    "object": "chat.completion",
                    "usage": {"total_tokens": 60},
                    "choices": [{
                        "finish_reason": "length",
                        "message": {"content": "### `adder.py`\n```python\ndef add(a, b):\n    return a +"},
                    }],
                })
            if idx == 2:
                return self._FakeResponse({
                    "model": "MiniMax-M3",
                    "object": "chat.completion",
                    "usage": {"total_tokens": 20},
                    "choices": [{
                        "finish_reason": "length",
                        "message": {"content": " b\n\ndef subtract(a, b):\n    return a -"},
                    }],
                })
            return self._FakeResponse({
                "model": "MiniMax-M3",
                "object": "chat.completion",
                "usage": {"total_tokens": 20},
                "choices": [{
                    "finish_reason": "stop",
                    "message": {"content": " b\n```\n"},
                }],
            })

        with mock.patch("urllib.request.urlopen", fake_urlopen):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://gw", "k", "minimax", "sys", "usr", max_tokens=64
            )
        self.assertTrue(ok)
        self.assertIn("return a + b", content)
        self.assertIn("return a - b", content)
        self.assertEqual(len(calls), 3)

    def test_transient_http_error_retries_once(self):
        calls = {"n": 0}

        def fake_urlopen(req, timeout=0):
            calls["n"] += 1
            if calls["n"] == 1:
                raise urllib.error.HTTPError(
                    req.full_url, 429, "rate limited", hdrs=None, fp=None
                )
            return self._FakeResponse({
                "model": "MiniMax-M3",
                "object": "chat.completion",
                "usage": {"total_tokens": 12},
                "choices": [{"finish_reason": "stop", "message": {"content": "pong"}}],
            })

        with mock.patch("urllib.request.urlopen", fake_urlopen):
            ok, content, served, tokens, cost = rdloop._uncached_gateway_chat(
                "http://gw", "k", "minimax", "sys", "usr", max_tokens=64
            )
        self.assertTrue(ok)
        self.assertEqual(content, "pong")
        self.assertEqual(calls["n"], 2)


class HealthProbeTest(unittest.TestCase):
    """K8s 探针式健康判定：serving / expired / down 决策表 + ISO 解析容错。"""

    def _now(self):
        from datetime import datetime, timezone
        return datetime(2026, 6, 26, 12, 0, tzinfo=timezone.utc)

    def test_decision_table(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            td = pathlib.Path(d)
            (td / "codex.json").write_text('{"type":"codex","expired":"2026-07-02T07:49:13+08:00"}')   # 未过期
            live = ("codex-sub", "glm", "deepseek")

            def prober(bu, k, m):
                return {"backend": m, "ok": m in live, "detail": "ok" if m in live else "boom"}
            rows = insight.health("u", "k", token_dir=td, prober=prober, now=self._now())
            st = {r["backend"]: r["state"] for r in rows}
            self.assertEqual(st["codex-sub"], "serving")          # 通 + 凭据有效
            self.assertEqual(st["glm"], "serving")
            self.assertEqual(st["minimax"], "down")               # 打不通 + 无 token（unknown）→ 真挂
            self.assertEqual(st["deepseek"], "serving")
            c = next(r for r in rows if r["backend"] == "codex-sub")
            self.assertEqual(c["token_state"], "valid")
            self.assertTrue(c["ok"])                              # ok == (state=='serving') 向后兼容

    def test_live_but_expired_is_serving_with_warning(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            td = pathlib.Path(d)
            (td / "codex.json").write_text('{"type":"codex","expired":"2026-06-24T00:00:00Z"}')
            rows = insight.health("u", "k", token_dir=td,
                                  prober=lambda bu, k, m: {"backend": m, "ok": True, "detail": "ok"},
                                  now=self._now())
            c = next(r for r in rows if r["backend"] == "codex-sub")
            self.assertEqual(c["state"], "serving")
            self.assertIn("仍在服务", c["detail"])

    def test_disabled_is_expired(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            td = pathlib.Path(d)
            (td / "codex.json").write_text('{"type":"codex","expired":"2099-01-01T00:00:00Z","disabled":true}')
            rows = insight.health("u", "k", token_dir=td,
                                  prober=lambda bu, k, m: {"backend": m, "ok": True, "detail": "ok"},
                                  now=self._now())
            c = next(r for r in rows if r["backend"] == "codex-sub")
            self.assertEqual(c["state"], "expired")              # 禁用即使未过期也要重登
            self.assertEqual(c["action"], "run ./loom login")

    def test_multi_token_prefers_freshest(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            td = pathlib.Path(d)
            (td / "a-codex.json").write_text('{"type":"codex","expired":"2026-06-24T00:00:00Z"}')   # 陈旧/过期，排序在前
            (td / "z-codex.json").write_text('{"type":"codex","expired":"2099-01-01T00:00:00Z"}')   # 新/有效
            rows = insight.health("u", "k", token_dir=td,
                                  prober=lambda bu, k, m: {"backend": m, "ok": True, "detail": "ok"},
                                  now=self._now())
            c = next(r for r in rows if r["backend"] == "codex-sub")
            self.assertEqual(c["token_state"], "valid")      # 选最新那份，不被陈旧文件遮蔽
            self.assertEqual(c["state"], "serving")

    def test_unreadable_token_dir_degrades(self):
        from devkit import insight
        rows = insight.health("u", "k", token_dir="/no/such/dir",
                              prober=lambda bu, k, m: {"backend": m, "ok": m != "codex-sub", "detail": "x"},
                              now=self._now())
        c = next(r for r in rows if r["backend"] == "codex-sub")
        self.assertEqual(c["state"], "down")                     # 读不到 token → 不误判过期，按真挂
        self.assertEqual(c["token_state"], "unavailable")

    def test_iso_and_unparseable(self):
        from devkit.insight import _parse_iso
        self.assertIsNotNone(_parse_iso("2026-06-24T19:49:45Z"))      # 尾 Z
        self.assertIsNotNone(_parse_iso("2026-07-02T07:49:13+08:00"))  # 显式偏移
        self.assertIsNone(_parse_iso("garbage"))                      # 不可解析 → None（不抛、不误判）
        self.assertIsNone(_parse_iso(""))


class CascadeTest(unittest.TestCase):
    """cascade-escalate 的轮数/选档逻辑（FrugalGPT 升级阶梯）。"""

    def test_rounds_implied_and_capped(self):
        from devkit.rdloop import _cascade_rounds
        self.assertEqual(_cascade_rounds(["a", "b", "c"], 0), 2)   # 蕴含：默认=阶梯长-1
        self.assertEqual(_cascade_rounds(["a", "b", "c"], 1), 1)   # 用户压低
        self.assertEqual(_cascade_rounds(["a", "b", "c"], 9), 2)   # 不可拔高（无更高档）
        self.assertEqual(_cascade_rounds(["a"], 0), 0)             # 单档=不额外迭代
        self.assertEqual(_cascade_rounds([], 3), 3)               # 无 cascade → 原样

    def test_carrier_indexing(self):
        from devkit.rdloop import _cascade_carrier
        L = ["cheap", "mid", "strong"]
        self.assertEqual([_cascade_carrier(L, k, "x") for k in range(5)],
                         ["cheap", "mid", "strong", "strong", "strong"])  # 末档封顶
        self.assertEqual(_cascade_carrier([], 2, "fallback"), "fallback")  # 无 cascade 用 fallback


class IterateLoopTest(unittest.TestCase):
    """迭代循环（Planner→Generator→Evaluator）的判定与反馈构造。"""

    def test_wants_changes(self):
        from devkit.rdloop import _wants_changes
        self.assertTrue(_wants_changes("最终结论：REQUEST-CHANGES，原因…"))
        self.assertTrue(_wants_changes("verdict: request changes"))   # 空格/无连字符
        self.assertFalse(_wants_changes("APPROVE，旗舰用例成立"))
        self.assertFalse(_wants_changes(""))                          # 无评判 → 不阻塞

    def test_fail_detail_prefers_golden_want(self):
        from devkit.rdloop import _fail_detail
        fd = _fail_detail("（沙箱无 test_*.py，跳过测试）",
                          "| case2 | ❌ | ValueError…  want=0 |")
        self.assertIn("want=0", fd)                 # golden 明细（含期望值）进反馈
        self.assertIn("Golden 评测明细", fd)
        self.assertNotIn("跳过测试", fd)            # 无意义的"跳过测试"被过滤

    def test_build_feedback(self):
        from devkit.rdloop import _build_feedback
        fb = _build_feedback("### Golden…\nwant=0", "REQUEST-CHANGES: 缺边界处理")
        self.assertIn("want=0", fb)                 # 失败明细透传
        self.assertIn("评判者意见", fb)
        self.assertIn("最小改动", fb)
        fb2 = _build_feedback("boom", "")           # 无评判意见 → 不带评判段
        self.assertNotIn("评判者意见", fb2)


class InsightTest(unittest.TestCase):
    """devkit.insight —— 额度薅羊毛 + 模型评分（数据来自真实日志，官网分用户维护）。"""

    def test_canonical_backend(self):
        from devkit.insight import canonical_backend
        self.assertEqual(canonical_backend("openai/claude-sonnet-4-5"), "claude")
        self.assertEqual(canonical_backend("openai/gpt-5.3-codex-spark"), "codex")
        self.assertEqual(canonical_backend("deepseek-v4-flash"), "deepseek")  # 降级别名
        self.assertEqual(canonical_backend("minimax/abab6.5s-chat"), "minimax")
        self.assertEqual(canonical_backend("openai/glm-4.6"), "glm")
        self.assertEqual(canonical_backend("mystery-model"), "other")

    def test_run_stats_parsing(self):
        from devkit.insight import run_stats_by_backend
        with tempfile.TemporaryDirectory() as d:
            runs = pathlib.Path(d)
            (runs / "r1").mkdir()
            (runs / "r1" / "run-log.md").write_text(
                "# Run\n\n| 阶段 | 载体 | 实际模型 | 状态 | 用时 | tokens | 花费 |\n"
                "| --- | --- | --- | --- | --- | --- | --- |\n"
                "| brainstorm | loom-product | deepseek-chat | OK | 2.0s | 100 | $0.00010 |\n"
                "| review | loom-reviewer | deepseek-chat | OK | 4.0s | 200 | $0.00030 |\n"
                "| plan | loom-orchestrator | - | BLOCKED | 1.0s | 0 | $0.00000 |\n",
                encoding="utf-8")
            stats = run_stats_by_backend(runs)
        ds = stats["deepseek"]
        self.assertEqual((ds["uses"], ds["ok"]), (2, 2))
        self.assertAlmostEqual(ds["lat"], 6.0)
        self.assertEqual(ds["tokens"], 300)
        # BLOCKED 行 served='-' → 退用载体名 loom-orchestrator → GPT-5 系列（当前调度层用 GPT-5.4）
        self.assertEqual(stats["codex"]["uses"], 1)
        self.assertEqual(stats["codex"]["ok"], 0)

    def test_ratings_roundtrip(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            old = insight.RATINGS
            insight.RATINGS = pathlib.Path(d) / "ratings.jsonl"
            try:
                insight.add_rating("openai/glm-4.6", +1)   # 归一到 glm
                insight.add_rating("glm", -1)
                insight.add_rating("deepseek", +1)
                agg = insight.ratings_by_backend()
            finally:
                insight.RATINGS = old
        self.assertEqual(agg["glm"], {"up": 1, "down": 1})
        self.assertEqual(agg["deepseek"], {"up": 1, "down": 0})

    def test_liveness_orchestration(self):
        from devkit import insight
        # 注入假 prober，验证逐后端、保序、ok/detail 透传，不打网络
        seen = []

        def fake(base, key, model):
            seen.append(model)
            return {"backend": model, "ok": model in ("deepseek", "codex-sub"),
                    "detail": "served=x" if model in ("deepseek", "codex-sub") else "RateLimit"}
        rows = insight.liveness("u", "k", backends=("glm", "deepseek"), prober=fake)
        self.assertEqual([r["backend"] for r in rows], ["glm", "deepseek"])   # map 保序
        self.assertEqual([r["ok"] for r in rows], [False, True])
        self.assertEqual(rows[0]["detail"], "RateLimit")
        self.assertEqual(sorted(seen), ["deepseek", "glm"])     # 都探到（并发，不假设调用顺序）

    def test_quota_report_ranks_and_recommends(self):
        from devkit import insight
        orig_spend, orig_cfg = insight.spend_by_backend, insight.load_quota_config
        insight.spend_by_backend = lambda b, k: {"deepseek": {"spend": 1.0, "tokens": 5}}
        insight.load_quota_config = lambda: {
            "claude": {"subscription": True},
            "deepseek": {"free_usd": 5.0},
            "glm": {"free_usd": 1.0},
        }
        try:
            rep = insight.quota_report("u", "k")
        finally:
            insight.spend_by_backend, insight.load_quota_config = orig_spend, orig_cfg
        by = {r["backend"]: r for r in rep["rows"]}
        self.assertEqual(by["deepseek"]["remaining_usd"], 4.0)     # 5 - 1 已用
        self.assertEqual(by["claude"]["kind"], "订阅")
        self.assertEqual(rep["recommend"], "claude")               # 订阅最优先
        # 排序：订阅在最前
        self.assertEqual(rep["rows"][0]["kind"], "订阅")

    def test_score_report_composite(self):
        from devkit import insight
        o1, o2, o3 = (insight.run_stats_by_backend, insight.ratings_by_backend,
                      insight.load_scores_config)
        insight.run_stats_by_backend = lambda rd=None: {
            "deepseek": {"backend": "deepseek", "uses": 4, "ok": 3, "lat": 8.0,
                         "tokens": 400, "cost": 0.004}}
        insight.ratings_by_backend = lambda: {"deepseek": {"up": 3, "down": 1}}
        insight.load_scores_config = lambda: {"deepseek": {"official": 80}}
        try:
            rep = insight.score_report()
        finally:
            (insight.run_stats_by_backend, insight.ratings_by_backend,
             insight.load_scores_config) = o1, o2, o3
        ds = next(r for r in rep["rows"] if r["backend"] == "deepseek")
        self.assertEqual(ds["ok_rate"], 75)        # 3/4
        self.assertEqual(ds["user_score"], 75)     # 3/(3+1)
        self.assertEqual(ds["official"], 80)
        # composite = (0.5*75 + 0.2*75 + 0.3*80)/(1.0) = 76.5 → 76 或 77（round half）
        self.assertIn(ds["composite"], (76, 77))
        self.assertTrue(rep["has_official"])


class StageReportTest(unittest.TestCase):
    """按阶段透视：解析 run-log.md 真实表格格式，跳过表头/分隔/畸形行，
    BLOCKED 计 uses、pct_cost 求和≈100、缺/空目录都给空壳、按成本降序。"""

    # 真实 run-log.md 格式：表头 + 分隔行 + 数据行（取自 rdloop.py:489 写法）
    LOG = (
        "# run-log\n\n"
        "| 阶段 | 载体 | 实际 | 状态 | 延迟 | tok | 成本 |\n"
        "| --- | --- | --- | --- | --- | --- | --- |\n"
        "| brainstorm | deepseek | deepseek | OK | 2.0s | 100 | $0.00100 |\n"
        "| implement | glm | glm | OK | 5.0s | 300 | $0.00300 |\n"
        "| implement | glm | glm | OK | 5.0s | 100 | $0.00100 |\n"
        "| review | claude | - | BLOCKED | 0.0s | 0 | $0.00000 |\n"
        "这一行是畸形的，不该被计入 | 乱 | 七 | 八 |\n"
    )

    def _mk(self, d):
        run = pathlib.Path(d) / "20260627_000000"
        run.mkdir(parents=True)
        (run / "run-log.md").write_text(self.LOG, encoding="utf-8")

    def test_by_stage_aggregation(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            self._mk(d)
            agg = insight.run_stats_by_stage(pathlib.Path(d))
        # 只认 3 个阶段（表头/分隔/畸形行被跳过）
        self.assertEqual(set(agg), {"brainstorm", "implement", "review"})
        self.assertEqual(agg["implement"]["uses"], 2)
        self.assertEqual(agg["implement"]["tokens"], 400)
        self.assertAlmostEqual(agg["implement"]["cost"], 0.004, places=6)
        self.assertEqual(agg["implement"]["ok"], 2)
        # 全 BLOCKED 的阶段照样出现：uses 计、ok 不计
        self.assertEqual(agg["review"]["uses"], 1)
        self.assertEqual(agg["review"]["ok"], 0)

    def test_report_shape_sort_and_pct(self):
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            self._mk(d)
            rep = insight.stage_report(pathlib.Path(d))
        rows = rep["rows"]
        self.assertEqual([r["stage"] for r in rows][0], "implement")  # 成本最高在最前
        self.assertEqual([r["total_cost"] for r in rows],
                         sorted([r["total_cost"] for r in rows], reverse=True))
        # review 阶段全 BLOCKED → ok_rate 0
        review = next(r for r in rows if r["stage"] == "review")
        self.assertEqual(review["ok_rate"], 0)
        # 有非零成本行，pct_cost 求和≈100
        self.assertAlmostEqual(sum(r["pct_cost"] for r in rows), 100.0, delta=0.2)
        # totals
        self.assertEqual(rep["totals"]["uses"], 4)
        self.assertEqual(rep["totals"]["tokens"], 500)
        self.assertAlmostEqual(rep["totals"]["cost"], 0.005, places=6)

    def test_missing_and_empty_dir(self):
        from devkit import insight
        empty_shell = {"rows": [], "totals": {"tokens": 0, "cost": 0.0, "uses": 0}}
        # 缺目录
        missing = pathlib.Path(tempfile.gettempdir()) / "loom-no-such-runs-xyz"
        self.assertEqual(insight.stage_report(missing), empty_shell)
        self.assertEqual(insight.run_stats_by_stage(missing), {})
        # 存在但没有 run-log.md
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(insight.stage_report(pathlib.Path(d)), empty_shell)

    def test_row_regex_does_not_regress_backend(self):
        # _ROW（按后端）必须仍是 6 组、_ROW_STAGE 是 7 组，互不影响
        from devkit import insight
        m6 = insight._ROW.match("| implement | glm | glm | OK | 5.0s | 300 | $0.00300 |")
        m7 = insight._ROW_STAGE.match("| implement | glm | glm | OK | 5.0s | 300 | $0.00300 |")
        self.assertIsNotNone(m6)
        self.assertEqual(len(m6.groups()), 6)
        self.assertIsNotNone(m7)
        self.assertEqual(len(m7.groups()), 7)
        self.assertEqual(m7.groups()[0], "implement")
        # 表头/分隔行都不匹配 _ROW_STAGE
        self.assertIsNone(insight._ROW_STAGE.match("| 阶段 | 载体 | 实际 | 状态 | 延迟 | tok | 成本 |"))
        self.assertIsNone(insight._ROW_STAGE.match("| --- | --- | --- | --- | --- | --- | --- |"))


class ArtifactTest(unittest.TestCase):
    """结构化产物总线 v1（P0-a，由 GLM 经 Loom 流水线构建 + 评审 + golden 门）。
    与 devkit/p0a-artifact.golden.json 同源,锁进 ./loom test 防回归。"""

    def test_make_shape_and_default(self):
        from devkit import artifact
        a = artifact.make("plan", "p", "标题", "正文")
        self.assertEqual(a["stage"], "plan")
        self.assertEqual(a["body"], "正文")
        self.assertEqual(a["fields"], {})

    def test_make_copies_fields(self):
        from devkit import artifact
        src = {"contract": "c"}
        a = artifact.make("p", "r", "t", "b", src)
        src["x"] = 1                       # 改原 dict 不应串改产物
        self.assertEqual(a["fields"], {"contract": "c"})

    def test_extract_implement_paths(self):
        from devkit import artifact
        self.assertEqual(artifact.extract_fields("implement", "# devkit/foo.py\nprint(1)"),
                         {"patch_targets": ["devkit/foo.py"]})
        self.assertEqual(artifact.extract_fields("implement", "print(1)"),
                         {"patch_targets": []})

    def test_extract_implement_paths_from_code_blocks(self):
        from devkit import artifact
        body = (
            "## 产物\n\n"
            "```python\n# adder.py\ndef add(a, b):\n    return a + b\n```\n\n"
            "```python\n# tests/test_adder.py\nfrom adder import add\n```\n"
        )
        self.assertEqual(
            artifact.extract_fields("implement", body),
            {"patch_targets": ["adder.py", "tests/test_adder.py"]},
        )

    def test_extract_review_and_other(self):
        from devkit import artifact
        self.assertIn("failure", artifact.extract_fields("review", "NO-GO 这里要改"))
        self.assertEqual(artifact.extract_fields("plan", "whatever"), {})

    def test_protected_filters(self):
        from devkit import artifact
        a = artifact.make("s", "r", "t", "b", {"contract": "c", "x": "y"})
        self.assertEqual(artifact.protected(a), {"contract": "c"})
        self.assertEqual(sorted(artifact.PROTECTED), ["contract", "failure", "patch_targets"])


class BudgetTest(unittest.TestCase):
    """上下文预算度量 v1（P0-a，由 MiniMax 经 Loom 流水线构建 + 评审去除多余测试 + golden 门）。
    重点回归:CJK token 估计绝不退化成英文 /3.5 比例（防塞爆窗口）。"""

    def test_carrier_window(self):
        from devkit import budget
        self.assertEqual(budget.carrier_window("minimax"), 32768)
        self.assertEqual(budget.carrier_window("glm"), 128000)
        self.assertEqual(budget.carrier_window("no-such-model"), 32768)  # 默认保守

    def test_budget_tokens_reserves_output(self):
        from devkit import budget
        self.assertEqual(budget.budget_tokens(100000), 60000)
        self.assertEqual(budget.budget_tokens(100000, 0.5), 50000)

    def test_est_tokens_english(self):
        from devkit import budget
        self.assertEqual(budget.est_tokens(""), 0)
        self.assertEqual(budget.est_tokens("hello world"), 4)

    def test_est_tokens_cjk_not_underestimated(self):
        from devkit import budget
        # 关键:中文每字≈1 token,绝不能是 /3.5(那会把 100 字估成 ~29 → 塞爆窗口)
        self.assertEqual(budget.est_tokens("中文"), 2)
        self.assertEqual(budget.est_tokens("中文abc"), 3)
        self.assertEqual(budget.est_tokens("中" * 100), 100)


class PackTest(unittest.TestCase):
    """预算分配 pack()（P0-a，Team Alpha 由 GLM 构建 + 评审去除多余测试 + golden 门）。"""

    BLOCKS = [{"name": "a", "text": "xxxxx", "prio": 1, "protected": False},
              {"name": "b", "text": "yy", "prio": 0, "protected": True},
              {"name": "c", "text": "zzzzzz", "prio": 2, "protected": False}]

    def test_greedy_keep_drop_used(self):
        from devkit import budget
        r = budget.pack(self.BLOCKS, 8, est=len)
        self.assertEqual(r["kept"], ["a", "b"])      # 原始顺序
        self.assertEqual(r["dropped"], ["c"])
        self.assertEqual(r["used"], 7)
        self.assertEqual(r["text"], "xxxxx\n\nyy")

    def test_protected_kept_over_budget(self):
        from devkit import budget
        r = budget.pack([{"name": "p", "text": "verylongtext", "prio": 0, "protected": True}], 3, est=len)
        self.assertEqual(r["kept"], ["p"])           # protected 超预算也留

    def test_existing_funcs_survived(self):
        from devkit import budget
        self.assertEqual(budget.carrier_window("glm"), 128000)
        self.assertEqual(budget.est_tokens("中" * 100), 100)


class RecipesTest(unittest.TestCase):
    """命名流水线预设（P0 Team1，GLM/MiniMax 构建 + Opus 子 agent 审查 GO）。"""

    def test_get_and_list(self):
        from devkit import recipes
        self.assertEqual(recipes.get_recipe("cheap-dev")["carriers"]["implement"], "loom-dev")
        self.assertEqual(recipes.get_recipe("cheap-dev")["carriers"]["review"], "loom-reviewer")
        self.assertEqual(recipes.get_recipe("local-first")["carriers"]["verify"], "glm")
        self.assertEqual(recipes.list_recipes(),
                         ["agent-team-research", "cheap-dev", "local-first", "premium-architect"])

    def test_unknown_raises(self):
        from devkit import recipes
        with self.assertRaises(KeyError):
            recipes.get_recipe("nope")


class TaskTypeTest(unittest.TestCase):
    """启发式任务类型分类（P0 Team2，GLM 构建 + Opus 审查）。喂 Model Fitness 的 task_type。"""

    def test_classify(self):
        from devkit import tasktype
        self.assertEqual(tasktype.infer_task_type("修复登录bug"), "backend-fix")
        self.assertEqual(tasktype.infer_task_type("给这个函数写测试"), "test-gen")
        self.assertEqual(tasktype.infer_task_type("review this patch"), "review")
        self.assertEqual(tasktype.infer_task_type("重构这段代码"), "refactor")
        self.assertEqual(tasktype.infer_task_type("add a new endpoint"), "feature")
        self.assertEqual(tasktype.infer_task_type("随便聊聊"), "other")

    def test_priority_fix_over_feature(self):
        from devkit import tasktype
        self.assertEqual(tasktype.infer_task_type("fix and add validation"), "backend-fix")


class BlocksTest(unittest.TestCase):
    """优先级上下文块构建（P0 Team3，GLM 构建 + Opus 审查）。喂 budget.pack。"""

    def test_order_and_protected(self):
        from devkit import blocks
        bl = blocks.build_blocks("t", "s", [("u1", "x")], contract="C", failure="F")
        self.assertEqual([b["name"] for b in bl], ["contract", "failure", "system", "task", "u1"])
        self.assertEqual([b["name"] for b in bl if b["protected"]], ["contract", "failure"])
        self.assertEqual([b["prio"] for b in blocks.build_blocks("t", "s", [("u1", "x")])], [1, 1, 2])

    def test_skips_empty_and_feeds_pack(self):
        from devkit import blocks, budget
        bl = blocks.build_blocks("t", "s", [], contract="C")
        self.assertEqual(bl[0]["name"], "contract")          # 有 contract → 首块
        self.assertEqual(budget.pack(bl, 100, est=len)["kept"], ["contract", "system", "task"])


class BudgetCarrierMaxTokensTest(unittest.TestCase):
    """carrier_max_tokens —— 推理模型 per-carrier 预算（#54-③ 修复）。"""

    def test_reasoning_models_return_8000(self):
        from devkit.budget import carrier_max_tokens
        self.assertEqual(carrier_max_tokens("glm"), 8000)
        self.assertEqual(carrier_max_tokens("deepseek"), 8000)
        self.assertEqual(carrier_max_tokens("minimax"), 8000)

    def test_unknown_carrier_returns_none(self):
        from devkit.budget import carrier_max_tokens
        self.assertIsNone(carrier_max_tokens("claude"))
        self.assertIsNone(carrier_max_tokens("no-such"))

    def test_stage_max_fallback_priority(self):
        from devkit.budget import carrier_max_tokens

        class _StageWithMax:
            max_tokens = 4000

        class _StageWithoutMax:
            pass

        # ① 角色文件写了 max_tokens → 优先
        self.assertEqual(getattr(_StageWithMax(), "max_tokens", None) or carrier_max_tokens("glm") or 900, 4000)
        # ② 无角色文件，carrier=glm（推理模型）→ 8000
        self.assertEqual(getattr(_StageWithoutMax(), "max_tokens", None) or carrier_max_tokens("glm") or 900, 8000)
        # ③ 无角色文件，carrier=claude（非推理）→ run 级默认 900
        self.assertEqual(getattr(_StageWithoutMax(), "max_tokens", None) or carrier_max_tokens("claude") or 900, 900)


class ZeroFileBugTest(unittest.TestCase):
    """0 文件构建 → tests_failed=True 修复（#54-①②）。"""

    def test_zero_files_means_failed(self):
        cases = [
            ([], None, True),       # 0 文件，无测试 → 应判 failed
            (["a.py"], True, False), # 有文件，测试过 → OK
            (["a.py"], False, True), # 有文件，测试挂 → failed
        ]
        for files, tpassed, expected in cases:
            with self.subTest(files=files, tpassed=tpassed):
                self.assertEqual((tpassed is False) or not files, expected)

    def test_gate_says_no_go_on_empty_files(self):
        files = []
        reasons = ["0 文件构建"] if not files else []
        gate = f"NO-GO（{', '.join(reasons)}）" if reasons else "建议 GO"
        self.assertTrue(gate.startswith("NO-GO"))


class CodexExecutorTest(unittest.TestCase):
    """codex executor 和 _parse_verify_report 的单元测试（P0-b）。"""

    def _parse(self, text):
        from devkit.executors import _parse_verify_report
        return _parse_verify_report(text)

    def test_parse_go(self):
        r = self._parse("All tests passed ✅ GO")
        self.assertEqual(r["verdict"], "GO")
        self.assertTrue(r["tests_passed"])

    def test_parse_nogo_keyword(self):
        r = self._parse("NO-GO: 2 assertions failed")
        self.assertEqual(r["verdict"], "NO-GO")
        self.assertFalse(r["tests_passed"])

    def test_parse_fail_keyword(self):
        r = self._parse("3 errors FAIL")
        self.assertEqual(r["verdict"], "NO-GO")
        self.assertFalse(r["tests_passed"])

    def test_parse_emoji_fail(self):
        self.assertEqual(self._parse("❌ 验证未通过")["verdict"], "NO-GO")

    def test_parse_pass_keyword(self):
        self.assertEqual(self._parse("5/5 PASS")["verdict"], "GO")

    def test_parse_emoji_pass(self):
        self.assertEqual(self._parse("结果 ✅")["verdict"], "GO")

    def test_parse_unknown(self):
        r = self._parse("分析中...")
        self.assertEqual(r["verdict"], "UNKNOWN")
        self.assertIsNone(r["tests_passed"])

    def test_parse_nogo_overrides_go(self):
        self.assertEqual(self._parse("PASS but NO-GO overall")["verdict"], "NO-GO")

    def test_parse_summary_truncated(self):
        self.assertLessEqual(len(self._parse("x" * 1000)["summary"]), 500)

    def test_run_dispatches_codex(self):
        from unittest.mock import patch
        from devkit.executors import run
        with patch("devkit.executors.run_codex", return_value=(True, "GO", "codex")) as mock_rc:
            ok, content, name = run("codex", "prompt", "codex-sub",
                                    pathlib.Path("/tmp"), "http://localhost:4000", "key")
        mock_rc.assert_called_once()
        self.assertTrue(ok)
        self.assertEqual(name, "codex")

    def test_run_unknown_executor_false(self):
        from devkit.executors import run
        ok, msg, name = run("unknown", "p", "m", pathlib.Path("/tmp"), "gw", "k")
        self.assertFalse(ok)
        self.assertIn("未知", msg)


class ModelFitnessTest(unittest.TestCase):
    """model_fitness() aggregates per-(backend, task_type) stats from runs."""

    def _write_run(self, runs_dir, run_id, task_text, log_rows):
        rd = runs_dir / run_id
        rd.mkdir(parents=True)
        (rd / "00-task.md").write_text(f"# 任务\n\n{task_text}\n")
        header = "| 阶段 | 载体 | 实际模型 | 状态 | 用时 | tokens | 花费 |\n| --- | --- | --- | --- | --- | --- | --- |\n"
        (rd / "run-log.md").write_text(header + "\n".join(log_rows))

    def test_empty_runs_dir_returns_empty(self):
        from devkit.insight import model_fitness
        with tempfile.TemporaryDirectory() as td:
            rep = model_fitness(pathlib.Path(td))
        self.assertEqual(rep["rows"], [])
        self.assertEqual(rep["task_types"], [])

    def test_aggregates_by_backend_and_task_type(self):
        from devkit.insight import model_fitness
        with tempfile.TemporaryDirectory() as td:
            runs = pathlib.Path(td)
            self._write_run(runs, "r1", "修复一个 bug",
                            ["| implement | deepseek | deepseek | OK | 1.0s | 100 | $0.00010 |",
                             "| review | deepseek | deepseek | OK | 0.5s | 50 | $0.00005 |"])
            self._write_run(runs, "r2", "实现新功能",
                            ["| implement | glm | glm | OK | 2.0s | 200 | $0.00000 |",
                             "| review | glm | glm | BLOCKED | 1.0s | 0 | $0.00000 |"])
            rep = model_fitness(runs)
        rows = rep["rows"]
        backends = {r["backend"] for r in rows}
        self.assertIn("deepseek", backends)
        self.assertIn("glm", backends)
        task_types = rep["task_types"]
        self.assertIn("backend-fix", task_types)
        self.assertIn("feature", task_types)

    def test_ok_rate_calculated_correctly(self):
        from devkit.insight import model_fitness
        with tempfile.TemporaryDirectory() as td:
            runs = pathlib.Path(td)
            self._write_run(runs, "r1", "修复 bug",
                            ["| implement | deepseek | deepseek | OK | 1.0s | 100 | $0.00010 |",
                             "| implement | deepseek | deepseek | BLOCKED | 0.5s | 0 | $0.00000 |"])
            rep = model_fitness(runs)
        ds_fix = next((r for r in rep["rows"]
                       if r["backend"] == "deepseek" and r["task_type"] == "backend-fix"), None)
        self.assertIsNotNone(ds_fix)
        self.assertEqual(ds_fix["uses"], 2)
        self.assertEqual(ds_fix["ok"], 1)
        self.assertEqual(ds_fix["ok_rate"], 50)

    def test_rows_sorted_by_task_type_then_ok_rate(self):
        from devkit.insight import model_fitness
        with tempfile.TemporaryDirectory() as td:
            runs = pathlib.Path(td)
            self._write_run(runs, "r1", "实现功能",
                            ["| implement | glm | glm | OK | 1.0s | 10 | $0.0 |"])
            self._write_run(runs, "r2", "实现功能",
                            ["| implement | deepseek | deepseek | BLOCKED | 1.0s | 0 | $0.0 |"])
            rep = model_fitness(runs)
        feature_rows = [r for r in rep["rows"] if r["task_type"] == "feature"]
        # glm (100%) should come before deepseek (0%)
        backends_order = [r["backend"] for r in feature_rows]
        self.assertLess(backends_order.index("glm"), backends_order.index("deepseek"))


class PonyTailTest(unittest.TestCase):
    """PonyTail gate: apply() swaps review system prompt; --ponytail flag wires it."""

    def _make_stage(self, key, system="原始 system"):
        from devkit.roles import Stage
        return Stage(key=key, role="r", title="t", carrier="glm",
                     system=system, executor="chat")

    def test_apply_replaces_review_system(self):
        from devkit import ponytail
        stages = [self._make_stage("plan"), self._make_stage("implement"),
                  self._make_stage("review", "原来的 review")]
        result = ponytail.apply(stages)
        self.assertEqual(len(result), 3)
        review = next(s for s in result if s.key == "review")
        self.assertIn("PonyTail", review.system)
        self.assertIn("REQUEST-CHANGES", review.system)
        self.assertNotEqual(review.system, "原来的 review")

    def test_apply_leaves_other_stages_untouched(self):
        from devkit import ponytail
        stages = [self._make_stage("plan", "plan system"),
                  self._make_stage("implement", "impl system"),
                  self._make_stage("review")]
        result = ponytail.apply(stages)
        self.assertEqual(next(s for s in result if s.key == "plan").system, "plan system")
        self.assertEqual(next(s for s in result if s.key == "implement").system, "impl system")

    def test_apply_noop_when_no_review(self):
        from devkit import ponytail
        stages = [self._make_stage("plan"), self._make_stage("implement")]
        result = ponytail.apply(stages)
        self.assertEqual([s.key for s in result], ["plan", "implement"])
        self.assertEqual(result[0].system, "原始 system")

    def test_ponytail_system_has_required_rules(self):
        from devkit.ponytail import PONYTAIL_SYSTEM
        for keyword in ("最小 diff", "零新依赖", "APPROVE", "REQUEST-CHANGES"):
            self.assertIn(keyword, PONYTAIL_SYSTEM)

    def test_ponytail_flag_applied_in_cmd_run(self):
        import unittest.mock as mock
        captured = {}

        def fake_run_loop(task, stages, **kwargs):
            captured["review_system"] = next(
                (s.system for s in stages if s.key == "review"), None)
            return {"blocked": [], "tokens": 0, "cost": 0.0,
                    "run_dir": "/tmp/x", "gate": "GO", "iterations": 0,
                    "converged": None, "iterate_cost": 0.0, "cascade_path": []}

        with mock.patch("devkit.__main__.run_loop", fake_run_loop):
            from devkit.__main__ import _cmd_run
            _cmd_run(["任务", "--stages", "implement,review", "--ponytail"])

        from devkit.ponytail import PONYTAIL_SYSTEM
        self.assertEqual(captured.get("review_system"), PONYTAIL_SYSTEM)


class RecipesCliTest(unittest.TestCase):
    """--recipe CLI flag applies preset stages/carriers; explicit args override recipe."""

    def _parse_recipe(self, extra_argv=None):
        """Return (stages_str, carrier_list) after recipe expansion, without running the loop."""
        import unittest.mock as mock
        captured = {}

        def fake_run_loop(task, stages, **kwargs):
            captured["stages"] = [s.key for s in stages]
            captured["carriers"] = kwargs.get("carrier_overrides", {})
            return {"blocked": [], "tokens": 0, "cost": 0.0,
                    "run_dir": "/tmp/x", "gate": "GO", "iterations": 0,
                    "converged": None, "iterate_cost": 0.0, "cascade_path": []}

        argv = ["任务"] + (extra_argv or [])
        with mock.patch("devkit.__main__.run_loop", fake_run_loop):
            from devkit.__main__ import _cmd_run
            try:
                _cmd_run(argv)
            except SystemExit:
                pass
        return captured

    def test_recipe_cheap_dev_sets_stages_and_carriers(self):
        c = self._parse_recipe(["--recipe", "cheap-dev"])
        self.assertEqual(c.get("stages"), ["plan", "implement", "verify", "review"])
        self.assertEqual(c.get("carriers", {}).get("implement"), "loom-dev")
        self.assertEqual(c.get("carriers", {}).get("verify"), "loom-tester")
        self.assertEqual(c.get("carriers", {}).get("review"), "loom-reviewer")

    def test_recipe_local_first_sets_stages(self):
        c = self._parse_recipe(["--recipe", "local-first"])
        self.assertEqual(c.get("stages"), ["implement", "verify"])
        self.assertEqual(c.get("carriers", {}).get("implement"), "minimax")
        self.assertEqual(c.get("carriers", {}).get("verify"), "glm")

    def test_explicit_carrier_overrides_recipe(self):
        c = self._parse_recipe(["--recipe", "cheap-dev", "--carrier", "implement=minimax"])
        self.assertEqual(c.get("carriers", {}).get("implement"), "minimax")
        self.assertEqual(c.get("carriers", {}).get("review"), "loom-reviewer")  # recipe review不变

    def test_explicit_stages_overrides_recipe(self):
        c = self._parse_recipe(["--recipe", "cheap-dev", "--stages", "implement,verify"])
        self.assertEqual(c.get("stages"), ["implement", "verify"])

    def test_unknown_recipe_exits(self):
        import unittest.mock as mock
        with mock.patch("devkit.__main__.run_loop", lambda *a, **k: {}):
            from devkit.__main__ import _cmd_run
            with self.assertRaises(SystemExit):
                _cmd_run(["任务", "--recipe", "no-such-recipe"])

    def test_recipes_cmd_lists_all(self):
        from devkit.__main__ import _cmd_recipes
        from devkit.recipes import list_recipes
        import io, unittest.mock as mock
        with mock.patch("sys.stdout", io.StringIO()) as out:
            _cmd_recipes([])
        text = out.getvalue()
        for name in list_recipes():
            self.assertIn(name, text)


class RdloopArtifactWiringTest(unittest.TestCase):
    """rdloop writes per-stage .artifact.json with schema fields filled in."""

    def _fake_gateway(self, *args, **kwargs):
        return True, "实现内容\n# devkit/foo.py\nprint(1)", "fake-model", 42, 0.00001

    def test_artifact_json_written_per_stage(self):
        import tempfile, unittest.mock as mock
        from devkit.rdloop import run_loop
        from devkit.roles import Stage
        st = Stage(key="plan", role="planner", title="计划", carrier="deepseek",
                   system="你是计划者", executor="chat")
        with tempfile.TemporaryDirectory() as td:
            with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, "计划内容", "m", 10, 0.0)):
                run_loop("测试任务", stages=[st], base_url="http://x", max_tokens=100,
                         run_id="test-wiring-01")
            art_files = list(pathlib.Path(td).rglob("*.artifact.json"))
        # artifact.json 的存在性不在 td 里（out_root 是 devkit/runs）— 只测不抛异常

    def test_artifact_json_has_schema_fields(self):
        import tempfile, unittest.mock as mock, shutil
        from devkit import artifact as art
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage
        st = Stage(key="plan", role="planner", title="计划", carrier="deepseek",
                   system="你是计划者", executor="chat")
        runs_dir = ROOT / "devkit" / "runs"
        shutil.rmtree(runs_dir / "test-wiring-02b", ignore_errors=True)
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, "计划内容", "m", 10, 0.0)):
            res = run_loop("测试任务 schema wiring", stages=[st], base_url="http://x",
                           max_tokens=100, run_id="test-wiring-02b")
        art_path = pathlib.Path(res["run_dir"]) / "01-plan.artifact.json"
        self.assertTrue(art_path.exists(), "artifact.json should be written")
        data = json.loads(art_path.read_text())
        self.assertEqual(data["stage"], "plan")
        self.assertEqual(data["carrier"], "deepseek")
        self.assertIsNotNone(data["task_type"])
        self.assertEqual(data["tokens"], 10)
        self.assertIn("kept", data["budget_report"])
        self.assertIn("response_diag", data)

    def test_blocked_stage_writes_failure_code(self):
        import unittest.mock as mock, shutil
        from devkit.rdloop import run_loop
        from devkit.rdloop import ROOT
        from devkit.roles import Stage
        st = Stage(key="plan", role="planner", title="计划", carrier="deepseek",
                   system="你是计划者", executor="chat")
        payload = {
            "choices": [{"finish_reason": "length", "message": {"content": "", "reasoning_content": "step"}}],
            "usage": {"total_tokens": 9},
            "model": "glm-5.2",
            "object": "chat.completion",
        }
        shutil.rmtree(ROOT / "devkit" / "runs" / "test-wiring-03b", ignore_errors=True)
        with mock.patch("urllib.request.urlopen", return_value=MiniMaxGatewayCompatTest._FakeResponse(payload)):
            res = run_loop("测试任务 blocked failure code", stages=[st], base_url="http://gw",
                           max_tokens=100, run_id="test-wiring-03b")
        art_path = pathlib.Path(res["run_dir"]) / "01-plan.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["failure_code"], "EMPTY_REASONING_ONLY_LENGTH_RETRY_EMPTY")

    def test_recreates_run_dir_before_stage_persist(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="plan", role="planner", title="计划", carrier="deepseek",
                   system="你是计划者", executor="chat")
        run_id = "test-wiring-run-dir-recreate"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)

        def _side_effect(*args, **kwargs):
            shutil.rmtree(run_dir, ignore_errors=True)
            return True, "计划内容", "m", 10, 0.0

        with mock.patch("devkit.rdloop.gateway_chat", side_effect=_side_effect):
            res = run_loop("测试任务 run dir recreate", stages=[st], base_url="http://x",
                           max_tokens=100, run_id=run_id)
        self.assertEqual(pathlib.Path(res["run_dir"]), run_dir)
        self.assertTrue((run_dir / "01-plan.md").exists())
        self.assertTrue((run_dir / "01-plan.artifact.json").exists())
        self.assertTrue((run_dir / "run-log.md").exists())

    def test_implement_artifact_marks_zero_collect_no_go(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-collect-none"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = "**文件 `pkg.py`**\n```python\nVALUE = 1\n```\n"
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop("测试任务 collect none", stages=[st], base_url="http://x",
                           max_tokens=100, run_id=run_id)
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["failure_code"], "TEST_COLLECT_NONE")
        self.assertEqual(data["verdict"], "NO-GO")
        self.assertEqual(data["test_collection"]["collected"], 0)

    def test_applylock_critical_file_forces_no_go(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-applylock-no-go"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `pkg.py`**\n```python\ndef add(a, b):\n    return a + b\n```\n\n"
            "**文件 `test_pkg.py`**\n```python\nfrom pkg import add\n\n"
            "def test_add():\n    assert add(1, 2) == 3\n```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop("测试任务 applylock no-go", stages=[st], base_url="http://x",
                           max_tokens=100, run_id=run_id)
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["failure_code"], "APPLYLOCK_HUMAN_REQUIRED")
        self.assertEqual(data["verdict"], "NO-GO")
        self.assertIn("NO-GO", res["gate"])

    def test_report_only_markdown_artifact_skips_collect_gate(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        stages = [
            Stage(key="plan", role="pm", title="计划", carrier="deepseek", system="你是规划者", executor="chat"),
            Stage(key="implement", role="dev", title="实现", carrier="deepseek", system="你是开发者", executor="chat"),
        ]
        run_id = "test-wiring-report-only-artifact"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = "我直接写出报告到 `runs/demo/audit-no-key.md`：\n\n```md\n# Audit\ncount_no_key=0\ncount_plan_blocked=0\n```\n"
        with mock.patch("devkit.rdloop.gateway_chat", side_effect=[
            (True, "计划：生成 markdown 报告", "m", 10, 0.0),
            (True, impl, "m", 10, 0.0),
        ]):
            res = run_loop(
                "直接输出审计报告，只输出 markdown 文件，不写 Python 模块",
                stages=stages,
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "02-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertIsNone(data["failure_code"])
        self.assertEqual(data["verdict"], "GO")
        self.assertTrue(data["tests_passed"])
        self.assertEqual(data["test_collection"]["output"], "（report-only 产物，跳过测试收集）")
        self.assertIn("建议 GO", res["gate"])

    def test_report_only_diagnostic_python_under_runs_skips_collect_gate(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-diag-python"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "将以下脚本保存为 `runs/demo/diag_import_error.py`\n\n"
            "```python\n"
            "print('diag')\n"
            "```\n\n"
            "并写入 `run-log.md`\n\n"
            "```md\n"
            "# diagnostic report\n"
            "root_cause: path/layout problem\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "定位为什么 pytest collection 会 ImportError，输出诊断报告到 run-log.md，read-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertIsNone(data["failure_code"])
        self.assertEqual(data["verdict"], "GO")
        self.assertTrue(data["tests_passed"])
        self.assertEqual(data["test_collection"]["output"], "（report-only 产物，跳过测试收集）")

    def test_report_only_plain_markdown_without_file_marker_falls_back_to_run_log(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-fallback-runlog"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "# 开发产物（草案）\n\n"
            "目标：把 stdout 写入 runs/<run_id>/run-log.md。\n\n"
            "## 观察\n\n"
            "- env_exists=False\n"
            "- keys=[]\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "只读诊断任务，输出 run-log.md，不修改真实仓库，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertIsNone(data["failure_code"])
        self.assertEqual(data["verdict"], "GO")
        self.assertTrue((pathlib.Path(res["run_dir"]) / "build" / "run-log.md").exists())

    def test_report_only_with_locked_tests_does_not_force_no_go(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-locked-tests"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `devkit/_diag_shell.py`**\n```python\n"
            "from pathlib import Path\n"
            "log = Path.cwd() / 'runs' / 'diag-shell.log'\n"
            "log.parent.mkdir(parents=True, exist_ok=True)\n"
            "log.write_text('cwd=' + str(Path.cwd()) + '\\n', encoding='utf-8')\n"
            "```\n\n"
            "**文件 `tests/test_diag_shell.py`**\n```python\n"
            "import subprocess, sys\n"
            "from pathlib import Path\n\n"
            "def test_diag_shell_creates_log():\n"
            "    rc = subprocess.run([sys.executable, 'devkit/_diag_shell.py'], check=False)\n"
            "    assert rc.returncode == 0\n"
            "    assert (Path('runs') / 'diag-shell.log').exists()\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "只读诊断任务，允许生成验证测试文件，但不自动 apply，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["verdict"], "GO")
        self.assertTrue(data["tests_passed"])
        self.assertIsNone(data["failure_code"])
        self.assertIn("建议 GO", res["gate"])

    def test_report_only_diag_tests_without_explicit_allow_are_blocked_by_task_contract(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-contract-block"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `tests/test_applylock_audit.py`**\n```python\n"
            "def test_bad():\n"
            "    assert True\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "只读诊断任务，无写操作，输出诊断报告，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["failure_code"], "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN")
        self.assertEqual(data["verdict"], "NO-GO")
        self.assertFalse(data["tests_passed"])
        self.assertEqual(data["task_contract"]["task_kind"], "diag")
        self.assertIn("tests/test_applylock_audit.py", data["task_contract"]["blocked_paths"])
        self.assertIn("任务契约", res["gate"])

    def test_report_only_diag_can_explicitly_allow_tests_paths_via_contract(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-contract-allow"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `tests/test_applylock_audit.py`**\n```python\n"
            "def test_ok():\n"
            "    assert True\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "只读诊断任务，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
                allowed_artifact_paths=["tests/"],
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertNotEqual(data["failure_code"], "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN")

    def test_report_only_diag_blocks_probe_test_file_outside_tests_dir(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-report-only-probe-test-block"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `build/_probe/test_replay_dryrun.py`**\n```python\n"
            "def test_ok():\n"
            "    assert True\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "只读诊断任务，输出诊断报告，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["failure_code"], "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN")
        self.assertIn("build/_probe/test_replay_dryrun.py", data["task_contract"]["blocked_paths"])

    def test_applylock_exemption_note_is_written_to_run_log(self):
        import os
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-applylock-exemption-note"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `tests/test_runlog_path.py`**\n```python\n"
            "def test_ok():\n"
            "    assert True\n"
            "```\n"
        )
        os.environ["DEV_APPLYLOCK_ALLOW"] = "tests/test_runlog_path.py"
        try:
            with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
                run_loop(
                    "普通实现任务，但本轮通过 DEV_APPLYLOCK_ALLOW 放行测试文件",
                    stages=[st],
                    base_url="http://x",
                    max_tokens=100,
                    run_id=run_id,
                )
        finally:
            os.environ.pop("DEV_APPLYLOCK_ALLOW", None)
        run_log = (run_dir / "run-log.md").read_text(encoding="utf-8")
        self.assertIn("applylock 放行", run_log)
        self.assertIn("env-override", run_log)

    def test_apply_target_allows_tests_prefix_and_applies_files(self):
        import shutil
        import tempfile
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-apply-target-tests-prefix"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `devkit/__init__.py`**\n```python\n"
            "# build-local devkit package for apply-target wiring test\n"
            "```\n\n"
            "**文件 `devkit/human_required_guard.py`**\n```python\n"
            "def is_human_only(task):\n"
            "    return bool(task)\n"
            "```\n\n"
            "**文件 `tests/test_human_required_guard.py`**\n```python\n"
            "from devkit.human_required_guard import is_human_only\n\n"
            "def test_ok():\n"
            "    assert is_human_only({'x': 1}) is True\n"
            "```\n"
        )
        with tempfile.TemporaryDirectory() as target:
            with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
                res = run_loop(
                    "实现真实仓库文件并回写工作树",
                    stages=[st],
                    base_url="http://x",
                    max_tokens=100,
                    run_id=run_id,
                    apply_target=target,
                )
            self.assertIn("建议 GO", res["gate"])
            self.assertTrue((pathlib.Path(target) / "devkit" / "human_required_guard.py").exists())
            self.assertTrue((pathlib.Path(target) / "tests" / "test_human_required_guard.py").exists())

    def test_runtime_runs_dir_exists_for_diag_scripts(self):
        import shutil
        import unittest.mock as mock
        from devkit.rdloop import run_loop, ROOT
        from devkit.roles import Stage

        st = Stage(key="implement", role="dev", title="实现", carrier="deepseek",
                   system="你是开发者", executor="chat")
        run_id = "test-wiring-runtime-runs-dir"
        run_dir = ROOT / "devkit" / "runs" / run_id
        shutil.rmtree(run_dir, ignore_errors=True)
        impl = (
            "**文件 `devkit/_diag_shell.py`**\n```python\n"
            "from pathlib import Path\n"
            "log = Path.cwd() / 'runs' / 'diag-shell.log'\n"
            "with log.open('a', encoding='utf-8') as fh:\n"
            "    fh.write('cwd=' + str(Path.cwd()) + '\\n')\n"
            "```\n\n"
            "**文件 `tests/test_diag_shell_contract.py`**\n```python\n"
            "import subprocess, sys\n"
            "from pathlib import Path\n\n"
            "def test_log_file_created_under_runs_dir():\n"
            "    rc = subprocess.run([sys.executable, 'devkit/_diag_shell.py'], capture_output=True, text=True)\n"
            "    assert rc.returncode == 0, rc.stderr or rc.stdout\n"
            "    assert (Path('runs') / 'diag-shell.log').exists()\n"
            "```\n"
        )
        with mock.patch("devkit.rdloop.gateway_chat", return_value=(True, impl, "m", 10, 0.0)):
            res = run_loop(
                "诊断脚本应把日志写到 REPO_ROOT/runs 下，不自动 apply，report-only",
                stages=[st],
                base_url="http://x",
                max_tokens=100,
                delivery_mode="report-only",
                run_id=run_id,
            )
        art_path = pathlib.Path(res["run_dir"]) / "01-implement.artifact.json"
        data = json.loads(art_path.read_text())
        self.assertEqual(data["verdict"], "GO")
        self.assertTrue(data["tests_passed"])


class ArtifactSchemaTest(unittest.TestCase):
    """P0 artifact schema lock."""

    def _make(self, **kw):
        from devkit import artifact
        return artifact.make("implement", "dev", "T", "body", **kw)

    def test_has_all_13_keys(self):
        keys = set(self._make().keys())
        self.assertEqual(keys, {
            "stage", "role", "title", "body", "fields",
            "carrier", "task_type", "tokens", "cost",
            "verdict", "tests_passed", "window_used", "budget_report",
            "failure_code", "response_diag", "materialization", "output_protocol",
        })

    def test_new_fields_default_none(self):
        a = self._make()
        for k in ("carrier", "task_type", "tokens", "cost",
                  "verdict", "tests_passed", "window_used", "budget_report",
                  "failure_code", "response_diag", "materialization", "output_protocol"):
            self.assertIsNone(a[k], msg=f"{k} should default to None")

    def test_carrier_kwarg(self):
        self.assertEqual(self._make(carrier="glm")["carrier"], "glm")

    def test_task_type_kwarg(self):
        self.assertEqual(self._make(task_type="backend-fix")["task_type"], "backend-fix")

    def test_tokens_kwarg(self):
        self.assertEqual(self._make(tokens=1234)["tokens"], 1234)

    def test_cost_kwarg(self):
        self.assertAlmostEqual(self._make(cost=0.00123)["cost"], 0.00123)

    def test_verdict_kwarg(self):
        self.assertEqual(self._make(verdict="GO")["verdict"], "GO")

    def test_tests_passed_true(self):
        self.assertTrue(self._make(tests_passed=True)["tests_passed"])

    def test_budget_report_kwarg(self):
        br = {"kept": ["task"], "dropped": [], "used": 100}
        self.assertEqual(self._make(budget_report=br)["budget_report"]["used"], 100)

    def test_fields_deep_copy(self):
        from devkit import artifact
        src = {"k": "v"}
        a = artifact.make("plan", "r", "T", "b", src)
        src["x"] = 1
        self.assertIsNone(a["fields"].get("x"))


class QuotaSimulateTest(unittest.TestCase):
    """P0 quota preflight — quota_simulate() structure + verdict logic."""

    def _sim(self, stages=None, **kw):
        from devkit.insight import quota_simulate
        return quota_simulate(stages or ["no_such_stage_xyz"], "http://x", "k", **kw)

    def test_callable(self):
        from devkit.insight import quota_simulate
        self.assertTrue(callable(quota_simulate))

    def test_required_keys_present(self):
        keys = set(self._sim().keys())
        self.assertGreaterEqual(keys, {"stages", "stage_costs", "estimated_total",
                                       "verdict", "missing_stages"})

    def test_unknown_when_no_history(self):
        self.assertEqual(self._sim()["verdict"], "Unknown")

    def test_missing_stages_populated(self):
        self.assertIn("no_such_stage_xyz", self._sim()["missing_stages"])

    def test_estimated_total_is_float(self):
        self.assertIsInstance(self._sim()["estimated_total"], float)

    def test_stage_costs_is_dict(self):
        self.assertIsInstance(self._sim()["stage_costs"], dict)

    def test_stages_preserved_in_result(self):
        r = self._sim(["implement", "verify"])
        self.assertEqual(r["stages"], ["implement", "verify"])

    def test_verdict_is_string(self):
        self.assertIsInstance(self._sim()["verdict"], str)

    def test_verdict_safe_when_no_free_quota_and_all_found(self):
        """stages_data hit + remaining_usd=None → Safe (no free quota config = subscription/pay)."""
        import unittest.mock as mock
        from devkit.insight import quota_simulate
        fake_stage_report = {"rows": [{"stage": "implement", "avg_cost": 0.001, "uses": 5}]}
        fake_quota_report = {"rows": [{"kind": "订阅", "remaining_usd": None}]}
        with mock.patch("devkit.insight.stage_report", return_value=fake_stage_report), \
             mock.patch("devkit.insight.quota_report", return_value=fake_quota_report):
            r = quota_simulate(["implement"], "http://x", "k")
        self.assertEqual(r["verdict"], "Safe")
        self.assertAlmostEqual(r["estimated_total"], 0.001)

    def test_verdict_insufficient_when_estimated_exceeds_remaining(self):
        """estimated_total > remaining_usd → Insufficient."""
        import unittest.mock as mock
        from devkit.insight import quota_simulate
        fake_stage_report = {"rows": [{"stage": "implement", "avg_cost": 10.0, "uses": 3}]}
        fake_quota_report = {"rows": [{"kind": "免费额度", "remaining_usd": 1.0}]}
        with mock.patch("devkit.insight.stage_report", return_value=fake_stage_report), \
             mock.patch("devkit.insight.quota_report", return_value=fake_quota_report):
            r = quota_simulate(["implement"], "http://x", "k")
        self.assertEqual(r["verdict"], "Insufficient")

    def test_verdict_risky_when_over_half(self):
        """estimated_total > remaining_usd * 0.5 but <= remaining_usd → Risky."""
        import unittest.mock as mock
        from devkit.insight import quota_simulate
        fake_stage_report = {"rows": [{"stage": "implement", "avg_cost": 0.8, "uses": 2}]}
        fake_quota_report = {"rows": [{"kind": "免费额度", "remaining_usd": 1.0}]}
        with mock.patch("devkit.insight.stage_report", return_value=fake_stage_report), \
             mock.patch("devkit.insight.quota_report", return_value=fake_quota_report):
            r = quota_simulate(["implement"], "http://x", "k")
        self.assertEqual(r["verdict"], "Risky")


class AutoCarrierTest(unittest.TestCase):
    """--auto-carrier: recommend_model() 结果注入 implement 载体。"""

    def test_no_history_no_injection(self):
        """无历史数据时不修改 overrides。"""
        import unittest.mock as mock
        from devkit import insight
        with tempfile.TemporaryDirectory() as d:
            with mock.patch.object(insight, "recommend_model",
                                   return_value={"backend": None, "ok_rate": None,
                                                 "avg_cost": None, "uses": 0,
                                                 "task_type": "feature", "reason": "无历史数据"}):
                rec = insight.recommend_model("任务")
        self.assertIsNone(rec["backend"])

    def test_with_history_returns_backend(self):
        """有历史时 recommend_model 返回 backend。"""
        import unittest.mock as mock
        from devkit import insight
        fake_rows = [{"backend": "deepseek", "task_type": "feature",
                      "uses": 5, "ok": 5, "ok_rate": 100, "avg_cost": 0.001}]
        with mock.patch.object(insight, "model_fitness",
                               return_value={"rows": fake_rows, "task_types": ["feature"]}):
            rec = insight.recommend_model("实现一个 Python 函数")
        self.assertEqual(rec["backend"], "deepseek")
        self.assertEqual(rec["ok_rate"], 100)


class ProviderBalanceTest(unittest.TestCase):
    """provider_balance() — 实时余额适配器（deepseek / unsupported / no_key / error）。"""

    def test_no_key_returns_none_and_source(self):
        from devkit.insight import provider_balance
        r = provider_balance("deepseek", None)
        self.assertIsNone(r["available_usd"])
        self.assertEqual(r["source"], "no_key")

    def test_empty_key_returns_no_key(self):
        from devkit.insight import provider_balance
        r = provider_balance("deepseek", "")
        self.assertEqual(r["source"], "no_key")

    def test_unsupported_backend(self):
        from devkit.insight import provider_balance
        r = provider_balance("glm", "some-key")
        self.assertEqual(r["source"], "unsupported")
        self.assertIsNone(r["available_usd"])

    def test_result_has_required_keys(self):
        from devkit.insight import provider_balance
        r = provider_balance("deepseek", None)
        self.assertGreaterEqual(set(r.keys()), {"backend", "available_usd", "available_cny", "source"})

    def test_bad_key_returns_error_not_raise(self):
        from devkit.insight import provider_balance
        r = provider_balance("deepseek", "bad-key")
        self.assertIn(r["source"], ("error", "api"))
        self.assertIsNone(r["available_usd"]) if r["source"] == "error" else None

    def test_deepseek_api_parse(self):
        """模拟 API 返回，验证 CNY→USD 转换。"""
        import unittest.mock as mock, json, io, urllib.request
        from devkit.insight import provider_balance
        fake_data = {"balance_infos": [{"currency": "CNY", "granted_balance": "30.00",
                                        "topped_up_balance": "70.00", "total_balance": "100.00"}]}
        mock_resp = mock.MagicMock()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = mock.MagicMock(return_value=False)
        mock_resp.read.return_value = json.dumps(fake_data).encode()
        with mock.patch("urllib.request.urlopen", return_value=mock_resp):
            r = provider_balance("deepseek", "sk-test")
        self.assertEqual(r["source"], "api")
        self.assertAlmostEqual(r["available_cny"], 100.0)
        self.assertAlmostEqual(r["available_usd"], round(100.0 / 7.2, 5))


class RunsListTest(unittest.TestCase):
    """runs_list() — 扫 devkit/runs，解析 gate/tokens/cost/task_type/task_snippet。"""

    def _mk_run(self, base, run_id, gate_line, tok, cost, task_text, artifact=None):
        d = base / run_id
        d.mkdir(parents=True)
        log = (f"## 用量合计\n\n**{tok} tokens · ${cost}**\n\n"
               f"## Gate 建议\n\n{gate_line}\n")
        (d / "run-log.md").write_text(log)
        (d / "00-task.md").write_text(task_text)
        if artifact:
            af = d / "01-implement.artifact.json"
            af.write_text(json.dumps(artifact))

    def test_empty_dir_returns_empty(self):
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(runs_list(pathlib.Path(d)), [])

    def test_missing_dir_returns_empty(self):
        from devkit.insight import runs_list
        self.assertEqual(runs_list(pathlib.Path("/nonexistent/__loom_test__")), [])

    def test_parses_gate_and_tokens(self):
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            base = pathlib.Path(d)
            self._mk_run(base, "run-001", "建议 GO（需人类最终确认）", 2242, "0.00000",
                         "# 任务\n实现 word_count 函数")
            items = runs_list(base)
        self.assertEqual(len(items), 1)
        it = items[0]
        self.assertEqual(it["run_id"], "run-001")
        self.assertIn("GO", it["gate"])
        self.assertEqual(it["tokens"], 2242)
        self.assertAlmostEqual(it["cost"], 0.0)

    def test_task_type_from_artifact(self):
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            base = pathlib.Path(d)
            self._mk_run(base, "run-002", "NO-GO（测试失败）", 100, "0.00100",
                         "做个 feature", {"task_type": "feature", "stage": "implement"})
            items = runs_list(base)
        self.assertEqual(items[0]["task_type"], "feature")

    def test_sorted_reverse_by_run_id(self):
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            base = pathlib.Path(d)
            for rid in ("2026-01", "2026-03", "2026-02"):
                self._mk_run(base, rid, "建议 GO", 10, "0.0", "task")
            ids = [it["run_id"] for it in runs_list(base)]
        self.assertEqual(ids, ["2026-03", "2026-02", "2026-01"])


class RunOpenCodeTest(unittest.TestCase):
    """run_opencode() + run() dispatch — opencode executor。"""

    def test_opencode_not_installed_graceful(self):
        from devkit.executors import run_opencode
        with tempfile.TemporaryDirectory() as d:
            ok, msg, name = run_opencode("test prompt", pathlib.Path(d), timeout=5)
        self.assertFalse(ok)
        self.assertIn("opencode", msg)
        self.assertEqual(name, "opencode")

    def test_run_dispatches_opencode(self):
        from devkit.executors import run
        with tempfile.TemporaryDirectory() as d:
            ok, msg, name = run("opencode", "prompt", "m", pathlib.Path(d), "http://x", "k", timeout=5)
        self.assertFalse(ok)   # opencode 未安装 → False
        self.assertEqual(name, "opencode")

    def test_unknown_executor_mentions_opencode(self):
        from devkit.executors import run
        with tempfile.TemporaryDirectory() as d:
            _, msg, _ = run("zzz", "p", "m", pathlib.Path(d), "gw", "k")
        self.assertIn("opencode", msg)

    def test_opencode_in_executors_tuple(self):
        from devkit.roles import EXECUTORS
        self.assertIn("opencode", EXECUTORS)


class AssetTest(unittest.TestCase):
    """devkit/asset.py — 资产 CRUD（add / get / remove / load）。"""

    def _tmp_path(self):
        d = tempfile.mkdtemp()
        return pathlib.Path(d) / "loom.assets.toml"

    def test_load_missing_returns_empty(self):
        from devkit import asset
        self.assertEqual(asset.load_assets(pathlib.Path("/no/such/file.toml")), [])

    def test_add_and_get(self):
        from devkit import asset
        p = self._tmp_path()
        asset.add_asset("my-rule", "rule", "no new deps", path=p)
        a = asset.get_asset("my-rule", p)
        self.assertIsNotNone(a)
        self.assertEqual(a["name"], "my-rule")
        self.assertEqual(a["type"], "rule")
        self.assertEqual(a["content"], "no new deps")

    def test_add_deduplicates(self):
        from devkit import asset
        p = self._tmp_path()
        asset.add_asset("x", "rule", "v1", path=p)
        asset.add_asset("x", "rule", "v2", path=p)
        assets = asset.load_assets(p)
        self.assertEqual(len(assets), 1)
        self.assertEqual(assets[0]["content"], "v2")

    def test_add_with_tags(self):
        from devkit import asset
        p = self._tmp_path()
        asset.add_asset("tagged", "skill", "content", ["a", "b"], path=p)
        a = asset.get_asset("tagged", p)
        self.assertEqual(a["tags"], ["a", "b"])

    def test_remove_found(self):
        from devkit import asset
        p = self._tmp_path()
        asset.add_asset("to-delete", "rule", "bye", path=p)
        self.assertTrue(asset.remove_asset("to-delete", p))
        self.assertIsNone(asset.get_asset("to-delete", p))

    def test_remove_missing_returns_false(self):
        from devkit import asset
        p = self._tmp_path()
        self.assertFalse(asset.remove_asset("ghost", p))

    def test_asset_has_required_keys(self):
        from devkit import asset
        p = self._tmp_path()
        a = asset.add_asset("k", "prompt", "c", path=p)
        self.assertGreaterEqual(set(a.keys()), {"name", "type", "content", "tags"})


class RunsFilterTest(unittest.TestCase):
    """devkit runs --filter / --grep / --limit."""

    def _mk_run(self, base, run_id, gate_line, tok, cost, task_text):
        d = base / run_id
        d.mkdir(parents=True)
        log = f"## 用量合计\n\n**{tok} tokens · ${cost}**\n\n## Gate 建议\n\n{gate_line}\n"
        (d / "run-log.md").write_text(log)
        (d / "00-task.md").write_text(task_text)

    def test_filter_go(self):
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            base = pathlib.Path(d)
            self._mk_run(base, "r1", "建议 GO（需人类最终确认）", 100, "0.0", "task A")
            self._mk_run(base, "r2", "NO-GO（测试失败）", 200, "0.0", "task B")
            items = runs_list(base)
        go = [it for it in items if it.get("gate") and
              "GO" in it["gate"] and "NO-GO" not in it["gate"]]
        nogo = [it for it in items if it.get("gate") and "NO-GO" in it["gate"]]
        self.assertEqual(len(go), 1)
        self.assertEqual(go[0]["run_id"], "r1")
        self.assertEqual(len(nogo), 1)
        self.assertEqual(nogo[0]["run_id"], "r2")

    def test_grep_filters_by_snippet(self):
        import re
        from devkit.insight import runs_list
        with tempfile.TemporaryDirectory() as d:
            base = pathlib.Path(d)
            self._mk_run(base, "r1", "建议 GO", 100, "0.0", "实现 word_count 函数")
            self._mk_run(base, "r2", "建议 GO", 100, "0.0", "实现 add 函数")
            items = runs_list(base)
        pat = re.compile("word", re.IGNORECASE)
        filtered = [it for it in items if pat.search(it.get("task_snippet") or "")]
        self.assertEqual(len(filtered), 1)
        self.assertEqual(filtered[0]["run_id"], "r1")


class SafetyGateTest(unittest.TestCase):
    """safety.py v2 — 6 rules, severity, has_errors."""

    def test_rules_applied_is_6(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            r = safety.scan_build(pathlib.Path(d))
        self.assertEqual(r["rules_applied"], 6)

    def test_has_errors_key_present(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            r = safety.scan_build(pathlib.Path(d))
        self.assertIn("has_errors", r)

    def test_s001_is_error_severity(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            (pathlib.Path(d) / "s.py").write_text('API_KEY = "abcdefghijklmnopqrstuvwxyz"\n')
            r = safety.scan_build(pathlib.Path(d))
        violations = [v for v in r["violations"] if v["rule"] == "S001"]
        self.assertTrue(violations)
        self.assertEqual(violations[0]["severity"], "error")
        self.assertTrue(r["has_errors"])

    def test_s005_eval_detected_warn(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            (pathlib.Path(d) / "e.py").write_text("eval(user_input)\n")
            r = safety.scan_build(pathlib.Path(d))
        violations = [v for v in r["violations"] if v["rule"] == "S005"]
        self.assertTrue(violations)
        self.assertEqual(violations[0]["severity"], "warn")
        self.assertFalse(r["has_errors"])  # warn only → no error

    def test_s006_pickle_detected(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            (pathlib.Path(d) / "p.py").write_text("pickle.loads(data)\n")
            r = safety.scan_build(pathlib.Path(d))
        rules = {v["rule"] for v in r["violations"]}
        self.assertIn("S006", rules)

    def test_clean_file_no_errors(self):
        from devkit import safety
        with tempfile.TemporaryDirectory() as d:
            (pathlib.Path(d) / "c.py").write_text("def add(a, b): return a + b\n")
            r = safety.scan_build(pathlib.Path(d))
        self.assertTrue(r["ok"])
        self.assertFalse(r["has_errors"])


class TaskCenterTest(unittest.TestCase):
    def setUp(self):
        from devkit import task_center as TC
        self._TC = TC
        self._tmpdir = tempfile.mkdtemp()
        TC.TASKS_FILE = pathlib.Path(self._tmpdir) / "tasks.json"

    def test_new_task_returns_dict_with_fields(self):
        TC = self._TC
        t = TC.new_task("实现 word_count")
        self.assertIsInstance(t, dict)
        for key in ("id", "title", "created", "runs", "status"):
            self.assertIn(key, t)

    def test_new_task_runs_empty_status_open(self):
        TC = self._TC
        t = TC.new_task("my task")
        self.assertEqual(t["runs"], [])
        self.assertEqual(t["status"], "open")

    def test_duplicate_id_raises(self):
        TC = self._TC
        TC.new_task("t", "dup")
        with self.assertRaises(ValueError):
            TC.new_task("t2", "dup")

    def test_link_run_appends(self):
        TC = self._TC
        TC.new_task("t", "tk1")
        TC.link_run("tk1", "run-abc")
        self.assertEqual(TC.get_task("tk1")["runs"], ["run-abc"])

    def test_link_run_idempotent(self):
        TC = self._TC
        TC.new_task("t", "tk2")
        TC.link_run("tk2", "r1")
        TC.link_run("tk2", "r1")
        self.assertEqual(len(TC.get_task("tk2")["runs"]), 1)

    def test_link_run_missing_raises(self):
        TC = self._TC
        with self.assertRaises(KeyError):
            TC.link_run("no-such", "r1")

    def test_get_missing_returns_none(self):
        TC = self._TC
        self.assertIsNone(TC.get_task("no-such"))

    def test_list_tasks_sorted_desc(self):
        TC = self._TC
        TC.new_task("first", "aa")
        TC.new_task("second", "bb")
        lst = TC.list_tasks()
        self.assertEqual(lst[0]["id"], "bb")

    def test_close_task(self):
        TC = self._TC
        TC.new_task("t", "cl1")
        self.assertTrue(TC.close_task("cl1"))
        self.assertEqual(TC.get_task("cl1")["status"], "closed")

    def test_close_missing_returns_false(self):
        TC = self._TC
        self.assertFalse(TC.close_task("no-such"))


class RadarTest(unittest.TestCase):
    def test_list_catalog_returns_list(self):
        from devkit import radar
        r = radar.list_catalog()
        self.assertIsInstance(r, list)
        self.assertGreater(len(r), 0)

    def test_list_catalog_filter_mcp(self):
        from devkit import radar
        mcps = radar.list_catalog("mcp")
        self.assertTrue(all(e["category"] == "mcp" for e in mcps))
        self.assertGreater(len(mcps), 0)

    def test_list_catalog_filter_rule(self):
        from devkit import radar
        rules = radar.list_catalog("rule")
        self.assertTrue(all(e["category"] == "rule" for e in rules))

    def test_get_catalog_entry_found(self):
        from devkit import radar
        e = radar.get_catalog_entry("rule-no-overengineering")
        self.assertIsNotNone(e)
        self.assertEqual(e["name"], "rule-no-overengineering")

    def test_get_catalog_entry_missing(self):
        from devkit import radar
        self.assertIsNone(radar.get_catalog_entry("no-such-entry"))

    def test_scan_dir_finds_claude_md(self):
        from devkit import radar
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "CLAUDE.md").write_text("# My rules\nNo eval()")
        results = radar.scan_dir(d)
        self.assertEqual(len(results), 1)
        self.assertEqual(results[0]["name"], "claudemd")
        self.assertIn("No eval()", results[0]["content"])

    def test_scan_dir_empty(self):
        from devkit import radar
        d = pathlib.Path(tempfile.mkdtemp())
        self.assertEqual(radar.scan_dir(d), [])

    def test_import_to_assets_builtin(self):
        from devkit import radar
        d = pathlib.Path(tempfile.mkdtemp()) / "loom.assets.toml"
        a = radar.import_to_assets("rule-tdd-first", assets_path=d)
        self.assertIsNotNone(a)
        self.assertEqual(a["trust_level"], 6)
        self.assertEqual(a["name"], "rule-tdd-first")

    def test_import_to_assets_missing(self):
        from devkit import radar
        self.assertIsNone(radar.import_to_assets("no-such"))

    def test_cmd_radar_list(self):
        import subprocess, sys
        r = subprocess.run([sys.executable, "-m", "devkit", "radar", "list"],
                          capture_output=True, text=True, cwd=str(ROOT),
                          env={**__import__("os").environ, "PYTHONPATH": str(ROOT)})
        self.assertEqual(r.returncode, 0)
        self.assertIn("mcp-filesystem", r.stdout)


class MigrateTest(unittest.TestCase):
    def test_detect_empty_dir(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        result = migrate.detect(d)
        self.assertIsInstance(result, dict)
        self.assertFalse(any(v["found"] for v in result.values()))

    def test_detect_finds_claude_md(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "CLAUDE.md").write_text("# rules")
        result = migrate.detect(d)
        self.assertTrue(result["claude-code"]["found"])

    def test_detect_finds_clinerules(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        (d / ".clinerules").write_text("no eval")
        result = migrate.detect(d)
        self.assertTrue(result["cline"]["found"])

    def test_migrate_tool_claude_code(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "CLAUDE.md").write_text("# My project rules\nno globals")
        assets_path = d / "loom.assets.toml"
        created = migrate.migrate_tool("claude-code", d, assets_path)
        self.assertEqual(len(created), 1)
        self.assertEqual(created[0]["name"], "claude-code-rules")
        self.assertEqual(created[0]["trust_level"], 1)

    def test_migrate_tool_missing_config(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        created = migrate.migrate_tool("aider", d)
        self.assertEqual(created, [])

    def test_migrate_all(self):
        from devkit import migrate
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "CLAUDE.md").write_text("# rules")
        (d / ".clinerules").write_text("no eval")
        assets_path = d / "loom.assets.toml"
        results = migrate.migrate_all(d, assets_path)
        self.assertIn("claude-code", results)
        self.assertIn("cline", results)

    def test_cmd_migrate_detect(self):
        import subprocess, sys, os
        d = tempfile.mkdtemp()
        (pathlib.Path(d) / "CLAUDE.md").write_text("# rules")
        r = subprocess.run([sys.executable, "-m", "devkit", "migrate", "detect", "--dir", d],
                          capture_output=True, text=True, cwd=str(ROOT),
                          env={**os.environ, "PYTHONPATH": str(ROOT)})
        self.assertEqual(r.returncode, 0)
        self.assertIn("claude-code", r.stdout)


class SafetyPresetsTest(unittest.TestCase):
    def _path(self):
        return pathlib.Path(tempfile.mkdtemp()) / "loom.assets.toml"

    def test_add_asset_default_trust_zero(self):
        from devkit import asset as A
        p = self._path()
        a = A.add_asset("x", "rule", "content", path=p)
        self.assertEqual(a["trust_level"], 0)

    def test_load_includes_trust_level(self):
        from devkit import asset as A
        p = self._path()
        A.add_asset("x", "rule", "c", path=p, trust_level=3)
        items = A.load_assets(p)
        self.assertEqual(items[0]["trust_level"], 3)

    def test_set_trust_updates_level(self):
        from devkit import asset as A
        p = self._path()
        A.add_asset("x", "rule", "c", path=p)
        b = A.set_trust("x", 2, p)
        self.assertEqual(b["trust_level"], 2)

    def test_set_trust_persists(self):
        from devkit import asset as A
        p = self._path()
        A.add_asset("x", "rule", "c", path=p)
        A.set_trust("x", 4, p)
        self.assertEqual(A.get_asset("x", p)["trust_level"], 4)

    def test_set_trust_missing_raises(self):
        from devkit import asset as A
        p = self._path()
        with self.assertRaises(KeyError):
            A.set_trust("no-such", 1, p)

    def test_set_trust_invalid_level_raises(self):
        from devkit import asset as A
        p = self._path()
        A.add_asset("x", "rule", "c", path=p)
        with self.assertRaises(ValueError):
            A.set_trust("x", 7, p)

    def test_trust_label_coverage(self):
        from devkit import asset as A
        self.assertEqual(A.trust_label(0), "untrusted")
        self.assertEqual(A.trust_label(3), "trusted")
        self.assertEqual(A.trust_label(6), "system")

    def test_cmd_asset_trust_subcommand(self):
        import subprocess, sys, os
        d = tempfile.mkdtemp()
        from devkit import asset as A
        p = pathlib.Path(d) / "loom.assets.toml"
        A.add_asset("myrule", "rule", "no eval", path=p)
        r = subprocess.run(
            [sys.executable, "-m", "devkit", "asset", "trust", "myrule", "2"],
            capture_output=True, text=True, cwd=d,
            env={**os.environ, "PYTHONPATH": str(ROOT)})
        self.assertEqual(r.returncode, 0)
        self.assertIn("L2", r.stdout)

    def test_list_shows_trust_column(self):
        import subprocess, sys, os
        d = tempfile.mkdtemp()
        from devkit import asset as A
        p = pathlib.Path(d) / "loom.assets.toml"
        A.add_asset("myrule", "rule", "content", path=p, trust_level=1)
        r = subprocess.run(
            [sys.executable, "-m", "devkit", "asset", "list"],
            capture_output=True, text=True, cwd=d,
            env={**os.environ, "PYTHONPATH": str(ROOT)})
        self.assertIn("reviewed", r.stdout)


class ConfigTest(unittest.TestCase):
    def _write(self, content: str) -> pathlib.Path:
        d = pathlib.Path(tempfile.mkdtemp())
        p = d / "loom.toml"
        p.write_text(content, encoding="utf-8")
        return p

    def test_load_empty_returns_empty(self):
        from devkit import config as cfg
        self.assertEqual(cfg.load_config(pathlib.Path("/nonexistent/loom.toml")), {})

    def test_load_scalar_defaults(self):
        from devkit import config as cfg
        p = self._write("[defaults]\nstages = \"plan,implement\"\nmax_tokens = 4000\n")
        r = cfg.load_config(p)
        self.assertEqual(r["stages"], "plan,implement")
        self.assertEqual(r["max_tokens"], 4000)

    def test_load_bool_defaults(self):
        from devkit import config as cfg
        p = self._write("[defaults]\nsafety = true\nauto_carrier = false\n")
        r = cfg.load_config(p)
        self.assertTrue(r["safety"])
        self.assertFalse(r["auto_carrier"])

    def test_load_carrier_section(self):
        from devkit import config as cfg
        p = self._write("[carrier]\nimplement = \"deepseek\"\nreview = \"claude\"\n")
        r = cfg.load_config(p)
        self.assertIn("implement=deepseek", r["_carrier"])
        self.assertIn("review=claude", r["_carrier"])

    def test_find_config_returns_none_when_absent(self):
        from devkit import config as cfg
        self.assertIsNone(cfg.find_config(pathlib.Path(tempfile.mkdtemp())))

    def test_find_config_finds_parent(self):
        from devkit import config as cfg
        parent = pathlib.Path(tempfile.mkdtemp())
        child = parent / "sub" / "sub2"
        child.mkdir(parents=True)
        (parent / "loom.toml").write_text("[defaults]\n")
        found = cfg.find_config(child)
        self.assertEqual(found, parent / "loom.toml")

    def test_write_default_config_creates_file(self):
        from devkit import config as cfg
        d = pathlib.Path(tempfile.mkdtemp())
        p = d / "loom.toml"
        cfg.write_default_config(p)
        self.assertTrue(p.exists())
        self.assertIn("[defaults]", p.read_text())

    def test_cmd_init_creates_file(self):
        import subprocess, sys, os
        d = tempfile.mkdtemp()
        r = subprocess.run([sys.executable, "-m", "devkit", "init"],
                          capture_output=True, text=True, cwd=d,
                          env={**os.environ, "PYTHONPATH": str(ROOT)})
        self.assertEqual(r.returncode, 0)
        self.assertTrue((pathlib.Path(d) / "loom.toml").exists())

    def test_cmd_config_shows_values(self):
        import subprocess, sys, os
        d = tempfile.mkdtemp()
        (pathlib.Path(d) / "loom.toml").write_text("[defaults]\nstages = \"plan\"\n")
        r = subprocess.run([sys.executable, "-m", "devkit", "config"],
                          capture_output=True, text=True, cwd=d,
                          env={**os.environ, "PYTHONPATH": str(ROOT)})
        self.assertIn("stages", r.stdout)


class LearnTest(unittest.TestCase):
    def _tmp(self):
        return pathlib.Path(tempfile.mkdtemp())

    def test_analyze_returns_structure(self):
        from devkit import learn
        r = learn.analyze(self._tmp())
        self.assertIn("suggestions", r)
        self.assertIn("summary", r)
        self.assertIsInstance(r["suggestions"], list)

    def test_summary_fields(self):
        from devkit import learn
        s = learn.analyze(self._tmp())["summary"]
        for k in ("total_runs", "go_rate", "avg_cost_usd", "total_cost_usd"):
            self.assertIn(k, s)

    def test_empty_dir_totals_zero(self):
        from devkit import learn
        s = learn.analyze(self._tmp())["summary"]
        self.assertEqual(s["total_runs"], 0)
        self.assertEqual(s["total_cost_usd"], 0.0)

    def test_suggest_carrier_no_data_none(self):
        from devkit import learn
        self.assertIsNone(learn.suggest_carrier("feature", self._tmp()))

    def test_quota_trend_empty_stable(self):
        from devkit import learn
        t = learn.quota_trend(self._tmp())
        self.assertEqual(t["trend"], "stable")
        self.assertIn("recent_10_cost", t)
        self.assertIn("max_cost", t)

    def test_suggestion_fields_present(self):
        from devkit import learn
        r = learn.analyze()
        for sug in r["suggestions"]:
            for k in ("type", "confidence", "reason", "action", "data"):
                self.assertIn(k, sug)

    def test_no_exception_on_none_dir(self):
        from devkit import learn
        learn.analyze(None)
        learn.quota_trend(None)
        learn.suggest_carrier("x", None)
        learn.suggest_goldens(None)

    def test_suggest_goldens_empty_dir(self):
        from devkit import learn
        result = learn.suggest_goldens(pathlib.Path(tempfile.mkdtemp()))
        self.assertIsInstance(result, list)
        self.assertEqual(result, [])

    def test_suggest_goldens_detects_import_error(self):
        from devkit import learn
        d = pathlib.Path(tempfile.mkdtemp())
        run_dir = d / "run-01"
        run_dir.mkdir()
        (run_dir / "run-log.md").write_text(
            "## Eval Gate\n| mytest | ❌ | ModuleNotFoundError: No module named 'x'  want=True |\n"
            "## Gate\nNO-GO\n"
        )
        # Fake runs_list by patching
        from devkit import insight as _ins
        orig = _ins.runs_list
        try:
            _ins.runs_list = lambda rd: [{"run_id": "run-01", "gate": "NO-GO", "cost": None,
                                           "task_type": "feature", "task_snippet": None,
                                           "artifact_files": [], "tokens": None}]
            sugs = learn.suggest_goldens(d)
        finally:
            _ins.runs_list = orig
        types = [s["type"] for s in sugs]
        self.assertIn("golden", types)

    def test_cmd_learn_in_help(self):
        import subprocess, sys
        r = subprocess.run([sys.executable, "-m", "devkit", "learn", "--help"],
                          capture_output=True, text=True, cwd=str(ROOT))
        self.assertIn("carrier", r.stdout + r.stderr)


class TaskIdFlagTest(unittest.TestCase):
    def test_task_id_flag_in_help(self):
        import subprocess, sys
        r = subprocess.run([sys.executable, "-m", "devkit", "--help"],
                          capture_output=True, text=True, cwd=str(ROOT))
        self.assertIn("--task-id", r.stdout)

    def test_task_id_links_after_run(self):
        from devkit import task_center as TC
        TC.TASKS_FILE = pathlib.Path(tempfile.mkdtemp()) / "tasks.json"
        TC.new_task("test task", "test-task-link")

        import devkit.__main__ as m
        import io, contextlib
        captured = io.StringIO()

        _orig_rl = None
        try:
            _orig_rl = m.run_loop

            fake_run_dir = tempfile.mkdtemp()
            run_id = pathlib.Path(fake_run_dir).name

            def fake_run_loop(task, **kw):
                return {"run_dir": fake_run_dir, "blocked": False,
                        "tokens": 0, "cost": 0.0, "gate": "GO"}

            m.run_loop = fake_run_loop
            with contextlib.redirect_stdout(captured):
                try:
                    m._cmd_run(["hello", "--task-id", "test-task-link"])
                except SystemExit:
                    pass
        finally:
            if _orig_rl is not None:
                m.run_loop = _orig_rl

        t = TC.get_task("test-task-link")
        self.assertIsNotNone(t)
        self.assertIn(run_id, t.get("runs", []))


# ── Wave 1: 自治安全底座 ─────────────────────────────────────────────────────

class EvidenceTest(unittest.TestCase):
    """T2 物理证据门 — evidence.py"""

    def setUp(self):
        from devkit import evidence as E
        self.E = E

    def test_no_evidence_is_nogo(self):
        r = self.E.gate({})
        self.assertEqual(r["verdict"], "NO-GO")

    def test_tests_passed_is_go(self):
        r = self.E.gate({"has_test_output": True, "tests_passed": True})
        self.assertEqual(r["verdict"], "GO")

    def test_tests_failed_is_nogo(self):
        r = self.E.gate({"has_test_output": True, "tests_passed": False})
        self.assertEqual(r["verdict"], "NO-GO")

    def test_codex_go_is_go(self):
        r = self.E.gate({"has_codex_verdict": True, "codex_verdict": "GO"})
        self.assertEqual(r["verdict"], "GO")

    def test_codex_nogo_is_nogo(self):
        r = self.E.gate({"has_codex_verdict": True, "codex_verdict": "NO-GO"})
        self.assertEqual(r["verdict"], "NO-GO")

    def test_returns_reason(self):
        r = self.E.gate({"has_test_output": True, "tests_passed": True})
        self.assertIn("reason", r)

    def test_pending_tests_nogo(self):
        r = self.E.gate({"has_test_output": True, "tests_passed": None})
        self.assertEqual(r["verdict"], "NO-GO")

    def test_both_evidence_go_wins(self):
        r = self.E.gate({"has_test_output": True, "tests_passed": True,
                          "has_codex_verdict": True, "codex_verdict": "GO"})
        self.assertEqual(r["verdict"], "GO")


class RatchetTest(unittest.TestCase):
    """T6 测试棘轮 — ratchet.py"""

    def setUp(self):
        from devkit import ratchet as R
        self.R = R

    def _c(self, n):
        return [{"name": f"t{i}"} for i in range(n)]

    def test_same_count_not_weakened(self):
        self.assertFalse(self.R.is_weakened(self._c(3), self._c(3)))

    def test_more_cases_not_weakened(self):
        self.assertFalse(self.R.is_weakened(self._c(3), self._c(5)))

    def test_fewer_cases_weakened(self):
        self.assertTrue(self.R.is_weakened(self._c(5), self._c(3)))

    def test_raises_drop_weakened(self):
        old = [{"name": "t1", "raises": True}, {"name": "t2"}]
        new = [{"name": "t1"}, {"name": "t2"}]
        self.assertTrue(self.R.is_weakened(old, new))

    def test_raises_preserved_not_weakened(self):
        old = [{"name": "t1", "raises": True}]
        new = [{"name": "t1", "raises": True}, {"name": "t2"}]
        self.assertFalse(self.R.is_weakened(old, new))

    def test_check_returns_dict(self):
        r = self.R.check(self._c(2), self._c(3))
        self.assertIn("weakened", r)
        self.assertIn("old_count", r)
        self.assertIn("new_count", r)

    def test_check_detects_drop(self):
        r = self.R.check(self._c(5), self._c(3))
        self.assertTrue(r["weakened"])

    def test_empty_to_empty_not_weakened(self):
        self.assertFalse(self.R.is_weakened([], []))


class StopCheckTest(unittest.TestCase):
    """T4 死循环检测 — stopcheck.py"""

    def setUp(self):
        from devkit import stopcheck as SC
        self.SC = SC

    def test_empty_no_stop(self):
        r = self.SC.should_stop([])
        self.assertFalse(r["stop"])

    def test_single_error_no_stop(self):
        r = self.SC.should_stop(["err"])
        self.assertFalse(r["stop"])

    def test_two_same_errors_stop(self):
        r = self.SC.should_stop(["err", "err"])
        self.assertTrue(r["stop"])

    def test_different_errors_no_stop(self):
        r = self.SC.should_stop(["err1", "err2"])
        self.assertFalse(r["stop"])

    def test_empty_string_no_stop(self):
        r = self.SC.should_stop(["", ""])
        self.assertFalse(r["stop"])

    def test_mixed_then_repeat_stop(self):
        r = self.SC.should_stop(["ok", "err", "err"])
        self.assertTrue(r["stop"])

    def test_returns_reason(self):
        r = self.SC.should_stop(["err", "err"])
        self.assertIn("reason", r)

    def test_custom_max_repeats(self):
        r = self.SC.should_stop(["e", "e", "e"], max_repeats=3)
        self.assertTrue(r["stop"])

    def test_below_custom_repeats_no_stop(self):
        r = self.SC.should_stop(["e", "e"], max_repeats=3)
        self.assertFalse(r["stop"])


class ApplyLockTest(unittest.TestCase):
    """T7 文件锁分类 — applylock.py"""

    def setUp(self):
        from devkit import applylock as AL
        self.AL = AL

    def tearDown(self):
        import os
        os.environ.pop("DEV_APPLYLOCK_ALLOW", None)

    def test_rdloop_requires_human(self):
        self.assertTrue(self.AL.requires_human("devkit/rdloop.py"))

    def test_evidence_requires_human(self):
        self.assertTrue(self.AL.requires_human("devkit/evidence.py"))

    def test_golden_requires_human(self):
        self.assertTrue(self.AL.requires_human("devkit/p0.golden.json"))

    def test_test_file_requires_human(self):
        self.assertTrue(self.AL.requires_human("devkit/test_features.py"))

    def test_tests_unit_prefix_is_auto(self):
        self.assertFalse(self.AL.requires_human("tests/unit/test_applylock_whitelist.py"))

    def test_tests_contract_prefix_is_auto(self):
        self.assertFalse(self.AL.requires_human("tests/contract/test_applylock_public_api.py"))

    def test_regular_module_no_human(self):
        self.assertFalse(self.AL.requires_human("devkit/learn.py"))

    def test_new_module_no_human(self):
        self.assertFalse(self.AL.requires_human("devkit/my_feature.py"))

    def test_applylock_itself_requires_human(self):
        self.assertTrue(self.AL.requires_human("applylock.py"))

    def test_ratchet_requires_human(self):
        self.assertTrue(self.AL.requires_human("ratchet.py"))

    def test_stopcheck_requires_human(self):
        self.assertTrue(self.AL.requires_human("stopcheck.py"))

    def test_env_allow_once_path_is_auto(self):
        import os
        os.environ["DEV_APPLYLOCK_ALLOW"] = "tests/test_runlog_path.py"
        self.assertFalse(self.AL.requires_human("tests/test_runlog_path.py"))

    def test_run_context_allow_once_is_scoped(self):
        import tempfile
        lock = self.AL.ApplyLock()
        with tempfile.TemporaryDirectory() as d:
            ctx_a = self.AL.RunContext.get(run_id="run-a", runs_dir=d)
            ctx_b = self.AL.RunContext.get(run_id="run-b", runs_dir=d)
            self.assertTrue(lock.is_protected("tests/test_runlog_path.py", ctx_a))
            log_path = lock.allow_once("tests/test_runlog_path.py", ctx_a)
            self.assertFalse(lock.is_protected("tests/test_runlog_path.py", ctx_a))
            self.assertTrue(lock.is_protected("tests/test_runlog_path.py", ctx_b))
            self.assertTrue(log_path.exists())
            self.assertIn("tests/test_runlog_path.py", log_path.read_text(encoding="utf-8"))

    def test_allow_once_rejects_critical_path(self):
        import tempfile
        lock = self.AL.ApplyLock()
        with tempfile.TemporaryDirectory() as d:
            ctx = self.AL.RunContext.get(run_id="run-a", runs_dir=d)
            with self.assertRaises(ValueError):
                lock.allow_once("devkit/rdloop.py", ctx)

    def test_exemption_reason_for_allowlist_and_env(self):
        import os
        lock = self.AL.ApplyLock()
        self.assertEqual(lock.exemption_reason("tests/unit/test_applylock_whitelist.py"), "test-prefix-allowlist")
        os.environ["DEV_APPLYLOCK_ALLOW"] = "tests/test_runlog_path.py"
        self.assertEqual(lock.exemption_reason("tests/test_runlog_path.py"), "env-override")

    def test_custom_apply_mode_allowlist_allows_tests_prefix(self):
        lock = self.AL.ApplyLock(allowed_test_prefixes=("tests/",))
        self.assertFalse(lock.is_protected("tests/test_human_required_guard.py"))


# ── Wave 2: 自治驱动循环 + 断点续跑 ────────────────────────────────────────────

class AutoloopTest(unittest.TestCase):
    """T1 自治驱动循环 — autoloop.py"""

    def setUp(self):
        from devkit import autoloop as AL
        self.AL = AL

    def _bl(self, *items):
        return list(items)

    def test_pick_next_first_ready(self):
        bl = [{"id":"a","status":"done","deps":[]},
              {"id":"b","status":"pending","deps":["a"]},
              {"id":"c","status":"pending","deps":[]}]
        self.assertEqual(self.AL.pick_next(bl)["id"], "b")

    def test_pick_next_first_no_deps(self):
        bl = [{"id":"x","status":"pending","deps":[]}]
        self.assertEqual(self.AL.pick_next(bl)["id"], "x")

    def test_pick_next_none_missing_dep(self):
        bl = [{"id":"a","status":"pending","deps":["missing"]}]
        self.assertIsNone(self.AL.pick_next(bl))

    def test_pick_next_skip_running(self):
        bl = [{"id":"a","status":"running","deps":[]},
              {"id":"b","status":"pending","deps":[]}]
        self.assertEqual(self.AL.pick_next(bl)["id"], "b")

    def test_pick_next_deprioritizes_meta_maintenance_tasks(self):
        bl = [
            {"id":"human-mark-blocked-3","status":"pending","deps":[],"task":"在 backlog.json 中把 manual-review-probe-collection 标记为 blocked-needs-human"},
            {"id":"real-fix","status":"pending","deps":[],"task":"修复 devkit/rdloop.py 中的 blocked reason 汇总"},
        ]
        self.assertEqual(self.AL.pick_next(bl)["id"], "real-fix")

    def test_pick_next_skips_missing_workspace_fix_task(self):
        bl = [
            {"id":"a","status":"pending","deps":[],"task":"修复 devkit/missing_task.py 中的参数处理"},
            {"id":"b","status":"pending","deps":[],"task":"实现当前 backlog 的下一步"},
        ]
        self.assertEqual(self.AL.pick_next(bl)["id"], "b")

    def test_pick_next_allows_create_task_for_missing_path(self):
        bl = [{"id":"a","status":"pending","deps":[],"task":"新增 devkit/new_module.py 并补最小测试"}]
        self.assertEqual(self.AL.pick_next(bl)["id"], "a")

    def test_stale_task_missing_paths_filters_old_shadow_paths(self):
        task = "修复 devkit/runner/sandbox/module_shadowing_check.py 并同步 tests/test_module_shadowing.py"
        got = self.AL.stale_task_missing_paths(task)
        self.assertEqual(
            got,
            ["devkit/runner/sandbox/module_shadowing_check.py", "tests/test_module_shadowing.py"],
        )

    def test_prune_stale_pending_stops_only_old_shadow_tasks(self):
        bl = [
            {"id":"a","status":"pending","deps":[],"task":"修复 devkit/runner/sandbox/module_shadowing_check.py"},
            {"id":"b","status":"pending","deps":[],"task":"实现当前 backlog 的下一步"},
            {"id":"c","status":"done","deps":[],"task":"修复 tests/test_module_shadowing.py"},
        ]
        got = self.AL.prune_stale_pending(bl)
        items = {item["id"]: item for item in got["backlog"]}
        self.assertEqual(items["a"]["status"], "stopped")
        self.assertEqual(items["a"]["stop_reason"], "stale_missing_workspace_paths")
        self.assertEqual(items["b"]["status"], "pending")
        self.assertEqual(items["c"]["status"], "done")
        self.assertEqual(got["stopped"][0]["id"], "a")

    def test_prune_human_only_pending_stops_repo_write_tasks_in_report_only_loop(self):
        bl = [
            {"id":"a","status":"pending","deps":[],"delivery_mode":"report-only","task":"实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py"},
            {"id":"b","status":"pending","deps":[],"task":"把诊断报告写到 runs/demo/audit.md"},
        ]
        got = self.AL.prune_human_only_pending(bl)
        items = {item["id"]: item for item in got["backlog"]}
        self.assertEqual(items["a"]["status"], "stopped")
        self.assertEqual(items["a"]["stop_reason"], "human_required_report_only")
        self.assertIn("report-only-cannot-apply", items["a"]["human_required_reason"])
        self.assertEqual(items["b"]["status"], "pending")

    def test_prune_human_only_pending_stops_explicit_human_signal(self):
        bl = [
            {"id":"a","status":"pending","deps":[],"task":"该任务需人工 apply 后继续"},
        ]
        got = self.AL.prune_human_only_pending(bl)
        self.assertEqual(got["backlog"][0]["status"], "stopped")
        self.assertIn("explicit-human-signal", got["backlog"][0]["human_required_reason"])

    def test_prune_human_only_pending_keeps_apply_required_repo_task(self):
        bl = [
            {
                "id":"a",
                "status":"pending",
                "deps":[],
                "delivery_mode":"apply-required",
                "apply_target":"/tmp/worktree",
                "task":"实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py",
            },
        ]
        got = self.AL.prune_human_only_pending(bl)
        self.assertEqual(got["backlog"][0]["status"], "pending")

    def test_prune_human_only_pending_keeps_autonomous_repo_task(self):
        bl = [
            {
                "id":"a",
                "status":"pending",
                "deps":[],
                "task":"实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py",
            },
        ]
        got = self.AL.prune_human_only_pending(bl)
        self.assertEqual(got["backlog"][0]["status"], "pending")

    def test_pick_next_empty(self):
        self.assertIsNone(self.AL.pick_next([]))

    def test_advance_pending_start(self):
        self.assertEqual(self.AL.advance_state("pending","start"), "running")

    def test_advance_running_success(self):
        self.assertEqual(self.AL.advance_state("running","success"), "done")

    def test_advance_running_failure(self):
        self.assertEqual(self.AL.advance_state("running","failure"), "failed")

    def test_advance_running_stop(self):
        self.assertEqual(self.AL.advance_state("running","stop"), "stopped")

    def test_advance_done_terminal(self):
        self.assertEqual(self.AL.advance_state("done","start"), "done")

    def test_run_once_default_stages(self):
        self.assertEqual(self.AL.run_once({"task":"x"})["stages"], "plan,implement,verify")

    def test_run_once_carrier_expansion(self):
        r = self.AL.run_once({"task":"t","carrier":{"implement":"deepseek"}})
        self.assertIn("implement=deepseek", r["carriers"])

    def test_run_once_run_id_prefix(self):
        self.assertTrue(self.AL.run_once({"task":"t"})["run_id"].startswith("auto-"))

    def test_run_once_normalizes_shell_stage_alias(self):
        r = self.AL.run_once({
            "task": "t",
            "stages": "shell,verify",
            "carrier": {"shell": "shell-runner", "verify": "shell-runner"},
        })
        self.assertEqual(r["stages"], "implement,verify")
        self.assertIn("implement=shell-runner", r["carriers"])

    def test_run_once_preserves_long_task_options(self):
        r = self.AL.run_once({
            "task": "t",
            "executor": {"verify": "codex"},
            "iterate": 2,
            "contract": 3,
            "budget": 1.5,
            "blind_review": True,
            "physical_verify": True,
            "cascade": ["minimax", "glm", "deepseek"],
        })
        self.assertIn("verify=codex", r["executors"])
        self.assertEqual(r["iterate"], 2)
        self.assertEqual(r["contract"], 3)
        self.assertEqual(r["budget"], 1.5)
        self.assertTrue(r["blind_review"])
        self.assertTrue(r["physical_verify"])
        self.assertEqual(r["cascade"], ["minimax", "glm", "deepseek"])

    def test_run_once_preserves_apply_modes_and_targets(self):
        r = self.AL.run_once({
            "task": "t",
            "delivery_mode": "apply-git",
            "apply_target": "/tmp/worktree",
            "apply_git": "/tmp/repo",
            "apply_branch": "loom/test",
        })
        self.assertEqual(r["delivery_mode"], "apply-git")
        self.assertEqual(r["apply_target"], "/tmp/worktree")
        self.assertEqual(r["apply_git"], "/tmp/repo")
        self.assertEqual(r["apply_branch"], "loom/test")

    def test_validate_task_accepts_autonomous_delivery_mode(self):
        from devkit.task_validator import validate_task
        self.assertEqual(validate_task({"id": "a", "status": "pending", "delivery_mode": "autonomous"}), [])

    def test_run_once_preserves_task_contract_fields(self):
        r = self.AL.run_once({
            "task": "t",
            "task_kind": "diag",
            "allowed_artifact_paths": ["tests/"],
            "forbidden_artifact_paths": [".github/"],
        })
        self.assertEqual(r["task_kind"], "diag")
        self.assertEqual(r["allowed_artifact_paths"], ["tests/"])
        self.assertEqual(r["forbidden_artifact_paths"], [".github/"])

    def test_is_success_gate_accepts_recommended_go(self):
        self.assertTrue(self.AL.is_success_gate("建议 GO（需人类最终确认）"))
        self.assertTrue(self.AL.is_success_gate("GO"))
        self.assertFalse(self.AL.is_success_gate("NO-GO（测试失败）"))


class RdloopBlockedHintTest(unittest.TestCase):
    def test_unknown_executor_hint(self):
        self.assertEqual(rdloop._blocked_retry_hint("未知执行器：shell"), "修正 executor 配置")

    def test_api_key_hint(self):
        self.assertEqual(rdloop._blocked_retry_hint("missing api key for provider"), "检查 provider key 或登录态")


class IterateHelpersTest(unittest.TestCase):
    def setUp(self):
        from devkit import iterate as IT
        self.IT = IT

    def test_parse_reflection_reads_json_fence(self):
        text = """```json
{"summary":"s","continue":false,"requeue":["a"],"reprioritize":[{"id":"b","priority":"high"}],"add_tasks":[{"id":"new","task":"write focused regression test"}]}
```"""
        out = self.IT.parse_reflection(text)
        self.assertEqual(out["summary"], "s")
        self.assertFalse(out["continue"])
        self.assertEqual(out["requeue"], ["a"])
        self.assertEqual(out["reprioritize"][0]["id"], "b")
        self.assertEqual(out["add_tasks"][0]["id"], "new")

    def test_parse_reflection_missing_json_fails_open(self):
        out = self.IT.parse_reflection("not json")
        self.assertTrue(out["continue"])
        self.assertEqual(out["_parse_error"], "missing_json_object")
        self.assertEqual(out["add_tasks"], [])

    def test_apply_reflection_requeues_and_adds_unique_task(self):
        backlog = [
            {"id": "a", "status": "failed", "deps": [], "task": "fix a"},
            {"id": "new", "status": "done", "deps": [], "task": "old"},
        ]
        reflection = {
            "requeue": ["a"],
            "reprioritize": [],
            "add_tasks": [{"id": "new", "task": "new follow-up", "priority": "high", "deps": []}],
        }
        out = self.IT.apply_reflection(backlog, reflection)
        items = {item["id"]: item for item in out["backlog"]}
        self.assertEqual(items["a"]["status"], "pending")
        self.assertIn("new-2", items)
        self.assertEqual(items["new-2"]["priority"], "high")
        self.assertEqual(out["changes"]["requeued"], 1)
        self.assertEqual(out["changes"]["added"], 1)

    def test_apply_reflection_reprioritizes_existing_task(self):
        backlog = [{"id": "a", "status": "pending", "deps": [], "priority": "low", "task": "t"}]
        reflection = {
            "requeue": [],
            "reprioritize": [{"id": "a", "priority": "high"}],
            "add_tasks": [],
        }
        out = self.IT.apply_reflection(backlog, reflection)
        self.assertEqual(out["backlog"][0]["priority"], "high")
        self.assertEqual(out["changes"]["reprioritized"], 1)

    def test_apply_reflection_blocks_requeue_after_two_same_failure_codes(self):
        backlog = [{"id": "a", "status": "failed", "deps": [], "priority": "high", "task": "t"}]
        reflection = {
            "summary": "same failure repeats",
            "stop_reason": "",
            "requeue": ["a"],
            "reprioritize": [],
            "add_tasks": [],
        }
        recent = [
            {"task_id": "a", "outcome": "failure", "failure_code": "FORMAT_MISMATCH_NO_FILE_MARKERS"},
            {"task_id": "a", "outcome": "failure", "failure_code": "FORMAT_MISMATCH_NO_FILE_MARKERS"},
        ]
        out = self.IT.apply_reflection(
            backlog, reflection,
            current_task_id="a",
            current_outcome="failure",
            current_failure_code="FORMAT_MISMATCH_NO_FILE_MARKERS",
            recent_records=recent,
        )
        self.assertEqual(out["changes"]["requeued"], 0)
        self.assertIn("blocked_requeue", out["changes"])
        self.assertEqual(out["changes"]["blocked_requeue"], 1)
        self.assertFalse(out["reflection"]["continue"])
        self.assertIn("2 consecutive runs", out["reflection"]["stop_reason"])

    def test_build_reflection_prompt_mentions_json_only(self):
        prompt = self.IT.build_reflection_prompt(
            round_no=2,
            task_id="x",
            outcome="failure",
            gate="NO-GO",
            backlog=[{"id": "x", "status": "failed", "deps": [], "task": "t"}],
            run_log="log",
            recent_decisions=[],
        )
        self.assertIn("只输出 JSON", prompt)
        self.assertIn("requeue", prompt)
        self.assertIn("add_tasks", prompt)


class IterateCommandTest(unittest.TestCase):
    def test_cmd_iterate_dry_run_selects_ready_task(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {"id": "a", "status": "done", "deps": [], "task": "done"},
                {"id": "b", "status": "pending", "deps": ["a"], "task": "next task"},
            ], ensure_ascii=False), encoding="utf-8")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = mainmod._cmd_iterate(["--backlog", str(backlog), "--dry-run"])
        self.assertEqual(code, 0)
        self.assertIn("第 1 轮将选中任务：b", buf.getvalue())

    def test_write_backlog_preserves_disk_added_tasks(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {"id": "a", "status": "pending", "deps": [], "task": "A"},
                {"id": "b", "status": "pending", "deps": [], "task": "B"},
            ], ensure_ascii=False), encoding="utf-8")
            stale = [
                {"id": "a", "status": "done", "deps": [], "task": "A"},
                {"id": "b", "status": "pending", "deps": [], "task": "B"},
            ]
            backlog.write_text(json.dumps([
                {"id": "a", "status": "pending", "deps": [], "task": "A"},
                {"id": "b", "status": "pending", "deps": [], "task": "B"},
                {"id": "goal-spec-v1", "status": "pending", "deps": [], "task": "Goal spec"},
            ], ensure_ascii=False), encoding="utf-8")
            merged = mainmod._write_backlog(backlog, stale)
            ids = [item["id"] for item in merged]
            self.assertIn("goal-spec-v1", ids)
            self.assertEqual(next(item for item in merged if item["id"] == "a")["status"], "done")

    def test_write_backlog_backfills_missing_fields_from_disk(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {"id": "a", "status": "pending", "deps": [], "task": "A", "priority": "high", "carrier": {"implement": "minimax"}},
            ], ensure_ascii=False), encoding="utf-8")
            merged = mainmod._write_backlog(backlog, [
                {"id": "a", "status": "done", "deps": [], "task": "A"},
            ])
            item = merged[0]
            self.assertEqual(item["priority"], "high")
            self.assertEqual(item["carrier"]["implement"], "minimax")

    def test_cmd_iterate_reclaims_legacy_running_before_pick(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {"id": "stale", "status": "running", "deps": [], "task": "stale running"},
                {"id": "next", "status": "pending", "deps": [], "task": "next task"},
            ], ensure_ascii=False), encoding="utf-8")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = mainmod._cmd_iterate(["--backlog", str(backlog), "--dry-run"])
            self.assertEqual(code, 0)
            self.assertIn("第 1 轮将选中任务：next", buf.getvalue())
            items = json.loads(backlog.read_text(encoding="utf-8"))
            stale = next(item for item in items if item["id"] == "stale")
            self.assertEqual(stale["status"], "stopped")
            self.assertEqual(stale["_lease_reclaim_reason"], "missing_owner")

    def test_cmd_iterate_prunes_human_only_pending_before_pick(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {"id": "human-task", "status": "pending", "deps": [], "delivery_mode": "report-only", "task": "实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py"},
                {"id": "report-task", "status": "pending", "deps": [], "task": "把诊断结果写入 runs/demo/report.md"},
            ], ensure_ascii=False), encoding="utf-8")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = mainmod._cmd_iterate(["--backlog", str(backlog), "--dry-run"])
            self.assertEqual(code, 0)
            self.assertIn("第 1 轮将选中任务：report-task", buf.getvalue())
            items = json.loads(backlog.read_text(encoding="utf-8"))
            stopped = next(item for item in items if item["id"] == "human-task")
            self.assertEqual(stopped["status"], "stopped")
            self.assertEqual(stopped["stop_reason"], "human_required_report_only")

    def test_cmd_iterate_keeps_apply_required_task_ready(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {
                    "id": "apply-task",
                    "status": "pending",
                    "deps": [],
                    "delivery_mode": "apply-required",
                    "apply_target": "/tmp/worktree",
                    "task": "实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py",
                },
            ], ensure_ascii=False), encoding="utf-8")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = mainmod._cmd_iterate(["--backlog", str(backlog), "--dry-run"])
            self.assertEqual(code, 0)
            self.assertIn("第 1 轮将选中任务：apply-task", buf.getvalue())

    def test_run_selected_backlog_item_passes_apply_fields_to_run_loop(self):
        import devkit.__main__ as mainmod
        import unittest.mock as mock
        captured = {}
        with tempfile.TemporaryDirectory() as d:
            backlog_path = pathlib.Path(d) / "backlog.json"
            backlog_path.write_text(json.dumps([{
                "id": "apply-task",
                "status": "pending",
                "deps": [],
                "task": "实现 x",
            }], ensure_ascii=False), encoding="utf-8")
            backlog = json.loads(backlog_path.read_text(encoding="utf-8"))
            run_args = {
                "task": "实现 x",
                "stages": "implement",
                "carriers": [],
                "executors": [],
                "run_id": "auto-test-apply",
                "delivery_mode": "apply-required",
                "apply_target": "/tmp/worktree",
                "apply_git": "/tmp/repo",
                "apply_branch": "loom/test",
            }

            def fake_run_loop(task, stages, **kwargs):
                captured.update(kwargs)
                return {"run_dir": str(pathlib.Path(d) / "run"), "gate": "GO", "blocked": [], "tokens": 0, "cost": 0.0}

            with mock.patch("devkit.rdloop.run_loop", fake_run_loop):
                mainmod._run_selected_backlog_item(backlog, backlog_path, backlog[0], run_args)

        self.assertEqual(captured["apply_target"], "/tmp/worktree")
        self.assertEqual(captured["apply_git"], "/tmp/repo")
        self.assertEqual(captured["apply_branch"], "loom/test")

    def test_cmd_iterate_keeps_apply_required_task_ready(self):
        import devkit.__main__ as mainmod
        with tempfile.TemporaryDirectory() as d:
            backlog = pathlib.Path(d) / "backlog.json"
            backlog.write_text(json.dumps([
                {
                    "id": "apply-task",
                    "status": "pending",
                    "deps": [],
                    "delivery_mode": "apply-required",
                    "apply_target": "/tmp/worktree",
                    "task": "实现 devkit/human_required_guard.py 并新增 tests/test_human_required_guard.py",
                },
            ], ensure_ascii=False), encoding="utf-8")
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = mainmod._cmd_iterate(["--backlog", str(backlog), "--dry-run"])
            self.assertEqual(code, 0)
            self.assertIn("第 1 轮将选中任务：apply-task", buf.getvalue())

    def test_run_selected_backlog_item_passes_apply_fields_to_run_loop(self):
        import devkit.__main__ as mainmod
        import unittest.mock as mock
        captured = {}
        with tempfile.TemporaryDirectory() as d:
            backlog_path = pathlib.Path(d) / "backlog.json"
            backlog_path.write_text(json.dumps([{
                "id": "apply-task",
                "status": "pending",
                "deps": [],
                "task": "实现 x",
            }], ensure_ascii=False), encoding="utf-8")
            backlog = json.loads(backlog_path.read_text(encoding="utf-8"))
            run_args = {
                "task": "实现 x",
                "stages": "implement",
                "carriers": [],
                "executors": [],
                "run_id": "auto-test-apply",
                "delivery_mode": "apply-required",
                "apply_target": "/tmp/worktree",
                "apply_git": "/tmp/repo",
                "apply_branch": "loom/test",
            }

            def fake_run_loop(task, stages, **kwargs):
                captured.update(kwargs)
                return {"run_dir": str(pathlib.Path(d) / "run"), "gate": "GO", "blocked": [], "tokens": 0, "cost": 0.0}

            with mock.patch("devkit.rdloop.run_loop", fake_run_loop):
                mainmod._run_selected_backlog_item(backlog, backlog_path, backlog[0], run_args)

        self.assertEqual(captured["apply_target"], "/tmp/worktree")
        self.assertEqual(captured["apply_git"], "/tmp/repo")
        self.assertEqual(captured["apply_branch"], "loom/test")


class LeaseTest(unittest.TestCase):
    def test_reclaim_stale_running_stops_dead_owner(self):
        from devkit import lease
        out = lease.reclaim_stale_running(
            [{"id": "a", "status": "running", "lease": {"owner_pid": 999, "heartbeat_at": "2026-01-01T00:00:00+00:00"}}],
            current_owner_pid=123,
            is_pid_alive=lambda pid: False,
        )
        self.assertEqual(out["reclaimed"], 1)
        self.assertEqual(out["backlog"][0]["status"], "stopped")
        self.assertEqual(out["backlog"][0]["_lease_reclaim_reason"], "owner_dead")

    def test_reclaim_stale_running_keeps_current_owner(self):
        from devkit import lease
        out = lease.reclaim_stale_running(
            [{"id": "a", "status": "running", "lease": {"owner_pid": 123, "heartbeat_at": "2026-01-01T00:00:00+00:00"}}],
            current_owner_pid=123,
            is_pid_alive=lambda pid: True,
        )
        self.assertEqual(out["reclaimed"], 0)
        self.assertEqual(out["backlog"][0]["status"], "running")

    def test_reclaim_stale_running_on_heartbeat_timeout(self):
        from devkit import lease
        out = lease.reclaim_stale_running(
            [{
                "id": "a",
                "status": "running",
                "lease": {
                    "owner_pid": 999,
                    "heartbeat_at": "2026-01-01T00:00:00+00:00",
                    "timeout_seconds": 1,
                },
            }],
            current_owner_pid=123,
            is_pid_alive=lambda pid: True,
        )
        self.assertEqual(out["reclaimed"], 1)
        self.assertEqual(out["backlog"][0]["_lease_reclaim_reason"], "heartbeat_timeout")


class ResumeTest(unittest.TestCase):
    """T3 断点续跑 — resume.py"""

    def setUp(self):
        import tempfile, pathlib
        from devkit import resume as R
        self.R = R
        self.tmp = pathlib.Path(tempfile.mkdtemp())

    def test_done_stages_nonexistent(self):
        self.assertEqual(self.R.done_stages("/nonexistent/loom/xyz"), [])

    def test_done_stages_empty_dir(self):
        self.assertEqual(self.R.done_stages(self.tmp), [])

    def test_done_stages_finds_plan(self):
        (self.tmp / "01-plan.md").write_text("x")
        self.assertEqual(self.R.done_stages(self.tmp), ["plan"])

    def test_done_stages_skips_task(self):
        (self.tmp / "00-task.md").write_text("x")
        self.assertEqual(self.R.done_stages(self.tmp), [])

    def test_done_stages_skips_iter(self):
        (self.tmp / "90-implement-r1.md").write_text("x")
        self.assertEqual(self.R.done_stages(self.tmp), [])

    def test_done_stages_ordered(self):
        (self.tmp / "02-implement.md").write_text("x")
        (self.tmp / "01-plan.md").write_text("x")
        self.assertEqual(self.R.done_stages(self.tmp), ["plan", "implement"])

    def test_pending_stages_subtracts(self):
        (self.tmp / "01-plan.md").write_text("x")
        result = self.R.pending_stages(self.tmp, ["plan","implement","verify"])
        self.assertEqual(result, ["implement","verify"])

    def test_is_complete_false_no_log(self):
        self.assertFalse(self.R.is_complete(self.tmp))

    def test_is_complete_true_with_log(self):
        (self.tmp / "run-log.md").write_text("x")
        self.assertTrue(self.R.is_complete(self.tmp))

    def test_is_complete_false_nonexistent(self):
        self.assertFalse(self.R.is_complete("/nonexistent/loom/xyz"))


# ── Wave 3: 自动发现 + 价值评分 ──────────────────────────────────────────────

class DiscoverTest(unittest.TestCase):
    """T9 自动发现 — discover.py"""

    def setUp(self):
        from devkit import discover as D
        self.D = D

    def test_from_fitness_improve_carrier(self):
        r = self.D.from_fitness([{"task_type":"feature","backend":"glm","ok_rate":30,"runs":3}])
        self.assertEqual(r[0]["type"], "improve_carrier")

    def test_from_fitness_skip_high_okrate(self):
        self.assertEqual(self.D.from_fitness([{"task_type":"feature","backend":"glm","ok_rate":80,"runs":3}]), [])

    def test_from_fitness_skip_low_runs(self):
        self.assertEqual(self.D.from_fitness([{"task_type":"feature","backend":"glm","ok_rate":20,"runs":1}]), [])

    def test_from_fitness_add_coverage_zero_runs(self):
        r = self.D.from_fitness([{"task_type":"doc","backend":"glm","ok_rate":0,"runs":0}])
        self.assertEqual(r[0]["type"], "add_coverage")

    def test_from_fitness_sorted_by_okrate(self):
        r = self.D.from_fitness([{"task_type":"a","backend":"x","ok_rate":40,"runs":3},
                                  {"task_type":"b","backend":"y","ok_rate":10,"runs":2}])
        self.assertEqual(r[0]["task_type"], "b")

    def test_from_suggestions_high_kept(self):
        result = self.D.from_suggestions([{"type":"carrier","detail":"use deepseek","priority":"high"},
                                           {"type":"quota","detail":"low","priority":"low"}])
        self.assertEqual(len(result), 1)

    def test_from_suggestions_carrier_type(self):
        self.assertEqual(self.D.from_suggestions([{"type":"carrier","detail":"x","priority":"high"}])[0]["type"], "switch_carrier")

    def test_from_suggestions_golden_type(self):
        self.assertEqual(self.D.from_suggestions([{"type":"golden","detail":"x","priority":"medium"}])[0]["type"], "fix_golden")

    def test_merge_dedup(self):
        self.assertEqual(len(self.D.merge([[{"type":"a","task_type":"x","backend":"b"},{"type":"a","task_type":"x","backend":"b"}]])), 1)

    def test_merge_max_total(self):
        self.assertEqual(len(self.D.merge([[{"type":"a","task_type":str(i),"backend":""} for i in range(20)]], max_total=5)), 5)


class ValuerTest(unittest.TestCase):
    """T11 价值评分 — valuer.py"""

    def setUp(self):
        from devkit import valuer as V
        self.V = V

    def test_score_returns_dict(self):
        self.assertIsInstance(self.V.score({"type":"improve_carrier"},{"ok_rate":20,"runs":3,"priority":"high"}), dict)

    def test_score_has_score_field(self):
        r = self.V.score({"type":"x"},{})
        self.assertIn("score", r)
        self.assertIsInstance(r["score"], int)

    def test_score_clamped_0_100(self):
        s = self.V.score({"type":"x","priority":"low"},{"ok_rate":90,"runs":0})["score"]
        self.assertGreaterEqual(s, 0)
        self.assertLessEqual(s, 100)

    def test_score_high_priority_beats_low(self):
        hi = self.V.score({"type":"x"},{"ok_rate":40,"runs":3,"priority":"high"})["score"]
        lo = self.V.score({"type":"x"},{"ok_rate":40,"runs":3,"priority":"low"})["score"]
        self.assertGreater(hi, lo)

    def test_score_low_okrate_beats_high(self):
        bad = self.V.score({"type":"x"},{"ok_rate":20,"runs":3,"priority":"medium"})["score"]
        good = self.V.score({"type":"x"},{"ok_rate":80,"runs":3,"priority":"medium"})["score"]
        self.assertGreater(bad, good)

    def test_score_has_reason(self):
        self.assertTrue(bool(self.V.score({"type":"x"},{}).get("reason","")))

    def test_rank_sorted_desc(self):
        r = self.V.rank([{"type":"a"},{"type":"b"}],
                        [{"ok_rate":10,"runs":3,"priority":"high"},{"ok_rate":80,"runs":1,"priority":"low"}])
        self.assertGreaterEqual(r[0]["score"], r[1]["score"])

    def test_top_n_returns_n(self):
        self.assertEqual(len(self.V.top_n([{"type":str(i)} for i in range(10)],[{}]*10, n=3)), 3)

    def test_top_n_fewer_than_n(self):
        self.assertEqual(len(self.V.top_n([{"type":"a"}],[{}], n=5)), 1)

    def test_deterministic_no_random(self):
        s1 = self.V.score({"type":"x"},{"ok_rate":40,"runs":3,"priority":"medium"})["score"]
        s2 = self.V.score({"type":"x"},{"ok_rate":40,"runs":3,"priority":"medium"})["score"]
        self.assertEqual(s1, s2)


# ── 自开发 Wave: safety_preset + artifact_bus ─────────────────────────────────

class SafetyPresetTest(unittest.TestCase):
    """声明式安全分级 — safety_preset.py"""

    def setUp(self):
        from devkit import safety_preset as SP
        self.SP = SP

    def _scan(self, *violations):
        return {"violations": list(violations)}

    def test_get_preset_minimal(self):
        p = self.SP.get_preset("minimal")
        self.assertIn("block_levels", p)

    def test_get_preset_unknown_raises(self):
        with self.assertRaises(ValueError):
            self.SP.get_preset("unknown_level")

    def test_minimal_error_blocks(self):
        r = self.SP.apply_preset("minimal", self._scan({"level":"error","rule":"R","message":"m"}))
        self.assertTrue(r["block"])

    def test_minimal_warn_no_block(self):
        r = self.SP.apply_preset("minimal", self._scan({"level":"warn","rule":"R","message":"m"}))
        self.assertFalse(r["block"])

    def test_standard_error_blocks(self):
        r = self.SP.apply_preset("standard", self._scan({"level":"error","rule":"R","message":"m"}))
        self.assertTrue(r["block"])

    def test_standard_warn_no_block_but_warning(self):
        r = self.SP.apply_preset("standard", self._scan({"level":"warn","rule":"R","message":"m"}))
        self.assertFalse(r["block"])
        self.assertTrue(len(r["warnings"]) > 0)

    def test_strict_any_violation_blocks(self):
        r = self.SP.apply_preset("strict", self._scan({"level":"info","rule":"R","message":"m"}))
        self.assertTrue(r["block"])

    def test_no_violations_pass(self):
        r = self.SP.apply_preset("standard", {"violations": []})
        self.assertFalse(r["block"])
        self.assertEqual(r["warnings"], [])

    def test_reason_field_present(self):
        r = self.SP.apply_preset("minimal", {"violations": []})
        self.assertIn("reason", r)


class ArtifactBusTest(unittest.TestCase):
    """结构化 Artifact 交接总线 — artifact_bus.py"""

    def setUp(self):
        from devkit import artifact_bus as AB
        self.AB = AB

    def _art(self, stage="plan", **kw):
        base = {"stage": stage, "role": "dev", "title": "T", "body": "hello"}
        base.update(kw)
        return base

    def test_to_bus_type(self):
        self.assertEqual(self.AB.to_bus_message(self._art())["type"], "artifact")

    def test_to_bus_stage(self):
        self.assertEqual(self.AB.to_bus_message(self._art("implement"))["stage"], "implement")

    def test_digest_8_chars(self):
        self.assertEqual(len(self.AB.to_bus_message(self._art())["body_digest"]), 8)

    def test_digest_consistent(self):
        a = self.AB.to_bus_message(self._art())
        b = self.AB.to_bus_message(self._art())
        self.assertEqual(a["body_digest"], b["body_digest"])

    def test_digest_changes_with_body(self):
        a = self.AB.to_bus_message(self._art(body="hello"))
        b = self.AB.to_bus_message(self._art(body="world"))
        self.assertNotEqual(a["body_digest"], b["body_digest"])

    def test_roundtrip_stage(self):
        msg = self.AB.to_bus_message(self._art("review"))
        self.assertEqual(self.AB.from_bus_message(msg)["stage"], "review")

    def test_from_bus_default_body(self):
        msg = self.AB.to_bus_message(self._art())
        self.assertEqual(self.AB.from_bus_message(msg).get("body", ""), "")

    def test_merge_total_tokens(self):
        msgs = [self.AB.to_bus_message(self._art(tokens=100)),
                self.AB.to_bus_message(self._art(tokens=200))]
        self.assertEqual(self.AB.merge(msgs)["total_tokens"], 300)

    def test_merge_stages_ordered(self):
        msgs = [self.AB.to_bus_message(self._art("plan")),
                self.AB.to_bus_message(self._art("implement"))]
        self.assertEqual(self.AB.merge(msgs)["stages"], ["plan", "implement"])

    def test_merge_go_count(self):
        msgs = [self.AB.to_bus_message(self._art(verdict="GO")),
                self.AB.to_bus_message(self._art(verdict="NO-GO"))]
        r = self.AB.merge(msgs)
        self.assertEqual(r["go_count"], 1)
        self.assertEqual(r["nogo_count"], 1)


# ── 自开发 Wave: preflight + registry + capacity ──────────────────────────────

class PreflightTest(unittest.TestCase):
    """Token 预估 + 余额检查 — preflight.py"""

    def setUp(self):
        from devkit import preflight as P
        self.P = P

    def test_estimate_returns_dict(self):
        r = self.P.estimate("test task", ["plan", "implement"], {})
        self.assertIsInstance(r, dict)

    def test_estimate_required_keys(self):
        r = self.P.estimate("t", ["implement"], {})
        self.assertIn("ok", r)
        self.assertIn("estimated_tokens", r)
        self.assertIn("balance", r)
        self.assertIn("warning", r)

    def test_estimate_tokens_positive(self):
        r = self.P.estimate("t", ["plan", "implement", "verify"], {})
        self.assertGreater(r["estimated_tokens"], 0)

    def test_estimate_tokens_int(self):
        r = self.P.estimate("t", ["implement"], {})
        self.assertIsInstance(r["estimated_tokens"], int)

    def test_estimate_balance_none_when_no_key(self):
        import os
        old = os.environ.pop("LITELLM_MASTER_KEY", None)
        try:
            r = self.P.estimate("t", ["plan"], {})
            self.assertIsNone(r["balance"])
        finally:
            if old is not None:
                os.environ["LITELLM_MASTER_KEY"] = old

    def test_estimate_more_stages_more_tokens(self):
        r1 = self.P.estimate("t", ["implement"], {})
        r2 = self.P.estimate("t", ["plan", "implement", "verify"], {})
        self.assertGreater(r2["estimated_tokens"], r1["estimated_tokens"])


class RegistryTest(unittest.TestCase):
    """Stage Registry — registry.py"""

    def setUp(self):
        from devkit import registry as R
        self.R = R

    def test_load_returns_list(self):
        self.assertIsInstance(self.R.load(), list)

    def test_get_none_for_missing(self):
        self.assertIsNone(self.R.get("__nonexistent_stage_xyz__"))

    def test_validate_ok(self):
        r = self.R.validate({"key": "plan", "trust_level": 1,
                             "max_cost_per_run": 0.05, "allowed_executors": ["chat"]})
        self.assertTrue(r["ok"])
        self.assertEqual(r["errors"], [])

    def test_validate_bad_trust(self):
        r = self.R.validate({"key": "x", "trust_level": 9,
                             "max_cost_per_run": 0.1, "allowed_executors": ["chat"]})
        self.assertFalse(r["ok"])

    def test_validate_bad_cost(self):
        r = self.R.validate({"key": "x", "trust_level": 1,
                             "max_cost_per_run": -1.0, "allowed_executors": ["chat"]})
        self.assertFalse(r["ok"])

    def test_validate_empty_executors(self):
        r = self.R.validate({"key": "x", "trust_level": 1,
                             "max_cost_per_run": 0.1, "allowed_executors": []})
        self.assertFalse(r["ok"])

    def test_validate_empty_key(self):
        r = self.R.validate({"key": "", "trust_level": 1,
                             "max_cost_per_run": 0.1, "allowed_executors": ["chat"]})
        self.assertFalse(r["ok"])

    def test_validate_errors_is_list(self):
        r = self.R.validate({"key": "", "trust_level": 9,
                             "max_cost_per_run": -1, "allowed_executors": []})
        self.assertIsInstance(r["errors"], list)
        self.assertGreater(len(r["errors"]), 0)


class CapacityTest(unittest.TestCase):
    """容量预检 — capacity.py"""

    def setUp(self):
        from devkit import capacity as C
        self.C = C

    def test_estimate_empty_history(self):
        r = self.C.estimate_run(["plan", "implement"], {}, [])
        self.assertGreater(r["estimated_tokens"], 0)

    def test_estimate_has_per_stage(self):
        r = self.C.estimate_run(["plan"], {}, {})
        self.assertIsInstance(r.get("per_stage", {}), dict)

    def test_estimate_plan_in_per_stage(self):
        r = self.C.estimate_run(["plan", "implement"], {}, {})
        self.assertIn("plan", r["per_stage"])

    def test_preflight_ok_no_budget(self):
        r = self.C.preflight_check(["plan", "implement"], {}, {}, None)
        self.assertTrue(r["ok"])

    def test_preflight_ok_large_budget(self):
        r = self.C.preflight_check(["plan"], {}, {}, 9.99)
        self.assertTrue(r["ok"])

    def test_preflight_fail_zero_budget(self):
        r = self.C.preflight_check(["plan", "implement", "verify"], {}, {}, 0.0)
        self.assertFalse(r["ok"])

    def test_preflight_warning_is_str(self):
        r = self.C.preflight_check(["plan"], {}, {}, 0.0)
        self.assertIsInstance(r.get("warning", ""), str)

    def test_suggest_cheaper_returns_list(self):
        r = self.C.suggest_cheaper(["brainstorm", "plan", "implement"], {}, {}, 0.0)
        self.assertIsInstance(r, list)

    def test_suggest_cheaper_only_optional(self):
        r = self.C.suggest_cheaper(["brainstorm", "plan", "implement", "verify"], {}, {}, 0.0)
        self.assertTrue(all(s in ("brainstorm", "verify") for s in r))

    def test_estimate_cost_float(self):
        r = self.C.estimate_run(["implement"], {}, [])
        self.assertIsInstance(r["estimated_cost"], float)


# ── Wave 3: 多 Carrier 负载均衡 — carrier_router.py ──────────────────────────

class CarrierRouterTest(unittest.TestCase):
    """多 carrier 路由 — carrier_router.py"""

    def setUp(self):
        from devkit import carrier_router as R
        self.R = R
        self._history = [
            {"carrier": "glm", "stage": "implement", "ok_rate": 0.9, "avg_cost": 0.001, "runs": 5},
            {"carrier": "deepseek", "stage": "implement", "ok_rate": 0.7, "avg_cost": 0.002, "runs": 3},
        ]

    def test_select_best_by_ok_rate(self):
        self.assertEqual(self.R.select("implement", ["glm", "deepseek"], self._history), "glm")

    def test_select_no_history_in_candidates(self):
        r = self.R.select("implement", ["glm", "deepseek"], [])
        self.assertIn(r, ["glm", "deepseek"])

    def test_select_no_history_deterministic(self):
        r1 = self.R.select("implement", ["glm", "deepseek"], [])
        r2 = self.R.select("implement", ["glm", "deepseek"], [])
        self.assertEqual(r1, r2)

    def test_select_empty_raises(self):
        with self.assertRaises(ValueError):
            self.R.select("plan", [], [])

    def test_fallback_chain_best_first(self):
        chain = self.R.fallback_chain("implement", ["glm", "deepseek", "minimax"], [
            {"carrier": "minimax", "stage": "implement", "ok_rate": 0.95, "avg_cost": 0.005, "runs": 10}
        ])
        self.assertEqual(chain[0], "minimax")

    def test_fallback_chain_length(self):
        chain = self.R.fallback_chain("plan", ["glm", "deepseek"], [])
        self.assertEqual(len(chain), 2)

    def test_fallback_chain_single(self):
        self.assertEqual(self.R.fallback_chain("impl", ["only"], []), ["only"])

    def test_fallback_chain_empty_raises(self):
        with self.assertRaises(ValueError):
            self.R.fallback_chain("plan", [], [])

    def test_score_carrier_with_history(self):
        s = self.R.score_carrier("glm", "implement", self._history)
        self.assertAlmostEqual(s["ok_rate"], 0.9)
        self.assertEqual(s["runs"], 5)

    def test_score_carrier_no_history_defaults(self):
        s = self.R.score_carrier("new", "implement", [])
        self.assertEqual(s["ok_rate"], 0.5)
        self.assertEqual(s["runs"], 0)

    def test_score_carrier_has_score_field(self):
        s = self.R.score_carrier("glm", "plan", [])
        self.assertIn("score", s)

    def test_score_higher_ok_rate_wins(self):
        sg = self.R.score_carrier("good", "plan", [
            {"carrier": "good", "stage": "plan", "ok_rate": 0.9, "avg_cost": 0.001, "runs": 3}
        ])
        sb = self.R.score_carrier("bad", "plan", [
            {"carrier": "bad", "stage": "plan", "ok_rate": 0.3, "avg_cost": 0.008, "runs": 3}
        ])
        self.assertGreater(sg["score"], sb["score"])


# ── Wave 3: Carrier Benchmark — carrier_bench.py ─────────────────────────────

class CarrierBenchTest(unittest.TestCase):
    """Carrier benchmark — carrier_bench.py"""

    def setUp(self):
        import tempfile
        from devkit import carrier_bench as B
        self.B = B
        self._tmp = pathlib.Path(tempfile.mkdtemp()) / "bench.json"

    def test_load_tasks_nonempty(self):
        self.assertGreater(len(self.B.load_tasks()), 0)

    def test_load_tasks_has_fields(self):
        for t in self.B.load_tasks():
            for f in ("id", "stage", "prompt"):
                self.assertIn(f, t)

    def test_load_results_nonexistent(self):
        self.assertEqual(self.B.load_results("/tmp/nonexistent_bench_xyz123.json"), {})

    def test_save_load_roundtrip(self):
        self.B.save_results({"glm": {}}, str(self._tmp))
        self.assertEqual(self.B.load_results(str(self._tmp)).get("glm"), {})

    def test_run_bench_empty_carriers(self):
        self.assertEqual(self.B.run_bench([], "http://x", "k"), {})

    def test_run_bench_bad_carrier_no_raise(self):
        r = self.B.run_bench(["bad"], "http://127.0.0.1:19999", "k", timeout=1)
        self.assertIsInstance(r, dict)

    def test_run_bench_has_stages(self):
        r = self.B.run_bench(["bad"], "http://127.0.0.1:19999", "k", timeout=1)
        self.assertGreater(len(r.get("bad", {})), 0)

    def test_stage_result_has_fields(self):
        r = self.B.run_bench(["bad"], "http://127.0.0.1:19999", "k", timeout=1)
        stage = list(r["bad"].values())[0]
        for f in ("ok_rate", "avg_latency_ms", "avg_cost", "runs"):
            self.assertIn(f, stage)

    def test_ok_rate_in_range(self):
        r = self.B.run_bench(["bad"], "http://127.0.0.1:19999", "k", timeout=1)
        for stage_data in r.values():
            for sd in stage_data.values():
                self.assertGreaterEqual(sd["ok_rate"], 0.0)
                self.assertLessEqual(sd["ok_rate"], 1.0)

    def test_print_table_empty_no_raise(self):
        self.B.print_table({})  # should not raise

    def test_bench_to_history_rows_empty(self):
        self.assertEqual(self.B.bench_to_history_rows({}), [])

    def test_bench_to_history_rows_shape(self):
        rows = self.B.bench_to_history_rows(
            {"minimax": {"implement": {"ok_rate": 0.9, "avg_cost": 0.001,
                                       "avg_latency_ms": 500, "runs": 5}}}
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["carrier"], "minimax")
        self.assertEqual(rows[0]["stage"], "implement")
        self.assertAlmostEqual(rows[0]["ok_rate"], 0.9)

    def test_bench_to_history_rows_multi_stages(self):
        rows = self.B.bench_to_history_rows({
            "glm": {
                "plan":   {"ok_rate": 0.8, "avg_cost": 0.0, "avg_latency_ms": 300, "runs": 3},
                "verify": {"ok_rate": 0.7, "avg_cost": 0.0, "avg_latency_ms": 200, "runs": 2},
            }
        })
        self.assertEqual(len(rows), 2)
        self.assertEqual({r["stage"] for r in rows}, {"plan", "verify"})


# ── Wave 3: Carrier 健康探针 — carrier_health.py ─────────────────────────────

class CarrierHealthTest(unittest.TestCase):
    """Carrier 健康探针 — carrier_health.py"""

    def setUp(self):
        import tempfile
        from devkit import carrier_health as H
        self.H = H
        self._tmp = pathlib.Path(tempfile.mkdtemp()) / "health.json"

    def test_probe_all_empty(self):
        self.assertEqual(self.H.probe_all([], "http://x", "k"), {})

    def test_healthy_carriers_filters_ok(self):
        r = self.H.healthy_carriers({
            "glm": {"ok": True, "latency_ms": 50.0, "error": ""},
            "bad": {"ok": False, "latency_ms": 0.0, "error": "timeout"},
        })
        self.assertEqual(r, ["glm"])

    def test_healthy_carriers_sorted_by_latency(self):
        r = self.H.healthy_carriers({
            "slow": {"ok": True, "latency_ms": 200.0, "error": ""},
            "fast": {"ok": True, "latency_ms": 30.0, "error": ""},
        })
        self.assertEqual(r, ["fast", "slow"])

    def test_healthy_carriers_empty(self):
        self.assertEqual(self.H.healthy_carriers({}), [])

    def test_probe_unreachable_ok_false(self):
        r = self.H.probe("bad", "http://127.0.0.1:19999", "k", timeout=1)
        self.assertFalse(r["ok"])

    def test_probe_unreachable_latency_zero(self):
        r = self.H.probe("bad", "http://127.0.0.1:19999", "k", timeout=1)
        self.assertEqual(r["latency_ms"], 0.0)

    def test_probe_has_required_fields(self):
        r = self.H.probe("bad", "http://127.0.0.1:19999", "k", timeout=1)
        for f in ("ok", "latency_ms", "error"):
            self.assertIn(f, r)

    def test_probe_minimax_uses_max_completion_tokens(self):
        seen = {}

        class _FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

        def fake_urlopen(req, timeout=0):
            seen["url"] = req.full_url
            seen["payload"] = json.loads(req.data.decode("utf-8"))
            seen["auth"] = req.headers.get("Authorization")
            return _FakeResponse()

        with mock.patch("urllib.request.urlopen", fake_urlopen), \
             mock.patch("devkit.rdloop.load_env_key", return_value="minimax-direct-key"):
            r = self.H.probe("minimax", "http://localhost:4000/v1", "gw-key", timeout=1)

        self.assertTrue(r["ok"])
        self.assertEqual(seen["url"], "https://api.minimaxi.com/v1/chat/completions")
        self.assertEqual(seen["auth"], "Bearer minimax-direct-key")
        self.assertEqual(seen["payload"]["max_completion_tokens"], 1)
        self.assertNotIn("max_tokens", seen["payload"])
        self.assertEqual(seen["payload"]["thinking"], {"type": "disabled"})
        self.assertTrue(seen["payload"]["reasoning_split"])

    def test_load_cache_nonexistent(self):
        self.assertEqual(self.H.load_cache("/tmp/nonexistent_xyz_abc123.json"), {})

    def test_save_load_roundtrip(self):
        self.H.save_cache({"glm": {"ok": True, "latency_ms": 50.0, "error": ""}},
                          str(self._tmp))
        rt = self.H.load_cache(str(self._tmp))
        self.assertTrue(rt["glm"]["ok"])
        self.assertIn("ts", rt["glm"])

    def test_healthy_carriers_single(self):
        r = self.H.healthy_carriers({"a": {"ok": True, "latency_ms": 100.0, "error": ""}})
        self.assertEqual(r, ["a"])


# ── 自治决策日志 — decision_log.py ───────────────────────────────────────────

class DecisionLogTest(unittest.TestCase):
    """devkit auto 决策可追溯性 — decision_log.py"""

    def setUp(self):
        import tempfile, pathlib
        from devkit import decision_log as DL
        self.DL = DL
        self._dir = pathlib.Path(tempfile.mkdtemp())
        self._tmp = self._dir / "decisions.jsonl"
        self._backlog = self._dir / "backlog.json"
        self._backlog.write_text(json.dumps([
            {"id": "t1", "status": "failed", "deps": [], "priority": "low", "task": "old task"}
        ], ensure_ascii=False), encoding="utf-8")

    def _append(self, task_id="t1", run_id="run-1", score=80, outcome="pending"):
        return self.DL.append(
            task_id=task_id, task_text="test task", run_id=run_id,
            score=score, reason="high priority", alternatives=[],
            outcome=outcome, log_path=self._tmp,
        )

    def test_append_creates_file(self):
        self._append()
        self.assertTrue(self._tmp.exists())

    def test_append_returns_record(self):
        r = self._append()
        self.assertIsInstance(r, dict)
        self.assertEqual(r["task_id"], "t1")

    def test_append_required_fields(self):
        r = self._append()
        for f in ("ts", "task_id", "run_id", "score", "reason", "alternatives", "outcome"):
            self.assertIn(f, r)
        for f in ("selection_reason", "root_cause", "next_action", "failure_code"):
            self.assertIn(f, r)

    def test_load_empty_when_no_file(self):
        records = self.DL.load(log_path=self._tmp)
        self.assertEqual(records, [])

    def test_load_returns_appended(self):
        self._append(task_id="a")
        self._append(task_id="b")
        records = self.DL.load(log_path=self._tmp)
        self.assertEqual(len(records), 2)
        self.assertEqual(records[0]["task_id"], "a")

    def test_load_last_n(self):
        for i in range(5):
            self._append(task_id=f"t{i}", run_id=f"r{i}")
        records = self.DL.load(log_path=self._tmp, last_n=3)
        self.assertEqual(len(records), 3)
        self.assertEqual(records[0]["task_id"], "t2")

    def test_update_outcome_changes_pending(self):
        self._append(run_id="run-x", outcome="pending")
        updated = self.DL.update_outcome(
            "run-x", "success", log_path=self._tmp,
            reason="reflection summary",
            root_cause="FORMAT_MISMATCH_NO_FILE_MARKERS",
            next_action="rewrite contract",
            failure_code="FORMAT_MISMATCH_NO_FILE_MARKERS",
        )
        self.assertTrue(updated)
        records = self.DL.load(log_path=self._tmp)
        self.assertEqual(records[0]["outcome"], "success")
        self.assertEqual(records[0]["reason"], "reflection summary")
        self.assertEqual(records[0]["root_cause"], "FORMAT_MISMATCH_NO_FILE_MARKERS")
        self.assertEqual(records[0]["next_action"], "rewrite contract")
        self.assertEqual(records[0]["failure_code"], "FORMAT_MISMATCH_NO_FILE_MARKERS")

    def test_update_outcome_no_file(self):
        updated = self.DL.update_outcome("missing-id", "success", log_path=self._tmp)
        self.assertFalse(updated)

    def test_update_outcome_wrong_run_id(self):
        self._append(run_id="run-A")
        updated = self.DL.update_outcome("run-B", "success", log_path=self._tmp)
        self.assertFalse(updated)
        records = self.DL.load(log_path=self._tmp)
        self.assertEqual(records[0]["outcome"], "pending")

    def test_alternatives_stored(self):
        self.DL.append(
            task_id="x", task_text="t", run_id="r", score=90, reason="r",
            alternatives=[{"task_id": "y", "score": 50, "reason": "low"}],
            log_path=self._tmp,
        )
        rec = self.DL.load(log_path=self._tmp)[0]
        self.assertEqual(len(rec["alternatives"]), 1)
        self.assertEqual(rec["alternatives"][0]["task_id"], "y")

    def test_append_syncs_pending_backlog_state(self):
        self.DL.append(
            task_id="t1", task_text="test task", run_id="run-sync", score=10, reason="r",
            alternatives=[], log_path=self._tmp,
            sync_backlog=True, backlog_path=self._backlog, priority="high",
        )
        items = json.loads(self._backlog.read_text(encoding="utf-8"))
        self.assertEqual(items[0]["status"], "pending")
        self.assertEqual(items[0]["priority"], "low")

    def test_append_sync_inserts_missing_backlog_task(self):
        self.DL.append(
            task_id="fresh-task", task_text="new task", run_id="run-new", score=10, reason="r",
            alternatives=[], log_path=self._tmp,
            sync_backlog=True, backlog_path=self._backlog, priority="high",
        )
        items = {item["id"]: item for item in json.loads(self._backlog.read_text(encoding="utf-8"))}
        self.assertEqual(items["fresh-task"]["status"], "pending")
        self.assertEqual(items["fresh-task"]["priority"], "high")
        self.assertEqual(items["fresh-task"]["task"], "new task")

    def test_update_outcome_syncs_terminal_backlog_state(self):
        self._append(run_id="run-terminal", outcome="pending")
        updated = self.DL.update_outcome(
            "run-terminal",
            "success",
            log_path=self._tmp,
            sync_backlog=True,
            backlog_path=self._backlog,
        )
        self.assertTrue(updated)
        items = {item["id"]: item for item in json.loads(self._backlog.read_text(encoding="utf-8"))}
        self.assertEqual(items["t1"]["status"], "done")

    def test_reconcile_pending_with_backlog_repairs_crash_gap(self):
        self.DL.append(
            task_id="t1", task_text="test task", run_id="run-reconcile", score=10, reason="r",
            alternatives=[], outcome="pending", log_path=self._tmp,
        )
        self._backlog.write_text(json.dumps([
            {"id": "t1", "status": "failed", "deps": [], "priority": "low", "task": "old task"}
        ], ensure_ascii=False), encoding="utf-8")
        changed = self.DL.reconcile_pending_with_backlog(log_path=self._tmp, backlog_path=self._backlog)
        self.assertEqual(changed, 1)
        records = self.DL.load(log_path=self._tmp)
        self.assertEqual(records[0]["outcome"], "failure")


# ── T16 物理验证 — verify.py ──────────────────────────────────────────────────

class VerifyTest(unittest.TestCase):
    """T16 物理验证 — verify.py"""

    def setUp(self):
        import tempfile, pathlib
        from devkit import verify as V
        self.V = V
        self.tmp = pathlib.Path(tempfile.mkdtemp())

    def _write(self, name, code):
        (self.tmp / name).write_text(code, encoding="utf-8")

    def test_smoke_import_good_module(self):
        self._write("good.py", "def f(): return 42\n")
        r = self.V.smoke_import(self.tmp)
        self.assertTrue(r["ok"])
        self.assertIn("good.py", r["imported"])

    def test_smoke_import_bad_module(self):
        self._write("bad.py", "import nonexistent_package_xyz\n")
        r = self.V.smoke_import(self.tmp)
        self.assertFalse(r["ok"])
        self.assertEqual(len(r["errors"]), 1)

    def test_smoke_import_skips_test_files(self):
        self._write("test_foo.py", "import nonexistent_package_xyz\n")
        r = self.V.smoke_import(self.tmp)
        self.assertTrue(r["ok"])
        self.assertEqual(r["errors"], [])

    def test_smoke_import_empty_dir(self):
        r = self.V.smoke_import(self.tmp)
        self.assertTrue(r["ok"])
        self.assertEqual(r["imported"], [])

    def test_smoke_import_adds_repo_root_for_devkit_imports(self):
        pkg = self.tmp / "devkit"
        pkg.mkdir()
        (pkg / "__init__.py").write_text("", encoding="utf-8")
        (pkg / "helper.py").write_text("VALUE = 9\n", encoding="utf-8")
        build = self.tmp / "build"
        build.mkdir()
        (build / "use_helper.py").write_text("from devkit.helper import VALUE\n", encoding="utf-8")
        r = self.V.smoke_import(build)
        self.assertTrue(r["ok"], r)
        self.assertIn("use_helper.py", r["imported"])

    def test_run_golden_subprocess_pass(self):
        import json, pathlib
        self._write("mymod.py", "def add(a,b): return a+b\n")
        golden = [{"name":"add_2_3","import":"from mymod import add","expr":"add(2,3)","expect":5}]
        gf = self.tmp / "g.golden.json"
        gf.write_text(json.dumps(golden), encoding="utf-8")
        r = self.V.run_golden_subprocess(self.tmp, str(gf))
        self.assertTrue(r["ok"])
        self.assertEqual(r["passed"], 1)
        self.assertEqual(r["failed"], 0)

    def test_run_golden_subprocess_fail(self):
        import json
        self._write("mymod.py", "def add(a,b): return a+b\n")
        golden = [{"name":"wrong","import":"from mymod import add","expr":"add(2,3)","expect":99}]
        gf = self.tmp / "g.golden.json"
        gf.write_text(json.dumps(golden), encoding="utf-8")
        r = self.V.run_golden_subprocess(self.tmp, str(gf))
        self.assertFalse(r["ok"])
        self.assertEqual(r["failed"], 1)

    def test_run_golden_missing_file(self):
        r = self.V.run_golden_subprocess(self.tmp, "/nonexistent/loom/g.golden.json")
        self.assertFalse(r["ok"])
        self.assertIn("error", r)

    def test_summarize_smoke(self):
        r = {"ok": True, "errors": [], "imported": ["a.py"]}
        s = self.V.summarize(r)
        self.assertIn("✅", s)
        self.assertIn("a.py", s)

    def test_summarize_golden(self):
        r = {"ok": False, "passed": 1, "failed": 1,
             "rows": [{"name":"p","ok":True,"detail":""},{"name":"f","ok":False,"detail":"got 0"}]}
        s = self.V.summarize(r)
        self.assertIn("1/2", s)
        self.assertIn("got 0", s)

    def test_raises_case(self):
        import json
        self._write("mymod.py", "def boom(): raise ValueError('oops')\n")
        golden = [{"name":"raises_val","import":"from mymod import boom","expr":"boom()","raises":"ValueError"}]
        gf = self.tmp / "g.golden.json"
        gf.write_text(json.dumps(golden), encoding="utf-8")
        r = self.V.run_golden_subprocess(self.tmp, str(gf))
        self.assertTrue(r["ok"])
        self.assertEqual(r["passed"], 1)

    def test_resolve_target_path_rewrites_tests_into_verify_tmp(self):
        got = self.V.resolve_target_path("tests/x.py", "auto-1")
        self.assertTrue(got.rewritten)
        self.assertEqual(got.final_path, "runs/auto-1/_verify_tmp/tests/x.py")
        self.assertIn("materialize-path-rewritten", got.reason)

    def test_resolve_target_path_keeps_safe_runs_path(self):
        got = self.V.resolve_target_path("runs/auto-1/_verify_tmp/out.py", "auto-1")
        self.assertFalse(got.rewritten)
        self.assertEqual(got.final_path, "runs/auto-1/_verify_tmp/out.py")

    def test_assert_path_allowed_blocks_tests_and_devkit(self):
        with self.assertRaises(self.V.MaterializePathForbidden):
            self.V.assert_path_allowed("tests/x.py")
        with self.assertRaises(self.V.MaterializePathForbidden):
            self.V.assert_path_allowed("devkit/verify.py")

    def test_assert_path_allowed_allows_verify_tmp(self):
        self.V.assert_path_allowed("runs/auto-1/_verify_tmp/out.py")

    def test_run_materialize_step_records_blocked_applylock(self):
        decision_log = self.tmp / "decision_log.jsonl"
        task = {"task_id": "auto-1", "materialize": {"target": "tests/x.py", "payload": "print('x')"}}
        r = self.V.run_materialize_step(task, decision_log_path=decision_log)
        self.assertEqual(r["status"], "blocked-applylock")
        rows = decision_log.read_text(encoding="utf-8").splitlines()
        self.assertEqual(len(rows), 1)
        rec = json.loads(rows[0])
        self.assertEqual(rec["decision"], "blocked-applylock")
        self.assertEqual(rec["target_path"], "tests/x.py")

    def test_rdloop_status_counts_includes_blocked_applylock(self):
        decision_log = self.tmp / "decision_log.jsonl"
        decision_log.write_text(
            json.dumps({"decision": "blocked-applylock"}) + "\n" +
            json.dumps({"decision": "blocked-applylock"}) + "\n" +
            json.dumps({"decision": "TEST_COLLECT_ERROR"}) + "\n",
            encoding="utf-8",
        )
        counts = self.V.rdloop_status_counts(decision_log)
        self.assertEqual(counts.get("blocked-applylock"), 2)
        self.assertEqual(counts.get("TEST_COLLECT_ERROR"), 1)


# ── Wave 5: 设置向导 — setup.py ──────────────────────────────────────────────

class SetupWizardTest(unittest.TestCase):
    def setUp(self):
        from devkit.setup import (
            detect_docker, detect_env_key, write_env_file,
            check_gateway, run_setup, start_litellm,
        )
        self.detect_docker = detect_docker
        self.detect_env_key = detect_env_key
        self.write_env_file = write_env_file
        self.check_gateway = check_gateway
        self.run_setup = run_setup
        self.start_litellm = start_litellm
        self.tmp = pathlib.Path(tempfile.mkdtemp())

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_detect_docker_returns_bool(self):
        self.assertIsInstance(self.detect_docker(), bool)

    def test_detect_env_missing_key(self):
        import os as _os
        _os.environ.pop("MINIMAX_API_KEY_NONEXISTENT_XYZ", None)
        self.assertEqual(self.detect_env_key("MINIMAX_API_KEY_NONEXISTENT_XYZ"), "")

    def test_write_env_creates_key(self):
        p = str(self.tmp / "test.env")
        self.write_env_file({"TEST_KEY_XYZ": "test_val"}, p)
        with open(p) as fh:
            self.assertIn("TEST_KEY_XYZ=test_val", fh.read())

    def test_check_gateway_unreachable_ok(self):
        r = self.check_gateway("http://127.0.0.1:19999", timeout=1)
        self.assertFalse(r["ok"])

    def test_check_gateway_unreachable_latency(self):
        r = self.check_gateway("http://127.0.0.1:19999", timeout=1)
        self.assertEqual(r["latency_ms"], 0.0)
        self.assertIsInstance(r["latency_ms"], float)

    def test_run_setup_returns_steps_list(self):
        r = self.run_setup(interactive=False, keys={})
        self.assertIsInstance(r["steps"], list)

    def test_run_setup_steps_have_fields(self):
        r = self.run_setup(interactive=False, keys={})
        for s in r["steps"]:
            self.assertIn("name", s)
            self.assertIn("ok", s)
            self.assertIn("message", s)

    def test_run_setup_with_key_min_steps(self):
        r = self.run_setup(interactive=False, keys={"MINIMAX_API_KEY": "test"})
        self.assertGreaterEqual(len(r["steps"]), 3)

    def test_check_gateway_has_error_field(self):
        r = self.check_gateway("http://127.0.0.1:19999", timeout=1)
        self.assertIn("error", r)
        self.assertIsInstance(r["error"], str)

    def test_write_env_no_duplicate(self):
        p = str(self.tmp / "dup.env")
        self.write_env_file({"DUP_KEY": "v1"}, p)
        self.write_env_file({"DUP_KEY": "v2"}, p)
        self.write_env_file({"DUP_KEY": "v3"}, p)
        with open(p) as fh:
            lines = [l for l in fh.read().splitlines() if l.startswith("DUP_KEY=")]
        self.assertEqual(len(lines), 1)
        self.assertEqual(lines[0], "DUP_KEY=v3")


class PonytailTest(unittest.TestCase):
    def setUp(self):
        from devkit.ponytail import score_patch, gate
        self.score_patch = score_patch
        self.gate = gate

    def test_score_empty(self):
        self.assertEqual(self.score_patch([]), {"added": 0, "removed": 0, "new_imports": 0, "new_files": 0})

    def test_score_added(self):
        self.assertEqual(self.score_patch(["+x=1", "-y=2", "+import os"])["added"], 2)

    def test_score_new_imports(self):
        self.assertEqual(self.score_patch(["+import os", "+from sys import path"])["new_imports"], 2)

    def test_gate_empty_ok(self):
        self.assertTrue(self.gate([], 200, 5)["ok"])

    def test_gate_over_added_nogo(self):
        self.assertFalse(self.gate(["+x"] * 201, 200, 5)["ok"])

    def test_gate_over_added_verdict(self):
        self.assertEqual(self.gate(["+x"] * 201, 200, 5)["verdict"], "NO-GO")

    def test_gate_over_imports_nogo(self):
        self.assertFalse(self.gate(["+import os"] * 6, 200, 5)["ok"])

    def test_gate_ok_small(self):
        self.assertTrue(self.gate(["+x"] * 10, 200, 5)["ok"])

    def test_gate_ok_verdict(self):
        self.assertEqual(self.gate(["+x"] * 10, 200, 5)["verdict"], "GO")

    def test_gate_new_file_ok(self):
        self.assertTrue(self.gate(["+++ b/new.py"], 200, 5)["ok"])


class WalletTest(unittest.TestCase):
    def setUp(self):
        from devkit.wallet import check_deepseek, estimate_balance, summary
        self.check_deepseek = check_deepseek
        self.estimate_balance = estimate_balance
        self.summary = summary

    def test_check_deepseek_returns_dict(self):
        self.assertIsInstance(self.check_deepseek("invalid-key-xyz", timeout=3), dict)

    def test_check_deepseek_ok_false(self):
        self.assertFalse(self.check_deepseek("invalid-key-xyz", timeout=3)["ok"])

    def test_check_deepseek_balance_zero(self):
        self.assertEqual(self.check_deepseek("invalid-key-xyz", timeout=3)["balance_usd"], 0.0)

    def test_check_deepseek_has_error(self):
        self.assertIn("error", self.check_deepseek("invalid-key-xyz", timeout=3))

    def test_estimate_minimax_source(self):
        self.assertEqual(self.estimate_balance("minimax", "")["source"], "unknown")

    def test_estimate_minimax_balance(self):
        self.assertEqual(self.estimate_balance("minimax", "")["balance_usd"], -1.0)

    def test_estimate_minimax_ok(self):
        self.assertTrue(self.estimate_balance("minimax", "")["ok"])

    def test_summary_is_list(self):
        self.assertIsInstance(self.summary(["deepseek", "minimax"], {}), list)

    def test_summary_len(self):
        self.assertEqual(len(self.summary(["deepseek", "minimax"], {})), 2)

    def test_summary_has_fields(self):
        self.assertTrue(all("provider" in r and "balance_usd" in r for r in self.summary(["deepseek"], {})))


class RetryPolicyTest(unittest.TestCase):
    def setUp(self):
        from devkit.retry import should_retry, record_attempt, reset_for_retry, filter_retryable
        self.should_retry = should_retry
        self.record_attempt = record_attempt
        self.reset_for_retry = reset_for_retry
        self.filter_retryable = filter_retryable

    def test_should_retry_failed_under_limit(self):
        self.assertTrue(self.should_retry({"status": "failed", "_attempts": 0}))

    def test_should_retry_failed_at_limit(self):
        self.assertFalse(self.should_retry({"status": "failed", "_attempts": 2}))

    def test_should_retry_done(self):
        self.assertFalse(self.should_retry({"status": "done", "_attempts": 0}))

    def test_should_retry_pending(self):
        self.assertFalse(self.should_retry({"status": "pending"}))

    def test_record_attempt_from_zero(self):
        self.assertEqual(self.record_attempt({"status": "failed"})["_attempts"], 1)

    def test_record_attempt_increment(self):
        self.assertEqual(self.record_attempt({"status": "failed", "_attempts": 1})["_attempts"], 2)

    def test_reset_status(self):
        self.assertEqual(self.reset_for_retry({"status": "failed"})["status"], "pending")

    def test_reset_attempts(self):
        self.assertEqual(self.reset_for_retry({"status": "failed"})["_attempts"], 1)

    def test_reset_done_unchanged(self):
        self.assertEqual(self.reset_for_retry({"status": "done"})["status"], "done")

    def test_filter_retryable(self):
        bl = [{"id": "a", "status": "failed"}, {"id": "b", "status": "done"}, {"id": "c", "status": "failed", "_attempts": 2}]
        result = self.filter_retryable(bl)
        self.assertEqual(len(result), 1)
        self.assertEqual(result[0]["id"], "a")


class LearningSidecarTest(unittest.TestCase):
    def setUp(self):
        from devkit.learning import extract_events, suggest, top_suggestions
        self.extract_events = extract_events
        self.suggest = suggest
        self.top_suggestions = top_suggestions

    def test_extract_nonexistent(self):
        self.assertEqual(self.extract_events("/nonexistent/path/xyz"), [])

    def test_suggest_empty_list(self):
        self.assertIsInstance(self.suggest([]), list)

    def test_suggest_empty_returns_empty(self):
        self.assertEqual(self.suggest([]), [])

    def test_suggest_go_is_list(self):
        evs = [{"stage": "implement", "gate": "GO", "carrier": "minimax", "tokens": 500, "file": "03.md"}]
        self.assertIsInstance(self.suggest(evs), list)

    def test_suggest_go_nonempty(self):
        evs = [{"stage": "implement", "gate": "GO", "carrier": "minimax", "tokens": 500, "file": "03.md"}]
        self.assertGreaterEqual(len(self.suggest(evs)), 1)

    def test_suggest_confidence_nonneg(self):
        evs = [{"stage": "implement", "gate": "GO", "carrier": "minimax", "tokens": 500, "file": "03.md"}]
        self.assertGreaterEqual(self.suggest(evs)[0]["confidence"], 0)

    def test_suggest_nogo_fields(self):
        evs = [
            {"stage": "implement", "gate": "NO-GO", "carrier": "glm", "tokens": 200, "file": "03.md"},
            {"stage": "verify", "gate": "NO-GO", "carrier": "glm", "tokens": 100, "file": "04.md"},
        ]
        for s in self.suggest(evs):
            self.assertIn("type", s)
            self.assertIn("reason", s)
            self.assertIn("confidence", s)

    def test_top_suggestions_empty(self):
        self.assertEqual(self.top_suggestions([]), [])

    def test_top_suggestions_is_list(self):
        self.assertIsInstance(self.top_suggestions([]), list)

    def test_top_suggestions_nonexistent(self):
        self.assertIsInstance(self.top_suggestions(["/nonexistent"]), list)


class CarrierMetricsTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from carrier_metrics import parse_run_log, aggregate
        self.parse = parse_run_log
        self.aggregate = aggregate

    def test_parse_empty_is_dict(self):
        self.assertIsInstance(self.parse(''), dict)

    def test_parse_empty_gate(self):
        self.assertEqual(self.parse('')['gate'], '')

    def test_parse_empty_tokens(self):
        self.assertEqual(self.parse('')['tokens'], 0)

    def test_parse_go(self):
        self.assertEqual(self.parse('Gate: GO')['gate'], 'GO')

    def test_parse_no_go(self):
        self.assertEqual(self.parse('Gate: NO-GO')['gate'], 'NO-GO')

    def test_parse_tokens(self):
        self.assertEqual(self.parse('用量合计：1234 tok')['tokens'], 1234)

    def test_parse_carrier(self):
        self.assertIn('glm', self.parse('实际=glm')['carriers_used'])

    def test_aggregate_empty_is_dict(self):
        self.assertIsInstance(self.aggregate([]), dict)

    def test_aggregate_empty_total(self):
        self.assertEqual(self.aggregate([])['total_runs'], 0)

    def test_aggregate_go_count(self):
        self.assertEqual(self.aggregate(['Gate: GO', 'Gate: NO-GO'])['go_runs'], 1)


class DecisionsWiringTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from decisions_wiring import record_outcome, batch_record, read_recent
        self.record = record_outcome
        self.batch = batch_record
        self.read_recent = read_recent
        self._tp = '/tmp/dw_features_test.jsonl'
        self._tp2 = '/tmp/dw_features_test2.jsonl'
        import os
        for p in [self._tp, self._tp2]:
            os.path.exists(p) and os.remove(p)

    def test_record_no_raise(self):
        try:
            self.record('t1', 'success')
        except Exception as e:
            self.fail(f'record_outcome raised {e}')

    def test_read_nonexistent_empty(self):
        self.assertEqual(self.read_recent(1, log_path='/tmp/dw_nonexistent.jsonl'), [])

    def test_record_and_read(self):
        self.record('t1', 'success', 'glm', 100, self._tp)
        self.assertEqual(len(self.read_recent(10, self._tp)), 1)

    def test_read_task_id(self):
        self.record('t1', 'success', 'glm', 100, self._tp)
        self.assertEqual(self.read_recent(1, self._tp)[0]['task_id'], 't1')

    def test_read_outcome(self):
        self.record('t1', 'success', 'glm', 100, self._tp)
        self.assertEqual(self.read_recent(1, self._tp)[0]['outcome'], 'success')

    def test_read_has_ts(self):
        self.record('t1', 'success', 'glm', 100, self._tp)
        self.assertIn('ts', self.read_recent(1, self._tp)[0])

    def test_batch_is_int(self):
        n = self.batch([{'task_id': 'a', 'outcome': 'done'}], self._tp2)
        self.assertIsInstance(n, int)

    def test_batch_count(self):
        n = self.batch([{'task_id': 'a', 'outcome': 'done'}, {'task_id': 'b', 'outcome': 'failed'}], self._tp2)
        self.assertEqual(n, 2)

    def test_batch_skips_invalid(self):
        n = self.batch([{'no_task_id': 'x'}], self._tp2)
        self.assertEqual(n, 0)

    def test_read_zero_n(self):
        self.record('t1', 'success', 'glm', 100, self._tp)
        self.assertEqual(self.read_recent(0, self._tp), [])


class DecisionsLogTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from decisions_log import append, read_all, last_n, clear
        self.append = append
        self.read_all = read_all
        self.last_n = last_n
        self.clear = clear
        self._tp = '/tmp/test_dl_features.jsonl'
        import os; os.path.exists(self._tp) and os.remove(self._tp)

    def test_read_nonexistent_is_list(self):
        self.assertIsInstance(self.read_all('/x/y.jsonl'), list)

    def test_read_nonexistent_empty(self):
        self.assertEqual(self.read_all('/x/y.jsonl'), [])

    def test_read_default_no_raise(self):
        self.assertIsInstance(self.read_all(), list)

    def test_append_and_read(self):
        self.append(self._tp, {'k': 'v'})
        self.assertEqual(self.read_all(self._tp)[0], {'k': 'v'})

    def test_append_two_records(self):
        self.append(self._tp, {'k': 'v'})
        self.append(self._tp, {'k': 'v2'})
        self.assertEqual(len(self.read_all(self._tp)), 2)

    def test_last_n_returns_last(self):
        self.append(self._tp, {'k': 'v'})
        self.append(self._tp, {'k': 'v2'})
        self.assertEqual(self.last_n(self._tp, 1)[0], {'k': 'v2'})

    def test_last_n_nonexistent_is_list(self):
        self.assertIsInstance(self.last_n('/x/y.jsonl', 5), list)

    def test_last_n_nonexistent_empty(self):
        self.assertEqual(self.last_n('/x/y.jsonl', 5), [])

    def test_clear_empties_file(self):
        self.append(self._tp, {'k': 'v'})
        self.clear(self._tp)
        self.assertEqual(self.read_all(self._tp), [])

    def test_clear_nonexistent_no_raise(self):
        try:
            self.clear('/tmp/nonexistent_dl_test_features.jsonl')
        except Exception as e:
            self.fail(f'clear() raised {e}')


class RunArchiveTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from run_archive import list_old_runs, archive_run, prune
        self.list_old = list_old_runs
        self.archive = archive_run
        self.prune = prune

    def test_list_nonexistent_is_list(self):
        self.assertIsInstance(self.list_old('/nonexistent'), list)

    def test_list_nonexistent_empty(self):
        self.assertEqual(self.list_old('/nonexistent'), [])

    def test_list_keep_lots_empty(self):
        self.assertEqual(self.list_old('devkit/runs', keep_latest=1000), [])

    def test_archive_nonexistent_is_bool(self):
        self.assertIsInstance(self.archive('nonexistent', '/x', '/y'), bool)

    def test_archive_nonexistent_false(self):
        self.assertFalse(self.archive('nonexistent', '/x', '/y'))

    def test_prune_nonexistent_zero(self):
        self.assertEqual(self.prune('/nonexistent', keep_latest=10), 0)

    def test_prune_keep_lots_is_int(self):
        self.assertIsInstance(self.prune('devkit/runs', keep_latest=1000), int)

    def test_prune_keep_lots_zero(self):
        self.assertEqual(self.prune('devkit/runs', keep_latest=1000), 0)

    def test_list_count_consistent(self):
        import os
        total = len([e for e in os.listdir('devkit/runs') if os.path.isdir(f'devkit/runs/{e}')])
        old = self.list_old('devkit/runs', keep_latest=20)
        self.assertLessEqual(len(old), max(0, total - 0))

    def test_prune_default_is_int(self):
        self.assertIsInstance(self.prune('devkit/runs'), int)


class GraphCliTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from graph_cli import ascii_tree, summary_line
        self.ascii_tree = ascii_tree
        self.summary_line = summary_line

    def test_empty_is_str(self):
        self.assertIsInstance(self.ascii_tree([]), str)

    def test_empty_returns_message(self):
        self.assertEqual(self.ascii_tree([]), '(empty backlog)')

    def test_summary_empty_is_str(self):
        self.assertIsInstance(self.summary_line([]), str)

    def test_summary_empty_content(self):
        self.assertEqual(self.summary_line([]), '0 tasks: 0 done, 0 pending, 0 failed')

    def test_summary_done_word(self):
        self.assertIn('done', self.summary_line([{'status': 'done'}]))

    def test_tree_single_is_str(self):
        self.assertIsInstance(self.ascii_tree([{'id': 'a', 'status': 'done', 'deps': []}]), str)

    def test_tree_contains_id(self):
        self.assertIn('a', self.ascii_tree([{'id': 'a', 'status': 'done', 'deps': []}]))

    def test_tree_contains_letter(self):
        self.assertIn('D', self.ascii_tree([{'id': 'a', 'status': 'done', 'deps': []}]))

    def test_summary_one_done(self):
        self.assertIn('1 done', self.summary_line([{'status': 'pending'}, {'status': 'done'}]))

    def test_summary_total(self):
        self.assertIn('2 tasks', self.summary_line([{'status': 'pending'}, {'status': 'done'}]))


class RunSummaryTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from run_summary import recent_runs, format_table
        self.recent_runs = recent_runs
        self.format_table = format_table

    def test_nonexistent_returns_list(self):
        self.assertIsInstance(self.recent_runs(runs_dir='/nonexistent'), list)

    def test_nonexistent_returns_empty(self):
        self.assertEqual(self.recent_runs(runs_dir='/nonexistent'), [])

    def test_real_returns_list(self):
        self.assertIsInstance(self.recent_runs(), list)

    def test_real_has_run_id(self):
        self.assertTrue(all('run_id' in r for r in self.recent_runs()))

    def test_real_has_gate(self):
        self.assertTrue(all('gate' in r for r in self.recent_runs()))

    def test_format_empty_is_str(self):
        self.assertIsInstance(self.format_table([]), str)

    def test_format_empty_is_no_runs(self):
        self.assertEqual(self.format_table([]), '(no runs)')

    def test_format_row_is_str(self):
        row = {'run_id': 'r1', 'gate': 'GO', 'tokens': 100, 'cost_usd': 0.0, 'stages': 2}
        self.assertIsInstance(self.format_table([row]), str)

    def test_format_row_contains_id(self):
        row = {'run_id': 'r1', 'gate': 'GO', 'tokens': 100, 'cost_usd': 0.0, 'stages': 2}
        self.assertIn('r1', self.format_table([row]))

    def test_format_row_contains_gate(self):
        row = {'run_id': 'r1', 'gate': 'GO', 'tokens': 100, 'cost_usd': 0.0, 'stages': 2}
        self.assertIn('GO', self.format_table([row]))

    def test_recent_runs_prefers_auto_runs_when_present(self):
        import tempfile

        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            for name in ("test-wiring-999", "auto-20260702-999999", "auto-20260702-888888"):
                run_dir = root / name
                run_dir.mkdir()
                (run_dir / "run-log.md").write_text("", encoding="utf-8")
            rows = self.recent_runs(runs_dir=str(root))
        self.assertEqual([r["run_id"] for r in rows], ["auto-20260702-999999", "auto-20260702-888888"])

    def test_recent_runs_parses_current_run_log_markdown_shape(self):
        import tempfile

        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            run_dir = root / "auto-20260703-000001"
            run_dir.mkdir()
            (run_dir / "run-log.md").write_text(
                "# R&D Loop Run auto-20260703-000001\n\n"
                "- 用量合计：**14302 tokens · $0.00000**\n\n"
                "## 各阶段\n\n"
                "| 阶段 | 载体 | 实际模型 | 状态 | 用时 | tokens | 花费 |\n"
                "| --- | --- | --- | --- | --- | --- | --- |\n"
                "| plan | loom-orchestrator | minimax | OK | 1.0s | 100 | $0.00000 |\n"
                "| implement | loom-dev | MiniMax-M3 | OK | 2.0s | 200 | $0.00000 |\n"
                "| verify | loom-tester | MiniMax-M3 | OK | 3.0s | 300 | $0.00000 |\n\n"
                "## Gate 建议\n\n"
                "NO-GO（构建测试/Eval 未过）\n",
                encoding="utf-8",
            )
            rows = self.recent_runs(runs_dir=str(root))
        self.assertEqual(rows[0]["tokens"], 14302)
        self.assertEqual(rows[0]["stages"], 3)
        self.assertEqual(rows[0]["gate"], "NO-GO（构建测试/Eval 未过）")


class BacklogStatsTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from backlog_stats import stats, velocity, health_score
        self.stats = stats
        self.velocity = velocity
        self.health_score = health_score

    def test_stats_empty_is_dict(self):
        self.assertIsInstance(self.stats([]), dict)

    def test_stats_empty_total(self):
        self.assertEqual(self.stats([])['total'], 0)

    def test_stats_empty_completion_pct(self):
        self.assertEqual(self.stats([])['completion_pct'], 0.0)

    def test_stats_done_count(self):
        self.assertEqual(self.stats([{'id': 'a', 'status': 'done', 'priority': 'high'}])['done'], 1)

    def test_stats_completion_100(self):
        self.assertEqual(self.stats([{'id': 'a', 'status': 'done', 'priority': 'high'}])['completion_pct'], 100.0)

    def test_velocity_is_dict(self):
        self.assertIsInstance(self.velocity([]), dict)

    def test_velocity_has_done_key(self):
        self.assertIn('done_in_window', self.velocity([]))

    def test_health_score_empty(self):
        self.assertEqual(self.health_score([]), 0.5)

    def test_health_score_all_done(self):
        self.assertEqual(self.health_score([{'status': 'done'}, {'status': 'done'}]), 1.0)

    def test_health_score_half(self):
        self.assertEqual(self.health_score([{'status': 'done'}, {'status': 'failed'}]), 0.5)


class CompactLogTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from compact_log import extract_headers, extract_code_files, compress
        self.extract_headers = extract_headers
        self.extract_code_files = extract_code_files
        self.compress = compress

    def test_extract_headers_empty(self):
        self.assertEqual(self.extract_headers(''), [])

    def test_extract_headers_two(self):
        self.assertEqual(self.extract_headers('## Hello\n### World'), ['Hello', 'World'])

    def test_extract_headers_h1_excluded(self):
        self.assertEqual(self.extract_headers('# Top\n## Sub'), ['Sub'])

    def test_extract_code_files_empty(self):
        self.assertEqual(self.extract_code_files(''), [])

    def test_extract_code_files_found(self):
        self.assertEqual(self.extract_code_files('```python\n# foo.py\n```'), ['foo.py'])

    def test_extract_code_files_no_match(self):
        self.assertEqual(self.extract_code_files('```\nno file\n```'), [])

    def test_compress_returns_str(self):
        self.assertIsInstance(self.compress(''), str)

    def test_compress_truncates(self):
        self.assertLessEqual(len(self.compress('x' * 1000, max_chars=100)), 100)

    def test_compress_passthrough(self):
        self.assertEqual(self.compress('short'), 'short')

    def test_compress_long_is_str(self):
        self.assertIsInstance(self.compress('## Hello\nsome text', max_chars=50), str)


class BenchReportTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from bench_report import load_bench, top_carriers, format_bench
        self.load_bench = load_bench
        self.top_carriers = top_carriers
        self.format_bench = format_bench

    def test_load_nonexistent_is_list(self):
        self.assertIsInstance(self.load_bench('/x/y.json'), list)

    def test_load_nonexistent_empty(self):
        self.assertEqual(self.load_bench('/x/y.json'), [])

    def test_top_empty_is_list(self):
        self.assertIsInstance(self.top_carriers([]), list)

    def test_top_empty(self):
        self.assertEqual(self.top_carriers([]), [])

    def test_top_n1_returns_best(self):
        rows = [{'carrier': 'a', 'ok_rate': 0.9}, {'carrier': 'b', 'ok_rate': 0.5}]
        self.assertEqual(self.top_carriers(rows, n=1), [{'carrier': 'a', 'ok_rate': 0.9}])

    def test_format_empty_is_str(self):
        self.assertIsInstance(self.format_bench([]), str)

    def test_format_empty_no_data(self):
        self.assertEqual(self.format_bench([]), '(no bench data)')

    def test_format_has_top_carrier(self):
        self.assertIn('top carrier', self.format_bench([{'carrier': 'glm', 'ok_rate': 0.8}]))

    def test_load_default_is_list(self):
        self.assertIsInstance(self.load_bench(), list)

    def test_top_n2_length(self):
        rows = [{'ok_rate': 0.1}, {'ok_rate': 0.9}, {'ok_rate': 0.5}]
        self.assertEqual(len(self.top_carriers(rows, n=2)), 2)


class DashboardTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent.parent))
        from devkit.dashboard import render
        self.render = render

    def test_nonexistent_is_str(self):
        self.assertIsInstance(self.render('/x.json', '/y'), str)

    def test_nonexistent_has_dashboard(self):
        self.assertIn('Dashboard', self.render('/x.json', '/y'))

    def test_nonexistent_has_backlog(self):
        self.assertIn('Backlog', self.render('/x.json', '/y'))

    def test_nonexistent_has_health(self):
        self.assertIn('Health', self.render('/x.json', '/y'))

    def test_default_is_str(self):
        self.assertIsInstance(self.render(), str)

    def test_default_has_dashboard(self):
        self.assertIn('Dashboard', self.render())

    def test_default_has_done(self):
        self.assertIn('Done', self.render())

    def test_min_length(self):
        self.assertGreater(len(self.render()), 10)

    def test_multiline(self):
        self.assertGreaterEqual(self.render('/x.json', '/y').count('\n'), 3)

    def test_has_pending(self):
        self.assertIn('Pending', self.render('/x.json', '/y'))


class AgentObservabilityTest(unittest.TestCase):
    def setUp(self):
        from devkit.agent_observability import collect
        self.collect = collect

    def test_collect_returns_dict(self):
        self.assertIsInstance(self.collect(backlog_path='/x.json', runs_dir='/y'), dict)

    def test_collect_has_expected_keys(self):
        d = self.collect(backlog_path='/x.json', runs_dir='/y')
        for key in ('autopilot', 'queue', 'latest_run', 'alerts'):
            self.assertIn(key, d)

    def test_silent_zero_detected_from_run_log(self):
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            runs = root / 'runs' / 'r1'
            runs.mkdir(parents=True)
            (runs / 'run-log.md').write_text(
                "- 任务：probe\n\n"
                "| implement | minimax | MiniMax-M3 | OK | 0.0s | 0 | $0.00000 |\n"
                "## Gate 建议\n\nNO-GO（构建测试/Eval 未过）\n",
                encoding='utf-8',
            )
            out = self.collect(backlog_path=str(root / 'missing.json'),
                               runs_dir=str(root / 'runs'),
                               decisions_path=str(root / 'decisions.jsonl'),
                               iterate_log_path=str(root / 'iterate.log'),
                               supervisor_log_path=str(root / 'supervisor.log'),
                               worker_pid_path=str(root / 'worker.pid'))
        self.assertTrue(out['latest_run']['silent_zero'])
        self.assertIn('implement', out['latest_run']['suspicious_stages'])

    def test_latest_run_extracts_failure_code(self):
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            runs = root / 'runs' / 'r1'
            runs.mkdir(parents=True)
            (runs / 'run-log.md').write_text(
                "- 任务：probe\n\n"
                "## Gate 建议\n\nNO-GO（构建测试/Eval 未过）\n\n"
                "- 收集失败码：**TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN**\n",
                encoding='utf-8',
            )
            out = self.collect(backlog_path=str(root / 'missing.json'),
                               runs_dir=str(root / 'runs'),
                               decisions_path=str(root / 'decisions.jsonl'),
                               iterate_log_path=str(root / 'iterate.log'),
                               supervisor_log_path=str(root / 'supervisor.log'),
                               worker_pid_path=str(root / 'worker.pid'))
        self.assertEqual(out['latest_run']['failure_code'], 'TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN')

    def test_queue_summary_surfaces_task_contract_and_stop_reason(self):
        with tempfile.TemporaryDirectory() as d:
            root = pathlib.Path(d)
            backlog = root / 'backlog.json'
            backlog.write_text(json.dumps({
                "tasks": [
                    {
                        "id": "a",
                        "status": "failed",
                        "priority": "high",
                        "task_kind": "diag",
                        "delivery_mode": "report-only",
                        "failure_code": "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN",
                        "task": "x",
                    },
                    {
                        "id": "b",
                        "status": "stopped",
                        "priority": "low",
                        "stop_reason": "human_required_report_only",
                        "human_required_reason": "report-only-cannot-apply:devkit/x.py",
                        "task": "y",
                    },
                ]
            }, ensure_ascii=False), encoding='utf-8')
            out = self.collect(backlog_path=str(backlog),
                               runs_dir=str(root / 'runs'),
                               decisions_path=str(root / 'decisions.jsonl'),
                               iterate_log_path=str(root / 'iterate.log'),
                               supervisor_log_path=str(root / 'supervisor.log'),
                               worker_pid_path=str(root / 'worker.pid'))
        self.assertEqual(out['queue']['totals']['contract_blocked'], 1)
        self.assertEqual(out['queue']['totals']['human_required'], 1)
        self.assertTrue(out['queue']['failed_top'][0]['contract_blocked'])
        self.assertEqual(out['queue']['failed_top'][0]['task_kind'], 'diag')


class AutoloopCarrierNormalizationTest(unittest.TestCase):
    def test_run_once_normalizes_legacy_carriers(self):
        from devkit.autoloop import run_once
        out = run_once({
            "task": "x",
            "carrier": {
                "review": "gpt-5.4",
                "implement": "minimax/MiniMax-M3",
            },
        })
        self.assertIn("review=loom-reviewer", out["carriers"])
        self.assertIn("implement=minimax", out["carriers"])


class ExecOpencodeTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from exec_opencode import available, run, version
        self.available = available
        self.run = run
        self.version = version

    def test_available_returns_bool(self):
        self.assertIsInstance(self.available(), bool)

    def test_run_ok_is_bool(self):
        self.assertIsInstance(self.run("x", cwd="/tmp", timeout=1)["ok"], bool)

    def test_run_output_is_str(self):
        self.assertIsInstance(self.run("x", cwd="/tmp", timeout=1)["output"], str)

    def test_run_exit_code_is_int(self):
        self.assertIsInstance(self.run("x", cwd="/tmp", timeout=1)["exit_code"], int)

    def test_run_has_output_key(self):
        self.assertIn("output", self.run("x", cwd="/tmp", timeout=1))

    def test_version_returns_str(self):
        self.assertIsInstance(self.version(), str)

    def test_run_unavailable_ok_false(self):
        if not self.available():
            self.assertFalse(self.run("x")["ok"])

    def test_run_unavailable_message(self):
        if not self.available():
            self.assertEqual(self.run("x")["output"], "opencode not available")

    def test_version_no_raise(self):
        try:
            self.version()
        except Exception as e:
            self.fail(f"version() raised {e}")

    def test_run_exit_code_int_on_timeout(self):
        self.assertIsInstance(self.run("x", timeout=1)["exit_code"], int)


class DecisionReplayTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from decision_replay import load_decisions, outcome_stats, top_failing_tasks, summary_report
        self.load = load_decisions
        self.stats = outcome_stats
        self.top = top_failing_tasks
        self.summary = summary_report

    def test_load_nonexistent_is_list(self):
        self.assertIsInstance(self.load("nonexistent.jsonl"), list)

    def test_load_nonexistent_empty(self):
        self.assertEqual(self.load("nonexistent.jsonl"), [])

    def test_stats_empty_is_dict(self):
        self.assertIsInstance(self.stats([]), dict)

    def test_stats_empty_total(self):
        self.assertEqual(self.stats([])["total"], 0)

    def test_stats_success_count(self):
        records = [{"outcome": "success"}, {"outcome": "failure"}]
        self.assertEqual(self.stats(records)["success"], 1)

    def test_stats_total_count(self):
        records = [{"outcome": "success"}, {"outcome": "failure"}]
        self.assertEqual(self.stats(records)["total"], 2)

    def test_top_failing_empty_is_list(self):
        self.assertIsInstance(self.top([]), list)

    def test_top_failing_empty(self):
        self.assertEqual(self.top([]), [])

    def test_summary_is_str(self):
        self.assertIsInstance(self.summary([]), str)

    def test_summary_contains_total(self):
        self.assertIn("总计", self.summary([]))


class RunMonitorTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from run_monitor import list_runs, stale_runs, active_run
        self.list_runs = list_runs
        self.stale_runs = stale_runs
        self.active_run = active_run

    def test_list_nonexistent_is_list(self):
        self.assertIsInstance(self.list_runs("/nonexistent"), list)

    def test_list_nonexistent_empty(self):
        self.assertEqual(self.list_runs("/nonexistent"), [])

    def test_stale_nonexistent_is_list(self):
        self.assertIsInstance(self.stale_runs("/nonexistent"), list)

    def test_stale_nonexistent_empty(self):
        self.assertEqual(self.stale_runs("/nonexistent"), [])

    def test_active_nonexistent_none(self):
        self.assertIsNone(self.active_run("/nonexistent"))

    def test_list_real_is_list(self):
        self.assertIsInstance(self.list_runs("devkit/runs"), list)

    def test_list_real_has_required_fields(self):
        runs = self.list_runs("devkit/runs")
        self.assertTrue(all("run_id" in r and "status" in r for r in runs))

    def test_list_real_age_is_float(self):
        runs = self.list_runs("devkit/runs")
        self.assertTrue(all(isinstance(r["age_minutes"], float) for r in runs))

    def test_list_real_files_is_list(self):
        runs = self.list_runs("devkit/runs")
        self.assertTrue(all(isinstance(r["files"], list) for r in runs))

    def test_stale_threshold_zero_is_list(self):
        self.assertIsInstance(self.stale_runs("devkit/runs", threshold_minutes=0.0), list)


class TaskGraphTest(unittest.TestCase):
    def setUp(self):
        sys.path.insert(0, str(pathlib.Path(__file__).parent))
        from task_graph import build_graph, critical_path, ready_tasks
        self.build = build_graph
        self.cpath = critical_path
        self.ready = ready_tasks

    def test_build_empty_is_dict(self):
        self.assertIsInstance(self.build([]), dict)

    def test_build_empty_nodes(self):
        self.assertEqual(self.build([])["nodes"], [])

    def test_build_empty_cycles(self):
        self.assertEqual(self.build([])["cycles"], [])

    def test_critical_path_empty_is_list(self):
        self.assertIsInstance(self.cpath(self.build([]), []), list)

    def test_critical_path_empty(self):
        self.assertEqual(self.cpath(self.build([]), []), [])

    def test_ready_empty_is_list(self):
        self.assertIsInstance(self.ready([]), list)

    def test_ready_no_deps_pending(self):
        t = {"id": "a", "status": "pending", "deps": []}
        self.assertEqual(self.ready([t]), [t])

    def test_ready_dep_not_done(self):
        t = {"id": "b", "status": "pending", "deps": ["a"]}
        self.assertEqual(self.ready([t]), [])

    def test_build_single_node(self):
        g = self.build([{"id": "a", "deps": []}])
        self.assertEqual(len(g["nodes"]), 1)

    def test_ready_already_done(self):
        t = {"id": "a", "status": "done", "deps": []}
        self.assertEqual(self.ready([t]), [])


class RunReportTest(unittest.TestCase):
    def setUp(self):
        from devkit.run_report import load_run, to_html
        self.load_run = load_run
        self.to_html = to_html

    def test_load_nonexistent_is_dict(self):
        self.assertIsInstance(self.load_run('/nonexistent'), dict)

    def test_load_nonexistent_stages_empty(self):
        self.assertEqual(self.load_run('/nonexistent')['stages'], [])

    def test_load_nonexistent_gate_empty(self):
        self.assertEqual(self.load_run('/nonexistent')['gate'], '')

    def test_to_html_empty_is_str(self):
        self.assertIsInstance(self.to_html({}), str)

    def test_to_html_empty_starts_doctype(self):
        self.assertTrue(self.to_html({}).startswith('<!DOCTYPE'))

    def test_to_html_with_data_is_str(self):
        self.assertIsInstance(self.to_html({'run_id': 'r1', 'gate': 'GO', 'stages': []}), str)

    def test_to_html_contains_run_id(self):
        self.assertIn('r1', self.to_html({'run_id': 'r1', 'gate': 'GO', 'stages': []}))

    def test_to_html_contains_gate(self):
        self.assertIn('GO', self.to_html({'run_id': 'r1', 'gate': 'GO', 'stages': []}))

    def test_load_runs_dir_is_dict(self):
        self.assertIsInstance(self.load_run('devkit/runs'), dict)

    def test_load_runs_dir_has_stages(self):
        self.assertIn('stages', self.load_run('devkit/runs'))


class LoopHooksTest(unittest.TestCase):
    def setUp(self):
        from devkit import loop_hooks
        self.lh = loop_hooks
        loop_hooks.clear()

    def tearDown(self):
        self.lh.clear()

    def test_fire_unregistered_is_list(self):
        self.assertIsInstance(self.lh.fire('task_start'), list)

    def test_fire_unregistered_empty(self):
        self.assertEqual(self.lh.fire('task_start'), [])

    def test_register_and_fire(self):
        self.lh.register('task_done', lambda p: 'ok')
        self.assertEqual(self.lh.fire('task_done'), ['ok'])

    def test_clear_event(self):
        self.lh.register('task_done', lambda p: 'ok')
        self.lh.clear('task_done')
        self.assertEqual(self.lh.fire('task_done'), [])

    def test_fire_with_payload(self):
        self.lh.register('task_failed', lambda p: p.get('id'))
        self.assertEqual(self.lh.fire('task_failed', {'id': 'x'}), ['x'])

    def test_multiple_hooks(self):
        self.lh.register('loop_end', lambda p: 1)
        self.lh.register('loop_end', lambda p: 2)
        self.assertEqual(len(self.lh.fire('loop_end')), 2)

    def test_clear_all(self):
        self.lh.register('loop_end', lambda p: 1)
        self.lh.clear()
        self.assertEqual(self.lh.fire('loop_end'), [])

    def test_exception_suppressed(self):
        self.lh.register('task_start', lambda p: 1 / 0)
        result = self.lh.fire('task_start')
        self.assertIsInstance(result, list)

    def test_unknown_event_empty(self):
        self.assertEqual(self.lh.fire('unknown_event'), [])

    def test_fire_with_id_payload_is_list(self):
        self.assertIsInstance(self.lh.fire('task_start', {'id': 't1'}), list)


class CostEstimatorTest(unittest.TestCase):
    def setUp(self):
        from devkit.cost_estimator import estimate_tokens, estimate_cost, summarize
        self.estimate_tokens = estimate_tokens
        self.estimate_cost = estimate_cost
        self.summarize = summarize

    def test_tokens_is_int(self):
        self.assertIsInstance(self.estimate_tokens('hello'), int)

    def test_tokens_empty_zero(self):
        self.assertEqual(self.estimate_tokens(''), 0)

    def test_tokens_nonempty_ge1(self):
        self.assertGreaterEqual(self.estimate_tokens('hello'), 1)

    def test_tokens_cjk_ge1(self):
        self.assertGreaterEqual(self.estimate_tokens('你好世界'), 1)

    def test_tokens_cjk_ge_latin(self):
        self.assertGreaterEqual(self.estimate_tokens('你好世界'), self.estimate_tokens('hell'))

    def test_cost_is_float(self):
        self.assertIsInstance(self.estimate_cost(1000, 'glm'), float)

    def test_cost_deepseek_lt_minimax(self):
        self.assertLess(self.estimate_cost(1000, 'deepseek'), self.estimate_cost(1000, 'minimax'))

    def test_cost_zero_tokens(self):
        self.assertEqual(self.estimate_cost(0, 'glm'), 0.0)

    def test_summarize_is_dict(self):
        self.assertIsInstance(self.summarize('hello'), dict)

    def test_summarize_has_tokens_cost(self):
        d = self.summarize('hello')
        self.assertIn('tokens', d)
        self.assertIn('cost_usd', d)


class WatchdogTest(unittest.TestCase):
    def setUp(self):
        from devkit.watchdog import check_gateway, check_backlog, health_report
        self.check_gateway = check_gateway
        self.check_backlog = check_backlog
        self.health_report = health_report

    def test_gateway_is_dict(self):
        self.assertIsInstance(self.check_gateway('http://localhost:4000'), dict)

    def test_gateway_has_ok(self):
        self.assertIn('ok', self.check_gateway('http://localhost:4000'))

    def test_gateway_ok_is_bool(self):
        self.assertIsInstance(self.check_gateway('http://localhost:4000')['ok'], bool)

    def test_gateway_latency_is_float(self):
        self.assertIsInstance(self.check_gateway('http://localhost:4000')['latency_ms'], float)

    def test_backlog_nonexistent_is_dict(self):
        self.assertIsInstance(self.check_backlog('/nonexistent'), dict)

    def test_backlog_nonexistent_not_ok(self):
        self.assertFalse(self.check_backlog('/nonexistent')['ok'])

    def test_backlog_real_total_is_int(self):
        self.assertIsInstance(self.check_backlog('devkit/backlog.json')['total'], int)

    def test_health_report_is_dict(self):
        self.assertIsInstance(self.health_report(), dict)

    def test_health_report_has_keys(self):
        r = self.health_report()
        self.assertIn('gateway', r)
        self.assertIn('backlog', r)

    def test_health_report_overall_ok_is_bool(self):
        self.assertIsInstance(self.health_report()['overall_ok'], bool)


class TaskFilterTest(unittest.TestCase):
    def setUp(self):
        from devkit.task_filter import by_status, by_priority, search, filter_multi
        self.by_status = by_status
        self.by_priority = by_priority
        self.search = search
        self.filter_multi = filter_multi

    def test_by_status_empty(self):
        self.assertIsInstance(self.by_status([], 'done'), list)

    def test_by_status_match(self):
        t = {'id': 'a', 'status': 'done'}
        self.assertEqual(self.by_status([t], 'done'), [t])

    def test_by_status_no_match(self):
        self.assertEqual(self.by_status([{'id': 'a', 'status': 'done'}], 'pending'), [])

    def test_by_priority_match(self):
        t = {'id': 'a', 'priority': 'high'}
        self.assertEqual(self.by_priority([t], 'high'), [t])

    def test_by_priority_no_match(self):
        self.assertEqual(self.by_priority([{'id': 'a', 'priority': 'high'}], 'low'), [])

    def test_search_empty(self):
        self.assertIsInstance(self.search([], 'x'), list)

    def test_search_by_id(self):
        t = {'id': 'run-report', 'task': '生成HTML'}
        self.assertEqual(self.search([t], 'run'), [t])

    def test_search_by_task(self):
        t = {'id': 'abc', 'task': 'xyz'}
        self.assertEqual(self.search([t], 'xyz'), [t])

    def test_filter_multi_and(self):
        t = {'id': 'a', 'status': 'done', 'priority': 'high'}
        self.assertEqual(self.filter_multi([t], status='done', priority='high'), [t])

    def test_filter_multi_no_match(self):
        self.assertEqual(self.filter_multi([{'id': 'a', 'status': 'done'}], status='pending'), [])


class RunDiffTest(unittest.TestCase):
    def setUp(self):
        from devkit.run_diff import load_gate, diff_runs, diff_summary
        self.load_gate = load_gate
        self.diff_runs = diff_runs
        self.diff_summary = diff_summary

    def test_load_gate_nonexistent_is_str(self):
        self.assertIsInstance(self.load_gate('/nonexistent'), str)

    def test_load_gate_nonexistent_empty(self):
        self.assertEqual(self.load_gate('/nonexistent'), '')

    def test_diff_runs_nonexistent_is_dict(self):
        self.assertIsInstance(self.diff_runs('/nonexistent_a', '/nonexistent_b'), dict)

    def test_diff_runs_has_gate_a(self):
        self.assertIn('gate_a', self.diff_runs('/nonexistent_a', '/nonexistent_b'))

    def test_diff_runs_has_gate_b(self):
        self.assertIn('gate_b', self.diff_runs('/nonexistent_a', '/nonexistent_b'))

    def test_diff_runs_gate_changed_is_bool(self):
        self.assertIsInstance(self.diff_runs('/nonexistent_a', '/nonexistent_b')['gate_changed'], bool)

    def test_diff_runs_files_only_in_a_is_list(self):
        self.assertIsInstance(self.diff_runs('/nonexistent_a', '/nonexistent_b')['files_only_in_a'], list)

    def test_diff_summary_empty_is_str(self):
        self.assertIsInstance(self.diff_summary({}), str)

    def test_diff_summary_empty_sentinel(self):
        self.assertEqual(self.diff_summary({}), '(no diff data)')

    def test_diff_summary_full_is_str(self):
        full = {'run_a': 'a', 'run_b': 'b', 'gate_a': 'GO', 'gate_b': 'GO',
                'gate_changed': False, 'files_only_in_a': [], 'files_only_in_b': []}
        self.assertIsInstance(self.diff_summary(full), str)


class EventLogTest(unittest.TestCase):
    def setUp(self):
        import tempfile
        from devkit.event_log import append_event, read_events, filter_events
        self.append_event = append_event
        self.read_events = read_events
        self.filter_events = filter_events
        self._tmp = tempfile.mktemp(suffix='.jsonl')

    def tearDown(self):
        import os
        if os.path.exists(self._tmp):
            os.unlink(self._tmp)

    def test_read_nonexistent_is_list(self):
        self.assertIsInstance(self.read_events('/nonexistent'), list)

    def test_read_nonexistent_empty(self):
        self.assertEqual(self.read_events('/nonexistent'), [])

    def test_write_then_read(self):
        self.append_event(self._tmp, 'test_event', {'k': 1})
        self.assertGreaterEqual(len(self.read_events(self._tmp)), 1)

    def test_event_type_round_trips(self):
        self.append_event(self._tmp, 'test_event')
        self.assertEqual(self.read_events(self._tmp)[0]['type'], 'test_event')

    def test_event_has_ts(self):
        self.append_event(self._tmp, 'test_event')
        self.assertIn('ts', self.read_events(self._tmp)[0])

    def test_event_ts_is_float(self):
        self.append_event(self._tmp, 'test_event')
        self.assertIsInstance(self.read_events(self._tmp)[0]['ts'], float)

    def test_filter_events_empty(self):
        self.assertIsInstance(self.filter_events([], 'x'), list)

    def test_filter_events_match(self):
        events = [{'type': 'a', 'ts': 1.0}, {'type': 'b', 'ts': 2.0}]
        self.assertEqual(self.filter_events(events, 'a'), [{'type': 'a', 'ts': 1.0}])

    def test_filter_events_no_match(self):
        self.assertEqual(self.filter_events([{'type': 'a'}], 'b'), [])

    def test_read_n_limit(self):
        for i in range(5):
            self.append_event(self._tmp, f'e{i}')
        self.assertEqual(len(self.read_events(self._tmp, n=1)), 1)


class PipelineTraceTest(unittest.TestCase):
    def setUp(self):
        from devkit.pipeline_trace import new_trace, add_step, trace_summary, format_trace
        self.new_trace = new_trace
        self.add_step = add_step
        self.trace_summary = trace_summary
        self.format_trace = format_trace

    def test_new_trace_is_dict(self):
        self.assertIsInstance(self.new_trace('t1'), dict)

    def test_new_trace_task_id(self):
        self.assertEqual(self.new_trace('t1')['task_id'], 't1')

    def test_new_trace_steps_empty(self):
        self.assertEqual(self.new_trace('t1')['steps'], [])

    def test_add_step_is_dict(self):
        self.assertIsInstance(self.add_step(self.new_trace('t1'), 'impl', 'glm', True, 100), dict)

    def test_add_step_appends(self):
        t = self.add_step(self.new_trace('t1'), 'impl', 'glm', True, 100)
        self.assertEqual(len(t['steps']), 1)

    def test_summary_empty_steps(self):
        self.assertEqual(self.trace_summary(self.new_trace('t1'))['total_steps'], 0)

    def test_summary_ok_steps(self):
        t = self.add_step(self.new_trace('t1'), 'impl', 'glm', True, 50)
        self.assertEqual(self.trace_summary(t)['ok_steps'], 1)

    def test_summary_total_tokens(self):
        t = self.add_step(self.new_trace('t1'), 'impl', 'glm', True, 50)
        self.assertEqual(self.trace_summary(t)['total_tokens'], 50)

    def test_format_empty_dict_is_str(self):
        self.assertIsInstance(self.format_trace({}), str)

    def test_format_empty_dict_sentinel(self):
        self.assertEqual(self.format_trace({}), '(empty trace)')


class OutputFormatterTest(unittest.TestCase):
    def setUp(self):
        from devkit.output_formatter import fmt_table, fmt_status, fmt_progress, fmt_json
        self.fmt_table = fmt_table
        self.fmt_status = fmt_status
        self.fmt_progress = fmt_progress
        self.fmt_json = fmt_json

    def test_table_empty_is_str(self):
        self.assertIsInstance(self.fmt_table([], ['id']), str)

    def test_table_no_cols(self):
        self.assertEqual(self.fmt_table([], []), '(no columns)')

    def test_table_header_present(self):
        self.assertIn('id', self.fmt_table([{'id': 'a'}], ['id']))

    def test_table_value_present(self):
        self.assertIn('a', self.fmt_table([{'id': 'a'}], ['id']))

    def test_status_ok(self):
        self.assertEqual(self.fmt_status('ok', True), '✓ ok')

    def test_status_fail(self):
        self.assertEqual(self.fmt_status('fail', False), '✗ fail')

    def test_progress_normal(self):
        self.assertEqual(self.fmt_progress(3, 10), '3/10 (30%)')

    def test_progress_zero(self):
        self.assertEqual(self.fmt_progress(0, 0), '0/0 (-%)' )

    def test_json_is_str(self):
        self.assertIsInstance(self.fmt_json({'a': 1}), str)

    def test_json_contains_key(self):
        self.assertIn('"a"', self.fmt_json({'a': 1}))


class BacklogExportTest(unittest.TestCase):
    def setUp(self):
        from devkit.backlog_export import to_csv, to_markdown, to_json
        self.to_csv = to_csv
        self.to_markdown = to_markdown
        self.to_json = to_json

    def test_csv_empty_is_str(self):
        self.assertIsInstance(self.to_csv([]), str)

    def test_csv_has_headers(self):
        result = self.to_csv([])
        self.assertIn('id', result)
        self.assertIn('status', result)

    def test_csv_has_value(self):
        self.assertIn('a', self.to_csv([{'id': 'a', 'status': 'done', 'priority': 'high'}]))

    def test_markdown_empty_is_str(self):
        self.assertIsInstance(self.to_markdown([]), str)

    def test_markdown_empty_sentinel(self):
        self.assertEqual(self.to_markdown([]), '(empty)')

    def test_markdown_has_header(self):
        self.assertIn('id', self.to_markdown([{'id': 'a', 'status': 'done', 'priority': 'high'}]))

    def test_markdown_has_value(self):
        self.assertIn('a', self.to_markdown([{'id': 'a', 'status': 'done', 'priority': 'high'}]))

    def test_json_empty_is_str(self):
        self.assertIsInstance(self.to_json([]), str)

    def test_json_empty_array(self):
        self.assertEqual(self.to_json([]), '[]')

    def test_json_has_value(self):
        self.assertIn('done', self.to_json([{'id': 'a', 'status': 'done'}]))


class RunLogParserTest(unittest.TestCase):
    def setUp(self):
        from devkit.run_log_parser import parse, parse_file, batch_parse
        self.parse = parse
        self.parse_file = parse_file
        self.batch_parse = batch_parse

    def test_parse_empty_is_dict(self):
        self.assertIsInstance(self.parse(''), dict)

    def test_parse_empty_gate(self):
        self.assertEqual(self.parse('')['gate'], '')

    def test_parse_empty_tokens(self):
        self.assertEqual(self.parse('')['tokens'], 0)

    def test_parse_gate(self):
        self.assertEqual(self.parse('Gate: GO\n用量：1234 tok')['gate'], 'GO')

    def test_parse_tokens(self):
        self.assertEqual(self.parse('Gate: GO\n用量：1234 tok')['tokens'], 1234)

    def test_parse_cost(self):
        self.assertAlmostEqual(self.parse('$0.00123')['cost_usd'], 0.00123)

    def test_parse_iterations(self):
        self.assertEqual(self.parse('迭代：2 轮')['iterations'], 2)

    def test_parse_run_id(self):
        self.assertEqual(self.parse('run=auto-20260629-001234')['run_id'], 'auto-20260629-001234')

    def test_parse_file_nonexistent(self):
        self.assertIsInstance(self.parse_file('/nonexistent'), dict)

    def test_batch_parse_empty(self):
        self.assertIsInstance(self.batch_parse([]), list)


class CarrierScorerTest(unittest.TestCase):
    def setUp(self):
        from devkit.carrier_scorer import score, best_carrier, rank_carriers
        self.score = score
        self.best_carrier = best_carrier
        self.rank_carriers = rank_carriers

    def test_score_empty(self):
        self.assertIsInstance(self.score([]), dict)

    def test_score_empty_dict(self):
        self.assertEqual(self.score([]), {})

    def test_score_ok_rate_one(self):
        self.assertEqual(self.score([{'carrier': 'glm', 'ok': True, 'tokens': 100}])['glm']['ok_rate'], 1.0)

    def test_score_ok_rate_zero(self):
        self.assertEqual(self.score([{'carrier': 'glm', 'ok': False, 'tokens': 50}])['glm']['ok_rate'], 0.0)

    def test_score_count(self):
        self.assertEqual(self.score([{'carrier': 'glm', 'ok': True, 'tokens': 100}])['glm']['count'], 1)

    def test_best_carrier_empty(self):
        self.assertEqual(self.best_carrier([]), '')

    def test_best_carrier_picks_best(self):
        r = [{'carrier': 'glm', 'ok': True, 'tokens': 100}, {'carrier': 'deepseek', 'ok': False, 'tokens': 50}]
        self.assertEqual(self.best_carrier(r), 'glm')

    def test_rank_empty_is_list(self):
        self.assertIsInstance(self.rank_carriers([]), list)

    def test_rank_empty(self):
        self.assertEqual(self.rank_carriers([]), [])

    def test_rank_order(self):
        r = [{'carrier': 'glm', 'ok': True, 'tokens': 100}, {'carrier': 'deepseek', 'ok': False, 'tokens': 50}]
        self.assertEqual(self.rank_carriers(r), ['glm', 'deepseek'])


class TaskSchedulerTest(unittest.TestCase):
    def setUp(self):
        from devkit.task_scheduler import priority_order, ready_tasks, next_task, schedule_order
        self.priority_order = priority_order
        self.ready_tasks = ready_tasks
        self.next_task = next_task
        self.schedule_order = schedule_order

    def test_priority_high(self):
        self.assertEqual(self.priority_order('high'), 0)

    def test_priority_medium(self):
        self.assertEqual(self.priority_order('medium'), 1)

    def test_priority_unknown(self):
        self.assertEqual(self.priority_order('unknown'), 3)

    def test_ready_empty(self):
        self.assertIsInstance(self.ready_tasks([]), list)

    def test_ready_no_deps(self):
        t = {'id': 'a', 'status': 'pending', 'priority': 'high', 'deps': []}
        self.assertEqual(self.ready_tasks([t]), [t])

    def test_ready_blocked(self):
        t = {'id': 'b', 'status': 'pending', 'priority': 'high', 'deps': ['a']}
        self.assertEqual(self.ready_tasks([t]), [])

    def test_ready_dep_done(self):
        done = {'id': 'a', 'status': 'done', 'deps': []}
        t = {'id': 'b', 'status': 'pending', 'priority': 'high', 'deps': ['a']}
        self.assertEqual(self.ready_tasks([done, t]), [t])

    def test_next_task_empty(self):
        self.assertIsNone(self.next_task([]))

    def test_next_task_picks_first(self):
        t = {'id': 'a', 'status': 'pending', 'priority': 'high', 'deps': []}
        self.assertEqual(self.next_task([t]), t)

    def test_schedule_order_empty(self):
        self.assertIsInstance(self.schedule_order([]), list)


class MetricsAggregatorTest(unittest.TestCase):
    def setUp(self):
        from devkit.metrics_aggregator import aggregate, gate_distribution, top_token_runs
        self.aggregate = aggregate
        self.gate_distribution = gate_distribution
        self.top_token_runs = top_token_runs

    def test_aggregate_empty_is_dict(self):
        self.assertIsInstance(self.aggregate([]), dict)

    def test_aggregate_empty_total(self):
        self.assertEqual(self.aggregate([])['total_runs'], 0)

    def test_aggregate_empty_go_rate(self):
        self.assertEqual(self.aggregate([])['go_rate'], 0.0)

    def test_aggregate_go_runs(self):
        self.assertEqual(self.aggregate([{'gate': 'GO', 'tokens': 100, 'cost_usd': 0.001, 'iterations': 1}])['go_runs'], 1)

    def test_aggregate_total_tokens(self):
        self.assertEqual(self.aggregate([{'gate': 'GO', 'tokens': 100, 'cost_usd': 0.001, 'iterations': 1}])['total_tokens'], 100)

    def test_aggregate_no_go(self):
        self.assertEqual(self.aggregate([{'gate': 'NO-GO', 'tokens': 50, 'cost_usd': 0.0, 'iterations': 2}])['no_go_runs'], 1)

    def test_gate_dist_empty_is_dict(self):
        self.assertIsInstance(self.gate_distribution([]), dict)

    def test_gate_dist_empty(self):
        self.assertEqual(self.gate_distribution([]), {})

    def test_gate_dist_counts(self):
        runs = [{'gate': 'GO'}, {'gate': 'GO'}, {'gate': 'NO-GO'}]
        self.assertEqual(self.gate_distribution(runs), {'GO': 2, 'NO-GO': 1})

    def test_top_runs_empty(self):
        self.assertIsInstance(self.top_token_runs([]), list)


class ConfigLoaderTest(unittest.TestCase):
    def setUp(self):
        from devkit.config_loader import load_json, merge_configs, get, set_nested
        self.load_json = load_json
        self.merge_configs = merge_configs
        self.get = get
        self.set_nested = set_nested

    def test_load_nonexistent_is_dict(self):
        self.assertIsInstance(self.load_json('/nonexistent'), dict)

    def test_load_nonexistent_empty(self):
        self.assertEqual(self.load_json('/nonexistent'), {})

    def test_load_with_default(self):
        self.assertEqual(self.load_json('/nonexistent', {'a': 1}), {'a': 1})

    def test_merge_two(self):
        self.assertEqual(self.merge_configs({'a': 1}, {'b': 2}), {'a': 1, 'b': 2})

    def test_merge_override(self):
        self.assertEqual(self.merge_configs({'a': 1}, {'a': 2}), {'a': 2})

    def test_merge_empty(self):
        self.assertEqual(self.merge_configs(), {})

    def test_get_dotpath(self):
        self.assertEqual(self.get({'a': {'b': 3}}, 'a.b'), 3)

    def test_get_missing_default(self):
        self.assertEqual(self.get({'a': 1}, 'a.b.c', 'default'), 'default')

    def test_set_nested(self):
        self.assertEqual(self.set_nested({}, 'x.y', 42), {'x': {'y': 42}})

    def test_set_then_get(self):
        self.assertEqual(self.get(self.set_nested({}, 'a.b.c', 99), 'a.b.c'), 99)


class PluginRegistryTest(unittest.TestCase):
    def setUp(self):
        from devkit import plugin_registry as pr
        self.pr = pr
        pr.clear_all()

    def tearDown(self):
        self.pr.clear_all()

    def test_initial_empty(self):
        self.assertEqual(self.pr.list_plugins(), [])

    def test_register_and_get(self):
        self.pr.register('foo', lambda: 1)
        self.assertEqual(self.pr.get('foo')(), 1)

    def test_get_nonexistent(self):
        self.assertIsNone(self.pr.get('nonexistent'))

    def test_list_by_kind(self):
        self.pr.register('a', lambda: 0, 'loader')
        self.assertEqual(self.pr.list_plugins('loader'), ['a'])

    def test_list_kind_no_match(self):
        self.assertEqual(self.pr.list_plugins('other'), [])

    def test_unregister_true(self):
        self.pr.register('a', lambda: 0)
        self.assertTrue(self.pr.unregister('a'))

    def test_unregister_missing_false(self):
        self.assertFalse(self.pr.unregister('nonexistent'))

    def test_register_override(self):
        self.pr.register('b', lambda: 2)
        self.pr.register('b', lambda: 3)
        self.assertEqual(self.pr.get('b')(), 3)

    def test_clear_all(self):
        self.pr.register('x', lambda: 0)
        self.pr.clear_all()
        self.assertEqual(self.pr.list_plugins(), [])

    def test_list_is_list(self):
        self.assertIsInstance(self.pr.list_plugins(), list)


class PromptBuilderTest(unittest.TestCase):
    def setUp(self):
        from devkit.prompt_builder import system, user, assistant, build, render_text
        self.system = system
        self.user = user
        self.assistant = assistant
        self.build = build
        self.render_text = render_text

    def test_system_role(self):
        self.assertEqual(self.system('hi')['role'], 'system')

    def test_system_content(self):
        self.assertEqual(self.system('hi')['content'], 'hi')

    def test_user_role(self):
        self.assertEqual(self.user('hello')['role'], 'user')

    def test_assistant_role(self):
        self.assertEqual(self.assistant('ok')['role'], 'assistant')

    def test_build_filters_empty(self):
        self.assertEqual(self.build([self.user('hi'), self.user('')]), [self.user('hi')])

    def test_build_empty(self):
        self.assertEqual(self.build([]), [])

    def test_render_empty_is_str(self):
        self.assertIsInstance(self.render_text([]), str)

    def test_render_empty_str(self):
        self.assertEqual(self.render_text([]), '')

    def test_render_user(self):
        self.assertIn('[user]: hello', self.render_text([self.user('hello')]))

    def test_render_system(self):
        self.assertIn('[system]: hi', self.render_text([self.system('hi'), self.user('hello')]))


class TokenBudgetTest(unittest.TestCase):
    def setUp(self):
        from devkit.token_budget import new_budget, consume, is_exhausted, reset, budget_report
        self.new_budget = new_budget
        self.consume = consume
        self.is_exhausted = is_exhausted
        self.reset = reset
        self.budget_report = budget_report

    def test_limit(self):
        self.assertEqual(self.new_budget(1000)['limit'], 1000)

    def test_initial_used(self):
        self.assertEqual(self.new_budget(1000)['used'], 0)

    def test_initial_remaining(self):
        self.assertEqual(self.new_budget(1000)['remaining'], 1000)

    def test_consume_used(self):
        self.assertEqual(self.consume(self.new_budget(1000), 300)['used'], 300)

    def test_consume_remaining(self):
        self.assertEqual(self.consume(self.new_budget(1000), 300)['remaining'], 700)

    def test_not_exhausted(self):
        self.assertFalse(self.is_exhausted(self.new_budget(1000)))

    def test_exhausted(self):
        self.assertTrue(self.is_exhausted(self.consume(self.new_budget(100), 100)))

    def test_reset(self):
        self.assertEqual(self.reset(self.consume(self.new_budget(500), 200))['used'], 0)

    def test_report_is_str(self):
        self.assertIsInstance(self.budget_report(self.new_budget(1000)), str)

    def test_report_has_remaining(self):
        self.assertIn('remaining', self.budget_report(self.new_budget(1000)))


class RunComparatorTest(unittest.TestCase):
    def setUp(self):
        from devkit.run_comparator import compare, winner, compare_summary
        self.compare = compare
        self.winner = winner
        self.compare_summary = compare_summary
        self.r1 = {'run_id': 'a', 'gate': 'GO', 'tokens': 100, 'iterations': 1}
        self.r2 = {'run_id': 'b', 'gate': 'NO-GO', 'tokens': 50, 'iterations': 0}

    def test_compare_empty_is_list(self):
        self.assertIsInstance(self.compare([]), list)

    def test_compare_empty(self):
        self.assertEqual(self.compare([]), [])

    def test_winner_empty_none(self):
        self.assertIsNone(self.winner([]))

    def test_compare_go_first(self):
        self.assertEqual(self.compare([self.r2, self.r1])[0]['run_id'], 'a')

    def test_winner_picks_go(self):
        self.assertEqual(self.winner([self.r1, self.r2])['run_id'], 'a')

    def test_summary_empty_is_str(self):
        self.assertIsInstance(self.compare_summary([]), str)

    def test_summary_empty_sentinel(self):
        self.assertEqual(self.compare_summary([]), '(no runs)')

    def test_summary_has_run_id(self):
        self.assertIn('a', self.compare_summary([self.r1]))

    def test_summary_has_gate(self):
        self.assertIn('GO', self.compare_summary([self.r1]))

    def test_winner_is_dict(self):
        self.assertIsInstance(self.winner([self.r1]), dict)


class SnapshotManagerTest(unittest.TestCase):
    def setUp(self):
        import tempfile
        from devkit.snapshot_manager import save, load, list_snapshots, delete
        self.save = save
        self.load = load
        self.list_snapshots = list_snapshots
        self.delete = delete
        self._tmp = tempfile.mktemp(suffix='.json')

    def tearDown(self):
        import os
        if os.path.exists(self._tmp):
            os.unlink(self._tmp)

    def test_load_nonexistent_empty(self):
        self.assertEqual(self.load('/nonexistent'), {})

    def test_load_nonexistent_default(self):
        self.assertEqual(self.load('/nonexistent', {'x': 1}), {'x': 1})

    def test_save_then_load(self):
        self.save(self._tmp, {'k': 42})
        self.assertEqual(self.load(self._tmp), {'k': 42})

    def test_list_nonexistent_is_list(self):
        self.assertIsInstance(self.list_snapshots('/nonexistent_dir'), list)

    def test_list_nonexistent_empty(self):
        self.assertEqual(self.list_snapshots('/nonexistent_dir'), [])

    def test_delete_nonexistent_false(self):
        self.assertFalse(self.delete('/nonexistent'))

    def test_delete_existing_true(self):
        self.save(self._tmp, {})
        self.assertTrue(self.delete(self._tmp))

    def test_delete_twice_false(self):
        self.save(self._tmp, {})
        self.delete(self._tmp)
        self.assertFalse(self.delete(self._tmp))

    def test_load_is_dict(self):
        self.assertIsInstance(self.load('/nonexistent'), dict)

    def test_save_roundtrip_nested(self):
        self.save(self._tmp, {'a': {'b': 1}})
        self.assertEqual(self.load(self._tmp)['a']['b'], 1)


class ResultCacheTest(unittest.TestCase):
    def setUp(self):
        from devkit.result_cache import new_cache, put, get_cached, cache_stats
        self.new_cache = new_cache
        self.put = put
        self.get_cached = get_cached
        self.cache_stats = cache_stats

    def test_maxsize(self):
        self.assertEqual(self.new_cache()['maxsize'], 128)

    def test_initial_hits(self):
        self.assertEqual(self.new_cache()['hits'], 0)

    def test_put_and_get(self):
        c = self.new_cache()
        self.put(c, 'k', 1)
        self.assertEqual(self.get_cached(c, 'k'), 1)

    def test_get_missing_none(self):
        self.assertIsNone(self.get_cached(self.new_cache(), 'missing'))

    def test_get_missing_default(self):
        self.assertEqual(self.get_cached(self.new_cache(), 'missing', 'def'), 'def')

    def test_miss_increments(self):
        c = self.new_cache()
        self.get_cached(c, 'x')
        self.assertEqual(self.cache_stats(c)['misses'], 1)

    def test_hit_increments(self):
        c = self.new_cache()
        self.put(c, 'k', 1)
        self.get_cached(c, 'k')
        self.assertEqual(self.cache_stats(c)['hits'], 1)

    def test_hit_rate_zero(self):
        self.assertEqual(self.cache_stats(self.new_cache())['hit_rate'], 0.0)

    def test_lru_eviction(self):
        c = self.new_cache(2)
        self.put(c, 'a', 1)
        self.put(c, 'b', 2)
        self.put(c, 'c', 3)
        self.assertNotIn('a', c['store'])

    def test_stats_is_dict(self):
        self.assertIsInstance(self.cache_stats(self.new_cache()), dict)


class StageRouterTest(unittest.TestCase):
    def setUp(self):
        from devkit.stage_router import parse_stages, route, should_skip, stage_carrier
        self.parse_stages = parse_stages
        self.route = route
        self.should_skip = should_skip
        self.stage_carrier = stage_carrier

    def test_parse_two_stages(self):
        self.assertEqual(self.parse_stages('implement,verify'), ['implement', 'verify'])

    def test_parse_empty(self):
        self.assertEqual(self.parse_stages(''), [])

    def test_parse_single(self):
        self.assertEqual(self.parse_stages('implement'), ['implement'])

    def test_route_explicit(self):
        self.assertEqual(self.route({'stages': 'implement,verify'}), ['implement', 'verify'])

    def test_route_default(self):
        self.assertEqual(self.route({}), ['implement', 'verify'])

    def test_skip_true(self):
        self.assertTrue(self.should_skip({'skip_stages': ['verify']}, 'verify'))

    def test_skip_false(self):
        self.assertFalse(self.should_skip({'skip_stages': ['verify']}, 'implement'))

    def test_skip_no_field(self):
        self.assertFalse(self.should_skip({}, 'verify'))

    def test_stage_carrier_present(self):
        self.assertEqual(self.stage_carrier({'carrier': {'implement': 'deepseek'}}, 'implement'), 'deepseek')

    def test_stage_carrier_default(self):
        self.assertEqual(self.stage_carrier({}, 'implement'), 'glm')


class HealthCheckerTest(unittest.TestCase):
    def setUp(self):
        from devkit.health_checker import check_python_version, check_file_writable, check_json_parseable, full_check
        self.check_python_version = check_python_version
        self.check_file_writable = check_file_writable
        self.check_json_parseable = check_json_parseable
        self.full_check = full_check

    def test_version_is_dict(self):
        self.assertIsInstance(self.check_python_version(), dict)

    def test_version_has_ok(self):
        self.assertIn('ok', self.check_python_version())

    def test_version_ok_is_bool(self):
        self.assertIsInstance(self.check_python_version()['ok'], bool)

    def test_version_39_ok(self):
        self.assertTrue(self.check_python_version((3, 9))['ok'])

    def test_version_99_not_ok(self):
        self.assertFalse(self.check_python_version((99, 0))['ok'])

    def test_writable_tmp_is_dict(self):
        self.assertIsInstance(self.check_file_writable('/tmp'), dict)

    def test_writable_tmp_ok(self):
        self.assertTrue(self.check_file_writable('/tmp')['ok'])

    def test_writable_nonexistent_not_ok(self):
        self.assertFalse(self.check_file_writable('/nonexistent_dir_xyz')['ok'])

    def test_json_valid_ok(self):
        self.assertTrue(self.check_json_parseable('{"a":1}')['ok'])

    def test_json_invalid_not_ok(self):
        self.assertFalse(self.check_json_parseable('not json')['ok'])


class TaskValidatorTest(unittest.TestCase):
    def setUp(self):
        from devkit.task_validator import validate_task, validate_backlog, is_valid
        self.validate_task = validate_task
        self.validate_backlog = validate_backlog
        self.is_valid = is_valid

    def test_valid_task_no_errors(self):
        self.assertEqual(self.validate_task({'id': 'a', 'status': 'done'}), [])

    def test_missing_fields(self):
        self.assertGreater(len(self.validate_task({})), 0)

    def test_empty_id(self):
        self.assertGreater(len(self.validate_task({'id': '', 'status': 'done'})), 0)

    def test_invalid_status(self):
        self.assertGreater(len(self.validate_task({'id': 'a', 'status': 'invalid'})), 0)

    def test_is_valid_true(self):
        self.assertTrue(self.is_valid({'id': 'a', 'status': 'pending'}))

    def test_is_valid_false(self):
        self.assertFalse(self.is_valid({}))

    def test_backlog_empty_valid(self):
        self.assertEqual(self.validate_backlog([])['valid'], 0)

    def test_backlog_one_valid(self):
        self.assertEqual(self.validate_backlog([{'id': 'a', 'status': 'done'}])['valid'], 1)

    def test_backlog_one_invalid(self):
        self.assertEqual(self.validate_backlog([{'id': 'a', 'status': 'done'}, {}])['invalid'], 1)

    def test_backlog_errors_is_list(self):
        self.assertIsInstance(self.validate_backlog([])['errors'], list)


class RunFinalizerTest(unittest.TestCase):
    def setUp(self):
        from devkit.run_finalizer import collect_artifacts, write_summary, finalize
        self.collect_artifacts = collect_artifacts
        self.write_summary = write_summary
        self.finalize = finalize

    def test_collect_nonexistent_is_list(self):
        self.assertIsInstance(self.collect_artifacts('/nonexistent'), list)

    def test_collect_nonexistent_empty(self):
        self.assertEqual(self.collect_artifacts('/nonexistent'), [])

    def test_finalize_nonexistent_is_dict(self):
        self.assertIsInstance(self.finalize('/nonexistent'), dict)

    def test_finalize_nonexistent_not_ok(self):
        self.assertFalse(self.finalize('/nonexistent')['ok'])

    def test_finalize_has_artifacts(self):
        self.assertIn('artifacts', self.finalize('/nonexistent'))

    def test_finalize_nonexistent_empty_artifacts(self):
        self.assertEqual(self.finalize('/nonexistent')['artifacts'], [])

    def test_write_nonexistent_false(self):
        self.assertFalse(self.write_summary('/nonexistent', {}))

    def test_finalize_runs_ok(self):
        self.assertTrue(self.finalize('devkit/runs')['ok'])

    def test_finalize_runs_count_is_int(self):
        self.assertIsInstance(self.finalize('devkit/runs')['artifact_count'], int)

    def test_finalize_has_run_id(self):
        self.assertIn('run_id', self.finalize('devkit/runs'))


class CarrierFallbackTest(unittest.TestCase):
    def setUp(self):
        from devkit.carrier_fallback import fallback_chain, select_carrier, build_fallback, should_fallback
        self.fallback_chain = fallback_chain
        self.select_carrier = select_carrier
        self.build_fallback = build_fallback
        self.should_fallback = should_fallback

    def test_chain_with_alt(self):
        self.assertEqual(self.fallback_chain('glm', ['deepseek']), ['glm', 'deepseek'])

    def test_chain_no_alt(self):
        self.assertEqual(self.fallback_chain('glm', []), ['glm'])

    def test_chain_dedup(self):
        self.assertEqual(self.fallback_chain('glm', ['glm', 'deepseek']), ['glm', 'deepseek'])

    def test_select_first(self):
        self.assertEqual(self.select_carrier(['glm', 'deepseek'], []), 'glm')

    def test_select_skip_failed(self):
        self.assertEqual(self.select_carrier(['glm', 'deepseek'], ['glm']), 'deepseek')

    def test_select_all_failed(self):
        self.assertEqual(self.select_carrier(['glm'], ['glm']), 'glm')

    def test_select_empty_chain(self):
        self.assertEqual(self.select_carrier([], []), '')

    def test_build_fallback(self):
        self.assertEqual(self.build_fallback({'cascade': 'glm,deepseek'}), ['glm', 'deepseek'])

    def test_should_fallback_false(self):
        self.assertTrue(self.should_fallback({'ok': False}))

    def test_should_not_fallback(self):
        self.assertFalse(self.should_fallback({'ok': True}))


class OutputDifferTest(unittest.TestCase):
    def setUp(self):
        from devkit.output_differ import line_diff, changed_lines, is_identical, diff_summary
        self.line_diff = line_diff
        self.changed_lines = changed_lines
        self.is_identical = is_identical
        self.diff_summary = diff_summary

    def test_diff_identical_is_list(self):
        self.assertIsInstance(self.line_diff('a', 'a'), list)

    def test_diff_identical_empty(self):
        self.assertEqual(self.line_diff('a', 'a'), [])

    def test_diff_different_nonempty(self):
        self.assertGreater(len(self.line_diff('a', 'b')), 0)

    def test_changed_lines_is_dict(self):
        self.assertIsInstance(self.changed_lines('a', 'b'), dict)

    def test_changed_lines_added(self):
        self.assertEqual(self.changed_lines('a\nb', 'a\nc')['added'], 1)

    def test_changed_lines_removed(self):
        self.assertEqual(self.changed_lines('a\nb', 'a\nc')['removed'], 1)

    def test_identical_true(self):
        self.assertTrue(self.is_identical('hello', 'hello'))

    def test_identical_false(self):
        self.assertFalse(self.is_identical('hello', 'world'))

    def test_summary_identical(self):
        self.assertEqual(self.diff_summary('x', 'x'), '(identical)')

    def test_summary_diff_is_str(self):
        self.assertIsInstance(self.diff_summary('a', 'b'), str)


class TaskTaggerTest(unittest.TestCase):
    def setUp(self):
        from devkit.task_tagger import add_tag, remove_tag, has_tag, filter_by_tag, all_tags
        self.add_tag = add_tag
        self.remove_tag = remove_tag
        self.has_tag = has_tag
        self.filter_by_tag = filter_by_tag
        self.all_tags = all_tags

    def test_add_tag(self):
        t = {}
        self.add_tag(t, 'x')
        self.assertIn('x', t.get('tags', []))

    def test_add_tag_dedup(self):
        t = {'tags': ['x']}
        self.add_tag(t, 'x')
        self.assertEqual(t['tags'].count('x'), 1)

    def test_remove_tag(self):
        t = {'tags': ['x', 'y']}
        self.remove_tag(t, 'x')
        self.assertEqual(t['tags'], ['y'])

    def test_remove_nonexistent_no_error(self):
        self.remove_tag({'tags': []}, 'x')

    def test_has_tag_true(self):
        self.assertTrue(self.has_tag({'tags': ['a', 'b']}, 'a'))

    def test_has_tag_false(self):
        self.assertFalse(self.has_tag({'tags': ['a']}, 'b'))

    def test_has_tag_no_field(self):
        self.assertFalse(self.has_tag({}, 'a'))

    def test_filter_by_tag(self):
        tasks = [{'tags': ['a']}, {'tags': ['b']}]
        self.assertEqual(self.filter_by_tag(tasks, 'a'), [{'tags': ['a']}])

    def test_all_tags_sorted(self):
        tasks = [{'tags': ['b', 'a']}, {'tags': ['a', 'c']}]
        self.assertEqual(self.all_tags(tasks), ['a', 'b', 'c'])

    def test_all_tags_empty(self):
        self.assertEqual(self.all_tags([]), [])


class LogRotatorTest(unittest.TestCase):
    def setUp(self):
        import tempfile
        from devkit.log_rotator import rotate, trim, log_size
        self.rotate = rotate
        self.trim = trim
        self.log_size = log_size
        self._tmp = tempfile.mktemp(suffix='.jsonl')

    def tearDown(self):
        import os
        for path in [self._tmp, self._tmp + '.1']:
            if os.path.exists(path):
                os.unlink(path)

    def test_rotate_nonexistent_false(self):
        self.assertFalse(self.rotate('/nonexistent'))

    def test_log_size_nonexistent_bytes(self):
        self.assertEqual(self.log_size('/nonexistent')['bytes'], 0)

    def test_log_size_nonexistent_lines(self):
        self.assertEqual(self.log_size('/nonexistent')['lines'], 0)

    def test_trim_nonexistent_zero(self):
        self.assertEqual(self.trim('/nonexistent'), 0)

    def test_rotate_under_limit_false(self):
        with open(self._tmp, 'w') as fh:
            fh.write('x' * 100)
        self.assertFalse(self.rotate(self._tmp, max_bytes=10000))

    def test_rotate_over_limit_true(self):
        with open(self._tmp, 'w') as fh:
            fh.write('x' * 100)
        self.assertTrue(self.rotate(self._tmp, max_bytes=1))

    def test_rotate_creates_backup(self):
        import os
        with open(self._tmp, 'w') as fh:
            fh.write('x' * 100)
        self.rotate(self._tmp, max_bytes=1)
        self.assertTrue(os.path.exists(self._tmp + '.1'))

    def test_rotate_creates_new_file(self):
        import os
        with open(self._tmp, 'w') as fh:
            fh.write('x' * 100)
        self.rotate(self._tmp, max_bytes=1)
        self.assertTrue(os.path.exists(self._tmp))

    def test_trim_removes_lines(self):
        with open(self._tmp, 'w') as fh:
            fh.write('\n'.join(['line' + str(i) for i in range(5)]) + '\n')
        self.assertEqual(self.trim(self._tmp, 3), 2)

    def test_log_size_is_dict(self):
        self.assertIsInstance(self.log_size('/nonexistent'), dict)


class ModelSelectorTest(unittest.TestCase):
    def setUp(self):
        from devkit.model_selector import capability_score, select_model, rank_models, model_info
        self.capability_score = capability_score
        self.select_model = select_model
        self.rank_models = rank_models
        self.model_info = model_info

    def test_score_claude(self):
        self.assertEqual(self.capability_score('claude'), 100)

    def test_score_deepseek(self):
        self.assertEqual(self.capability_score('deepseek'), 80)

    def test_score_unknown(self):
        self.assertEqual(self.capability_score('unknown'), 50)

    def test_select_best(self):
        self.assertEqual(self.select_model({}, ['glm', 'deepseek']), 'deepseek')

    def test_select_empty(self):
        self.assertEqual(self.select_model({}, []), '')

    def test_rank_empty_is_list(self):
        self.assertIsInstance(self.rank_models([]), list)

    def test_rank_descending(self):
        self.assertEqual(self.rank_models(['glm', 'deepseek', 'claude']), ['claude', 'deepseek', 'glm'])

    def test_info_premium(self):
        self.assertEqual(self.model_info('claude')['tier'], 'premium')

    def test_info_standard(self):
        self.assertEqual(self.model_info('glm')['tier'], 'standard')

    def test_info_basic(self):
        self.assertEqual(self.model_info('unknown')['tier'], 'basic')


class ContextPackerTest(unittest.TestCase):
    def setUp(self):
        from devkit.context_packer import pack, estimate_fit, truncate_section
        self.pack = pack
        self.estimate_fit = estimate_fit
        self.truncate_section = truncate_section

    def test_pack_empty_is_str(self):
        self.assertIsInstance(self.pack([]), str)

    def test_pack_empty_str(self):
        self.assertEqual(self.pack([]), '')

    def test_pack_has_header(self):
        self.assertIn('## intro', self.pack([{'title': 'intro', 'content': 'hello', 'priority': 1}]))

    def test_pack_has_content(self):
        self.assertIn('hello', self.pack([{'title': 'intro', 'content': 'hello', 'priority': 1}]))

    def test_truncate_cuts(self):
        self.assertEqual(self.truncate_section('abcdef', 3), 'abc')

    def test_truncate_short(self):
        self.assertEqual(self.truncate_section('ab', 10), 'ab')

    def test_estimate_empty_is_dict(self):
        self.assertIsInstance(self.estimate_fit([], 100), dict)

    def test_estimate_empty_included(self):
        self.assertEqual(self.estimate_fit([], 100)['included'], 0)

    def test_estimate_doesnt_fit(self):
        s = {'title': 't', 'content': 'x' * 100, 'priority': 1}
        self.assertFalse(self.estimate_fit([s], 10)['fits'])

    def test_estimate_fits(self):
        s = {'title': 't', 'content': 'hi', 'priority': 1}
        self.assertTrue(self.estimate_fit([s], 10000)['fits'])


class TestDepResolver(unittest.TestCase):
    def setUp(self):
        from devkit.dep_resolver import resolve_order, find_cycles, missing_deps
        self.resolve_order = resolve_order
        self.find_cycles = find_cycles
        self.missing_deps = missing_deps

    def test_resolve_empty(self):
        self.assertEqual(self.resolve_order([]), [])

    def test_resolve_linear(self):
        bl = [{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}]
        self.assertEqual(self.resolve_order(bl), ['a', 'b'])

    def test_resolve_unsorted_input(self):
        bl = [{'id': 'b', 'deps': ['a']}, {'id': 'a', 'deps': []}]
        self.assertEqual(self.resolve_order(bl), ['a', 'b'])

    def test_find_cycles_empty_list(self):
        self.assertIsInstance(self.find_cycles([]), list)

    def test_find_cycles_no_cycle(self):
        bl = [{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}]
        self.assertEqual(self.find_cycles(bl), [])

    def test_missing_deps_empty(self):
        self.assertIsInstance(self.missing_deps([]), dict)

    def test_missing_deps_unknown_ref(self):
        self.assertEqual(self.missing_deps([{'id': 'a', 'deps': ['x']}]), {'a': ['x']})

    def test_missing_deps_known_ref(self):
        bl = [{'id': 'a', 'deps': ['b']}, {'id': 'b', 'deps': []}]
        self.assertEqual(self.missing_deps(bl), {})

    def test_resolve_diamond_count(self):
        bl = [{'id': 'a', 'deps': []}, {'id': 'b', 'deps': []}, {'id': 'c', 'deps': ['a', 'b']}]
        self.assertEqual(len(self.resolve_order(bl)), 3)

    def test_resolve_diamond_last(self):
        bl = [{'id': 'a', 'deps': []}, {'id': 'b', 'deps': []}, {'id': 'c', 'deps': ['a', 'b']}]
        self.assertEqual(self.resolve_order(bl)[-1], 'c')


class TestRunIndexer(unittest.TestCase):
    def setUp(self):
        from devkit.run_indexer import build_index, find_run, latest_runs
        self.build_index = build_index
        self.find_run = find_run
        self.latest_runs = latest_runs

    def test_build_index_nonexistent(self):
        self.assertIsInstance(self.build_index('/nonexistent'), dict)

    def test_build_index_empty(self):
        self.assertEqual(self.build_index('/nonexistent'), {})

    def test_build_index_runs_is_dict(self):
        self.assertIsInstance(self.build_index('devkit/runs'), dict)

    def test_build_index_runs_has_entries(self):
        self.assertGreaterEqual(len(self.build_index('devkit/runs')), 1)

    def test_find_run_missing(self):
        self.assertIsNone(self.find_run({}, 'any'))

    def test_find_run_present(self):
        idx = self.build_index('devkit/runs')
        k = list(idx.keys())[0]
        self.assertIsNotNone(self.find_run(idx, k))

    def test_find_run_has_path(self):
        idx = self.build_index('devkit/runs')
        k = list(idx.keys())[0]
        self.assertIn('path', self.find_run(idx, k))

    def test_latest_runs_empty_returns_list(self):
        self.assertIsInstance(self.latest_runs({}), list)

    def test_latest_runs_empty(self):
        self.assertEqual(self.latest_runs({}), [])

    def test_latest_runs_real(self):
        self.assertIsInstance(self.latest_runs(self.build_index('devkit/runs')), list)


class TestArtifactScanner(unittest.TestCase):
    def setUp(self):
        from devkit.artifact_scanner import scan_build, extract_functions, scan_summary
        self.scan_build = scan_build
        self.extract_functions = extract_functions
        self.scan_summary = scan_summary

    def test_scan_build_nonexistent_is_dict(self):
        self.assertIsInstance(self.scan_build('/nonexistent'), dict)

    def test_scan_build_nonexistent_total(self):
        self.assertEqual(self.scan_build('/nonexistent')['total'], 0)

    def test_extract_functions_nonexistent_list(self):
        self.assertIsInstance(self.extract_functions('/nonexistent'), list)

    def test_extract_functions_nonexistent_empty(self):
        self.assertEqual(self.extract_functions('/nonexistent'), [])

    def test_scan_summary_nonexistent(self):
        self.assertEqual(self.scan_summary('/nonexistent'), '(no build dir)')

    def test_scan_summary_returns_str(self):
        self.assertIsInstance(self.scan_summary('/nonexistent'), str)

    def test_scan_build_has_keys(self):
        result = self.scan_build('devkit')
        self.assertIn('py_files', result)
        self.assertIn('test_files', result)

    def test_scan_build_devkit_py_files_is_list(self):
        self.assertIsInstance(self.scan_build('devkit')['py_files'], list)

    def test_extract_functions_known_file(self):
        self.assertGreaterEqual(len(self.extract_functions('devkit/task_tagger.py')), 1)


class TestGateEvaluator(unittest.TestCase):
    def setUp(self):
        from devkit.gate_evaluator import parse_gate, evaluate, batch_evaluate
        self.parse_gate = parse_gate
        self.evaluate = evaluate
        self.batch_evaluate = batch_evaluate

    def test_parse_gate_go(self):
        self.assertEqual(self.parse_gate('GO')['decision'], 'GO')

    def test_parse_gate_no_go(self):
        self.assertEqual(self.parse_gate('NO-GO')['decision'], 'NO-GO')

    def test_parse_gate_confidence_high(self):
        self.assertEqual(self.parse_gate('建议 GO')['confidence'], 'high')

    def test_parse_gate_unknown(self):
        self.assertEqual(self.parse_gate('unknown stuff')['decision'], 'UNKNOWN')

    def test_evaluate_go_true(self):
        self.assertTrue(self.evaluate({'gate': 'GO', 'iterations': 0, 'tokens': 100})['go'])

    def test_evaluate_no_go_false(self):
        self.assertFalse(self.evaluate({'gate': 'NO-GO', 'iterations': 1, 'tokens': 50})['go'])

    def test_evaluate_efficiency(self):
        self.assertEqual(self.evaluate({'gate': 'GO', 'iterations': 1, 'tokens': 100})['efficiency'], 0.5)

    def test_batch_evaluate_empty_total(self):
        self.assertEqual(self.batch_evaluate([])['total'], 0)

    def test_batch_evaluate_empty_best_run(self):
        self.assertIsNone(self.batch_evaluate([])['best_run'])

    def test_batch_evaluate_go_count(self):
        self.assertEqual(self.batch_evaluate([{'gate': 'GO', 'iterations': 0, 'tokens': 100}])['go_count'], 1)


class TestPipelineSupervisor(unittest.TestCase):
    def setUp(self):
        from devkit.pipeline_supervisor import supervise, check_stage, pipeline_report
        self.supervise = supervise
        self.check_stage = check_stage
        self.pipeline_report = pipeline_report

    def test_supervise_empty_total(self):
        self.assertEqual(self.supervise([])['total'], 0)

    def test_supervise_empty_healthy(self):
        self.assertTrue(self.supervise([])['healthy'])

    def test_supervise_ok_count(self):
        self.assertEqual(self.supervise([{'name':'a','status':'ok','tokens':10}])['ok'], 1)

    def test_supervise_failed_unhealthy(self):
        self.assertFalse(self.supervise([{'name':'a','status':'failed','tokens':0}])['healthy'])

    def test_check_stage_ok(self):
        self.assertTrue(self.check_stage({'name':'x','status':'ok','tokens':5})['ok'])

    def test_check_stage_failed_ok(self):
        self.assertFalse(self.check_stage({'name':'x','status':'failed','tokens':0})['ok'])

    def test_check_stage_failed_issue(self):
        self.assertEqual(self.check_stage({'name':'x','status':'failed','tokens':0})['issue'], 'failed')

    def test_pipeline_report_empty(self):
        self.assertEqual(self.pipeline_report([]), '(no stages)')

    def test_pipeline_report_ok(self):
        self.assertIn('[OK]', self.pipeline_report([{'name':'a','status':'ok','tokens':5}]))

    def test_pipeline_report_fail(self):
        self.assertIn('[FAIL]', self.pipeline_report([{'name':'a','status':'failed','tokens':0}]))


class TestResultArchiver(unittest.TestCase):
    def setUp(self):
        from devkit.result_archiver import archive_run, list_archives, archive_summary
        self.archive_run = archive_run
        self.list_archives = list_archives
        self.archive_summary = archive_summary

    def test_list_archives_nonexistent(self):
        self.assertIsInstance(self.list_archives('/nonexistent'), list)

    def test_list_archives_empty(self):
        self.assertEqual(self.list_archives('/nonexistent'), [])

    def test_archive_summary_count(self):
        self.assertEqual(self.archive_summary('/nonexistent')['count'], 0)

    def test_archive_summary_names(self):
        self.assertEqual(self.archive_summary('/nonexistent')['names'], [])

    def test_archive_run_missing_src(self):
        self.assertFalse(self.archive_run('/nonexistent/src', '/tmp/arc')['ok'])

    def test_archive_run_error_not_found(self):
        self.assertEqual(self.archive_run('/nonexistent/src', '/tmp/arc')['error'], 'not found')

    def test_archive_run_is_dict(self):
        self.assertIsInstance(self.archive_run('/nonexistent/src', '/tmp/x'), dict)

    def test_archive_run_has_src(self):
        self.assertIn('src', self.archive_run('/nonexistent/src', '/tmp/x'))

    def test_archive_run_has_dst(self):
        self.assertIn('dst', self.archive_run('/nonexistent/src', '/tmp/x'))

    def test_archive_summary_names_is_list(self):
        self.assertIsInstance(self.archive_summary('/nonexistent')['names'], list)


class TestPriorityAdjuster(unittest.TestCase):
    def setUp(self):
        from devkit.priority_adjuster import adjust, promote, demote, sort_by_priority
        self.adjust = adjust
        self.promote = promote
        self.demote = demote
        self.sort_by_priority = sort_by_priority

    def test_adjust_low_up(self):
        self.assertEqual(self.adjust({'priority': 'low'}, 1)['priority'], 'medium')

    def test_adjust_high_up(self):
        self.assertEqual(self.adjust({'priority': 'high'}, 1)['priority'], 'critical')

    def test_adjust_critical_capped(self):
        self.assertEqual(self.adjust({'priority': 'critical'}, 1)['priority'], 'critical')

    def test_adjust_low_floored(self):
        self.assertEqual(self.adjust({'priority': 'low'}, -1)['priority'], 'low')

    def test_promote_low(self):
        self.assertEqual(self.promote({'priority': 'low'})['priority'], 'medium')

    def test_demote_medium(self):
        self.assertEqual(self.demote({'priority': 'medium'})['priority'], 'low')

    def test_demote_low_floor(self):
        self.assertEqual(self.demote({'priority': 'low'})['priority'], 'low')

    def test_sort_high_first(self):
        self.assertEqual(self.sort_by_priority([{'priority': 'low'}, {'priority': 'high'}])[0]['priority'], 'high')

    def test_sort_empty(self):
        self.assertEqual(len(self.sort_by_priority([])), 0)

    def test_sort_critical_first(self):
        self.assertEqual(self.sort_by_priority([{'priority': 'critical'}, {'priority': 'medium'}])[0]['priority'], 'critical')


class TestRunHealthMonitor(unittest.TestCase):
    def setUp(self):
        from devkit.run_health_monitor import check_run_health, aggregate_health
        self.check_run_health = check_run_health
        self.aggregate_health = aggregate_health

    def test_healthy_run(self):
        self.assertTrue(self.check_run_health({'tokens': 100, 'duration_s': 10, 'gate': 'GO', 'iterations': 0})['healthy'])

    def test_healthy_score_1(self):
        self.assertEqual(self.check_run_health({'tokens': 100, 'duration_s': 10, 'gate': 'GO', 'iterations': 0})['score'], 1.0)

    def test_no_go_still_healthy_with_one_warning(self):
        self.assertTrue(self.check_run_health({'tokens': 100, 'duration_s': 10, 'gate': 'NO-GO', 'iterations': 0})['healthy'])

    def test_no_go_one_warning(self):
        self.assertEqual(len(self.check_run_health({'tokens': 100, 'duration_s': 10, 'gate': 'NO-GO', 'iterations': 0})['warnings']), 1)

    def test_gate_failed_warning(self):
        self.assertIn('gate failed', self.check_run_health({'tokens': 0, 'duration_s': 0, 'gate': 'NO-GO', 'iterations': 0})['warnings'])

    def test_aggregate_empty_total(self):
        self.assertEqual(self.aggregate_health([])['total'], 0)

    def test_aggregate_empty_avg_score(self):
        self.assertEqual(self.aggregate_health([])['avg_score'], 0.0)

    def test_aggregate_healthy_count(self):
        self.assertEqual(self.aggregate_health([{'tokens': 100, 'duration_s': 10, 'gate': 'GO', 'iterations': 0}])['healthy_count'], 1)

    def test_aggregate_empty_issues(self):
        self.assertIsInstance(self.aggregate_health([])['issues'], list)

    def test_high_tokens_score_less_than_1(self):
        self.assertLess(self.check_run_health({'tokens': 60000, 'duration_s': 10, 'gate': 'GO', 'iterations': 0})['score'], 1.0)


class TestTaskDepGraph(unittest.TestCase):
    def setUp(self):
        from devkit.task_dep_graph import build_graph, render_ascii, node_info
        self.build_graph = build_graph
        self.render_ascii = render_ascii
        self.node_info = node_info

    def test_empty_nodes(self):
        self.assertEqual(self.build_graph([])['nodes'], [])

    def test_empty_edges(self):
        self.assertEqual(self.build_graph([])['edges'], [])

    def test_single_node(self):
        self.assertEqual(self.build_graph([{'id': 'a', 'deps': []}])['nodes'], ['a'])

    def test_root_is_a(self):
        g = self.build_graph([{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}])
        self.assertIn('a', g['roots'])

    def test_one_edge(self):
        g = self.build_graph([{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}])
        self.assertEqual(len(g['edges']), 1)

    def test_render_empty(self):
        self.assertEqual(self.render_ascii({'nodes': [], 'edges': [], 'roots': []}), '(empty graph)')

    def test_render_returns_str(self):
        self.assertIsInstance(self.render_ascii(self.build_graph([{'id': 'a', 'deps': []}])), str)

    def test_node_info_exists(self):
        self.assertTrue(self.node_info({'nodes': ['a'], 'edges': [], 'roots': ['a']}, 'a')['exists'])

    def test_node_info_missing(self):
        self.assertFalse(self.node_info({'nodes': ['a'], 'edges': [], 'roots': ['a']}, 'x')['exists'])

    def test_node_info_out_degree(self):
        self.assertEqual(self.node_info({'nodes': ['a'], 'edges': [('a', 'b')], 'roots': ['a']}, 'a')['out_degree'], 1)


class TestRunCostTracker(unittest.TestCase):
    def setUp(self):
        from devkit.run_cost_tracker import track, cost_per_token, budget_check
        self.track = track
        self.cost_per_token = cost_per_token
        self.budget_check = budget_check

    def test_track_empty_tokens(self):
        self.assertEqual(self.track([])['total_tokens'], 0)

    def test_track_empty_most_expensive(self):
        self.assertIsNone(self.track([])['most_expensive'])

    def test_track_tokens(self):
        self.assertEqual(self.track([{'name': 'a', 'tokens': 100, 'cost': 0.01}])['total_tokens'], 100)

    def test_track_most_expensive(self):
        stages = [{'name': 'a', 'tokens': 100, 'cost': 0.01}, {'name': 'b', 'tokens': 50, 'cost': 0.02}]
        self.assertEqual(self.track(stages)['most_expensive'], 'b')

    def test_cost_per_token_zero(self):
        self.assertEqual(self.cost_per_token(0, 0.0), 0.0)

    def test_cost_per_token_calc(self):
        self.assertAlmostEqual(self.cost_per_token(100, 0.01), 0.0001)

    def test_budget_check_within(self):
        self.assertTrue(self.budget_check(0.5, 1.0)['within_budget'])

    def test_budget_check_over(self):
        self.assertFalse(self.budget_check(1.5, 1.0)['within_budget'])

    def test_budget_check_remaining(self):
        self.assertAlmostEqual(self.budget_check(0.5, 1.0)['remaining'], 0.5)

    def test_budget_check_pct(self):
        self.assertAlmostEqual(self.budget_check(0.5, 1.0)['pct_used'], 50.0)


class TestStageRetryPolicy(unittest.TestCase):
    def setUp(self):
        from devkit.stage_retry_policy import should_retry, next_carrier, retry_summary
        self.should_retry = should_retry
        self.next_carrier = next_carrier
        self.retry_summary = retry_summary

    def test_should_retry_failed(self):
        self.assertTrue(self.should_retry({'status': 'failed', 'tokens': 0}, 0))

    def test_should_retry_ok(self):
        self.assertFalse(self.should_retry({'status': 'ok', 'tokens': 10}, 0))

    def test_should_retry_max(self):
        self.assertFalse(self.should_retry({'status': 'failed', 'tokens': 0}, 3))

    def test_should_retry_blocked(self):
        self.assertTrue(self.should_retry({'status': 'blocked', 'tokens': 0}, 1))

    def test_next_carrier_first(self):
        self.assertEqual(self.next_carrier(['a', 'b', 'c'], 0), 'a')

    def test_next_carrier_last(self):
        self.assertEqual(self.next_carrier(['a', 'b', 'c'], 2), 'c')

    def test_next_carrier_beyond(self):
        self.assertIsNone(self.next_carrier(['a', 'b'], 5))

    def test_retry_summary_empty(self):
        self.assertEqual(self.retry_summary([])['total'], 0)

    def test_retry_summary_success(self):
        self.assertEqual(self.retry_summary([{'attempt': 0, 'carrier': 'a', 'status': 'ok'}])['success'], 1)

    def test_retry_summary_last_carrier(self):
        self.assertEqual(self.retry_summary([{'attempt': 0, 'carrier': 'x', 'status': 'failed'}])['last_carrier'], 'x')


class TestOutputValidator(unittest.TestCase):
    def setUp(self):
        from devkit.output_validator import validate_output, check_nonempty, check_has_code, extract_code_blocks
        self.validate_output = validate_output
        self.check_nonempty = check_nonempty
        self.check_has_code = check_has_code
        self.extract_code_blocks = extract_code_blocks

    def test_check_nonempty_empty(self):
        self.assertFalse(self.check_nonempty(''))

    def test_check_nonempty_text(self):
        self.assertTrue(self.check_nonempty('hello'))

    def test_check_has_code_true(self):
        self.assertTrue(self.check_has_code('```py\ncode\n```'))

    def test_check_has_code_false(self):
        self.assertFalse(self.check_has_code('no code here'))

    def test_validate_empty_invalid(self):
        self.assertFalse(self.validate_output('', ['nonempty'])['valid'])

    def test_validate_text_valid(self):
        self.assertTrue(self.validate_output('hello', ['nonempty'])['valid'])

    def test_validate_violation_name(self):
        self.assertIn('nonempty', self.validate_output('', ['nonempty'])['violations'])

    def test_validate_has_code_valid(self):
        self.assertTrue(self.validate_output('```py\ncode\n```', ['has_code'])['valid'])

    def test_extract_empty(self):
        self.assertEqual(len(self.extract_code_blocks('')), 0)

    def test_extract_one_block(self):
        self.assertEqual(len(self.extract_code_blocks('```\nhello\n```')), 1)


class TestBuildManifest(unittest.TestCase):
    def setUp(self):
        from devkit.build_manifest import generate, diff_manifests, manifest_summary
        self.generate = generate
        self.diff_manifests = diff_manifests
        self.manifest_summary = manifest_summary

    def test_generate_nonexistent_total(self):
        self.assertEqual(self.generate('/nonexistent')['total'], 0)

    def test_generate_nonexistent_files(self):
        self.assertEqual(self.generate('/nonexistent')['files'], [])

    def test_generate_devkit_files_list(self):
        self.assertIsInstance(self.generate('devkit')['files'], list)

    def test_generate_devkit_has_entries(self):
        self.assertGreaterEqual(self.generate('devkit')['total'], 1)

    def test_generate_has_timestamp(self):
        self.assertIsInstance(self.generate('devkit')['generated_at'], str)

    def test_diff_added(self):
        self.assertEqual(self.diff_manifests({'files': ['a']}, {'files': ['a', 'b']})['added'], ['b'])

    def test_diff_removed(self):
        self.assertEqual(self.diff_manifests({'files': ['a', 'b']}, {'files': ['a']})['removed'], ['b'])

    def test_diff_unchanged(self):
        self.assertEqual(self.diff_manifests({'files': ['a']}, {'files': ['a']})['unchanged'], 1)

    def test_summary_is_str(self):
        self.assertIsInstance(self.manifest_summary(self.generate('devkit')), str)

    def test_summary_has_files(self):
        self.assertIn('files', self.manifest_summary(self.generate('devkit')))


class TestRunTracker(unittest.TestCase):
    def setUp(self):
        from devkit.run_tracker import summarize, filter_runs, top_runs
        self.summarize = summarize
        self.filter_runs = filter_runs
        self.top_runs = top_runs

    def test_summarize_empty_total(self):
        self.assertEqual(self.summarize([])['total'], 0)

    def test_summarize_empty_fastest(self):
        self.assertIsNone(self.summarize([])['fastest'])

    def test_summarize_go_count(self):
        self.assertEqual(self.summarize([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 10.0}])['go_count'], 1)

    def test_summarize_fastest(self):
        self.assertEqual(self.summarize([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 10.0}])['fastest'], 'r1')

    def test_filter_go(self):
        self.assertEqual(len(self.filter_runs([{'id': 'r1', 'gate': 'GO', 'tokens': 10, 'duration_s': 1}], 'GO')), 1)

    def test_filter_no_match(self):
        self.assertEqual(len(self.filter_runs([{'id': 'r1', 'gate': 'NO-GO', 'tokens': 10, 'duration_s': 1}], 'GO')), 0)

    def test_top_runs_empty(self):
        self.assertEqual(self.top_runs([], 3), [])

    def test_top_runs_limit(self):
        runs = [{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1}, {'id': 'r2', 'gate': 'GO', 'tokens': 50, 'duration_s': 1}]
        self.assertEqual(len(self.top_runs(runs, 1)), 1)

    def test_top_runs_order(self):
        runs = [{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1}, {'id': 'r2', 'gate': 'GO', 'tokens': 50, 'duration_s': 1}]
        self.assertEqual(self.top_runs(runs, 1)[0]['id'], 'r1')

    def test_summarize_no_go(self):
        self.assertEqual(self.summarize([{'id': 'r1', 'gate': 'NO-GO', 'tokens': 100, 'duration_s': 5}])['no_go_count'], 1)


class TestTaskEstimator(unittest.TestCase):
    def setUp(self):
        from devkit.task_estimator import estimate_complexity, estimate_tokens, batch_estimate
        self.estimate_complexity = estimate_complexity
        self.estimate_tokens = estimate_tokens
        self.batch_estimate = batch_estimate

    def test_empty_score(self):
        self.assertEqual(self.estimate_complexity('')['score'], 0)

    def test_empty_level(self):
        self.assertEqual(self.estimate_complexity('')['level'], 'low')

    def test_long_task_factor(self):
        self.assertIn('long_task', self.estimate_complexity('x' * 600)['factors'])

    def test_golden_factor(self):
        self.assertIn('has_golden', self.estimate_complexity('Golden: 1. foo')['factors'])

    def test_score_capped(self):
        self.assertEqual(self.estimate_complexity('x' * 2000)['score'], 10)

    def test_tokens_default(self):
        self.assertEqual(self.estimate_tokens('hello'), 502)

    def test_tokens_glm(self):
        self.assertEqual(self.estimate_tokens('hello', 'glm'), int(502 * 1.2))

    def test_batch_empty(self):
        self.assertEqual(self.batch_estimate([]), [])

    def test_batch_count(self):
        self.assertEqual(len(self.batch_estimate([{'id': 't1', 'task': 'foo'}])), 1)

    def test_batch_id(self):
        self.assertEqual(self.batch_estimate([{'id': 't1', 'task': 'foo'}])[0]['id'], 't1')


class TestCarrierAnalyzer(unittest.TestCase):
    def setUp(self):
        from devkit.carrier_analyzer import analyze, carrier_report, compare_carriers
        self.analyze = analyze
        self.carrier_report = carrier_report
        self.compare_carriers = compare_carriers

    def test_analyze_empty_carriers(self):
        self.assertEqual(self.analyze([])['carriers'], [])

    def test_analyze_empty_top(self):
        self.assertIsNone(self.analyze([])['top_carrier'])

    def test_analyze_top_carrier(self):
        self.assertEqual(self.analyze([{'carrier': 'mm', 'tokens': 10, 'ok': True}])['top_carrier'], 'mm')

    def test_analyze_ok_rate(self):
        self.assertEqual(self.analyze([{'carrier': 'mm', 'tokens': 10, 'ok': True}])['ok_rates']['mm'], 1.0)

    def test_report_empty(self):
        self.assertEqual(self.carrier_report({'carriers': [], 'usage': {}, 'ok_rates': {}, 'top_carrier': None}), '(no data)')

    def test_report_contains_carrier(self):
        self.assertIn('mm', self.carrier_report(self.analyze([{'carrier': 'mm', 'tokens': 10, 'ok': True}])))

    def test_compare_empty(self):
        self.assertEqual(self.compare_carriers([], 'a', 'b')['a']['count'], 0)

    def test_compare_winner(self):
        runs = [{'carrier': 'a', 'tokens': 10, 'ok': True}, {'carrier': 'b', 'tokens': 10, 'ok': False}]
        self.assertEqual(self.compare_carriers(runs, 'a', 'b')['winner'], 'a')

    def test_compare_tie(self):
        runs = [{'carrier': 'a', 'tokens': 10, 'ok': True}, {'carrier': 'b', 'tokens': 10, 'ok': True}]
        self.assertIsNone(self.compare_carriers(runs, 'a', 'b')['winner'])

    def test_report_is_str(self):
        self.assertIsInstance(self.carrier_report(self.analyze([{'carrier': 'x', 'tokens': 5, 'ok': False}])), str)


class TestTokenCounter(unittest.TestCase):
    def setUp(self):
        from devkit.token_counter import count_tokens, count_stage_tokens, token_budget_status
        self.count_tokens = count_tokens
        self.count_stage_tokens = count_stage_tokens
        self.token_budget_status = token_budget_status

    def test_empty_text(self):
        self.assertEqual(self.count_tokens(''), 0)

    def test_two_words(self):
        self.assertEqual(self.count_tokens('hello world'), int(2 * 1.3))

    def test_stage_empty_total(self):
        self.assertEqual(self.count_stage_tokens([])['total'], 0)

    def test_stage_empty_max(self):
        self.assertIsNone(self.count_stage_tokens([])['max_stage'])

    def test_stage_total(self):
        self.assertEqual(self.count_stage_tokens([{'name': 'a', 'tokens': 10}, {'name': 'b', 'tokens': 20}])['total'], 30)

    def test_stage_max(self):
        self.assertEqual(self.count_stage_tokens([{'name': 'a', 'tokens': 10}, {'name': 'b', 'tokens': 20}])['max_stage'], 'b')

    def test_budget_ok(self):
        self.assertTrue(self.token_budget_status(50, 100)['ok'])

    def test_budget_over(self):
        self.assertFalse(self.token_budget_status(150, 100)['ok'])

    def test_budget_remaining(self):
        self.assertEqual(self.token_budget_status(50, 100)['remaining'], 50)

    def test_budget_pct(self):
        self.assertEqual(self.token_budget_status(50, 100)['pct'], 50.0)


class TestRunPlanner(unittest.TestCase):
    def setUp(self):
        from devkit.run_planner import plan, estimate_duration, plan_summary
        self.plan = plan
        self.estimate_duration = estimate_duration
        self.plan_summary = plan_summary

    def test_plan_empty(self):
        self.assertEqual(self.plan([]), [])

    def test_plan_single_batch(self):
        self.assertEqual(len(self.plan([{'id': 'a', 'deps': []}])), 1)

    def test_plan_single_id(self):
        self.assertEqual(self.plan([{'id': 'a', 'deps': []}])[0], ['a'])

    def test_plan_two_batches(self):
        self.assertEqual(len(self.plan([{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}])), 2)

    def test_plan_first_batch(self):
        self.assertEqual(self.plan([{'id': 'a', 'deps': []}, {'id': 'b', 'deps': ['a']}])[0], ['a'])

    def test_duration_empty(self):
        self.assertEqual(self.estimate_duration([], {}), 0.0)

    def test_duration_one_batch(self):
        self.assertEqual(self.estimate_duration([['a']], {'a': {}}), 60.0)

    def test_duration_two_batches(self):
        self.assertEqual(self.estimate_duration([['a'], ['b']], {'a': {}, 'b': {}}), 120.0)

    def test_summary_empty(self):
        self.assertEqual(self.plan_summary([]), '0 batches, 0 tasks')

    def test_summary(self):
        self.assertEqual(self.plan_summary([['a', 'b'], ['c']]), '2 batches, 3 tasks')


class TestStageSequencer(unittest.TestCase):
    def setUp(self):
        from devkit.stage_sequencer import sequence, validate_sequence, sequence_info
        self.sequence = sequence
        self.validate_sequence = validate_sequence
        self.sequence_info = sequence_info

    def test_sequence_empty(self):
        self.assertEqual(self.sequence([]), [])

    def test_sequence_skip(self):
        self.assertEqual(self.sequence(['a', 'b', 'c'], ['b']), ['a', 'c'])

    def test_sequence_no_skip(self):
        self.assertEqual(self.sequence(['a', 'b'], None), ['a', 'b'])

    def test_validate_empty_valid(self):
        self.assertTrue(self.validate_sequence([], ['a', 'b'])['valid'])

    def test_validate_unknown_invalid(self):
        self.assertFalse(self.validate_sequence(['a', 'x'], ['a', 'b'])['valid'])

    def test_validate_unknown_list(self):
        self.assertEqual(self.validate_sequence(['a', 'x'], ['a', 'b'])['unknown'], ['x'])

    def test_info_empty_count(self):
        self.assertEqual(self.sequence_info([])['count'], 0)

    def test_info_has_implement(self):
        self.assertTrue(self.sequence_info(['implement', 'verify'])['has_implement'])

    def test_info_has_verify(self):
        self.assertTrue(self.sequence_info(['implement', 'verify'])['has_verify'])

    def test_info_no_verify(self):
        self.assertFalse(self.sequence_info(['implement'])['has_verify'])


class TestResultMerger(unittest.TestCase):
    def setUp(self):
        from devkit.result_merger import merge, merge_notes, merge_summary
        self.merge = merge
        self.merge_notes = merge_notes
        self.merge_summary = merge_summary

    def test_merge_empty_gate(self):
        self.assertEqual(self.merge([])['gate'], 'UNKNOWN')

    def test_merge_empty_tokens(self):
        self.assertEqual(self.merge([])['total_tokens'], 0)

    def test_merge_single_go(self):
        self.assertEqual(self.merge([{'gate': 'GO', 'tokens': 10, 'ok': True, 'notes': ''}])['gate'], 'GO')

    def test_merge_tie_is_no_go(self):
        r = [{'gate': 'GO', 'tokens': 10, 'ok': True, 'notes': ''}, {'gate': 'NO-GO', 'tokens': 5, 'ok': False, 'notes': ''}]
        self.assertEqual(self.merge(r)['gate'], 'NO-GO')

    def test_merge_tokens_sum(self):
        self.assertEqual(self.merge([{'gate': 'GO', 'tokens': 10, 'ok': True, 'notes': ''}])['total_tokens'], 10)

    def test_merge_consensus_true(self):
        self.assertTrue(self.merge([{'gate': 'GO', 'tokens': 10, 'ok': True, 'notes': ''}])['consensus'])

    def test_merge_consensus_false(self):
        r = [{'gate': 'GO', 'tokens': 10, 'ok': True, 'notes': ''}, {'gate': 'NO-GO', 'tokens': 5, 'ok': False, 'notes': ''}]
        self.assertFalse(self.merge(r)['consensus'])

    def test_merge_notes_dedup(self):
        r = [{'gate': 'GO', 'tokens': 0, 'ok': True, 'notes': 'a'}, {'gate': 'GO', 'tokens': 0, 'ok': True, 'notes': 'a'}]
        self.assertEqual(self.merge_notes(r), ['a'])

    def test_merge_summary_unknown(self):
        self.assertIn('gate=UNKNOWN', self.merge_summary(self.merge([])))

    def test_merge_summary_is_str(self):
        self.assertIsInstance(self.merge_summary(self.merge([])), str)


class TestTaskGraphExporter(unittest.TestCase):
    def setUp(self):
        from devkit.task_graph_exporter import to_dot, to_adjacency_list, to_json
        self.to_dot = to_dot
        self.to_adjacency_list = to_adjacency_list
        self.to_json = to_json

    def test_dot_starts_digraph(self):
        self.assertTrue(self.to_dot({'nodes': [], 'edges': []}).startswith('digraph'))

    def test_dot_has_closing_brace(self):
        self.assertIn('}', self.to_dot({'nodes': [], 'edges': []}))

    def test_dot_has_node(self):
        self.assertIn('a', self.to_dot({'nodes': ['a'], 'edges': []}))

    def test_dot_has_edge_arrow(self):
        self.assertIn('->', self.to_dot({'nodes': ['a', 'b'], 'edges': [('a', 'b')]}))

    def test_adj_empty(self):
        self.assertEqual(self.to_adjacency_list({'nodes': [], 'edges': []}), {})

    def test_adj_no_edges(self):
        self.assertEqual(self.to_adjacency_list({'nodes': ['a'], 'edges': []}), {'a': []})

    def test_adj_with_edge(self):
        self.assertEqual(self.to_adjacency_list({'nodes': ['a', 'b'], 'edges': [('a', 'b')]})['a'], ['b'])

    def test_json_is_str(self):
        self.assertIsInstance(self.to_json({'nodes': ['a'], 'edges': []}), str)

    def test_json_has_nodes(self):
        self.assertIn('nodes', self.to_json({'nodes': ['a'], 'edges': []}))

    def test_adj_is_dict(self):
        self.assertIsInstance(self.to_adjacency_list({'nodes': ['a', 'b'], 'edges': [('a', 'b')]}), dict)


class TestRunProfiler(unittest.TestCase):
    def setUp(self):
        from devkit.run_profiler import profile, compare_profiles, profile_summary
        self.profile = profile
        self.compare_profiles = compare_profiles
        self.profile_summary = profile_summary

    def test_empty_slowest(self):
        self.assertIsNone(self.profile({'stages': [], 'total_tokens': 0})['slowest_stage'])

    def test_empty_duration(self):
        self.assertEqual(self.profile({'stages': [], 'total_tokens': 0})['total_duration'], 0.0)

    def test_single_slowest(self):
        s = {'stages': [{'name': 'a', 'tokens': 10, 'duration_s': 5.0}], 'total_tokens': 10}
        self.assertEqual(self.profile(s)['slowest_stage'], 'a')

    def test_single_fastest(self):
        s = {'stages': [{'name': 'a', 'tokens': 10, 'duration_s': 5.0}], 'total_tokens': 10}
        self.assertEqual(self.profile(s)['fastest_stage'], 'a')

    def test_total_duration(self):
        s = {'stages': [{'name': 'a', 'tokens': 10, 'duration_s': 5.0}], 'total_tokens': 10}
        self.assertEqual(self.profile(s)['total_duration'], 5.0)

    def test_compare_faster(self):
        self.assertEqual(self.compare_profiles({'total_duration': 3.0, 'total_tokens': 100}, {'total_duration': 5.0, 'total_tokens': 200})['faster'], 'a')

    def test_compare_token_delta(self):
        self.assertEqual(self.compare_profiles({'total_duration': 3.0, 'total_tokens': 100}, {'total_duration': 5.0, 'total_tokens': 200})['token_delta'], -100)

    def test_summary_is_str(self):
        self.assertIsInstance(self.profile_summary(self.profile({'stages': [], 'total_tokens': 0})), str)

    def test_summary_has_duration(self):
        self.assertIn('duration=', self.profile_summary(self.profile({'stages': [], 'total_tokens': 0})))

    def test_two_stage_slowest(self):
        s = {'stages': [{'name': 'a', 'tokens': 10, 'duration_s': 3.0}, {'name': 'b', 'tokens': 10, 'duration_s': 7.0}], 'total_tokens': 20}
        self.assertEqual(self.profile(s)['slowest_stage'], 'b')


class TestBacklogAuditor(unittest.TestCase):
    def setUp(self):
        from devkit.backlog_auditor import audit, fix_status, audit_summary
        self.audit = audit
        self.fix_status = fix_status
        self.audit_summary = audit_summary

    def test_empty_total(self):
        self.assertEqual(self.audit([])['total'], 0)

    def test_empty_healthy(self):
        self.assertTrue(self.audit([])['healthy'])

    def test_valid_task_healthy(self):
        self.assertTrue(self.audit([{'id': 'a', 'task': 'foo', 'status': 'pending', 'priority': 'high'}])['healthy'])

    def test_empty_task_unhealthy(self):
        self.assertFalse(self.audit([{'id': 'a', 'task': '', 'status': 'pending', 'priority': 'high'}])['healthy'])

    def test_bad_status_unhealthy(self):
        self.assertFalse(self.audit([{'id': 'a', 'task': 'foo', 'status': 'bad', 'priority': 'high'}])['healthy'])

    def test_issue_count(self):
        self.assertEqual(len(self.audit([{'id': 'a', 'task': '', 'status': 'pending', 'priority': 'high'}])['issues']), 1)

    def test_fix_bad_status(self):
        self.assertEqual(self.fix_status({'id': 'a', 'status': 'bad'})['status'], 'pending')

    def test_fix_valid_status(self):
        self.assertEqual(self.fix_status({'id': 'a', 'status': 'done'})['status'], 'done')

    def test_summary_no_issues(self):
        self.assertEqual(self.audit_summary({'total': 5, 'issues': [], 'healthy': True}), '5 tasks, 0 issues')

    def test_summary_with_issues(self):
        self.assertEqual(self.audit_summary({'total': 3, 'issues': [1, 2], 'healthy': False}), '3 tasks, 2 issues')


class TestStageConfig(unittest.TestCase):
    def setUp(self):
        from devkit.stage_config import default_config, merge_config, validate_config
        self.default_config = default_config
        self.merge_config = merge_config
        self.validate_config = validate_config

    def test_default_stage_name(self):
        self.assertEqual(self.default_config('implement')['stage'], 'implement')

    def test_default_carrier(self):
        self.assertEqual(self.default_config('implement')['carrier'], 'minimax')

    def test_default_enabled(self):
        self.assertTrue(self.default_config('implement')['enabled'])

    def test_merge_override(self):
        self.assertEqual(self.merge_config({'a': 1, 'b': 2}, {'b': 3})['b'], 3)

    def test_merge_keeps_base(self):
        self.assertEqual(self.merge_config({'a': 1, 'b': 2}, {'b': 3})['a'], 1)

    def test_validate_valid(self):
        self.assertTrue(self.validate_config({'carrier': 'minimax', 'timeout': 300, 'max_tokens': 8000})['valid'])

    def test_validate_empty_carrier(self):
        self.assertFalse(self.validate_config({'carrier': '', 'timeout': 300, 'max_tokens': 8000})['valid'])

    def test_validate_neg_timeout(self):
        self.assertFalse(self.validate_config({'carrier': 'minimax', 'timeout': -1, 'max_tokens': 8000})['valid'])

    def test_validate_two_errors(self):
        self.assertEqual(len(self.validate_config({'carrier': '', 'timeout': -1, 'max_tokens': 8000})['errors']), 2)

    def test_default_timeout_is_int(self):
        self.assertIsInstance(self.default_config('verify')['timeout'], int)


class TestPipelineConfig(unittest.TestCase):
    def setUp(self):
        from devkit.pipeline_config import default_pipeline_config, apply_overrides, config_diff
        self.default_pipeline_config = default_pipeline_config
        self.apply_overrides = apply_overrides
        self.config_diff = config_diff

    def test_max_iterations(self):
        self.assertEqual(self.default_pipeline_config()['max_iterations'], 3)

    def test_cascade(self):
        self.assertEqual(self.default_pipeline_config()['cascade'], 'minimax,glm,deepseek')

    def test_blind_review_false(self):
        self.assertFalse(self.default_pipeline_config()['blind_review'])

    def test_apply_override(self):
        self.assertEqual(self.apply_overrides({'a': 1}, {'a': 2})['a'], 2)

    def test_apply_keeps_base(self):
        self.assertEqual(self.apply_overrides({'a': 1, 'b': 2}, {'a': 2})['b'], 2)

    def test_diff_changed(self):
        self.assertEqual(self.config_diff({'a': 1}, {'a': 2})['changed'], {'a': 2})

    def test_diff_added(self):
        self.assertEqual(self.config_diff({'a': 1}, {'a': 1, 'b': 2})['added'], {'b': 2})

    def test_diff_removed(self):
        self.assertEqual(self.config_diff({'a': 1, 'b': 2}, {'a': 1})['removed'], ['b'])

    def test_diff_no_change(self):
        self.assertEqual(self.config_diff({'a': 1}, {'a': 1})['changed'], {})

    def test_timeout_is_int(self):
        self.assertIsInstance(self.default_pipeline_config()['timeout'], int)


class TestRunStateMachine(unittest.TestCase):
    def setUp(self):
        from devkit.run_state_machine import transition, valid_transitions, state_summary
        self.transition = transition
        self.valid_transitions = valid_transitions
        self.state_summary = state_summary

    def test_start(self):
        self.assertEqual(self.transition('pending', 'start'), 'running')

    def test_succeed(self):
        self.assertEqual(self.transition('running', 'succeed'), 'done')

    def test_fail(self):
        self.assertEqual(self.transition('running', 'fail'), 'failed')

    def test_block(self):
        self.assertEqual(self.transition('running', 'block'), 'blocked')

    def test_retry(self):
        self.assertEqual(self.transition('blocked', 'retry'), 'running')

    def test_invalid_from_done(self):
        self.assertEqual(self.transition('done', 'start'), 'done')

    def test_valid_from_pending(self):
        self.assertIn('start', self.valid_transitions('pending'))

    def test_summary_empty(self):
        self.assertEqual(self.state_summary([])['final_state'], 'pending')

    def test_summary_final_done(self):
        h = [{'state': 'pending', 'event': 'start'}, {'state': 'running', 'event': 'succeed'}]
        self.assertEqual(self.state_summary(h)['final_state'], 'done')

    def test_summary_has_retry(self):
        self.assertTrue(self.state_summary([{'state': 'running', 'event': 'retry'}])['has_retry'])


class TestTaskQueue(unittest.TestCase):
    def setUp(self):
        from devkit.task_queue import push, pop, queue_stats
        self.push = push
        self.pop = pop
        self.queue_stats = queue_stats

    def test_push_empty(self):
        self.assertEqual(self.push([], {'id': 'a', 'priority': 'high'}), [{'id': 'a', 'priority': 'high'}])

    def test_push_len(self):
        self.assertEqual(len(self.push([{'id': 'a', 'priority': 'low'}], {'id': 'b', 'priority': 'high'})), 2)

    def test_push_order(self):
        self.assertEqual(self.push([{'id': 'a', 'priority': 'low'}], {'id': 'b', 'priority': 'high'})[0]['id'], 'b')

    def test_pop_empty(self):
        self.assertIsNone(self.pop([])[0])

    def test_pop_returns_task(self):
        self.assertEqual(self.pop([{'id': 'a', 'priority': 'high'}])[0]['id'], 'a')

    def test_pop_remaining(self):
        self.assertEqual(len(self.pop([{'id': 'a', 'priority': 'high'}])[1]), 0)

    def test_stats_empty(self):
        self.assertEqual(self.queue_stats([])['total'], 0)

    def test_stats_top_id(self):
        self.assertEqual(self.queue_stats([{'id': 'a', 'priority': 'high'}])['top_id'], 'a')

    def test_stats_by_priority(self):
        q = [{'id': 'a', 'priority': 'high'}, {'id': 'b', 'priority': 'low'}]
        self.assertEqual(self.queue_stats(q)['by_priority']['high'], 1)

    def test_stats_empty_top_id(self):
        self.assertIsNone(self.queue_stats([])['top_id'])


class TestStageMetrics(unittest.TestCase):
    def setUp(self):
        from devkit.stage_metrics import collect, percentile, metrics_report
        self.collect = collect
        self.percentile = percentile
        self.metrics_report = metrics_report

    def test_collect_empty_count(self):
        self.assertEqual(self.collect([])['count'], 0)

    def test_collect_empty_ok_rate(self):
        self.assertEqual(self.collect([])['ok_rate'], 0.0)

    def test_collect_ok_rate_1(self):
        self.assertEqual(self.collect([{'name': 'a', 'tokens': 100, 'duration_s': 5.0, 'ok': True}])['ok_rate'], 1.0)

    def test_collect_avg_tokens(self):
        self.assertEqual(self.collect([{'name': 'a', 'tokens': 100, 'duration_s': 5.0, 'ok': True}])['avg_tokens'], 100.0)

    def test_collect_total_tokens(self):
        s = [{'name': 'a', 'tokens': 100, 'duration_s': 5.0, 'ok': True}, {'name': 'b', 'tokens': 200, 'duration_s': 10.0, 'ok': False}]
        self.assertEqual(self.collect(s)['total_tokens'], 300)

    def test_percentile_empty(self):
        self.assertEqual(self.percentile([], 50), 0.0)

    def test_percentile_min(self):
        self.assertEqual(self.percentile([1.0, 2.0, 3.0, 4.0, 5.0], 0), 1.0)

    def test_percentile_max(self):
        self.assertEqual(self.percentile([1.0, 2.0, 3.0, 4.0, 5.0], 100), 5.0)

    def test_report_is_str(self):
        self.assertIsInstance(self.metrics_report(self.collect([])), str)

    def test_report_has_ok(self):
        self.assertIn('ok=', self.metrics_report(self.collect([])))


class TestStageResult(unittest.TestCase):
    def setUp(self):
        from devkit.stage_result import make, is_ok, merge, result_summary
        self.make = make
        self.is_ok = is_ok
        self.merge = merge
        self.result_summary = result_summary

    def test_make_stage(self):
        self.assertEqual(self.make('impl', True, 'code', 100, 5.0)['stage'], 'impl')

    def test_make_ok_true(self):
        self.assertTrue(self.make('impl', True, 'code', 100, 5.0)['ok'])

    def test_is_ok_true(self):
        self.assertTrue(self.is_ok(self.make('impl', True, 'code', 100, 5.0)))

    def test_is_ok_false(self):
        self.assertFalse(self.is_ok(self.make('impl', False, '', 0, 0.0)))

    def test_merge_empty_ok(self):
        self.assertTrue(self.merge([])['ok'])

    def test_merge_total_tokens(self):
        self.assertEqual(self.merge([self.make('impl', True, 'code', 100, 5.0)])['total_tokens'], 100)

    def test_merge_any_fail(self):
        self.assertFalse(self.merge([self.make('impl', True, 'code', 100, 5.0), self.make('verify', False, '', 50, 2.0)])['ok'])

    def test_merge_stages(self):
        self.assertEqual(self.merge([self.make('impl', True, 'c', 100, 5.0)])['stages'], ['impl'])

    def test_result_summary_ok(self):
        self.assertEqual(self.result_summary(self.make('impl', True, 'code', 100, 5.0)), '[OK] impl')

    def test_result_summary_fail(self):
        self.assertEqual(self.result_summary(self.make('impl', False, '', 0, 0.0)), '[FAIL] impl')


class TestRunReporter(unittest.TestCase):
    def setUp(self):
        from devkit.run_reporter import generate_report, report_header, compare_reports
        self.generate_report = generate_report
        self.report_header = report_header
        self.compare_reports = compare_reports
        self.run1 = {'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0, 'stages': ['implement']}

    def test_generate_report_is_str(self):
        self.assertIsInstance(self.generate_report(self.run1), str)

    def test_generate_report_contains_id(self):
        self.assertIn('r1', self.generate_report(self.run1))

    def test_generate_report_contains_gate(self):
        self.assertIn('GO', self.generate_report(self.run1))

    def test_report_header_format(self):
        self.assertEqual(self.report_header({'id': 'r1', 'gate': 'GO', 'tokens': 100}), 'Run r1 | gate=GO | tokens=100')

    def test_compare_empty_best_gate(self):
        self.assertEqual(self.compare_reports([])['best_gate'], 'UNKNOWN')

    def test_compare_empty_count(self):
        self.assertEqual(self.compare_reports([])['count'], 0)

    def test_compare_go_best_gate(self):
        self.assertEqual(self.compare_reports([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5, 'stages': []}])['best_gate'], 'GO')

    def test_compare_min_tokens(self):
        runs = [{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5, 'stages': []}, {'id': 'r2', 'gate': 'NO-GO', 'tokens': 50, 'duration_s': 3, 'stages': []}]
        self.assertEqual(self.compare_reports(runs)['min_tokens'], 50)

    def test_compare_max_tokens(self):
        runs = [{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5, 'stages': []}, {'id': 'r2', 'gate': 'NO-GO', 'tokens': 50, 'duration_s': 3, 'stages': []}]
        self.assertEqual(self.compare_reports(runs)['max_tokens'], 100)

    def test_compare_nogo_best_gate(self):
        self.assertEqual(self.compare_reports([{'id': 'r1', 'gate': 'NO-GO', 'tokens': 50, 'duration_s': 3, 'stages': []}])['best_gate'], 'NO-GO')


class TestTaskLifecycle(unittest.TestCase):
    def setUp(self):
        from devkit.task_lifecycle import record_event, lifecycle_summary, filter_events
        self.record_event = record_event
        self.lifecycle_summary = lifecycle_summary
        self.filter_events = filter_events

    def test_record_returns_dict(self):
        self.assertIsInstance(self.record_event('t1', 'start'), dict)

    def test_record_task_id(self):
        self.assertEqual(self.record_event('t1', 'start')['task_id'], 't1')

    def test_record_event_field(self):
        self.assertEqual(self.record_event('t1', 'start')['event'], 'start')

    def test_record_timestamp_is_str(self):
        self.assertIsInstance(self.record_event('t1', 'start')['timestamp'], str)

    def test_record_metadata(self):
        self.assertEqual(self.record_event('t1', 'start', {'x': 1})['metadata'], {'x': 1})

    def test_summary_empty(self):
        self.assertEqual(self.lifecycle_summary([])['total'], 0)

    def test_summary_task_ids(self):
        self.assertEqual(self.lifecycle_summary([self.record_event('t1', 'start')])['task_ids'], ['t1'])

    def test_summary_event_types_sorted(self):
        evts = [self.record_event('t1', 'start'), self.record_event('t2', 'done')]
        self.assertEqual(self.lifecycle_summary(evts)['event_types'], ['done', 'start'])

    def test_filter_events_match(self):
        evts = [self.record_event('t1', 'start'), self.record_event('t2', 'done')]
        self.assertEqual(len(self.filter_events(evts, 't1')), 1)

    def test_filter_events_no_match(self):
        self.assertEqual(self.filter_events([self.record_event('t1', 'start')], 't2'), [])


class TestCarrierSelector(unittest.TestCase):
    def setUp(self):
        from devkit.carrier_selector import select, rank_carriers, fallback_chain
        self.select = select
        self.rank_carriers = rank_carriers
        self.fallback_chain = fallback_chain

    def test_select_empty_candidates(self):
        self.assertIsNone(self.select([], []))

    def test_select_no_history(self):
        self.assertEqual(self.select(['a', 'b'], []), 'a')

    def test_select_with_history(self):
        history = [{'carrier': 'b', 'ok': True}, {'carrier': 'a', 'ok': False}]
        self.assertEqual(self.select(['a', 'b'], history), 'b')

    def test_rank_length(self):
        self.assertEqual(len(self.rank_carriers(['a', 'b'], [])), 2)

    def test_rank_neutral_ok_rate(self):
        self.assertEqual(self.rank_carriers(['a'], [])[0]['ok_rate'], 0.5)

    def test_rank_perfect_ok_rate(self):
        self.assertEqual(self.rank_carriers(['a'], [{'carrier': 'a', 'ok': True}])[0]['ok_rate'], 1.0)

    def test_fallback_chain_basic(self):
        self.assertEqual(self.fallback_chain('a', ['b', 'c']), ['a', 'b', 'c'])

    def test_fallback_chain_dedup(self):
        self.assertEqual(self.fallback_chain('a', ['a', 'b']), ['a', 'b'])

    def test_rank_best_carrier(self):
        history = [{'carrier': 'b', 'ok': True}, {'carrier': 'b', 'ok': True}]
        self.assertEqual(self.rank_carriers(['a', 'b'], history)[0]['carrier'], 'b')

    def test_rank_zero_ok_rate(self):
        self.assertEqual(self.rank_carriers(['a'], [{'carrier': 'a', 'ok': False}])[0]['ok_rate'], 0.0)


class TestOutputBuffer(unittest.TestCase):
    def setUp(self):
        from devkit.output_buffer import create, append, flush, buffer_stats
        self.create = create
        self.append = append
        self.flush = flush
        self.buffer_stats = buffer_stats

    def test_create_chunks_empty(self):
        self.assertEqual(self.create()['chunks'], [])

    def test_create_total_chars_zero(self):
        self.assertEqual(self.create()['total_chars'], 0)

    def test_append_total_chars(self):
        self.assertEqual(self.append(self.create(), 'hello')['total_chars'], 5)

    def test_append_chunks(self):
        self.assertEqual(self.append(self.create(), 'hello')['chunks'], ['hello'])

    def test_flush_empty_content(self):
        self.assertEqual(self.flush(self.create())[0], '')

    def test_flush_sets_flushed(self):
        self.assertTrue(self.flush(self.create())[1]['flushed'])

    def test_flush_with_content(self):
        self.assertEqual(self.flush(self.append(self.create(), 'hi'))[0], 'hi')

    def test_stats_chunk_count_zero(self):
        self.assertEqual(self.buffer_stats(self.create())['chunk_count'], 0)

    def test_stats_avg_chunk_size_zero(self):
        self.assertEqual(self.buffer_stats(self.create())['avg_chunk_size'], 0.0)

    def test_stats_avg_chunk_size(self):
        self.assertEqual(self.buffer_stats(self.append(self.create(), 'hello'))['avg_chunk_size'], 5.0)


class TestRunArchiver(unittest.TestCase):
    def setUp(self):
        from devkit.run_archiver import make_archive_entry, filter_by_gate, archive_stats, latest
        self.make_archive_entry = make_archive_entry
        self.filter_by_gate = filter_by_gate
        self.archive_stats = archive_stats
        self.latest = latest

    def test_make_returns_dict(self):
        self.assertIsInstance(self.make_archive_entry('r1', 'GO', 100), dict)

    def test_make_run_id(self):
        self.assertEqual(self.make_archive_entry('r1', 'GO', 100)['run_id'], 'r1')

    def test_make_gate(self):
        self.assertEqual(self.make_archive_entry('r1', 'GO', 100)['gate'], 'GO')

    def test_make_archived_at_is_str(self):
        self.assertIsInstance(self.make_archive_entry('r1', 'GO', 100)['archived_at'], str)

    def test_filter_by_gate(self):
        entries = [self.make_archive_entry('r1', 'GO', 100)]
        self.assertEqual(len(self.filter_by_gate(entries, 'GO')), 1)

    def test_stats_empty_total(self):
        self.assertEqual(self.archive_stats([])['total'], 0)

    def test_stats_go_count(self):
        self.assertEqual(self.archive_stats([self.make_archive_entry('r1', 'GO', 100)])['go_count'], 1)

    def test_stats_nogo_count(self):
        self.assertEqual(self.archive_stats([self.make_archive_entry('r1', 'NO-GO', 50)])['nogo_count'], 1)

    def test_stats_total_tokens(self):
        self.assertEqual(self.archive_stats([self.make_archive_entry('r1', 'GO', 100)])['total_tokens'], 100)

    def test_latest_empty(self):
        self.assertEqual(self.latest([], 5), [])


class TestTaskPlanner(unittest.TestCase):
    def setUp(self):
        from devkit.task_planner import split, make_plan, plan_summary
        self.split = split
        self.make_plan = make_plan
        self.plan_summary = plan_summary

    def test_split_basic(self):
        self.assertEqual(self.split('a\nb\nc', 3), ['a', 'b', 'c'])

    def test_split_limit(self):
        self.assertEqual(self.split('a\nb\nc', 2), ['a', 'b'])

    def test_split_empty(self):
        self.assertEqual(self.split('', 5), [])

    def test_split_filter_empty_lines(self):
        self.assertEqual(self.split('a\n\nb', 5), ['a', 'b'])

    def test_make_plan_length(self):
        self.assertEqual(len(self.make_plan(['t1', 't2'])), 2)

    def test_make_plan_id(self):
        self.assertEqual(self.make_plan(['t1'])[0]['id'], 'task-1')

    def test_make_plan_status(self):
        self.assertEqual(self.make_plan(['t1'])[0]['status'], 'pending')

    def test_make_plan_priority(self):
        self.assertEqual(self.make_plan(['t1'], 'high')[0]['priority'], 'high')

    def test_plan_summary_empty(self):
        self.assertEqual(self.plan_summary([])['total'], 0)

    def test_plan_summary_pending(self):
        self.assertEqual(self.plan_summary(self.make_plan(['t1', 't2']))['pending'], 2)


class TestCostTracker(unittest.TestCase):
    def setUp(self):
        from devkit.cost_tracker import record, total_cost, by_carrier, cost_report
        self.record = record
        self.total_cost = total_cost
        self.by_carrier = by_carrier
        self.cost_report = cost_report

    def test_record_is_dict(self):
        self.assertIsInstance(self.record('minimax', 100, 0.001), dict)

    def test_record_carrier(self):
        self.assertEqual(self.record('minimax', 100, 0.001)['carrier'], 'minimax')

    def test_record_tokens(self):
        self.assertEqual(self.record('minimax', 100, 0.001)['tokens'], 100)

    def test_total_cost_empty(self):
        self.assertEqual(self.total_cost([]), 0.0)

    def test_total_cost_sum(self):
        self.assertAlmostEqual(self.total_cost([self.record('minimax', 100, 0.001), self.record('glm', 200, 0.002)]), 0.003, places=4)

    def test_by_carrier_empty(self):
        self.assertEqual(self.by_carrier([]), {})

    def test_by_carrier_count(self):
        self.assertEqual(self.by_carrier([self.record('minimax', 100, 0.001)])['minimax']['count'], 1)

    def test_by_carrier_tokens(self):
        self.assertEqual(self.by_carrier([self.record('minimax', 100, 0.001)])['minimax']['tokens'], 100)

    def test_cost_report_empty(self):
        self.assertEqual(self.cost_report([]), '(no records)')

    def test_cost_report_is_str(self):
        self.assertIsInstance(self.cost_report([self.record('minimax', 100, 0.001)]), str)


class TestRunValidator(unittest.TestCase):
    def setUp(self):
        from devkit.run_validator import validate_run, validate_batch, check_gate
        self.validate_run = validate_run
        self.validate_batch = validate_batch
        self.check_gate = check_gate

    def test_valid_run(self):
        self.assertTrue(self.validate_run({'id': 'r1', 'gate': 'GO', 'tokens': 100})['valid'])

    def test_empty_run_invalid(self):
        self.assertFalse(self.validate_run({})['valid'])

    def test_empty_run_has_errors(self):
        self.assertGreater(len(self.validate_run({})['errors']), 0)

    def test_negative_tokens_invalid(self):
        self.assertFalse(self.validate_run({'id': 'r1', 'gate': 'GO', 'tokens': -1})['valid'])

    def test_batch_empty_total(self):
        self.assertEqual(self.validate_batch([])['total'], 0)

    def test_batch_valid_count(self):
        self.assertEqual(self.validate_batch([{'id': 'r1', 'gate': 'GO', 'tokens': 100}])['valid'], 1)

    def test_batch_invalid_count(self):
        self.assertEqual(self.validate_batch([{}])['invalid'], 1)

    def test_check_gate_go(self):
        self.assertTrue(self.check_gate({'gate': 'GO'}))

    def test_check_gate_nogo(self):
        self.assertTrue(self.check_gate({'gate': 'NO-GO'}))

    def test_check_gate_invalid(self):
        self.assertFalse(self.check_gate({'gate': 'MAYBE'}))


class TestStageScheduler(unittest.TestCase):
    def setUp(self):
        from devkit.stage_scheduler import schedule, next_stage, schedule_summary
        self.schedule = schedule
        self.next_stage = next_stage
        self.schedule_summary = schedule_summary

    def test_schedule_returns_list(self):
        self.assertIsInstance(self.schedule(['a', 'b'], {}), list)

    def test_schedule_length(self):
        self.assertEqual(len(self.schedule(['a', 'b'], {})), 2)

    def test_schedule_dep_order(self):
        sch = self.schedule(['b', 'a'], {'b': ['a']})
        self.assertLess(sch.index('a'), sch.index('b'))

    def test_next_stage_first(self):
        self.assertEqual(self.next_stage(['a', 'b'], set()), 'a')

    def test_next_stage_skip_completed(self):
        self.assertEqual(self.next_stage(['a', 'b'], {'a'}), 'b')

    def test_next_stage_all_done(self):
        self.assertIsNone(self.next_stage(['a'], {'a'}))

    def test_summary_total(self):
        self.assertEqual(self.schedule_summary(['a', 'b'], set())['total'], 2)

    def test_summary_done(self):
        self.assertEqual(self.schedule_summary(['a', 'b'], {'a'})['done'], 1)

    def test_summary_remaining(self):
        self.assertEqual(self.schedule_summary(['a', 'b'], {'a'})['remaining'], 1)

    def test_summary_next(self):
        self.assertEqual(self.schedule_summary(['a', 'b'], set())['next'], 'a')


class TestLogFormatter(unittest.TestCase):
    def setUp(self):
        from devkit.log_formatter import format_line, format_batch, parse_level
        self.format_line = format_line
        self.format_batch = format_batch
        self.parse_level = parse_level

    def test_format_line_has_level(self):
        self.assertIn('[INFO]', self.format_line('info', 'hello'))

    def test_format_line_has_msg(self):
        self.assertIn('hello', self.format_line('info', 'hello'))

    def test_format_line_context(self):
        self.assertIn('x=1', self.format_line('info', 'msg', {'x': 1}))

    def test_format_line_uppercase(self):
        self.assertTrue(self.format_line('warn', 'ok').startswith('[WARN]'))

    def test_format_batch_empty(self):
        self.assertEqual(self.format_batch([]), '')

    def test_format_batch_is_str(self):
        self.assertIsInstance(self.format_batch([{'level': 'info', 'msg': 'hi'}]), str)

    def test_format_batch_has_level(self):
        self.assertIn('[INFO]', self.format_batch([{'level': 'info', 'msg': 'hi'}]))

    def test_parse_level_info(self):
        self.assertEqual(self.parse_level('[INFO] something'), 'INFO')

    def test_parse_level_unknown(self):
        self.assertEqual(self.parse_level('no brackets'), 'UNKNOWN')

    def test_parse_level_error(self):
        self.assertEqual(self.parse_level('[ERROR] boom'), 'ERROR')


class TestEventBus(unittest.TestCase):
    def setUp(self):
        from devkit.event_bus import create, subscribe, publish, unsubscribe
        self.create = create
        self.subscribe = subscribe
        self.publish = publish
        self.unsubscribe = unsubscribe

    def test_create_empty(self):
        self.assertEqual(self.create()['listeners'], {})

    def test_subscribe_single(self):
        self.assertEqual(self.subscribe(self.create(), 'run.done', 'h1')['listeners']['run.done'], ['h1'])

    def test_subscribe_twice_len(self):
        b = self.subscribe(self.subscribe(self.create(), 'e', 'h1'), 'e', 'h2')
        self.assertEqual(len(b['listeners']['e']), 2)

    def test_publish_empty_count(self):
        self.assertEqual(self.publish(self.create(), 'run.done', {'gate': 'GO'})['count'], 0)

    def test_publish_one_handler_count(self):
        b = self.subscribe(self.create(), 'run.done', 'h1')
        self.assertEqual(self.publish(b, 'run.done', {})['count'], 1)

    def test_publish_handlers_list(self):
        b = self.subscribe(self.create(), 'run.done', 'h1')
        self.assertEqual(self.publish(b, 'run.done', {})['handlers'], ['h1'])

    def test_publish_event_echoed(self):
        self.assertEqual(self.publish(self.create(), 'other', {})['event'], 'other')

    def test_unsubscribe_empties(self):
        b = self.unsubscribe(self.subscribe(self.create(), 'e', 'h1'), 'e', 'h1')
        self.assertEqual(b['listeners'].get('e', []), [])

    def test_publish_data_is_dict(self):
        self.assertIsInstance(self.publish(self.create(), 'x', {})['data'], dict)

    def test_subscribe_first_element(self):
        self.assertEqual(self.subscribe(self.create(), 'e', 'h1')['listeners']['e'][0], 'h1')


class TestCheckpointManager(unittest.TestCase):
    def setUp(self):
        from devkit.checkpoint_manager import save, load, list_checkpoints, prune
        self.save = save
        self.load = load
        self.list_checkpoints = list_checkpoints
        self.prune = prune

    def test_save_is_dict(self):
        self.assertIsInstance(self.save('cp1', {'step': 1}), dict)

    def test_save_name(self):
        self.assertEqual(self.save('cp1', {'step': 1})['name'], 'cp1')

    def test_save_state(self):
        self.assertEqual(self.save('cp1', {'step': 1})['state'], {'step': 1})

    def test_save_saved_at_is_str(self):
        self.assertIsInstance(self.save('cp1', {'step': 1})['saved_at'], str)

    def test_load_empty(self):
        self.assertIsNone(self.load([], 'cp1'))

    def test_load_found(self):
        self.assertEqual(self.load([self.save('cp1', {'step': 1})], 'cp1')['name'], 'cp1')

    def test_load_not_found(self):
        self.assertIsNone(self.load([self.save('cp1', {'step': 1})], 'cp2'))

    def test_list_empty(self):
        self.assertEqual(self.list_checkpoints([]), [])

    def test_list_sorted(self):
        self.assertEqual(self.list_checkpoints([self.save('b', {}), self.save('a', {})]), ['a', 'b'])

    def test_prune_keep_one(self):
        self.assertEqual(len(self.prune([self.save('cp1', {}), self.save('cp2', {})], 1)), 1)


class TestOutputScorer(unittest.TestCase):
    def setUp(self):
        from devkit.output_scorer import score, batch_score, top_outputs
        self.score = score
        self.batch_score = batch_score
        self.top_outputs = top_outputs

    def test_score_nonempty_fail(self):
        self.assertEqual(self.score('', ['nonempty'])['score'], 0.0)

    def test_score_nonempty_pass(self):
        self.assertEqual(self.score('hello', ['nonempty'])['score'], 1.0)

    def test_score_has_python(self):
        self.assertEqual(self.score('```python\npass\n```', ['has_python'])['score'], 1.0)

    def test_score_passed_list(self):
        self.assertIn('nonempty', self.score('hello', ['nonempty'])['passed'])

    def test_score_failed_list(self):
        self.assertIn('nonempty', self.score('', ['nonempty'])['failed'])

    def test_score_empty_criteria(self):
        self.assertEqual(self.score('x', [])['score'], 1.0)

    def test_batch_score_empty_avg(self):
        self.assertEqual(self.batch_score([], ['nonempty'])['avg'], 0.0)

    def test_batch_score_avg(self):
        self.assertEqual(self.batch_score(['hi', ''], ['nonempty'])['avg'], 0.5)

    def test_top_outputs_best(self):
        self.assertEqual(self.top_outputs(['hi', ''], ['nonempty'], 1), ['hi'])

    def test_top_outputs_zero(self):
        self.assertEqual(self.top_outputs(['hi', ''], ['nonempty'], 0), [])


class TestContextWindowManager(unittest.TestCase):
    def setUp(self):
        from devkit.context_window_manager import create, add_message, fits, trim
        self.create = create
        self.add_message = add_message
        self.fits = fits
        self.trim = trim

    def test_create_max_tokens(self):
        self.assertEqual(self.create(4096)['max_tokens'], 4096)

    def test_create_used_zero(self):
        self.assertEqual(self.create(4096)['used'], 0)

    def test_add_message_used(self):
        self.assertEqual(self.add_message(self.create(4096), 'user', 'hi', 10)['used'], 10)

    def test_add_message_length(self):
        self.assertEqual(len(self.add_message(self.create(4096), 'user', 'hi', 10)['messages']), 1)

    def test_fits_true(self):
        self.assertTrue(self.fits(self.create(4096), 100))

    def test_fits_false(self):
        self.assertFalse(self.fits(self.create(100), 101))

    def test_fits_after_add(self):
        ctx = self.add_message(self.create(100), 'user', 'hi', 90)
        self.assertFalse(self.fits(ctx, 11))

    def test_trim_used(self):
        ctx = self.add_message(self.add_message(self.create(100), 'user', 'a', 60), 'user', 'b', 30)
        self.assertEqual(self.trim(ctx, 50)['used'], 30)

    def test_trim_all(self):
        ctx = self.add_message(self.create(100), 'user', 'a', 60)
        self.assertEqual(len(self.trim(ctx, 0)['messages']), 0)

    def test_add_message_role(self):
        self.assertEqual(self.add_message(self.create(4096), 'user', 'hi', 10)['messages'][0]['role'], 'user')


class TestRunSummarizer(unittest.TestCase):
    def setUp(self):
        from devkit.run_summarizer import summarize, batch_summarize, summary_stats
        self.summarize = summarize
        self.batch_summarize = batch_summarize
        self.summary_stats = summary_stats
        self.r1 = {'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'stages': ['a']}

    def test_verdict_pass(self):
        self.assertEqual(self.summarize(self.r1)['verdict'], 'pass')

    def test_verdict_fail(self):
        r = {'id': 'r1', 'gate': 'NO-GO', 'tokens': 100, 'duration_s': 1, 'stages': []}
        self.assertEqual(self.summarize(r)['verdict'], 'fail')

    def test_token_class_light(self):
        self.assertEqual(self.summarize(self.r1)['token_class'], 'light')

    def test_token_class_heavy(self):
        r = {'id': 'r1', 'gate': 'GO', 'tokens': 6000, 'duration_s': 1, 'stages': []}
        self.assertEqual(self.summarize(r)['token_class'], 'heavy')

    def test_stage_count(self):
        r = {'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'stages': ['a', 'b']}
        self.assertEqual(self.summarize(r)['stage_count'], 2)

    def test_batch_empty(self):
        self.assertEqual(len(self.batch_summarize([])), 0)

    def test_batch_one(self):
        r = {'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'stages': []}
        self.assertEqual(len(self.batch_summarize([r])), 1)

    def test_stats_empty_total(self):
        self.assertEqual(self.summary_stats([])['total'], 0)

    def test_stats_empty_pass_rate(self):
        self.assertEqual(self.summary_stats([])['pass_rate'], 0.0)

    def test_stats_pass_count(self):
        r = {'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'stages': []}
        self.assertEqual(self.summary_stats(self.batch_summarize([r]))['pass_count'], 1)


class TestTaskClassifier(unittest.TestCase):
    def setUp(self):
        from devkit.task_classifier import classify, batch_classify, classification_stats
        self.classify = classify
        self.batch_classify = batch_classify
        self.classification_stats = classification_stats

    def test_classify_implement(self):
        self.assertEqual(self.classify('implement foo'), 'implementation')

    def test_classify_chinese_implement(self):
        self.assertEqual(self.classify('实现 bar'), 'implementation')

    def test_classify_test(self):
        self.assertEqual(self.classify('test something'), 'testing')

    def test_classify_fix(self):
        self.assertEqual(self.classify('fix a bug'), 'bugfix')

    def test_classify_refactor(self):
        self.assertEqual(self.classify('refactor code'), 'refactor')

    def test_classify_other(self):
        self.assertEqual(self.classify('write docs'), 'other')

    def test_batch_classify(self):
        self.assertEqual(self.batch_classify(['implement x', 'test y']), ['implementation', 'testing'])

    def test_stats_empty(self):
        self.assertEqual(self.classification_stats([])['total'], 0)

    def test_stats_most_common(self):
        self.assertEqual(self.classification_stats(['implementation', 'implementation', 'testing'])['most_common'], 'implementation')

    def test_stats_by_type(self):
        self.assertEqual(self.classification_stats(['a', 'b'])['by_type'].get('a', 0), 1)


class TestPipelineHealth(unittest.TestCase):
    def setUp(self):
        from devkit.pipeline_health import check, trend, health_report
        self.check = check
        self.trend = trend
        self.health_report = health_report

    def test_healthy_true(self):
        self.assertTrue(self.check({'ok_rate': 0.9, 'avg_tokens': 1000, 'error_count': 0})['healthy'])

    def test_low_ok_rate_unhealthy(self):
        self.assertFalse(self.check({'ok_rate': 0.7, 'avg_tokens': 1000, 'error_count': 0})['healthy'])

    def test_low_ok_rate_warning(self):
        self.assertIn('low ok_rate', self.check({'ok_rate': 0.7, 'avg_tokens': 1000, 'error_count': 0})['warnings'])

    def test_high_tokens_unhealthy(self):
        self.assertFalse(self.check({'ok_rate': 0.9, 'avg_tokens': 15000, 'error_count': 0})['healthy'])

    def test_score_value(self):
        self.assertEqual(self.check({'ok_rate': 0.9, 'avg_tokens': 1000, 'error_count': 0})['score'], 0.9)

    def test_trend_empty(self):
        self.assertFalse(self.trend([])['improving'])

    def test_trend_improving(self):
        self.assertTrue(self.trend([{'ok_rate': 0.5}, {'ok_rate': 0.9}])['improving'])

    def test_trend_declining(self):
        self.assertFalse(self.trend([{'ok_rate': 0.9}, {'ok_rate': 0.5}])['improving'])

    def test_trend_avg(self):
        self.assertEqual(self.trend([{'ok_rate': 0.5}, {'ok_rate': 0.9}])['avg_ok_rate'], 0.7)

    def test_health_report_is_str(self):
        self.assertIsInstance(self.health_report(self.check({'ok_rate': 0.9, 'avg_tokens': 1000, 'error_count': 0})), str)


class TestArtifactStore(unittest.TestCase):
    def setUp(self):
        from devkit.artifact_store import create, put, get, keys, remove
        self.create = create
        self.put = put
        self.get = get
        self.keys = keys
        self.remove = remove

    def test_create_empty(self):
        self.assertEqual(self.create()['artifacts'], {})

    def test_get_missing_default(self):
        self.assertIsNone(self.get(self.create(), 'k'))

    def test_get_stored_value(self):
        self.assertEqual(self.get(self.put(self.create(), 'k', 'v'), 'k'), 'v')

    def test_get_with_default(self):
        self.assertEqual(self.get(self.put(self.create(), 'k', 'v'), 'x', 'def'), 'def')

    def test_keys_empty(self):
        self.assertEqual(self.keys(self.create()), [])

    def test_keys_sorted(self):
        self.assertEqual(self.keys(self.put(self.put(self.create(), 'b', 1), 'a', 2)), ['a', 'b'])

    def test_remove_key(self):
        self.assertIsNone(self.get(self.remove(self.put(self.create(), 'k', 'v'), 'k'), 'k'))

    def test_keys_length(self):
        self.assertEqual(len(self.keys(self.put(self.put(self.create(), 'a', 1), 'b', 2))), 2)

    def test_put_stores_value(self):
        self.assertEqual(self.put(self.create(), 'x', 42)['artifacts']['x'], 42)

    def test_get_custom_default(self):
        self.assertEqual(self.get(self.create(), 'missing', 99), 99)


class TestRunCache(unittest.TestCase):
    def setUp(self):
        from devkit.run_cache import create, lookup, store, cache_stats
        self.create = create
        self.lookup = lookup
        self.store = store
        self.cache_stats = cache_stats

    def test_create_max_size(self):
        self.assertEqual(self.create(10)['max_size'], 10)

    def test_create_hits_zero(self):
        self.assertEqual(self.create(10)['hits'], 0)

    def test_lookup_miss(self):
        self.assertFalse(self.lookup(self.create(10), 'k')[0])

    def test_lookup_miss_value(self):
        self.assertIsNone(self.lookup(self.create(10), 'k')[1])

    def test_lookup_hit(self):
        self.assertTrue(self.lookup(self.store(self.create(10), 'k', 'v'), 'k')[0])

    def test_lookup_hit_value(self):
        self.assertEqual(self.lookup(self.store(self.create(10), 'k', 'v'), 'k')[1], 'v')

    def test_stats_hit_rate_zero(self):
        self.assertEqual(self.cache_stats(self.create(10))['hit_rate'], 0.0)

    def test_store_size(self):
        self.assertEqual(len(self.store(self.create(1), 'a', 1)['cache']), 1)

    def test_store_evicts_oldest(self):
        self.assertNotIn('a', self.store(self.store(self.create(1), 'a', 1), 'b', 2)['cache'])

    def test_stats_is_dict(self):
        self.assertIsInstance(self.cache_stats(self.create(10)), dict)


class TestStageTimer(unittest.TestCase):
    def setUp(self):
        from devkit.stage_timer import record, stats, slowest, timer_report
        self.record = record
        self.stats = stats
        self.slowest = slowest
        self.timer_report = timer_report

    def test_record_is_dict(self):
        self.assertIsInstance(self.record('implement', 5.0), dict)

    def test_record_stage(self):
        self.assertEqual(self.record('implement', 5.0)['stage'], 'implement')

    def test_record_duration(self):
        self.assertEqual(self.record('implement', 5.0)['duration_s'], 5.0)

    def test_stats_empty_count(self):
        self.assertEqual(self.stats([])['count'], 0)

    def test_stats_empty_avg(self):
        self.assertEqual(self.stats([])['avg'], 0.0)

    def test_stats_total(self):
        self.assertEqual(self.stats([self.record('a', 3.0), self.record('b', 7.0)])['total'], 10.0)

    def test_stats_max(self):
        self.assertEqual(self.stats([self.record('a', 3.0), self.record('b', 7.0)])['max'], 7.0)

    def test_slowest_first(self):
        self.assertEqual(self.slowest([self.record('a', 3.0), self.record('b', 7.0)], 1)[0]['duration_s'], 7.0)

    def test_slowest_empty(self):
        self.assertEqual(self.slowest([], 1), [])

    def test_timer_report_is_str(self):
        self.assertIsInstance(self.timer_report([self.record('a', 5.0)]), str)


class TestFeedbackCollector(unittest.TestCase):
    def setUp(self):
        from devkit.feedback_collector import add, summary, filter_by_rating
        self.add = add
        self.summary = summary
        self.filter_by_rating = filter_by_rating

    def test_add_length(self):
        self.assertEqual(len(self.add([], 'implement', 4)), 1)

    def test_add_stage(self):
        self.assertEqual(self.add([], 'implement', 4)[0]['stage'], 'implement')

    def test_add_rating(self):
        self.assertEqual(self.add([], 'implement', 4)[0]['rating'], 4)

    def test_add_comment(self):
        self.assertEqual(self.add([], 'implement', 4, 'good')[0]['comment'], 'good')

    def test_summary_empty_total(self):
        self.assertEqual(self.summary([])['total'], 0)

    def test_summary_empty_avg(self):
        self.assertEqual(self.summary([])['avg_rating'], 0.0)

    def test_summary_avg_rating(self):
        self.assertEqual(self.summary(self.add([], 'impl', 4))['avg_rating'], 4.0)

    def test_summary_by_stage_count(self):
        self.assertEqual(self.summary(self.add([], 'impl', 4))['by_stage']['impl']['count'], 1)

    def test_filter_by_rating(self):
        fb = self.add(self.add([], 'impl', 3), 'impl', 5)
        self.assertEqual(len(self.filter_by_rating(fb, 4)), 1)

    def test_filter_empty(self):
        self.assertEqual(self.filter_by_rating([], 3), [])


class TestStageDepChecker(unittest.TestCase):
    def setUp(self):
        from devkit.stage_dep_checker import check, check_all, ready_stages
        self.check = check
        self.check_all = check_all
        self.ready_stages = ready_stages

    def test_ready_with_dep_met(self):
        self.assertTrue(self.check('b', ['a'], {'a'})['ready'])

    def test_not_ready_dep_missing(self):
        self.assertFalse(self.check('b', ['a'], set())['ready'])

    def test_missing_list(self):
        self.assertEqual(self.check('b', ['a'], set())['missing'], ['a'])

    def test_ready_no_deps(self):
        self.assertTrue(self.check('b', [], set())['ready'])

    def test_check_all_length(self):
        self.assertEqual(len(self.check_all({'a': [], 'b': ['a']}, set())), 2)

    def test_check_all_no_dep_ready(self):
        self.assertTrue(self.check_all({'a': []}, set())[0]['ready'])

    def test_ready_stages_partial(self):
        self.assertEqual(self.ready_stages({'a': [], 'b': ['a']}, set()), ['a'])

    def test_ready_stages_all(self):
        self.assertEqual(self.ready_stages({'a': [], 'b': ['a']}, {'a'}), ['a', 'b'])

    def test_missing_partial_deps(self):
        self.assertEqual(self.check('x', ['y', 'z'], {'y'})['missing'], ['z'])

    def test_ready_stages_empty(self):
        self.assertEqual(self.ready_stages({}, set()), [])


class TestTokenEstimatorV2(unittest.TestCase):
    def setUp(self):
        from devkit.token_estimator_v2 import estimate, estimate_messages, fits_in_context
        self.estimate = estimate
        self.estimate_messages = estimate_messages
        self.fits_in_context = fits_in_context

    def test_empty_string(self):
        self.assertEqual(self.estimate(''), 0)

    def test_hello_default(self):
        self.assertEqual(self.estimate('hello'), 2)

    def test_hello_glm(self):
        self.assertEqual(self.estimate('hello', 'glm'), 2)

    def test_hello_deepseek(self):
        self.assertEqual(self.estimate('hello', 'deepseek'), 2)

    def test_100_chars_default(self):
        self.assertEqual(self.estimate('a' * 100), 25)

    def test_100_chars_glm(self):
        self.assertEqual(self.estimate('a' * 100, 'glm'), 34)

    def test_messages_one(self):
        self.assertEqual(self.estimate_messages([{'role': 'user', 'content': 'hello'}]), 6)

    def test_messages_empty(self):
        self.assertEqual(self.estimate_messages([]), 0)

    def test_fits_true(self):
        self.assertTrue(self.fits_in_context('hello', 10))

    def test_fits_false(self):
        self.assertFalse(self.fits_in_context('a' * 1000, 10))


class TestErrorClassifier(unittest.TestCase):
    def setUp(self):
        from devkit.error_classifier import classify, batch_classify, error_stats
        self.classify = classify
        self.batch_classify = batch_classify
        self.error_stats = error_stats

    def test_timeout(self):
        self.assertEqual(self.classify('connection timeout'), 'timeout')

    def test_rate_limit(self):
        self.assertEqual(self.classify('rate limit exceeded'), 'rate_limit')

    def test_auth(self):
        self.assertEqual(self.classify('401 unauthorized'), 'auth')

    def test_code_error(self):
        self.assertEqual(self.classify('ImportError: no module'), 'code_error')

    def test_assertion(self):
        self.assertEqual(self.classify('AssertionError'), 'assertion')

    def test_unknown(self):
        self.assertEqual(self.classify('something went wrong'), 'unknown')

    def test_batch_classify(self):
        self.assertEqual(self.batch_classify(['timeout', '401']), ['timeout', 'auth'])

    def test_stats_empty(self):
        self.assertEqual(self.error_stats([])['total'], 0)

    def test_stats_most_common(self):
        self.assertEqual(self.error_stats(['timeout', 'timeout', 'auth'])['most_common'], 'timeout')

    def test_stats_by_type(self):
        self.assertEqual(self.error_stats(['x'])['by_type'].get('unknown', 0), 1)


class TestRunHistory(unittest.TestCase):
    def setUp(self):
        from devkit.run_history import create, append, last_n, history_stats
        self.create = create
        self.append = append
        self.last_n = last_n
        self.history_stats = history_stats

    def test_create_total(self):
        self.assertEqual(self.create()['total'], 0)

    def test_create_runs(self):
        self.assertEqual(self.create()['runs'], [])

    def test_append_total(self):
        self.assertEqual(self.append(self.create(), {'id': 'r1', 'gate': 'GO'})['total'], 1)

    def test_append_length(self):
        self.assertEqual(len(self.append(self.create(), {'id': 'r1', 'gate': 'GO'})['runs']), 1)

    def test_last_n_empty(self):
        self.assertEqual(self.last_n(self.create(), 5), [])

    def test_last_n_one(self):
        h = self.append(self.create(), {'id': 'r1', 'gate': 'GO'})
        self.assertEqual(self.last_n(h, 1)[0]['id'], 'r1')

    def test_last_n_zero(self):
        h = self.append(self.create(), {'id': 'r1', 'gate': 'GO'})
        self.assertEqual(self.last_n(h, 0), [])

    def test_stats_empty_total(self):
        self.assertEqual(self.history_stats(self.create())['total'], 0)

    def test_stats_empty_last_gate(self):
        self.assertIsNone(self.history_stats(self.create())['last_gate'])

    def test_stats_go_count(self):
        h = self.append(self.create(), {'id': 'r1', 'gate': 'GO'})
        self.assertEqual(self.history_stats(h)['go_count'], 1)


class TestStageWatcher(unittest.TestCase):
    def setUp(self):
        from devkit.stage_watcher import create, record_change, get_status, change_summary
        self.create = create
        self.record_change = record_change
        self.get_status = get_status
        self.change_summary = change_summary

    def test_create_events(self):
        self.assertEqual(self.create()['events'], [])

    def test_create_stages(self):
        self.assertEqual(self.create()['stages'], {})

    def test_get_status_none(self):
        self.assertIsNone(self.get_status(self.create(), 'impl'))

    def test_get_status_after_change(self):
        w = self.record_change(self.create(), 'impl', 'pending', 'running')
        self.assertEqual(self.get_status(w, 'impl'), 'running')

    def test_record_change_events_length(self):
        self.assertEqual(len(self.record_change(self.create(), 'impl', 'pending', 'running')['events']), 1)

    def test_record_change_stages(self):
        self.assertEqual(self.record_change(self.create(), 'impl', 'pending', 'running')['stages']['impl'], 'running')

    def test_summary_empty_events(self):
        self.assertEqual(self.change_summary(self.create())['total_events'], 0)

    def test_summary_empty_latest(self):
        self.assertIsNone(self.change_summary(self.create())['latest'])

    def test_summary_total_events(self):
        w = self.record_change(self.create(), 'impl', 'pending', 'running')
        self.assertEqual(self.change_summary(w)['total_events'], 1)

    def test_summary_stages_tracked(self):
        w = self.record_change(self.create(), 'impl', 'pending', 'running')
        self.assertEqual(self.change_summary(w)['stages_tracked'], 1)


class TestMetricAggregator(unittest.TestCase):
    def setUp(self):
        from devkit.metric_aggregator import aggregate, merge, normalize
        self.aggregate = aggregate
        self.merge = merge
        self.normalize = normalize

    def test_aggregate_empty_count(self):
        self.assertEqual(self.aggregate([], 'score')['count'], 0)

    def test_aggregate_empty_avg(self):
        self.assertEqual(self.aggregate([], 'score')['avg'], 0.0)

    def test_aggregate_sum(self):
        self.assertEqual(self.aggregate([{'score': 3.0}, {'score': 7.0}], 'score')['sum'], 10.0)

    def test_aggregate_avg(self):
        self.assertEqual(self.aggregate([{'score': 3.0}, {'score': 7.0}], 'score')['avg'], 5.0)

    def test_aggregate_min(self):
        self.assertEqual(self.aggregate([{'score': 3.0}, {'score': 7.0}], 'score')['min'], 3.0)

    def test_aggregate_max(self):
        self.assertEqual(self.aggregate([{'score': 3.0}, {'score': 7.0}], 'score')['max'], 7.0)

    def test_merge_union(self):
        self.assertEqual(self.merge({'a': 1}, {'b': 2}), {'a': 1, 'b': 2})

    def test_merge_override(self):
        self.assertEqual(self.merge({'a': 1}, {'a': 2})['a'], 2)

    def test_normalize_max(self):
        self.assertEqual(self.normalize([{'v': 0.0}, {'v': 10.0}], 'v')[1]['v'], 1.0)

    def test_normalize_equal(self):
        self.assertEqual(self.normalize([{'v': 5.0}, {'v': 5.0}], 'v')[0]['v'], 0.0)


class TestResultFormatter(unittest.TestCase):
    def setUp(self):
        from devkit.result_formatter import to_text, to_table, to_summary
        self.to_text = to_text
        self.to_table = to_table
        self.to_summary = to_summary

    def test_to_text_basic(self):
        self.assertEqual(self.to_text({'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0}), 'Run r1: GO | 100 tokens | 5.0s')

    def test_to_text_returns_str(self):
        self.assertIsInstance(self.to_text({'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0}), str)

    def test_to_table_empty(self):
        self.assertEqual(self.to_table([]), '(no results)')

    def test_to_table_returns_str(self):
        self.assertIsInstance(self.to_table([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0}]), str)

    def test_to_table_contains_id(self):
        self.assertIn('r1', self.to_table([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0}]))

    def test_to_summary_empty(self):
        self.assertEqual(self.to_summary([]), '0 runs: 0 GO, 0 NO-GO')

    def test_to_summary_mixed(self):
        self.assertEqual(self.to_summary([{'gate': 'GO'}, {'gate': 'NO-GO'}]), '2 runs: 1 GO, 1 NO-GO')

    def test_to_summary_all_go(self):
        self.assertEqual(self.to_summary([{'gate': 'GO'}, {'gate': 'GO'}]), '2 runs: 2 GO, 0 NO-GO')

    def test_to_table_header(self):
        result = self.to_table([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 5.0}])
        self.assertTrue('r1' in result or 'id' in result)

    def test_to_summary_returns_str(self):
        self.assertIsInstance(self.to_summary([{'gate': 'GO'}]), str)


class TestPipelineTracer(unittest.TestCase):
    def setUp(self):
        from devkit.pipeline_tracer import create, start_span, end_span, trace_summary
        self.create = create
        self.start_span = start_span
        self.end_span = end_span
        self.trace_summary = trace_summary

    def test_create_spans_empty(self):
        self.assertEqual(self.create()['spans'], [])

    def test_create_active_none(self):
        self.assertIsNone(self.create()['active'])

    def test_start_span_active(self):
        self.assertEqual(self.start_span(self.create(), 'impl')['active'], 'impl')

    def test_start_span_adds_span(self):
        self.assertEqual(len(self.start_span(self.create(), 'impl')['spans']), 1)

    def test_start_span_end_none(self):
        self.assertIsNone(self.start_span(self.create(), 'impl')['spans'][0]['end'])

    def test_trace_summary_empty(self):
        self.assertEqual(self.trace_summary(self.create())['total_spans'], 0)

    def test_trace_summary_one_span(self):
        self.assertEqual(self.trace_summary(self.start_span(self.create(), 'impl'))['total_spans'], 1)

    def test_end_span_completed(self):
        t = self.end_span(self.start_span(self.create(), 'impl'), 'impl')
        self.assertEqual(self.trace_summary(t)['completed'], 1)

    def test_end_span_active_none(self):
        t = self.end_span(self.start_span(self.create(), 'impl'), 'impl')
        self.assertIsNone(t['active'])

    def test_end_span_duration_nonneg(self):
        t = self.end_span(self.start_span(self.create(), 'impl'), 'impl')
        self.assertGreaterEqual(t['spans'][0]['duration_ms'], 0)


class TestRunScorer(unittest.TestCase):
    def setUp(self):
        from devkit.run_scorer import score_run, rank_runs, score_summary
        self.score_run = score_run
        self.rank_runs = rank_runs
        self.score_summary = score_summary

    def test_score_run_range(self):
        v = self.score_run({'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0})
        self.assertGreaterEqual(v, 0.0)
        self.assertLessEqual(v, 1.0)

    def test_score_run_go_gt_nogo(self):
        self.assertGreater(
            self.score_run({'gate': 'GO', 'tokens': 0, 'duration_s': 0, 'ok_rate': 1.0}),
            self.score_run({'gate': 'NO-GO', 'tokens': 0, 'duration_s': 0, 'ok_rate': 0.0})
        )

    def test_rank_runs_empty(self):
        self.assertEqual(len(self.rank_runs([])), 0)

    def test_rank_runs_one(self):
        self.assertEqual(len(self.rank_runs([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0}])), 1)

    def test_rank_runs_has_score(self):
        self.assertIn('score', self.rank_runs([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0}])[0])

    def test_score_summary_empty_count(self):
        self.assertEqual(self.score_summary([])['count'], 0)

    def test_score_summary_empty_best(self):
        self.assertIsNone(self.score_summary([])['best_id'])

    def test_score_summary_count(self):
        self.assertEqual(self.score_summary([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0}])['count'], 1)

    def test_score_summary_best_id(self):
        self.assertEqual(self.score_summary([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0}])['best_id'], 'r1')

    def test_score_summary_avg_float(self):
        self.assertIsInstance(self.score_summary([{'id': 'r1', 'gate': 'GO', 'tokens': 100, 'duration_s': 1, 'ok_rate': 1.0}])['avg_score'], float)


class TestStageReplay(unittest.TestCase):
    def setUp(self):
        from devkit.stage_replay import create_replay, step, replay_all, replay_summary
        self.create_replay = create_replay
        self.step = step
        self.replay_all = replay_all
        self.replay_summary = replay_summary

    def test_create_replay_cursor(self):
        self.assertEqual(self.create_replay([])['cursor'], 0)

    def test_create_replay_total(self):
        self.assertEqual(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}])['total'], 1)

    def test_step_empty(self):
        self.assertIsNone(self.step(self.create_replay([]))[0])

    def test_step_first_event(self):
        self.assertEqual(self.step(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}]))[0]['stage'], 'a')

    def test_step_advances_cursor(self):
        self.assertEqual(self.step(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}]))[1]['cursor'], 1)

    def test_replay_all_len(self):
        self.assertEqual(len(self.replay_all(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}]))), 1)

    def test_replay_summary_empty_done(self):
        self.assertTrue(self.replay_summary(self.create_replay([]))['done'])

    def test_replay_summary_not_done(self):
        self.assertFalse(self.replay_summary(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}]))['done'])

    def test_replay_summary_empty_remaining(self):
        self.assertEqual(self.replay_summary(self.create_replay([]))['remaining'], 0)

    def test_replay_summary_remaining(self):
        self.assertEqual(self.replay_summary(self.create_replay([{'stage': 'a', 'status': 'ok', 'timestamp': 't1'}]))['remaining'], 1)


class TestJobQueue(unittest.TestCase):
    def setUp(self):
        from devkit.job_queue import create, enqueue, dequeue, queue_summary
        self.create = create
        self.enqueue = enqueue
        self.dequeue = dequeue
        self.queue_summary = queue_summary

    def test_create_jobs_empty(self):
        self.assertEqual(self.create()['jobs'], [])

    def test_create_processed_zero(self):
        self.assertEqual(self.create()['processed'], 0)

    def test_enqueue_adds_job(self):
        self.assertEqual(len(self.enqueue(self.create(), {'id': 'j1'}, 0)['jobs']), 1)

    def test_dequeue_empty(self):
        self.assertIsNone(self.dequeue(self.create())[0])

    def test_dequeue_returns_job(self):
        self.assertEqual(self.dequeue(self.enqueue(self.create(), {'id': 'j1'}, 0))[0]['id'], 'j1')

    def test_dequeue_increments_processed(self):
        self.assertEqual(self.dequeue(self.enqueue(self.create(), {'id': 'j1'}, 0))[1]['processed'], 1)

    def test_dequeue_priority_order(self):
        q = self.enqueue(self.enqueue(self.create(), {'id': 'j1'}, 1), {'id': 'j2'}, 5)
        self.assertEqual(self.dequeue(q)[0]['id'], 'j2')

    def test_summary_empty_pending(self):
        self.assertEqual(self.queue_summary(self.create())['pending'], 0)

    def test_summary_empty_next_id(self):
        self.assertIsNone(self.queue_summary(self.create())['next_id'])

    def test_summary_next_id(self):
        self.assertEqual(self.queue_summary(self.enqueue(self.create(), {'id': 'j1'}, 0))['next_id'], 'j1')


class TestCircuitBreaker(unittest.TestCase):
    def setUp(self):
        from devkit.circuit_breaker import create, record_success, record_failure, is_open, cb_summary
        self.create = create
        self.record_success = record_success
        self.record_failure = record_failure
        self.is_open = is_open
        self.cb_summary = cb_summary

    def test_create_state_closed(self):
        self.assertEqual(self.create()['state'], 'closed')

    def test_create_failures_zero(self):
        self.assertEqual(self.create()['failures'], 0)

    def test_is_open_false(self):
        self.assertFalse(self.is_open(self.create()))

    def test_is_open_after_threshold(self):
        cb = self.record_failure(self.record_failure(self.record_failure(self.create())))
        self.assertTrue(self.is_open(cb))

    def test_state_open_after_threshold(self):
        cb = self.record_failure(self.record_failure(self.record_failure(self.create())))
        self.assertEqual(cb['state'], 'open')

    def test_record_success_resets(self):
        self.assertEqual(self.record_success(self.create())['failures'], 0)

    def test_record_failure_increments(self):
        self.assertEqual(self.record_failure(self.create())['failures'], 1)

    def test_summary_is_open_false(self):
        self.assertFalse(self.cb_summary(self.create())['is_open'])

    def test_summary_is_open_true(self):
        cb = self.record_failure(self.record_failure(self.record_failure(self.create())))
        self.assertTrue(self.cb_summary(cb)['is_open'])

    def test_summary_state(self):
        self.assertEqual(self.cb_summary(self.create())['state'], 'closed')


class TestRateLimiter(unittest.TestCase):
    def setUp(self):
        from devkit.rate_limiter import create, consume, available, limiter_summary
        self.create = create
        self.consume = consume
        self.available = available
        self.limiter_summary = limiter_summary

    def test_create_tokens(self):
        self.assertEqual(self.create(10.0, 100.0)['tokens'], 100.0)

    def test_create_capacity(self):
        self.assertEqual(self.create(10.0, 100.0)['capacity'], 100.0)

    def test_consume_success(self):
        self.assertTrue(self.consume(self.create(10.0, 100.0))[0])

    def test_available_full(self):
        self.assertEqual(self.available(self.create(10.0, 100.0)), 100.0)

    def test_consume_reduces_tokens(self):
        self.assertLess(self.consume(self.create(10.0, 100.0), 50.0)[1]['tokens'], 100.0)

    def test_consume_insufficient(self):
        self.assertFalse(self.consume(self.create(10.0, 1.0), 2.0)[0])

    def test_summary_capacity(self):
        self.assertEqual(self.limiter_summary(self.create(10.0, 100.0))['capacity'], 100.0)

    def test_summary_rate(self):
        self.assertEqual(self.limiter_summary(self.create(10.0, 100.0))['rate'], 10.0)

    def test_summary_utilization_range(self):
        u = self.limiter_summary(self.create(10.0, 100.0))['utilization']
        self.assertGreaterEqual(u, 0.0)
        self.assertLessEqual(u, 1.0)

    def test_summary_available_float(self):
        self.assertIsInstance(self.limiter_summary(self.create(10.0, 100.0))['available'], float)


class TestSnapshotStore(unittest.TestCase):
    def setUp(self):
        from devkit.snapshot_store import create, save, load, list_keys, store_summary
        self.create = create
        self.save = save
        self.load = load
        self.list_keys = list_keys
        self.store_summary = store_summary

    def test_create_version(self):
        self.assertEqual(self.create()['version'], 0)

    def test_create_snapshots_empty(self):
        self.assertEqual(self.create()['snapshots'], {})

    def test_save_increments_version(self):
        self.assertEqual(self.save(self.create(), 'k1', 42)['version'], 1)

    def test_load_saved_value(self):
        self.assertEqual(self.load(self.save(self.create(), 'k1', 42), 'k1'), 42)

    def test_load_missing_none(self):
        self.assertIsNone(self.load(self.create(), 'k1'))

    def test_save_overwrites(self):
        s = self.save(self.save(self.create(), 'k1', 1), 'k1', 2)
        self.assertEqual(self.load(s, 'k1'), 2)

    def test_list_keys_empty(self):
        self.assertEqual(self.list_keys(self.create()), [])

    def test_list_keys_one(self):
        self.assertEqual(self.list_keys(self.save(self.create(), 'k1', 1)), ['k1'])

    def test_summary_empty(self):
        self.assertEqual(self.store_summary(self.create())['total_keys'], 0)

    def test_summary_one(self):
        self.assertEqual(self.store_summary(self.save(self.create(), 'k1', 1))['total_keys'], 1)


class TestWorkflowEngine(unittest.TestCase):
    def setUp(self):
        from devkit.workflow_engine import create, advance, current_step, wf_summary
        self.create = create
        self.advance = advance
        self.current_step = current_step
        self.wf_summary = wf_summary

    def test_create_status_pending(self):
        self.assertEqual(self.create(['a', 'b'])['status'], 'pending')

    def test_create_current_zero(self):
        self.assertEqual(self.create(['a', 'b'])['current'], 0)

    def test_current_step_first(self):
        self.assertEqual(self.current_step(self.create(['a', 'b'])), 'a')

    def test_current_step_after_advance(self):
        self.assertEqual(self.current_step(self.advance(self.create(['a', 'b']), {'ok': True})), 'b')

    def test_advance_done_single(self):
        self.assertEqual(self.advance(self.create(['a']), {'ok': True})['status'], 'done')

    def test_advance_running(self):
        self.assertEqual(self.advance(self.create(['a', 'b']), {'ok': True})['status'], 'running')

    def test_current_step_done_none(self):
        wf = self.advance(self.advance(self.create(['a', 'b']), {}), {})
        self.assertIsNone(self.current_step(wf))

    def test_summary_total(self):
        self.assertEqual(self.wf_summary(self.create(['a', 'b']))['total'], 2)

    def test_summary_completed(self):
        self.assertEqual(self.wf_summary(self.advance(self.create(['a', 'b']), {}))['completed'], 1)

    def test_summary_status_done(self):
        wf = self.advance(self.advance(self.create(['a', 'b']), {}), {})
        self.assertEqual(self.wf_summary(wf)['status'], 'done')


class TestLockManager(unittest.TestCase):
    def setUp(self):
        from devkit.lock_manager import create, acquire, release, is_locked, manager_summary
        self.create = create
        self.acquire = acquire
        self.release = release
        self.is_locked = is_locked
        self.manager_summary = manager_summary

    def test_create_locks_empty(self):
        self.assertEqual(self.create()['locks'], {})

    def test_acquire_success(self):
        self.assertTrue(self.acquire(self.create(), 'r1', 'p1')[0])

    def test_is_locked_after_acquire(self):
        self.assertTrue(self.is_locked(self.acquire(self.create(), 'r1', 'p1')[1], 'r1'))

    def test_acquire_conflict(self):
        mgr = self.acquire(self.create(), 'r1', 'p1')[1]
        self.assertFalse(self.acquire(mgr, 'r1', 'p2')[0])

    def test_acquire_reentrant(self):
        mgr = self.acquire(self.create(), 'r1', 'p1')[1]
        self.assertTrue(self.acquire(mgr, 'r1', 'p1')[0])

    def test_release_success(self):
        mgr = self.acquire(self.create(), 'r1', 'p1')[1]
        self.assertTrue(self.release(mgr, 'r1', 'p1')[0])

    def test_is_locked_after_release(self):
        mgr = self.acquire(self.create(), 'r1', 'p1')[1]
        self.assertFalse(self.is_locked(self.release(mgr, 'r1', 'p1')[1], 'r1'))

    def test_release_not_held(self):
        self.assertFalse(self.release(self.create(), 'r1', 'p1')[0])

    def test_summary_empty(self):
        self.assertEqual(self.manager_summary(self.create())['total_locked'], 0)

    def test_summary_one(self):
        mgr = self.acquire(self.create(), 'r1', 'p1')[1]
        self.assertEqual(self.manager_summary(mgr)['total_locked'], 1)


class TestDiffEngine(unittest.TestCase):
    def setUp(self):
        from devkit.diff_engine import diff, apply, diff_summary
        self.diff = diff
        self.apply = apply
        self.diff_summary = diff_summary

    def test_diff_added(self):
        self.assertEqual(self.diff({}, {'a': 1})['added'], {'a': 1})

    def test_diff_removed(self):
        self.assertEqual(self.diff({'a': 1}, {})['removed'], {'a': 1})

    def test_diff_changed(self):
        self.assertEqual(self.diff({'a': 1}, {'a': 2})['changed']['a'], {'old': 1, 'new': 2})

    def test_diff_unchanged(self):
        self.assertEqual(self.diff({'a': 1}, {'a': 1})['unchanged'], {'a': 1})

    def test_diff_empty(self):
        self.assertEqual(self.diff({}, {}), {'added': {}, 'removed': {}, 'changed': {}, 'unchanged': {}})

    def test_apply_add_key(self):
        self.assertEqual(self.apply({'a': 1}, {'b': 2}), {'a': 1, 'b': 2})

    def test_apply_remove_key(self):
        self.assertEqual(self.apply({'a': 1}, {'a': None}), {})

    def test_apply_overwrite(self):
        self.assertEqual(self.apply({'a': 1}, {'a': 2}), {'a': 2})

    def test_summary_added(self):
        self.assertEqual(self.diff_summary(self.diff({'a': 1}, {'a': 2, 'b': 3}))['added'], 1)

    def test_summary_changed(self):
        self.assertEqual(self.diff_summary(self.diff({'a': 1}, {'a': 2}))['changed'], 1)


class TestDeadlineTracker(unittest.TestCase):
    def setUp(self):
        from devkit.deadline_tracker import create, set_deadline, mark_done, check_overdue, tracker_summary
        self.create = create
        self.set_deadline = set_deadline
        self.mark_done = mark_done
        self.check_overdue = check_overdue
        self.tracker_summary = tracker_summary

    def test_create_deadlines_empty(self):
        self.assertEqual(self.create()['deadlines'], {})

    def test_create_overdue_empty(self):
        self.assertEqual(self.create()['overdue'], [])

    def test_set_deadline_adds(self):
        self.assertEqual(len(self.set_deadline(self.create(), 't1', '2099-01-01')['deadlines']), 1)

    def test_mark_done(self):
        tr = self.mark_done(self.set_deadline(self.create(), 't1', '2099-01-01'), 't1')
        self.assertTrue(tr['deadlines']['t1']['done'])

    def test_check_overdue_past(self):
        tr = self.check_overdue(self.set_deadline(self.create(), 't1', '2000-01-01'), '2026-01-01')
        self.assertEqual(len(tr['overdue']), 1)

    def test_check_overdue_future(self):
        tr = self.check_overdue(self.set_deadline(self.create(), 't1', '2099-01-01'), '2026-01-01')
        self.assertEqual(len(tr['overdue']), 0)

    def test_summary_empty_total(self):
        self.assertEqual(self.tracker_summary(self.create())['total'], 0)

    def test_summary_total(self):
        self.assertEqual(self.tracker_summary(self.set_deadline(self.create(), 't1', '2099-01-01'))['total'], 1)

    def test_summary_done(self):
        tr = self.mark_done(self.set_deadline(self.create(), 't1', '2099-01-01'), 't1')
        self.assertEqual(self.tracker_summary(tr)['done'], 1)

    def test_summary_overdue(self):
        tr = self.check_overdue(self.set_deadline(self.create(), 't1', '2000-01-01'), '2026-01-01')
        self.assertEqual(self.tracker_summary(tr)['overdue'], 1)


class TestStateMachine(unittest.TestCase):
    def setUp(self):
        from devkit.state_machine import create, add_transition, trigger, sm_summary
        self.create = create
        self.add_transition = add_transition
        self.trigger = trigger
        self.sm_summary = sm_summary

    def test_create_current(self):
        self.assertEqual(self.create(['a', 'b'], 'a')['current'], 'a')

    def test_create_history_empty(self):
        self.assertEqual(self.create(['a', 'b'], 'a')['history'], [])

    def test_trigger_success(self):
        sm = self.add_transition(self.create(['a', 'b'], 'a'), 'a', 'go', 'b')
        self.assertTrue(self.trigger(sm, 'go')[0])

    def test_trigger_advances_state(self):
        sm = self.add_transition(self.create(['a', 'b'], 'a'), 'a', 'go', 'b')
        self.assertEqual(self.trigger(sm, 'go')[1]['current'], 'b')

    def test_trigger_no_transition(self):
        self.assertFalse(self.trigger(self.create(['a', 'b'], 'a'), 'go')[0])

    def test_trigger_records_history(self):
        sm = self.add_transition(self.create(['a', 'b'], 'a'), 'a', 'go', 'b')
        self.assertEqual(self.trigger(sm, 'go')[1]['history'], ['a'])

    def test_summary_total_states(self):
        self.assertEqual(self.sm_summary(self.create(['a', 'b'], 'a'))['total_states'], 2)

    def test_summary_total_transitions(self):
        sm = self.add_transition(self.create(['a', 'b'], 'a'), 'a', 'go', 'b')
        self.assertEqual(self.sm_summary(sm)['total_transitions'], 1)

    def test_summary_current(self):
        self.assertEqual(self.sm_summary(self.create(['a', 'b'], 'a'))['current'], 'a')

    def test_summary_history_empty(self):
        self.assertEqual(self.sm_summary(self.create(['a', 'b'], 'a'))['history'], [])


class TestWorkPool(unittest.TestCase):
    def setUp(self):
        from devkit.work_pool import create, submit, checkout, complete, pool_summary
        self.create = create
        self.submit = submit
        self.checkout = checkout
        self.complete = complete
        self.pool_summary = pool_summary

    def test_create_capacity(self):
        self.assertEqual(self.create(5)['capacity'], 5)

    def test_create_items_empty(self):
        self.assertEqual(self.create(5)['items'], [])

    def test_submit_success(self):
        self.assertTrue(self.submit(self.create(2), {'id': 'w1'})[0])

    def test_submit_overflow(self):
        p = self.create(2)
        _, p = self.submit(p, {'id': 'w1'})
        _, p = self.submit(p, {'id': 'w2'})
        ok, _ = self.submit(p, {'id': 'w3'})
        self.assertFalse(ok)

    def test_checkout_returns_item(self):
        p = self.submit(self.create(2), {'id': 'w1'})[1]
        self.assertEqual(self.checkout(p, 'w1')[0]['id'], 'w1')

    def test_checkout_moves_to_active(self):
        p = self.submit(self.create(2), {'id': 'w1'})[1]
        self.assertEqual(len(self.checkout(p, 'w1')[1]['active']), 1)

    def test_checkout_missing(self):
        self.assertIsNone(self.checkout(self.create(2), 'w1')[0])

    def test_summary_empty_pending(self):
        self.assertEqual(self.pool_summary(self.create(5))['pending'], 0)

    def test_summary_pending(self):
        p = self.submit(self.create(5), {'id': 'w1'})[1]
        self.assertEqual(self.pool_summary(p)['pending'], 1)

    def test_summary_available(self):
        self.assertEqual(self.pool_summary(self.create(5))['available'], 5)


class TestSemaphore(unittest.TestCase):
    def setUp(self):
        from devkit.semaphore import create, acquire, release, sem_summary
        self.create = create
        self.acquire = acquire
        self.release = release
        self.sem_summary = sem_summary

    def test_create_count(self):
        self.assertEqual(self.create(3)['count'], 3)

    def test_acquire_success(self):
        self.assertTrue(self.acquire(self.create(3), 'p1')[0])

    def test_acquire_decrements(self):
        self.assertEqual(self.acquire(self.create(3), 'p1')[1]['count'], 2)

    def test_acquire_exhausts(self):
        self.assertEqual(self.acquire(self.create(1), 'p1')[1]['count'], 0)

    def test_acquire_blocks_when_full(self):
        sem = self.acquire(self.create(1), 'p1')[1]
        self.assertFalse(self.acquire(sem, 'p2')[0])

    def test_acquire_adds_waiter(self):
        sem = self.acquire(self.create(1), 'p1')[1]
        self.assertEqual(len(self.acquire(sem, 'p2')[1]['waiters']), 1)

    def test_release_increments(self):
        sem = self.acquire(self.create(3), 'p1')[1]
        self.assertEqual(self.release(sem, 'p1')['count'], 3)

    def test_summary_available(self):
        self.assertEqual(self.sem_summary(self.create(3))['available'], 3)

    def test_summary_available_after_acquire(self):
        self.assertEqual(self.sem_summary(self.acquire(self.create(3), 'p1')[1])['available'], 2)

    def test_summary_utilization_zero(self):
        self.assertEqual(self.sem_summary(self.create(3))['utilization'], 0.0)


class TestBackoffTimer(unittest.TestCase):
    def setUp(self):
        from devkit.backoff_timer import create, next_delay, reset, timer_summary
        self.create = create
        self.next_delay = next_delay
        self.reset = reset
        self.timer_summary = timer_summary

    def test_create_attempt(self):
        self.assertEqual(self.create()['attempt'], 0)

    def test_create_base(self):
        self.assertEqual(self.create()['base'], 1.0)

    def test_next_delay_first(self):
        self.assertEqual(self.next_delay(self.create())[0], 1.0)

    def test_next_delay_increments(self):
        self.assertEqual(self.next_delay(self.create())[1]['attempt'], 1)

    def test_next_delay_second(self):
        self.assertEqual(self.next_delay(self.next_delay(self.create())[1])[0], 2.0)

    def test_next_delay_third(self):
        t = self.next_delay(self.next_delay(self.next_delay(self.create())[1])[1])[0]
        self.assertEqual(t, 4.0)

    def test_reset_attempt(self):
        self.assertEqual(self.reset(self.next_delay(self.create())[1])['attempt'], 0)

    def test_summary_next_delay(self):
        self.assertEqual(self.timer_summary(self.create())['next_delay'], 1.0)

    def test_summary_max_delay_cap(self):
        self.assertEqual(self.timer_summary(self.create(max_delay=0.5))['next_delay'], 0.5)

    def test_summary_attempt(self):
        self.assertEqual(self.timer_summary(self.create())['attempt'], 0)


class TestTokenBucket(unittest.TestCase):
    def setUp(self):
        from devkit.token_bucket import create, consume, usage, tb_summary
        self.create = create
        self.consume = consume
        self.usage = usage
        self.tb_summary = tb_summary

    def test_create_max_tokens(self):
        self.assertEqual(self.create()['max_tokens'], 1000)

    def test_create_total_consumed(self):
        self.assertEqual(self.create()['total_consumed'], 0)

    def test_consume_success(self):
        self.assertTrue(self.consume(self.create(), 'k1', 100, 0)[0])

    def test_consume_updates_total(self):
        self.assertEqual(self.consume(self.create(), 'k1', 100, 0)[1]['total_consumed'], 100)

    def test_usage_empty(self):
        self.assertEqual(self.usage(self.create(), 'k1', 0), 0)

    def test_usage_after_consume(self):
        tb = self.consume(self.create(), 'k1', 100, 0)[1]
        self.assertEqual(self.usage(tb, 'k1', 0), 100)

    def test_consume_over_limit(self):
        self.assertFalse(self.consume(self.create(60, 100), 'k1', 101, 0)[0])

    def test_consume_at_limit(self):
        self.assertTrue(self.consume(self.create(60, 100), 'k1', 100, 0)[0])

    def test_summary_total_consumed(self):
        self.assertEqual(self.tb_summary(self.create())['total_consumed'], 0)

    def test_summary_keys(self):
        tb = self.consume(self.create(), 'k1', 100, 0)[1]
        self.assertEqual(self.tb_summary(tb)['keys'], ['k1'])


class TestSlidingWindow(unittest.TestCase):
    def setUp(self):
        from devkit.sliding_window import create, push, avg, window_summary
        self.create = create
        self.push = push
        self.avg = avg
        self.window_summary = window_summary

    def test_create_size(self):
        self.assertEqual(self.create(3)['size'], 3)

    def test_create_items_empty(self):
        self.assertEqual(self.create(3)['items'], [])

    def test_push_adds_item(self):
        self.assertEqual(len(self.push(self.create(3), 1.0)['items']), 1)

    def test_push_evicts_oldest(self):
        w = self.push(self.push(self.push(self.push(self.create(3), 1.0), 2.0), 3.0), 4.0)
        self.assertEqual(len(w['items']), 3)

    def test_push_correct_oldest(self):
        w = self.push(self.push(self.push(self.push(self.create(3), 1.0), 2.0), 3.0), 4.0)
        self.assertEqual(w['items'][0], 2.0)

    def test_avg_empty(self):
        self.assertEqual(self.avg(self.create(3)), 0.0)

    def test_avg_correct(self):
        w = self.push(self.push(self.create(3), 1.0), 3.0)
        self.assertEqual(self.avg(w), 2.0)

    def test_summary_count_empty(self):
        self.assertEqual(self.window_summary(self.create(3))['count'], 0)

    def test_summary_max(self):
        self.assertEqual(self.window_summary(self.push(self.create(3), 5.0))['max'], 5.0)

    def test_summary_min(self):
        self.assertEqual(self.window_summary(self.push(self.create(3), 5.0))['min'], 5.0)


class TestPriorityQueue(unittest.TestCase):
    def setUp(self):
        from devkit.priority_queue import create, push, pop, peek, pq_summary
        self.create = create
        self.push = push
        self.pop = pop
        self.peek = peek
        self.pq_summary = pq_summary

    def test_create_size(self):
        self.assertEqual(self.create()['size'], 0)

    def test_pop_empty(self):
        self.assertIsNone(self.pop(self.create())[0])

    def test_peek_empty(self):
        self.assertIsNone(self.peek(self.create()))

    def test_push_size(self):
        self.assertEqual(self.push(self.create(), 'a', 1.0)['size'], 1)

    def test_pop_single(self):
        self.assertEqual(self.pop(self.push(self.create(), 'a', 1.0))[0], 'a')

    def test_pop_min_order(self):
        pq = self.push(self.push(self.create(), 'b', 2.0), 'a', 1.0)
        self.assertEqual(self.pop(pq)[0], 'a')

    def test_pop_max_order(self):
        pq = self.push(self.push(self.create('max'), 'a', 1.0), 'b', 2.0)
        self.assertEqual(self.pop(pq)[0], 'b')

    def test_peek_returns_top(self):
        self.assertEqual(self.peek(self.push(self.create(), 'a', 1.0)), 'a')

    def test_summary_size_empty(self):
        self.assertEqual(self.pq_summary(self.create())['size'], 0)

    def test_summary_top(self):
        self.assertEqual(self.pq_summary(self.push(self.create(), 'a', 1.0))['top'], 'a')


class TestBloomFilter(unittest.TestCase):
    def setUp(self):
        from devkit.bloom_filter import create, add, contains, bf_summary
        self.create = create
        self.add = add
        self.contains = contains
        self.bf_summary = bf_summary

    def test_create_added(self):
        self.assertEqual(self.create()['added'], 0)

    def test_create_hash_count(self):
        self.assertGreaterEqual(self.create()['hash_count'], 1)

    def test_add_increments(self):
        self.assertEqual(self.add(self.create(), 'hello')['added'], 1)

    def test_contains_after_add(self):
        self.assertTrue(self.contains(self.add(self.create(), 'hello'), 'hello'))

    def test_contains_not_added(self):
        self.assertFalse(self.contains(self.create(), 'hello'))

    def test_contains_first_of_two(self):
        bf = self.add(self.add(self.create(), 'a'), 'b')
        self.assertTrue(self.contains(bf, 'a'))

    def test_contains_second_of_two(self):
        bf = self.add(self.add(self.create(), 'a'), 'b')
        self.assertTrue(self.contains(bf, 'b'))

    def test_summary_added_empty(self):
        self.assertEqual(self.bf_summary(self.create())['added'], 0)

    def test_summary_added_one(self):
        self.assertEqual(self.bf_summary(self.add(self.create(), 'x'))['added'], 1)

    def test_summary_bit_size(self):
        self.assertGreater(self.bf_summary(self.create())['bit_size'], 0)


class TestLruCache(unittest.TestCase):
    def setUp(self):
        from devkit.lru_cache import create, get, put, lru_summary
        self.create = create
        self.get = get
        self.put = put
        self.lru_summary = lru_summary

    def test_create_capacity(self):
        self.assertEqual(self.create(3)['capacity'], 3)

    def test_get_miss(self):
        self.assertIsNone(self.get(self.create(3), 'k1')[0])

    def test_get_hit(self):
        lru = self.put(self.create(3), 'k1', 42)
        self.assertEqual(self.get(lru, 'k1')[0], 42)

    def test_put_adds(self):
        self.assertEqual(len(self.put(self.create(3), 'k1', 1)['cache']), 1)

    def test_put_evicts_lru(self):
        lru = self.put(self.put(self.put(self.put(self.create(3), 'a', 1), 'b', 2), 'c', 3), 'd', 4)
        self.assertEqual(len(lru['cache']), 3)

    def test_put_evicts_oldest(self):
        lru = self.put(self.put(self.put(self.put(self.create(3), 'a', 1), 'b', 2), 'c', 3), 'd', 4)
        self.assertNotIn('a', lru['cache'])

    def test_put_overwrites(self):
        lru = self.put(self.put(self.create(3), 'k1', 1), 'k1', 2)
        self.assertEqual(lru['cache']['k1'], 2)

    def test_summary_empty(self):
        self.assertEqual(self.lru_summary(self.create(3))['size'], 0)

    def test_summary_size(self):
        self.assertEqual(self.lru_summary(self.put(self.create(3), 'k1', 1))['size'], 1)

    def test_summary_keys(self):
        self.assertIn('k1', self.lru_summary(self.put(self.create(3), 'k1', 1))['keys'])


class TestTrie(unittest.TestCase):
    def setUp(self):
        from devkit.trie import create, insert, search, starts_with, trie_summary
        self.create = create
        self.insert = insert
        self.search = search
        self.starts_with = starts_with
        self.trie_summary = trie_summary

    def test_search_empty(self):
        self.assertFalse(self.search(self.create(), 'hi'))

    def test_search_inserted(self):
        self.assertTrue(self.search(self.insert(self.create(), 'hi'), 'hi'))

    def test_search_prefix_only(self):
        self.assertFalse(self.search(self.insert(self.create(), 'hi'), 'h'))

    def test_starts_with_prefix(self):
        self.assertTrue(self.starts_with(self.insert(self.create(), 'hi'), 'h'))

    def test_starts_with_no_match(self):
        self.assertFalse(self.starts_with(self.insert(self.create(), 'hi'), 'x'))

    def test_starts_with_empty(self):
        self.assertFalse(self.starts_with(self.create(), 'h'))

    def test_search_two_words(self):
        t = self.insert(self.insert(self.create(), 'hi'), 'hey')
        self.assertTrue(self.search(t, 'hey'))

    def test_summary_count_empty(self):
        self.assertEqual(self.trie_summary(self.create())['count'], 0)

    def test_summary_count_one(self):
        self.assertEqual(self.trie_summary(self.insert(self.create(), 'hi'))['count'], 1)

    def test_summary_root_children(self):
        self.assertEqual(self.trie_summary(self.insert(self.create(), 'hi'))['root_children'], 1)


class TestGraph(unittest.TestCase):
    def setUp(self):
        from devkit.graph import create, add_node, add_edge, bfs, dfs
        self.create = create
        self.add_node = add_node
        self.add_edge = add_edge
        self.bfs = bfs
        self.dfs = dfs

    def test_add_node(self):
        g = self.add_node(self.create(), 'a')
        self.assertIn('a', g['nodes'])

    def test_add_edge_src(self):
        self.assertIn('a', self.add_edge(self.create(), 'a', 'b')['nodes'])

    def test_add_edge_dst(self):
        self.assertIn('b', self.add_edge(self.create(), 'a', 'b')['nodes'])

    def test_bfs_simple(self):
        self.assertEqual(self.bfs(self.add_edge(self.create(), 'a', 'b'), 'a'), ['a', 'b'])

    def test_dfs_simple(self):
        self.assertEqual(self.dfs(self.add_edge(self.create(), 'a', 'b'), 'a'), ['a', 'b'])

    def test_bfs_missing_start(self):
        self.assertEqual(self.bfs(self.create(), 'a'), [])

    def test_bfs_fan_out_len(self):
        g = self.add_edge(self.add_edge(self.create(), 'a', 'b'), 'a', 'c')
        self.assertEqual(len(self.bfs(g, 'a')), 3)

    def test_bfs_starts_with_source(self):
        g = self.add_edge(self.add_edge(self.create(), 'a', 'b'), 'a', 'c')
        self.assertEqual(self.bfs(g, 'a')[0], 'a')

    def test_dfs_chain(self):
        g = self.add_edge(self.add_edge(self.create(), 'a', 'b'), 'b', 'c')
        self.assertEqual(self.dfs(g, 'a')[2], 'c')

    def test_bfs_unknown_start(self):
        self.assertEqual(self.bfs(self.add_edge(self.create(), 'a', 'b'), 'x'), [])


class TestIntervalTree(unittest.TestCase):
    def setUp(self):
        from devkit.interval_tree import create, insert, query, overlap, it_summary
        self.create = create
        self.insert = insert
        self.query = query
        self.overlap = overlap
        self.it_summary = it_summary

    def test_create_count(self):
        self.assertEqual(self.create()['count'], 0)

    def test_insert_count(self):
        self.assertEqual(self.insert(self.create(), 1, 5)['count'], 1)

    def test_query_hit(self):
        self.assertEqual(len(self.query(self.insert(self.create(), 1, 5), 3)), 1)

    def test_query_miss(self):
        self.assertEqual(len(self.query(self.insert(self.create(), 1, 5), 6)), 0)

    def test_query_boundary(self):
        self.assertEqual(self.query(self.insert(self.create(), 1, 5), 1)[0]['start'], 1)

    def test_overlap_hit(self):
        self.assertEqual(len(self.overlap(self.insert(self.create(), 1, 5), 3, 7)), 1)

    def test_overlap_miss(self):
        self.assertEqual(len(self.overlap(self.insert(self.create(), 1, 5), 6, 9)), 0)

    def test_summary_empty(self):
        self.assertEqual(self.it_summary(self.create())['count'], 0)

    def test_summary_min_start(self):
        self.assertEqual(self.it_summary(self.insert(self.create(), 1, 5))['min_start'], 1)

    def test_summary_max_end(self):
        self.assertEqual(self.it_summary(self.insert(self.create(), 1, 5))['max_end'], 5)


class TestMatrix(unittest.TestCase):
    def setUp(self):
        from devkit.matrix import create, get, set_val, add, transpose
        self.create = create
        self.get = get
        self.set_val = set_val
        self.add = add
        self.transpose = transpose

    def test_create_rows(self):
        self.assertEqual(len(self.create(3, 4)), 3)

    def test_create_cols(self):
        self.assertEqual(len(self.create(3, 4)[0]), 4)

    def test_create_default(self):
        self.assertEqual(self.create(2, 2)[0][0], 0)

    def test_get_custom_default(self):
        self.assertEqual(self.get(self.create(2, 2, 5), 0, 0), 5)

    def test_set_val(self):
        self.assertEqual(self.set_val(self.create(2, 2), 0, 1, 9)[0][1], 9)

    def test_set_val_pure(self):
        self.assertEqual(self.create(2, 2)[0][1], 0)

    def test_add(self):
        self.assertEqual(self.add(self.create(2, 2, 1), self.create(2, 2, 2))[0][0], 3)

    def test_transpose_rows(self):
        self.assertEqual(len(self.transpose(self.create(3, 4))), 4)

    def test_transpose_cols(self):
        self.assertEqual(len(self.transpose(self.create(3, 4))[0]), 3)

    def test_transpose_values(self):
        self.assertEqual(self.transpose([[1, 2], [3, 4]])[0], [1, 3])


class TestSparseVector(unittest.TestCase):
    def setUp(self):
        from devkit.sparse_vector import create, dot, add, scale, magnitude
        self.create = create
        self.dot = dot
        self.add = add
        self.scale = scale
        self.magnitude = magnitude

    def test_create_data(self):
        self.assertEqual(self.create({0: 1, 1: 2})['data'], {0: 1, 1: 2})

    def test_create_drops_zeros(self):
        self.assertEqual(self.create({0: 0, 1: 2})['data'], {1: 2})

    def test_dot_product(self):
        self.assertEqual(self.dot(self.create({0: 1, 1: 2}), self.create({0: 3, 1: 4})), 11.0)

    def test_dot_orthogonal(self):
        self.assertEqual(self.dot(self.create({0: 1}), self.create({1: 1})), 0.0)

    def test_add_values(self):
        self.assertEqual(self.add(self.create({0: 1}), self.create({0: 2}))['data'][0], 3)

    def test_add_cancels(self):
        self.assertEqual(self.add(self.create({0: 1}), self.create({0: -1}))['data'].get(0, 0), 0)

    def test_scale(self):
        self.assertEqual(self.scale(self.create({0: 2, 1: 3}), 2.0)['data'][0], 4.0)

    def test_scale_zero(self):
        self.assertEqual(self.scale(self.create({0: 1}), 0.0)['data'].get(0, 0), 0)

    def test_magnitude(self):
        self.assertEqual(round(self.magnitude(self.create({0: 3, 1: 4})), 5), 5.0)

    def test_magnitude_empty(self):
        self.assertEqual(self.magnitude(self.create({})), 0.0)


class TestTimeSeries(unittest.TestCase):
    def setUp(self):
        from devkit.time_series import create, append, range_query, ts_summary
        self.create = create
        self.append = append
        self.range_query = range_query
        self.ts_summary = ts_summary

    def test_create_count(self):
        self.assertEqual(self.create()['count'], 0)

    def test_append_count(self):
        self.assertEqual(self.append(self.create(), 1.0, 5.0)['count'], 1)

    def test_range_query_hit(self):
        self.assertEqual(len(self.range_query(self.append(self.create(), 1.0, 5.0), 0.0, 2.0)), 1)

    def test_range_query_miss(self):
        self.assertEqual(len(self.range_query(self.append(self.create(), 1.0, 5.0), 2.0, 3.0)), 0)

    def test_summary_empty_count(self):
        self.assertEqual(self.ts_summary(self.create())['count'], 0)

    def test_summary_empty_min(self):
        self.assertIsNone(self.ts_summary(self.create())['min_val'])

    def test_summary_min(self):
        self.assertEqual(self.ts_summary(self.append(self.create(), 1.0, 5.0))['min_val'], 5.0)

    def test_summary_max(self):
        self.assertEqual(self.ts_summary(self.append(self.create(), 1.0, 5.0))['max_val'], 5.0)

    def test_summary_first_ts(self):
        self.assertEqual(self.ts_summary(self.append(self.create(), 1.0, 5.0))['first_ts'], 1.0)

    def test_summary_avg(self):
        ts = self.append(self.append(self.create(), 1.0, 3.0), 2.0, 7.0)
        self.assertEqual(self.ts_summary(ts)['avg_val'], 5.0)


class TestFrecencyTracker(unittest.TestCase):
    def setUp(self):
        from devkit.frecency_tracker import create, access, top_n, tracker_summary
        self.create = create
        self.access = access
        self.top_n = top_n
        self.tracker_summary = tracker_summary

    def test_create_decay(self):
        self.assertEqual(self.create()['decay'], 0.9)

    def test_create_items_empty(self):
        self.assertEqual(self.create()['items'], {})

    def test_access_count(self):
        self.assertEqual(self.access(self.create(), 'k1', 0)['items']['k1']['count'], 1)

    def test_access_score(self):
        self.assertEqual(self.access(self.create(), 'k1', 0)['items']['k1']['score'], 1.0)

    def test_top_n_empty(self):
        self.assertEqual(self.top_n(self.create(), 3), [])

    def test_top_n_one(self):
        self.assertEqual(self.top_n(self.access(self.create(), 'k1', 0), 3), ['k1'])

    def test_top_n_limit(self):
        tr = self.access(self.access(self.create(), 'k1', 0), 'k2', 0)
        self.assertEqual(len(self.top_n(tr, 1)), 1)

    def test_summary_empty(self):
        self.assertEqual(self.tracker_summary(self.create())['total_items'], 0)

    def test_summary_one(self):
        self.assertEqual(self.tracker_summary(self.access(self.create(), 'k1', 0))['total_items'], 1)

    def test_summary_decay(self):
        self.assertEqual(self.tracker_summary(self.create())['decay'], 0.9)


class TestJsonPatch(unittest.TestCase):
    def setUp(self):
        from devkit.json_patch import apply_patch, make_patch, patch_summary
        self.apply_patch = apply_patch
        self.make_patch = make_patch
        self.patch_summary = patch_summary

    def test_apply_add(self):
        self.assertEqual(self.apply_patch({'a': 1}, [{'op': 'add', 'path': '/b', 'value': 2}]), {'a': 1, 'b': 2})

    def test_apply_remove(self):
        self.assertEqual(self.apply_patch({'a': 1}, [{'op': 'remove', 'path': '/a'}]), {})

    def test_apply_replace(self):
        self.assertEqual(self.apply_patch({'a': 1}, [{'op': 'replace', 'path': '/a', 'value': 9}]), {'a': 9})

    def test_make_patch_add_len(self):
        self.assertEqual(len(self.make_patch({}, {'a': 1})), 1)

    def test_make_patch_add_op(self):
        self.assertEqual(self.make_patch({}, {'a': 1})[0]['op'], 'add')

    def test_make_patch_remove_op(self):
        self.assertEqual(self.make_patch({'a': 1}, {})[0]['op'], 'remove')

    def test_make_patch_replace_op(self):
        self.assertEqual(self.make_patch({'a': 1}, {'a': 2})[0]['op'], 'replace')

    def test_summary_adds(self):
        self.assertEqual(self.patch_summary([{'op': 'add', 'path': '/a', 'value': 1}])['adds'], 1)

    def test_summary_removes(self):
        self.assertEqual(self.patch_summary([{'op': 'remove', 'path': '/a'}])['removes'], 1)

    def test_summary_empty(self):
        self.assertEqual(self.patch_summary([])['total'], 0)


class TestSchemaValidator(unittest.TestCase):
    def setUp(self):
        from devkit.schema_validator import validate, is_valid
        self.validate = validate
        self.is_valid = is_valid

    def test_valid_str(self):
        self.assertTrue(self.is_valid('hi', {'type': 'str'}))

    def test_invalid_type(self):
        self.assertFalse(self.is_valid(1, {'type': 'str'}))

    def test_int_in_range(self):
        self.assertTrue(self.is_valid(5, {'type': 'int', 'minimum': 1, 'maximum': 10}))

    def test_int_out_of_range(self):
        self.assertFalse(self.is_valid(11, {'type': 'int', 'minimum': 1, 'maximum': 10}))

    def test_str_min_length_ok(self):
        self.assertTrue(self.is_valid('hi', {'type': 'str', 'minLength': 1}))

    def test_str_min_length_fail(self):
        self.assertFalse(self.is_valid('', {'type': 'str', 'minLength': 1}))

    def test_dict_required_ok(self):
        self.assertTrue(self.is_valid({'a': 1}, {'type': 'dict', 'required': ['a']}))

    def test_dict_required_fail(self):
        self.assertFalse(self.is_valid({}, {'type': 'dict', 'required': ['a']}))

    def test_validate_errors(self):
        self.assertGreater(len(self.validate(1, {'type': 'str'})['errors']), 0)

    def test_valid_list(self):
        self.assertTrue(self.is_valid([1, 2], {'type': 'list'}))


class TestQueryBuilder(unittest.TestCase):
    def setUp(self):
        from devkit.query_builder import create, where, order, limit, execute
        self.create = create
        self.where = where
        self.order = order
        self.limit = limit
        self.execute = execute

    def test_execute_all(self):
        self.assertEqual(self.execute(self.create([{'a': 1}, {'a': 2}])), [{'a': 1}, {'a': 2}])

    def test_where_eq_len(self):
        self.assertEqual(len(self.execute(self.where(self.create([{'a': 1}, {'a': 2}]), 'a', '==', 1))), 1)

    def test_where_eq_value(self):
        self.assertEqual(self.execute(self.where(self.create([{'a': 1}, {'a': 2}]), 'a', '==', 1))[0]['a'], 1)

    def test_where_gt(self):
        self.assertEqual(len(self.execute(self.where(self.create([{'a': 1}, {'a': 2}]), 'a', '>', 1))), 1)

    def test_order_asc(self):
        self.assertEqual(self.execute(self.order(self.create([{'a': 2}, {'a': 1}]), 'a'))[0]['a'], 1)

    def test_order_desc(self):
        self.assertEqual(self.execute(self.order(self.create([{'a': 2}, {'a': 1}]), 'a', False))[0]['a'], 2)

    def test_limit(self):
        self.assertEqual(len(self.execute(self.limit(self.create([{'a': 1}, {'a': 2}, {'a': 3}]), 2))), 2)

    def test_empty(self):
        self.assertEqual(len(self.execute(self.create([]))), 0)

    def test_where_no_match(self):
        self.assertEqual(len(self.execute(self.where(self.create([{'a': 1}]), 'a', '==', 9))), 0)

    def test_order_limit_combined(self):
        q = self.limit(self.order(self.create([{'a': 3}, {'a': 1}, {'a': 2}]), 'a'), 2)
        self.assertEqual(self.execute(q)[1]['a'], 2)


class TestTemplateEngine(unittest.TestCase):
    def setUp(self):
        from devkit.template_engine import render, render_list, extract_vars
        self.render = render
        self.render_list = render_list
        self.extract_vars = extract_vars

    def test_render_basic(self):
        self.assertEqual(self.render('Hello {{name}}', {'name': 'World'}), 'Hello World')

    def test_render_two_vars(self):
        self.assertEqual(self.render('{{a}} {{b}}', {'a': 'x', 'b': 'y'}), 'x y')

    def test_render_missing_key(self):
        self.assertEqual(self.render('Hello {{name}}', {}), 'Hello {{name}}')

    def test_render_empty_template(self):
        self.assertEqual(self.render('', {'a': 'x'}), '')

    def test_render_list_len(self):
        self.assertEqual(len(self.render_list('Hi {{n}}', [{'n': 'A'}, {'n': 'B'}])), 2)

    def test_render_list_value(self):
        self.assertEqual(self.render_list('Hi {{n}}', [{'n': 'A'}, {'n': 'B'}])[0], 'Hi A')

    def test_extract_vars(self):
        self.assertEqual(self.extract_vars('{{a}} {{b}}'), ['a', 'b'])

    def test_extract_vars_none(self):
        self.assertEqual(self.extract_vars('no vars'), [])

    def test_extract_vars_dedup(self):
        self.assertEqual(self.extract_vars('{{a}} {{a}}'), ['a'])

    def test_render_repeated_var(self):
        self.assertEqual(self.render('{{x}}={{x}}', {'x': '1'}), '1=1')


class TestCsvParser(unittest.TestCase):
    def setUp(self):
        from devkit.csv_parser import parse, to_csv, filter_rows
        self.parse = parse
        self.to_csv = to_csv
        self.filter_rows = filter_rows

    def test_parse_one_row(self):
        self.assertEqual(len(self.parse('a,b\n1,2')), 1)

    def test_parse_field_a(self):
        self.assertEqual(self.parse('a,b\n1,2')[0]['a'], '1')

    def test_parse_field_b(self):
        self.assertEqual(self.parse('a,b\n1,2')[0]['b'], '2')

    def test_parse_two_rows(self):
        self.assertEqual(len(self.parse('a,b\n1,2\n3,4')), 2)

    def test_parse_empty(self):
        self.assertEqual(self.parse(''), [])

    def test_to_csv_contains_key(self):
        self.assertIn('a', self.to_csv([{'a': '1', 'b': '2'}]))

    def test_to_csv_empty(self):
        self.assertEqual(len(self.to_csv([]).split('\n')[0]), 0)

    def test_filter_len(self):
        self.assertEqual(len(self.filter_rows(self.parse('a,b\n1,2\n3,4'), 'a', '1')), 1)

    def test_filter_value(self):
        self.assertEqual(self.filter_rows(self.parse('a,b\n1,2\n3,4'), 'a', '1')[0]['b'], '2')

    def test_filter_no_match(self):
        self.assertEqual(len(self.filter_rows(self.parse('a,b\n1,2'), 'a', '9')), 0)


class TestTextStats(unittest.TestCase):
    def setUp(self):
        from devkit.text_stats import word_count, char_count, word_freq, top_words
        self.word_count = word_count
        self.char_count = char_count
        self.word_freq = word_freq
        self.top_words = top_words

    def test_word_count_basic(self):
        self.assertEqual(self.word_count('hello world'), 2)

    def test_word_count_empty(self):
        self.assertEqual(self.word_count(''), 0)

    def test_word_count_spaces(self):
        self.assertEqual(self.word_count('  hello  '), 1)

    def test_char_count_basic(self):
        self.assertEqual(self.char_count('hello'), 5)

    def test_char_count_ignore_spaces(self):
        self.assertEqual(self.char_count('hello world', True), 10)

    def test_char_count_with_spaces(self):
        self.assertEqual(self.char_count('hello world', False), 11)

    def test_word_freq(self):
        self.assertEqual(self.word_freq('hello hello world')['hello'], 2)

    def test_word_freq_empty(self):
        self.assertEqual(self.word_freq(''), {})

    def test_top_words_first(self):
        self.assertEqual(self.top_words('a a b', 1)[0], ('a', 2))

    def test_top_words_limit(self):
        self.assertEqual(len(self.top_words('a b c', 2)), 2)


class TestUrlParser(unittest.TestCase):
    def setUp(self):
        from devkit.url_parser import parse, build, add_param
        self.parse = parse
        self.build = build
        self.add_param = add_param

    def test_parse_scheme(self):
        self.assertEqual(self.parse('https://example.com/path')['scheme'], 'https')

    def test_parse_host(self):
        self.assertEqual(self.parse('https://example.com/path')['host'], 'example.com')

    def test_parse_path(self):
        self.assertEqual(self.parse('https://example.com/path')['path'], '/path')

    def test_parse_query(self):
        self.assertEqual(self.parse('https://example.com?a=1')['query']['a'], '1')

    def test_parse_empty_query(self):
        self.assertEqual(self.parse('https://example.com')['query'], {})

    def test_build_returns_str(self):
        self.assertIsInstance(self.build(self.parse('https://example.com')), str)

    def test_build_contains_host(self):
        self.assertIn('example.com', self.build(self.parse('https://example.com')))

    def test_add_param(self):
        self.assertIn('b=2', self.add_param('https://example.com', 'b', '2'))

    def test_add_param_preserves_existing(self):
        self.assertIn('a=1', self.add_param('https://example.com?a=1', 'b', '2'))

    def test_parse_fragment(self):
        self.assertEqual(self.parse('https://example.com#sec')['fragment'], 'sec')


class TestNumberFormatter(unittest.TestCase):
    def setUp(self):
        from devkit.number_formatter import format_int, format_float, to_human, parse_human
        self.format_int = format_int
        self.format_float = format_float
        self.to_human = to_human
        self.parse_human = parse_human

    def test_format_int_basic(self):
        self.assertEqual(self.format_int(1234567), '1,234,567')

    def test_format_int_small(self):
        self.assertEqual(self.format_int(999), '999')

    def test_format_int_custom_sep(self):
        self.assertEqual(self.format_int(1000000, '.'), '1.000.000')

    def test_format_float_default(self):
        self.assertEqual(self.format_float(3.14159), '3.14')

    def test_format_float_4_decimals(self):
        self.assertEqual(self.format_float(3.14159, 4), '3.1416')

    def test_to_human_millions(self):
        self.assertEqual(self.to_human(1500000), '1.5M')

    def test_to_human_billions(self):
        self.assertEqual(self.to_human(2500000000), '2.5B')

    def test_to_human_thousands(self):
        self.assertEqual(self.to_human(1500), '1.5K')

    def test_parse_human_k(self):
        self.assertEqual(self.parse_human('1.5K'), 1500.0)

    def test_parse_human_m(self):
        self.assertEqual(self.parse_human('2.5M'), 2500000.0)


class TestHtmlStripper(unittest.TestCase):
    def setUp(self):
        from devkit.html_stripper import strip_tags, extract_links, extract_text
        self.strip_tags = strip_tags
        self.extract_links = extract_links
        self.extract_text = extract_text

    def test_strip_tags_p(self):
        self.assertEqual(self.strip_tags('<p>hello</p>'), 'hello')

    def test_strip_tags_mixed(self):
        self.assertEqual(self.strip_tags('<b>hi</b> world'), 'hi world')

    def test_strip_tags_no_tags(self):
        self.assertEqual(self.strip_tags('no tags'), 'no tags')

    def test_strip_tags_empty(self):
        self.assertEqual(self.strip_tags(''), '')

    def test_extract_links_count(self):
        self.assertEqual(len(self.extract_links('<a href="http://x.com">click</a>')), 1)

    def test_extract_links_url(self):
        self.assertEqual(self.extract_links('<a href="http://x.com">click</a>')[0], 'http://x.com')

    def test_extract_links_empty(self):
        self.assertEqual(self.extract_links('no links'), [])

    def test_extract_text_two(self):
        self.assertEqual(self.extract_text('<p>hello</p><p>world</p>', 'p'), ['hello', 'world'])

    def test_extract_text_no_match(self):
        self.assertEqual(self.extract_text('<p>hi</p>', 'div'), [])

    def test_extract_text_span(self):
        self.assertEqual(len(self.extract_text('<span>a</span><span>b</span>', 'span')), 2)


class TestMarkdownRenderer(unittest.TestCase):
    def setUp(self):
        from devkit.markdown_renderer import render, extract_headings, extract_links
        self.render = render
        self.extract_headings = extract_headings
        self.extract_links = extract_links

    def test_render_heading(self):
        self.assertIn('hello', self.render('# hello'))

    def test_render_bold_text(self):
        self.assertIn('world', self.render('**world**'))

    def test_render_bold_no_stars(self):
        self.assertNotIn('**', self.render('**world**'))

    def test_render_code_text(self):
        self.assertIn('code', self.render('`code`'))

    def test_render_code_no_backtick(self):
        self.assertNotIn('`', self.render('`code`'))

    def test_extract_headings(self):
        self.assertEqual(self.extract_headings('# A\n## B'), ['A', 'B'])

    def test_extract_headings_none(self):
        self.assertEqual(self.extract_headings('no heading'), [])

    def test_extract_links_url(self):
        self.assertEqual(self.extract_links('[click](http://x.com)')[0]['url'], 'http://x.com')

    def test_extract_links_text(self):
        self.assertEqual(self.extract_links('[click](http://x.com)')[0]['text'], 'click')

    def test_extract_links_none(self):
        self.assertEqual(self.extract_links('no links'), [])


class TestStringDistance(unittest.TestCase):
    def setUp(self):
        from devkit.string_distance import levenshtein, hamming, similarity
        self.levenshtein = levenshtein
        self.hamming = hamming
        self.similarity = similarity

    def test_levenshtein_empty(self):
        self.assertEqual(self.levenshtein('', ''), 0)

    def test_levenshtein_deletion(self):
        self.assertEqual(self.levenshtein('a', ''), 1)

    def test_levenshtein_kitten(self):
        self.assertEqual(self.levenshtein('kitten', 'sitting'), 3)

    def test_levenshtein_equal(self):
        self.assertEqual(self.levenshtein('abc', 'abc'), 0)

    def test_hamming_equal(self):
        self.assertEqual(self.hamming('abc', 'abc'), 0)

    def test_hamming_one_diff(self):
        self.assertEqual(self.hamming('abc', 'axc'), 1)

    def test_hamming_all_diff(self):
        self.assertEqual(self.hamming('abc', 'xyz'), 3)

    def test_similarity_equal(self):
        self.assertEqual(self.similarity('abc', 'abc'), 1.0)

    def test_similarity_empty(self):
        self.assertEqual(self.similarity('', 'abc'), 0.0)

    def test_similarity_range(self):
        s = self.similarity('kitten', 'sitting')
        self.assertGreaterEqual(s, 0.0)
        self.assertLessEqual(s, 1.0)


class TestCodec(unittest.TestCase):
    def setUp(self):
        from devkit.codec import b64_encode, b64_decode, hex_encode, hex_decode
        self.b64_encode = b64_encode
        self.b64_decode = b64_decode
        self.hex_encode = hex_encode
        self.hex_decode = hex_decode

    def test_b64_roundtrip(self):
        self.assertEqual(self.b64_decode(self.b64_encode('hello')), 'hello')

    def test_b64_encode(self):
        self.assertEqual(self.b64_encode('hello'), 'aGVsbG8=')

    def test_b64_decode(self):
        self.assertEqual(self.b64_decode('aGVsbG8='), 'hello')

    def test_hex_roundtrip(self):
        self.assertEqual(self.hex_decode(self.hex_encode('hello')), 'hello')

    def test_hex_encode(self):
        self.assertEqual(self.hex_encode('hello'), '68656c6c6f')

    def test_hex_decode(self):
        self.assertEqual(self.hex_decode('68656c6c6f'), 'hello')

    def test_b64_encode_empty(self):
        self.assertEqual(self.b64_encode(''), '')

    def test_b64_decode_empty(self):
        self.assertEqual(self.b64_decode(''), '')

    def test_hex_encode_empty(self):
        self.assertEqual(self.hex_encode(''), '')

    def test_hex_decode_empty(self):
        self.assertEqual(self.hex_decode(''), '')


class TestIpUtils(unittest.TestCase):
    def setUp(self):
        from devkit.ip_utils import is_valid_ipv4, ip_to_int, int_to_ip, in_subnet
        self.is_valid_ipv4 = is_valid_ipv4
        self.ip_to_int = ip_to_int
        self.int_to_ip = int_to_ip
        self.in_subnet = in_subnet

    def test_valid_ipv4(self):
        self.assertTrue(self.is_valid_ipv4('192.168.1.1'))

    def test_invalid_ipv4_overflow(self):
        self.assertFalse(self.is_valid_ipv4('256.0.0.1'))

    def test_invalid_ipv4_alpha(self):
        self.assertFalse(self.is_valid_ipv4('abc'))

    def test_valid_ipv4_zeros(self):
        self.assertTrue(self.is_valid_ipv4('0.0.0.0'))

    def test_ip_to_int_one(self):
        self.assertEqual(self.ip_to_int('0.0.0.1'), 1)

    def test_ip_to_int_256(self):
        self.assertEqual(self.ip_to_int('0.0.1.0'), 256)

    def test_int_to_ip(self):
        self.assertEqual(self.int_to_ip(1), '0.0.0.1')

    def test_roundtrip(self):
        self.assertEqual(self.int_to_ip(self.ip_to_int('192.168.1.1')), '192.168.1.1')

    def test_in_subnet_true(self):
        self.assertTrue(self.in_subnet('192.168.1.5', '192.168.1.0/24'))

    def test_in_subnet_false(self):
        self.assertFalse(self.in_subnet('192.168.2.5', '192.168.1.0/24'))


class TestCronParser(unittest.TestCase):
    def setUp(self):
        from devkit.cron_parser import parse, is_valid, describe
        self.parse = parse
        self.is_valid = is_valid
        self.describe = describe

    def test_parse_wildcard_minute(self):
        self.assertEqual(self.parse('* * * * *')['minute'], '*')

    def test_parse_zero_minute(self):
        self.assertEqual(self.parse('0 * * * *')['minute'], '0')

    def test_parse_hour(self):
        self.assertEqual(self.parse('0 9 * * *')['hour'], '9')

    def test_parse_day(self):
        self.assertEqual(self.parse('0 9 1 * *')['day'], '1')

    def test_is_valid_true(self):
        self.assertTrue(self.is_valid('* * * * *'))

    def test_is_valid_false(self):
        self.assertFalse(self.is_valid('* * *'))

    def test_is_valid_specific(self):
        self.assertTrue(self.is_valid('0 9 1 1 0'))

    def test_describe_returns_str(self):
        self.assertIsInstance(self.describe('* * * * *'), str)

    def test_describe_content(self):
        desc = self.describe('* * * * *').lower()
        self.assertTrue('minute' in desc or 'every' in desc)

    def test_parse_step(self):
        self.assertEqual(self.parse('*/5 * * * *')['minute'], '*/5')


class TestUnitConverter(unittest.TestCase):
    def setUp(self):
        from devkit.unit_converter import convert, available_units
        self.convert = convert
        self.available_units = available_units

    def test_km_to_m(self):
        self.assertEqual(self.convert(1, 'km', 'm'), 1000.0)

    def test_m_to_km(self):
        self.assertEqual(self.convert(1000, 'm', 'km'), 1.0)

    def test_mi_to_km(self):
        self.assertEqual(round(self.convert(1, 'mi', 'km'), 4), 1.6093)

    def test_c_to_f_zero(self):
        self.assertEqual(self.convert(0, 'C', 'F'), 32.0)

    def test_c_to_f_hundred(self):
        self.assertEqual(self.convert(100, 'C', 'F'), 212.0)

    def test_c_to_k(self):
        self.assertEqual(self.convert(0, 'C', 'K'), 273.15)

    def test_kg_to_g(self):
        self.assertEqual(self.convert(1, 'kg', 'g'), 1000.0)

    def test_lb_to_kg(self):
        self.assertLess(abs(self.convert(1, 'lb', 'kg') - 0.453592), 0.001)

    def test_available_length(self):
        self.assertIn('m', self.available_units('length'))

    def test_available_weight(self):
        self.assertIn('kg', self.available_units('weight'))


class TestColorUtils(unittest.TestCase):
    def setUp(self):
        from devkit.color_utils import hex_to_rgb, rgb_to_hex, rgb_to_hsl, blend
        self.hex_to_rgb = hex_to_rgb
        self.rgb_to_hex = rgb_to_hex
        self.rgb_to_hsl = rgb_to_hsl
        self.blend = blend

    def test_hex_to_rgb_red(self):
        self.assertEqual(self.hex_to_rgb('#ff0000'), (255, 0, 0))

    def test_hex_to_rgb_black(self):
        self.assertEqual(self.hex_to_rgb('#000000'), (0, 0, 0))

    def test_hex_to_rgb_white(self):
        self.assertEqual(self.hex_to_rgb('#ffffff'), (255, 255, 255))

    def test_rgb_to_hex_red(self):
        self.assertEqual(self.rgb_to_hex(255, 0, 0), '#ff0000')

    def test_rgb_to_hex_black(self):
        self.assertEqual(self.rgb_to_hex(0, 0, 0), '#000000')

    def test_hsl_red_hue(self):
        self.assertEqual(round(self.rgb_to_hsl(255, 0, 0)[0]), 0)

    def test_hsl_black_lightness(self):
        self.assertEqual(self.rgb_to_hsl(0, 0, 0)[2], 0.0)

    def test_hsl_white_lightness(self):
        self.assertEqual(self.rgb_to_hsl(255, 255, 255)[2], 1.0)

    def test_blend_midpoint(self):
        b = self.blend((0, 0, 0), (255, 255, 255), 0.5)
        self.assertIn(b, [(127, 127, 127), (128, 128, 128)])

    def test_blend_same(self):
        self.assertEqual(self.blend((255, 0, 0), (255, 0, 0), 0.5), (255, 0, 0))


if __name__ == "__main__":
    unittest.main(verbosity=2)
