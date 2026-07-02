"""
token_estimator_v2.py — 改进的 token 估算器（纯标准库）

提供按模型分档的字符→token 估算，以及消息级估算和上下文适配判断。
所有估算均为上界（ceil），便于保守判断是否装得下上下文窗口。
"""
from __future__ import annotations

import math
from typing import Iterable, List, Mapping

# 每字符对应的"分母"：tokens ≈ ceil(len(text) / divisor)
# 数值越小 → 单 token 能容纳的字符越少 → 估算越激进（token 数偏多）
_MODEL_DIVISORS: Mapping[str, float] = {
    'default': 4.0,
    'minimax': 4.0,   # 与 default 走同一档
    'glm': 3.0,
    'deepseek': 4.5,
}

# 每条消息的固定开销（role 标记 + 结构分隔）
_MESSAGE_OVERHEAD = 4

def estimate(text: str, model: str = 'default') -> int:
    """
    估算单段文本的 token 数。

    Args:
        text:  待估算的文本。
        model: 模型档位，支持 'default'/'minimax'/'glm'/'deepseek'；
               未知模型回退到 'default'。

    Returns:
        ceil(len(text) / divisor) 的整数 token 估计。
    """
    divisor = _MODEL_DIVISORS.get(model, _MODEL_DIVISORS['default'])
    # 空串：len=0 → ceil(0/...) = 0，直接走 math.ceil 同样得 0
    return math.ceil(len(text) / divisor)

def estimate_messages(messages: Iterable[Mapping[str, str]],
                      model: str = 'default') -> int:
    """
    估算一组消息的总 token 数。

    每条消息: estimate(content, model) + 固定开销 _MESSAGE_OVERHEAD。

    Args:
        messages: 可迭代的消息序列，每条为 {'role': str, 'content': str} 形态。
                   'content' 缺失时按空串处理。
        model:    透传给 estimate()。

    Returns:
        所有消息 token 之和。空列表返回 0。
    """
    total = 0
    for msg in messages:
        content = msg.get('content', '')  # 缺失按空串，不抛 KeyError
        total += estimate(content, model) + _MESSAGE_OVERHEAD
    return total

def fits_in_context(text: str, max_tokens: int, model: str = 'default') -> bool:
    """
    判断文本能否装入给定 token 上限的上下文窗口。

    Returns:
        estimate(text, model) <= max_tokens 时为 True，否则 False。
    """
    return estimate(text, model) <= max_tokens

__all__ = ['estimate', 'estimate_messages', 'fits_in_context']
