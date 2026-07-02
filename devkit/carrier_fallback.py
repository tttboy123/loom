"""Carrier fallback strategy for devkit runtime.

Pure standard-library helpers for building, selecting, parsing, and
evaluating carrier fallback chains. No I/O, no side effects.
"""
from __future__ import annotations

def fallback_chain(primary: str, alternatives: list[str]) -> list[str]:
    """Return ``[primary] + alternatives``, deduplicated, order preserved.

    The primary carrier always comes first; alternatives that duplicate
    the primary or each other are dropped while keeping first-seen order.
    """
    seen: set[str] = set()
    chain: list[str] = []
    for name in [primary, *alternatives]:
        if name not in seen:
            seen.add(name)
            chain.append(name)
    return chain

def select_carrier(chain: list[str], failed: list[str]) -> str:
    """Return the first carrier in ``chain`` not present in ``failed``.

    * If every carrier has failed, the last entry of ``chain`` is returned
      (acts as a final sentinel).
    * If ``chain`` is empty, returns ``''``.
    """
    if not chain:
        return ''
    failed_set = set(failed)
    for name in chain:
        if name not in failed_set:
            return name
    return chain[-1]

def build_fallback(task: dict) -> list[str]:
    """Parse ``task['cascade']`` (comma-separated) into a fallback chain.

    Empty fragments are dropped. Defaults to ``'glm'`` when the field
    is absent.
    """
    raw = task.get('cascade', 'glm')
    return [item for item in raw.split(',') if item]

def should_fallback(result: dict) -> bool:
    """Return ``True`` when ``result['ok']`` is exactly ``False``."""
    return result.get('ok') is False
