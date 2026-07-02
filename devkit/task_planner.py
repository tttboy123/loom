# task_planner.py
"""Devkit 任务规划器（纯标准库）。

将复杂任务按换行拆分为子任务，生成带 id / 优先级 / 状态的计划，并提供汇总。
"""
from __future__ import annotations

from typing import Dict, List

def split(task: str, n: int) -> List[str]:
    """将 task 按换行拆分为最多 n 个子任务。

    - n <= 0 时返回空列表。
    - 每段去除首尾空格并过滤空行。
    """
    if n <= 0:
        return []
    if not task:
        return []
    pieces: List[str] = []
    for line in task.split('\n'):
        stripped = line.strip()
        if stripped:
            pieces.append(stripped)
            if len(pieces) >= n:
                break
    return pieces

def make_plan(tasks: List[str], priority: str = 'medium') -> List[Dict[str, str]]:
    """根据任务列表生成计划项。"""
    return [
        {
            'id': f'task-{i + 1}',
            'task': t,
            'priority': priority,
            'status': 'pending',
        }
        for i, t in enumerate(tasks)
    ]

def plan_summary(plan: List[Dict[str, str]]) -> Dict[str, object]:
    """汇总计划：总数、按优先级分组、待办数。"""
    by_priority: Dict[str, int] = {}
    pending = 0
    for item in plan:
        p = item.get('priority', '')
        by_priority[p] = by_priority.get(p, 0) + 1
        if item.get('status') == 'pending':
            pending += 1
    return {
        'total': len(plan),
        'by_priority': by_priority,
        'pending': pending,
    }
