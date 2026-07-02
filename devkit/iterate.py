"""Project-level iteration helpers for autonomous run -> reflect -> continue loops."""

from __future__ import annotations

import json
import pathlib
import re
from datetime import datetime

from devkit import autoloop

VALID_PRIORITIES = {"high", "medium", "low"}
VALID_STATUSES = {"pending", "done", "failed", "running", "skipped", "stopped"}
_FAILURE_CODE_RE = re.compile(r"\b([A-Z][A-Z0-9_]{5,})\b")


def backlog_stats(backlog: list[dict]) -> dict:
    total = len(backlog)
    done = sum(1 for item in backlog if item.get("status") == "done")
    pending = sum(1 for item in backlog if item.get("status") == "pending")
    failed = sum(1 for item in backlog if item.get("status") == "failed")
    running = sum(1 for item in backlog if item.get("status") == "running")
    ready = 1 if autoloop.pick_next(backlog) else 0
    return {
        "total": total,
        "done": done,
        "pending": pending,
        "failed": failed,
        "running": running,
        "ready": ready,
    }


def build_reflection_prompt(
    *,
    round_no: int,
    task_id: str,
    outcome: str,
    gate: str,
    backlog: list[dict],
    run_log: str,
    recent_decisions: list[dict],
) -> str:
    stats = backlog_stats(backlog)
    decisions = json.dumps(recent_decisions[-3:], ensure_ascii=False, indent=2)
    backlog_text = json.dumps(backlog, ensure_ascii=False, indent=2)
    return (
        f"你是 Loom 的自治迭代反思代理。当前是第 {round_no} 轮，刚执行完任务 `{task_id}`。\n"
        f"本轮结果：outcome={outcome}；gate={gate}\n\n"
        "目标：基于本轮 run 证据，判断是否继续下一轮，并在必要时写回 backlog。\n"
        "要求：\n"
        "1. 保守、具体、可执行。\n"
        "2. 允许动作只有：requeue 失败任务、reprioritize 已有任务、add_tasks 新任务、stop。\n"
        "3. 新任务必须可测试、可独立执行，不能是抽象口号。\n"
        "4. 如果没有新增信息，不要胡乱生成任务。\n"
        "5. 只输出 JSON，不要解释，不要 markdown。\n\n"
        "输出 JSON schema：\n"
        "{\n"
        '  "summary": "string",\n'
        '  "continue": true,\n'
        '  "stop_reason": "string",\n'
        '  "requeue": ["task-id"],\n'
        '  "reprioritize": [{"id": "task-id", "priority": "high|medium|low"}],\n'
        '  "add_tasks": [\n'
        "    {\n"
        '      "id": "short-id",\n'
        '      "task": "明确可测试的任务描述",\n'
        '      "priority": "high|medium|low",\n'
        '      "deps": ["optional-task-id"],\n'
        '      "stages": "optional stages string",\n'
        '      "carrier": {"implement": "optional-model"}\n'
        "    }\n"
        "  ]\n"
        "}\n\n"
        f"最近决策记录：\n{decisions}\n\n"
        f"当前 backlog：\n{backlog_text[:12000]}\n\n"
        f"本轮 run-log：\n{run_log[:12000]}\n"
    )


def parse_reflection(text: str) -> dict:
    cleaned = re.sub(r"```(?:json)?", "", text or "", flags=re.IGNORECASE).strip()
    match = re.search(r"\{.*\}", cleaned, re.S)
    if not match:
        return {
            "summary": "",
            "continue": True,
            "stop_reason": "",
            "requeue": [],
            "reprioritize": [],
            "add_tasks": [],
            "_parse_error": "missing_json_object",
            "_raw": text or "",
        }
    try:
        raw = json.loads(match.group(0))
    except json.JSONDecodeError:
        return {
            "summary": "",
            "continue": True,
            "stop_reason": "",
            "requeue": [],
            "reprioritize": [],
            "add_tasks": [],
            "_parse_error": "invalid_json",
            "_raw": text or "",
        }

    return {
        "summary": str(raw.get("summary", "")).strip(),
        "continue": bool(raw.get("continue", True)),
        "stop_reason": str(raw.get("stop_reason", "")).strip(),
        "requeue": _normalize_id_list(raw.get("requeue", [])),
        "reprioritize": _normalize_reprioritize(raw.get("reprioritize", [])),
        "add_tasks": _normalize_tasks(raw.get("add_tasks", [])),
        "_raw": text or "",
    }


def infer_failure_code(*texts: str) -> str:
    for text in texts:
        body = str(text or "")
        for match in _FAILURE_CODE_RE.finditer(body):
            code = match.group(1)
            if code not in {"REQUEST", "CHANGES", "STALLED_NO_READY_TASK"}:
                return code
    return ""


def _count_recent_same_failure(records: list[dict], task_id: str, failure_code: str) -> int:
    if not task_id or not failure_code:
        return 0
    count = 0
    for rec in reversed(records):
        if str(rec.get("task_id", "")).strip() != task_id:
            continue
        if str(rec.get("outcome", "")).strip() != "failure":
            if count:
                break
            continue
        if str(rec.get("failure_code", "")).strip() == failure_code:
            count += 1
            continue
        if count:
            break
    return count


def next_action_text(reflection: dict) -> str:
    add_tasks = reflection.get("add_tasks", []) if isinstance(reflection.get("add_tasks", []), list) else []
    if add_tasks:
        first = add_tasks[0]
        return f"add:{first.get('id', '')}".strip(":")
    requeue = reflection.get("requeue", []) if isinstance(reflection.get("requeue", []), list) else []
    if requeue:
        return f"requeue:{requeue[0]}"
    reprioritize = reflection.get("reprioritize", []) if isinstance(reflection.get("reprioritize", []), list) else []
    if reprioritize:
        return f"reprioritize:{reprioritize[0].get('id', '')}".strip(":")
    if not reflection.get("continue", True):
        return "stop"
    return ""


def apply_reflection(
    backlog: list[dict],
    reflection: dict,
    *,
    current_task_id: str = "",
    current_outcome: str = "",
    current_failure_code: str = "",
    recent_records: list[dict] | None = None,
) -> dict:
    items = [dict(item) for item in backlog]
    reflection_out = dict(reflection or {})
    by_id = {item.get("id"): item for item in items if item.get("id")}
    changed = {"requeued": 0, "reprioritized": 0, "added": 0, "added_ids": [], "blocked_requeue": 0}

    blocked_requeue_ids: set[str] = set()
    if current_outcome == "failure" and current_task_id and current_failure_code:
        same_count = _count_recent_same_failure(recent_records or [], current_task_id, current_failure_code)
        if same_count >= 2 and current_task_id in set(reflection_out.get("requeue", []) or []):
            blocked_requeue_ids.add(current_task_id)
            reflection_out["requeue"] = [tid for tid in reflection_out.get("requeue", []) if tid != current_task_id]
            reflection_out["continue"] = False
            msg = (f"blocked requeue for {current_task_id}: {current_failure_code} repeated "
                   f"{same_count} consecutive runs with no new signal")
            old_stop = str(reflection_out.get("stop_reason", "")).strip()
            reflection_out["stop_reason"] = f"{old_stop}; {msg}".strip("; ")
            changed["blocked_requeue"] = 1

    for task_id in reflection_out.get("requeue", []):
        item = by_id.get(task_id)
        if not item:
            continue
        if item.get("status") in {"failed", "stopped", "skipped"}:
            item["status"] = "pending"
            changed["requeued"] += 1

    for entry in reflection_out.get("reprioritize", []):
        item = by_id.get(entry.get("id"))
        priority = entry.get("priority")
        if not item or priority not in VALID_PRIORITIES:
            continue
        item["priority"] = priority
        changed["reprioritized"] += 1

    existing_ids = {item.get("id") for item in items if item.get("id")}
    for task in reflection_out.get("add_tasks", []):
        new_task = dict(task)
        new_id = _unique_id(new_task["id"], existing_ids)
        new_task["id"] = new_id
        items.append(new_task)
        existing_ids.add(new_id)
        changed["added"] += 1
        changed["added_ids"].append(new_id)

    return {"backlog": items, "changes": changed, "reflection": reflection_out}


def reflection_markdown(
    *,
    round_no: int,
    task_id: str,
    run_id: str,
    outcome: str,
    gate: str,
    reflection: dict,
    changes: dict,
) -> str:
    summary = reflection.get("summary", "").strip() or "（无摘要）"
    stop_reason = reflection.get("stop_reason", "").strip()
    return (
        f"# Iteration Reflection {round_no}\n\n"
        f"- 时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n"
        f"- run_id: {run_id}\n"
        f"- task_id: {task_id}\n"
        f"- outcome: {outcome}\n"
        f"- gate: {gate}\n"
        f"- continue: {bool(reflection.get('continue', True))}\n"
        + (f"- stop_reason: {stop_reason}\n" if stop_reason else "")
        + "\n## Summary\n\n"
        + summary
        + "\n\n## Applied Changes\n\n"
        + json.dumps(changes, ensure_ascii=False, indent=2)
        + "\n\n## Raw Reflection\n\n```json\n"
        + json.dumps(
            {
                "summary": reflection.get("summary", ""),
                "continue": reflection.get("continue", True),
                "stop_reason": reflection.get("stop_reason", ""),
                "requeue": reflection.get("requeue", []),
                "reprioritize": reflection.get("reprioritize", []),
                "add_tasks": reflection.get("add_tasks", []),
                "_parse_error": reflection.get("_parse_error", ""),
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n```\n"
    )


def _normalize_id_list(values) -> list[str]:
    result = []
    for value in values if isinstance(values, list) else []:
        text = str(value).strip()
        if text:
            result.append(text)
    return result


def _normalize_reprioritize(values) -> list[dict]:
    result = []
    for value in values if isinstance(values, list) else []:
        if not isinstance(value, dict):
            continue
        task_id = str(value.get("id", "")).strip()
        priority = str(value.get("priority", "")).strip().lower()
        if task_id and priority in VALID_PRIORITIES:
            result.append({"id": task_id, "priority": priority})
    return result


def _normalize_tasks(values) -> list[dict]:
    result = []
    for value in values if isinstance(values, list) else []:
        task = _normalize_task(value)
        if task is not None:
            result.append(task)
    return result


def _normalize_task(value) -> dict | None:
    if not isinstance(value, dict):
        return None
    task_text = str(value.get("task", "")).strip()
    if not task_text:
        return None
    task_id = _slug(str(value.get("id", "")).strip() or task_text[:32])
    deps = [str(dep).strip() for dep in value.get("deps", []) if str(dep).strip()] if isinstance(value.get("deps", []), list) else []
    priority = str(value.get("priority", "medium")).strip().lower()
    if priority not in VALID_PRIORITIES:
        priority = "medium"
    task = {
        "id": task_id,
        "task": task_text,
        "status": "pending",
        "deps": deps,
        "priority": priority,
    }
    if isinstance(value.get("stages"), str) and value.get("stages", "").strip():
        task["stages"] = value["stages"].strip()
    if isinstance(value.get("cascade"), str) and value.get("cascade", "").strip():
        task["cascade"] = value["cascade"].strip()
    if isinstance(value.get("carrier"), dict):
        task["carrier"] = {
            str(k).strip(): str(v).strip()
            for k, v in value["carrier"].items()
            if str(k).strip() and str(v).strip()
        }
    if isinstance(value.get("executor"), dict):
        task["executor"] = {
            str(k).strip(): str(v).strip()
            for k, v in value["executor"].items()
            if str(k).strip() and str(v).strip()
        }
    for key in ("iterate", "contract", "contract_rounds"):
        if isinstance(value.get(key), int):
            task[key] = value[key]
    if isinstance(value.get("budget"), (int, float)):
        task["budget"] = float(value["budget"])
    if isinstance(value.get("blind_review"), bool):
        task["blind_review"] = value["blind_review"]
    if isinstance(value.get("physical_verify"), bool):
        task["physical_verify"] = value["physical_verify"]
    return task


def _slug(text: str) -> str:
    slug = re.sub(r"[^\w]+", "-", text.strip().lower()).strip("-")
    return slug[:48] or "iter-task"


def _unique_id(task_id: str, existing_ids: set[str]) -> str:
    candidate = task_id
    index = 2
    while candidate in existing_ids:
        candidate = f"{task_id}-{index}"
        index += 1
    return candidate
