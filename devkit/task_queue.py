# devkit/task_queue.py
"""
优先级任务队列（纯标准库）。

优先级（高→低）：critical > high > medium > low
未知优先级视为最低。
所有函数均返回新对象，不修改入参。
"""
from __future__ import annotations

from copy import deepcopy
from typing import Optional

_PRIORITY_ORDER = {"critical": 0, "high": 1, "medium": 2, "low": 3}
_UNKNOWN_RANK = len(_PRIORITY_ORDER)  # 未知优先级排在最末

def _rank(task: dict) -> int:
    return _PRIORITY_ORDER.get(task.get("priority"), _UNKNOWN_RANK)

def push(queue: list[dict], task: dict) -> list[dict]:
    """按优先级插入任务；返回新列表（不修改原队列）。"""
    new_queue = list(queue) + [task]
    new_queue.sort(key=_rank)
    return new_queue

def pop(queue: list[dict]) -> tuple[Optional[dict], list[dict]]:
    """取出最高优先级任务；空队列返回 (None, [])。"""
    if not queue:
        return None, []
    sorted_queue = sorted(queue, key=_rank)
    head, *rest = sorted_queue
    return head, rest

def queue_stats(queue: list[dict]) -> dict:
    """返回队列统计：total / by_priority / top_id。"""
    if not queue:
        return {"total": 0, "by_priority": {}, "top_id": None}

    by_priority: dict[str, int] = {}
    for t in queue:
        p = t.get("priority", "unknown")
        by_priority[p] = by_priority.get(p, 0) + 1

    top_id = sorted(queue, key=_rank)[0].get("id")
    return {
        "total": len(queue),
        "by_priority": by_priority,
        "top_id": top_id,
    }
