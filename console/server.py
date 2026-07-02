#!/usr/bin/env python3
"""
Loom 开发套件 · 全局控制台（中枢）。

把分散的看板整合到一个页面：
  - devkit 流程：运行总台账 RUNS.md + 每次 run 的各阶段产物
  - 网关可观测：LiteLLM 用量/花费（/spend/logs 聚合）
  - 后端健康：5 个角色载体 → 厂商映射 + 各服务在线状态
  - 一键发起一次 devkit 流程运行
  - 深链：Agent UI(:3000) / LiteLLM UI(:4000/ui)

只用 Python 标准库；宿主机直接 `python3 -m console`（或 `python3 console/server.py`）。
"""
from __future__ import annotations

import json
import os
import pathlib
import re
import subprocess
import sys
import threading
import urllib.error
import urllib.request
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = pathlib.Path(__file__).resolve().parent.parent          # agent-platform/
RUNS_DIR = ROOT / "devkit" / "runs"
LEDGER = ROOT / "devkit" / "RUNS.md"
CONFIG = ROOT / "litellm" / "config.full.yaml"
HTML = pathlib.Path(__file__).resolve().parent / "index.html"
# 宿主机跑 → localhost；容器内跑 → compose 注入服务名（见 docker-compose.yml）。
GW = os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
AGENTOS_URL = os.environ.get("AGENTOS_URL", "http://localhost:8000")
AGENTUI_URL = os.environ.get("AGENTUI_URL", "http://localhost:3000")
CLIPROXY_URL = os.environ.get("CLIPROXY_URL", "http://localhost:8317")
PORT = int(os.environ.get("CONSOLE_PORT", "8899"))
# 可选访问令牌：设了就全量鉴权（用 ?token= 打开一次写 cookie）；不设则本地零摩擦。
CONSOLE_TOKEN = os.environ.get("CONSOLE_TOKEN", "")


# --------------------------------------------------------------------------- #
def master_key() -> str:
    if os.environ.get("LITELLM_MASTER_KEY"):
        return os.environ["LITELLM_MASTER_KEY"]
    env = ROOT / ".env"
    if env.exists():
        for line in env.read_text().splitlines():
            if line.startswith("LITELLM_MASTER_KEY="):
                return line.split("=", 1)[1].strip()
    return ""


def _get(url: str, headers: dict | None = None, timeout: int = 4):
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return json.loads(r.read())
    except Exception:  # noqa: BLE001
        return None


def _ping(url: str, headers: dict | None = None, timeout: int = 2) -> bool:
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status < 500
    except urllib.error.HTTPError as e:
        return e.code < 500          # 401/404 也算"在线"
    except Exception:  # noqa: BLE001
        return False


# --------------------------------------------------------------------------- #
def carrier_backends() -> dict:
    """从 config.full.yaml 解析 loom-* 载体 → 实际厂商模型。"""
    out = {}
    if CONFIG.exists():
        txt = CONFIG.read_text()
        for m in re.finditer(r"model_name:\s*(loom-[\w-]+)[\s\S]*?model:\s*([^\n#]+)", txt):
            out[m.group(1)] = m.group(2).strip()
    return out


def roles() -> list:
    """devkit 阶段 → 角色 → 载体 → 后端（读用户自定义角色，无则内置默认）。"""
    try:
        from devkit.roles import load_stages
        cb = carrier_backends()
        # carrier 是 loom-* 则查映射；直接写后端名（deepseek/...）则后端=它自己
        return [{"stage": s.key, "role": s.role.split("（")[0][:14], "carrier": s.carrier,
                 "backend": cb.get(s.carrier) or s.carrier, "title": s.title}
                for s in load_stages()]
    except Exception:  # noqa: BLE001
        return []


def roles_full() -> dict:
    """在线编辑器用：完整角色（含 system）+ 来源 + 保存目标 + carrier 候选。"""
    from devkit import roles as R
    err = ""
    try:
        stages = [{"key": s.key, "role": s.role, "title": s.title, "carrier": s.carrier,
                   "executor": s.executor, "max_tokens": s.max_tokens, "system": s.system}
                  for s in R.load_stages()]
    except Exception as e:  # noqa: BLE001
        stages, err = [], str(e)
    cb = carrier_backends()
    try:
        save_target = str(R.CONSOLE_ROLES_PATH.relative_to(ROOT))
    except ValueError:
        save_target = str(R.CONSOLE_ROLES_PATH)
    return {"stages": stages, "source": R.active_source(), "save_target": save_target,
            "carriers": list(cb.keys()) + list(REMAP_TARGETS.keys()), "error": err}


def save_roles(stages) -> dict:
    """校验并写控制台共享角色文件（与 CLI devkit 同源）。"""
    from devkit import roles as R
    try:
        return R.save_stages(stages)
    except Exception as e:  # noqa: BLE001
        return {"error": str(e)}


def reset_roles() -> dict:
    from devkit import roles as R
    return {"ok": True, "deleted": R.delete_console_roles()}


# 角色载体可改成这 5 个基础后端之一（写入 config.full.yaml，重启网关生效）
REMAP_TARGETS = {
    "codex-sub": ("openai/gpt-5.5", "http://cliproxy:8317/v1", "sk-cliproxy-local"),
    "glm": ("openai/glm-4.6", "https://open.bigmodel.cn/api/paas/v4", "os.environ/ZHIPU_API_KEY"),
    "minimax": ("minimax/abab6.5s-chat", None, "os.environ/MINIMAX_API_KEY"),
    "deepseek": ("deepseek/deepseek-chat", None, "os.environ/DEEPSEEK_API_KEY"),
}


def carriers() -> dict:
    """列出 loom-* 载体 + 当前后端 model；以及可选目标。"""
    return {"carriers": [{"carrier": k, "model": v} for k, v in carrier_backends().items()],
            "targets": list(REMAP_TARGETS)}


def remap_carrier(carrier: str, target: str) -> dict:
    """把某 loom-* 载体的后端改成某基础后端（写 config.full.yaml；需重启网关生效）。"""
    if not carrier or not carrier.startswith("loom-"):
        return {"error": "只能改 loom-* 角色载体"}
    if target not in REMAP_TARGETS:
        return {"error": f"target 需为 {list(REMAP_TARGETS)}"}
    if not CONFIG.exists():
        return {"error": "找不到 config.full.yaml"}
    model, api_base, api_key = REMAP_TARGETS[target]
    text = CONFIG.read_text()
    if f"model_name: {carrier}" not in text:
        return {"error": f"配置里没有 {carrier}"}
    # 只替换 litellm_params 那几行，保留 `- model_name:` 行（含注释）与块后空行 → 不破坏格式
    params = f"    litellm_params:\n      model: {model}\n"
    if api_base:
        params += f"      api_base: {api_base}\n"
    params += f"      api_key: {api_key}\n"
    pat = rf"(  - model_name: {re.escape(carrier)}[^\n]*\n)    litellm_params:[^\n]*\n(?:      [^\n]*\n)+"
    new = re.sub(pat, lambda m: m.group(1) + params, text)
    if new == text:
        return {"error": "未能定位该载体块（配置结构可能已改）"}
    try:
        CONFIG.write_text(new)
    except Exception as e:  # noqa: BLE001
        return {"error": f"写配置失败（mount 是否 rw？）：{e}"}
    return {"ok": True, "carrier": carrier, "target": target, "model": model,
            "restart": "docker compose restart litellm"}


def list_runs() -> list:
    runs = []
    if not RUNS_DIR.exists():
        return runs
    dirs = [p for p in RUNS_DIR.iterdir() if p.is_dir()]
    for d in sorted(dirs, key=lambda p: p.stat().st_mtime, reverse=True):  # 按真实时间排序，跨时区稳健
        task = (d / "00-task.md").read_text().replace("# 任务", "").strip()[:120] if (d / "00-task.md").exists() else ""
        gate, stages, tokens, cost = "?", [], 0, 0.0
        rl = d / "run-log.md"
        if rl.exists():
            t = rl.read_text()
            mg = re.search(r"## Gate 建议\s*\n+([^\n]+)", t)
            gate = mg.group(1).strip() if mg else "?"
            mu = re.search(r"用量合计：\*\*(\d+)\s*tokens\s*·\s*\$([\d.]+)\*\*", t)
            if mu:
                tokens, cost = int(mu.group(1)), float(mu.group(2))
            for row in re.findall(r"\|\s*(\w+)\s*\|\s*([\w.-]+)\s*\|\s*([\w./-]+)\s*\|\s*(\w[\w-]*)\s*\|\s*([\d.]+s)\s*\|", t):
                if row[0] != "阶段":
                    stages.append({"stage": row[0], "carrier": row[1], "model": row[2], "status": row[3], "dt": row[4]})
        runs.append({"ts": d.name, "task": task, "gate": gate, "stages": stages, "tokens": tokens, "cost": cost})
    return runs


APPLIED_DIR = ROOT / "devkit" / "applied"


def _tests_ok(d: pathlib.Path) -> bool:
    rl = d / "run-log.md"
    if not rl.exists():
        return False
    t = rl.read_text()
    mg = re.search(r"## Gate 建议\s*\n+([^\n]+)", t)
    gate = mg.group(1).strip() if mg else ""
    return gate.startswith("建议 GO") and "构建测试失败" not in gate


def _build_files(build: pathlib.Path) -> list:
    from devkit.diff import list_build_files   # 与 CLI 共用同一份排除规则
    return list_build_files(build)


def run_detail(ts: str) -> dict:
    d = RUNS_DIR / ts
    if not d.is_dir() or ".." in ts or "/" in ts:
        return {"error": "not found"}
    files = [{"name": f.name, "content": f.read_text()} for f in sorted(d.glob("*.md"))]
    build = None
    bdir = d / "build"
    if bdir.is_dir():
        tout = (bdir / "_test-output.txt").read_text() if (bdir / "_test-output.txt").exists() else ""
        bf = _build_files(bdir)
        contents = {}
        for n in bf:
            try:
                contents[n] = (bdir / n).read_text()[:6000]
            except Exception:  # noqa: BLE001
                contents[n] = "（无法读取）"
        build = {"files": bf, "contents": contents, "test_output": tout[-1500:], "tests_ok": _tests_ok(d)}
    done = (d / "run-log.md").exists()
    stages_done = [f.name.split("-", 1)[1].replace(".md", "")
                   for f in sorted(d.glob("[0-9][0-9]-*.md")) if not f.name.startswith("00-")]
    return {"ts": ts, "files": files, "build": build, "done": done, "stages_done": stages_done}


def apply_run(ts: str) -> dict:
    """人类门一键 apply：把 runs/<ts>/build 复制到 devkit/applied/<ts>（宿主机可见）。"""
    if ".." in ts or "/" in ts:
        return {"error": "bad ts"}
    d = RUNS_DIR / ts
    build = d / "build"
    if not build.is_dir():
        return {"error": "该运行没有 build 产物（implement 未产出代码）"}
    if not _tests_ok(d):
        return {"error": "测试未通过，拒绝 apply（人类门：先修绿再 apply）"}
    from devkit import apply as _apply
    target = APPLIED_DIR / ts
    applied = _apply.apply_files(build, str(target))
    try:
        rel = str(target.relative_to(ROOT))
    except ValueError:
        rel = str(target)
    return {"applied": applied, "target": rel}


def diff_runs(ts: str, against: str = "") -> dict:
    """改动预览：委托 devkit.diff（与 `python3 -m devkit diff` CLI 共用同一份实现）。"""
    from devkit.diff import diff_runs as _diff
    return _diff(RUNS_DIR, ts, against)


_LIVENESS_CACHE = {"t": 0.0, "data": None}


def liveness_panel(ttl: int = 60) -> dict:
    """后端真活性（关降级的真实推理探测）—— 委托 devkit.insight。
    缓存 ttl 秒：每次探测=5 个计费推理，避免轮询/刷新反复烧额度。"""
    import time
    if _LIVENESS_CACHE["data"] and time.time() - _LIVENESS_CACHE["t"] < ttl:
        return {**_LIVENESS_CACHE["data"], "cached": True}
    from devkit.insight import health
    try:
        rows = health(GW, master_key())   # 含 state/action/token_state；rows[].ok/detail 向后兼容
    except Exception as e:                # noqa: BLE001  防御：health 理论上不抛，真抛也不崩面板
        return {"rows": [], "dead": [], "token_unavailable": False, "ttl": ttl,
                "error": str(e)[:120], "cached": False}
    data = {"rows": rows,
            "dead": [r["backend"] for r in rows if r["state"] != "serving"],
            "token_unavailable": any(r.get("token_state") == "unavailable" for r in rows),
            "ttl": ttl}
    _LIVENESS_CACHE.update(t=time.time(), data=data)
    return {**data, "cached": False}


def quota_panel() -> dict:
    """额度薅羊毛：委托 devkit.insight（与 `devkit quota` CLI 同源）。"""
    from devkit.insight import quota_report
    return quota_report(GW, master_key())


def scores_panel() -> dict:
    """模型评分：委托 devkit.insight（与 `devkit scores` CLI 同源）。"""
    from devkit.insight import score_report
    return score_report()


def rate_backend(backend: str, up: bool) -> dict:
    """记一次 👍/👎（与 `devkit rate` 同源）。"""
    if not backend:
        return {"error": "缺 backend"}
    from devkit.insight import add_rating
    return {"ok": True, "backend": add_rating(backend, 1 if up else -1)}


def usage() -> dict:
    logs = _get(f"{GW}/spend/logs", {"Authorization": f"Bearer {master_key()}"}) or []
    agg = {}
    for r in logs:
        m = r.get("model", "?")
        a = agg.setdefault(m, {"model": m, "calls": 0, "tokens": 0, "spend": 0.0})
        a["calls"] += 1
        a["tokens"] += int(r.get("total_tokens", 0) or 0)
        a["spend"] += float(r.get("spend", 0) or 0)
    return {"total": len(logs), "by_model": sorted(agg.values(), key=lambda x: -x["calls"])}


def help_doc(lang: str = "zh") -> dict:
    name = "USAGE.en.md" if str(lang).startswith("en") else "USAGE.zh.md"
    return doc_file(name)


DOC_PATHS = {
    "CONSTITUTION.md": ROOT / "CONSTITUTION.md",
    "ROADMAP.md": ROOT / "ROADMAP.md",
    "LOOM-ROLES.md": ROOT / "LOOM-ROLES.md",
    "USAGE.zh.md": ROOT / "USAGE.zh.md",
    "USAGE.en.md": ROOT / "USAGE.en.md",
    "MEMORY.md": ROOT / "devkit" / "MEMORY.md",
}


def doc_file(name: str) -> dict:
    f = DOC_PATHS.get(name)
    if not f:
        return {"name": name, "content": "（未授权文档 / not allowed）"}
    return {"name": name, "content": f.read_text() if f.exists() else "（暂无 / empty）"}


def health() -> dict:
    return {
        "litellm": _ping(f"{GW}/health/liveliness"),
        "agentos": _ping(f"{AGENTOS_URL}/health"),
        "agent_ui": _ping(AGENTUI_URL),
        "cliproxy": _ping(f"{CLIPROXY_URL}/v1/models", {"Authorization": "Bearer sk-cliproxy-local"}),
    }


# Stage file prefixes in order: 01-brainstorm, 02-plan, 03-implement, 04-verify, 05-review
_STAGE_ORDER = ["brainstorm", "plan", "implement", "verify", "review"]


def progress_state() -> dict:
    """Return {run_id, stage, status, pct, message} for the most-recently-modified run."""
    if not RUNS_DIR.exists():
        return {"run_id": None, "stage": None, "status": "idle", "pct": 0, "message": "无运行中任务"}
    dirs = [d for d in RUNS_DIR.iterdir() if d.is_dir()]
    if not dirs:
        return {"run_id": None, "stage": None, "status": "idle", "pct": 0, "message": "无运行中任务"}
    latest = max(dirs, key=lambda d: d.stat().st_mtime)
    run_id = latest.name
    run_log = latest / "run-log.md"
    if run_log.exists():
        return {"run_id": run_id, "stage": "done", "status": "done", "pct": 100,
                "message": "已完成"}
    # Count completed stage files (e.g. 03-implement.md present → implement done)
    done_stages = []
    for f in sorted(latest.iterdir()):
        fname = f.name
        for s in _STAGE_ORDER:
            if s in fname and fname.endswith(".md") and fname != "00-task.md":
                if s not in done_stages:
                    done_stages.append(s)
    n_total = len(_STAGE_ORDER)
    n_done = len(done_stages)
    current_stage = _STAGE_ORDER[n_done] if n_done < n_total else _STAGE_ORDER[-1]
    pct = int(n_done / n_total * 100)
    # Check stream.log last line for live message
    message = f"{n_done}/{n_total} 阶段完成"
    stream = latest / "stream.log"
    if stream.exists():
        try:
            lines = stream.read_text(encoding="utf-8", errors="replace").splitlines()
            last = next((l for l in reversed(lines) if l.strip()), "")
            if last:
                message = last[:120]
        except OSError:
            pass
    status = "running" if n_done < n_total else "done"
    return {"run_id": run_id, "stage": current_stage, "status": status,
            "pct": pct, "message": message}


def backlog_health() -> dict:
    """从 devkit/backlog.json 读取并计算健康度指标。"""
    bl_path = ROOT / "devkit" / "backlog.json"
    try:
        items = json.loads(bl_path.read_text(encoding="utf-8"))
        if isinstance(items, dict):
            items = items.get("items", [])
        if not isinstance(items, list):
            items = []
    except Exception:
        items = []
    total = len(items)
    done = sum(1 for it in items if isinstance(it, dict) and it.get("status") == "done")
    failed = sum(1 for it in items if isinstance(it, dict) and it.get("status") == "failed")
    pending = max(0, total - done - failed)
    completion_pct = float(done / total) if total > 0 else 0.0
    health_score = float(max(0.0, min(1.0, (done - failed) / total)) if total > 0 else 0.0)
    return {"total": total, "done": done, "pending": pending, "failed": failed,
            "health_score": health_score, "completion_pct": completion_pct}


def agent_observability() -> dict:
    from devkit.agent_observability import collect
    return collect()


def artifact_chain(run_id: str) -> dict:
    """返回 {run_id, ok, stages: [{name, gate, carrier, tokens, cost_usd, files}]}。"""
    import re as _re
    if not run_id:
        return {"run_id": run_id, "ok": False, "stages": [], "error": "empty run_id"}
    run_dir = RUNS_DIR / run_id
    if not run_dir.is_dir():
        return {"run_id": run_id, "ok": False, "stages": [], "error": f"run not found: {run_id}"}

    _gate_re = _re.compile(r"(?im)^#\s*(?:Gate|gate)\s*:\s*(.+)$")
    _carrier_re = _re.compile(r"(?im)^#\s*(?:carrier|Carrier)\s*:\s*(.+)$")
    _tokens_re = _re.compile(r"(?im)^#\s*tokens\s*:\s*(\d+)$")
    _cost_re = _re.compile(r"(?im)^#\s*cost\s*:\s*\$?([\d.]+)$")

    def _first(pat, text, cast=str, default=None):
        m = pat.search(text)
        if not m:
            return default
        try:
            return cast(m.group(1).strip())
        except (ValueError, TypeError):
            return default

    all_files = sorted(run_dir.iterdir())
    stages = []
    for md_file in sorted(run_dir.glob("*.md")):
        try:
            text = md_file.read_text(encoding="utf-8", errors="replace")
        except OSError:
            text = ""
        stages.append({
            "name": md_file.stem,
            "gate": _first(_gate_re, text) or "unknown",
            "carrier": _first(_carrier_re, text) or "unknown",
            "tokens": _first(_tokens_re, text, int, 0),
            "cost_usd": _first(_cost_re, text, float, 0.0),
            "files": [f.name for f in all_files if f.is_file()],
        })
    return {"run_id": run_id, "ok": True, "stages": stages}


def ask_model(model: str, prompt: str) -> dict:
    """@ask-model（借鉴 Kode）：临时问某个载体/后端一句，返回答案 + 用量。

    核心实现在 devkit.ask（与 `python3 -m devkit ask` CLI 共用）；这里只做 UI 形状适配：
    单个 → 扁平返回（向后兼容）；多个 → {compare, results, tot_*}。
    """
    if not model or not prompt.strip():
        return {"error": "缺 model 或 prompt"}
    from devkit.ask import ask_models, ask_one_with_fallback
    models = [m.strip() for m in model.split(",") if m.strip()]
    if len(models) == 1:
        results = [ask_one_with_fallback(models, prompt, GW, master_key(), tag="console:ask")]
    else:
        results = ask_models(models, prompt, GW, master_key(), tag="console:ask")
    if len(models) == 1:
        r = results[0]
        return ({"error": r["error"]} if not r["ok"]
                else {"ok": True, "served": r["served"], "tokens": r["tokens"],
                      "cost": r["cost"], "content": r["content"]})
    return {"ok": True, "compare": True, "results": results,
            "tot_tokens": sum(r.get("tokens", 0) for r in results),
            "tot_cost": sum(r.get("cost", 0.0) for r in results)}
    return {"ok": True, "compare": True, "results": results,
            "tot_tokens": sum(r.get("tokens", 0) for r in results),
            "tot_cost": sum(r.get("cost", 0.0) for r in results)}


def executors_avail() -> dict:
    try:
        from devkit import executors as _ex
        return _ex.available()
    except Exception:  # noqa: BLE001
        return {"chat": True, "hermes": False, "openclaw": False}


def trigger_run(task: str, stages: str = "", carriers: list | None = None,
                executor: str = "", budget: str = "", iterate: str = "",
                contract: str = "") -> dict:
    if not task.strip():
        return {"ok": False, "msg": "任务为空"}
    ts = datetime.now().strftime("%Y%m%d-%H%M%S")  # 控制台控制 run-id → 可实时跟踪
    cmd = [sys.executable, "-m", "devkit", task, "--run-id", ts]
    if stages.strip():
        cmd += ["--stages", stages.strip()]
    for c in (carriers or []):
        cmd += ["--carrier", c]
    if executor in ("hermes", "openclaw"):
        cmd += ["--executor", f"implement={executor}"]
    try:                                  # 软预算护栏（可选）：超了停剩余阶段并 NO-GO
        if budget and float(budget) > 0:
            cmd += ["--budget", str(float(budget))]
    except (TypeError, ValueError):
        pass
    try:                                  # 迭代循环（可选）：评判 NO-GO 回灌修复，最多 N 轮
        if iterate and int(iterate) > 0:
            cmd += ["--iterate", str(int(iterate))]
    except (TypeError, ValueError):
        pass
    try:                                  # Sprint Contract（可选）：先约定 N 条验收点
        if contract and int(contract) > 0:
            cmd += ["--contract", str(int(contract))]
    except (TypeError, ValueError):
        pass
    run_dir = RUNS_DIR / ts
    run_dir.mkdir(parents=True, exist_ok=True)
    stream_log = run_dir / "stream.log"
    logf_stream = stream_log.open("w")
    logf_shared = (ROOT / "devkit" / "console-trigger.log").open("a")
    proc = subprocess.Popen(cmd, cwd=str(ROOT), stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    def _tee():
        for line in proc.stdout:
            logf_stream.write(line); logf_stream.flush()
            logf_shared.write(line); logf_shared.flush()
    threading.Thread(target=_tee, daemon=True).start()
    return {"ok": True, "ts": ts, "msg": "已发起，实时进度见下方。", "cmd": " ".join(cmd[2:])}


def _plan_task(task: str) -> dict:
    """Heuristic intent parsing: infer task type and suggest a plan."""
    task_l = task.lower()
    # Infer task type
    if any(w in task_l for w in ["test", "测试", "spec", "验证"]):
        task_type = "test"
    elif any(w in task_l for w in ["fix", "bug", "修复", "错误", "报错"]):
        task_type = "fix"
    elif any(w in task_l for w in ["refactor", "重构", "优化", "clean"]):
        task_type = "refactor"
    elif any(w in task_l for w in ["doc", "文档", "readme", "注释"]):
        task_type = "doc"
    else:
        task_type = "feature"

    presets = [
        {"name": "快速实现", "desc": "deepseek 实现，minimax 验证", "stages": "plan,implement,verify",
         "carriers": ["implement=deepseek"], "est_cost": "$0.001", "recommended": True},
        {"name": "高质量", "desc": "codex-sub 控制面，独立审查", "stages": "plan,implement,verify,review",
         "carriers": ["implement=loom-dev", "review=loom-reviewer"], "est_cost": "$0.010", "recommended": False},
        {"name": "省钱", "desc": "minimax 全流程", "stages": "plan,implement,verify",
         "carriers": ["implement=minimax", "verify=minimax"], "est_cost": "$0.000", "recommended": False},
        {"name": "安全优先", "desc": "deepseek + safety 扫描 + golden 验证", "stages": "plan,implement,verify",
         "carriers": ["implement=deepseek"], "flags": ["--safety"], "est_cost": "$0.002", "recommended": False},
    ]

    return {"task_type": task_type, "presets": presets}


# --------------------------------------------------------------------------- #
class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):  # 静音
        pass

    def _json(self, obj, code=200):
        b = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, X-Console-Token")
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, X-Console-Token")
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _authed(self) -> bool:
        if not CONSOLE_TOKEN:
            return True
        from urllib.parse import urlparse, parse_qs
        if parse_qs(urlparse(self.path).query).get("token", [""])[0] == CONSOLE_TOKEN:
            return True
        if f"loom_token={CONSOLE_TOKEN}" in self.headers.get("Cookie", ""):
            return True
        return self.headers.get("X-Console-Token") == CONSOLE_TOKEN

    def do_GET(self):
        if not self._authed():
            self._json({"error": "unauthorized",
                        "hint": "用 http://localhost:8899/?token=<CONSOLE_TOKEN> 打开一次"}, 401)
            return
        p = self.path.split("?")[0]
        if p in ("/", "/index.html"):
            b = HTML.read_text().encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            if CONSOLE_TOKEN:  # 写 cookie，后续同源请求自动带上
                self.send_header("Set-Cookie", f"loom_token={CONSOLE_TOKEN}; Path=/; SameSite=Strict")
            self.send_header("Content-Length", str(len(b)))
            self.end_headers()
            self.wfile.write(b)
        elif p == "/api/health":
            self._json(health())
        elif p == "/api/roles":
            self._json(roles())
        elif p == "/api/roles/full":
            self._json(roles_full())
        elif p == "/api/runs":
            self._json(list_runs())
        elif p.startswith("/api/run/stream/"):
            ts = p[len("/api/run/stream/"):]
            self._sse_run(ts)
        elif p.startswith("/api/run/"):
            self._json(run_detail(p[len("/api/run/"):]))
        elif p == "/api/usage":
            self._json(usage())
        elif p == "/api/carriers":
            self._json(carriers())
        elif p == "/api/executors":
            self._json(executors_avail())
        elif p == "/api/help":
            lang = self.path.split("lang=")[-1] if "lang=" in self.path else "zh"
            self._json(help_doc(lang))
        elif p == "/api/doc":
            from urllib.parse import urlparse, parse_qs
            name = parse_qs(urlparse(self.path).query).get("name", [""])[0]
            self._json(doc_file(name))
        elif p == "/api/diff":
            from urllib.parse import urlparse, parse_qs
            q = parse_qs(urlparse(self.path).query)
            self._json(diff_runs(q.get("ts", [""])[0], q.get("against", [""])[0]))
        elif p == "/api/progress":
            self._json(progress_state())
        elif p == "/api/liveness":
            self._json(liveness_panel())
        elif p == "/api/quota":
            self._json(quota_panel())
        elif p == "/api/scores":
            self._json(scores_panel())
        elif p == "/api/fitness":
            from urllib.parse import urlparse, parse_qs
            task_type = parse_qs(urlparse(self.path).query).get("task_type", [None])[0]
            from devkit.insight import model_fitness
            self._json(model_fitness(task_type=task_type))
        elif p == "/api/assets":
            from devkit.asset import load_assets
            self._json({"assets": load_assets()})
        elif p == "/api/tasks":
            from devkit import task_center as TC
            self._json({"tasks": TC.list_tasks()})
        elif p == "/api/learn":
            from devkit import learn as L
            self._json(L.analyze())
        elif p == "/api/backlog-health":
            self._json(backlog_health())
        elif p == "/api/agent-observability":
            self._json(agent_observability())
        elif p.startswith("/api/artifact-chain/"):
            run_id = p.split("/api/artifact-chain/", 1)[-1].strip("/")
            self._json(artifact_chain(run_id))
        else:
            self._json({"error": "not found"}, 404)

    def do_POST(self):
        if not self._authed():
            self._json({"error": "unauthorized"}, 401)
            return
        if self.path == "/api/run":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(trigger_run(body.get("task", ""), body.get("stages", ""),
                                   body.get("carriers"), body.get("executor", ""),
                                   body.get("budget", ""), body.get("iterate", ""),
                                   body.get("contract", "")))
        elif self.path == "/api/apply":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(apply_run(body.get("ts", "")))
        elif self.path == "/api/remap":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(remap_carrier(body.get("carrier", ""), body.get("target", "")))
        elif self.path == "/api/ask":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(ask_model(body.get("model", ""), body.get("prompt", "")))
        elif self.path == "/api/rate":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(rate_backend(body.get("backend", ""), bool(body.get("up"))))
        elif self.path == "/api/roles/save":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(save_roles(body.get("stages", [])))
        elif self.path == "/api/roles/reset":
            self._json(reset_roles())
        elif self.path == "/api/plan":
            n = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(n) or b"{}")
            self._json(_plan_task(body.get("task", "")))
        else:
            self._json({"error": "not found"}, 404)

    def _sse_run(self, ts: str):
        import time
        if ".." in ts or "/" in ts:
            self._json({"error": "bad ts"}, 400)
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream; charset=utf-8")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()

        run_dir = RUNS_DIR / ts
        stream_log = run_dir / "stream.log"
        deadline = time.time() + 300
        sent_lines = 0

        def _emit(obj):
            data = json.dumps(obj, ensure_ascii=False)
            self.wfile.write(f"data: {data}\n\n".encode())
            self.wfile.flush()

        try:
            while time.time() < deadline:
                # Stream new log lines
                if stream_log.exists():
                    lines = stream_log.read_text(errors="replace").splitlines()
                    for line in lines[sent_lines:]:
                        _emit({"type": "log", "line": line})
                    sent_lines = len(lines)

                # Check done
                run_log = run_dir / "run-log.md"
                if run_log.exists() and sent_lines > 0:
                    # Give one more pass to flush remaining lines
                    if stream_log.exists():
                        lines = stream_log.read_text(errors="replace").splitlines()
                        for line in lines[sent_lines:]:
                            _emit({"type": "log", "line": line})

                    # Parse gate from run-log.md
                    txt = run_log.read_text()
                    mg = re.search(r"## Gate 建议\s*\n+([^\n]+)", txt)
                    gate = mg.group(1).strip() if mg else "?"
                    mu = re.search(r"用量合计：\*\*(\d+)\s*tokens\s*·\s*\$([\d.]+)\*\*", txt)
                    cost = float(mu.group(2)) if mu else 0.0
                    _emit({"type": "done", "gate": gate, "cost": cost, "ts": ts})
                    return

                time.sleep(0.4)

            _emit({"type": "error", "msg": "timeout (5 min)"})
        except (BrokenPipeError, ConnectionResetError):
            pass  # client disconnected


def main():
    host = os.environ.get("CONSOLE_HOST", "0.0.0.0")  # 容器内必须 0.0.0.0 才能被映射端口访问
    print(f"Loom 全局控制台  →  http://localhost:{PORT}  (bind {host}:{PORT})")
    ThreadingHTTPServer((host, PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
