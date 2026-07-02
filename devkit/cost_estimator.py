# devkit/cost_estimator.py
"""
Token and cost estimation utilities for devkit.

Functions:
  estimate_tokens(text: str) -> int
  estimate_cost(tokens: int, carrier: str = 'glm') -> float
  summarize(text: str, carrier: str = 'glm') -> dict
"""

def _is_cjk(char: str) -> bool:
    """Check if a single character is CJK (CJK Unified Ideographs block)."""
    code = ord(char)
    return 0x4E00 <= code <= 0x9FFF or 0x3400 <= code <= 0x4DBF

def estimate_tokens(text: str) -> int:
    """Rough token estimation: English -> len//4, Chinese-dominant -> len//2."""
    if not text:
        return 0
    cjk_count = sum(1 for ch in text if _is_cjk(ch))
    total = len(text)
    if total == 0:
        return 0
    # 中文字符 >= 50% 时用 //2，否则 //4
    if cjk_count / total >= 0.5:
        result = total // 2
    else:
        result = total // 4
    # 非空时最小返回 1
    return max(result, 1)

def estimate_cost(tokens: int, carrier: str = 'glm') -> float:
    """Estimate USD cost for given token count under carrier's pricing."""
    if tokens <= 0:
        return 0.0
    rates = {
        'glm': 0.001,
        'deepseek': 0.0005,
        'minimax': 0.002
    }
    rate = rates.get(carrier, 0.001)  # default to glm rate
    return rate * tokens / 1000.0

def summarize(text: str, carrier: str = 'glm') -> dict:
    """Return dict with tokens, cost_usd, and carrier."""
    tokens = estimate_tokens(text)
    cost = estimate_cost(tokens, carrier)
    return {
        'tokens': tokens,
        'cost_usd': cost,
        'carrier': carrier
    }
