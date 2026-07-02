"""Score carriers from historical records. Pure standard library."""

from __future__ import annotations

def score(records: list[dict]) -> dict:
    """Aggregate records by carrier.

    Each record must contain: carrier (str), ok (bool), tokens (int).
    Returns {carrier_name: {"ok_rate": float, "avg_tokens": float, "count": int}}.
    ok_rate = ok_count / count; 0.0 when count == 0.
    """
    bucket: dict = {}
    for r in records:
        name = r["carrier"]
        if name not in bucket:
            bucket[name] = {"ok_count": 0, "token_sum": 0, "count": 0}
        s = bucket[name]
        s["count"] += 1
        s["token_sum"] += r["tokens"]
        if r["ok"]:
            s["ok_count"] += 1

    out: dict = {}
    for name, s in bucket.items():
        count = s["count"]
        out[name] = {
            "ok_rate": s["ok_count"] / count if count else 0.0,
            "avg_tokens": s["token_sum"] / count if count else 0.0,
            "count": count,
        }
    return out

def best_carrier(records: list[dict], min_count: int = 1) -> str:
    """Carrier with highest ok_rate among those with count >= min_count.

    Ties broken by lower avg_tokens. '' when none qualifies.
    """
    scored = score(records)
    candidates = [n for n, s in scored.items() if s["count"] >= min_count]
    if not candidates:
        return ""
    # primary key: -ok_rate (desc), secondary: avg_tokens (asc)
    candidates.sort(key=lambda n: (-scored[n]["ok_rate"], scored[n]["avg_tokens"]))
    return candidates[0]

def rank_carriers(records: list[dict]) -> list[str]:
    """Carrier names sorted by ok_rate descending, deduplicated."""
    scored = score(records)
    # score() already dedupes by carrier; sort by -ok_rate (stable on insertion order)
    return sorted(scored.keys(), key=lambda n: -scored[n]["ok_rate"])
