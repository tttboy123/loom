"""devkit/stage_replay.py

Pure-stdlib stage execution replay. Draft, not committed to a real repo.

Public API:
    create_replay(events) -> dict
    step(replay)          -> (current_event, new_replay)
    replay_all(replay)    -> list[dict]
    replay_summary(replay) -> dict
"""

from copy import deepcopy

def create_replay(events):
    """Wrap an event list into a replay state.

    Args:
        events: list of dicts with at least {stage, status, timestamp}.

    Returns:
        dict with keys {events, cursor, total}. Cursor starts at 0.
    """
    return {
        "events": list(events),
        "cursor": 0,
        "total": len(events),
    }

def step(replay):
    """Advance the cursor by one and return (current_event, new_replay).

    Does not mutate the input replay — returns a new dict.
    Returns (None, replay) once the cursor has reached the end.
    """
    new_replay = deepcopy(replay)
    cursor = new_replay["cursor"]
    total = new_replay["total"]

    if cursor >= total:
        return None, new_replay

    current_event = new_replay["events"][cursor]
    new_replay["cursor"] = cursor + 1
    return current_event, new_replay

def replay_all(replay):
    """Return the full event list without moving the cursor."""
    return list(replay["events"])

def replay_summary(replay):
    """Return {total, cursor, remaining, done} for a replay state."""
    total = replay["total"]
    cursor = replay["cursor"]
    remaining = max(0, total - cursor)
    done = remaining == 0
    return {
        "total": total,
        "cursor": cursor,
        "remaining": remaining,
        "done": done,
    }
