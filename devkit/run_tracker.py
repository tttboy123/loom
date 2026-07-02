# devkit/run_tracker.py
# 纯标准库实现：追踪多个 run 的状态汇总。
from __future__ import annotations

from typing import Any

def _gate_kind(gate: str) -> str | None:
    """Return 'go', 'no_go', or None based on case-insensitive gate label."""
    if not isinstance(gate, str):
        return None
    upper = gate.strip().upper()
    if upper == "GO":
        return "go"
    if upper == "NO-GO" or upper == "NOGO":
        return "no_go"
    return None

def _is_match(gate: str, target: str) -> bool:
    """Case-insensitive match: 'GO' matches any 'GO...' / '...GO' containing GO.
    'NO-GO' matches labels containing NO-GO (case-insensitive)."""
    if not isinstance(gate, str):
        return False
    upper = gate.strip().upper()
    if target == "GO":
        # Must contain "GO" but NOT "NO-GO" (i.e. not a no-go label)
        if "NO-GO" in upper or "NOGO" in upper:
            return False
        return "GO" in upper
    if target == "NO-GO":
        return "NO-GO" in upper or "NOGO" in upper
    return False

def summarize(runs: list[dict]) -> dict:
    """Summarize a list of run dicts into aggregate stats.

    Each run: {'id': str, 'gate': str, 'tokens': int, 'duration_s': float}
    Returns: {total, go_count, no_go_count, avg_tokens, avg_duration, fastest}
    """
    total = len(runs)
    if total == 0:
        return {
            "total": 0,
            "go_count": 0,
            "no_go_count": 0,
            "avg_tokens": 0.0,
            "avg_duration": 0.0,
            "fastest": None,
        }

    go_count = 0
    no_go_count = 0
    total_tokens = 0
    total_duration = 0.0
    fastest_id: Any = None
    fastest_dur: float = float("inf")

    for r in runs:
        gate = r.get("gate", "")
        kind = _gate_kind(gate)
        if kind == "go":
            go_count += 1
        elif kind == "no_go":
            no_go_count += 1

        tokens = r.get("tokens", 0) or 0
        duration = r.get("duration_s", 0.0) or 0.0

        total_tokens += int(tokens)
        total_duration += float(duration)

        if duration < fastest_dur:
            fastest_dur = float(duration)
            fastest_id = r.get("id")

    return {
        "total": total,
        "go_count": go_count,
        "no_go_count": no_go_count,
        "avg_tokens": total_tokens / total,
        "avg_duration": total_duration / total,
        "fastest": fastest_id,
    }

def filter_runs(runs: list[dict], gate: str) -> list[dict]:
    """Filter runs by gate label (case-insensitive, contains 'GO' or 'NO-GO')."""
    target = gate.strip().upper()
    result = []
    for r in runs:
        g = r.get("gate", "")
        if _is_match(g, target):
            result.append(r)
    return result

def top_runs(runs: list[dict], n: int = 3) -> list[dict]:
    """Return the top-n runs sorted by tokens (descending)."""
    if not runs:
        return []
    sorted_runs = sorted(runs, key=lambda r: r.get("tokens", 0) or 0, reverse=True)
    return sorted_runs[: max(0, int(n))]
