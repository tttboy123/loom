"""Pure-stdlib backlog statistics. Draft — not yet wired to real data.

Provides three pure functions over a backlog expressed as list[dict]:
    stats(backlog)            -> aggregate counts + completion + priority mix
    velocity(backlog, window)  -> approximate done/failed counts
    health_score(backlog)      -> done / (done + failed), with neutral default
"""

from __future__ import annotations
from typing import Any

_KNOWN_STATUSES = ('done', 'pending', 'failed', 'running', 'stopped')
_KNOWN_PRIORITIES = ('high', 'medium', 'low')

def stats(backlog: list[dict[str, Any]]) -> dict[str, Any]:
    """Aggregate counts, completion percentage, and priority breakdown.

    completion_pct = done / total * 100, or 0.0 when total == 0.
    by_priority counts items across all statuses; unknown priorities are ignored.
    """
    total = len(backlog)

    status_counts = {s: 0 for s in _KNOWN_STATUSES}
    by_priority = {p: 0 for p in _KNOWN_PRIORITIES}

    for item in backlog:
        status = item.get('status')
        if status in status_counts:
            status_counts[status] += 1
        priority = item.get('priority')
        if priority in by_priority:
            by_priority[priority] += 1

    completion_pct = (status_counts['done'] / total * 100.0) if total > 0 else 0.0

    return {
        'total': total,
        'done': status_counts['done'],
        'pending': status_counts['pending'],
        'failed': status_counts['failed'],
        'running': status_counts['running'],
        'stopped': status_counts['stopped'],
        'completion_pct': completion_pct,
        'by_priority': by_priority,
    }

def velocity(backlog: list[dict[str, Any]], window: int = 7) -> dict[str, int]:
    """Approximate velocity.

    No timestamps available in this version, so done_in_window / failed_in_window
    collapse to the totals across the entire backlog. `window` is kept in the
    signature for forward-compatibility with a future time-aware implementation.
    """
    del window  # intentionally unused — API parity only
    done_in_window = sum(1 for it in backlog if it.get('status') == 'done')
    failed_in_window = sum(1 for it in backlog if it.get('status') == 'failed')
    return {
        'done_in_window': done_in_window,
        'failed_in_window': failed_in_window,
    }

def health_score(backlog: list[dict[str, Any]]) -> float:
    """Health = done / (done + failed).

    Returns 0.5 when there are no terminal outcomes (empty backlog or only
    pending/running items), to avoid over-reacting to cold starts.
    """
    done = sum(1 for it in backlog if it.get('status') == 'done')
    failed = sum(1 for it in backlog if it.get('status') == 'failed')
    denom = done + failed
    return done / denom if denom > 0 else 0.5
