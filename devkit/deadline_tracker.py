"""
devkit/deadline_tracker.py

纯标准库实现的任务截止时间追踪器。
所有函数均为**纯函数**：不修改入参 dict，而是返回新的 dict。

公开 API：
    create() -> dict
    set_deadline(tracker, task_id, deadline_iso) -> dict
    mark_done(tracker, task_id) -> dict
    check_overdue(tracker, now_iso) -> dict
    tracker_summary(tracker) -> dict
"""

from __future__ import annotations

import copy
from datetime import datetime
from typing import Any, Dict, List

# ---------------------------------------------------------------------------
# 内部辅助
# ---------------------------------------------------------------------------

def _clone(tracker: Dict[str, Any]) -> Dict[str, Any]:
    """深拷贝 tracker，避免污染调用方持有的对象。"""
    return copy.deepcopy(tracker)

def _parse_iso(iso_str: str) -> datetime:
    """
    解析 ISO-8601 时间字符串。

    优先尝试完整 datetime（'YYYY-MM-DDTHH:MM:SS'），失败则回退到纯日期
    （'YYYY-MM-DD'，按当日 00:00:00 处理）。

    任何无法解析的输入直接抛出 ValueError，不做静默吞错。
    """
    if not isinstance(iso_str, str) or not iso_str:
        raise ValueError(f"invalid ISO datetime string: {iso_str!r}")

    # 1) 完整 datetime
    try:
        return datetime.fromisoformat(iso_str)
    except ValueError:
        pass

    # 2) 仅日期 (YYYY-MM-DD) —— 当日 00:00:00
    try:
        return datetime.strptime(iso_str, "%Y-%m-%d")
    except ValueError as exc:
        raise ValueError(
            f"unrecognized ISO datetime string: {iso_str!r}"
        ) from exc

# ---------------------------------------------------------------------------
# 公开 API
# ---------------------------------------------------------------------------

def create() -> Dict[str, Any]:
    """构造一个空的 tracker。"""
    return {"deadlines": {}, "overdue": []}

def set_deadline(
    tracker: Dict[str, Any], task_id: str, deadline_iso: str
) -> Dict[str, Any]:
    """
    记录/覆盖 task_id 的截止时间。

    返回新 tracker；不会修改入参。
    """
    if not isinstance(task_id, str) or not task_id:
        raise ValueError(f"task_id must be a non-empty string, got {task_id!r}")

    # 立即校验 ISO 字符串是否合法（fail-fast）
    _parse_iso(deadline_iso)

    new_tracker = _clone(tracker)
    new_tracker["deadlines"][task_id] = {
        "deadline": deadline_iso,
        "done": False,
    }

    # 设了新 deadline 后，已 done 的任务不应再被视为 overdue
    new_tracker["overdue"] = [
        tid for tid in new_tracker["overdue"] if tid != task_id
    ]
    return new_tracker

def mark_done(tracker: Dict[str, Any], task_id: str) -> Dict[str, Any]:
    """将 task_id 标记为已完成。从 overdue 列表中移除（若存在）。"""
    if "deadlines" not in tracker or task_id not in tracker["deadlines"]:
        raise KeyError(f"unknown task_id: {task_id!r}")

    new_tracker = _clone(tracker)
    new_tracker["deadlines"][task_id]["done"] = True
    new_tracker["overdue"] = [
        tid for tid in new_tracker["overdue"] if tid != task_id
    ]
    return new_tracker

def check_overdue(tracker: Dict[str, Any], now_iso: str) -> Dict[str, Any]:
    """
    重新计算 overdue 列表：
    deadline < now_iso 且 done == False 的 task_id。
    """
    now_dt = _parse_iso(now_iso)

    new_tracker = _clone(tracker)
    new_overdue: List[str] = []
    for tid, entry in new_tracker["deadlines"].items():
        if entry.get("done"):
            continue
        deadline_dt = _parse_iso(entry["deadline"])
        if deadline_dt < now_dt:
            new_overdue.append(tid)
    new_tracker["overdue"] = new_overdue
    return new_tracker

def tracker_summary(tracker: Dict[str, Any]) -> Dict[str, int]:
    """
    返回统计快照：
        total   : 截止时间条目总数
        done    : 已完成数
        overdue : 当前 overdue 列表长度（以 tracker["overdue"] 为准）
        pending : total - done
    """
    deadlines = tracker.get("deadlines", {})
    total = len(deadlines)
    done = sum(1 for e in deadlines.values() if e.get("done"))
    overdue = len(tracker.get("overdue", []))
    pending = total - done
    return {
        "total": total,
        "done": done,
        "overdue": overdue,
        "pending": pending,
    }
