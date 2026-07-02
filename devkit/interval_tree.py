# devkit/interval_tree.py

def create() -> dict:
    """Create a new empty interval tree."""
    return {"intervals": [], "count": 0}

def insert(it: dict, start: float, end: float, data=None) -> dict:
    """Insert an interval [start, end] with optional data. Returns the updated tree dict."""
    interval = {"start": start, "end": end, "data": data}
    it["intervals"].append(interval)
    it["count"] += 1
    return it

def query(it: dict, point: float) -> list[dict]:
    """Return all intervals that contain `point` (start <= point <= end)."""
    return [iv for iv in it["intervals"] if iv["start"] <= point <= iv["end"]]

def overlap(it: dict, start: float, end: float) -> list[dict]:
    """Return all intervals that overlap with [start, end]."""
    return [
        iv
        for iv in it["intervals"]
        if not (iv["end"] < start or iv["start"] > end)
    ]

def it_summary(it: dict) -> dict:
    """Return summary: count, min_start, max_end."""
    if it["count"] == 0:
        return {"count": 0, "min_start": None, "max_end": None}
    min_start = min(iv["start"] for iv in it["intervals"])
    max_end = max(iv["end"] for iv in it["intervals"])
    return {"count": it["count"], "min_start": min_start, "max_end": max_end}
