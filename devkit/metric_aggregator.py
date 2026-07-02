# devkit/metric_aggregator.py
# Pure standard library — no external deps.
# Aggregates metrics across multiple pipeline stages.

def aggregate(metrics: list[dict], field: str) -> dict:
    """Aggregate the values of `field` across a list of metric dicts.

    Returns:
        dict with keys: count, sum, avg, min, max.
        For an empty input list, count is 0 and sum/avg/min/max are all 0.0.
    """
    values = [float(m[field]) for m in metrics if field in m]

    if not values:
        return {
            "count": 0,
            "sum": 0.0,
            "avg": 0.0,
            "min": 0.0,
            "max": 0.0,
        }

    total = sum(values)
    return {
        "count": len(values),
        "sum": total,
        "avg": total / len(values),
        "min": min(values),
        "max": max(values),
    }

def merge(a: dict, b: dict) -> dict:
    """Shallow-merge two metric dicts; values in `b` win on key conflicts.

    Returns a NEW dict — neither input is mutated.
    """
    result = dict(a)
    result.update(b)
    return result

def normalize(metrics: list[dict], field: str) -> list[dict]:
    """Min-max normalize `field` to [0, 1] across the list.

    If min == max (or the list is empty / field missing everywhere),
    all returned values for the field are 0.0.

    Returns a NEW list of NEW dicts — originals are not mutated.
    """
    values = [float(m[field]) for m in metrics if field in m]

    if not values:
        return [dict(m) for m in metrics]

    lo = min(values)
    hi = max(values)
    span = hi - lo

    out = []
    for m in metrics:
        new_m = dict(m)
        if field in m:
            v = float(m[field])
            new_m[field] = 0.0 if span == 0 else (v - lo) / span
        out.append(new_m)
    return out
