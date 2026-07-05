"""devkit/gatekeeper.py — Phase D Loom Gatekeeper (evidence source classifier + final gate).

The Gatekeeper is the single source of truth for *final state* in the new Loom
architecture. It removes ad-hoc string-grep gate logic from rdloop / iterate
and replaces it with a typed, schema-validated verdict object.

Responsibilities
----------------
1. Classify an ``EvidencePacket`` by where its evidence was actually
   produced (``inner_sandbox`` / ``materialized_repo`` / ``external_signal``
   / ``unknown``). This drives how trustworthy the verdict is.
2. Evaluate the *final gate* for a single run: was evidence written? did
   tests regress? did cost blow past the budget cap? is the artifact
   manifest intact?
3. Persist the resulting :class:`GateVerdict` to disk atomically and reload
   it later (the runner reads from this file, not from run-log strings).

This module is **stdlib + jsonschema only** (same constraint as
``devkit.repairer``). It does not touch the backlog, run-loop, or
reflection prompt — it just produces the verdict.

Public API
----------
* :class:`GateVerdict` — dataclass mirroring the schema.
* :func:`classify_evidence_source` — map an evidence_packet → enum.
* :func:`evaluate_final_gate` — read evidence.json, produce a verdict.
* :func:`write_verdict` — atomic JSON write.
* :func:`load_verdict` — load + validate a stored verdict.
* :func:`validate_verdict` — jsonschema check (used internally; exposed for tests).

Failure codes
-------------
Each verdict carries a ``failure_codes`` list drawn from the
:class:`GateVerdict` schema. Currently the Gatekeeper can emit:

* ``EVIDENCE_MISSING`` — ``evidence.json`` was not present at
  ``devkit/runs/<run_id>/evidence.json``.
* ``TEST_REGRESSION`` — at least one stage reported a failing test
  (or ``tests_failed`` > 0 in the evidence summary).
* ``BUDGET_EXCEEDED`` — ``cost_usd`` exceeded ``budget_cap_usd``.
* ``EVIDENCE_INVALID`` — ``artifact_manifest.source`` missing/invalid
  *or* the evidence packet itself failed schema validation when supplied.
* ``SCHEMA_VALIDATION_ERROR`` — verdict could not be written because the
  payload did not match ``gate_verdict.schema.json``.

The verdict is ``passed=True`` only when ``failure_codes`` is empty.
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import threading
import time
import uuid
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import Any, Iterable, Optional

from jsonschema import Draft202012Validator, ValidationError

from devkit import budget as _budget

# ----------------------------------------------------------------------------
# Paths
# ----------------------------------------------------------------------------
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA_PATH = REPO_ROOT / "devkit" / "protocol_schemas" / "gate_verdict.schema.json"
DEFAULT_RUNS_DIR = REPO_ROOT / "devkit" / "runs"

PROTOCOL_VERSION = "loom.dev/v1"
GATEKEEPER_KIND = "GateVerdict"

# Required at construction; kept as module constants so tests and other
# modules can import the canonical names instead of magic strings.
EVIDENCE_INNER_SANDBOX = "inner_sandbox"
EVIDENCE_MATERIALIZED_REPO = "materialized_repo"
EVIDENCE_EXTERNAL_SIGNAL = "external_signal"
EVIDENCE_UNKNOWN = "unknown"

EVIDENCE_SOURCES: tuple[str, ...] = (
    EVIDENCE_INNER_SANDBOX,
    EVIDENCE_MATERIALIZED_REPO,
    EVIDENCE_EXTERNAL_SIGNAL,
    EVIDENCE_UNKNOWN,
)

# Failure codes — kept in sync with the schema's ``failure_codes.items.enum``.
FC_EVIDENCE_MISSING = "EVIDENCE_MISSING"
FC_TEST_REGRESSION = "TEST_REGRESSION"
FC_BUDGET_EXCEEDED = "BUDGET_EXCEEDED"
FC_EVIDENCE_INVALID = "EVIDENCE_INVALID"
FC_SCHEMA_VALIDATION_ERROR = "SCHEMA_VALIDATION_ERROR"

FAILURE_CODES: tuple[str, ...] = (
    FC_EVIDENCE_MISSING,
    FC_TEST_REGRESSION,
    FC_BUDGET_EXCEEDED,
    FC_EVIDENCE_INVALID,
    FC_SCHEMA_VALIDATION_ERROR,
)

# Aliases the evidence_packet / artifact_manifest schemas already emit.
# We accept them so the gatekeeper does not have to fork the taxonomy.
_EVIDENCE_PACKET_SOURCE_ALIASES: dict[str, str] = {
    # direct matches
    EVIDENCE_INNER_SANDBOX: EVIDENCE_INNER_SANDBOX,
    EVIDENCE_MATERIALIZED_REPO: EVIDENCE_MATERIALIZED_REPO,
    EVIDENCE_EXTERNAL_SIGNAL: EVIDENCE_EXTERNAL_SIGNAL,
    EVIDENCE_UNKNOWN: EVIDENCE_UNKNOWN,
    # legacy / sister-schema aliases
    "runtime_support": EVIDENCE_INNER_SANDBOX,
    "declared": EVIDENCE_MATERIALIZED_REPO,
    "loom_runtime": EVIDENCE_INNER_SANDBOX,
    "external": EVIDENCE_EXTERNAL_SIGNAL,
}

DEFAULT_BUDGET_CAP_USD: float = _budget.DEFAULT_COST_LIMIT_USD

# ----------------------------------------------------------------------------
# Logger (module-level so tests can assert log records)
# ----------------------------------------------------------------------------
logger = logging.getLogger("devkit.gatekeeper")
if not logger.handlers:
    handler = logging.NullHandler()
    logger.addHandler(handler)
logger.setLevel(logging.INFO)


# ----------------------------------------------------------------------------
# Schema loader (lazy + cached, thread-safe — same shape as repairer)
# ----------------------------------------------------------------------------
_schema_lock = threading.Lock()
_cached_validator: Optional[Draft202012Validator] = None
_cached_schema: Optional[dict] = None


def _load_schema() -> dict:
    if not SCHEMA_PATH.exists():
        raise FileNotFoundError(f"gate_verdict schema not found: {SCHEMA_PATH}")
    return json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))


def _get_schema() -> dict:
    global _cached_schema
    with _schema_lock:
        if _cached_schema is None:
            _cached_schema = _load_schema()
        return _cached_schema


def get_validator() -> Draft202012Validator:
    """Return a memoized jsonschema validator for the GateVerdict schema."""
    global _cached_validator
    with _schema_lock:
        if _cached_validator is None:
            # NOTE: must call _load_schema() directly, not _get_schema(),
            # because _get_schema() also acquires _schema_lock and would
            # deadlock. _load_schema() is the un-locked loader.
            _cached_validator = Draft202012Validator(_load_schema())
        return _cached_validator


def reset_validator_cache() -> None:
    """Clear the memoized validator (used by tests when they patch the schema)."""
    global _cached_validator, _cached_schema
    with _schema_lock:
        _cached_validator = None
        _cached_schema = None


# ----------------------------------------------------------------------------
# Internal helpers
# ----------------------------------------------------------------------------
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _ensure_str(value: Any, field_name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def _ensure_path(value: Any, field_name: str) -> pathlib.Path:
    if value is None:
        raise ValueError(f"{field_name} is required (path-like)")
    return pathlib.Path(value)


def _coerce_failure_codes(values: Iterable[Any]) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for value in values or []:
        text = str(value or "").strip()
        if not text:
            continue
        if text not in FAILURE_CODES:
            raise ValueError(
                f"unknown failure code {text!r}; allowed: {sorted(FAILURE_CODES)}"
            )
        if text in seen:
            continue
        seen.add(text)
        out.append(text)
    return out


def _new_verdict_id(run_id: str, work_item_id: str) -> str:
    """Stable verdict id — short, sortable, traceable."""
    safe_run = "".join(ch if ch.isalnum() or ch in "-_" else "-" for ch in run_id)[:48]
    safe_wi = "".join(ch if ch.isalnum() or ch in "-_" else "-" for ch in work_item_id)[:48]
    suffix = uuid.uuid4().hex[:8]
    return f"verdict-{safe_run}-{safe_wi}-{suffix}"


# ----------------------------------------------------------------------------
# GateVerdict — typed mirror of the schema.
# ----------------------------------------------------------------------------
@dataclass
class GateVerdict:
    """A Gatekeeper decision. Always carry one through the Runner boundary.

    Fields mirror ``gate_verdict.schema.json``. :meth:`to_dict` is what gets
    written to disk; :meth:`from_dict` validates + hydrates from JSON.
    """

    api_version: str = PROTOCOL_VERSION
    kind: str = GATEKEEPER_KIND
    metadata: dict = field(default_factory=dict)
    spec: dict = field(default_factory=dict)

    # --- convenience accessors (don't go on disk) -----------------------
    @property
    def verdict_id(self) -> str:
        return str(self.metadata.get("id", ""))

    @property
    def run_id(self) -> str:
        return str(self.metadata.get("run_id", ""))

    @property
    def work_item_id(self) -> str:
        return str(self.metadata.get("work_item_id", ""))

    @property
    def timestamp(self) -> str:
        return str(self.metadata.get("timestamp", ""))

    @property
    def evidence_source(self) -> str:
        return str(self.spec.get("evidence_source", EVIDENCE_UNKNOWN))

    @property
    def passed(self) -> bool:
        return bool(self.spec.get("passed", False))

    @property
    def reason(self) -> str:
        return str(self.spec.get("reason", ""))

    @property
    def failure_codes(self) -> list[str]:
        raw = self.spec.get("failure_codes", [])
        if not isinstance(raw, list):
            return []
        return [str(x) for x in raw if str(x).strip()]

    # --- core: serialize / validate -------------------------------------
    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        """Raise jsonschema.ValidationError if this verdict violates the schema."""
        get_validator().validate(self.to_dict())

    # --- core: hydrate --------------------------------------------------
    @classmethod
    def from_dict(cls, payload: dict) -> "GateVerdict":
        if not isinstance(payload, dict):
            raise ValueError(f"verdict payload must be a dict, got {type(payload).__name__}")
        return cls(
            api_version=str(payload.get("api_version", PROTOCOL_VERSION)),
            kind=str(payload.get("kind", GATEKEEPER_KIND)),
            metadata=dict(payload.get("metadata", {}) or {}),
            spec=dict(payload.get("spec", {}) or {}),
        )

    # --- convenience constructors --------------------------------------
    @classmethod
    def build(
        cls,
        *,
        run_id: str,
        work_item_id: str,
        evidence_source: str,
        passed: bool,
        reason: str,
        failure_codes: list[str] | tuple[str, ...] = (),
        checks: dict | None = None,
        lineage: dict | None = None,
        timestamp: str | None = None,
        schema_version: str | None = None,
        verdict_id: str | None = None,
    ) -> "GateVerdict":
        """Construct a verdict with required-field validation."""
        rid = _ensure_str(run_id, "run_id")
        wid = _ensure_str(work_item_id, "work_item_id")
        src = _ensure_str(evidence_source, "evidence_source")
        if src not in EVIDENCE_SOURCES:
            raise ValueError(
                f"evidence_source must be one of {list(EVIDENCE_SOURCES)}, got {src!r}"
            )
        rsn = str(reason or "").strip() or ("all_gates_passed" if passed else "unknown")
        codes = _coerce_failure_codes(failure_codes)
        if passed and codes:
            # passed=True requires empty failure_codes (single source of truth)
            raise ValueError(
                f"passed=True requires empty failure_codes; got {codes!r}"
            )
        if (not passed) and (not codes):
            # every failure must carry at least one code so consumers can branch
            raise ValueError(
                "passed=False requires at least one entry in failure_codes"
            )
        ts = (timestamp or _utc_now_iso()).strip()
        meta: dict[str, Any] = {
            "id": (verdict_id or _new_verdict_id(rid, wid)).strip(),
            "run_id": rid,
            "work_item_id": wid,
            "timestamp": ts,
        }
        if schema_version:
            meta["schema_version"] = str(schema_version).strip()

        spec: dict[str, Any] = {
            "evidence_source": src,
            "passed": bool(passed),
            "reason": rsn,
            "failure_codes": codes,
        }
        if isinstance(checks, dict) and checks:
            spec["checks"] = dict(checks)
        if isinstance(lineage, dict) and lineage:
            spec["lineage"] = dict(lineage)
        return cls(metadata=meta, spec=spec)

    # --- dunder --------------------------------------------------------
    def __post_init__(self) -> None:
        # Defensive normalization so callers can't sneak None / wrong types
        # into the spec dict and surprise downstream consumers.
        if not isinstance(self.metadata, dict):
            self.metadata = {}
        if not isinstance(self.spec, dict):
            self.spec = {}
        self.metadata.setdefault("id", "")
        self.metadata.setdefault("run_id", "")
        self.metadata.setdefault("work_item_id", "")
        self.metadata.setdefault("timestamp", _utc_now_iso())
        self.spec.setdefault("evidence_source", EVIDENCE_UNKNOWN)
        self.spec.setdefault("passed", False)
        self.spec.setdefault("reason", "")
        if "failure_codes" not in self.spec or self.spec["failure_codes"] is None:
            self.spec["failure_codes"] = []
        if not isinstance(self.spec["failure_codes"], list):
            self.spec["failure_codes"] = list(self.spec["failure_codes"])


# ----------------------------------------------------------------------------
# Atomic write helpers (same pattern as repairer._atomic_write_json)
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


def write_verdict(verdict: GateVerdict | dict, path: pathlib.Path | str) -> pathlib.Path:
    """Validate then atomically write a GateVerdict to ``path``.

    Raises :class:`jsonschema.ValidationError` if the verdict does not
    match ``gate_verdict.schema.json``.
    """
    target = pathlib.Path(path)
    payload = verdict.to_dict() if isinstance(verdict, GateVerdict) else dict(verdict)
    try:
        get_validator().validate(payload)
    except ValidationError as exc:
        logger.warning("write_verdict refused: payload failed schema: %s", exc.message)
        raise
    return _atomic_write_json(target, payload)


def load_verdict(path: pathlib.Path | str) -> GateVerdict | None:
    """Load a verdict from ``path`` if it exists and is valid.

    Returns ``None`` for missing files, unreadable files, or files that do
    not validate against the schema. Errors are logged at INFO; callers
    should treat ``None`` as "no verdict on disk yet".
    """
    target = pathlib.Path(path)
    if not target.exists():
        return None
    try:
        raw = json.loads(target.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        logger.info("load_verdict: %s is unreadable: %s", target, exc)
        return None
    if not isinstance(raw, dict):
        logger.info("load_verdict: %s payload is not a dict", target)
        return None
    try:
        get_validator().validate(raw)
    except ValidationError as exc:
        logger.info("load_verdict: %s failed schema: %s", target, exc.message)
        return None
    return GateVerdict.from_dict(raw)


def validate_verdict(payload: dict) -> None:
    """Convenience wrapper around the schema validator (raises on bad input)."""
    get_validator().validate(payload)


# ----------------------------------------------------------------------------
# Evidence-source classification
# ----------------------------------------------------------------------------
def _extract_spec(packet: Any) -> dict:
    """Return ``packet.spec`` whether the caller hands us a dict or a dataclass."""
    if packet is None:
        return {}
    if hasattr(packet, "spec") and not isinstance(packet, dict):
        spec = getattr(packet, "spec")
        if isinstance(spec, dict):
            return spec
        if spec is None:
            return {}
    if isinstance(packet, dict):
        spec = packet.get("spec")
        return spec if isinstance(spec, dict) else {}
    return {}


def classify_evidence_source(evidence_packet: dict | Any) -> str:
    """Inspect ``evidence_packet.spec.source`` and return the canonical enum.

    Accepted values:

    * ``inner_sandbox`` — evidence was produced inside the devkit sandbox
      (e.g. a unit test or fixture-only run). Counts as a real signal but
      not as a materialised repo result.
    * ``materialized_repo`` — the patch was applied in-place to a real
      checkout and re-tested there.
    * ``external_signal`` — evidence came from outside the devkit (e.g.
      CI, a remote run, a human-attested signal).
    * ``unknown`` — packet missing, malformed, or the source field is
      not one of the above. Callers should treat ``unknown`` as
      "no classification possible".

    Aliases ``runtime_support`` → ``inner_sandbox`` and
    ``declared`` → ``materialized_repo`` are accepted to match the
    sister-schemas without forcing taxonomy unification.
    """
    if evidence_packet is None:
        return EVIDENCE_UNKNOWN
    if not isinstance(evidence_packet, dict) and not hasattr(evidence_packet, "spec"):
        return EVIDENCE_UNKNOWN
    spec = _extract_spec(evidence_packet)
    raw = spec.get("source")
    if raw is None:
        # also check the top-level for legacy packets
        if isinstance(evidence_packet, dict):
            raw = evidence_packet.get("source")
    text = str(raw or "").strip().lower()
    if not text:
        return EVIDENCE_UNKNOWN
    if text in _EVIDENCE_PACKET_SOURCE_ALIASES:
        return _EVIDENCE_PACKET_SOURCE_ALIASES[text]
    return EVIDENCE_UNKNOWN


# ----------------------------------------------------------------------------
# Evidence parsing (best-effort — schema is permissive)
# ----------------------------------------------------------------------------
def _coerce_int(value: Any) -> int:
    if value is None or value == "":
        return 0
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


def _coerce_float(value: Any) -> float:
    if value is None or value == "":
        return 0.0
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def _read_evidence(path: pathlib.Path) -> dict | None:
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def _summarize_evidence(evidence: dict | None) -> dict:
    """Pull the per-check counters out of an evidence dict (best-effort)."""
    out: dict[str, Any] = {
        "evidence_file_present": evidence is not None,
        "tests_passed_count": 0,
        "tests_failed_count": 0,
        "cost_usd": 0.0,
        "budget_cap_usd": DEFAULT_BUDGET_CAP_USD,
        "artifact_manifest_present": False,
        "artifact_manifest_source": "",
    }
    if not isinstance(evidence, dict):
        return out

    # Phase B / Phase D evidence.json is a freeform envelope; we pull
    # counters from the most common locations without requiring a schema.
    summary = evidence.get("summary") if isinstance(evidence.get("summary"), dict) else {}
    spec = evidence.get("spec") if isinstance(evidence.get("spec"), dict) else {}
    # Phase D EvidencePacket writer (``devkit/evidence_writer.write_run_evidence``)
    # embeds a structured counters object under ``spec.metrics`` — and the
    # canonical ``spec.summary`` is a string (per `evidence_packet.schema.json`).
    # When the summary isn't a dict, fall back to ``spec.metrics`` first so
    # the packet output is consumable by the gatekeeper without a parallel
    # summary-table rewrite.
    metrics = spec.get("metrics") if isinstance(spec.get("metrics"), dict) else {}
    results = evidence.get("results") if isinstance(evidence.get("results"), list) else []
    if not results and isinstance(evidence.get("stages"), list):
        results = list(evidence.get("stages") or [])

    def _first_int(*candidates: Any) -> int:
        for cand in candidates:
            if cand is None or cand == "":
                continue
            value = _coerce_int(cand)
            if value:
                return value
        return 0

    passed = _first_int(
        summary.get("tests_passed"),
        summary.get("passed"),
        metrics.get("tests_passed"),
        spec.get("tests_passed"),
    )
    failed = _first_int(
        summary.get("tests_failed"),
        summary.get("failed"),
        metrics.get("tests_failed"),
        spec.get("tests_failed"),
    )
    if not passed and not failed and results:
        for entry in results:
            if not isinstance(entry, dict):
                continue
            outcome = str(entry.get("outcome") or entry.get("status") or "").strip().lower()
            tests = entry.get("tests")
            if isinstance(tests, dict):
                passed += _coerce_int(tests.get("passed", 0))
                failed += _coerce_int(tests.get("failed", 0))
            else:
                if outcome in {"passed", "ok", "success"}:
                    passed += 1
                elif outcome in {"failed", "failure", "regression"}:
                    failed += 1

    cost = _coerce_float(
        summary.get("cost_usd",
                    spec.get("cost_usd",
                             metrics.get("cost_usd",
                                         evidence.get("cost_usd", 0.0))))
    )
    cap = _coerce_float(
        summary.get("budget_cap_usd",
                    spec.get("budget_cap_usd",
                             metrics.get("budget_cap_usd",
                                         evidence.get("budget_cap_usd", DEFAULT_BUDGET_CAP_USD))))
    )

    # The artifact manifest may live at the top level (legacy/Phase B
    # envelope) or under spec (Phase D EvidencePacket). Look at both so the
    # gatekeeper works with either writer.
    manifest = evidence.get("artifact_manifest")
    if not isinstance(manifest, dict) and isinstance(spec.get("artifact_manifest"), dict):
        manifest = spec["artifact_manifest"]
    manifest_source = ""
    manifest_present = False
    if isinstance(manifest, dict):
        manifest_present = True
        manifest_source = str(manifest.get("source", "")).strip()
        if not manifest_source and isinstance(manifest.get("spec"), dict):
            manifest_source = str(manifest["spec"].get("source", "")).strip()

    out.update(
        tests_passed_count=passed,
        tests_failed_count=failed,
        cost_usd=cost,
        budget_cap_usd=cap if cap > 0 else DEFAULT_BUDGET_CAP_USD,
        artifact_manifest_present=manifest_present,
        artifact_manifest_source=manifest_source,
    )
    return out


def _classify_from_summary(summary: dict) -> str:
    """Best-effort evidence source from the evidence.json payload.

    Looks at the common spots:

    * top-level ``source`` / ``evidence_source``
    * ``spec.source`` (mirrors the EvidencePacket contract)
    * ``summary.source`` (in case the producer chose to nest it)
    * ``artifact_manifest.source`` (Phase B writes this; trustworthy enough)

    Falls back to ``unknown`` unless one of those is explicit.
    The Gatekeeper trusts :func:`classify_evidence_source` over this
    heuristic — callers should pass the live ``EvidencePacket`` when
    they have one.
    """
    if not isinstance(summary, dict):
        return EVIDENCE_UNKNOWN

    candidates: list[str] = []
    for top_key in ("evidence_source", "source"):
        v = str(summary.get(top_key, "")).strip().lower()
        if v:
            candidates.append(v)
    spec = summary.get("spec")
    if isinstance(spec, dict):
        v = str(spec.get("source", "")).strip().lower()
        if v:
            candidates.append(v)
    nested_summary = summary.get("summary")
    if isinstance(nested_summary, dict):
        for top_key in ("evidence_source", "source"):
            v = str(nested_summary.get(top_key, "")).strip().lower()
            if v:
                candidates.append(v)
    manifest = summary.get("artifact_manifest")
    if isinstance(manifest, dict):
        v = str(manifest.get("source", "")).strip().lower()
        if v:
            candidates.append(v)
        if isinstance(manifest.get("spec"), dict):
            v = str(manifest["spec"].get("source", "")).strip().lower()
            if v:
                candidates.append(v)

    for v in candidates:
        if v in _EVIDENCE_PACKET_SOURCE_ALIASES:
            return _EVIDENCE_PACKET_SOURCE_ALIASES[v]
    return EVIDENCE_UNKNOWN


# ----------------------------------------------------------------------------
# Evidence location + verdict composition
# ----------------------------------------------------------------------------
def _locate_evidence_file(edir: pathlib.Path, run_id: str) -> tuple[pathlib.Path | None, dict | None, str]:
    """Locate the evidence payload under ``edir`` for ``run_id``.

    Lookup order (first hit wins; missing-file reads are silent):

    1. ``<edir>/<run_id>/evidence_packet.json`` — the canonical Phase D
       per-run packet written by ``devkit.evidence_writer.write_run_evidence``.
    2. ``<edir>/evidence.json`` — the legacy Phase B top-level dump used by
       pre-Phase-D tests (the writer was added without an upgrade path).
    3. ``None`` if neither exists.

    Returns ``(path, payload_or_none, picked_label)`` where ``picked_label``
    is one of ``"evidence_packet"`` / ``"evidence_legacy"`` / ``"missing"``
    — the verdict line preserves the picked location so callers can tell
    which path was used.
    """
    packet_path = edir / run_id / "evidence_packet.json"
    packet = _read_evidence(packet_path)
    if packet is not None:
        return packet_path, packet, "evidence_packet"
    legacy_path = edir / "evidence.json"
    legacy = _read_evidence(legacy_path)
    if legacy is not None:
        return legacy_path, legacy, "evidence_legacy"
    return None, None, "missing"


# ----------------------------------------------------------------------------
# evaluate_final_gate — the main entry point used by the Runner
# ----------------------------------------------------------------------------
def evaluate_final_gate(
    run_id: str,
    work_item_id: str,
    evidence_dir: pathlib.Path | str,
    *,
    evidence_packet: dict | Any | None = None,
    budget_cap_usd: float | None = None,
) -> GateVerdict:
    """Read the per-run evidence packet (or legacy ``evidence.json``) and produce a verdict.

    Lookup under ``evidence_dir`` for ``run_id``:

    1. ``<evidence_dir>/<run_id>/evidence_packet.json`` — canonical Phase D
       per-run packet written by :func:`devkit.evidence_writer.write_run_evidence`.
    2. ``<evidence_dir>/evidence.json`` — legacy Phase B top-level dump,
       kept as a transparent fallback so pre-existing tests keep working.

    When **neither** file exists, the verdict is ``passed=False`` with code
    ``EVIDENCE_MISSING`` and the reason names both candidate paths.

    Failure conditions (any one flips ``passed=False`` and adds its code):

    * neither evidence file found → ``EVIDENCE_MISSING``
    * any test failed (or stage marked ``failure``/``regression``) → ``TEST_REGRESSION``
    * ``cost_usd`` > budget cap (default :data:`DEFAULT_BUDGET_CAP_USD`,
      overridable by kwarg or env ``LOOM_COST_LIMIT_USD``) → ``BUDGET_EXCEEDED``
    * artifact manifest is present but its ``source`` field is missing
      or not in :data:`EVIDENCE_SOURCES` → ``EVIDENCE_INVALID``

    Evidence source priority:

    1. ``evidence_packet`` argument (typed packet wins).
    2. ``spec.source`` inside the evidence file.
    3. ``EVIDENCE_UNKNOWN``.
    """
    rid = _ensure_str(run_id, "run_id")
    wid = _ensure_str(work_item_id, "work_item_id")
    edir = _ensure_path(evidence_dir, "evidence_dir")
    if not edir.is_absolute():
        edir = (REPO_ROOT / edir).resolve()

    failure_codes: list[str] = []
    reasons: list[str] = []

    evidence_path, evidence, picked_label = _locate_evidence_file(edir, rid)
    summary = _summarize_evidence(evidence)

    if evidence is None:
        failure_codes.append(FC_EVIDENCE_MISSING)
        reasons.append(
            f"evidence missing under {edir} for run_id={rid!r} "
            f"(looked for <edir>/<run_id>/evidence_packet.json and <edir>/evidence.json)"
        )

    tests_failed = int(summary.get("tests_failed_count", 0)) > 0
    if tests_failed:
        failure_codes.append(FC_TEST_REGRESSION)
        reasons.append(
            f"{summary.get('tests_failed_count', 0)} test(s) failed"
        )

    cap = float(budget_cap_usd) if budget_cap_usd is not None else float(summary.get("budget_cap_usd", DEFAULT_BUDGET_CAP_USD))
    if cap <= 0:
        cap = DEFAULT_BUDGET_CAP_USD
    cost = float(summary.get("cost_usd", 0.0))
    if cost > cap:
        failure_codes.append(FC_BUDGET_EXCEEDED)
        reasons.append(f"cost_usd={cost:.4f} exceeds budget_cap_usd={cap:.4f}")

    if summary.get("artifact_manifest_present"):
        source = str(summary.get("artifact_manifest_source", "")).strip().lower()
        if not source or source not in EVIDENCE_SOURCES:
            failure_codes.append(FC_EVIDENCE_INVALID)
            reasons.append(
                f"artifact_manifest.source missing or invalid ({source!r})"
            )

    evidence_source = (
        classify_evidence_source(evidence_packet)
        if evidence_packet is not None
        else _classify_from_summary(evidence if isinstance(evidence, dict) else {})
    )
    if evidence_source == EVIDENCE_UNKNOWN and isinstance(evidence, dict):
        # try harder — sometimes the source is nested deeper
        evidence_source = _classify_from_summary(evidence)

    passed = not failure_codes
    reason = "all_gates_passed" if passed else "; ".join(reasons) or "unknown"

    checks = dict(summary)
    checks["evidence_file_present"] = summary.get("evidence_file_present", False)

    lineage: dict[str, Any] = {}
    if isinstance(evidence, dict) and isinstance(evidence.get("lineage"), dict):
        lineage = dict(evidence["lineage"])
    if evidence_path is not None:
        lineage["evidence_path"] = str(evidence_path)
        lineage["evidence_source_kind"] = picked_label

    return GateVerdict.build(
        run_id=rid,
        work_item_id=wid,
        evidence_source=evidence_source,
        passed=passed,
        reason=reason,
        failure_codes=failure_codes,
        checks=checks,
        lineage=lineage,
    )


__all__ = [
    # dataclass
    "GateVerdict",
    # constants
    "PROTOCOL_VERSION",
    "GATEKEEPER_KIND",
    "EVIDENCE_SOURCES",
    "EVIDENCE_INNER_SANDBOX",
    "EVIDENCE_MATERIALIZED_REPO",
    "EVIDENCE_EXTERNAL_SIGNAL",
    "EVIDENCE_UNKNOWN",
    "FAILURE_CODES",
    "FC_EVIDENCE_MISSING",
    "FC_TEST_REGRESSION",
    "FC_BUDGET_EXCEEDED",
    "FC_EVIDENCE_INVALID",
    "FC_SCHEMA_VALIDATION_ERROR",
    "DEFAULT_BUDGET_CAP_USD",
    "SCHEMA_PATH",
    # API
    "classify_evidence_source",
    "evaluate_final_gate",
    "write_verdict",
    "load_verdict",
    "validate_verdict",
    "get_validator",
    "reset_validator_cache",
]