"""Scan build artifacts and extract metadata. Stdlib only."""
from __future__ import annotations

import os
import re

_FUNC_PATTERN = re.compile(r"^(?:async )?def (\w+)")


def scan_build(build_dir: str) -> dict:
    """Return {py_files, test_files, other_files, total} for build_dir. Empty if dir missing."""
    result: dict = {"py_files": [], "test_files": [], "other_files": [], "total": 0}
    if not os.path.isdir(build_dir):
        return result
    for entry in os.listdir(build_dir):
        full_path = os.path.join(build_dir, entry)
        if not os.path.isfile(full_path):
            continue
        result["total"] += 1
        if not entry.endswith(".py"):
            result["other_files"].append(entry)
        elif entry.startswith("test_"):
            result["test_files"].append(entry)
        else:
            result["py_files"].append(entry)
    return result


def extract_functions(py_path: str) -> list[str]:
    """Extract top-level function names from a .py file. Returns [] if file missing."""
    if not os.path.isfile(py_path):
        return []
    funcs: list[str] = []
    with open(py_path, encoding="utf-8", errors="replace") as f:
        for line in f:
            m = _FUNC_PATTERN.match(line)
            if m:
                funcs.append(m.group(1))
    return funcs


def scan_summary(build_dir: str) -> str:
    """Return '{N} py files, {M} test files', or '(no build dir)' if missing."""
    if not os.path.isdir(build_dir):
        return "(no build dir)"
    info = scan_build(build_dir)
    return f"{len(info['py_files'])} py files, {len(info['test_files'])} test files"
