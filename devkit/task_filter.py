# devkit/task_filter.py
"""Backlog task filtering and search helpers (pure standard library).

Design notes
------------
* All public functions are pure: input list is not mutated.
* String comparisons are case-insensitive (spec requirement).
* ``filter_multi`` uses AND logic; an empty-string argument for any
  dimension disables filtering on that dimension.
* No third-party deps; safe to import in restricted environments.
"""
from __future__ import annotations

from typing import List, Dict

def by_status(backlog: List[Dict], status: str) -> List[Dict]:
    """Return tasks whose ``status`` field equals ``status`` (case-insensitive)."""
    target = status.lower()
    return [t for t in backlog if str(t.get('status', '')).lower() == target]

def by_priority(backlog: List[Dict], priority: str) -> List[Dict]:
    """Return tasks whose ``priority`` field equals ``priority`` (case-insensitive)."""
    target = priority.lower()
    return [t for t in backlog if str(t.get('priority', '')).lower() == target]

def search(backlog: List[Dict], pattern: str) -> List[Dict]:
    """Return tasks whose ``id`` or ``task`` contains ``pattern`` (case-insensitive)."""
    needle = pattern.lower()
    return [
        t for t in backlog
        if needle in str(t.get('id', '')).lower()
        or needle in str(t.get('task', '')).lower()
    ]

def filter_multi(
    backlog: List[Dict],
    status: str = '',
    priority: str = '',
    pattern: str = '',
) -> List[Dict]:
    """Combine filters with AND logic. Empty string == "skip this dimension"."""
    result = backlog
    if status:
        result = by_status(result, status)
    if priority:
        result = by_priority(result, priority)
    if pattern:
        result = search(result, pattern)
    return result

__all__ = ['by_status', 'by_priority', 'search', 'filter_multi']
