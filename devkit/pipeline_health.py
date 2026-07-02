"""Pipeline health evaluation — pure stdlib."""
from typing import Dict, List

def check(metrics: dict) -> dict:
    """Evaluate pipeline metrics, return health snapshot.

    Args:
        metrics: dict with optional keys ``ok_rate`` (float 0..1),
            ``avg_tokens`` (float), ``error_count`` (int).

    Returns:
        dict with keys ``healthy`` (bool), ``warnings`` (list[str]),
        ``score`` (float == ok_rate, or 0.0 when missing).
    """
    warnings: List[str] = []

    ok_rate = metrics.get("ok_rate")
    avg_tokens = metrics.get("avg_tokens", 0)
    error_count = metrics.get("error_count", 0)

    # ok_rate: missing or below 0.8 triggers warning
    if ok_rate is None or ok_rate < 0.8:
        warnings.append("low ok_rate")

    # avg_tokens: above 10000 triggers warning
    if avg_tokens > 10000:
        warnings.append("high token usage")

    # error_count: above 5 triggers warning
    if error_count > 5:
        warnings.append("high error count")

    score = float(ok_rate) if isinstance(ok_rate, (int, float)) else 0.0

    return {
        "healthy": len(warnings) == 0,
        "warnings": warnings,
        "score": score,
    }

def trend(history: list) -> dict:
    """Evaluate pipeline trend over a sequence of ok_rate samples.

    Args:
        history: list of dicts each containing ``ok_rate`` (float).

    Returns:
        dict with keys ``improving`` (bool), ``avg_ok_rate`` (float),
        ``samples`` (int).
    """
    if not history:
        return {"improving": False, "avg_ok_rate": 0.0, "samples": 0}

    rates = [float(h.get("ok_rate", 0.0)) for h in history]
    improving = rates[-1] >= rates[0]
    avg_ok_rate = sum(rates) / len(rates)

    return {
        "improving": improving,
        "avg_ok_rate": avg_ok_rate,
        "samples": len(history),
    }

def health_report(result: dict) -> str:
    """Format a health-check result into a one-line summary string."""
    return (
        f"healthy={result['healthy']} "
        f"score={result['score']:.2f} "
        f"warnings={len(result['warnings'])}"
    )
