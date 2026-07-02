"""
devkit/carrier_analyzer.py

Pure-stdlib analyzer for carrier usage patterns.

Public API:
    analyze(runs)              -> dict
    carrier_report(analysis)   -> str
    compare_carriers(runs, a, b) -> dict
"""

from __future__ import annotations

from collections import defaultdict
from typing import Dict, List, Optional

def analyze(runs: list[dict]) -> dict:
    """Analyze carrier usage patterns from a list of run records.

    Args:
        runs: list of dicts, each shaped like
              {'carrier': str, 'tokens': int, 'ok': bool}

    Returns:
        dict with keys:
            carriers    - list[str], unique carriers in first-seen order
            usage       - dict[carrier -> int count]
            ok_rates    - dict[carrier -> float in [0, 1]]
            top_carrier - str | None, the carrier with the highest count
                         (ties broken by first-seen order; None if empty)
    """
    usage_counts: Dict[str, int] = defaultdict(int)
    ok_counts: Dict[str, int] = defaultdict(int)
    seen: List[str] = []

    for run in runs:
        carrier = run["carrier"]
        if carrier not in usage_counts:
            seen.append(carrier)
        usage_counts[carrier] += 1
        if run["ok"]:
            ok_counts[carrier] += 1

    carriers = list(seen)
    usage = dict(usage_counts)
    ok_rates = {
        c: (ok_counts[c] / usage_counts[c]) if usage_counts[c] else 0.0
        for c in carriers
    }

    top_carrier: Optional[str] = None
    if carriers:
        # max over (count, -first_seen_index) so ties keep first-seen carrier
        first_idx = {c: i for i, c in enumerate(carriers)}
        top_carrier = max(
            carriers,
            key=lambda c: (usage_counts[c], -first_idx[c]),
        )

    return {
        "carriers": carriers,
        "usage": usage,
        "ok_rates": ok_rates,
        "top_carrier": top_carrier,
    }

def carrier_report(analysis: dict) -> str:
    """Render an analysis dict as a multi-line human-readable report.

    Each line: '{carrier}: {count} uses, {ok_rate:.0%} ok rate'
    Returns '(no data)' when there are no carriers.
    """
    carriers = analysis.get("carriers", [])
    usage = analysis.get("usage", {})
    ok_rates = analysis.get("ok_rates", {})

    if not carriers:
        return "(no data)"

    lines = []
    for carrier in carriers:
        count = usage.get(carrier, 0)
        rate = ok_rates.get(carrier, 0.0)
        lines.append(f"{carrier}: {count} uses, {rate:.0%} ok rate")
    return "\n".join(lines)

def compare_carriers(runs: list[dict], a: str, b: str) -> dict:
    """Compare two specific carriers over a run list.

    Returns:
        {
            a: {'count': int, 'ok_rate': float},
            b: {'count': int, 'ok_rate': float},
            winner: str | None   # higher ok_rate wins; tie -> None
        }
    """
    a_count = 0
    a_ok = 0
    b_count = 0
    b_ok = 0

    for run in runs:
        carrier = run["carrier"]
        if carrier == a:
            a_count += 1
            if run["ok"]:
                a_ok += 1
        elif carrier == b:
            b_count += 1
            if run["ok"]:
                b_ok += 1

    a_rate = (a_ok / a_count) if a_count else 0.0
    b_rate = (b_ok / b_count) if b_count else 0.0

    if a_rate > b_rate:
        winner: Optional[str] = a
    elif b_rate > a_rate:
        winner = b
    else:
        winner = None

    return {
        a: {"count": a_count, "ok_rate": a_rate},
        b: {"count": b_count, "ok_rate": b_rate},
        "winner": winner,
    }
