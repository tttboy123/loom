# devkit/task_classifier.py
"""Task classifier using keyword matching (pure standard library)."""

def classify(task_text: str) -> str:
    """Classify a single task description."""
    text_lower = task_text.lower()

    if 'implement' in text_lower or '实现' in text_lower:
        return 'implementation'
    if 'test' in text_lower or '测试' in text_lower:
        return 'testing'
    if 'fix' in text_lower or 'bug' in text_lower or '修复' in text_lower:
        return 'bugfix'
    if 'refactor' in text_lower or '重构' in text_lower:
        return 'refactor'

    return 'other'

def batch_classify(tasks: list[str]) -> list[str]:
    """Classify a list of tasks."""
    return [classify(task) for task in tasks]

def classification_stats(classes: list[str]) -> dict:
    """Compute statistics from a list of classifications."""
    if not classes:
        return {'total': 0, 'by_type': {}, 'most_common': None}

    total = len(classes)
    by_type: dict[str, int] = {}
    for c in classes:
        by_type[c] = by_type.get(c, 0) + 1

    max_count = max(by_type.values())
    candidates = sorted([t for t, cnt in by_type.items() if cnt == max_count])
    most_common: str | None = candidates[0] if candidates else None

    return {'total': total, 'by_type': by_type, 'most_common': most_common}
