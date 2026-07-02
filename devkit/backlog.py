"""
Initializer + feature backlog 单特性增量（借鉴 Anthropic harness 阶段二/三）。

  devkit backlog "<应用想法>"   → Initializer：规划者把想法拆成可独立交付/可测的特性清单
                                  （backlog.json，全部 todo）+ progress.md，建项目目录。
  devkit feature                → 取**优先级最高的未完成特性**，读当前代码库 + 进度，
                                  增量实现这一个特性、跑测试、绿了才标 done 并记进度（可迭代）。

强制增量 / 强制测试 / 干净交接：每次只动一个特性、跑测试通过才算数、进度落盘可恢复。
状态全在项目目录里（backlog.json + progress.md + 代码），下次接着干。
"""
from __future__ import annotations

import json
import pathlib
import re
from datetime import datetime
from typing import List, Optional

from devkit.rdloop import ROOT

PROJECTS = ROOT / "devkit" / "projects"


def _slug(idea: str) -> str:
    s = re.sub(r"[^\w一-鿿]+", "-", idea.strip().lower())[:40].strip("-")
    return s or "app"


def project_dir(idea_or_slug: str) -> pathlib.Path:
    return PROJECTS / _slug(idea_or_slug)


def load_backlog(pd: pathlib.Path) -> dict:
    p = pathlib.Path(pd) / "backlog.json"
    if not p.is_file():
        raise FileNotFoundError(f"没有 backlog.json（先 `devkit backlog \"想法\"` 初始化）：{p}")
    return json.loads(p.read_text(encoding="utf-8"))


def save_backlog(pd: pathlib.Path, bl: dict) -> None:
    (pathlib.Path(pd) / "backlog.json").write_text(
        json.dumps(bl, ensure_ascii=False, indent=2), encoding="utf-8")


def next_todo(bl: dict) -> Optional[dict]:
    todo = [f for f in bl.get("features", []) if f.get("status") == "todo"]
    todo.sort(key=lambda f: f.get("priority", 99))
    return todo[0] if todo else None


def counts(bl: dict):
    feats = bl.get("features", [])
    done = sum(1 for f in feats if f.get("status") == "done")
    return done, len(feats)


# --------------------------------------------------------------------------- #
# Initializer
# --------------------------------------------------------------------------- #
_INIT_SYS = (
    "你是【编排/规划】角色。把一个应用想法拆成一份**可增量交付**的特性清单。"
    "每个特性都必须是一个**具体、可独立实现、能写出测试断言**的行为——"
    "即某个函数/接口的明确输入→输出（例如「reverse(s) 返回逆序字符串」「is_palindrome(s) 判断回文返回布尔」）。"
    "**第一个特性就要是能跑能测的最小可用切片**；"
    "**严禁**出现「搭建项目骨架 / 设计核心数据模型 / 基础设施 / 架构设计 / 初始化环境」这类抽象项——"
    "它们写不出测试，会让构建者空转。需要的结构由具体特性自带。")
_INIT_FMT = (
    "只输出一个 JSON 数组，不要解释。每个元素：{{\"id\":\"f1\",\"title\":\"一句话特性\",\"priority\":1}}。"
    "给 {n} 个左右，priority 越小越先做。"
    "title 要具体到据此能直接写 assert（如「count_words(s) 返回单词数」），"
    "不要写「实现核心逻辑 / 完善功能」这种没法测的话。")


def init_backlog(idea: str, base_url: str, api_key: str, carrier: str = "loom-orchestrator",
                 n: int = 8, pd: Optional[pathlib.Path] = None) -> dict:
    from devkit.rdloop import gateway_chat
    pd = pathlib.Path(pd) if pd else project_dir(idea)
    pd.mkdir(parents=True, exist_ok=True)
    user = f"## 应用想法\n{idea}\n\n## 输出格式\n{_INIT_FMT.format(n=n)}"
    ok, content, _sv, tokens, cost = gateway_chat(
        base_url, api_key, carrier, _INIT_SYS, user, 1100, tags=["backlog", "initializer"])
    feats = []
    if ok:
        m = re.search(r"\[.*\]", re.sub(r"```(?:json)?", "", content), re.S)
        if m:
            try:
                raw = json.loads(m.group(0))
                for i, f in enumerate(raw, 1):
                    if isinstance(f, dict) and f.get("title"):
                        feats.append({"id": f.get("id") or f"f{i}",
                                      "title": str(f["title"]).strip(),
                                      "priority": int(f.get("priority", i)),
                                      "status": "todo"})
            except Exception:  # noqa: BLE001
                pass
    bl = {"idea": idea, "created": datetime.now().strftime("%Y-%m-%d %H:%M:%S"), "features": feats}
    save_backlog(pd, bl)
    (pd / "progress.md").write_text(
        f"# {idea}\n\n初始化 {datetime.now():%Y-%m-%d %H:%M}，共 {len(feats)} 个特性（全部 todo）。\n\n"
        + "".join(f"- [ ] {f['id']} {f['title']}\n" for f in feats), encoding="utf-8")
    return {"project_dir": str(pd), "features": len(feats), "tokens": tokens, "cost": cost}


# --------------------------------------------------------------------------- #
# 单特性增量构建
# --------------------------------------------------------------------------- #
def _codebase_snapshot(pd: pathlib.Path, cap: int = 6000) -> str:
    parts, total = [], 0
    for f in sorted(pd.rglob("*")):
        rel = f.relative_to(pd)
        if (not f.is_file() or f.suffix in (".pyc", ".json", ".md", ".txt")
                or any(p.startswith((".", "_")) for p in rel.parts)):
            continue
        try:
            body = f.read_text(encoding="utf-8")
        except Exception:  # noqa: BLE001
            continue
        chunk = f"### {rel}\n```\n{body[:1500]}\n```\n"
        if total + len(chunk) > cap:
            break
        parts.append(chunk)
        total += len(chunk)
    return "\n".join(parts) or "（空项目，这是第一个特性）"


def _append_progress(pd: pathlib.Path, feat: dict, files: List[str], passed, extra: str = "") -> None:
    verdict = {True: "测试✅", False: "测试❌", None: "无测试"}[passed]
    with (pd / "progress.md").open("a", encoding="utf-8") as fh:
        fh.write(f"\n- [x] {feat['id']} {feat['title']} — {verdict}，"
                 f"文件：{', '.join(files) or '—'}{extra}（{datetime.now():%m-%d %H:%M}）")


# --------------------------------------------------------------------------- #
# 每特性一个 git 提交（就地，借鉴 Anthropic「Initializer git-init，每个特性一 commit」）
# --------------------------------------------------------------------------- #
_GITIGNORE = ("_deps/", "__pycache__/", "*.pyc", ".pytest_cache/")


def _git(pd: pathlib.Path, *args: str, timeout: int = 30):
    import subprocess
    return subprocess.run(["git", "-C", str(pd), *args],
                          capture_output=True, text=True, timeout=timeout)


def commit_feature(pd: pathlib.Path, feat: dict):
    """就地提交本特性：幂等 .gitignore + 幂等 git init + add -A + 带内联身份的 commit。
    返回 short hash；无改动返回 None；失败抛异常（由调用方 fail-open 兜住）。"""
    pd = pathlib.Path(pd)
    gi = pd / ".gitignore"
    lines = gi.read_text(encoding="utf-8").splitlines() if gi.exists() else []
    add = [x for x in _GITIGNORE if x not in lines]
    if add:                                            # 幂等：不重复、不覆盖用户内容
        gi.write_text("\n".join(lines + add).strip() + "\n", encoding="utf-8")
    if not (pd / ".git").exists():                     # 幂等 init：已是仓库就不动历史/分支
        _git(pd, "init", "-q")
    _git(pd, "add", "-A")
    msg = f"feat({feat['id']}): {feat['title']}"
    r = _git(pd, "-c", "user.email=loom@local", "-c", "user.name=Loom", "commit", "-q", "-m", msg)
    if r.returncode != 0:
        if "nothing to commit" in (r.stdout + r.stderr):
            return None                                # 无改动 → 跳过（非错误）
        raise RuntimeError((r.stderr or r.stdout or "git commit failed").strip()[:200])
    return _git(pd, "rev-parse", "--short", "HEAD").stdout.strip()


def build_feature(pd: pathlib.Path, base_url: str, api_key: str,
                  carrier: str = "loom-dev", iterate: int = 2, max_tokens: int = 1000,
                  commit: bool = False) -> dict:
    from devkit import apply as _apply
    from devkit.rdloop import (gateway_chat, normalize, _build_feedback,
                               NO_TOOLS_PREAMBLE, CONSTITUTION)
    pd = pathlib.Path(pd)
    bl = load_backlog(pd)
    feat = next_todo(bl)
    if not feat:
        done, total = counts(bl)
        return {"empty": True, "msg": f"backlog 全部完成 🎉（{done}/{total}）"}

    system = NO_TOOLS_PREAMBLE + CONSTITUTION + (
        "你是【开发】角色，在**增量构建**一个项目：只实现指定的这一个特性，"
        "**不破坏已有特性**，必须 TDD（给/更新测试）。给出新增或修改的完整文件。")
    base_user = (
        f"## 项目目标\n{bl['idea']}\n\n## 当前代码库\n{_codebase_snapshot(pd)}\n\n"
        f"## 本次只实现这一个特性\n[{feat['id']}] {feat['title']}\n\n"
        f"给出新增/修改的完整文件（每个用代码块，**第一行写 `# 相对路径`**），含对应测试 test_*.py。\n"
        f"约束：测试**只覆盖本特性已实现的行为**，不要测尚未规划的功能；用标准库 unittest 即可，"
        f"`tests/__init__.py` 若需要请留空。")
    tot_tk, tot_co, fail = 0, 0.0, ""
    for attempt in range(iterate + 1):
        user = base_user if attempt == 0 else base_user + "\n\n" + _build_feedback(fail, "")
        ok, content, _sv, tk, co = gateway_chat(base_url, api_key, carrier, system, user,
                                                max_tokens, tags=["backlog", feat["id"]])
        tot_tk += tk
        tot_co += co
        if not ok:
            return {"feature": feat, "error": str(content)[:200], "tokens": tot_tk, "cost": tot_co}
        files = _apply.materialize(normalize(content), pd)
        if not files:                            # 没物化出文件 = 没产出，别误判 done
            fail = "未识别到任何文件——每个代码块**第一行必须写 `# 相对路径`**（如 `# tip_calc/core.py`）。"
            continue
        passed, tout = _apply.run_tests(pd)
        if passed is not False:                  # 通过 或 无测试 → 接受（无测试给警告）
            feat["status"] = "done"
            save_backlog(pd, bl)                  # 先落 done，再提交（commit 含 done 状态）
            res = {"feature": feat, "files": files, "tests": passed, "attempts": attempt + 1,
                   "tokens": tot_tk, "cost": tot_co}
            note = ""
            if commit:                           # 每特性一 commit；fail-open：失败绝不回滚 done / 不崩
                try:
                    h = commit_feature(pd, feat)
                    if h:
                        res["commit"] = h
                        note = f"，commit {h}"
                    else:
                        note = "，无改动跳过提交"
                except Exception as e:           # noqa: BLE001
                    res["commit_error"] = str(e)[:160]
                    note = "，commit 失败"
            _append_progress(pd, feat, files, passed, note)
            done, total = counts(bl)
            res["done"], res["total"] = done, total
            return res
        fail = (f"### 测试失败\n```\n{tout[:1400]}\n```")
    return {"feature": feat, "tests": False, "attempts": iterate + 1,
            "fail": fail[-600:], "tokens": tot_tk, "cost": tot_co}
