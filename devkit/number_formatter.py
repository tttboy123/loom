"""数字格式化工具 —— 纯标准库实现。

契约（与 tests/test_number_formatter.py 一致）：
- format_int(n, sep=","): 千位分隔符，负数保留前导 "-"
- format_float(n, decimals=2): 固定小数位，含尾随零；使用 banker's rounding (round-half-even)
- to_human(n): >=1e9 -> "X.XB", >=1e6 -> "X.XM", >=1e3 -> "X.XK", 否则 str(n)
- parse_human(s): 解析 K/M/B 后缀（不区分大小写，容忍空白），非法输入抛 ValueError
"""
from __future__ import annotations
from typing import Union

# ---------- format_int ----------
def format_int(n: int, sep: str = ",") -> str:
    """千位分隔符格式化整数。负数保留前导 '-'。"""
    if not isinstance(n, int) or isinstance(n, bool):
        raise TypeError(f"format_int expects int, got {type(n).__name__}")
    sign = "-" if n < 0 else ""
    digits = str(abs(n))
    # 从右往左每 3 位插 sep
    groups = []
    for i in range(len(digits), 0, -3):
        groups.append(digits[max(0, i - 3):i])
    return sign + sep.join(reversed(groups))

# ---------- format_float ----------
def format_float(n: float, decimals: int = 2) -> str:
    """保留指定小数位。rounding=ROUND_HALF_EVEN (banker's rounding)。"""
    if not isinstance(decimals, int) or isinstance(decimals, bool) or decimals < 0:
        raise ValueError("decimals must be a non-negative int")
    # tofixed 内部走 IEEE 754 的 round-half-even
    return f"{n:.{decimals}f}"

# ---------- to_human ----------
def to_human(n: float) -> str:
    """人类可读：K/M/B 后缀，1 位小数。"""
    abs_n = abs(n)
    sign = "-" if n < 0 else ""
    if abs_n >= 1_000_000_000:
        return f"{sign}{abs_n / 1_000_000_000:.1f}B"
    if abs_n >= 1_000_000:
        return f"{sign}{abs_n / 1_000_000:.1f}M"
    if abs_n >= 1_000:
        return f"{sign}{abs_n / 1_000:.1f}K"
    # < 1e3：原样输出（含 int / float 字面量）
    return str(n)

# ---------- parse_human ----------
_SUFFIX_MULT = {"K": 1_000.0, "M": 1_000_000.0, "B": 1_000_000_000.0}

def parse_human(s: str) -> float:
    """解析 "1.5K"/"2.5M"/"2.5B" 或纯数字。非法抛 ValueError。"""
    if not isinstance(s, str):
        raise TypeError("parse_human expects str")
    t = s.strip()
    if not t:
        raise ValueError("empty string")
    # 取出末尾字母作为后缀
    suffix = t[-1].upper() if t[-1].isalpha() else ""
    if suffix and suffix not in _SUFFIX_MULT:
        raise ValueError(f"unsupported suffix: {suffix}")
    num_part = t[:-1] if suffix else t
    try:
        value = float(num_part)
    except ValueError as e:
        raise ValueError(f"invalid number part: {num_part!r}") from e
    return value * _SUFFIX_MULT.get(suffix, 1.0)
