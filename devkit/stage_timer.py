# devkit/stage_timer.py
"""Record and analyze stage durations. Pure stdlib."""
from datetime import datetime, timezone

def record(stage: str, duration_s: float) -> dict:
    """Capture a single stage measurement.

    Returns a dict with keys: stage, duration_s, timestamp (ISO-8601 string).
    """
    return {
        "stage": stage,
        "duration_s": float(duration_s),
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }

def stats(records: list[dict]) -> dict:
    """Aggregate stats over a list of record() dicts.

    Empty input -> count=0, total/avg/min/max all 0.0.
    """
    if not records:
        return {"count": 0, "total": 0.0, "avg": 0.0, "min": 0.0, "max": 0.0}
    durations = [r["duration_s"] for r in records]
    total = sum(durations)
    count = len(durations)
    return {
        "count": count,
        "total": total,
        "avg": total / count,
        "min": min(durations),
        "max": max(durations),
    }

def slowest(records: list[dict], n: int) -> list[dict]:
    """Top-n records by duration_s descending. n<=0 -> []."""
    if n <= 0:
        return []
    return sorted(records, key=lambda r: r["duration_s"], reverse=True)[:n]

def timer_report(records: list[dict]) -> str:
    """One-line human summary: 'count=N avg=..s total=..s'."""
    s = stats(records)
    return f"count={s['count']} avg={s['avg']:.2f}s total={s['total']:.2f}s"
