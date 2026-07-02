# devkit/metrics_aggregator.py
"""Aggregate metrics across multiple evaluation runs.

Pure standard library; no I/O, no global state.
"""
from __future__ import annotations

_GO = 'GO'
_NO_GO = 'NO-GO'

def _empty_aggregate() -> dict:
    """Canonical empty-result payload (all counters 0, all rates 0.0)."""
    return {
        'total_runs': 0,
        'go_runs': 0,
        'no_go_runs': 0,
        'total_tokens': 0,
        'total_cost_usd': 0.0,
        'avg_tokens': 0.0,
        'avg_iterations': 0.0,
        'go_rate': 0.0,
    }

def aggregate(runs: list[dict]) -> dict:
    """Aggregate metrics across multiple runs.

    Each run is expected to contain:
        {gate: str, tokens: int, cost_usd: float, iterations: int}

    Returns a dict with total_runs, go_runs, no_go_runs, total_tokens,
    total_cost_usd, avg_tokens, avg_iterations, go_rate. Empty input
    yields all counters at 0 and all rates at 0.0.
    """
    if not runs:
        return _empty_aggregate()

    total_runs = len(runs)
    go_runs = sum(1 for r in runs if r.get('gate') == _GO)
    no_go_runs = sum(1 for r in runs if r.get('gate') == _NO_GO)
    total_tokens = sum(int(r.get('tokens', 0) or 0) for r in runs)
    total_cost_usd = sum(float(r.get('cost_usd', 0.0) or 0.0) for r in runs)
    total_iterations = sum(int(r.get('iterations', 0) or 0) for r in runs)

    return {
        'total_runs': total_runs,
        'go_runs': go_runs,
        'no_go_runs': no_go_runs,
        'total_tokens': total_tokens,
        'total_cost_usd': total_cost_usd,
        'avg_tokens': total_tokens / total_runs,
        'avg_iterations': total_iterations / total_runs,
        'go_rate': go_runs / total_runs,
    }

def gate_distribution(runs: list[dict]) -> dict:
    """Return a frequency dict mapping gate value -> count.

    Runs without a `gate` key are ignored. Insertion order is preserved
    (first-seen gate wins order).
    """
    dist: dict = {}
    for r in runs:
        gate = r.get('gate')
        if gate is None:
            continue
        dist[gate] = dist.get(gate, 0) + 1
    return dist

def top_token_runs(runs: list[dict], n: int = 3) -> list[dict]:
    """Return the top-n runs ordered by `tokens` descending.

    Uses a stable sort; ties preserve input order. If n >= len(runs),
    returns all runs sorted. n <= 0 returns [].
    """
    if n <= 0 or not runs:
        return []
    return sorted(runs, key=lambda r: r.get('tokens', 0), reverse=True)[:n]
