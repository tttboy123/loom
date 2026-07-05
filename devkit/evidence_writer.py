"""devkit/evidence_writer.py — Loom EvidencePacket writer (Phase D bridge).

Resolves the evidence-path mismatch between:

* Phase B's ``run_loop`` (writes ``devkit/runs/<run-id>/...``),
* Phase C's ``loom://evidence/<run-id>`` resource (reads
  ``devkit/evidence/<run-id>/evidence_packet.json``),
* Phase D's ``gatekeeper.evaluate_final_gate(evidence_dir=...)`` (reads
  ``<evidence_dir>/evidence.json``).

This module is the single writer of ``devkit/evidence/<run-id>/evidence_packet.json``.
Both Phase C (the loom:// reader) and Phase D (the gatekeeper pointed at
``devkit/evidence`` and thus finding ``<evidence_dir>/<run_id>/evidence_packet.json``)
read from the same path. The gatekeeper also has a transparent fallback to
legacy ``<evidence_dir>/evidence.json`` so pre-existing tests keep working.

Public API
----------
* :func:`write_run_evidence` — assemble + atomically write the EvidencePacket.

Failure modes
-------------
Inputs that fail to read are silently coerced to ``None`` (per the "tolerates
missing input files" constraint). Schema validation is performed via the
existing :mod:`devkit.protocol_schemas` machinery; a payload that does not
match the schema raises :class:`jsonschema.ValidationError`. Atomic writes
follow the ``tmp + fsync + os.replace`` pattern from
``devkit/gatekeeper._atomic_write_json``.
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import threading
import uuid
from datetime import datetime, timezone
from typing import Any, Optional

from jsonschema import Draft202012Validator, ValidationError

# ----------------------------------------------------------------------------
# Paths / constants
# ----------------------------------------------------------------------------
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA_PATH = REPO_ROOT / "devkit" / "protocol_schemas" / "evidence_packet.schema.json"
DEFAULT_EVIDENCE_ROOT: pathlib.Path = REPO_ROOT / "devkit" / "evidence"

PROTOCOL_VERSION = "loom.dev/v1"
EVIDENCE_KIND = "EvidencePacket"

# ----------------------------------------------------------------------------
# Logger (module-level so tests + loop can assert log records)
# ----------------------------------------------------------------------------
logger = logging.getLogger("devkit.evidence_writer")
if not logger.handlers:
    handler = logging.NullHandler()
    logger.addHandler(handler)
logger.setLevel(logging.INFO)


# ----------------------------------------------------------------------------
# Schema loader (lazy + cached, thread-safe — same shape as gatekeeper/repairer)
# ----------------------------------------------------------------------------
_schema_lock = threading.Lock()
_cached_validator: Optional[Draft202012Validator] = None
_cached_schema: Optional[dict] = None


def _load_schema() -> dict:
    if not SCHEMA_PATH.exists():
        raise FileNotFoundError(f"evidence_packet schema not found: {SCHEMA_PATH}")
    return json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))


def _get_schema() -> dict:
    global _cached_schema
    with _schema_lock:
        if _cached_schema is None:
            _cached_schema = _load_schema()
        return _cached_schema


def get_validator() -> Draft202012Validator:
    """Return a memoized jsonschema validator for the EvidencePacket schema."""
    global _cached_validator
    with _schema_lock:
        if _cached_validator is None:
            # NOTE: must call _load_schema() directly, not _get_schema(),
            # because _get_schema() also acquires _schema_lock and would
            # deadlock. _load_schema() is the un-locked loader.
            _cached_validator = Draft202012Validator(_load_schema())
        return _cached_validator


def reset_validator_cache() -> None:
    """Clear the memoized validator (tests)."""
    global _cached_validator, _cached_schema
    with _schema_lock:
        _cached_validator = None
        _cached_schema = None


# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _safe_read_text(path: pathlib.Path) -> str | None:
    try:
        if not path.exists() or not path.is_file():
            return None
        return path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return None


def _safe_read_jsonl(path: pathlib.Path, *, limit: int = 500) -> list[dict]:
    """Best-effort JSONL reader — returns dicts; skips bad lines.

    Used to surface the recent event stream into the EvidencePacket. Cap is
    generous (500) so the call is bounded even on long runs; the goal is to
    give Phase D a quick sketch of what happened, not a complete trace.
    """
    text = _safe_read_text(path)
    if not text:
        return []
    rows: list[dict] = []
    for line in text.splitlines():
        line = line.strip()
        if not line or not (line.startswith("{") and line.endswith("}")):
            continue
        try:
            row = json.loads(line)
        except (ValueError, TypeError):
            continue
        if isinstance(row, dict):
            rows.append(row)
        if len(rows) >= limit:
            break
    return rows


def _new_evidence_id(run_id: str, work_item_id: str) -> str:
    """Stable EvidencePacket id — short, sortable, traceable."""
    safe_run = "".join(ch if ch.isalnum() or ch in "-_" else "-" for ch in run_id)[:48]
    safe_wi = "".join(ch if ch.isalnum() or ch in "-_" else "-" for ch in work_item_id)[:48]
    suffix = uuid.uuid4().hex[:8]
    return f"evidence-{safe_run}-{safe_wi}-{suffix}"


def _coerce_optional_float(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


# ----------------------------------------------------------------------------
# Atomic write helpers — same pattern as gatekeeper._atomic_write_json
# ----------------------------------------------------------------------------
def _atomic_write_json(path: pathlib.Path, payload: dict) -> pathlib.Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fh:
        fh.write(json.dumps(payload, ensure_ascii=False, indent=2))
        fh.flush()
        os.fsync(fh.fileno())
    os.replace(tmp, path)
    return path


def validate_evidence_packet(payload: dict) -> None:
    """Convenience wrapper around the schema validator (raises on bad input)."""
    get_validator().validate(payload)


# ----------------------------------------------------------------------------
# Public API
# ----------------------------------------------------------------------------
def _build_summary_string(
    *,
    cost_usd: float | None,
    budget_cap_usd: float | None,
    status_code: str | None,
    gate: str | None,
    tests_passed: int | None = None,
    tests_failed: int | None = None,
) -> str:
    """Build a one-line ``spec.summary`` string.

    The schema requires ``spec.summary`` to be a string; this composes the
    canonical "tests X/Y, cost $A, gate Z" line that the gatekeeper and the
    loom://evidence resource both surface.
    """
    bits: list[str] = []
    if tests_passed is not None or tests_failed is not None:
        passed = int(tests_passed or 0)
        failed = int(tests_failed or 0)
        bits.append(f"tests {passed}/{passed + failed}")
    if cost_usd is not None:
        bits.append(f"cost_usd={float(cost_usd):.4f}")
    if budget_cap_usd is not None:
        bits.append(f"budget_cap_usd={float(budget_cap_usd):.4f}")
    if status_code:
        bits.append(f"status_code={status_code}")
    if gate:
        bits.append(f"gate={gate}")
    return "; ".join(bits) if bits else "no evidence collected"


def _build_metrics(
    *,
    cost_usd: float | None,
    budget_cap_usd: float | None,
    status_code: str | None,
    gate: str | None,
    tests_passed: int | None = None,
    tests_failed: int | None = None,
    gate_inputs: dict | None = None,
    events_summary: dict | None = None,
    run_log_excerpt: str | None = None,
    gate_decision_excerpt: str | None = None,
) -> dict:
    """Build the structured ``spec.metrics`` dict (keeps the string summary
    in :data:`spec.summary` while still preserving the per-field counters as
    an object under ``metrics``, which the existing gatekeeper heuristics can
    also pick up)."""
    metrics: dict[str, Any] = {}
    if cost_usd is not None:
        metrics["cost_usd"] = float(cost_usd)
    if budget_cap_usd is not None:
        metrics["budget_cap_usd"] = float(budget_cap_usd)
    if tests_passed is not None:
        metrics["tests_passed"] = int(tests_passed)
    if tests_failed is not None:
        metrics["tests_failed"] = int(tests_failed)
    if status_code:
        metrics["status_code"] = status_code
    if gate:
        metrics["gate"] = gate
    if isinstance(gate_inputs, dict) and gate_inputs:
        metrics["gate_inputs"] = dict(gate_inputs)
    if isinstance(events_summary, dict) and events_summary:
        metrics["events_summary"] = dict(events_summary)
    if run_log_excerpt:
        metrics["run_log_excerpt"] = run_log_excerpt
    if gate_decision_excerpt:
        metrics["gate_decision_excerpt"] = gate_decision_excerpt
    return metrics


def write_run_evidence(
    run_dir: pathlib.Path | str,
    run_id: str,
    work_item_id: str,
    *,
    status_code: str | None = None,
    gate: str | None = None,
    gate_inputs: dict | None = None,
    cost_usd: float | None = None,
    budget_cap_usd: float | None = None,
    tests_passed: int | None = None,
    tests_failed: int | None = None,
    artifact_manifest: dict | None = None,
    evidence_source: str | None = None,
    events_log_path: pathlib.Path | str | None = None,
    evidence_root: pathlib.Path | str | None = None,
    summary_extras: dict | None = None,
    extra_spec: dict | None = None,
    evidence_id: str | None = None,
) -> pathlib.Path:
    """Assemble + atomically write the per-run ``EvidencePacket``.

    Output path: ``<evidence_root>/<run-id>/evidence_packet.json``.

    Input files (all optional, missing files silently degrade to ``None``/empty):

    * ``<run_dir>/00-task.md`` — task description, surfaced in ``spec.summary``.
    * ``<run_dir>/99-gate.md`` — gate decision excerpt.
    * ``<run_dir>/events.jsonl`` — recent event stream (if
      ``events_log_path`` not given explicitly).

    Caller-supplied scalars (``cost_usd``, ``tests_passed``, ``status_code``,
    etc.) take precedence over what we read from disk — so the loop can pass
    in-memory values it has already collected without round-tripping the
    artifacts.

    Returns the path written.
    """
    run_dir_p = pathlib.Path(run_dir)
    rid = str(run_id or "").strip()
    wid = str(work_item_id or "").strip()
    if not rid:
        raise ValueError("run_id is required")
    if not wid:
        raise ValueError("work_item_id is required")

    root = pathlib.Path(evidence_root) if evidence_root is not None else DEFAULT_EVIDENCE_ROOT
    if not root.is_absolute():
        root = (REPO_ROOT / root).resolve()

    task_text = _safe_read_text(run_dir_p / "00-task.md")
    gate_text = _safe_read_text(run_dir_p / "99-gate.md")

    events_path = pathlib.Path(events_log_path) if events_log_path else (run_dir_p / "events.jsonl")
    events = _safe_read_jsonl(events_path)
    events_summary: dict[str, Any] = {
        "total_events": len(events),
        "last_event_type": events[-1].get("event_type") if events else None,
        "last_failure_code": events[-1].get("failure_code") if events else None,
    }
    if events:
        events_summary["last_timestamp"] = events[-1].get("timestamp")

    run_log_excerpt = None
    if task_text:
        excerpt = task_text.strip().splitlines()
        run_log_excerpt = "\n".join(excerpt[:5])[:600]

    gate_decision_excerpt = None
    if gate_text:
        excerpt = gate_text.strip().splitlines()
        gate_decision_excerpt = "\n".join(excerpt[:8])[:600]

    summary = _build_summary_string(
        cost_usd=_coerce_optional_float(cost_usd),
        budget_cap_usd=_coerce_optional_float(budget_cap_usd),
        status_code=status_code,
        gate=gate,
        tests_passed=tests_passed,
        tests_failed=tests_failed,
    )
    metrics = _build_metrics(
        cost_usd=_coerce_optional_float(cost_usd),
        budget_cap_usd=_coerce_optional_float(budget_cap_usd),
        status_code=status_code,
        gate=gate,
        tests_passed=tests_passed,
        tests_failed=tests_failed,
        gate_inputs=gate_inputs,
        events_summary=events_summary,
        run_log_excerpt=run_log_excerpt,
        gate_decision_excerpt=gate_decision_excerpt,
    )
    if isinstance(summary_extras, dict) and summary_extras:
        for k, v in summary_extras.items():
            if k not in metrics:
                metrics[k] = v

    # spec.source drives Gatekeeper's evidence_source classification; default to
    # "unknown" so callers who don't pass it explicitly still write a schema-
    # compliant packet.
    spec: dict[str, Any] = {"summary": summary}
    if evidence_source:
        spec["source"] = evidence_source
    if isinstance(artifact_manifest, dict) and artifact_manifest:
        spec["artifact_manifest"] = artifact_manifest
    if extra_spec is not None:
        for k, v in extra_spec.items():
            if k not in spec:
                spec[k] = v

    artifacts_list = []
    if isinstance(artifact_manifest, dict):
        artifacts_list.append({"kind": "artifact_manifest", "manifest": artifact_manifest})

    payload: dict[str, Any] = {
        "api_version": PROTOCOL_VERSION,
        "kind": EVIDENCE_KIND,
        "metadata": {
            "id": (evidence_id or _new_evidence_id(rid, wid)).strip(),
            "run_id": rid,
            "work_item_id": wid,
            "created_at": _utc_now_iso(),
            "run_dir": str(run_dir_p),
            "evidence_root": str(root),
        },
        "spec": {
            **spec,
            "artifacts": artifacts_list,
            "verify_commands": [],
            "lineage": {
                "run_id": rid,
                "work_item_id": wid,
                "ts": datetime.now(timezone.utc).isoformat(),
                "writer": "devkit.evidence_writer",
            },
        },
    }
    # Final assembly: ensure summary is the canonical string and a metrics
    # sub-dict carries the per-field counters.
    payload["spec"]["summary"] = summary
    payload["spec"]["metrics"] = metrics
    if isinstance(artifact_manifest, dict) and artifact_manifest:
        payload["spec"]["artifact_manifest"] = artifact_manifest

    # Validate before writing — fail loud on schema mismatch; the gatekeeper
    # expects to read a schema-conformant packet.
    try:
        get_validator().validate(payload)
    except ValidationError as exc:
        logger.warning(
            "write_run_evidence refused: payload failed schema for run_id=%s work_item_id=%s: %s",
            rid,
            wid,
            exc.message,
        )
        raise

    out_dir = root / rid
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / "evidence_packet.json"
    _atomic_write_json(out_path, payload)
    logger.info(
        "write_run_evidence: wrote %s (run_id=%s work_item_id=%s cost=%.5f tests=%s/%s)",
        out_path,
        rid,
        wid,
        float(cost_usd) if cost_usd is not None else 0.0,
        tests_passed,
        tests_failed,
    )
    return out_path


__all__ = [
    # constants
    "PROTOCOL_VERSION",
    "EVIDENCE_KIND",
    "SCHEMA_PATH",
    "DEFAULT_EVIDENCE_ROOT",
    # schema loader
    "get_validator",
    "reset_validator_cache",
    "validate_evidence_packet",
    # API
    "write_run_evidence",
]
