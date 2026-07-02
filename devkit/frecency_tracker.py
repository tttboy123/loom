# devkit/frecency_tracker.py
"""Frecency tracker: score = count * decay ** (timestamp - last_access).

Pure standard library. All functions are pure: inputs are not mutated.
"""

from __future__ import annotations

from typing import Dict, List

def create(decay: float = 0.9) -> dict:
    """Create a new, empty tracker with the given decay factor."""
    if not isinstance(decay, (int, float)):
        raise TypeError("decay must be a number")
    return {"decay": float(decay), "items": {}}

def access(tracker: dict, key: str, timestamp: int) -> dict:
    """Record an access of `key` at `timestamp`; return a new tracker.

    For the first access: count=1, score=1.0, last_access=timestamp.
    For subsequent accesses:
        count     -> count + 1
        score     -> count * decay ** (timestamp - last_access)
        last_access -> timestamp
    """
    if not isinstance(timestamp, int):
        raise TypeError("timestamp must be an int")

    decay = tracker["decay"]
    # Shallow-copy the items dict so the original tracker is untouched.
    new_items: Dict[str, dict] = {k: dict(v) for k, v in tracker["items"].items()}

    entry = new_items.get(key)
    if entry is None:
        new_items[key] = {
            "count": 1,
            "last_access": int(timestamp),
            "score": 1.0,
        }
    else:
        delta = int(timestamp) - int(entry["last_access"])
        new_count = entry["count"] + 1
        new_items[key] = {
            "count": new_count,
            "last_access": int(timestamp),
            "score": new_count * (decay ** delta),
        }

    return {"decay": decay, "items": new_items}

def top_n(tracker: dict, n: int) -> List[str]:
    """Return up to `n` keys ordered by descending score (ties: insertion order)."""
    if n <= 0 or not tracker["items"]:
        return []

    ranked = sorted(
        tracker["items"].items(),
        key=lambda kv: kv[1]["score"],
        reverse=True,
    )
    return [k for k, _ in ranked[:n]]

def tracker_summary(tracker: dict) -> dict:
    """Return a small summary dict: total_items + decay."""
    return {
        "total_items": len(tracker["items"]),
        "decay": tracker["decay"],
    }
