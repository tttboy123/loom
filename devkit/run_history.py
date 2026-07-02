# devkit/run_history.py
"""Pure-stdlib run execution history maintenance."""

from __future__ import annotations

from typing import Any

def create() -> dict[str, Any]:
    """Create a fresh empty history.

    Returns:
        A new history dict shaped as ``{"runs": [], "total": 0}``.
    """
    return {"runs": [], "total": 0}

def append(history: dict[str, Any], run: dict[str, Any]) -> dict[str, Any]:
    """Return a new history with ``run`` appended.

    The input ``history`` is not mutated.

    Args:
        history: Existing history dict (as produced by :func:`create`).
        run: Run record to append.

    Returns:
        New history dict with the run appended and ``total`` incremented.
    """
    new_runs = list(history.get("runs", [])) + [run]
    return {"runs": new_runs, "total": len(new_runs)}

def last_n(history: dict[str, Any], n: int) -> list[dict[str, Any]]:
    """Return the most recent ``n`` runs.

    Args:
        history: Existing history dict.
        n: Number of runs to return. Non-positive values return an empty list.

    Returns:
        List of the last ``n`` runs (preserving order). Returns ``[]`` when
        ``n <= 0``.
    """
    if n <= 0:
        return []
    runs = history.get("runs", [])
    return runs[-n:]

def history_stats(history: dict[str, Any]) -> dict[str, Any]:
    """Compute aggregate statistics for a history.

    Args:
        history: Existing history dict.

    Returns:
        Dict shaped as::

            {
                "total": int,
                "go_count": int,
                "nogo_count": int,
                "last_gate": str | None,
            }

        ``last_gate`` is the ``gate`` of the final run, or ``None`` when no
        runs exist or the final run has no ``gate`` key.
    """
    runs = history.get("runs", [])
    go_count = sum(1 for r in runs if r.get("gate") == "GO")
    nogo_count = sum(1 for r in runs if r.get("gate") == "NOGO")
    last_gate: str | None = None
    if runs:
        last_gate = runs[-1].get("gate")
    return {
        "total": len(runs),
        "go_count": go_count,
        "nogo_count": nogo_count,
        "last_gate": last_gate,
    }
