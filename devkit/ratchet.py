# ratchet.py
"""测试棘轮：确保 golden 用例只增不减，绝不弱化。"""

def _count_raises(cases):
    """统计 raises 用例数（列表里含真值 raises 键的用例个数）。"""
    return sum(1 for c in cases if isinstance(c, dict) and c.get("raises"))

def is_weakened(old_cases, new_cases) -> bool:
    """判断 new_cases 相对 old_cases 是否被弱化。

    被弱化当且仅当：用例总数减少，或 raises 用例数减少。
    """
    if len(new_cases) < len(old_cases):
        return True
    if _count_raises(new_cases) < _count_raises(old_cases):
        return True
    return False

def check(old_cases, new_cases) -> dict:
    """返回详细的弱化检查报告。"""
    old_count = len(old_cases)
    new_count = len(new_cases)
    old_raises = _count_raises(old_cases)
    new_raises = _count_raises(new_cases)

    reasons = []
    if new_count < old_count:
        reasons.append("cases dropped")
    if new_raises < old_raises:
        reasons.append("raises dropped")

    return {
        "weakened": bool(reasons),
        "old_count": old_count,
        "new_count": new_count,
        "old_raises": old_raises,
        "new_raises": new_raises,
        "reason": "; ".join(reasons) if reasons else "ok",
    }
