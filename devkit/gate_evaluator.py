"""Evaluate run gate decisions. Stdlib only."""
from __future__ import annotations


def parse_gate(gate_str: str) -> dict:
    """Return {decision, confidence, raw} from gate string."""
    raw = gate_str if isinstance(gate_str, str) else str(gate_str)
    upper = raw.upper()

    if "NO-GO" in upper or "NO_GO" in upper or "NOGO" in upper:
        decision = "NO-GO"
    elif "GO" in upper:
        decision = "GO"
    else:
        decision = "UNKNOWN"

    lowered = raw.lower()
    confidence = "high" if ("建议" in raw or "suggest" in lowered) else "low"
    return {"decision": decision, "confidence": confidence, "raw": raw}


def evaluate(run_data: dict) -> dict:
    """Return {go, iterations, tokens, efficiency} for a single run."""
    gate = run_data.get("gate", "")
    iterations = run_data.get("iterations", 0)
    tokens = run_data.get("tokens", 0)
    go = parse_gate(gate)["decision"] == "GO"
    efficiency = 1.0 / (iterations + 1) if iterations >= 0 else 1.0
    return {"go": go, "iterations": iterations, "tokens": tokens, "efficiency": efficiency}


def batch_evaluate(runs: list) -> dict:
    """Return {total, go_count, avg_efficiency, best_run} for a batch of runs."""
    total = len(runs)
    if total == 0:
        return {"total": 0, "go_count": 0, "avg_efficiency": 0.0, "best_run": None}

    evaluated = [evaluate(r) for r in runs]
    go_count = sum(1 for r in evaluated if r["go"])
    avg_efficiency = sum(r["efficiency"] for r in evaluated) / total

    best = None
    for ev in evaluated:
        if best is None:
            best = ev
        elif ev["go"] and not best["go"]:
            best = ev
        elif ev["go"] == best["go"] and ev["efficiency"] > best["efficiency"]:
            best = ev

    return {"total": total, "go_count": go_count, "avg_efficiency": avg_efficiency, "best_run": best}
