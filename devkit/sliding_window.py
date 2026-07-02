# devkit/sliding_window.py
"""Sliding window statistics using only the Python standard library."""

def create(size: int) -> dict:
    return {"size": size, "items": [], "total": 0}

def push(window: dict, value: float) -> dict:
    items = list(window.get("items", []))
    items.append(value)

    size = window["size"]
    while len(items) > size:
        items.pop(0)

    return {
        "size": size,
        "items": items,
        "total": sum(items),
    }

def avg(window: dict) -> float:
    items = window.get("items", [])
    if not items:
        return 0.0
    return window["total"] / len(items)

def window_summary(window: dict) -> dict:
    items = window.get("items", [])
    count = len(items)

    if count == 0:
        min_value = 0.0
        max_value = 0.0
    else:
        min_value = min(items)
        max_value = max(items)

    return {
        "size": window["size"],
        "count": count,
        "total": window["total"],
        "avg": avg(window),
        "min": min_value,
        "max": max_value,
    }
