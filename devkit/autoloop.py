# autoloop.py
"""Loom 自治驱动循环 —— 纯逻辑核心（无 IO、无 LLM 调用）。"""

from datetime import datetime
from pathlib import Path
import re

from devkit.model_aliases import normalize_model_name

_ROOT = Path(__file__).resolve().parent.parent
_PATH_RE = re.compile(r"\b((?:devkit|tests)/[\w./-]+\.[A-Za-z]+)\b")
_MODIFY_VERBS = ("修复", "修改", "恢复", "fix", "patch", "edit", "update")
_CREATE_VERBS = ("新增", "创建", "add", "create", "generate")
_STALE_PATH_PREFIXES = ("devkit/runner/sandbox/",)
_STALE_PATH_EXACT = {
    "tests/test_module_shadowing.py",
    "devkit/shadow_safe.py",
}
_STALE_LITERAL_PATHS = {
    "devkit/runner/sandbox/module_shadowing_check.py",
    "devkit/runner/sandbox/pre_build_hooks.py",
    "devkit/runner/sandbox/__init__.py",
    "devkit/runner/sandbox/materialize.py",
    "devkit/runner/sandbox/build.py",
    "tests/test_module_shadowing.py",
    "devkit/shadow_safe.py",
}
_STAGE_ALIASES = {"shell": "implement"}
_META_TASK_ID_PREFIXES = (
    "human-mark-blocked",
    "mark-stuck",
    "freeze-verify-applylock",
    "verify-carrier-applylock-audit",
)
_META_TASK_TEXT_SIGNALS = (
    "backlog.json",
    "decision_log.jsonl",
    "blocked-needs-human",
    "审计报告",
    "冻结并标记",
    "mark blocked",
    "mark_blocked",
    "stuck-loop",
)


def _requires_existing_paths(task_text: str) -> bool:
    text = task_text or ""
    lowered = text.lower()
    if any(token in text for token in _CREATE_VERBS if not token.isascii()):
        return False
    if any(token in lowered for token in _CREATE_VERBS if token.isascii()):
        return False
    return (
        any(token in text for token in _MODIFY_VERBS if not token.isascii())
        or any(token in lowered for token in _MODIFY_VERBS if token.isascii())
    )


def _missing_workspace_paths(task_text: str) -> list[str]:
    if not _requires_existing_paths(task_text):
        return []
    missing: list[str] = []
    for rel in _PATH_RE.findall(task_text or ""):
        if rel.startswith("devkit/runs/") or rel.startswith("runs/archive/"):
            continue
        if not (_ROOT / rel).exists():
            missing.append(rel)
    return missing


def stale_task_missing_paths(task_text: str) -> list[str]:
    """只返回明确属于旧 shadow 链路的失效路径。"""
    text = task_text or ""
    hits: list[str] = []
    for rel in _missing_workspace_paths(task_text):
        if rel in _STALE_PATH_EXACT or any(rel.startswith(prefix) for prefix in _STALE_PATH_PREFIXES):
            hits.append(rel)
    for rel in sorted(_STALE_LITERAL_PATHS):
        if rel in text and not (_ROOT / rel).exists() and rel not in hits:
            hits.append(rel)
    return hits


def prune_stale_pending(backlog: list[dict]) -> dict:
    """将引用旧 shadow 路径的 pending 任务标为 stopped。"""
    updated: list[dict] = []
    stopped: list[dict] = []
    for item in backlog or []:
        if not isinstance(item, dict):
            continue
        row = dict(item)
        if str(row.get("status", "")).lower() == "pending":
            missing = stale_task_missing_paths(str(row.get("task", "")))
            if missing:
                row["status"] = "stopped"
                row["stop_reason"] = "stale_missing_workspace_paths"
                row["stale_paths"] = missing
                stopped.append({"id": row.get("id"), "missing_paths": missing})
        updated.append(row)
    return {"backlog": updated, "stopped": stopped}


def prune_human_only_pending(backlog: list[dict]) -> dict:
    """将 report-only 自治无法真正落地的任务标为 stopped。"""
    from devkit import human_required_guard as _guard

    updated: list[dict] = []
    stopped: list[dict] = []
    for item in backlog or []:
        if not isinstance(item, dict):
            continue
        row = dict(item)
        if str(row.get("status", "")).lower() == "pending":
            reason = _guard.human_required_reason(row)
            if reason:
                row["status"] = "stopped"
                row["stop_reason"] = "human_required_report_only"
                row["human_required_reason"] = reason
                stopped.append({"id": row.get("id"), "reason": reason})
        updated.append(row)
    return {"backlog": updated, "stopped": stopped}


def _normalize_stage_key(key: str) -> str:
    return _STAGE_ALIASES.get((key or "").strip(), (key or "").strip())


def _task_priority(item: dict) -> tuple[int, str]:
    task_id = str(item.get("id", "")).strip()
    task_text = str(item.get("task", ""))
    lowered = task_text.lower()
    is_meta = (
        any(task_id.startswith(prefix) for prefix in _META_TASK_ID_PREFIXES)
        or any(sig in task_text for sig in _META_TASK_TEXT_SIGNALS if not sig.isascii())
        or any(sig in lowered for sig in _META_TASK_TEXT_SIGNALS if sig.isascii())
    )
    return (1 if is_meta else 0, task_id or task_text)

# ---------- M1 ----------
def pick_next(backlog: list[dict]) -> dict | None:
    """从 backlog 选取最高优先级的 ready 任务；低价值维护任务靠后。"""
    done_ids = {item["id"] for item in backlog if item.get("status") == "done"}
    ready: list[dict] = []
    for item in backlog:
        if item.get("status") != "pending":
            continue
        if _missing_workspace_paths(str(item.get("task", ""))):
            continue
        deps = item.get("deps", [])
        if all(dep in done_ids for dep in deps):
            ready.append(item)
    if not ready:
        return None
    return min(ready, key=_task_priority)

# ---------- M2 ----------
def advance_state(state: str, event: str) -> str:
    """按状态机规则推进任务状态；终止态与非法事件保持不变。"""
    if state in ("done", "failed", "stopped"):
        return state
    transitions = {
        ("pending", "start"): "running",
        ("running", "success"): "done",
        ("running", "failure"): "failed",
        ("running", "stop"): "stopped",
    }
    return transitions.get((state, event), state)

# ---------- M3 ----------
def run_once(task_spec: dict) -> dict:
    """将任务 spec 结构化为一次 devkit 运行的参数字典。

    Phase B: when ``task_spec`` carries a GoalSpec-shaped payload
    (``kind == "GoalSpec"`` with ``metadata`` / ``spec`` keys), validate it
    against ``devkit/protocol_schemas/goal_spec.schema.json`` via
    ``devkit.rdloop.validate_goal_spec`` so malformed input fails loud before
    any IO or LLM call. Non-GoalSpec dicts (the common case) skip validation.
    """
    if (
        isinstance(task_spec, dict)
        and task_spec.get("kind") == "GoalSpec"
        and "metadata" in task_spec
        and "spec" in task_spec
    ):
        from devkit.rdloop import validate_goal_spec
        validate_goal_spec(task_spec)

    # Phase B legacy flat-shape: `carrier` may be a single string
    # ("deepseek") instead of a stage→carrier dict. Default to applying
    # the string to all four standard stages; carrier=None or dict
    # also handled.
    _raw_carrier = task_spec.get("carrier")
    if isinstance(_raw_carrier, str):
        carrier_map = {
            "plan": _raw_carrier,
            "implement": _raw_carrier,
            "verify": _raw_carrier,
            "review": _raw_carrier,
        }
    elif isinstance(_raw_carrier, dict):
        carrier_map = _raw_carrier
    else:
        carrier_map = {}
    carriers = [
        f"{_normalize_stage_key(k)}={normalize_model_name(v, stage=_normalize_stage_key(k))}"
        for k, v in carrier_map.items()
    ]
    executor_map = task_spec.get("executor") or {}
    executors = [f"{_normalize_stage_key(k)}={v}" for k, v in executor_map.items()]
    raw_stages = [s.strip() for s in str(task_spec.get("stages", "plan,implement,verify")).split(",") if s.strip()]
    mapped_stages: list[str] = []
    for stage in raw_stages:
        normalized = _normalize_stage_key(stage)
        if normalized not in mapped_stages:
            mapped_stages.append(normalized)
    out = {
        "task": task_spec["task"],
        "stages": ",".join(mapped_stages) or "plan,implement,verify",
        "carriers": carriers,
        "executors": executors,
        "run_id": task_spec.get(
            "run_id", "auto-" + datetime.now().strftime("%Y%m%d-%H%M%S")
        ),
    }
    for key in (
        "iterate",
        "contract",
        "contract_rounds",
        "budget",
        "blind_review",
        "physical_verify",
        "cascade",
        "delivery_mode",
        "task_kind",
        "apply_target",
        "apply_git",
        "apply_branch",
        "allowed_artifact_paths",
        "forbidden_artifact_paths",
    ):
        if key in task_spec:
            out[key] = task_spec[key]
    return out


def is_success_gate(gate: str | None) -> bool:
    """判断 run_loop 的 gate 文本是否属于可接受成功态。"""
    text = (gate or "").strip()
    return text == "GO" or text.startswith("建议 GO")
