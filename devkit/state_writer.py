"""Deterministic state transition writer for Loom runtime objects."""
from __future__ import annotations

import json
import os
import pathlib
from datetime import datetime, timezone

PROTOCOL_VERSION = "loom.dev/v1"
EVENT_LOG_PATH = pathlib.Path(__file__).resolve().parent / "events.jsonl"
VALID_BACKLOG_STATUSES = frozenset({"pending", "running", "done", "failed", "blocked", "stopped", "skipped"})
FINAL_STATUSES = frozenset({"done", "failed", "stopped", "blocked", "skipped"})
STATUS_ALIASES = {
    "ready": "pending",
}
ALLOWED_TRANSITIONS = {
    "pending": {"running", "stopped", "skipped"},
    "running": {"done", "failed", "stopped", "blocked"},
    "blocked": {"running", "stopped", "skipped"},
    "failed": {"pending", "stopped", "skipped"},
    "stopped": {"pending", "skipped"},
    "skipped": {"pending"},
    "done": {"stopped"},
}


class TransitionError(RuntimeError):
    def __init__(self, message: str, *, failure_code: str) -> None:
        super().__init__(message)
        self.failure_code = failure_code


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def transition_task(
    *,
    backlog_path: pathlib.Path | str,
    task_id: str,
    to_status: str,
    actor: str,
    source_task_id: str,
    reason: str,
    event_path: pathlib.Path | str = EVENT_LOG_PATH,
    writer: str = "state_writer",
    protocol_version: str = PROTOCOL_VERSION,
    extra: dict | None = None,
) -> dict:
    items, wrapped = _load_backlog(backlog_path)
    target = _normalize_status(to_status)
    if not target:
        raise TransitionError("target status is required", failure_code="INVALID_STATUS")

    item = next((entry for entry in items if str(entry.get("id", "")).strip() == str(task_id).strip()), None)
    if item is None:
        _append_event(
            event_path,
            _event_record(
                task_id=task_id,
                actor=actor,
                source_task_id=source_task_id,
                reason=reason,
                protocol_version=protocol_version,
                previous_status=None,
                target_status=target,
                outcome="rejected",
                failure_code="TASK_NOT_FOUND",
                extra=extra,
            ),
        )
        raise TransitionError(f"task {task_id!r} not found", failure_code="TASK_NOT_FOUND")

    current = _normalize_status(item.get("status", "pending")) or "pending"
    if writer != "state_writer" and target in FINAL_STATUSES:
        _append_event(
            event_path,
            _event_record(
                task_id=task_id,
                actor=actor,
                source_task_id=source_task_id,
                reason=reason,
                protocol_version=protocol_version,
                previous_status=current,
                target_status=target,
                outcome="rejected",
                failure_code="DIRECT_FINAL_STATUS_WRITE_BLOCKED",
                extra=extra,
            ),
        )
        raise TransitionError(
            f"writer {writer!r} cannot persist final status {target!r}",
            failure_code="DIRECT_FINAL_STATUS_WRITE_BLOCKED",
        )

    allowed = ALLOWED_TRANSITIONS.get(current, set())
    if target not in allowed:
        _append_event(
            event_path,
            _event_record(
                task_id=task_id,
                actor=actor,
                source_task_id=source_task_id,
                reason=reason,
                protocol_version=protocol_version,
                previous_status=current,
                target_status=target,
                outcome="rejected",
                failure_code="TRANSITION_NOT_ALLOWED",
                extra=extra,
            ),
        )
        raise TransitionError(
            f"transition {current!r} -> {target!r} is not allowed",
            failure_code="TRANSITION_NOT_ALLOWED",
        )

    item["status"] = target
    if extra:
        for key, value in extra.items():
            item[key] = value
    _write_backlog(backlog_path, items, wrapped)
    record = _event_record(
        task_id=task_id,
        actor=actor,
        source_task_id=source_task_id,
        reason=reason,
        protocol_version=protocol_version,
        previous_status=current,
        target_status=target,
        outcome="accepted",
        failure_code="",
        extra=extra,
    )
    _append_event(event_path, record)
    return record


def enqueue_task(
    *,
    backlog_path: pathlib.Path | str,
    item: dict,
    actor: str,
    source_task_id: str,
    reason: str,
    event_path: pathlib.Path | str = EVENT_LOG_PATH,
    protocol_version: str = PROTOCOL_VERSION,
) -> dict:
    from devkit.task_validator import validate_task

    items, wrapped = _load_backlog(backlog_path)
    task_id = str(item.get("id", "")).strip()
    if not task_id:
        raise TransitionError("task id is required", failure_code="TASK_ID_REQUIRED")
    if any(str(entry.get("id", "")).strip() == task_id for entry in items):
        _append_event(
            event_path,
            _event_record(
                task_id=task_id,
                actor=actor,
                source_task_id=source_task_id,
                reason=reason,
                protocol_version=protocol_version,
                previous_status=None,
                target_status=str(item.get("status", "pending")).strip() or "pending",
                outcome="rejected",
                failure_code="TASK_ALREADY_EXISTS",
                extra={"task": dict(item)},
            ),
        )
        raise TransitionError(f"task {task_id!r} already exists", failure_code="TASK_ALREADY_EXISTS")

    status = _normalize_status(item.get("status", "pending")) or "pending"
    if status not in VALID_BACKLOG_STATUSES:
        raise TransitionError(f"invalid task status {status!r}", failure_code="INVALID_STATUS")

    new_item = dict(item)
    new_item["status"] = status
    validation_errors = validate_task(new_item)
    if validation_errors:
        failure_code = (
            "TASK_GROUNDING_ERROR"
            if any("referenced repo path does not exist:" in err for err in validation_errors)
            else "TASK_VALIDATION_ERROR"
        )
        _append_event(
            event_path,
            _event_record(
                task_id=task_id,
                actor=actor,
                source_task_id=source_task_id,
                reason=reason,
                protocol_version=protocol_version,
                previous_status=None,
                target_status=status,
                outcome="rejected",
                failure_code=failure_code,
                extra={"task": new_item, "errors": validation_errors},
            ),
        )
        raise TransitionError(
            f"task {task_id!r} failed validation: {'; '.join(validation_errors)}",
            failure_code=failure_code,
        )
    items.append(new_item)
    _write_backlog(backlog_path, items, wrapped)
    record = {
        "timestamp": now_iso(),
        "task_id": task_id,
        "actor": str(actor).strip() or "unknown",
        "source_task_id": str(source_task_id).strip(),
        "protocol_version": str(protocol_version).strip() or PROTOCOL_VERSION,
        "reason": str(reason).strip(),
        "transition": {"from": None, "to": status},
        "outcome": "accepted",
        "failure_code": "",
        "event_type": "enqueue",
        "extra": {"task": new_item},
    }
    _append_event(event_path, record)
    return record


def sync_task_metadata(
    *,
    backlog_path: pathlib.Path | str,
    task_id: str,
    actor: str,
    source_task_id: str,
    reason: str,
    patch: dict | None = None,
    remove_keys: list[str] | tuple[str, ...] | set[str] | None = None,
    event_path: pathlib.Path | str = EVENT_LOG_PATH,
    protocol_version: str = PROTOCOL_VERSION,
    record_event: bool = True,
) -> dict:
    items, wrapped = _load_backlog(backlog_path)
    item = next((entry for entry in items if str(entry.get("id", "")).strip() == str(task_id).strip()), None)
    if item is None:
        payload = {
            "timestamp": now_iso(),
            "task_id": str(task_id).strip(),
            "actor": str(actor).strip() or "unknown",
            "source_task_id": str(source_task_id).strip(),
            "protocol_version": str(protocol_version).strip() or PROTOCOL_VERSION,
            "reason": str(reason).strip(),
            "transition": {"from": None, "to": None},
            "outcome": "rejected",
            "failure_code": "TASK_NOT_FOUND",
            "event_type": "metadata_sync",
            "extra": {"patch": dict(patch or {}), "remove_keys": sorted(str(k) for k in (remove_keys or []))},
        }
        if record_event:
            _append_event(event_path, payload)
        raise TransitionError(f"task {task_id!r} not found", failure_code="TASK_NOT_FOUND")

    if patch and "status" in patch:
        raise TransitionError("metadata patch cannot include status", failure_code="STATUS_PATCH_FORBIDDEN")
    remove = [str(key).strip() for key in (remove_keys or []) if str(key).strip()]
    if any(key == "status" for key in remove):
        raise TransitionError("metadata sync cannot remove status", failure_code="STATUS_PATCH_FORBIDDEN")

    before = dict(item)
    for key, value in dict(patch or {}).items():
        item[key] = value
    for key in remove:
        item.pop(key, None)
    _write_backlog(backlog_path, items, wrapped)

    changed_set = {
        key: item.get(key)
        for key in dict(patch or {})
        if before.get(key) != item.get(key)
    }
    removed = [key for key in remove if key in before and key not in item]
    record = {
        "timestamp": now_iso(),
        "task_id": str(task_id).strip(),
        "actor": str(actor).strip() or "unknown",
        "source_task_id": str(source_task_id).strip(),
        "protocol_version": str(protocol_version).strip() or PROTOCOL_VERSION,
        "reason": str(reason).strip(),
        "transition": {"from": str(before.get("status", "")).strip() or None, "to": str(item.get("status", "")).strip() or None},
        "outcome": "accepted",
        "failure_code": "",
        "event_type": "metadata_sync",
        "extra": {
            "set": changed_set,
            "removed": removed,
        },
    }
    if record_event:
        _append_event(event_path, record)
    return record


def write_backlog_snapshot(
    *,
    backlog_path: pathlib.Path | str,
    items: list[dict],
    wrapped: bool | None = None,
) -> list[dict]:
    path = pathlib.Path(backlog_path)
    current, current_wrapped = _load_backlog(path)
    use_wrapped = current_wrapped if wrapped is None else bool(wrapped)
    current_by_id = {
        str(item.get("id", "")).strip(): dict(item)
        for item in current
        if isinstance(item, dict) and str(item.get("id", "")).strip()
    }
    merged: list[dict] = []
    seen_ids: set[str] = set()

    for item in items:
        if not isinstance(item, dict):
            continue
        merged_item = dict(item)
        task_id = str(merged_item.get("id", "")).strip()
        if task_id and task_id in current_by_id:
            disk_item = current_by_id[task_id]
            for key, value in disk_item.items():
                merged_item.setdefault(key, value)
            seen_ids.add(task_id)
        merged.append(merged_item)

    for item in current:
        if not isinstance(item, dict):
            continue
        task_id = str(item.get("id", "")).strip()
        if task_id and task_id in seen_ids:
            continue
        merged.append(dict(item))

    _write_backlog(path, merged, use_wrapped)
    return merged


def _event_record(
    *,
    task_id: str,
    actor: str,
    source_task_id: str,
    reason: str,
    protocol_version: str,
    previous_status: str | None,
    target_status: str,
    outcome: str,
    failure_code: str,
    extra: dict | None,
) -> dict:
    return {
        "timestamp": now_iso(),
        "task_id": str(task_id).strip(),
        "actor": str(actor).strip() or "unknown",
        "source_task_id": str(source_task_id).strip(),
        "protocol_version": str(protocol_version).strip() or PROTOCOL_VERSION,
        "reason": str(reason).strip(),
        "transition": {"from": previous_status, "to": target_status},
        "outcome": outcome,
        "failure_code": failure_code,
        "extra": dict(extra or {}),
    }


def _normalize_status(status: str | None) -> str:
    raw = str(status or "").strip().lower()
    return STATUS_ALIASES.get(raw, raw)


def _load_backlog(path: pathlib.Path | str) -> tuple[list[dict], bool]:
    path = pathlib.Path(path)
    if not path.exists():
        return [], False
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        return [dict(item) for item in data["tasks"] if isinstance(item, dict)], True
    if isinstance(data, list):
        return [dict(item) for item in data if isinstance(item, dict)], False
    return [], False


def _write_backlog(path: pathlib.Path | str, items: list[dict], wrapped: bool) -> None:
    payload = {"tasks": items} if wrapped else items
    _atomic_write_text(pathlib.Path(path), json.dumps(payload, ensure_ascii=False, indent=2) + "\n")


def _append_event(path: pathlib.Path | str, record: dict) -> None:
    event_path = pathlib.Path(path)
    event_path.parent.mkdir(parents=True, exist_ok=True)
    with event_path.open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(record, ensure_ascii=False) + "\n")


def _atomic_write_text(path: pathlib.Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fh:
        fh.write(text)
        fh.flush()
        os.fsync(fh.fileno())
    os.replace(tmp, path)
