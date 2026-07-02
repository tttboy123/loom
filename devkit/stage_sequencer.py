# devkit/stage_sequencer.py
"""Stage 执行序列管理。纯标准库，无外部依赖。"""

from __future__ import annotations

__all__ = ["sequence", "validate_sequence", "sequence_info"]

def sequence(stages: list[str], skip: list[str] | None = None) -> list[str]:
    """从 stages 中移除 skip 中的项，保序；skip=None 时不跳过。"""
    if skip is None:
        # 防御性拷贝，避免外部可变引用泄漏
        return list(stages)
    skip_set = set(skip)
    return [s for s in stages if s not in skip_set]

def validate_sequence(stages: list[str], allowed: list[str]) -> dict:
    """返回 {valid: bool, unknown: list[str]}；unknown 保留 stages 中首次出现的相对顺序。"""
    allowed_set = set(allowed)
    unknown = [s for s in stages if s not in allowed_set]
    return {
        "valid": len(unknown) == 0,
        "unknown": unknown,
    }

def sequence_info(stages: list[str]) -> dict:
    """返回 {count, has_implement, has_verify, stages}。"""
    return {
        "count": len(stages),
        "has_implement": "implement" in stages,
        "has_verify": "verify" in stages,
        "stages": list(stages),
    }
