"""snapshot_store.py — 键值快照存储（纯标准库）

公开 API：
- create() -> dict
- save(store, key, value) -> dict
- load(store, key)
- list_keys(store) -> list[str]
- store_summary(store) -> dict
"""
from __future__ import annotations

import copy
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

def create() -> dict:
    """创建一个新的空快照存储。

    Returns:
        {snapshots: {}, version: 0}
    """
    return {"snapshots": {}, "version": 0}

def save(store: dict, key: str, value: Any) -> dict:
    """保存一条快照。返回新的 store（不修改入参）。

    - snapshots[key] = {"value": value, "version": store["version"]+1, "timestamp": ISO}
    - version 自增 1
    """
    new_version = store["version"] + 1
    new_store = {
        "snapshots": dict(store["snapshots"]),
        "version": new_version,
    }
    new_store["snapshots"][key] = {
        "value": value,
        "version": new_version,
        "timestamp": _now_iso(),
    }
    return new_store

def load(store: dict, key: str) -> Any:
    """读取某个 key 的快照值。key 不存在时返回 None。"""
    entry = store["snapshots"].get(key)
    if entry is None:
        return None
    return entry["value"]

def list_keys(store: dict) -> List[str]:
    """返回所有已保存的 key（排序）。"""
    return sorted(store["snapshots"].keys())

def store_summary(store: dict) -> dict:
    """返回 store 的摘要信息。"""
    keys = list_keys(store)
    return {
        "version": store["version"],
        "total_keys": len(keys),
        "keys": keys,
    }

def _now_iso() -> str:
    """返回当前 UTC 时间的 ISO 8601 字符串（带 Z 后缀）。"""
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
