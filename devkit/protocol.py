"""devkit/protocol.py — Loom A2A (Agent2Agent) + MCP (Model Context Protocol) stub.

Phase C of the Loom new architecture.

Two protocol layers, both in-process for now:

  1. **A2A** — agents declare an ``AgentCard`` (identity + capability list +
     endpoint) and exchange ``AgentMessage`` envelopes. ``send_message``
     routes to the target agent's registered handler (in-process callable
     registry). No HTTP / no asyncio / no websockets — that is for later.

  2. **MCP** — tools (``ToolDescriptor``) and resources
     (``ResourceDescriptor``) are declared in the same shape that MCP
     clients expect. ``invoke_tool(name, arguments)`` looks up the handler
     in an in-process callable registry; ``read_resource(uri)`` returns the
     read-only content payload. The five ``loom://`` resources and four
     ``loom.*`` tools are auto-registered so external clients (VSCode,
     Claude Desktop, …) can introspect Loom state out of the box.

All mutations on the in-memory registries are protected by
``threading.Lock`` so the server can be embedded in the existing
single-process Loom daemon without races. Schema validation is delegated
to ``jsonschema.Draft202012Validator`` against the four schema JSON files
in ``devkit/protocol_schemas/``.

This module deliberately does **not** import asyncio / aiohttp /
websockets. HTTP and WebSocket transport layers are out of scope for
Phase C.
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import threading
import time
import uuid
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from typing import Any, Callable, Optional

from jsonschema import Draft202012Validator, ValidationError

# --------------------------------------------------------------------------- #
# Module-level paths and constants
# --------------------------------------------------------------------------- #
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMAS_DIR = REPO_ROOT / "devkit" / "protocol_schemas"

PROTOCOL_VERSION = "loom.dev/v1"

AGENT_CARD_KIND = "AgentCard"
AGENT_MESSAGE_KIND = "AgentMessage"
TOOL_DESCRIPTOR_KIND = "ToolDescriptor"
RESOURCE_DESCRIPTOR_KIND = "ResourceDescriptor"

AGENT_MESSAGE_KINDS = ("handoff", "request", "response", "broadcast")

LOOM_RESOURCE_BACKLOG = "loom://backlog"
LOOM_RESOURCE_EVENTS = "loom://events"
LOOM_RESOURCE_RUNS = "loom://runs"
LOOM_RESOURCE_INCIDENTS = "loom://incidents"
LOOM_EVIDENCE_URI_PREFIX = "loom://evidence/"

LOOM_TOOL_DISPATCH_INCIDENT = "loom.dispatch_incident"
LOOM_TOOL_ENQUEUE_TASK = "loom.enqueue_task"
LOOM_TOOL_TRANSITION_TASK = "loom.transition_task"
LOOM_TOOL_HEARTBEAT = "loom.heartbeat"

DEFAULT_BACKLOG_PATH = REPO_ROOT / "devkit" / "backlog.json"
DEFAULT_INCIDENT_LOG = REPO_ROOT / "devkit" / "logs" / "incidents.jsonl"
DEFAULT_EVENT_LOG = REPO_ROOT / "devkit" / "events.jsonl"
DEFAULT_RUNS_DIR = REPO_ROOT / "devkit" / "runs"
DEFAULT_HEARTBEAT_PATH = REPO_ROOT / "devkit" / "heartbeat.json"
DEFAULT_EVIDENCE_DIR = REPO_ROOT / "devkit" / "evidence"
EVIDENCE_PACKET_FILENAME = "evidence_packet.json"

EVENT_LOG_TAIL_BYTES = 64 * 1024  # 64 KiB

# --------------------------------------------------------------------------- #
# Logger (module-level so tests can assert log records)
# --------------------------------------------------------------------------- #
logger = logging.getLogger("devkit.protocol")
if not logger.handlers:
    handler = logging.NullHandler()
    logger.addHandler(handler)
logger.setLevel(logging.INFO)


# --------------------------------------------------------------------------- #
# Errors
# --------------------------------------------------------------------------- #
class ProtocolError(RuntimeError):
    """Base class for all devkit.protocol errors."""


class ValidationFailed(ProtocolError):
    """Raised when a payload does not satisfy its declared schema.

    The ``path`` attribute carries the JSON-pointer-ish list of keys that
    triggered the error (so callers can surface a helpful location).
    """

    def __init__(
        self,
        message: str,
        *,
        path: Optional[list] = None,
        validator_kind: Optional[str] = None,
        schema_path: Optional[list] = None,
    ) -> None:
        super().__init__(message)
        self.path = list(path or [])
        self.validator_kind = validator_kind or ""
        self.schema_path = list(schema_path or [])
        self.message = str(message)

    def to_dict(self) -> dict:
        return {
            "error": "ValidationFailed",
            "message": self.message,
            "validator_kind": self.validator_kind,
            "path": self.path,
            "schema_path": self.schema_path,
        }


class UnknownAgent(ProtocolError):
    """Raised when an A2A message targets an unregistered agent."""

    def __init__(self, agent_id: str) -> None:
        super().__init__(f"unknown agent: {agent_id!r}")
        self.agent_id = agent_id


class UnknownTool(ProtocolError):
    """Raised when ``invoke_tool`` is called for an unregistered tool."""

    def __init__(self, tool_name: str) -> None:
        super().__init__(f"unknown tool: {tool_name!r}")
        self.tool_name = tool_name


class UnknownResource(ProtocolError):
    """Raised when ``read_resource`` is called for an unregistered URI."""

    def __init__(self, uri: str) -> None:
        super().__init__(f"unknown resource uri: {uri!r}")
        self.uri = uri


class ToolInvocationError(ProtocolError):
    """Raised when a tool handler raises during invoke_tool."""

    def __init__(self, tool_name: str, original: BaseException) -> None:
        super().__init__(f"tool {tool_name!r} raised: {original}")
        self.tool_name = tool_name
        self.original = original


# --------------------------------------------------------------------------- #
# Schema validators (lazy + cached, thread-safe)
# --------------------------------------------------------------------------- #
_VALIDATOR_LOCK = threading.Lock()
_VALIDATORS: dict[str, Draft202012Validator] = {}


def _load_schema(path: pathlib.Path) -> dict:
    if not path.exists():
        raise FileNotFoundError(f"protocol schema not found: {path}")
    return json.loads(path.read_text(encoding="utf-8"))


def get_validator(kind: str) -> Draft202012Validator:
    """Return a memoized ``jsonschema`` validator for the given kind.

    Supported kinds: ``AgentCard``, ``AgentMessage``,
    ``ToolDescriptor``, ``ResourceDescriptor``. Any other input raises
    ``ValueError``.
    """
    with _VALIDATOR_LOCK:
        cached = _VALIDATORS.get(kind)
        if cached is not None:
            return cached
        filename = {
            AGENT_CARD_KIND: "a2a_agent_card.schema.json",
            AGENT_MESSAGE_KIND: "a2a_message.schema.json",
            TOOL_DESCRIPTOR_KIND: "mcp_tool_descriptor.schema.json",
            RESOURCE_DESCRIPTOR_KIND: "mcp_resource_descriptor.schema.json",
            PROTOCOL_BUNDLE_KIND: "protocol_bundle.schema.json",
        }.get(kind)
        if filename is None:
            raise ValueError(f"unknown schema kind: {kind!r}")
        schema = _load_schema(SCHEMAS_DIR / filename)
        validator = Draft202012Validator(schema)
        _VALIDATORS[kind] = validator
        return validator


def reset_validator_cache() -> None:
    """Clear memoized validators. Tests use this when patching schemas."""
    with _VALIDATOR_LOCK:
        _VALIDATORS.clear()


def validate(kind: str, payload: dict) -> None:
    """Raise ``ValidationFailed`` if ``payload`` does not match ``kind``."""
    try:
        get_validator(kind).validate(payload)
    except ValidationError as exc:
        raise ValidationFailed(
            exc.message,
            path=list(exc.absolute_path),
            validator_kind=kind,
            schema_path=list(exc.absolute_schema_path),
        ) from exc


# --------------------------------------------------------------------------- #
# Dataclasses — typed mirrors of the schemas
# --------------------------------------------------------------------------- #
@dataclass
class AgentCard:
    api_version: str = PROTOCOL_VERSION
    kind: str = AGENT_CARD_KIND
    metadata: dict = field(default_factory=dict)
    spec: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        validate(AGENT_CARD_KIND, self.to_dict())

    @property
    def agent_id(self) -> str:
        return str(self.metadata.get("id", "")).strip()


@dataclass
class AgentMessage:
    api_version: str = PROTOCOL_VERSION
    kind: str = AGENT_MESSAGE_KIND
    metadata: dict = field(default_factory=dict)
    spec: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        validate(AGENT_MESSAGE_KIND, self.to_dict())

    @property
    def message_id(self) -> str:
        return str(self.metadata.get("id", "")).strip()

    @property
    def from_agent(self) -> str:
        return str(self.metadata.get("from_agent", "")).strip()

    @property
    def to_agent(self) -> str:
        return str(self.metadata.get("to_agent", "")).strip()

    @property
    def spec_kind(self) -> str:
        return str(self.spec.get("kind", "")).strip()

    @property
    def correlation_id(self) -> str:
        return str(self.spec.get("correlation_id", "")).strip()


@dataclass
class ToolDescriptor:
    kind: str = TOOL_DESCRIPTOR_KIND
    name: str = ""
    description: str = ""
    input_schema: dict = field(default_factory=dict)
    protocol_version: str = PROTOCOL_VERSION
    metadata: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        validate(TOOL_DESCRIPTOR_KIND, self.to_dict())


@dataclass
class ResourceDescriptor:
    kind: str = RESOURCE_DESCRIPTOR_KIND
    uri: str = ""
    name: str = ""
    description: str = ""
    mime_type: str = ""
    protocol_version: str = PROTOCOL_VERSION
    metadata: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    def validate(self) -> None:
        validate(RESOURCE_DESCRIPTOR_KIND, self.to_dict())


@dataclass
class ToolInvocationResult:
    tool: str
    accepted: bool
    outcome: str           # "ok" | "rejected" | "error"
    result: Any = None
    failure_code: str = ""
    message: str = ""
    timestamp: str = field(default_factory=lambda: _utc_now_iso())

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class ResourceReadResult:
    uri: str
    mime_type: str
    content: Any
    found: bool = True
    message: str = ""
    timestamp: str = field(default_factory=lambda: _utc_now_iso())

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class MessageDeliveryResult:
    message_id: str
    to_agent: str
    accepted: bool
    outcome: str           # "delivered" | "rejected" | "no_handler" | "error"
    response: Any = None
    failure_code: str = ""
    message: str = ""
    timestamp: str = field(default_factory=lambda: _utc_now_iso())

    def to_dict(self) -> dict:
        return asdict(self)


# --------------------------------------------------------------------------- #
# Small helpers
# --------------------------------------------------------------------------- #
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _ensure_str(value: Any, field_name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def _atomic_write_text(path: pathlib.Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fh:
        fh.write(text)
        fh.flush()
        os.fsync(fh.fileno())
    os.replace(tmp, path)


def _atomic_write_json(path: pathlib.Path, payload: dict) -> None:
    _atomic_write_text(path, json.dumps(payload, ensure_ascii=False, indent=2))


def _read_json(path: pathlib.Path) -> Optional[dict]:
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def _read_jsonl_tail(path: pathlib.Path, max_bytes: int = EVENT_LOG_TAIL_BYTES) -> list[dict]:
    """Return up to ``max_bytes`` of trailing JSONL lines, parsed."""
    if not path.exists():
        return []
    try:
        size = path.stat().st_size
    except OSError:
        return []
    if size <= max_bytes:
        try:
            data = path.read_text(encoding="utf-8")
        except OSError:
            return []
    else:
        try:
            with path.open("rb") as fh:
                fh.seek(size - max_bytes)
                data = fh.read().decode("utf-8", errors="replace")
        except OSError:
            return []
    out: list[dict] = []
    for raw in data.splitlines():
        line = raw.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(obj, dict):
            out.append(obj)
    return out


def _new_id(prefix: str) -> str:
    return f"{prefix}-{uuid.uuid4().hex}"


# --------------------------------------------------------------------------- #
# Loom-specific MCP resource handlers
# --------------------------------------------------------------------------- #
def _read_backlog_summary(backlog_path: pathlib.Path) -> dict:
    """Produce a cheap, safe summary of the backlog."""
    payload = _read_json(backlog_path)
    items: list[dict] = []
    if isinstance(payload, dict) and isinstance(payload.get("tasks"), list):
        items = [it for it in payload["tasks"] if isinstance(it, dict)]
    elif isinstance(payload, list):
        items = [it for it in payload if isinstance(it, dict)]
    by_status: dict[str, int] = {}
    by_priority: dict[str, int] = {}
    for entry in items:
        st = str(entry.get("status", "unknown")).strip() or "unknown"
        pr = str(entry.get("priority", "unknown")).strip() or "unknown"
        by_status[st] = by_status.get(st, 0) + 1
        by_priority[pr] = by_priority.get(pr, 0) + 1
    return {
        "path": str(backlog_path),
        "exists": backlog_path.exists(),
        "total": len(items),
        "by_status": by_status,
        "by_priority": by_priority,
        "task_ids": [str(t.get("id", "")) for t in items if str(t.get("id", "")).strip()][:50],
        "sampled_at": _utc_now_iso(),
    }


def _read_events_tail(event_path: pathlib.Path) -> dict:
    events = _read_jsonl_tail(event_path)
    return {
        "path": str(event_path),
        "exists": event_path.exists(),
        "count": len(events),
        "recent": events[-25:],
        "sampled_at": _utc_now_iso(),
    }


def _list_runs(runs_dir: pathlib.Path) -> dict:
    if not runs_dir.exists():
        return {
            "path": str(runs_dir),
            "exists": False,
            "count": 0,
            "run_ids": [],
            "sampled_at": _utc_now_iso(),
        }
    run_ids: list[str] = []
    for entry in sorted(runs_dir.iterdir(), key=lambda p: p.name):
        try:
            if entry.is_dir():
                run_ids.append(entry.name)
        except OSError:
            continue
    return {
        "path": str(runs_dir),
        "exists": True,
        "count": len(run_ids),
        "run_ids": run_ids,
        "sampled_at": _utc_now_iso(),
    }


def _read_incidents(incident_log: pathlib.Path) -> dict:
    if not incident_log.exists():
        return {
            "path": str(incident_log),
            "exists": False,
            "count": 0,
            "recent": [],
            "sampled_at": _utc_now_iso(),
        }
    records: list[dict] = []
    try:
        with incident_log.open("r", encoding="utf-8") as fh:
            for raw in fh:
                line = raw.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if isinstance(obj, dict):
                    records.append(obj)
    except OSError:
        records = []
    return {
        "path": str(incident_log),
        "exists": True,
        "count": len(records),
        "recent": records[-25:],
        "sampled_at": _utc_now_iso(),
    }


def _read_evidence_packet(evidence_dir: pathlib.Path, run_id: str) -> dict:
    packet_path = evidence_dir / run_id / EVIDENCE_PACKET_FILENAME
    if not packet_path.exists():
        return {
            "uri": f"{LOOM_EVIDENCE_URI_PREFIX}{run_id}",
            "run_id": run_id,
            "path": str(packet_path),
            "found": False,
            "message": f"no evidence packet at {packet_path}",
        }
    data = _read_json(packet_path)
    return {
        "uri": f"{LOOM_EVIDENCE_URI_PREFIX}{run_id}",
        "run_id": run_id,
        "path": str(packet_path),
        "found": True,
        "content": data,
    }


# --------------------------------------------------------------------------- #
# Loom-specific MCP tool handler factories
#
# The handlers below close over the configured paths so that
# ``ProtocolServer(backlog_path=...)`` actually affects what the tool
# reads / writes. Each factory returns a plain callable with the
# ``(arguments: dict) -> dict`` signature ProtocolServer expects.
# --------------------------------------------------------------------------- #
def _make_tool_dispatch_incident(
    default_backlog: pathlib.Path,
    default_event_log: pathlib.Path,
    default_incident_log: pathlib.Path,
) -> ToolHandler:
    def _handler(arguments: dict) -> dict:
        if not isinstance(arguments, dict):
            raise ToolInvocationError(
                LOOM_TOOL_DISPATCH_INCIDENT,
                ValueError("arguments must be a dict"),
            )
        incident = arguments.get("incident")
        if not isinstance(incident, dict):
            raise ToolInvocationError(
                LOOM_TOOL_DISPATCH_INCIDENT,
                ValueError("arguments.incident (dict) is required"),
            )
        backlog_path = pathlib.Path(arguments.get("backlog_path") or default_backlog)
        event_path = pathlib.Path(arguments.get("event_path") or default_event_log)
        incident_log = arguments.get("incident_log") or default_incident_log
        if incident_log is not None:
            incident_log = pathlib.Path(incident_log)
        from devkit import repairer as _repairer
        result = _repairer.dispatch(
            incident,
            backlog_path=backlog_path,
            event_path=event_path,
            incident_log=incident_log,
        )
        return result.to_dict()
    return _handler


def _make_tool_enqueue_task(default_backlog: pathlib.Path, default_event_log: pathlib.Path) -> ToolHandler:
    def _handler(arguments: dict) -> dict:
        if not isinstance(arguments, dict):
            raise ToolInvocationError(
                LOOM_TOOL_ENQUEUE_TASK,
                ValueError("arguments must be a dict"),
            )
        item = arguments.get("item")
        if not isinstance(item, dict):
            raise ToolInvocationError(
                LOOM_TOOL_ENQUEUE_TASK,
                ValueError("arguments.item (dict) is required"),
            )
        backlog_path = pathlib.Path(arguments.get("backlog_path") or default_backlog)
        actor = str(arguments.get("actor", "protocol")).strip() or "protocol"
        source_task_id = str(arguments.get("source_task_id", "system.protocol")).strip()
        reason = str(arguments.get("reason", "loom.enqueue_task")).strip()
        event_path = pathlib.Path(arguments.get("event_path") or default_event_log)
        from devkit import state_writer as _sw
        record = _sw.enqueue_task(
            backlog_path=backlog_path,
            item=dict(item),
            actor=actor,
            source_task_id=source_task_id,
            reason=reason,
            event_path=event_path,
        )
        return record
    return _handler


def _make_tool_transition_task(default_backlog: pathlib.Path, default_event_log: pathlib.Path) -> ToolHandler:
    def _handler(arguments: dict) -> dict:
        if not isinstance(arguments, dict):
            raise ToolInvocationError(
                LOOM_TOOL_TRANSITION_TASK,
                ValueError("arguments must be a dict"),
            )
        task_id = arguments.get("task_id")
        to_status = arguments.get("to_status")
        if not isinstance(task_id, str) or not task_id.strip():
            raise ToolInvocationError(
                LOOM_TOOL_TRANSITION_TASK,
                ValueError("arguments.task_id (non-empty str) is required"),
            )
        if not isinstance(to_status, str) or not to_status.strip():
            raise ToolInvocationError(
                LOOM_TOOL_TRANSITION_TASK,
                ValueError("arguments.to_status (non-empty str) is required"),
            )
        backlog_path = pathlib.Path(arguments.get("backlog_path") or default_backlog)
        actor = str(arguments.get("actor", "protocol")).strip() or "protocol"
        source_task_id = str(arguments.get("source_task_id", "system.protocol")).strip()
        reason = str(arguments.get("reason", "loom.transition_task")).strip()
        event_path = pathlib.Path(arguments.get("event_path") or default_event_log)
        from devkit import state_writer as _sw
        record = _sw.transition_task(
            backlog_path=backlog_path,
            task_id=task_id,
            to_status=to_status,
            actor=actor,
            source_task_id=source_task_id,
            reason=reason,
            event_path=event_path,
        )
        return record
    return _handler


def _make_tool_heartbeat(default_path: pathlib.Path) -> ToolHandler:
    def _handler(arguments: dict) -> dict:
        if not isinstance(arguments, dict):
            arguments = {}
        heartbeat_path = pathlib.Path(arguments.get("heartbeat_path") or default_path)
        payload = arguments.get("payload")
        if not isinstance(payload, dict):
            payload = {
                "timestamp": _utc_now_iso(),
                "actor": str(arguments.get("actor", "protocol")).strip() or "protocol",
                "note": str(arguments.get("note", "loom.heartbeat")).strip() or "loom.heartbeat",
                "epoch_ms": int(time.time() * 1000),
            }
        else:
            payload.setdefault("timestamp", _utc_now_iso())
        _atomic_write_json(heartbeat_path, payload)
        return {
            "path": str(heartbeat_path),
            "payload": payload,
            "written_at": _utc_now_iso(),
        }
    return _handler


# --------------------------------------------------------------------------- #
# Default A2A agents (observer / triager / repairer)
#
# These three AgentCards turn ``ProtocolServer()`` from a capability
# surface with no live agents into a working mini-cluster: any A2A
# message addressed to ``observer``, ``triager``, or ``repairer`` is
# routed to the corresponding module function via an in-process
# ``@server.on_message(...)`` handler.
#
# The handlers use **lazy imports** of ``devkit.observer``,
# ``devkit.triager``, and ``devkit.repairer``. None of those modules
# currently import ``devkit.protocol``, so a top-level import would not
# be circular today — but the defensive lazy import keeps the import
# graph stable if any of them grows a backward dependency on the
# protocol package in the future.
# --------------------------------------------------------------------------- #
DEFAULT_AGENT_OBSERVER = "observer"
DEFAULT_AGENT_TRIAGER = "triager"
DEFAULT_AGENT_REPAIRER = "repairer"

DEFAULT_AGENT_IDS: tuple[str, ...] = (
    DEFAULT_AGENT_OBSERVER,
    DEFAULT_AGENT_TRIAGER,
    DEFAULT_AGENT_REPAIRER,
)


def _default_agent_cards() -> list[dict]:
    """Return the 3 Loom-default AgentCard dicts (validated against the schema)."""
    return [
        {
            "api_version": PROTOCOL_VERSION,
            "kind": AGENT_CARD_KIND,
            "metadata": {
                "id": DEFAULT_AGENT_OBSERVER,
                "name": "Observer",
                "description": (
                    "Read-only observer over Loom autopilot state: "
                    "backlog, runs, evidence, events."
                ),
            },
            "spec": {
                "capabilities": [
                    "backlog.read",
                    "runs.read",
                    "evidence.read",
                    "events.read",
                ],
                "endpoint": f"inproc://{DEFAULT_AGENT_OBSERVER}",
                "protocol_version": PROTOCOL_VERSION,
            },
        },
        {
            "api_version": PROTOCOL_VERSION,
            "kind": AGENT_CARD_KIND,
            "metadata": {
                "id": DEFAULT_AGENT_TRIAGER,
                "name": "Triager",
                "description": (
                    "Classifies Loom autopilot findings into incidents "
                    "for the repairer to consume."
                ),
            },
            "spec": {
                "capabilities": [
                    "incident.classify",
                    "findings.write",
                    "backlog.read",
                ],
                "endpoint": f"inproc://{DEFAULT_AGENT_TRIAGER}",
                "protocol_version": PROTOCOL_VERSION,
            },
        },
        {
            "api_version": PROTOCOL_VERSION,
            "kind": AGENT_CARD_KIND,
            "metadata": {
                "id": DEFAULT_AGENT_REPAIRER,
                "name": "Repairer",
                "description": (
                    "Dispatches whitelisted repair actions from incident "
                    "specs to the matching internal repair module."
                ),
            },
            "spec": {
                "capabilities": [
                    "incident.dispatch",
                    "repair.execute",
                    "backlog.read",
                ],
                "endpoint": f"inproc://{DEFAULT_AGENT_REPAIRER}",
                "protocol_version": PROTOCOL_VERSION,
            },
        },
    ]


def _observer_handler(msg: dict) -> dict:
    """A2A handler — invoke ``devkit.observer.snapshot(**body)``.

    Body is forwarded as kwargs to ``observer.snapshot``. Empty body
    uses the module's default paths. The returned ``ObserverSnapshot``
    is serialised via ``to_dict()``.
    """
    body = msg.get("spec", {}).get("body") if isinstance(msg, dict) else None
    if not isinstance(body, dict):
        body = {}
    try:
        from devkit import observer as _observer  # lazy: keep import graph clean
        snap = _observer.snapshot(**body)
        if hasattr(snap, "to_dict"):
            payload = snap.to_dict()
        elif isinstance(snap, dict):
            payload = snap
        else:
            payload = {"value": snap}
        return {
            "status": "ok",
            "agent_id": DEFAULT_AGENT_OBSERVER,
            "result": payload,
        }
    except Exception as exc:  # noqa: BLE001 - wrap so A2A caller sees a typed error
        logger.warning("observer handler raised: %s", exc)
        return {
            "status": "error",
            "agent_id": DEFAULT_AGENT_OBSERVER,
            "failure_code": "HANDLER_RAISED",
            "message": str(exc),
        }


def _triager_handler(msg: dict) -> dict:
    """A2A handler — invoke ``devkit.triager.triage(snap)``.

    Body shape::

        {
          "logs_dir":     "<override path, optional>",
          "backlog_path": "<override path, optional>"
        }

    The handler first takes a snapshot via
    ``observer.snapshot(**body)`` (so callers can drive triage against
    ad-hoc ``logs_dir`` / ``backlog_path`` overrides), then dispatches
    to ``triager.triage(snap)``.

    Note: the task spec named this function ``triager.classify(...)``
    but the codebase module only exposes ``triage(snap)`` (no
    ``classify`` symbol). We dispatch to the actual public API and
    document the divergence in the deliverable.
    """
    body = msg.get("spec", {}).get("body") if isinstance(msg, dict) else None
    if not isinstance(body, dict):
        body = {}
    try:
        from devkit import observer as _observer  # lazy
        from devkit import triager as _triager    # lazy
        snap = _observer.snapshot(**body)
        report = _triager.triage(snap)
        if hasattr(report, "to_dict"):
            payload = report.to_dict()
        elif isinstance(report, dict):
            payload = report
        else:
            payload = {"value": report}
        return {
            "status": "ok",
            "agent_id": DEFAULT_AGENT_TRIAGER,
            "result": payload,
        }
    except Exception as exc:  # noqa: BLE001
        logger.warning("triager handler raised: %s", exc)
        return {
            "status": "error",
            "agent_id": DEFAULT_AGENT_TRIAGER,
            "failure_code": "HANDLER_RAISED",
            "message": str(exc),
        }


def _repairer_handler(msg: dict) -> dict:
    """A2A handler — invoke ``devkit.repairer.dispatch(incident, **kwargs)``.

    Body shape::

        {
          "incident":      {<Incident dict, required>} ,
          "backlog_path":  "<optional>",
          "event_path":    "<optional>",
          "throttle_path": "<optional>",
          "incident_log":  "<optional>"
        }

    The handler rejects bodies missing ``incident`` with a typed
    ``BAD_ARGUMENTS`` failure (no module call is made). On success the
    returned ``RepairResult`` is serialised via ``to_dict()``.
    """
    body = msg.get("spec", {}).get("body") if isinstance(msg, dict) else None
    if not isinstance(body, dict):
        body = {}
    incident = body.get("incident")
    if not isinstance(incident, dict):
        return {
            "status": "error",
            "agent_id": DEFAULT_AGENT_REPAIRER,
            "failure_code": "BAD_ARGUMENTS",
            "message": "body.incident (dict) is required",
        }
    dispatch_kwargs = {k: v for k, v in body.items() if k != "incident"}
    try:
        from devkit import repairer as _repairer  # lazy
        result = _repairer.dispatch(incident, **dispatch_kwargs)
        if hasattr(result, "to_dict"):
            payload = result.to_dict()
        elif isinstance(result, dict):
            payload = result
        else:
            payload = {"value": result}
        return {
            "status": "ok",
            "agent_id": DEFAULT_AGENT_REPAIRER,
            "result": payload,
        }
    except Exception as exc:  # noqa: BLE001
        logger.warning("repairer handler raised: %s", exc)
        return {
            "status": "error",
            "agent_id": DEFAULT_AGENT_REPAIRER,
            "failure_code": "HANDLER_RAISED",
            "message": str(exc),
        }


def register_default_agents(server: ProtocolServer) -> None:
    """Register the 3 Loom-default A2A agents on ``server``.

    Idempotent: re-registering replaces the AgentCards and handlers in
    place. Use ``server.unregister_agent(...)`` to clear any one agent
    or ``server = ProtocolServer(auto_register_loom=False,
    auto_register_agents=False)`` to start with an empty A2A registry.

    The three agents are:

    * ``observer`` — ``backlog.read``, ``runs.read``, ``evidence.read``,
      ``events.read``. Handler delegates to
      ``devkit.observer.snapshot(**body)``.
    * ``triager`` — ``incident.classify``, ``findings.write``,
      ``backlog.read``. Handler delegates to
      ``devkit.triager.triage(snap)`` (the spec named this
      ``triager.classify`` but the module exposes only ``triage``).
    * ``repairer`` — ``incident.dispatch``, ``repair.execute``,
      ``backlog.read``. Handler delegates to
      ``devkit.repairer.dispatch(incident, **body)``.
    """
    if not isinstance(server, ProtocolServer):
        raise TypeError(
            f"register_default_agents expected ProtocolServer, got {type(server).__name__}"
        )

    # Register / overwrite the AgentCards first so introspection
    # (``list_agents()``) shows the right identity even before the
    # handlers fire.
    for card in _default_agent_cards():
        server.register_agent(card)

    # Then bind the handlers. The decorator stores them in the same
    # server-side dict ``register_agent`` doesn't touch, so this order
    # is safe.
    server.on_message(DEFAULT_AGENT_OBSERVER)(_observer_handler)
    server.on_message(DEFAULT_AGENT_TRIAGER)(_triager_handler)
    server.on_message(DEFAULT_AGENT_REPAIRER)(_repairer_handler)

    logger.info(
        "register_default_agents registered ids=%s",
        list(DEFAULT_AGENT_IDS),
    )


# --------------------------------------------------------------------------- #
# ProtocolServer — bundles all four registries
# --------------------------------------------------------------------------- #
Handler = Callable[[AgentMessage], Any]
ResourceReader = Callable[[], dict]
ToolHandler = Callable[[dict], Any]


class ProtocolServer:
    """In-process A2A + MCP server.

    All four registries (agents, message handlers, tools, resources) are
    dict-of-callable protected by per-registry ``threading.Lock`` so the
    server can be embedded in the single-process Loom daemon without
    races.

    The constructor can optionally auto-register the Loom-specific MCP
    resources and tools (default: yes). Pass ``auto_register_loom=False``
    in tests that want a clean slate.
    """

    def __init__(
        self,
        *,
        auto_register_loom: bool = True,
        auto_register_agents: bool = True,
        backlog_path: Optional[pathlib.Path] = None,
        incident_log: Optional[pathlib.Path] = None,
        event_log: Optional[pathlib.Path] = None,
        runs_dir: Optional[pathlib.Path] = None,
        heartbeat_path: Optional[pathlib.Path] = None,
        evidence_dir: Optional[pathlib.Path] = None,
    ) -> None:
        # A2A
        self._agents_lock = threading.Lock()
        self._agents: dict[str, dict] = {}      # agent_id -> AgentCard dict
        self._handlers_lock = threading.Lock()
        self._handlers: dict[str, Handler] = {}  # agent_id -> callable

        # MCP tools
        self._tools_lock = threading.Lock()
        self._tools: dict[str, dict] = {}        # name -> ToolDescriptor dict
        self._tool_handlers: dict[str, ToolHandler] = {}

        # MCP resources
        self._resources_lock = threading.Lock()
        self._resources: dict[str, dict] = {}    # uri -> ResourceDescriptor dict
        self._resource_readers: dict[str, ResourceReader] = {}

        # Configurable paths for Loom-specific stubs
        self._backlog_path = pathlib.Path(backlog_path) if backlog_path else DEFAULT_BACKLOG_PATH
        self._incident_log = pathlib.Path(incident_log) if incident_log else DEFAULT_INCIDENT_LOG
        self._event_log = pathlib.Path(event_log) if event_log else DEFAULT_EVENT_LOG
        self._runs_dir = pathlib.Path(runs_dir) if runs_dir else DEFAULT_RUNS_DIR
        self._heartbeat_path = pathlib.Path(heartbeat_path) if heartbeat_path else DEFAULT_HEARTBEAT_PATH
        self._evidence_dir = pathlib.Path(evidence_dir) if evidence_dir else DEFAULT_EVIDENCE_DIR

        # Toggle for the A2A default-agent registration (independent from
        # the loom-default resources/tools toggle above).
        self._auto_register_agents = bool(auto_register_agents)

        if auto_register_loom:
            self._register_loom_defaults()

    # ------------------------------------------------------------------ #
    # Loom-specific defaults
    # ------------------------------------------------------------------ #
    def _register_loom_defaults(self) -> None:
        """Register the Loom-specific MCP resources and tools."""
        # Resources
        self.register_resource(
            {
                "kind": RESOURCE_DESCRIPTOR_KIND,
                "uri": LOOM_RESOURCE_BACKLOG,
                "name": "loom.backlog",
                "description": "Summary of devkit/backlog.json (counts by status/priority + first 50 task ids).",
                "mime_type": "application/json",
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._resource_readers[LOOM_RESOURCE_BACKLOG] = lambda: _read_backlog_summary(self._backlog_path)

        self.register_resource(
            {
                "kind": RESOURCE_DESCRIPTOR_KIND,
                "uri": LOOM_RESOURCE_EVENTS,
                "name": "loom.events",
                "description": "Tail of devkit/events.jsonl (up to last 64KiB parsed as JSONL).",
                "mime_type": "application/json",
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._resource_readers[LOOM_RESOURCE_EVENTS] = lambda: _read_events_tail(self._event_log)

        self.register_resource(
            {
                "kind": RESOURCE_DESCRIPTOR_KIND,
                "uri": LOOM_RESOURCE_RUNS,
                "name": "loom.runs",
                "description": "List of run ids under devkit/runs/.",
                "mime_type": "application/json",
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._resource_readers[LOOM_RESOURCE_RUNS] = lambda: _list_runs(self._runs_dir)

        self.register_resource(
            {
                "kind": RESOURCE_DESCRIPTOR_KIND,
                "uri": LOOM_RESOURCE_INCIDENTS,
                "name": "loom.incidents",
                "description": "Tail of devkit/logs/incidents.jsonl (last 25 records).",
                "mime_type": "application/json",
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._resource_readers[LOOM_RESOURCE_INCIDENTS] = lambda: _read_incidents(self._incident_log)

        # Tools
        self.register_tool(
            {
                "kind": TOOL_DESCRIPTOR_KIND,
                "name": LOOM_TOOL_DISPATCH_INCIDENT,
                "description": "Dispatch an Incident dict through devkit.repairer.dispatch.",
                "input_schema": {
                    "type": "object",
                    "required": ["incident"],
                    "properties": {
                        "incident": {
                            "type": "object",
                            "description": "Incident document matching devkit/protocol_schemas/incident.schema.json.",
                        },
                    },
                    "additionalProperties": False,
                },
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._tool_handlers[LOOM_TOOL_DISPATCH_INCIDENT] = _make_tool_dispatch_incident(
            self._backlog_path, self._event_log, self._incident_log
        )

        self.register_tool(
            {
                "kind": TOOL_DESCRIPTOR_KIND,
                "name": LOOM_TOOL_ENQUEUE_TASK,
                "description": "Enqueue a task via devkit.state_writer.enqueue_task.",
                "input_schema": {
                    "type": "object",
                    "required": ["item"],
                    "properties": {
                        "item": {
                            "type": "object",
                            "description": "Task dict; must have 'id'. See devkit/task_validator.py.",
                        },
                        "backlog_path": {"type": "string"},
                        "actor": {"type": "string"},
                        "source_task_id": {"type": "string"},
                        "reason": {"type": "string"},
                        "event_path": {"type": "string"},
                    },
                    "additionalProperties": False,
                },
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._tool_handlers[LOOM_TOOL_ENQUEUE_TASK] = _make_tool_enqueue_task(
            self._backlog_path, self._event_log
        )

        self.register_tool(
            {
                "kind": TOOL_DESCRIPTOR_KIND,
                "name": LOOM_TOOL_TRANSITION_TASK,
                "description": "Transition a task status via devkit.state_writer.transition_task.",
                "input_schema": {
                    "type": "object",
                    "required": ["task_id", "to_status"],
                    "properties": {
                        "task_id": {"type": "string"},
                        "to_status": {"type": "string"},
                        "backlog_path": {"type": "string"},
                        "actor": {"type": "string"},
                        "source_task_id": {"type": "string"},
                        "reason": {"type": "string"},
                        "event_path": {"type": "string"},
                    },
                    "additionalProperties": False,
                },
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._tool_handlers[LOOM_TOOL_TRANSITION_TASK] = _make_tool_transition_task(
            self._backlog_path, self._event_log
        )

        self.register_tool(
            {
                "kind": TOOL_DESCRIPTOR_KIND,
                "name": LOOM_TOOL_HEARTBEAT,
                "description": "Write a heartbeat record to devkit/heartbeat.json.",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "heartbeat_path": {"type": "string"},
                        "payload": {"type": "object"},
                        "actor": {"type": "string"},
                        "note": {"type": "string"},
                    },
                    "additionalProperties": False,
                },
                "protocol_version": PROTOCOL_VERSION,
            }
        )
        self._tool_handlers[LOOM_TOOL_HEARTBEAT] = _make_tool_heartbeat(self._heartbeat_path)

        # A2A default agents (observer / triager / repairer). Off by
        # passing ``auto_register_agents=False`` so callers that only want
        # MCP surfaces (resources / tools) can opt out cleanly.
        if self._auto_register_agents:
            register_default_agents(self)

    # ================================================================== #
    # A2A — AgentCard registry
    # ================================================================== #
    def register_agent(self, card: dict) -> None:
        """Validate and store an AgentCard. Idempotent on agent_id."""
        if not isinstance(card, dict):
            raise ValidationFailed(
                "agent card must be a dict",
                validator_kind=AGENT_CARD_KIND,
            )
        validate(AGENT_CARD_KIND, card)
        agent_id = _ensure_str(card.get("metadata", {}).get("id"), "metadata.id")
        with self._agents_lock:
            self._agents[agent_id] = dict(card)
        logger.info("register_agent id=%s capabilities=%s", agent_id,
                    list(card.get("spec", {}).get("capabilities", [])))

    def unregister_agent(self, agent_id: str) -> None:
        """Remove an AgentCard and its handler (if any).

        Silently noops if the agent is not registered.
        """
        _ensure_str(agent_id, "agent_id")
        with self._agents_lock:
            removed = self._agents.pop(agent_id, None)
        with self._handlers_lock:
            self._handlers.pop(agent_id, None)
        if removed is None:
            logger.info("unregister_agent id=%s (noop, not registered)", agent_id)

    def list_agents(self) -> list[dict]:
        """Return all registered AgentCards as a list (sorted by id)."""
        with self._agents_lock:
            cards = [dict(c) for c in self._agents.values()]
        cards.sort(key=lambda c: str(c.get("metadata", {}).get("id", "")))
        return cards

    def get_agent(self, agent_id: str) -> Optional[dict]:
        """Return a single AgentCard by id, or ``None`` if not registered."""
        _ensure_str(agent_id, "agent_id")
        with self._agents_lock:
            card = self._agents.get(agent_id)
        return dict(card) if card is not None else None

    # ================================================================== #
    # A2A — Messaging
    # ================================================================== #
    def on_message(self, agent_id: str) -> Callable[[Handler], Handler]:
        """Decorator to register a message handler for ``agent_id``.

        Usage::

            @server.on_message("triager")
            def handle(msg: AgentMessage) -> dict:
                ...

        The decorated function is stored as the handler for ``agent_id``.
        If the agent's AgentCard is not yet registered, the handler is
        still stored — ``send_message`` will use it. Registering a
        handler does NOT auto-register the AgentCard.
        """
        _ensure_str(agent_id, "agent_id")

        def decorator(fn: Handler) -> Handler:
            if not callable(fn):
                raise TypeError(f"on_message handler must be callable, got {type(fn).__name__}")
            with self._handlers_lock:
                self._handlers[agent_id] = fn
            return fn

        return decorator

    def send_message(self, msg: dict) -> dict:
        """Validate an AgentMessage and dispatch it to the target agent.

        Returns a ``MessageDeliveryResult`` dict. The handler receives the
        validated message as either an ``AgentMessage`` instance (if the
        handler's first parameter is typed) or as a plain dict — for
        simplicity we always pass the validated dict; handlers may wrap it.
        """
        if not isinstance(msg, dict):
            raise ValidationFailed(
                "message must be a dict",
                validator_kind=AGENT_MESSAGE_KIND,
            )

        # Auto-fill id + timestamp BEFORE validation so a partial envelope
        # still passes schema. The schema requires both fields; injecting
        # defaults keeps the protocol permissive at the transport edge.
        metadata = dict(msg.get("metadata") or {})
        if not metadata.get("id"):
            metadata["id"] = _new_id("msg")
        if not metadata.get("timestamp"):
            metadata["timestamp"] = _utc_now_iso()
        msg = dict(msg)
        msg["metadata"] = metadata
        validate(AGENT_MESSAGE_KIND, msg)

        envelope = AgentMessage(
            api_version=msg.get("api_version", PROTOCOL_VERSION),
            kind=msg.get("kind", AGENT_MESSAGE_KIND),
            metadata=dict(msg.get("metadata") or {}),
            spec=dict(msg.get("spec") or {}),
        )

        target = envelope.to_agent
        with self._handlers_lock:
            handler = self._handlers.get(target)

        if handler is None:
            # broadcast (*) is allowed but no fan-out here; stub just records.
            logger.info("send_message to=%s message_id=%s (no handler)", target, envelope.message_id)
            result = MessageDeliveryResult(
                message_id=envelope.message_id,
                to_agent=target,
                accepted=True,
                outcome="no_handler",
                message=f"no handler registered for agent {target!r}",
            )
            return result.to_dict()

        try:
            response = handler(envelope.to_dict())
        except Exception as exc:  # noqa: BLE001 - we wrap every handler exception
            logger.warning("send_message handler raised: %s", exc)
            result = MessageDeliveryResult(
                message_id=envelope.message_id,
                to_agent=target,
                accepted=False,
                outcome="error",
                failure_code="HANDLER_RAISED",
                message=str(exc),
            )
            return result.to_dict()

        result = MessageDeliveryResult(
            message_id=envelope.message_id,
            to_agent=target,
            accepted=True,
            outcome="delivered",
            response=response,
        )
        return result.to_dict()

    # ================================================================== #
    # MCP — Tools
    # ================================================================== #
    def register_tool(self, tool: dict) -> None:
        """Validate and store a ToolDescriptor."""
        if not isinstance(tool, dict):
            raise ValidationFailed(
                "tool must be a dict",
                validator_kind=TOOL_DESCRIPTOR_KIND,
            )
        validate(TOOL_DESCRIPTOR_KIND, tool)
        name = _ensure_str(tool.get("name"), "name")
        with self._tools_lock:
            self._tools[name] = dict(tool)

    def list_tools(self) -> list[dict]:
        """Return all registered ToolDescriptors, sorted by name."""
        with self._tools_lock:
            tools = [dict(t) for t in self._tools.values()]
        tools.sort(key=lambda t: str(t.get("name", "")))
        return tools

    def invoke_tool(self, name: str, arguments: dict) -> dict:
        """Look up the handler for ``name`` and call it.

        Returns a ``ToolInvocationResult`` dict. If the handler raises,
        the exception is wrapped in ``ToolInvocationError`` and the
        result has ``outcome == "error"``.
        """
        if not isinstance(arguments, dict):
            arguments = {}
        with self._tools_lock:
            handler = self._tool_handlers.get(name)
        if handler is None:
            raise UnknownTool(name)
        try:
            payload = handler(arguments)
        except ToolInvocationError as exc:
            logger.warning("invoke_tool %s validation error: %s", name, exc)
            return ToolInvocationResult(
                tool=name,
                accepted=False,
                outcome="rejected",
                failure_code="BAD_ARGUMENTS",
                message=str(exc),
            ).to_dict()
        except Exception as exc:  # noqa: BLE001 - wrap so MCP client sees structured error
            logger.warning("invoke_tool %s raised: %s", name, exc)
            return ToolInvocationResult(
                tool=name,
                accepted=False,
                outcome="error",
                failure_code="HANDLER_RAISED",
                message=str(exc),
            ).to_dict()
        return ToolInvocationResult(
            tool=name,
            accepted=True,
            outcome="ok",
            result=payload,
        ).to_dict()

    # ================================================================== #
    # MCP — Resources
    # ================================================================== #
    def register_resource(self, resource: dict) -> None:
        """Validate and store a ResourceDescriptor."""
        if not isinstance(resource, dict):
            raise ValidationFailed(
                "resource must be a dict",
                validator_kind=RESOURCE_DESCRIPTOR_KIND,
            )
        validate(RESOURCE_DESCRIPTOR_KIND, resource)
        uri = _ensure_str(resource.get("uri"), "uri")
        with self._resources_lock:
            self._resources[uri] = dict(resource)

    def list_resources(self) -> list[dict]:
        """Return all registered ResourceDescriptors, sorted by URI."""
        with self._resources_lock:
            resources = [dict(r) for r in self._resources.values()]
        resources.sort(key=lambda r: str(r.get("uri", "")))
        return resources

    def read_resource(self, uri: str) -> dict:
        """Read the content addressed by ``uri``.

        Returns a ``ResourceReadResult`` dict. For evidence URIs
        (``loom://evidence/<run-id>``) the run-id is parsed and the
        evidence packet is loaded even if the descriptor is not
        separately registered.
        """
        _ensure_str(uri, "uri")

        # Special case: evidence URIs are dynamic (one per run-id).
        if uri.startswith(LOOM_EVIDENCE_URI_PREFIX):
            run_id = uri[len(LOOM_EVIDENCE_URI_PREFIX):].strip()
            if not run_id:
                raise ValidationFailed(
                    "evidence uri missing run-id",
                    path=["uri"],
                    validator_kind=RESOURCE_DESCRIPTOR_KIND,
                )
            payload = _read_evidence_packet(self._evidence_dir, run_id)
            result = ResourceReadResult(
                uri=uri,
                mime_type="application/json",
                content=payload,
                found=bool(payload.get("found")),
                message=str(payload.get("message", "")),
            )
            return result.to_dict()

        with self._resources_lock:
            descriptor = self._resources.get(uri)
            reader = self._resource_readers.get(uri)
        if descriptor is None or reader is None:
            raise UnknownResource(uri)
        try:
            payload = reader()
        except Exception as exc:  # noqa: BLE001
            logger.warning("read_resource %s raised: %s", uri, exc)
            return ResourceReadResult(
                uri=uri,
                mime_type=str(descriptor.get("mime_type", "application/json")),
                content=None,
                found=False,
                message=f"reader raised: {exc}",
            ).to_dict()
        mime = str(descriptor.get("mime_type", "application/json"))
        return ResourceReadResult(
            uri=uri,
            mime_type=mime,
            content=payload,
        ).to_dict()

    # ================================================================== #
    # Introspection helpers
    # ================================================================== #
    def snapshot(self) -> dict:
        """Return a JSON-serializable snapshot of every registry."""
        return {
            "protocol_version": PROTOCOL_VERSION,
            "agents": self.list_agents(),
            "tools": self.list_tools(),
            "resources": self.list_resources(),
            "agent_count": len(self.list_agents()),
            "tool_count": len(self.list_tools()),
            "resource_count": len(self.list_resources()),
            "sampled_at": _utc_now_iso(),
        }


# --------------------------------------------------------------------------- #
# Run protocol bundle (rdloop integration shim)
# --------------------------------------------------------------------------- #
PROTOCOL_BUNDLE_KIND = "RunProtocolBundle"
PROTOCOL_BUNDLE_FILENAME = "protocol_bundle.json"

_RUN_PROTOCOL_BUNDLE_EVENT_TAIL_BYTES = 64 * 1024  # 64 KiB
_RUN_PROTOCOL_BUNDLE_EVENT_MAX_ITEMS = 200


def write_run_protocol_bundle(
    run_dir: Any,
    run_id: Any,
    objective: Any,
    *,
    delivery_mode: Any = None,
    task_kind: Any = None,
    status_code: Any = None,
    gate: Any = None,
    candidate_state: Any = None,
    review_text: Any = None,
    blocked: Any = None,
    tests_failed: bool = False,
    gate_spec: Any = None,
    output_protocol: Any = None,
    artifact_manifest: Any = None,
    collect: Any = None,
    gate_result: Any = None,
) -> pathlib.Path:
    """Write ``run_dir/protocol_bundle.json`` aggregating the rdloop run state.

    This is the Phase C bridge that previously crashed silently inside
    ``devkit/rdloop.py`` because the function did not exist. The caller
    wraps the call in ``try: ... except Exception: pass``, so this
    implementation is intentionally lenient: every parameter is
    optional, every source file is read with ``try/except`` and a
    missing or unreadable file is reported as ``None`` rather than
    raising. Only an inability to *write* the bundle propagates.

    Sources ingested (all optional, all paths relative to ``run_dir``):

    * ``00-task.md``   → ``spec.objective`` and ``metadata.id`` hint.
    * ``99-gate.md``   → status text available via ``spec.status.gate``.
    * ``events.jsonl`` → ``spec.events`` (capped at the last 200 entries
      from the trailing 64 KiB).

    Everything else (gate_spec / output_protocol / artifact_manifest /
    collect / gate_result) is captured verbatim into ``spec.artifacts``
    so the envelope is a faithful snapshot for downstream tooling.

    Returns the absolute path of the bundle that was written.
    """
    run_dir_path = pathlib.Path(run_dir) if not isinstance(run_dir, pathlib.Path) else run_dir
    run_id_str = str(run_id) if run_id is not None else ""
    objective_str = str(objective) if objective is not None else None

    task_rel = pathlib.Path("00-task.md")
    gate_rel = pathlib.Path("99-gate.md")
    events_rel = pathlib.Path("events.jsonl")

    def _read_optional(rel_path: pathlib.Path) -> str | None:
        full = run_dir_path / rel_path
        try:
            return full.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            return None

    task_body = _read_optional(task_rel)
    gate_body = _read_optional(gate_rel)

    # Prefer the in-memory objective arg; fall back to disk only if caller
    # passed nothing. Both may be present — the arg wins because it is what
    # rdloop actually used to drive the run.
    if objective_str is None and task_body is not None:
        objective_str = task_body

    def _maybe_path(rel: pathlib.Path) -> str | None:
        """Return the relative path string only if the file actually exists."""
        full = run_dir_path / rel
        try:
            if full.exists():
                return rel.as_posix()
        except OSError:
            return None
        return None

    sources = {
        "task_path": _maybe_path(task_rel),
        "gate_path": _maybe_path(gate_rel),
        "events_path": _maybe_path(events_rel),
    }

    events = _read_jsonl_tail(
        run_dir_path / events_rel,
        max_bytes=_RUN_PROTOCOL_BUNDLE_EVENT_TAIL_BYTES,
    )
    if len(events) > _RUN_PROTOCOL_BUNDLE_EVENT_MAX_ITEMS:
        events = events[-_RUN_PROTOCOL_BUNDLE_EVENT_MAX_ITEMS:]

    # Normalise blocked → list[str]; tolerate None / str / iterable.
    if blocked is None:
        blocked_list: list[str] = []
    elif isinstance(blocked, str):
        blocked_list = [blocked] if blocked.strip() else []
    elif isinstance(blocked, (list, tuple, set)):
        blocked_list = [str(item) for item in blocked if item is not None]
    else:
        try:
            blocked_list = [str(blocked)]
        except Exception:  # noqa: BLE001
            blocked_list = []

    bundle: dict = {
        "api_version": PROTOCOL_VERSION,
        "kind": PROTOCOL_BUNDLE_KIND,
        "metadata": {
            "id": f"protocol-bundle-{_utc_now_iso()}",
            "run_id": run_id_str,
            "written_at": _utc_now_iso(),
            "run_dir": str(run_dir_path),
        },
        "spec": {
            "objective": objective_str,
            "objective_path": sources["task_path"],
            "sources": sources,
            "status": {
                "delivery_mode": delivery_mode,
                "task_kind": task_kind,
                "status_code": status_code,
                "gate": gate,
                "candidate_state": candidate_state,
                "blocked": blocked_list,
                "tests_failed": bool(tests_failed),
            },
            "artifacts": {
                "review_text": review_text,
                "gate_spec": gate_spec,
                "output_protocol": output_protocol,
                "artifact_manifest": artifact_manifest,
                "collect": collect,
                "gate_result": gate_result,
            },
            "events": events,
        },
    }

    # Best-effort schema self-check. If validation fails we still write
    # the bundle (caller has a broad except), but log so the mismatch is
    # visible in dev logs.
    try:
        validate(PROTOCOL_BUNDLE_KIND, bundle)
    except ValidationFailed as exc:
        logger.warning(
            "write_run_protocol_bundle: bundle failed self-validation (%s) — writing anyway",
            exc,
        )

    run_dir_path.mkdir(parents=True, exist_ok=True)
    out_path = run_dir_path / PROTOCOL_BUNDLE_FILENAME
    _atomic_write_json(out_path, bundle)
    return out_path


# --------------------------------------------------------------------------- #
# Convenience singletons
# --------------------------------------------------------------------------- #
_DEFAULT_SERVER: Optional[ProtocolServer] = None
_DEFAULT_SERVER_LOCK = threading.Lock()


def default_server() -> ProtocolServer:
    """Return a process-wide shared ``ProtocolServer`` (lazy)."""
    global _DEFAULT_SERVER
    with _DEFAULT_SERVER_LOCK:
        if _DEFAULT_SERVER is None:
            _DEFAULT_SERVER = ProtocolServer()
        return _DEFAULT_SERVER


def reset_default_server() -> None:
    """Drop the cached default server. Tests use this between cases."""
    global _DEFAULT_SERVER
    with _DEFAULT_SERVER_LOCK:
        _DEFAULT_SERVER = None


# --------------------------------------------------------------------------- #
# __all__
# --------------------------------------------------------------------------- #
__all__ = [
    # Constants
    "PROTOCOL_VERSION",
    "AGENT_CARD_KIND",
    "AGENT_MESSAGE_KIND",
    "TOOL_DESCRIPTOR_KIND",
    "RESOURCE_DESCRIPTOR_KIND",
    "AGENT_MESSAGE_KINDS",
    "LOOM_RESOURCE_BACKLOG",
    "LOOM_RESOURCE_EVENTS",
    "LOOM_RESOURCE_RUNS",
    "LOOM_RESOURCE_INCIDENTS",
    "LOOM_EVIDENCE_URI_PREFIX",
    "LOOM_TOOL_DISPATCH_INCIDENT",
    "LOOM_TOOL_ENQUEUE_TASK",
    "LOOM_TOOL_TRANSITION_TASK",
    "LOOM_TOOL_HEARTBEAT",
    "DEFAULT_BACKLOG_PATH",
    "DEFAULT_INCIDENT_LOG",
    "DEFAULT_EVENT_LOG",
    "DEFAULT_RUNS_DIR",
    "DEFAULT_HEARTBEAT_PATH",
    "DEFAULT_EVIDENCE_DIR",
    "EVIDENCE_PACKET_FILENAME",
    "PROTOCOL_BUNDLE_KIND",
    "PROTOCOL_BUNDLE_FILENAME",
    "DEFAULT_AGENT_OBSERVER",
    "DEFAULT_AGENT_TRIAGER",
    "DEFAULT_AGENT_REPAIRER",
    "DEFAULT_AGENT_IDS",
    # Errors
    "ProtocolError",
    "ValidationFailed",
    "UnknownAgent",
    "UnknownTool",
    "UnknownResource",
    "ToolInvocationError",
    # Schema helpers
    "get_validator",
    "reset_validator_cache",
    "validate",
    # Dataclasses
    "AgentCard",
    "AgentMessage",
    "ToolDescriptor",
    "ResourceDescriptor",
    "ToolInvocationResult",
    "ResourceReadResult",
    "MessageDeliveryResult",
    # Server
    "ProtocolServer",
    "default_server",
    "reset_default_server",
# Run protocol bundle (rdloop integration)
    "write_run_protocol_bundle",
    # Default agents (A2A / MCP)
    "register_default_agents",
]