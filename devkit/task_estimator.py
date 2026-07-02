"""任务复杂度与资源估算器（纯标准库）。

约定：
- score = len(task_text) // 200，上限 10
- level: score<=3 -> 'low', <=6 -> 'medium', else -> 'high'
- factors: 含 'long_task' 当 len > 500；含 'has_golden' 当字符串中含 'Golden'
- estimate_tokens: len//2 + 500，carrier=='glm' 时再 * 1.2（取 int）
- batch_estimate: 返回 [{'id', 'complexity'}, ...]
"""

from typing import Dict, List, Any, Iterable

__all__ = [
    'estimate_complexity',
    'estimate_tokens',
    'batch_estimate',
]

_SCORE_CAP = 10
_GLM_MULTIPLIER = 1.2
_TOKEN_BASE = 500

def _complexity_level(score: int) -> str:
    if score <= 3:
        return 'low'
    if score <= 6:
        return 'medium'
    return 'high'

def estimate_complexity(task_text: str) -> Dict[str, Any]:
    """估算任务复杂度。

    Returns:
        {'score': int(0..10), 'level': 'low'|'medium'|'high', 'factors': list[str]}
    """
    text = task_text or ''
    n = len(text)
    raw_score = n // 200
    score = raw_score if raw_score < _SCORE_CAP else _SCORE_CAP

    factors: List[str] = []
    if n > 500:
        factors.append('long_task')
    if 'Golden' in text:
        factors.append('has_golden')

    return {
        'score': score,
        'level': _complexity_level(score),
        'factors': factors,
    }

def estimate_tokens(task_text: str, carrier: str = 'minimax') -> int:
    """估算 implement 阶段 tokens。

    base = len(task_text) // 2 + 500
    carrier == 'glm' 时 base *= 1.2（取整）
    """
    text = task_text or ''
    base = len(text) // 2 + _TOKEN_BASE
    if carrier == 'glm':
        base = int(base * _GLM_MULTIPLIER)
    return base

def batch_estimate(tasks: Iterable[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """对每个含 'task' 字段的 dict 调用 estimate_complexity。

    契约：返回列表，长度与输入相同，每项为 {'id': <原 id 或 None>, 'complexity': ...}
    若缺 'task' 字段，则 'complexity' 为 None（行为可被测试预期锁定）。
    """
    out: List[Dict[str, Any]] = []
    for t in tasks or []:
        task_text = t.get('task')
        if isinstance(task_text, str):
            complexity = estimate_complexity(task_text)
        else:
            complexity = None
        out.append({'id': t.get('id'), 'complexity': complexity})
    return out
