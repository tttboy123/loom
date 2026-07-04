"""devkit/repairer.py — Loom autopilot whitelist-only repair actions (Phase A #3b).

Five whitelist actions only. Anything outside the whitelist is refused + logged.

  - reclaim_stale_running(work_item_id, lease_id)
  - release_orphan_lease(lease_id)
  - insert_repair_task(task_spec)
  - throttle_carry(carrier, scope)
  - mark_blocked(work_item_id, reason)

Plus one dispatcher:

  - dispatch(incident)   # maps Incident → whitelist action

All state mutations on ``devkit/backlog.json`` go through
``devkit.state_writer.transition_task`` / ``enqueue_task`` / ``sync_task_metadata``.
Repairer never reads or writes backlog.json directly.
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import threading
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import Any, Callable, Optional

from jsonschema import Draft202012Validator, ValidationError

# ----------------------------------------------------------------------------
# Paths
# ----------------------------------------------------------------------------
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMA_PATH = REPO_ROOT / "devkit" / "protocol_schemas" / "incident.schema.json"
DEFAULT_BACKLOG_PATH = REPO_ROOT / "devkit" / "backlog.json"
DEFAULT_THROTTLE_PATH = REPO_ROOT / "devkit" / "throttle.json"
DEFAULT_INCIDENT_LOG = REPO_ROOT / "devkit" / "logs" / "incidents.jsonl"

PROTOCOL_VERSION = "loom.dev/v1"
REPAIRER_ACTOR = "repairer"
REPAIRER_SOURCE_TASK = "system.repairer"

# ----------------------------------------------------------------------------
# Logger (module-level so tests can assert log records)
# ----------------------------------------------------------------------------
logger = logging.getLogger("devkit.repairer")
if not logger.handlers:
    handler = logging.NullHandler()
    logger.addHandler(handler)
logger.setLevel(logging.INFO)


# ----------------------------------------------------------------------------
# Whitelist — the ONLY actions the repairer may execute.
# Ordered for stable iteration in test/doc contexts.
# ----------------------------------------------------------------------------
WHITELIST_ACTIONS: tuple[str, ...] = (
    "reclaim_stale_running",
    "release_orphan_lease",
    "insert_repair_task",
    "throttle_carry",
    "mark_blocked",
)

# Dispatch map: incident spec.kind -> whitelist action
INCIDENT_TO_ACTION: dict[str, str] = {
    "stale_running": "reclaim_stale_running",
    "orphan_lease": "release_orphan_lease",
    "repair_needed": "insert_repair_task",
    "carrier_throttle": "throttle_carry",
    "manual_block": "mark_blocked",
}


# ----------------------------------------------------------------------------
# Schema loader (lazy + cached)
# ----------------------------------------------------------------------------
_schema_lock = threading.Lock()
_cached_validator: Optional[Draft202012Validator] = None


def _load_schema() -> dict:
    if not SCHEMA_PATH.exists():
        raise FileNotFoundError(f"incident schema not found: {SCHEMA_PATH}")
    return json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))


def get_validator() -> Draft202012Validator:
    """Return a memoized jsonschema validator for the Incident schema."""
    global _cached_validator
    with _schema_lock:
        if _cached_validator is None:
            _cached_validator = Draft202012Validator(_load_schema())
        return _cached_validator


def reset_validator_cache() -> None:
    """Clear the memoized validator (used by tests when they patch the schema)."""
    global _cached_validator
    with _schema_lock:
        _cached_validator = None


# ----------------------------------------------------------------------------
# Incident dataclass — typed mirror of the schema.
# ----------------------------------------------------------------------------
@dataclass
class Incident:
    api_version: str = PROTOCOL_VERSION
    kind: str = "Incident"
    metadata: dict = field(default_factory=dict)
    spec: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        """Raise jsonschema.ValidationError if this Incident is invalid."""
        get_validator().validate(self.to_dict())


# ----------------------------------------------------------------------------
# Result dataclass — every action returns one of these.
# ----------------------------------------------------------------------------
@dataclass
class RepairResult:
    action: str
    accepted: bool
    outcome: str                 # "applied" | "rejected" | "noop"
    failure_code: str = ""       # populated on reject / noop
    message: str = ""
    payload: dict = field(default_factory=dict)
    timestamp: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())

    def to_dict(self) -> dict:
        return asdict(self)


# ----------------------------------------------------------------------------
# Internal helpers
# ----------------------------------------------------------------------------
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _ensure_str(value: Any, field_name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def _ensure_known_action(action: str) -> None:
    if action not in WHITELIST_ACTIONS:
        raise ValueError(f"action {action!r} is not on the whitelist")


# ----------------------------------------------------------------------------
# Throttle persistence (small JSON file; safe to overwrite atomically)
# ----------------------------------------------------------------------------
def _atomic_write_json(path: pathlib.Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fh:
        fh.write(json.dumps(payload, ensure_ascii=False, indent=2))
        fh.flush()
        os.fsync(fh.fileno())
    os.replace(tmp, path)


def _read_json_or_default(path: pathlib.Path) -> dict:
    if not path.exists():
        return {}
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


# ----------------------------------------------------------------------------
# Whitelist action #1 — reclaim_stale_running
# ----------------------------------------------------------------------------
def reclaim_stale_running(
    work_item_id: str,
    lease_id: str,
    *,
    backlog_path: pathlib.Path | str | None = None,
    event_path: pathlib.Path | str | None = None,
    reason: str = "lease_expired_reclaim",
) -> RepairResult:
    """Reclaim a stale ``running`` task back to ``pending`` and clear its lease.

    Implementation contract:
      - the state machine only allows ``running -> stopped`` directly, so we
        do a two-step transition ``running -> stopped -> pending`` via
        ``state_writer.transition_task``. This keeps every state change in
        the same event log.
      - lease metadata is wiped via ``sync_task_metadata`` (remove lease keys).
    """
    wid = _ensure_str(work_item_id, "work_item_id")
    lid = _ensure_str(lease_id, "lease_id")
    bp = pathlib.Path(backlog_path) if backlog_path is not None else DEFAULT_BACKLOG_PATH

    from devkit import state_writer

    # Step 1: running -> stopped (always allowed; records the reclaim event)
    try:
        state_writer.transition_task(
            backlog_path=bp,
            task_id=wid,
            to_status="stopped",
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason=f"reclaim_stale_running: lease_id={lid} reason={reason}",
            event_path=event_path or state_writer.EVENT_LOG_PATH,
            writer="state_writer",
        )
    except state_writer.TransitionError as exc:
        logger.warning("reclaim_stale_running rejected at running->stopped: %s", exc)
        return RepairResult(
            action="reclaim_stale_running",
            accepted=False,
            outcome="rejected",
            failure_code=exc.failure_code,
            message=str(exc),
            payload={"work_item_id": wid, "lease_id": lid, "step": "running->stopped"},
        )

    # Step 2: stopped -> pending (allowed; task is back in the queue)
    try:
        state_writer.transition_task(
            backlog_path=bp,
            task_id=wid,
            to_status="pending",
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason=f"reclaim_stale_running: requeue lease_id={lid}",
            event_path=event_path or state_writer.EVENT_LOG_PATH,
            writer="state_writer",
        )
    except state_writer.TransitionError as exc:
        # The task is already in a safe "stopped" state; treat a requeue
        # failure as a soft noop rather than a hard failure of the whole
        # reclaim (which has already done its main job).
        logger.warning("reclaim_stale_running: requeue skipped: %s", exc)

    # Best-effort: clear any lease metadata keys. A failure here is logged but
    # the transition has already committed, so we report it as a soft follow-up.
    try:
        state_writer.sync_task_metadata(
            backlog_path=bp,
            task_id=wid,
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason=f"clear_lease_after_reclaim lease_id={lid}",
            remove_keys=("lease", "_lease_reclaim_reason", "_lease_reclaimed_at"),
            event_path=event_path or state_writer.EVENT_LOG_PATH,
        )
    except state_writer.TransitionError as exc:
        # task may not exist if transition went pending→pending from a non-existent
        # state; do not fail the whole action
        logger.info("reclaim_stale_running: lease cleanup skipped: %s", exc)

    return RepairResult(
        action="reclaim_stale_running",
        accepted=True,
        outcome="applied",
        message=f"reclaimed {wid} (lease {lid}) back to pending",
        payload={"work_item_id": wid, "lease_id": lid, "reason": reason},
    )


# ----------------------------------------------------------------------------
# Whitelist action #2 — release_orphan_lease
# ----------------------------------------------------------------------------
def release_orphan_lease(
    lease_id: str,
    *,
    backlog_path: pathlib.Path | str | None = None,
    event_path: pathlib.Path | str | None = None,
) -> RepairResult:
    """Drop lease metadata for a task whose lease points to nothing in the backlog.

    Because the task is not in the backlog we cannot call
    ``sync_task_metadata`` (the row doesn't exist). Instead we record an
    audit-only transition event by searching the in-memory backlog for the
    ``lease`` block and removing the keys via ``sync_task_metadata`` if the
    task IS present, or emitting a noop audit event if it is not.
    """
    lid = _ensure_str(lease_id, "lease_id")
    bp = pathlib.Path(backlog_path) if backlog_path is not None else DEFAULT_BACKLOG_PATH
    from devkit import state_writer

    # Probe backlog to see if the lease is attached to any task. We do this via
    # state_writer's loader so the format is identical to what it would write.
    items, _ = state_writer._load_backlog(bp)
    matched_id: Optional[str] = None
    for entry in items:
        lease = entry.get("lease") if isinstance(entry.get("lease"), dict) else {}
        if str(lease.get("run_id") or "") == lid:
            matched_id = str(entry.get("id", "")).strip()
            break

    if not matched_id:
        return RepairResult(
            action="release_orphan_lease",
            accepted=True,
            outcome="noop",
            message=f"lease {lid} not attached to any backlog task; noop",
            payload={"lease_id": lid},
        )

    try:
        state_writer.sync_task_metadata(
            backlog_path=bp,
            task_id=matched_id,
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason=f"release_orphan_lease lease_id={lid}",
            remove_keys=("lease", "_lease_reclaim_reason", "_lease_reclaimed_at"),
            event_path=event_path or state_writer.EVENT_LOG_PATH,
        )
    except state_writer.TransitionError as exc:
        logger.warning("release_orphan_lease rejected: %s", exc)
        return RepairResult(
            action="release_orphan_lease",
            accepted=False,
            outcome="rejected",
            failure_code=exc.failure_code,
            message=str(exc),
            payload={"lease_id": lid, "work_item_id": matched_id},
        )

    return RepairResult(
        action="release_orphan_lease",
        accepted=True,
        outcome="applied",
        message=f"released orphan lease {lid} from task {matched_id}",
        payload={"lease_id": lid, "work_item_id": matched_id},
    )


# ----------------------------------------------------------------------------
# Whitelist action #3 — insert_repair_task
# ----------------------------------------------------------------------------
def insert_repair_task(
    task_spec: dict,
    *,
    backlog_path: pathlib.Path | str | None = None,
    event_path: pathlib.Path | str | None = None,
) -> RepairResult:
    """Enqueue a new high-priority repair task via ``state_writer.enqueue_task``.

    The supplied task_spec must have at least ``id`` and ``task`` text; ``status``
    is forced to ``pending`` and ``priority`` is forced to ``high`` so the
    repairer cannot accidentally downgrade urgency.
    """
    if not isinstance(task_spec, dict):
        return RepairResult(
            action="insert_repair_task",
            accepted=False,
            outcome="rejected",
            failure_code="INVALID_TASK_SPEC",
            message="task_spec must be a dict",
        )
    spec = dict(task_spec)
    spec_id = str(spec.get("id", "")).strip()
    if not spec_id:
        return RepairResult(
            action="insert_repair_task",
            accepted=False,
            outcome="rejected",
            failure_code="TASK_ID_REQUIRED",
            message="task_spec.id is required",
        )
    spec["status"] = "pending"
    spec["priority"] = "high"
    spec.setdefault("created_at", _utc_now_iso())
    spec.setdefault("source", "repairer")

    bp = pathlib.Path(backlog_path) if backlog_path is not None else DEFAULT_BACKLOG_PATH
    from devkit import state_writer

    try:
        state_writer.enqueue_task(
            backlog_path=bp,
            item=spec,
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason="insert_repair_task",
            event_path=event_path or state_writer.EVENT_LOG_PATH,
        )
    except state_writer.TransitionError as exc:
        logger.warning("insert_repair_task rejected: %s", exc)
        return RepairResult(
            action="insert_repair_task",
            accepted=False,
            outcome="rejected",
            failure_code=exc.failure_code,
            message=str(exc),
            payload={"task_id": spec_id},
        )

    return RepairResult(
        action="insert_repair_task",
        accepted=True,
        outcome="applied",
        message=f"inserted high-priority repair task {spec_id}",
        payload={"task_id": spec_id, "priority": "high"},
    )


# ----------------------------------------------------------------------------
# Whitelist action #4 — throttle_carry
# ----------------------------------------------------------------------------
def throttle_carry(
    carrier: str,
    scope: str,
    *,
    throttle_path: pathlib.Path | str | None = None,
    duration_s: int = 300,
) -> RepairResult:
    """Mark a carrier as throttled for ``duration_s`` seconds.

    Writes ``devkit/throttle.json`` (atomic). The entry is keyed by
    ``(carrier, scope)``; existing entries are merged in.
    """
    c = _ensure_str(carrier, "carrier")
    s = _ensure_str(scope, "scope")
    if duration_s <= 0:
        return RepairResult(
            action="throttle_carry",
            accepted=False,
            outcome="rejected",
            failure_code="INVALID_DURATION",
            message="duration_s must be positive",
            payload={"carrier": c, "scope": s},
        )
    path = pathlib.Path(throttle_path) if throttle_path is not None else DEFAULT_THROTTLE_PATH
    payload = _read_json_or_default(path)
    carriers = payload.get("carriers")
    if not isinstance(carriers, dict):
        carriers = {}
    until = time.time() + int(duration_s)
    carriers[c] = {
        "scope": s,
        "until": until,
        "until_iso": datetime.fromtimestamp(until, tz=timezone.utc).isoformat(),
        "set_at": _utc_now_iso(),
    }
    payload["carriers"] = carriers
    payload["protocol_version"] = PROTOCOL_VERSION
    _atomic_write_json(path, payload)
    return RepairResult(
        action="throttle_carry",
        accepted=True,
        outcome="applied",
        message=f"throttled carrier={c} scope={s} for {duration_s}s",
        payload={"carrier": c, "scope": s, "duration_s": duration_s, "until": until},
    )


# ----------------------------------------------------------------------------
# Whitelist action #5 — mark_blocked
# ----------------------------------------------------------------------------
def mark_blocked(
    work_item_id: str,
    reason: str,
    *,
    backlog_path: pathlib.Path | str | None = None,
    event_path: pathlib.Path | str | None = None,
    incident: dict | None = None,
    incident_log: pathlib.Path | str | None = None,
) -> RepairResult:
    """Transition a ``running`` task to ``blocked`` and write a blocking incident.

    Both the transition and the incident log go through the dedicated
    write paths so the audit trail stays consistent.
    """
    wid = _ensure_str(work_item_id, "work_item_id")
    rsn = _ensure_str(reason, "reason")
    bp = pathlib.Path(backlog_path) if backlog_path is not None else DEFAULT_BACKLOG_PATH
    from devkit import state_writer

    try:
        state_writer.transition_task(
            backlog_path=bp,
            task_id=wid,
            to_status="blocked",
            actor=REPAIRER_ACTOR,
            source_task_id=REPAIRER_SOURCE_TASK,
            reason=f"mark_blocked: {rsn}",
            event_path=event_path or state_writer.EVENT_LOG_PATH,
            writer="state_writer",
        )
    except state_writer.TransitionError as exc:
        logger.warning("mark_blocked rejected: %s", exc)
        return RepairResult(
            action="mark_blocked",
            accepted=False,
            outcome="rejected",
            failure_code=exc.failure_code,
            message=str(exc),
            payload={"work_item_id": wid, "reason": rsn},
        )

    # Append the incident line to the audit log (if requested)
    inc_log = pathlib.Path(incident_log) if incident_log is not None else None
    if inc_log is not None:
        record = {
            "incident": incident or {},
            "work_item_id": wid,
            "reason": rsn,
            "actor": REPAIRER_ACTOR,
            "timestamp": _utc_now_iso(),
        }
        inc_log.parent.mkdir(parents=True, exist_ok=True)
        with inc_log.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(record, ensure_ascii=False) + "\n")

    return RepairResult(
        action="mark_blocked",
        accepted=True,
        outcome="applied",
        message=f"blocked {wid}: {rsn}",
        payload={"work_item_id": wid, "reason": rsn},
    )


# ----------------------------------------------------------------------------
# Dispatch — Incident → whitelist action
# ----------------------------------------------------------------------------
def dispatch(
    incident: Incident | dict,
    *,
    backlog_path: pathlib.Path | str | None = None,
    event_path: pathlib.Path | str | None = None,
    throttle_path: pathlib.Path | str | None = None,
    incident_log: pathlib.Path | str | None = None,
) -> RepairResult:
    """Route an Incident to the matching whitelist action.

    Any spec.kind not in ``INCIDENT_TO_ACTION`` is refused + logged. The
    incident is also validated against ``incident.schema.json`` first.
    """
    if isinstance(incident, Incident):
        inc_dict = incident.to_dict()
        target = incident
    elif isinstance(incident, dict):
        inc_dict = dict(incident)
        target = Incident(
            api_version=inc_dict.get("api_version", PROTOCOL_VERSION),
            kind=inc_dict.get("kind", "Incident"),
            metadata=inc_dict.get("metadata", {}) or {},
            spec=inc_dict.get("spec", {}) or {},
        )
    else:
        logger.warning("dispatch refused: incident is not Incident or dict: %r", type(incident).__name__)
        return RepairResult(
            action="dispatch",
            accepted=False,
            outcome="rejected",
            failure_code="INVALID_INCIDENT",
            message=f"incident must be Incident or dict, got {type(incident).__name__}",
        )

    try:
        target.validate()
    except ValidationError as exc:
        logger.warning("dispatch refused: incident failed schema: %s", exc.message)
        return RepairResult(
            action="dispatch",
            accepted=False,
            outcome="rejected",
            failure_code="SCHEMA_VALIDATION_ERROR",
            message=exc.message,
            payload={"path": list(exc.absolute_path)},
        )

    spec_kind = str(target.spec.get("kind", "")).strip()
    action = INCIDENT_TO_ACTION.get(spec_kind)
    if action is None:
        logger.warning(
            "dispatch refused: spec.kind=%r is not on the incidents->action map (whitelist)",
            spec_kind,
        )
        return RepairResult(
            action="dispatch",
            accepted=False,
            outcome="rejected",
            failure_code="NOT_ON_WHITELIST",
            message=f"spec.kind={spec_kind!r} has no whitelist action",
            payload={"spec_kind": spec_kind, "known": sorted(INCIDENT_TO_ACTION.keys())},
        )

    return _invoke_action(
        action,
        target,
        inc_dict,
        backlog_path=backlog_path,
        event_path=event_path,
        throttle_path=throttle_path,
        incident_log=incident_log,
    )


def _invoke_action(
    action: str,
    target: Incident,
    inc_dict: dict,
    *,
    backlog_path: pathlib.Path | str | None,
    event_path: pathlib.Path | str | None,
    throttle_path: pathlib.Path | str | None,
    incident_log: pathlib.Path | str | None,
) -> RepairResult:
    metadata = target.metadata or {}
    spec = target.spec or {}
    work_item_id = str(metadata.get("work_item_id", "")).strip()
    lease_id = str(metadata.get("lease_id", "")).strip() or work_item_id

    if action == "reclaim_stale_running":
        if not work_item_id:
            return _reject("reclaim_stale_running", "MISSING_WORK_ITEM_ID",
                           "metadata.work_item_id required for reclaim_stale_running")
        return reclaim_stale_running(
            work_item_id=work_item_id,
            lease_id=lease_id,
            backlog_path=backlog_path,
            event_path=event_path,
            reason=str(spec.get("reason", "lease_expired_reclaim")),
        )

    if action == "release_orphan_lease":
        if not lease_id:
            return _reject("release_orphan_lease", "MISSING_LEASE_ID",
                           "metadata.lease_id required for release_orphan_lease")
        return release_orphan_lease(
            lease_id=lease_id,
            backlog_path=backlog_path,
            event_path=event_path,
        )

    if action == "insert_repair_task":
        task_payload = spec.get("task") if isinstance(spec.get("task"), dict) else None
        if task_payload is None:
            return _reject("insert_repair_task", "MISSING_TASK_PAYLOAD",
                           "spec.task dict required for insert_repair_task")
        # work_item_id from metadata is used as task.id if the spec didn't set one
        if not str(task_payload.get("id", "")).strip() and work_item_id:
            task_payload = {**task_payload, "id": work_item_id}
        # carry evidence_refs onto the task so the repair task inherits the chain
        if "evidence_refs" in spec and "evidence_refs" not in task_payload:
            task_payload = {**task_payload, "evidence_refs": list(spec.get("evidence_refs") or [])}
        return insert_repair_task(
            task_spec=task_payload,
            backlog_path=backlog_path,
            event_path=event_path,
        )

    if action == "throttle_carry":
        carrier = str(spec.get("carrier", "")).strip()
        scope = str(spec.get("scope", "")).strip()
        if not carrier or not scope:
            return _reject("throttle_carry", "MISSING_CARRIER_OR_SCOPE",
                           "spec.carrier and spec.scope required for throttle_carry")
        duration = int(spec.get("duration_s", 300) or 300)
        return throttle_carry(
            carrier=carrier,
            scope=scope,
            throttle_path=throttle_path,
            duration_s=duration,
        )

    if action == "mark_blocked":
        if not work_item_id:
            return _reject("mark_blocked", "MISSING_WORK_ITEM_ID",
                           "metadata.work_item_id required for mark_blocked")
        return mark_blocked(
            work_item_id=work_item_id,
            reason=str(spec.get("reason", "marked blocked by repairer")),
            backlog_path=backlog_path,
            event_path=event_path,
            incident=inc_dict,
            incident_log=incident_log,
        )

    # Defensive: should be unreachable because INCIDENT_TO_ACTION is fixed.
    logger.error("dispatch reached unknown action %r — refusing", action)
    return _reject("dispatch", "UNKNOWN_ACTION", f"unknown action: {action!r}")


def _reject(action: str, code: str, message: str) -> RepairResult:
    return RepairResult(
        action=action,
        accepted=False,
        outcome="rejected",
        failure_code=code,
        message=message,
    )


# ----------------------------------------------------------------------------
# Convenience: a registry style lookup, useful for tests and introspection.
# ----------------------------------------------------------------------------
ACTION_REGISTRY: dict[str, Callable[..., RepairResult]] = {
    "reclaim_stale_running": reclaim_stale_running,
    "release_orphan_lease": release_orphan_lease,
    "insert_repair_task": insert_repair_task,
    "throttle_carry": throttle_carry,
    "mark_blocked": mark_blocked,
}


__all__ = [
    "WHITELIST_ACTIONS",
    "INCIDENT_TO_ACTION",
    "ACTION_REGISTRY",
    "Incident",
    "RepairResult",
    "SCHEMA_PATH",
    "PROTOCOL_VERSION",
    "get_validator",
    "reset_validator_cache",
    "reclaim_stale_running",
    "release_orphan_lease",
    "insert_repair_task",
    "throttle_carry",
    "mark_blocked",
    "dispatch",
]
