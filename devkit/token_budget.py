"""Token budget tracker (pure stdlib).

A small in-memory budget object with five operations:
  new_budget, consume, is_exhausted, reset, budget_report

`consume` and `reset` mutate the budget dict in place and also return it,
so callers can chain or assign. `remaining` is clamped to a minimum of 0,
so consuming past the limit never yields a negative remaining value.
"""

def new_budget(limit: int) -> dict:
    """Create a fresh budget with the given token limit."""
    return {'limit': limit, 'used': 0, 'remaining': limit}

def consume(budget: dict, tokens: int) -> dict:
    """Consume `tokens` from `budget` in place; return the budget.

    Updates `used` and `remaining`. `remaining` is clamped to a minimum of 0,
    so consuming more than the limit yields `remaining == 0` (budget exhausted),
    not a negative number.
    """
    budget['used'] += tokens
    budget['remaining'] = max(0, budget['limit'] - budget['used'])
    return budget

def is_exhausted(budget: dict) -> bool:
    """Return True if no tokens remain (remaining <= 0)."""
    return budget['remaining'] <= 0

def reset(budget: dict) -> dict:
    """Reset used=0 and remaining=limit in place; return the budget."""
    budget['used'] = 0
    budget['remaining'] = budget['limit']
    return budget

def budget_report(budget: dict) -> str:
    """Return 'used {used}/{limit} ({pct:.0f}%) remaining {remaining}'.

    Handles `limit == 0` gracefully (reports 0% instead of raising).
    """
    if budget['limit'] > 0:
        pct = (budget['used'] / budget['limit']) * 100
    else:
        pct = 0
    return f'used {budget["used"]}/{budget["limit"]} ({pct:.0f}%) remaining {budget["remaining"]}'
