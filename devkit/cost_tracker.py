"""Track model call costs. Stdlib only."""
from __future__ import annotations
from datetime import datetime, timezone


def record(carrier: str, tokens: int, cost_usd: float) -> dict:
    return {"carrier": carrier, "tokens": tokens, "cost_usd": cost_usd,
            "timestamp": datetime.now(timezone.utc).isoformat()}


def total_cost(records: list[dict]) -> float:
    return sum(r.get("cost_usd", 0.0) for r in records)


def by_carrier(records: list[dict]) -> dict:
    result: dict[str, dict] = {}
    for r in records:
        c = r.get("carrier", "")
        if c not in result:
            result[c] = {"tokens": 0, "cost_usd": 0.0, "count": 0}
        result[c]["tokens"] += r.get("tokens", 0)
        result[c]["cost_usd"] += r.get("cost_usd", 0.0)
        result[c]["count"] += 1
    return result


def cost_report(records: list[dict]) -> str:
    if not records:
        return "(no records)"
    agg = by_carrier(records)
    lines = [f"{c}: {v['count']} calls, {v['tokens']} tokens, ${v['cost_usd']:.4f}"
             for c, v in sorted(agg.items())]
    return "\n".join(lines)
