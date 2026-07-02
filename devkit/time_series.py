# devkit/time_series.py
"""时序数据存储与统计（纯标准库）。"""

from __future__ import annotations

def create() -> dict:
    """返回空时序结构 {points: [], count: 0}。"""
    return {"points": [], "count": 0}

def append(ts: dict, timestamp: float, value: float) -> dict:
    """追加 {timestamp, value}；count += 1；按 timestamp 排序；返回新 ts。"""
    ts["points"].append({"timestamp": timestamp, "value": value})
    ts["points"].sort(key=lambda p: p["timestamp"])
    ts["count"] += 1
    return ts

def range_query(ts: dict, start: float, end: float) -> list[dict]:
    """返回 start <= timestamp <= end 的点。"""
    return [
        p for p in ts["points"]
        if start <= p["timestamp"] <= end
    ]

def ts_summary(ts: dict) -> dict:
    """返回 {count, min_val, max_val, avg_val, first_ts, last_ts}。
    空时 min_val/max_val/avg_val/first_ts/last_ts 均为 None。"""
    points = ts["points"]
    count = ts["count"]
    if count == 0:
        return {
            "count": 0,
            "min_val": None,
            "max_val": None,
            "avg_val": None,
            "first_ts": None,
            "last_ts": None,
        }
    values = [p["value"] for p in points]
    min_val = min(values)
    max_val = max(values)
    avg_val = sum(values) / count
    first_ts = points[0]["timestamp"]
    last_ts = points[-1]["timestamp"]
    return {
        "count": count,
        "min_val": min_val,
        "max_val": max_val,
        "avg_val": avg_val,
        "first_ts": first_ts,
        "last_ts": last_ts,
    }
