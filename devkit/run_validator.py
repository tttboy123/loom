"""run_validator.py — Validate run data integrity (stdlib only).

L1 / report-only: returns validation results; does NOT mutate input.
"""
from typing import Any

_ALLOWED_GATES = {"GO", "NO-GO"}

def _err(msg: str) -> str:
    return msg

def _check_required_fields(run: Any) -> list[str]:
    errors: list[str] = []
    if not isinstance(run, dict):
        return [f"run must be a dict, got {type(run).__name__}"]

    # id: str
    if "id" not in run:
        errors.append("missing required field: id")
    elif not isinstance(run["id"], str) or not run["id"]:
        errors.append("id must be a non-empty string")

    # gate: str (semantic validity checked in check_gate; here only type)
    if "gate" not in run:
        errors.append("missing required field: gate")
    elif not isinstance(run["gate"], str):
        errors.append("gate must be a string")

    # tokens: int >= 0 (bool is subclass of int — reject explicitly)
    if "tokens" not in run:
        errors.append("missing required field: tokens")
    elif isinstance(run["tokens"], bool) or not isinstance(run["tokens"], int):
        errors.append("tokens must be an int")
    elif run["tokens"] < 0:
        errors.append("tokens must be >= 0")

    return errors

def validate_run(run: Any) -> dict:
    """Validate a single run dict.

    Returns: {"valid": bool, "errors": list[str]}
    """
    errors = _check_required_fields(run)
    return {"valid": len(errors) == 0, "errors": errors}

def validate_batch(runs: list[Any]) -> dict:
    """Validate a batch of runs.

    Returns: {"total": int, "valid": int, "invalid": int,
              "errors": [{"run_id": str|"unknown", "errors": list[str]}, ...]}
    Only invalid runs are listed in errors.
    """
    total = len(runs)
    valid_count = 0
    invalid_count = 0
    errors_out: list[dict] = []

    for run in runs:
        errs = _check_required_fields(run)
        if not errs:
            valid_count += 1
        else:
            invalid_count += 1
            run_id = run.get("id") if isinstance(run, dict) else None
            if not isinstance(run_id, str) or not run_id:
                run_id = "unknown"
            errors_out.append({"run_id": run_id, "errors": errs})

    return {
        "total": total,
        "valid": valid_count,
        "invalid": invalid_count,
        "errors": errors_out,
    }

def check_gate(run: Any) -> bool:
    """Return True iff run['gate'] is exactly 'GO' or 'NO-GO'."""
    if not isinstance(run, dict):
        return False
    gate = run.get("gate")
    return isinstance(gate, str) and gate in _ALLOWED_GATES
