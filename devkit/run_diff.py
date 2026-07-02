# devkit/run_diff.py
"""Compare two Loom run directories (gate diff, md file diff). Standard library only."""
from __future__ import annotations

import os
import re

_GATE_RE = re.compile(r"Gate[:：]\s*(.+)", re.IGNORECASE)


def load_gate(run_dir: str) -> str:
    """Extract gate string from run_dir/run-log.md; '' if missing."""
    log_path = os.path.join(run_dir, "run-log.md")
    if not os.path.isfile(log_path):
        return ""
    try:
        content = open(log_path, encoding="utf-8").read()
    except OSError:
        return ""
    m = _GATE_RE.search(content)
    return m.group(1).strip() if m else ""


def _md_files(run_dir: str) -> set[str]:
    if not os.path.isdir(run_dir):
        return set()
    try:
        return {f for f in os.listdir(run_dir) if f.endswith(".md")}
    except OSError:
        return set()


def diff_runs(run_a: str, run_b: str) -> dict:
    """Compare two run directories.

    Returns:
        {run_a, run_b, gate_a, gate_b, gate_changed: bool,
         files_only_in_a: list, files_only_in_b: list}
    """
    gate_a = load_gate(run_a)
    gate_b = load_gate(run_b)
    files_a = _md_files(run_a)
    files_b = _md_files(run_b)
    return {
        "run_a": run_a,
        "run_b": run_b,
        "gate_a": gate_a,
        "gate_b": gate_b,
        "gate_changed": gate_a != gate_b,
        "files_only_in_a": sorted(files_a - files_b),
        "files_only_in_b": sorted(files_b - files_a),
    }


def diff_summary(diff: dict) -> str:
    """One-line summary of a diff dict; '(no diff data)' for empty input."""
    if not diff:
        return "(no diff data)"
    a = diff.get("run_a", "?")
    b = diff.get("run_b", "?")
    ga = diff.get("gate_a", "")
    gb = diff.get("gate_b", "")
    only_b = diff.get("files_only_in_b", [])
    only_a = diff.get("files_only_in_a", [])
    gate_part = f"gate {ga}→{gb}" if ga != gb else f"gate {ga} (unchanged)"
    extra = []
    if only_b:
        extra.append(f"+{len(only_b)} files in b")
    if only_a:
        extra.append(f"+{len(only_a)} files in a")
    suffix = ", " + ", ".join(extra) if extra else ""
    return f"{a} vs {b}: {gate_part}{suffix}"
