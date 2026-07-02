# devkit/error_classifier.py
"""错误消息分类器（纯标准库）。

按关键词对错误消息进行分类，供上层做 retry / 告警 / 降级决策。
不依赖任何第三方库，O(n) 单遍扫描。
"""
from __future__ import annotations

from collections import Counter
from typing import Optional

# 类别优先级：先匹配到的类别胜出，避免 'auth' 被 'unknown' 等覆盖
_RULES: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("timeout",   ("timeout", "timed out")),
    ("rate_limit", ("rate limit", "429", "quota")),
    ("auth",      ("auth", "401", "unauthorized", "api key")),
    ("code_error", ("import", "module", "syntax")),
    ("assertion", ("assertion", "assert")),
)

def classify(error_msg) -> str:
    """根据关键词把一条错误消息分到预定义类别，未命中返回 'unknown'。"""
    if not isinstance(error_msg, str) or not error_msg:
        return "unknown"
    lower = error_msg.lower()
    for category, keywords in _RULES:
        for kw in keywords:
            if kw in lower:
                return category
    return "unknown"

def batch_classify(errors) -> list[str]:
    """对一批错误消息逐条分类，保持原顺序。"""
    return [classify(e) for e in errors]

def error_stats(errors) -> dict:
    """汇总一批错误的分类统计。

    返回:
        {
            "total": int,          # 输入条数
            "by_type": dict[str, int],  # 每类计数（已分类的类别；unknown 也包含）
            "most_common": str | None,  # 计数最多的类别；并列取先出现；空输入为 None
        }
    """
    categories = batch_classify(errors)
    total = len(categories)
    by_type: dict[str, int] = dict(Counter(categories))

    most_common: Optional[str] = None
    if categories:
        # Counter.most_common 按 (count desc, insertion order) 排序；
        # 我们需要"并列时取先出现"，因此手动按出现顺序扫描。
        seen: dict[str, int] = {}
        for c in categories:
            seen[c] = seen.get(c, 0) + 1
        max_count = max(seen.values())
        for c in categories:
            if seen[c] == max_count:
                most_common = c
                break

    return {"total": total, "by_type": by_type, "most_common": most_common}
