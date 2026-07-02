# decision_log.py
"""自治决策日志：每次 devkit auto 选取任务时记录决策过程，供事后追溯。

格式：JSONL（每行一条 JSON），追加写入 devkit/decisions.jsonl。
可用 `devkit decisions` 命令查看，也可直接 `tail -f decisions.jsonl`。
"""
from __future__ import annotations

import json
import os
import pathlib
from datetime import datetime, timezone

_LOG_PATH = pathlib.Path(__file__).parent / "decisions.jsonl"
_BACKLOG_PATH = pathlib.Path(__file__).parent / "backlog.json"


def append(
    *,
    task_id: str,
    task_text: str,
    run_id: str,
    score: int,
    reason: str,
    alternatives: list,
    outcome: str = "pending",
    log_path: pathlib.Path = _LOG_PATH,
    sync_backlog: bool = False,
    backlog_path: pathlib.Path = _BACKLOG_PATH,
    priority: str | None = None,
) -> dict:
    """追加一条决策记录，返回写入的 record。"""
    record = {
        "ts": datetime.now(tz=timezone.utc).isoformat(),
        "task_id": task_id,
        "run_id": run_id,
        "score": score,
        "reason": reason,
        "selection_reason": reason,
        "root_cause": "",
        "next_action": "",
        "failure_code": "",
        "alternatives": alternatives,
        "outcome": outcome,
        "task_text": task_text,
    }
    with log_path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(record, ensure_ascii=False) + "\n")
    if sync_backlog:
        sync_backlog_task(
            task_id=task_id,
            status="pending",
            task_text=task_text,
            priority=priority,
            backlog_path=backlog_path,
        )
    return record


def update_outcome(
    run_id: str,
    outcome: str,
    log_path: pathlib.Path = _LOG_PATH,
    *,
    reason: str | None = None,
    root_cause: str | None = None,
    next_action: str | None = None,
    failure_code: str | None = None,
    sync_backlog: bool = False,
    backlog_path: pathlib.Path = _BACKLOG_PATH,
) -> bool:
    """把匹配 run_id 且 outcome=pending 的记录更新为最终状态，就地重写该行。"""
    if not log_path.exists():
        return False
    lines = log_path.read_text(encoding="utf-8").splitlines()
    updated = False
    updated_task_id = ""
    updated_task_text = ""
    new_lines: list[str] = []
    for line in lines:
        stripped = line.strip()
        if not stripped:
            new_lines.append(line)
            continue
        try:
            rec = json.loads(stripped)
            should_enrich = any(v is not None for v in (reason, root_cause, next_action, failure_code))
            if rec.get("run_id") == run_id and (rec.get("outcome") == "pending" or should_enrich):
                rec["outcome"] = outcome
                if reason is not None:
                    rec["reason"] = reason
                if root_cause is not None:
                    rec["root_cause"] = root_cause
                if next_action is not None:
                    rec["next_action"] = next_action
                if failure_code is not None:
                    rec["failure_code"] = failure_code
                updated_task_id = str(rec.get("task_id", "")).strip()
                updated_task_text = str(rec.get("task_text", "")).strip()
                new_lines.append(json.dumps(rec, ensure_ascii=False))
                updated = True
                continue
        except (json.JSONDecodeError, KeyError):
            pass
        new_lines.append(line)
    if updated:
        _atomic_write_text(log_path, "\n".join(new_lines) + "\n")
        if sync_backlog and updated_task_id:
            sync_backlog_task(
                task_id=updated_task_id,
                status=_outcome_to_backlog_status(outcome),
                task_text=updated_task_text,
                backlog_path=backlog_path,
            )
    return updated


def sync_backlog_task(
    *,
    task_id: str,
    status: str,
    task_text: str = "",
    priority: str | None = None,
    backlog_path: pathlib.Path = _BACKLOG_PATH,
) -> bool:
    """同步 backlog 中对应任务的状态；缺失时插入最小任务骨架。"""
    task_id = str(task_id).strip()
    if not task_id:
        return False
    backlog_path = pathlib.Path(backlog_path)
    items, wrapped = _load_backlog(backlog_path)
    if items is None:
        return False

    updated = False
    normalized_status = str(status).strip().lower() or "pending"
    normalized_priority = str(priority or "").strip().lower()
    if normalized_priority not in {"high", "medium", "low"}:
        normalized_priority = "medium"

    for item in items:
        if str(item.get("id", "")).strip() != task_id:
            continue
        item["status"] = normalized_status
        if task_text and not str(item.get("task", "")).strip():
            item["task"] = task_text
        if normalized_priority and not str(item.get("priority", "")).strip():
            item["priority"] = normalized_priority
        item.setdefault("deps", [])
        updated = True
        break

    if not updated:
        items.append(
            {
                "id": task_id,
                "task": task_text or task_id,
                "status": normalized_status,
                "deps": [],
                "priority": normalized_priority,
            }
        )
        updated = True

    payload = {"tasks": items} if wrapped else items
    _atomic_write_text(backlog_path, json.dumps(payload, ensure_ascii=False, indent=2) + "\n")
    return updated


def to_history_rows(records: list) -> list:
    """将决策日志记录转成 carrier_router 可消费的 history_rows 格式。

    每条决策对应一个 carrier+stage 的运行结果，outcome=success → ok_rate=1.0，其余=0.0。
    返回 [{carrier, stage, ok_rate, avg_cost, runs}, ...]
    """
    rows = []
    for rec in records:
        outcome = rec.get("outcome", "pending")
        if outcome == "pending":
            continue
        task_id = rec.get("task_id", "")
        run_id = rec.get("run_id", "")
        ok_rate = 1.0 if outcome == "success" else 0.0
        # stage 从 run_id 推断不可靠，用 "implement" 作默认（auto 任务以实现为主）
        rows.append({
            "carrier": task_id,   # task_id 不是 carrier，用于溯源；实际 carrier 在 alternatives
            "stage": "implement",
            "ok_rate": ok_rate,
            "avg_cost": 0.0,
            "runs": 1,
            "run_id": run_id,
        })
        # 同时把备选 carriers 的信息回流（失败的 task 对应的备选可能更好）
        for alt in rec.get("alternatives", []):
            alt_carrier = alt.get("task_id", "")  # alternatives 里也是 task_id，非 carrier
            if alt_carrier:
                rows.append({
                    "carrier": alt_carrier,
                    "stage": "implement",
                    "ok_rate": 0.5,  # 备选未实际运行，给中性分
                    "avg_cost": 0.0,
                    "runs": 0,
                    "run_id": run_id,
                })
    return rows


def load(log_path: pathlib.Path = _LOG_PATH, last_n: int = 0) -> list[dict]:
    """读取决策日志，last_n=0 表示全量，否则返回最后 n 条。"""
    if not log_path.exists():
        return []
    records: list[dict] = []
    for line in log_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            pass
    return records[-last_n:] if last_n else records


def reconcile_pending_with_backlog(
    log_path: pathlib.Path = _LOG_PATH,
    backlog_path: pathlib.Path = _BACKLOG_PATH,
) -> int:
    """用 backlog 当前终态对账 decision log 中悬空的 pending 记录。"""
    items, _wrapped = _load_backlog(pathlib.Path(backlog_path))
    if items is None or not pathlib.Path(log_path).exists():
        return 0
    status_by_id = {
        str(item.get("id", "")).strip(): str(item.get("status", "")).strip().lower()
        for item in items
        if str(item.get("id", "")).strip()
    }
    lines = pathlib.Path(log_path).read_text(encoding="utf-8").splitlines()
    changed = 0
    new_lines: list[str] = []
    for line in lines:
        stripped = line.strip()
        if not stripped:
            new_lines.append(line)
            continue
        try:
            rec = json.loads(stripped)
        except json.JSONDecodeError:
            new_lines.append(line)
            continue
        if rec.get("outcome") == "pending":
            backlog_status = status_by_id.get(str(rec.get("task_id", "")).strip(), "")
            outcome = _backlog_status_to_outcome(backlog_status)
            if outcome:
                rec["outcome"] = outcome
                changed += 1
        new_lines.append(json.dumps(rec, ensure_ascii=False))
    if changed:
        _atomic_write_text(pathlib.Path(log_path), "\n".join(new_lines) + "\n")
    return changed


def _load_backlog(backlog_path: pathlib.Path) -> tuple[list[dict] | None, bool]:
    if not backlog_path.exists():
        return None, False
    try:
        data = json.loads(backlog_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None, False
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        return [dict(item) for item in data["tasks"] if isinstance(item, dict)], True
    if isinstance(data, list):
        return [dict(item) for item in data if isinstance(item, dict)], False
    return None, False


def _outcome_to_backlog_status(outcome: str) -> str:
    mapping = {
        "pending": "pending",
        "success": "done",
        "failure": "failed",
        "failed": "failed",
        "cancelled": "stopped",
        "canceled": "stopped",
        "skipped": "skipped",
        "stopped": "stopped",
    }
    return mapping.get(str(outcome).strip().lower(), "failed")


def _backlog_status_to_outcome(status: str) -> str:
    mapping = {
        "done": "success",
        "failed": "failure",
        "skipped": "skipped",
        "stopped": "cancelled",
    }
    return mapping.get(str(status).strip().lower(), "")


def _atomic_write_text(path: pathlib.Path, text: str) -> None:
    path = pathlib.Path(path)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fp:
        fp.write(text)
        fp.flush()
        os.fsync(fp.fileno())
    os.replace(tmp, path)
