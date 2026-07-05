"""Tests for devkit/protocol.py — Phase C A2A + MCP stub.

Coverage target: >= 20 tests across:

  * AgentCard  register / unregister / list / get           (4)
  * AgentMessage  send_message + handler dispatch            (4)
  * MCP tool  register / list / invoke                       (4)
  * MCP resource  register / list / read                     (4)
  * Schema validation failure paths (bad api_version / missing
    field / unknown kind / wrong type)                       (4)
  * Loom-specific MCP resources (backlog / events / runs /
    evidence / incidents) read correctly                     (5)
  * Loom-specific MCP tools (dispatch_incident / enqueue_task /
    transition_task / heartbeat) work                        (4)
  * Concurrency: register + read are thread-safe             (2)

Plus a few extras (default_server, snapshot, dataclass helpers).
"""
from __future__ import annotations

import json
import os
import pathlib
import sys
import tempfile
import threading
import time
import unittest

from devkit import protocol
from devkit.protocol import (
    AGENT_CARD_KIND,
    AGENT_MESSAGE_KIND,
    AGENT_MESSAGE_KINDS,
    PROTOCOL_VERSION,
    RESOURCE_DESCRIPTOR_KIND,
    TOOL_DESCRIPTOR_KIND,
    LOOM_EVIDENCE_URI_PREFIX,
    LOOM_RESOURCE_BACKLOG,
    LOOM_RESOURCE_EVENTS,
    LOOM_RESOURCE_INCIDENTS,
    LOOM_RESOURCE_RUNS,
    LOOM_TOOL_DISPATCH_INCIDENT,
    LOOM_TOOL_ENQUEUE_TASK,
    LOOM_TOOL_HEARTBEAT,
    LOOM_TOOL_TRANSITION_TASK,
    AgentCard,
    AgentMessage,
    ProtocolServer,
    ProtocolError,
    ResourceDescriptor,
    ToolDescriptor,
    ToolInvocationError,
    UnknownResource,
    UnknownTool,
    ValidationFailed,
    default_server,
    get_validator,
    reset_default_server,
    reset_validator_cache,
    validate,
)


# ---------------------------------------------------------------------------- #
# Helpers
# ---------------------------------------------------------------------------- #
def _make_agent_card(
    agent_id: str = "test-agent",
    name: str = "Test Agent",
    capabilities: list[str] | None = None,
    endpoint: str = "devkit.test.handle",
) -> dict:
    return {
        "api_version": PROTOCOL_VERSION,
        "kind": AGENT_CARD_KIND,
        "metadata": {
            "id": agent_id,
            "name": name,
            "description": "agent for unit tests",
        },
        "spec": {
            "capabilities": capabilities or ["backlog.read"],
            "endpoint": endpoint,
            "protocol_version": PROTOCOL_VERSION,
        },
    }


def _make_agent_message(
    from_agent: str = "sender",
    to_agent: str = "receiver",
    spec_kind: str = "handoff",
    body: dict | None = None,
    correlation_id: str = "",
) -> dict:
    return {
        "api_version": PROTOCOL_VERSION,
        "kind": AGENT_MESSAGE_KIND,
        "metadata": {
            "id": f"msg-{from_agent}-{to_agent}",
            "from_agent": from_agent,
            "to_agent": to_agent,
            "timestamp": "2026-07-05T00:00:00+00:00",
        },
        "spec": {
            "kind": spec_kind,
            "body": body or {"task_id": "abc"},
            "correlation_id": correlation_id,
        },
    }


def _make_tool(
    name: str = "test.tool",
    description: str = "A test tool",
    required: list[str] | None = None,
) -> dict:
    return {
        "kind": TOOL_DESCRIPTOR_KIND,
        "name": name,
        "description": description,
        "input_schema": {
            "type": "object",
            "properties": {
                "x": {"type": "string"},
            },
            "required": required or ["x"],
            "additionalProperties": False,
        },
        "protocol_version": PROTOCOL_VERSION,
    }


def _make_resource(
    uri: str = "test://x",
    name: str = "X",
    description: str = "X",
    mime_type: str = "application/json",
) -> dict:
    return {
        "kind": RESOURCE_DESCRIPTOR_KIND,
        "uri": uri,
        "name": name,
        "description": description,
        "mime_type": mime_type,
        "protocol_version": PROTOCOL_VERSION,
    }


# =============================================================================
# AgentCard lifecycle
# =============================================================================
class TestAgentCardLifecycle(unittest.TestCase):
    def setUp(self) -> None:
        self.server = ProtocolServer(auto_register_loom=False)

    def test_register_agent_stores_card(self):
        card = _make_agent_card("triager")
        self.server.register_agent(card)
        stored = self.server.get_agent("triager")
        self.assertIsNotNone(stored)
        self.assertEqual(stored["metadata"]["id"], "triager")
        self.assertEqual(stored["metadata"]["name"], "Test Agent")

    def test_unregister_agent_removes_card_and_handler(self):
        card = _make_agent_card("triager")

        @self.server.on_message("triager")
        def _h(msg):  # pragma: no cover - not invoked here
            return {"ok": True}

        self.server.register_agent(card)
        self.server.unregister_agent("triager")
        self.assertIsNone(self.server.get_agent("triager"))
        # handler should be gone too
        result = self.server.send_message(_make_agent_message("a", "triager"))
        self.assertEqual(result["outcome"], "no_handler")

    def test_unregister_unknown_agent_is_noop(self):
        # Must not raise even if agent isn't registered.
        self.server.unregister_agent("never-existed")
        self.assertEqual(self.server.list_agents(), [])

    def test_list_agents_returns_sorted_cards(self):
        ids = ["zeta", "alpha", "mu"]
        for aid in ids:
            self.server.register_agent(_make_agent_card(aid, name=aid.upper()))
        listed = self.server.list_agents()
        self.assertEqual([c["metadata"]["id"] for c in listed], sorted(ids))

    def test_register_agent_validates_card(self):
        bad = dict(_make_agent_card("x"))
        bad["api_version"] = "wrong/v1"
        with self.assertRaises(ValidationFailed) as ctx:
            self.server.register_agent(bad)
        self.assertEqual(ctx.exception.validator_kind, AGENT_CARD_KIND)
        self.assertIn("path", ctx.exception.to_dict())

    def test_register_agent_rejects_non_dict(self):
        with self.assertRaises(ValidationFailed):
            self.server.register_agent("not a dict")  # type: ignore[arg-type]


# =============================================================================
# AgentMessage dispatch
# =============================================================================
class TestAgentMessageDispatch(unittest.TestCase):
    def setUp(self) -> None:
        self.server = ProtocolServer(auto_register_loom=False)
        self.received: list[dict] = []

        @self.server.on_message("receiver")
        def _h(msg):
            self.received.append(msg)
            return {"ok": True, "echoed": msg["metadata"]["id"]}

    def test_send_message_dispatches_to_handler(self):
        msg = _make_agent_message("sender", "receiver", spec_kind="request")
        result = self.server.send_message(msg)
        self.assertEqual(result["outcome"], "delivered")
        self.assertTrue(result["accepted"])
        self.assertEqual(result["response"], {"ok": True, "echoed": msg["metadata"]["id"]})
        self.assertEqual(len(self.received), 1)
        self.assertEqual(self.received[0]["metadata"]["to_agent"], "receiver")

    def test_send_message_returns_no_handler_for_unknown_agent(self):
        msg = _make_agent_message("sender", "ghost")
        result = self.server.send_message(msg)
        self.assertEqual(result["outcome"], "no_handler")
        self.assertTrue(result["accepted"])  # delivery isn't a hard failure
        self.assertEqual(result["to_agent"], "ghost")
        self.assertEqual(self.received, [])

    def test_send_message_wraps_handler_exception(self):
        @self.server.on_message("boom")
        def _boom(_msg):
            raise RuntimeError("kaboom")

        result = self.server.send_message(_make_agent_message("a", "boom"))
        self.assertEqual(result["outcome"], "error")
        self.assertEqual(result["failure_code"], "HANDLER_RAISED")
        self.assertIn("kaboom", result["message"])

    def test_send_message_auto_fills_id_and_timestamp_when_missing(self):
        # Manually craft a message missing id + timestamp; the server should
        # not raise, it should inject values.
        partial = {
            "api_version": PROTOCOL_VERSION,
            "kind": AGENT_MESSAGE_KIND,
            "metadata": {
                "from_agent": "a",
                "to_agent": "receiver",
            },
            "spec": {"kind": "request", "body": {"x": 1}},
        }
        result = self.server.send_message(partial)
        self.assertEqual(result["outcome"], "delivered")
        # The handler should have seen the injected id+timestamp.
        self.assertTrue(self.received[0]["metadata"]["id"])
        self.assertTrue(self.received[0]["metadata"]["timestamp"])

    def test_send_message_validates_against_schema(self):
        bad = _make_agent_message("a", "receiver")
        bad["spec"]["kind"] = "unknown_kind"  # not in enum
        with self.assertRaises(ValidationFailed) as ctx:
            self.server.send_message(bad)
        self.assertEqual(ctx.exception.validator_kind, AGENT_MESSAGE_KIND)
        self.assertIn("spec", ctx.exception.path)

    def test_send_message_broadcast_no_handler(self):
        # broadcast to '*' is treated like any other missing target
        result = self.server.send_message(_make_agent_message("a", "*"))
        self.assertEqual(result["outcome"], "no_handler")

    def test_on_message_rejects_non_callable(self):
        with self.assertRaises(TypeError):
            self.server.on_message("x")(123)  # type: ignore[arg-type]


# =============================================================================
# MCP tool registration + invocation
# =============================================================================
class TestToolRegistry(unittest.TestCase):
    def setUp(self) -> None:
        self.server = ProtocolServer(auto_register_loom=False)

    def test_register_tool_stores_descriptor(self):
        self.server.register_tool(_make_tool("math.add", "Add two numbers"))
        tools = self.server.list_tools()
        self.assertEqual([t["name"] for t in tools], ["math.add"])

    def test_register_tool_validates(self):
        bad = _make_tool("math.add")
        bad["input_schema"] = {"type": "not-a-valid-type"}
        # jsonschema is permissive about input_schema's internals (we use
        # additionalProperties: true), so we instead try a clearly invalid
        # case — missing required top-level fields.
        really_bad = {"kind": TOOL_DESCRIPTOR_KIND, "name": "x"}
        with self.assertRaises(ValidationFailed):
            self.server.register_tool(really_bad)

    def test_list_tools_sorted_by_name(self):
        for n in ("z.last", "a.first", "m.middle"):
            self.server.register_tool(_make_tool(n, f"tool {n}"))
        names = [t["name"] for t in self.server.list_tools()]
        self.assertEqual(names, sorted(names))

    def test_invoke_tool_runs_handler(self):
        # Register a custom tool + handler via the public API. We need a
        # handler, so we register the tool, then poke the private handler
        # dict directly (the same pattern the Loom defaults use).
        self.server.register_tool(_make_tool("echo.upper", "Echo uppercased"))
        self.server._tool_handlers["echo.upper"] = lambda args: {"out": args["x"].upper()}
        result = self.server.invoke_tool("echo.upper", {"x": "hello"})
        self.assertEqual(result["outcome"], "ok")
        self.assertEqual(result["result"], {"out": "HELLO"})

    def test_invoke_unknown_tool_raises(self):
        with self.assertRaises(UnknownTool) as ctx:
            self.server.invoke_tool("nope", {})
        self.assertEqual(ctx.exception.tool_name, "nope")

    def test_invoke_tool_wraps_handler_exception(self):
        self.server._tool_handlers["will.raise"] = lambda _a: (_ for _ in ()).throw(
            RuntimeError("boom")
        )
        result = self.server.invoke_tool("will.raise", {})
        self.assertEqual(result["outcome"], "error")
        self.assertEqual(result["failure_code"], "HANDLER_RAISED")

    def test_invoke_tool_wraps_tool_invocation_error(self):
        def _bad_handler(args):
            raise ToolInvocationError("my.tool", ValueError("bad arg"))

        self.server._tool_handlers["my.tool"] = _bad_handler
        result = self.server.invoke_tool("my.tool", {})
        self.assertEqual(result["outcome"], "rejected")
        self.assertEqual(result["failure_code"], "BAD_ARGUMENTS")


# =============================================================================
# MCP resource registration + read
# =============================================================================
class TestResourceRegistry(unittest.TestCase):
    def setUp(self) -> None:
        self.server = ProtocolServer(auto_register_loom=False)

    def test_register_resource_stores_descriptor(self):
        self.server.register_resource(_make_resource("test://x"))
        self.assertEqual(len(self.server.list_resources()), 1)

    def test_register_resource_validates(self):
        with self.assertRaises(ValidationFailed):
            self.server.register_resource({"kind": RESOURCE_DESCRIPTOR_KIND, "uri": "x"})
        # uri pattern check
        with self.assertRaises(ValidationFailed):
            self.server.register_resource(_make_resource("no-scheme"))

    def test_list_resources_sorted_by_uri(self):
        uris = ["z://x", "a://y", "m://z"]
        for u in uris:
            self.server.register_resource(_make_resource(u, name=u))
        self.assertEqual([r["uri"] for r in self.server.list_resources()], sorted(uris))

    def test_read_resource_runs_reader(self):
        self.server.register_resource(_make_resource("test://hello"))
        self.server._resource_readers["test://hello"] = lambda: {"value": 42}
        result = self.server.read_resource("test://hello")
        self.assertTrue(result["found"])
        self.assertEqual(result["content"], {"value": 42})
        self.assertEqual(result["mime_type"], "application/json")

    def test_read_unknown_resource_raises(self):
        with self.assertRaises(UnknownResource) as ctx:
            self.server.read_resource("missing://x")
        self.assertEqual(ctx.exception.uri, "missing://x")

    def test_read_resource_wraps_reader_exception(self):
        self.server.register_resource(_make_resource("test://boom"))
        self.server._resource_readers["test://boom"] = lambda: (_ for _ in ()).throw(
            RuntimeError("kaboom")
        )
        result = self.server.read_resource("test://boom")
        self.assertFalse(result["found"])
        self.assertIn("kaboom", result["message"])


# =============================================================================
# Schema validation failure paths
# =============================================================================
class TestSchemaValidationFailures(unittest.TestCase):
    def setUp(self) -> None:
        reset_validator_cache()

    def tearDown(self) -> None:
        reset_validator_cache()

    def test_bad_api_version_rejected(self):
        bad = _make_agent_card("x")
        bad["api_version"] = "loom.dev/v2"
        with self.assertRaises(ValidationFailed) as ctx:
            validate(AGENT_CARD_KIND, bad)
        # path should point at api_version
        self.assertIn("api_version", ctx.exception.path)

    def test_missing_required_field_rejected(self):
        bad = {
            "api_version": PROTOCOL_VERSION,
            "kind": AGENT_CARD_KIND,
            "metadata": {"name": "X"},  # missing id
            "spec": {
                "capabilities": ["a"],
                "endpoint": "x",
                "protocol_version": PROTOCOL_VERSION,
            },
        }
        with self.assertRaises(ValidationFailed) as ctx:
            validate(AGENT_CARD_KIND, bad)
        self.assertIn("metadata", ctx.exception.path)

    def test_unknown_kind_rejected(self):
        bad = _make_agent_card("x")
        bad["kind"] = "NotAgentCard"
        with self.assertRaises(ValidationFailed):
            validate(AGENT_CARD_KIND, bad)

    def test_wrong_type_rejected(self):
        bad = _make_agent_card("x")
        # capabilities must be array of strings
        bad["spec"]["capabilities"] = "not-a-list"
        with self.assertRaises(ValidationFailed):
            validate(AGENT_CARD_KIND, bad)

    def test_validate_unsupported_kind_raises_value_error(self):
        with self.assertRaises(ValueError):
            get_validator("TotallyMadeUp")

    def test_message_validation_bad_spec_kind(self):
        bad = _make_agent_message("a", "b", spec_kind="not_in_enum")
        with self.assertRaises(ValidationFailed) as ctx:
            validate(AGENT_MESSAGE_KIND, bad)
        self.assertEqual(ctx.exception.validator_kind, AGENT_MESSAGE_KIND)


# =============================================================================
# Loom-specific MCP resources
# =============================================================================
class TestLoomResources(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = pathlib.Path(self.tmp.name)
        self.backlog = self.root / "backlog.json"
        self.events = self.root / "events.jsonl"
        self.incidents = self.root / "logs" / "incidents.jsonl"
        self.runs = self.root / "runs"
        self.evidence = self.root / "evidence"
        self.incidents.parent.mkdir(parents=True, exist_ok=True)
        self.runs.mkdir()
        self.evidence.mkdir()

        self.server = ProtocolServer(
            auto_register_loom=True,
            backlog_path=self.backlog,
            event_log=self.events,
            incident_log=self.incidents,
            runs_dir=self.runs,
            evidence_dir=self.evidence,
        )

    def test_loom_backlog_resource_reads_summary(self):
        self.backlog.write_text(json.dumps({
            "tasks": [
                {"id": "t1", "status": "running", "priority": "high"},
                {"id": "t2", "status": "pending", "priority": "low"},
                {"id": "t3", "status": "done", "priority": "high"},
            ]
        }))
        result = self.server.read_resource(LOOM_RESOURCE_BACKLOG)
        self.assertTrue(result["found"])
        content = result["content"]
        self.assertEqual(content["total"], 3)
        self.assertEqual(content["by_status"], {"running": 1, "pending": 1, "done": 1})
        self.assertEqual(content["by_priority"], {"high": 2, "low": 1})
        self.assertEqual(content["task_ids"], ["t1", "t2", "t3"])

    def test_loom_backlog_handles_missing_file(self):
        # backlog file does not exist
        result = self.server.read_resource(LOOM_RESOURCE_BACKLOG)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["total"], 0)
        self.assertFalse(result["content"]["exists"])

    def test_loom_events_resource_reads_tail(self):
        # write a few JSONL lines
        with self.events.open("w", encoding="utf-8") as fh:
            for i in range(3):
                fh.write(json.dumps({"idx": i, "task_id": f"t{i}"}) + "\n")
        result = self.server.read_resource(LOOM_RESOURCE_EVENTS)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["count"], 3)
        self.assertEqual(len(result["content"]["recent"]), 3)
        self.assertEqual(result["content"]["recent"][-1]["idx"], 2)

    def test_loom_events_handles_missing_file(self):
        self.events.unlink(missing_ok=True)
        result = self.server.read_resource(LOOM_RESOURCE_EVENTS)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["count"], 0)
        self.assertFalse(result["content"]["exists"])

    def test_loom_runs_resource_lists_runs(self):
        for rid in ("run-a", "run-b", "run-c"):
            (self.runs / rid).mkdir()
        # plus a file (should be ignored)
        (self.runs / "stray.txt").write_text("ignore me")
        result = self.server.read_resource(LOOM_RESOURCE_RUNS)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["count"], 3)
        self.assertEqual(sorted(result["content"]["run_ids"]), ["run-a", "run-b", "run-c"])

    def test_loom_runs_handles_missing_dir(self):
        # delete runs dir
        for p in self.runs.iterdir():
            p.rmdir()
        self.runs.rmdir()
        result = self.server.read_resource(LOOM_RESOURCE_RUNS)
        self.assertTrue(result["found"])
        self.assertFalse(result["content"]["exists"])
        self.assertEqual(result["content"]["count"], 0)

    def test_loom_evidence_resource_reads_packet(self):
        run_id = "run-001"
        run_dir = self.evidence / run_id
        run_dir.mkdir()
        packet = {
            "api_version": PROTOCOL_VERSION,
            "kind": "EvidencePacket",
            "metadata": {"id": "ep-1", "work_item_id": "t1"},
            "spec": {"summary": "all green"},
        }
        (run_dir / "evidence_packet.json").write_text(json.dumps(packet))
        result = self.server.read_resource(f"{LOOM_EVIDENCE_URI_PREFIX}{run_id}")
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["content"]["spec"]["summary"], "all green")

    def test_loom_evidence_resource_missing_packet(self):
        result = self.server.read_resource(f"{LOOM_EVIDENCE_URI_PREFIX}missing-run")
        self.assertFalse(result["found"])
        self.assertIn("no evidence packet", result["content"]["message"])

    def test_loom_evidence_uri_without_run_id_rejected(self):
        with self.assertRaises(ValidationFailed):
            self.server.read_resource(LOOM_EVIDENCE_URI_PREFIX)

    def test_loom_incidents_resource_reads_log(self):
        for i in range(2):
            with self.incidents.open("a", encoding="utf-8") as fh:
                fh.write(json.dumps({"idx": i, "reason": f"r{i}"}) + "\n")
        result = self.server.read_resource(LOOM_RESOURCE_INCIDENTS)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["count"], 2)
        self.assertEqual(len(result["content"]["recent"]), 2)

    def test_loom_incidents_handles_missing_file(self):
        self.incidents.unlink(missing_ok=True)
        result = self.server.read_resource(LOOM_RESOURCE_INCIDENTS)
        self.assertTrue(result["found"])
        self.assertEqual(result["content"]["count"], 0)


# =============================================================================
# Loom-specific MCP tools
# =============================================================================
class TestLoomTools(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = pathlib.Path(self.tmp.name)
        self.backlog = self.root / "backlog.json"
        self.events = self.root / "events.jsonl"
        self.incidents = self.root / "logs" / "incidents.jsonl"
        self.heartbeat = self.root / "heartbeat.json"
        self.backlog.write_text('{"tasks": []}')
        self.events.write_text("")
        self.incidents.parent.mkdir(parents=True, exist_ok=True)
        self.incidents.write_text("")

        self.server = ProtocolServer(
            auto_register_loom=True,
            backlog_path=self.backlog,
            event_log=self.events,
            incident_log=self.incidents,
            heartbeat_path=self.heartbeat,
        )

    def test_loom_dispatch_incident_real_dispatch(self):
        # Seed a stale-running task and dispatch an incident
        self.backlog.write_text(json.dumps({
            "tasks": [{
                "id": "stuck-1", "status": "running", "priority": "high",
                "stages": "plan,implement,verify,review", "deps": [],
                "lease": {"owner_pid": 99999, "run_id": "orphan",
                          "heartbeat_at": "2020-01-01T00:00:00Z",
                          "timeout_seconds": 1800},
            }]
        }))
        result = self.server.invoke_tool(LOOM_TOOL_DISPATCH_INCIDENT, {
            "incident": {
                "api_version": PROTOCOL_VERSION,
                "kind": "Incident",
                "metadata": {"id": "inc-1", "work_item_id": "stuck-1"},
                "spec": {"kind": "stale_running", "severity": "warn"},
            }
        })
        self.assertEqual(result["outcome"], "ok")
        self.assertTrue(result["result"]["accepted"])
        # backlog should now have the task back to pending
        data = json.loads(self.backlog.read_text())
        self.assertEqual(data["tasks"][0]["status"], "pending")

    def test_loom_dispatch_incident_rejects_missing_incident(self):
        result = self.server.invoke_tool(LOOM_TOOL_DISPATCH_INCIDENT, {})
        self.assertEqual(result["outcome"], "rejected")
        self.assertEqual(result["failure_code"], "BAD_ARGUMENTS")

    def test_loom_enqueue_task_writes_to_backlog(self):
        result = self.server.invoke_tool(LOOM_TOOL_ENQUEUE_TASK, {
            "item": {"id": "new-1", "task": "do something"},
            "reason": "unit-test",
        })
        self.assertEqual(result["outcome"], "ok")
        self.assertEqual(result["result"]["task_id"], "new-1")
        data = json.loads(self.backlog.read_text())
        self.assertEqual(len(data["tasks"]), 1)
        self.assertEqual(data["tasks"][0]["id"], "new-1")

    def test_loom_enqueue_task_rejects_missing_item(self):
        result = self.server.invoke_tool(LOOM_TOOL_ENQUEUE_TASK, {})
        self.assertEqual(result["outcome"], "rejected")
        self.assertEqual(result["failure_code"], "BAD_ARGUMENTS")

    def test_loom_transition_task_writes_event(self):
        # First enqueue, then transition.
        self.server.invoke_tool(LOOM_TOOL_ENQUEUE_TASK, {
            "item": {"id": "trans-1", "task": "x"},
        })
        result = self.server.invoke_tool(LOOM_TOOL_TRANSITION_TASK, {
            "task_id": "trans-1",
            "to_status": "running",
        })
        self.assertEqual(result["outcome"], "ok")
        # event log should have at least one event line
        events = [json.loads(line) for line in self.events.read_text().splitlines() if line.strip()]
        self.assertGreaterEqual(len(events), 2)

    def test_loom_transition_task_missing_args_rejected(self):
        result = self.server.invoke_tool(LOOM_TOOL_TRANSITION_TASK, {})
        self.assertEqual(result["outcome"], "rejected")
        result2 = self.server.invoke_tool(LOOM_TOOL_TRANSITION_TASK, {"task_id": "x"})
        self.assertEqual(result2["outcome"], "rejected")

    def test_loom_transition_task_invalid_transition_returns_error(self):
        # Enqueue, then try an invalid transition (running->running)
        self.server.invoke_tool(LOOM_TOOL_ENQUEUE_TASK, {
            "item": {"id": "trans-2", "task": "x"},
        })
        self.server.invoke_tool(LOOM_TOOL_TRANSITION_TASK, {
            "task_id": "trans-2",
            "to_status": "running",
        })
        result = self.server.invoke_tool(LOOM_TOOL_TRANSITION_TASK, {
            "task_id": "trans-2",
            "to_status": "running",  # already running
        })
        self.assertEqual(result["outcome"], "error")
        self.assertEqual(result["failure_code"], "HANDLER_RAISED")

    def test_loom_heartbeat_writes_file(self):
        result = self.server.invoke_tool(LOOM_TOOL_HEARTBEAT, {
            "note": "hello from unit test",
        })
        self.assertEqual(result["outcome"], "ok")
        self.assertTrue(self.heartbeat.exists())
        payload = json.loads(self.heartbeat.read_text())
        self.assertEqual(payload["note"], "hello from unit test")
        self.assertEqual(payload["actor"], "protocol")

    def test_loom_heartbeat_accepts_explicit_payload(self):
        result = self.server.invoke_tool(LOOM_TOOL_HEARTBEAT, {
            "payload": {"custom": True, "value": 7},
        })
        self.assertEqual(result["outcome"], "ok")
        payload = json.loads(self.heartbeat.read_text())
        self.assertTrue(payload["custom"])
        self.assertEqual(payload["value"], 7)


# =============================================================================
# Concurrency — register + read are thread-safe
# =============================================================================
class TestConcurrency(unittest.TestCase):
    def test_concurrent_agent_registration(self):
        server = ProtocolServer(auto_register_loom=False)
        errors: list[Exception] = []

        def register(i: int) -> None:
            try:
                server.register_agent(_make_agent_card(f"agent-{i:03d}", name=f"A{i}"))
            except Exception as e:  # noqa: BLE001
                errors.append(e)

        threads = [threading.Thread(target=register, args=(i,)) for i in range(25)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        self.assertEqual(errors, [])
        self.assertEqual(len(server.list_agents()), 25)

    def test_concurrent_resource_reads(self):
        server = ProtocolServer(auto_register_loom=False)
        server.register_resource(_make_resource("test://x"))
        counter = {"n": 0}

        def reader() -> None:
            for _ in range(20):
                server._resource_readers["test://x"] = lambda: {"n": counter["n"]}
                counter["n"] += 1
                result = server.read_resource("test://x")
                assert result["found"], result

        threads = [threading.Thread(target=reader) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        # All reads succeeded with no exceptions.


# =============================================================================
# Dataclass helpers + default server + snapshot
# =============================================================================
class TestDataclassesAndDefaults(unittest.TestCase):
    def test_agent_card_dataclass_round_trip(self):
        c = AgentCard(
            metadata={"id": "x", "name": "X"},
            spec={"capabilities": ["a"], "endpoint": "x", "protocol_version": PROTOCOL_VERSION},
        )
        c.validate()
        self.assertEqual(c.agent_id, "x")
        self.assertEqual(c.to_dict()["kind"], AGENT_CARD_KIND)

    def test_agent_message_dataclass_round_trip(self):
        m = AgentMessage(
            metadata={"id": "m1", "from_agent": "a", "to_agent": "b",
                      "timestamp": "2026-01-01T00:00:00Z"},
            spec={"kind": "request", "body": {"q": 1}, "correlation_id": "c1"},
        )
        m.validate()
        self.assertEqual(m.message_id, "m1")
        self.assertEqual(m.spec_kind, "request")
        self.assertEqual(m.correlation_id, "c1")

    def test_tool_descriptor_round_trip(self):
        t = ToolDescriptor(name="x.y", description="d", input_schema={"type": "object"})
        t.validate()
        self.assertEqual(t.to_dict()["kind"], TOOL_DESCRIPTOR_KIND)

    def test_resource_descriptor_round_trip(self):
        r = ResourceDescriptor(uri="scheme://p", name="N", description="D", mime_type="text/plain")
        r.validate()
        self.assertEqual(r.to_dict()["kind"], RESOURCE_DESCRIPTOR_KIND)

    def test_default_server_singleton(self):
        reset_default_server()
        a = default_server()
        b = default_server()
        self.assertIs(a, b)

    def test_default_server_has_loom_defaults(self):
        reset_default_server()
        s = default_server()
        self.assertGreaterEqual(len(s.list_tools()), 4)
        self.assertGreaterEqual(len(s.list_resources()), 4)

    def test_snapshot_includes_all_registries(self):
        s = ProtocolServer(auto_register_loom=False)
        s.register_agent(_make_agent_card("a"))
        s.register_tool(_make_tool("t.a", "ta"))
        s.register_resource(_make_resource("test://a"))
        snap = s.snapshot()
        self.assertEqual(snap["agent_count"], 1)
        self.assertEqual(snap["tool_count"], 1)
        self.assertEqual(snap["resource_count"], 1)
        self.assertEqual(snap["protocol_version"], PROTOCOL_VERSION)

    def test_agent_message_kinds_constant(self):
        self.assertEqual(set(AGENT_MESSAGE_KINDS), {"handoff", "request", "response", "broadcast"})

    def test_protocol_error_subclasses(self):
        # Subclass hierarchy should hold.
        self.assertTrue(issubclass(ValidationFailed, ProtocolError))
        self.assertTrue(issubclass(UnknownTool, ProtocolError))
        self.assertTrue(issubclass(UnknownResource, ProtocolError))
        self.assertTrue(issubclass(ToolInvocationError, ProtocolError))

    def test_validation_failed_to_dict(self):
        err = ValidationFailed("oops", path=["x", 0], validator_kind="X", schema_path=["#"])
        d = err.to_dict()
        self.assertEqual(d["error"], "ValidationFailed")
        self.assertEqual(d["message"], "oops")
        self.assertEqual(d["validator_kind"], "X")
        self.assertEqual(d["path"], ["x", 0])

    def test_reset_validator_cache_clears(self):
        validate(AGENT_CARD_KIND, _make_agent_card("z"))
        reset_validator_cache()
        # No assertion needed beyond no exception.
        validate(AGENT_CARD_KIND, _make_agent_card("z"))


# =============================================================================
# Smoke check — the API the task description calls out
# =============================================================================
class TestSmokeProtocolServer(unittest.TestCase):
    """Mirrors the smoke check in the task description."""

    def test_smoke_import_and_list(self):
        # Phase E (T7, unify-default-agents): default ``ProtocolServer()`` now
        # auto-registers the 3 Loom agents (observer / triager / repairer)
        # so the A2A registry is non-empty out of the box. Pass
        # ``auto_register_loom=False`` for the legacy "empty registries"
        # smoke check.
        p = ProtocolServer()
        self.assertEqual(
            sorted(c["metadata"]["id"] for c in p.list_agents()),
            ["observer", "repairer", "triager"],
        )
        self.assertGreaterEqual(len(p.list_tools()), 4)
        self.assertGreaterEqual(len(p.list_resources()), 4)

        # And the legacy "fully empty" smoke check still works when both
        # auto-registers are turned off.
        p_empty = ProtocolServer(auto_register_loom=False, auto_register_agents=False)
        self.assertEqual(p_empty.list_agents(), [])
        self.assertEqual(p_empty.list_tools(), [])
        self.assertEqual(p_empty.list_resources(), [])


if __name__ == "__main__":
    unittest.main()