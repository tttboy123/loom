"""Bridge between in-band lease (devkit/lease.py) and out-of-band lease (devkit/scheduler.py).

Two lease paradigms coexist in this codebase:

- ``devkit/lease.py`` — in-band: lease lives as a subfield on the task dict
  (``task["lease"]``). Lives only in the running process's memory. Default
  timeout: 1800s. Used by ``devkit/__main__.py`` and the autoloop/iterate
  run-loop.
- ``devkit/scheduler.py`` — out-of-band: lease lives in a separate JSON file
  at a path. Default stale window: 300s. Used by Phase D typed ``decide()``
  / ``select_next_pending()`` and the future ``devkit auto`` integration.

These are NOT directly swappable because their function signatures and
ownership model differ. This module provides the translation surface so
the two can coexist without duplicating state:

* :func:`sync_lease_to_file` — write an in-band ``task["lease"]`` to a
  scheduler-compatible JSON file (atomic).
* :func:`sync_lease_from_file` — read a scheduler-format file and return
  a ``task["lease"]`` subdict (or ``None``).
* :func:`release_lease_via_file` — delegate to
  :func:`devkit.scheduler.release_lease`.

The bridge is opt-in: existing callers of ``devkit.lease`` see no behavior
change unless they pass the new ``lease_path`` kwarg. ``reclaim_stale_running``
does not touch files because it operates on a backlog list (not a single
lease path).
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import tempfile
import uuid
from datetime import datetime, timezone
from typing import Any

logger = logging.getLogger("devkit.lease_sync")
if not logger.handlers:
    logger.addHandler(logging.NullHandler())
logger.setLevel(logging.INFO)


# ----------------------------------------------------------------------------
# Defaults — synced with lease.py (in-band) for cross-process compatibility.
# ----------------------------------------------------------------------------
# In-band timeout lives in lease.py; cross-process sync inherits that.
# The out-of-band scheduler uses 300s by default — we do NOT unify these.
DEFAULT_TIMEOUT_SECONDS = 1800


# ----------------------------------------------------------------------------
# Internal helpers
# ----------------------------------------------------------------------------
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _utc_now_epoch() -> float:
    return datetime.now(timezone.utc).timestamp()


def _parse_iso(value: Any) -> datetime | None:
    """Parse an ISO-8601 string into an aware datetime, or return None."""
    text = str(value or "").strip()
    if not text:
        return None
    try:
        dt = datetime.fromisoformat(text)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def _atomic_write_json(path: pathlib.Path, payload: dict) -> pathlib.Path:
    """Atomic write via tempfile + rename.

    Mirrors ``devkit.scheduler._atomic_write_json`` so cross-process consumers
    never see a half-written file. The scheduler helper is intentionally not
    imported here to avoid a hard dep; this is a small, stdlib-only
    duplication kept self-contained for testability.
    """
    path = pathlib.Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=str(path.parent)
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write(json.dumps(payload, ensure_ascii=False, indent=2))
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(tmp_name, path)
    except Exception:
        try:
            os.unlink(tmp_name)
        except OSError:
            pass
        raise
    return path


def _read_json(path: pathlib.Path) -> dict | None:
    """Read a JSON file and return the dict (or None for missing/malformed)."""
    if not path.exists():
        return None
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        logger.info("lease_sync: %s unreadable: %s", path, exc)
        return None
    return raw if isinstance(raw, dict) else None


def _build_lease_payload(
    *,
    work_item_id: str,
    run_id: str,
    owner_pid: int,
    heartbeat_at: str | None,
) -> dict:
    """Build a scheduler-compatible lease dict, including the in-band ``owner_pid``.

    The scheduler's native format is::

        {
          "protocol_version": "loom.dev/v1",
          "work_item_id": "...",
          "run_id": "...",
          "claimed_at": <epoch seconds>,
          "claimed_at_iso": "...",
          "lease_id": "lease-..."
        }

    We add an extra ``owner_pid`` field so cross-process consumers can see
    who owns the lease — ``devkit/scheduler.py`` itself does not write this
    field but tolerates it (it only reads the fields it knows about).
    """
    parsed = _parse_iso(heartbeat_at)
    if parsed is None:
        # Fresh claim: stamp now.
        now_epoch = _utc_now_epoch()
        claimed_at = now_epoch
        claimed_at_iso = _utc_now_iso()
    else:
        claimed_at = parsed.timestamp()
        claimed_at_iso = parsed.isoformat()

    return {
        "protocol_version": "loom.dev/v1",
        "work_item_id": work_item_id,
        "run_id": run_id,
        "claimed_at": claimed_at,
        "claimed_at_iso": claimed_at_iso,
        "lease_id": f"lease-{uuid.uuid4().hex[:12]}",
        # Extension field — in-band carry-over; scheduler doesn't write it
        # but its reader tolerates unknown keys.
        "owner_pid": int(owner_pid),
    }


# ----------------------------------------------------------------------------
# Public API
# ----------------------------------------------------------------------------
def sync_lease_to_file(task: dict, lease_path: str | pathlib.Path) -> None:
    """Write ``task["lease"]`` to ``lease_path`` in scheduler-compatible format.

    The task dict MUST have an ``id`` key (used as ``work_item_id``) and a
    ``lease`` subdict with at least ``owner_pid`` and ``run_id``. The lease's
    ``heartbeat_at`` (ISO timestamp) is used as the lease's ``claimed_at``
    so that heartbeats don't clobber the original claim epoch — if no
    heartbeat is set, the current UTC epoch is used.

    Raises:
        ValueError: when the task dict lacks the required keys or types.
        OSError: when the file cannot be written.
    """
    if not isinstance(task, dict):
        raise ValueError("task must be a dict")

    work_item_id = str(task.get("id", "")).strip()
    if not work_item_id:
        raise ValueError(
            "task['id'] must be a non-empty string to sync to a lease file"
        )

    lease = task.get("lease")
    if not isinstance(lease, dict):
        raise ValueError(
            "task['lease'] must be a dict to sync to a lease file "
            "(call devkit.lease.attach_lease first)"
        )

    owner_pid_raw = lease.get("owner_pid")
    try:
        owner_pid = int(owner_pid_raw)
    except (TypeError, ValueError) as exc:
        raise ValueError(
            f"task['lease']['owner_pid'] must be int-like, got {owner_pid_raw!r}"
        ) from exc

    run_id = str(lease.get("run_id", "")).strip()
    heartbeat_at = lease.get("heartbeat_at")

    payload = _build_lease_payload(
        work_item_id=work_item_id,
        run_id=run_id,
        owner_pid=owner_pid,
        heartbeat_at=str(heartbeat_at) if heartbeat_at else None,
    )
    _atomic_write_json(pathlib.Path(lease_path), payload)


def sync_lease_from_file(lease_path: str | pathlib.Path) -> dict | None:
    """Read a scheduler-format lease file and return a ``task["lease"]`` subdict.

    Returns ``None`` when the file is missing, malformed, or not a JSON
    object. The returned dict has the same shape that
    :func:`devkit.lease.attach_lease` would write::

        {
          "owner_pid": int,
          "run_id": str,
          "heartbeat_at": str (ISO-8601),
          "timeout_seconds": int,
        }

    The ``heartbeat_at`` prefers an explicit ``heartbeat_at`` field on disk
    (if a future writer sets one) and falls back to ``claimed_at_iso``.
    ``timeout_seconds`` defaults to :data:`DEFAULT_TIMEOUT_SECONDS` (1800s),
    matching the in-band lease — we do NOT inherit the scheduler's 300s
    default because that would silently weaken existing reclaim logic.
    """
    data = _read_json(pathlib.Path(lease_path))
    if not data:
        return None

    run_id = str(data.get("run_id", "")).strip()

    owner_pid_raw = data.get("owner_pid")
    try:
        owner_pid = int(owner_pid_raw) if owner_pid_raw is not None else 0
    except (TypeError, ValueError):
        owner_pid = 0

    heartbeat_at = (
        str(data.get("heartbeat_at", "")).strip()
        or str(data.get("claimed_at_iso", "")).strip()
    )

    timeout_raw = data.get("timeout_seconds")
    try:
        timeout_seconds = int(timeout_raw) if timeout_raw is not None else DEFAULT_TIMEOUT_SECONDS
    except (TypeError, ValueError):
        timeout_seconds = DEFAULT_TIMEOUT_SECONDS

    return {
        "owner_pid": owner_pid,
        "run_id": run_id,
        "heartbeat_at": heartbeat_at,
        "timeout_seconds": timeout_seconds,
    }


def release_lease_via_file(lease_path: str | pathlib.Path) -> None:
    """Best-effort delete of the lease file.

    Delegates to :func:`devkit.scheduler.release_lease` when available;
    falls back to a local unlink if the scheduler module is not yet
    importable. Missing file is not an error.
    """
    try:
        from devkit.scheduler import release_lease as _scheduler_release  # type: ignore
    except ImportError:
        # Fallback: stdlib unlink. Best-effort, missing is fine.
        path = pathlib.Path(lease_path)
        try:
            path.unlink()
        except FileNotFoundError:
            return
        except OSError as exc:
            logger.info("lease_sync: could not release lease %s: %s", path, exc)
        return
    _scheduler_release(lease_path)


__all__ = [
    "sync_lease_to_file",
    "sync_lease_from_file",
    "release_lease_via_file",
    "DEFAULT_TIMEOUT_SECONDS",
]