"""Task lifecycle event tracking — pure stdlib."""
from datetime import datetime, timezone

def record_event(task_id: str, event: str, metadata: dict | None = None) -> dict:
    """Record a single lifecycle event.

    Returns a dict with keys: task_id, event, metadata, timestamp.
    timestamp is an ISO 8601 string in UTC.
    """
    return {
        'task_id': task_id,
        'event': event,
        'metadata': dict(metadata) if metadata else {},
        'timestamp': datetime.now(timezone.utc).isoformat(),
    }

def lifecycle_summary(events: list[dict]) -> dict:
    """Summarize a list of events returned by record_event.

    Returns {total, task_ids, event_types} with task_ids and
    event_types deduplicated and sorted.
    """
    task_ids = sorted({e['task_id'] for e in events})
    event_types = sorted({e['event'] for e in events})
    return {
        'total': len(events),
        'task_ids': task_ids,
        'event_types': event_types,
    }

def filter_events(events: list[dict], task_id: str) -> list[dict]:
    """Return only events whose task_id matches."""
    return [e for e in events if e['task_id'] == task_id]
