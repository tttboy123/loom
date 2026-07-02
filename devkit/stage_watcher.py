"""stage_watcher.py

Pure-stdlib watcher for stage status changes.

Provides:
    create() -> dict
    record_change(watcher, stage, old_status, new_status) -> dict
    get_status(watcher, stage) -> str | None
    change_summary(watcher) -> dict
"""

from datetime import datetime, timezone

def create() -> dict:
    """Create a new, empty stage watcher.

    Returns:
        A watcher dict shaped as ``{"events": [], "stages": {}}``.
    """
    return {"events": [], "stages": {}}

def _now_iso() -> str:
    """Return the current UTC time as an ISO 8601 string.

    Uses ``datetime.now(timezone.utc)`` so every event has a real,
    unambiguous timestamp (``...+00:00``) suitable for ordering and
    serialization.
    """
    return datetime.now(timezone.utc).isoformat()

def record_change(
    watcher: dict,
    stage: str,
    old_status: str,
    new_status: str,
) -> dict:
    """Record a stage status change and return the updated watcher.

    A new event ``{"stage", "old", "new", "timestamp"}`` is appended to
    ``watcher["events"]``, and ``watcher["stages"][stage]`` is set to
    ``new_status``. The watcher is mutated in place and also returned,
    matching a fluent style.
    """
    watcher["events"].append(
        {
            "stage": stage,
            "old": old_status,
            "new": new_status,
            "timestamp": _now_iso(),
        }
    )
    watcher["stages"][stage] = new_status
    return watcher

def get_status(watcher: dict, stage: str) -> "str | None":
    """Return the current status of ``stage``, or ``None`` if unknown."""
    return watcher["stages"].get(stage)

def change_summary(watcher: dict) -> dict:
    """Return a summary of recorded activity in the watcher.

    Shape::

        {
            "total_events": int,
            "stages_tracked": int,
            "latest": dict | None,
        }
    """
    events = watcher["events"]
    return {
        "total_events": len(events),
        "stages_tracked": len(watcher["stages"]),
        "latest": events[-1] if events else None,
    }
