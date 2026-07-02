# devkit/run_log_parser.py
"""Parse run-log.md into structured data.

Pure standard-library parser for devkit run logs.
"""
from __future__ import annotations

import os
import re
from typing import Dict, List

__all__ = ["parse", "parse_file", "batch_parse"]

# 预编译正则：与规格逐字对齐
_GATE_RE = re.compile(r"Gate[：:]\s*(.+)")
_TOKENS_RE = re.compile(r"(\d+)\s*tok")
_COST_RE = re.compile(r"\$([\d.]+)")
_ITERATIONS_RE = re.compile(r"迭代[：:]?\s*(\d+)\s*轮")
_RUN_ID_RE = re.compile(r"run=([\w-]+)")

def parse(log_text: str) -> Dict[str, object]:
    """Extract {gate, tokens, cost_usd, iterations, run_id} from run-log text.

    Missing values default to '' (strings) or 0 / 0.0 (numbers).
    """
    result: Dict[str, object] = {
        "gate": "",
        "tokens": 0,
        "cost_usd": 0.0,
        "iterations": 0,
        "run_id": "",
    }

    if not log_text:
        return result

    m = _GATE_RE.search(log_text)
    if m:
        result["gate"] = m.group(1).strip()

    m = _TOKENS_RE.search(log_text)
    if m:
        result["tokens"] = int(m.group(1))

    m = _COST_RE.search(log_text)
    if m:
        result["cost_usd"] = float(m.group(1))

    m = _ITERATIONS_RE.search(log_text)
    if m:
        result["iterations"] = int(m.group(1))

    m = _RUN_ID_RE.search(log_text)
    if m:
        result["run_id"] = m.group(1)

    return result

def parse_file(path: str) -> Dict[str, object]:
    """Read ``path`` and parse it; return ``parse('')`` if missing/unreadable."""
    if not os.path.exists(path):
        return parse("")
    try:
        with open(path, "r", encoding="utf-8") as f:
            return parse(f.read())
    except (IOError, OSError):
        return parse("")

def batch_parse(run_dirs: List[str]) -> List[Dict[str, object]]:
    """Parse ``<dir>/run-log.md`` for each dir; returns a list of dicts."""
    return [parse_file(os.path.join(d, "run-log.md")) for d in run_dirs]
