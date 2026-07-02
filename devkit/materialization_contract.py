"""Materialization and apply-result contract helpers.

This module classifies whether an external-project run produced the required
durable outputs. It is intentionally project-agnostic: no downstream repo names,
queue lease state, or verification-surface semantics live here.
"""
from __future__ import annotations

import pathlib
from typing import Iterable, Mapping

APPLY_OUTCOMES = {
    "applied",
    "not_applied",
    "apply_blocked",
    "apply_partial",
    "apply_not_attempted",
}
APPLY_POLICIES = {"full_file", "minimal_patch"}
PATH_STATUSES = {"materialized", "missing", "partial", "blocked"}
BLOCK_REASONS = {"lock", "policy", "partial_write"}


def _safe_output_path(path: str) -> str:
    raw = str(path or "").strip().strip("`").replace("\\", "/")
    if not raw:
        raise ValueError("output path cannot be empty")
    p = pathlib.PurePosixPath(raw)
    if p.is_absolute() or ".." in p.parts or not p.parts or str(p) == ".":
        raise ValueError(f"unsafe output path: {path}")
    return str(p)


def _safe_output_paths(paths: Iterable[str] | None) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for path in paths or []:
        safe = _safe_output_path(path)
        if safe not in seen:
            seen.add(safe)
            out.append(safe)
    return out


def build_contract(
    required_output_paths: Iterable[str] | None = None,
    apply_policy: str = "full_file",
) -> dict:
    """Build a normalized materialization contract."""
    if apply_policy not in APPLY_POLICIES:
        raise ValueError(f"unknown apply_policy: {apply_policy}")
    return {
        "schema_version": 1,
        "required_output_paths": _safe_output_paths(required_output_paths),
        "apply_policy": apply_policy,
    }


def normalize_block_reason(reason: str | None) -> str:
    """Normalize the small block-reason vocabulary used by the contract."""
    raw = str(reason or "policy").strip().lower().replace("-", "_").replace(" ", "_")
    if raw not in BLOCK_REASONS:
        raise ValueError(f"unknown block reason: {reason}")
    return raw


def classify_paths(
    contract: Mapping,
    materialized_paths: Iterable[str] | None = None,
    partial_paths: Iterable[str] | None = None,
    blocked_paths: Iterable[str] | None = None,
    block_reasons: Mapping[str, str] | None = None,
) -> dict:
    """Classify required paths as materialized, missing, partial, or blocked."""
    required = _safe_output_paths(contract.get("required_output_paths", []))
    materialized = set(_safe_output_paths(materialized_paths))
    partial = set(_safe_output_paths(partial_paths))
    blocked = set(_safe_output_paths(blocked_paths))
    raw_reasons = {
        _safe_output_path(path): reason
        for path, reason in dict(block_reasons or {}).items()
    }

    per_path: dict[str, dict] = {}
    missing_required_outputs: list[str] = []
    for path in required:
        item = {"status": "missing"}
        if path in blocked:
            item["status"] = "blocked"
            item["block_reason"] = normalize_block_reason(raw_reasons.get(path))
        elif path in partial:
            item["status"] = "partial"
        elif path in materialized:
            item["status"] = "materialized"
        else:
            missing_required_outputs.append(path)
        per_path[path] = item

    return {
        "required_output_paths": required,
        "per_path": per_path,
        "missing_required_outputs": missing_required_outputs,
    }


def classify_apply_outcome(contract: Mapping, path_results: Mapping, attempted: bool) -> str:
    """Return the apply outcome for a materialization attempt."""
    if not attempted:
        return "apply_not_attempted"

    required = list(contract.get("required_output_paths", []))
    statuses = [
        item.get("status", "missing")
        for item in dict(path_results.get("per_path", {})).values()
    ]
    if any(status == "blocked" for status in statuses):
        return "apply_blocked"
    if required and statuses and all(status == "materialized" for status in statuses):
        return "applied"
    if any(status in {"materialized", "partial"} for status in statuses):
        return "apply_partial"
    return "not_applied"


def closeout_packet(
    run_id: str,
    contract: Mapping,
    path_results: Mapping,
    attempted: bool,
    gate_recommendation: str = "NO-GO",
) -> dict:
    """Build the deterministic closeout packet for this contract slice."""
    apply_outcome = classify_apply_outcome(contract, path_results, attempted=attempted)
    return {
        "schema_version": 1,
        "run_id": str(run_id),
        "apply_policy": contract.get("apply_policy", "full_file"),
        "apply_outcome": apply_outcome,
        "required_output_paths": list(path_results.get("required_output_paths", [])),
        "per_path": dict(path_results.get("per_path", {})),
        "missing_required_outputs": list(path_results.get("missing_required_outputs", [])),
        "gate_recommendation": str(gate_recommendation),
    }
