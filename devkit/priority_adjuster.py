"""Dynamic task priority adjuster.

Pure standard library. All exported functions return shallow copies and do not
mutate the input dictionaries or the input list (for sort_by_priority).
"""

from __future__ import annotations

# Priority mapping: name -> numeric level.
_LEVELS: dict[str, int] = {
    "low": 1,
    "medium": 2,
    "high": 3,
    "critical": 4,
}

# Reverse mapping: numeric level -> name. Built from _LEVELS to guarantee
# consistency (no duplicate / missing keys).
_NAMES: dict[int, str] = {v: k for k, v in _LEVELS.items()}

_MIN_LEVEL: int = min(_LEVELS.values())
_MAX_LEVEL: int = max(_LEVELS.values())

def _coerce_level(priority: object) -> int:
    """Look up the numeric level for a priority name.

    Raises:
        KeyError: if ``priority`` is not one of the known priority names.
    """
    if not isinstance(priority, str):
        raise KeyError(priority)
    return _LEVELS[priority]

def _to_name(level: int) -> str:
    """Convert a numeric level back to its priority name.

    Raises:
        KeyError: if ``level`` is not one of {1, 2, 3, 4}.
    """
    return _NAMES[level]

def adjust(task: dict, delta: int) -> dict:
    """Return a shallow copy of ``task`` with its priority shifted by ``delta``.

    The numeric level is clipped to ``[_MIN_LEVEL, _MAX_LEVEL]`` (i.e. [1, 4])
    before being converted back to a priority name.

    The original ``task`` dict is not modified.
    """
    current = _coerce_level(task["priority"])
    new_level = max(_MIN_LEVEL, min(_MAX_LEVEL, current + delta))
    new_task = task.copy()
    new_task["priority"] = _to_name(new_level)
    return new_task

def promote(task: dict) -> dict:
    """Return a shallow copy of ``task`` with its priority raised by 1.

    Saturated at ``critical`` (level ``_MAX_LEVEL``).
    """
    return adjust(task, 1)

def demote(task: dict) -> dict:
    """Return a shallow copy of ``task`` with its priority lowered by 1.

    Saturated at ``low`` (level ``_MIN_LEVEL``).
    """
    return adjust(task, -1)

def sort_by_priority(tasks: list[dict]) -> list[dict]:
    """Return a new list of tasks sorted by priority descending.

    ``critical`` (4) comes first, ``low`` (1) comes last. The sort is stable,
    so ties preserve the original relative order. The input list is not
    mutated.
    """
    return sorted(tasks, key=lambda t: _coerce_level(t["priority"]), reverse=True)
