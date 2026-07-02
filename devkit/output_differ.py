# devkit/output_differ.py
"""Pure-stdlib text output differ.

对比两段文本输出的差异：行级 unified diff、变更统计、完全相同判定、摘要。
不依赖任何第三方库；只使用 difflib。
"""
from __future__ import annotations

import difflib
from typing import Dict, List

def line_diff(a: str, b: str) -> List[str]:
    """Return unified-diff lines between ``a`` and ``b``.

    使用 ``difflib.unified_diff``；完全相同时返回 ``[]``。
    """
    a_lines = a.splitlines(keepends=True)
    b_lines = b.splitlines(keepends=True)
    return list(difflib.unified_diff(a_lines, b_lines, fromfile="a", tofile="b"))

def changed_lines(a: str, b: str) -> Dict[str, int]:
    """Count added / removed / unchanged lines in the unified diff.

    跳过 unified diff 的文件头（``---`` / ``+++``）与 hunk 头（``@@``），
    仅按行首字符 ``+`` / ``-`` / 空格 计入统计。
    """
    counts = {"added": 0, "removed": 0, "unchanged": 0}
    for line in line_diff(a, b):
        # 跳过文件头与 hunk 头
        if line.startswith("---") or line.startswith("+++") or line.startswith("@@"):
            continue
        if line.startswith(" "):
            counts["unchanged"] += 1
        elif line.startswith("+"):
            counts["added"] += 1
        elif line.startswith("-"):
            counts["removed"] += 1
    return counts

def is_identical(a: str, b: str) -> bool:
    """Return ``True`` iff ``a`` and ``b`` are exactly equal."""
    return a == b

def diff_summary(a: str, b: str) -> str:
    """Return a one-line summary of the diff.

    完全相同时返回 ``'(identical)'``；
    否则返回 ``'+{added} -{removed} ={unchanged}'``。
    """
    if is_identical(a, b):
        return "(identical)"
    c = changed_lines(a, b)
    return f"+{c['added']} -{c['removed']} ={c['unchanged']}"
