"""
devkit/run_reporter.py
======================

Pure-stdlib helpers for producing / comparing run reports.

Per Constitution §1 (honesty): functions only read what is in the
provided dict and never invent fields. Missing optional fields are
rendered as 'n/a' rather than hallucinated.

Public API
----------
- report_header(run)        -> str   (single line)
- generate_report(run)      -> str   (multi-line)
- compare_reports(runs)     -> dict  (aggregate stats)
"""
from __future__ import annotations

from typing import Any, Dict, Iterable, List

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _get(run: Dict[str, Any], key: str, default: Any = "n/a") -> Any:
    """Dict.get with a sentinel default; never raises KeyError."""
    return run.get(key, default)

# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def report_header(run: Dict[str, Any]) -> str:
    """Return a single-line summary: 'Run {id} | gate={gate} | tokens={tokens}'."""
    run_id = _get(run, "id", "unknown")
    gate = _get(run, "gate", "UNKNOWN")
    tokens = _get(run, "tokens", 0)
    return f"Run {run_id} | gate={gate} | tokens={tokens}"

def generate_report(run: Dict[str, Any]) -> str:
    """Return a multi-line human-readable report for a single run.

    Required fields surfaced (per spec):
        id, gate, tokens
    Optional fields surfaced when present:
        duration_s, stages
    """
    run_id = _get(run, "id", "unknown")
    gate = _get(run, "gate", "UNKNOWN")
    tokens = _get(run, "tokens", 0)
    duration_s = _get(run, "duration_s", "n/a")
    stages: Iterable[Any] = _get(run, "stages", []) or []

    lines: List[str] = []
    lines.append(report_header(run))
    lines.append(f"- duration_s: {duration_s}")
    lines.append(f"- tokens: {tokens}")

    if stages:
        lines.append("- stages:")
        for s in stages:
            lines.append(f"    * {s}")
    else:
        lines.append("- stages: (none)")

    # Trailer that always reminds the operator of the gate decision.
    lines.append(f"- decision: {gate}")
    return "\n".join(lines)

def compare_reports(runs: List[Dict[str, Any]]) -> Dict[str, Any]:
    """Aggregate stats over a list of run dicts.

    Returns
    -------
    dict with keys:
        best_gate  : 'GO' if any run is GO; else 'NO-GO' if any run is NO-GO;
                     else 'UNKNOWN' (also when ``runs`` is empty).
        min_tokens : int  (0 when empty)
        max_tokens : int  (0 when empty)
        count      : int
    """
    count = len(runs)

    if count == 0:
        return {
            "best_gate": "UNKNOWN",
            "min_tokens": 0,
            "max_tokens": 0,
            "count": 0,
        }

    gates = [str(_get(r, "gate", "")) for r in runs]
    tokens = [int(_get(r, "tokens", 0)) for r in runs]

    if "GO" in gates:
        best_gate = "GO"
    elif "NO-GO" in gates:
        best_gate = "NO-GO"
    else:
        best_gate = "UNKNOWN"

    return {
        "best_gate": best_gate,
        "min_tokens": min(tokens),
        "max_tokens": max(tokens),
        "count": count,
    }

__all__ = ["report_header", "generate_report", "compare_reports"]
