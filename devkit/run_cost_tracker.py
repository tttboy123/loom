"""devkit/run_cost_tracker.py

纯标准库：追踪 run 的 token 和费用。

提供三个函数：
  - track(stages):           汇总多阶段统计
  - cost_per_token(...):     计算单 token 成本
  - budget_check(...):       预算检查
"""

from typing import Dict, List, Optional

def track(stages: List[dict]) -> dict:
    """汇总 stages 列表，返回 total / by_stage / most_expensive。

    Args:
        stages: 形如 [{'name': str, 'tokens': int, 'cost': float}, ...]

    Returns:
        {
            'total_tokens': int,
            'total_cost': float,
            'by_stage': {name: {'tokens': int, 'cost': float}, ...},
            'most_expensive': str | None
        }
    """
    total_tokens = 0
    total_cost = 0.0
    by_stage: Dict[str, dict] = {}

    most_expensive: Optional[str] = None
    max_cost: Optional[float] = None

    for stage in stages:
        name = stage['name']
        tokens = int(stage['tokens'])
        cost = float(stage['cost'])

        total_tokens += tokens
        total_cost += cost
        by_stage[name] = {'tokens': tokens, 'cost': cost}

        if max_cost is None or cost > max_cost:
            max_cost = cost
            most_expensive = name

    return {
        'total_tokens': total_tokens,
        'total_cost': total_cost,
        'by_stage': by_stage,
        'most_expensive': most_expensive,
    }

def cost_per_token(tokens: int, cost: float) -> float:
    """返回 cost / tokens；tokens == 0 时返回 0.0。"""
    if tokens == 0:
        return 0.0
    return cost / tokens

def budget_check(total_cost: float, budget: float) -> dict:
    """预算检查。

    Returns:
        {
            'within_budget': bool,
            'remaining': float,
            'pct_used': float
        }
        budget == 0 时 pct_used=0.0
    """
    remaining = budget - total_cost
    within_budget = total_cost <= budget
    if budget == 0:
        pct_used = 0.0
    else:
        pct_used = total_cost / budget * 100
    return {
        'within_budget': within_budget,
        'remaining': remaining,
        'pct_used': pct_used,
    }
