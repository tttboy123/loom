# devkit/feedback_collector.py
"""Collect and summarize stage feedback. Pure stdlib."""
from __future__ import annotations

from datetime import datetime, timezone
from typing import TypedDict

class FeedbackEntry(TypedDict):
    stage: str
    rating: int
    comment: str
    timestamp: str

VALID_RATINGS = range(1, 6)  # 1..5 inclusive

def _now_iso() -> str:
    # UTC, ISO-8601 with seconds precision
    return datetime.now(timezone.utc).isoformat(timespec='seconds')

def add(
    feedback: list[dict],
    stage: str,
    rating: int,
    comment: str = '',
) -> list[dict]:
    """Append a feedback entry; return a NEW list (do not mutate input)."""
    entry: FeedbackEntry = {
        'stage': stage,
        'rating': rating,
        'comment': comment,
        'timestamp': _now_iso(),
    }
    # New list so callers cannot accidentally mutate history via reference.
    return [*feedback, entry]

def summary(feedback: list[dict]) -> dict:
    """Aggregate totals and per-stage stats. Empty-safe."""
    if not feedback:
        return {'total': 0, 'avg_rating': 0.0, 'by_stage': {}}

    total = len(feedback)
    avg_rating = sum(e['rating'] for e in feedback) / total

    by_stage: dict[str, dict] = {}
    for e in feedback:
        slot = by_stage.setdefault(e['stage'], {'count': 0, 'sum': 0})
        slot['count'] += 1
        slot['sum'] += e['rating']

    by_stage_out = {
        stage: {
            'count': data['count'],
            'avg_rating': data['sum'] / data['count'],
        }
        for stage, data in by_stage.items()
    }

    return {'total': total, 'avg_rating': avg_rating, 'by_stage': by_stage_out}

def filter_by_rating(feedback: list[dict], min_rating: int) -> list[dict]:
    """Return entries with rating >= min_rating. Inclusive, non-mutating."""
    return [e for e in feedback if e['rating'] >= min_rating]
