"""Lease helpers for backlog task ownership and stale-running reclaim."""

from __future__ import annotations

from datetime import datetime, timezone
import os

DEFAULT_TIMEOUT_SECONDS = 1800


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def attach_lease(
    task: dict,
    *,
    owner_pid: int,
    run_id: str = "",
    timeout_seconds: int = DEFAULT_TIMEOUT_SECONDS,
    heartbeat_at: str | None = None,
) -> dict:
    task["lease"] = {
        "owner_pid": int(owner_pid),
        "run_id": str(run_id or "").strip(),
        "heartbeat_at": heartbeat_at or now_iso(),
        "timeout_seconds": int(timeout_seconds),
    }
    return task


def heartbeat(task: dict, *, at: str | None = None) -> dict:
    lease = task.get("lease")
    if not isinstance(lease, dict):
        return task
    lease["heartbeat_at"] = at or now_iso()
    return task


def reclaim_stale_running(
    backlog: list[dict],
    *,
    current_owner_pid: int | None = None,
    is_pid_alive=None,
) -> dict:
    """Reclaim orphaned running tasks.

    A running task is reclaimed when:
    - it has no lease owner metadata, or
    - its lease owner pid is not alive, or
    - its heartbeat has exceeded timeout_seconds.
    """
    if is_pid_alive is None:
        is_pid_alive = _is_pid_alive

    items = [dict(item) for item in backlog]
    reclaimed = 0
    active = 0
    for item in items:
        if str(item.get("status", "")).lower() != "running":
            continue
        lease = item.get("lease") if isinstance(item.get("lease"), dict) else {}
        owner_pid = _coerce_int(lease.get("owner_pid"))
        if owner_pid is not None and current_owner_pid is not None and owner_pid == int(current_owner_pid):
            active += 1
            continue
        if owner_pid is None:
            _mark_reclaimed(item, "missing_owner")
            reclaimed += 1
            continue
        if is_pid_alive(owner_pid):
            if _is_lease_timed_out(lease):
                _mark_reclaimed(item, "heartbeat_timeout")
                reclaimed += 1
                continue
            active += 1
            continue
        _mark_reclaimed(item, "owner_dead")
        reclaimed += 1
    return {"backlog": items, "reclaimed": reclaimed, "active": active}


def _mark_reclaimed(task: dict, reason: str) -> None:
    task["status"] = "stopped"
    task["_lease_reclaimed_at"] = now_iso()
    task["_lease_reclaim_reason"] = reason


def _coerce_int(value) -> int | None:
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _parse_iso(value: str | None) -> datetime | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def _is_lease_timed_out(lease: dict) -> bool:
    beat = _parse_iso(str(lease.get("heartbeat_at") or ""))
    timeout = _coerce_int(lease.get("timeout_seconds")) or DEFAULT_TIMEOUT_SECONDS
    if beat is None:
        return True
    if beat.tzinfo is None:
        beat = beat.replace(tzinfo=timezone.utc)
    age = (datetime.now(timezone.utc) - beat).total_seconds()
    return age > max(timeout, 1)


def _is_pid_alive(pid: int) -> bool:
    try:
        os.kill(int(pid), 0)
        return True
    except OSError:
        return False
