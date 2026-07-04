"""Tests for devkit/repairer.py (Phase A #3b) and incident schema runtime.

Coverage targets (>=25):
  - 5 whitelist actions x 5 tests = 25
  - dispatch x 5
  - schema x 5
  - integration (triager -> repairer) x 5
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

from devkit import repairer
from devkit import state_writer
from devkit import triager
from devkit.repairer import (
    Incident,
    RepairResult,
    WHITELIST_ACTIONS,
    INCIDENT_TO_ACTION,
    ACTION_REGISTRY,
    PROTOCOL_VERSION,
    get_validator,
)


# =============================================================================
# Helpers
# =============================================================================
def _write(p: pathlib.Path, content) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(content, (dict, list)):
        content = json.dumps(content, ensure_ascii=False)
    p.write_text(content, encoding="utf-8")


def _make_backlog(items: list[dict], path: pathlib.Path) -> None:
    """Write a backlog.json with the given task list."""
    _write(path, {"tasks": items})


def _make_running_task(task_id: str, lease: dict | None = None) -> dict:
    item = {
        "id": task_id,
        "status": "running",
        "priority": "high",
        "stages": "plan,implement,verify,review",
        "deps": [],
    }
    if lease is not None:
        item["lease"] = lease
    return item


def _make_pending_task(task_id: str) -> dict:
    return {
        "id": task_id,
        "status": "pending",
        "priority": "high",
        "stages": "plan,implement,verify,review",
        "deps": [],
    }


def _make_event_path(tmp: pathlib.Path) -> pathlib.Path:
    p = tmp / "events.jsonl"
    if not p.exists():
        p.write_text("", encoding="utf-8")
    return p


# =============================================================================
# Whitelist contract
# =============================================================================
class TestWhitelist(unittest.TestCase):
    def test_exactly_five_actions(self):
        self.assertEqual(len(WHITELIST_ACTIONS), 5)

    def test_whitelist_names(self):
        expected = {
            "reclaim_stale_running",
            "release_orphan_lease",
            "insert_repair_task",
            "throttle_carry",
            "mark_blocked",
        }
        self.assertEqual(set(WHITELIST_ACTIONS), expected)

    def test_dispatch_map_covers_all_actions(self):
        self.assertEqual(set(INCIDENT_TO_ACTION.values()), set(WHITELIST_ACTIONS))

    def test_action_registry_matches_whitelist(self):
        self.assertEqual(set(ACTION_REGISTRY.keys()), set(WHITELIST_ACTIONS))

    def test_dispatch_map_no_dupes(self):
        self.assertEqual(len(INCIDENT_TO_ACTION), len(set(INCIDENT_TO_ACTION.values())))


# =============================================================================
# reclaim_stale_running
# =============================================================================
class TestReclaimStaleRunning(unittest.TestCase):
    def test_running_to_pending(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-001", {"run_id": "L-1"})], backlog)
            event = _make_event_path(tmp)
            r = repairer.reclaim_stale_running("wi-001", "L-1", backlog_path=backlog, event_path=event)
            self.assertTrue(r.accepted)
            self.assertEqual(r.outcome, "applied")
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["status"], "pending")
            # Lease metadata should have been cleared
            self.assertNotIn("lease", data["tasks"][0])

    def test_missing_task_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_pending_task("wi-001")], backlog)
            event = _make_event_path(tmp)
            # wi-999 doesn't exist in the backlog at all
            r = repairer.reclaim_stale_running("wi-999", "L-1", backlog_path=backlog, event_path=event)
            self.assertFalse(r.accepted)
            self.assertEqual(r.outcome, "rejected")
            self.assertEqual(r.failure_code, "TASK_NOT_FOUND")

    def test_writes_event_log_entry(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            event = tmp / "events.jsonl"
            _make_backlog([_make_running_task("wi-002", {"run_id": "L-2"})], backlog)
            r = repairer.reclaim_stale_running("wi-002", "L-2", backlog_path=backlog, event_path=event)
            self.assertTrue(r.accepted)
            lines = [l for l in event.read_text().splitlines() if l.strip()]
            self.assertTrue(any("wi-002" in l for l in lines))

    def test_blank_inputs_raise(self):
        with self.assertRaises(ValueError):
            repairer.reclaim_stale_running("", "L-1")
        with self.assertRaises(ValueError):
            repairer.reclaim_stale_running("wid", "")

    def test_logs_warning_on_rejection(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_pending_task("wi-003")], backlog)
            event = _make_event_path(tmp)
            with self.assertLogs("devkit.repairer", level="WARNING") as cm:
                # wi-999 doesn't exist → transition_task rejects with TASK_NOT_FOUND
                # and the repairer logs a warning
                repairer.reclaim_stale_running("wi-999", "L-3", backlog_path=backlog, event_path=event)
            self.assertTrue(any("reclaim_stale_running rejected" in l for l in cm.output))


# =============================================================================
# release_orphan_lease
# =============================================================================
class TestReleaseOrphanLease(unittest.TestCase):
    def test_no_orphan_noop(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-001", {"run_id": "L-other"})], backlog)
            event = _make_event_path(tmp)
            r = repairer.release_orphan_lease("L-orphan", backlog_path=backlog, event_path=event)
            self.assertTrue(r.accepted)
            self.assertEqual(r.outcome, "noop")
            self.assertIn("noop", r.message)

    def test_attached_lease_cleared(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-001", {"run_id": "L-target"})], backlog)
            event = _make_event_path(tmp)
            r = repairer.release_orphan_lease("L-target", backlog_path=backlog, event_path=event)
            self.assertTrue(r.accepted)
            self.assertEqual(r.outcome, "applied")
            data = json.loads(backlog.read_text())
            self.assertNotIn("lease", data["tasks"][0])

    def test_blank_lease_id_raises(self):
        with self.assertRaises(ValueError):
            repairer.release_orphan_lease("")

    def test_does_not_change_status(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-001", {"run_id": "L-x"})], backlog)
            event = _make_event_path(tmp)
            repairer.release_orphan_lease("L-x", backlog_path=backlog, event_path=event)
            data = json.loads(backlog.read_text())
            # status remains running, only the lease is cleared
            self.assertEqual(data["tasks"][0]["status"], "running")

    def test_no_backlog_noop(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"  # doesn't exist
            r = repairer.release_orphan_lease("L-anything", backlog_path=backlog)
            self.assertTrue(r.accepted)
            self.assertEqual(r.outcome, "noop")


# =============================================================================
# insert_repair_task
# =============================================================================
class TestInsertRepairTask(unittest.TestCase):
    def test_basic_insert(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            event = _make_event_path(tmp)
            r = repairer.insert_repair_task(
                {"id": "wi-repair-1", "task": "fix the broken thing"},
                backlog_path=backlog,
                event_path=event,
            )
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(len(data["tasks"]), 1)
            self.assertEqual(data["tasks"][0]["status"], "pending")
            self.assertEqual(data["tasks"][0]["priority"], "high")
            self.assertEqual(data["tasks"][0]["id"], "wi-repair-1")

    def test_high_priority_forced(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            r = repairer.insert_repair_task(
                {"id": "wi-rep-2", "task": "low pri task", "priority": "low"},
                backlog_path=backlog,
            )
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["priority"], "high")

    def test_status_pending_forced(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            r = repairer.insert_repair_task(
                {"id": "wi-rep-3", "task": "running task?", "status": "running"},
                backlog_path=backlog,
            )
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["status"], "pending")

    def test_duplicate_id_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([{"id": "wi-dup", "status": "pending", "task": "existing"}], backlog)
            r = repairer.insert_repair_task(
                {"id": "wi-dup", "task": "duplicate"},
                backlog_path=backlog,
            )
            self.assertFalse(r.accepted)
            self.assertEqual(r.failure_code, "TASK_ALREADY_EXISTS")

    def test_missing_id_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            r = repairer.insert_repair_task(
                {"task": "no id"}, backlog_path=backlog,
            )
            self.assertFalse(r.accepted)
            self.assertEqual(r.failure_code, "TASK_ID_REQUIRED")


# =============================================================================
# throttle_carry
# =============================================================================
class TestThrottleCarry(unittest.TestCase):
    def test_writes_throttle_marker(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            tpath = tmp / "throttle.json"
            r = repairer.throttle_carry("loom-orchestrator", "review", throttle_path=tpath, duration_s=60)
            self.assertTrue(r.accepted)
            data = json.loads(tpath.read_text())
            self.assertIn("carriers", data)
            self.assertIn("loom-orchestrator", data["carriers"])
            self.assertEqual(data["carriers"]["loom-orchestrator"]["scope"], "review")

    def test_atomic_write(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            tpath = tmp / "throttle.json"
            repairer.throttle_carry("loom-dev", "implement", throttle_path=tpath, duration_s=10)
            # No .tmp leftover
            self.assertFalse((tmp / ".throttle.json.tmp").exists())

    def test_overwrites_existing(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            tpath = tmp / "throttle.json"
            repairer.throttle_carry("a", "plan", throttle_path=tpath, duration_s=5)
            repairer.throttle_carry("b", "review", throttle_path=tpath, duration_s=10)
            data = json.loads(tpath.read_text())
            self.assertIn("a", data["carriers"])
            self.assertIn("b", data["carriers"])

    def test_blank_inputs_raise(self):
        with tempfile.TemporaryDirectory() as td:
            tpath = pathlib.Path(td) / "throttle.json"
            with self.assertRaises(ValueError):
                repairer.throttle_carry("", "scope", throttle_path=tpath)
            with self.assertRaises(ValueError):
                repairer.throttle_carry("carrier", "", throttle_path=tpath)

    def test_non_positive_duration_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tpath = pathlib.Path(td) / "throttle.json"
            r = repairer.throttle_carry("a", "plan", throttle_path=tpath, duration_s=0)
            self.assertFalse(r.accepted)
            self.assertEqual(r.failure_code, "INVALID_DURATION")


# =============================================================================
# mark_blocked
# =============================================================================
class TestMarkBlocked(unittest.TestCase):
    def test_running_to_blocked(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-blk-1")], backlog)
            event = _make_event_path(tmp)
            r = repairer.mark_blocked("wi-blk-1", "human needed", backlog_path=backlog, event_path=event)
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["status"], "blocked")

    def test_writes_incident_log(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-blk-2")], backlog)
            inc_log = tmp / "incidents.jsonl"
            r = repairer.mark_blocked(
                "wi-blk-2", "stuck", backlog_path=backlog, incident_log=inc_log,
            )
            self.assertTrue(r.accepted)
            lines = [l for l in inc_log.read_text().splitlines() if l.strip()]
            self.assertEqual(len(lines), 1)
            entry = json.loads(lines[0])
            self.assertEqual(entry["work_item_id"], "wi-blk-2")
            self.assertEqual(entry["reason"], "stuck")

    def test_non_running_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_pending_task("wi-blk-3")], backlog)
            r = repairer.mark_blocked("wi-blk-3", "nope", backlog_path=backlog)
            self.assertFalse(r.accepted)
            self.assertIn("not allowed", r.message.lower())

    def test_blank_inputs_raise(self):
        with self.assertRaises(ValueError):
            repairer.mark_blocked("", "reason")
        with self.assertRaises(ValueError):
            repairer.mark_blocked("wid", "")

    def test_missing_task_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            r = repairer.mark_blocked("wi-missing", "x", backlog_path=backlog)
            self.assertFalse(r.accepted)
            self.assertEqual(r.failure_code, "TASK_NOT_FOUND")


# =============================================================================
# Dispatch (Incident -> whitelist action)
# =============================================================================
class TestDispatch(unittest.TestCase):
    def _inc(self, spec_kind: str, **overrides) -> Incident:
        payload = {
            "api_version": PROTOCOL_VERSION,
            "kind": "Incident",
            "metadata": {"id": f"inc-{spec_kind}", "work_item_id": "wi-disp-1"},
            "spec": {"kind": spec_kind, "severity": "high", "evidence_refs": []},
        }
        # Apply overrides
        for k, v in overrides.items():
            if k == "spec":
                payload["spec"].update(v)
            elif k == "metadata":
                payload["metadata"].update(v)
        return Incident(**payload)

    def test_dispatch_stale_running(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-disp-1")], backlog)
            r = repairer.dispatch(self._inc("stale_running"), backlog_path=backlog)
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["status"], "pending")

    def test_dispatch_repair_needed(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([], backlog)
            inc = self._inc(
                "repair_needed",
                spec={"task": {"id": "wi-new", "task": "do it"}, "kind": "repair_needed", "severity": "high"},
            )
            r = repairer.dispatch(inc, backlog_path=backlog)
            self.assertTrue(r.accepted)
            data = json.loads(backlog.read_text())
            self.assertEqual(len(data["tasks"]), 1)
            self.assertEqual(data["tasks"][0]["id"], "wi-new")

    def test_dispatch_throttle(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            tpath = tmp / "throttle.json"
            inc = self._inc(
                "carrier_throttle",
                spec={"carrier": "loom-orchestrator", "scope": "review", "duration_s": 30},
            )
            r = repairer.dispatch(inc, throttle_path=tpath)
            self.assertTrue(r.accepted)
            data = json.loads(tpath.read_text())
            self.assertIn("loom-orchestrator", data["carriers"])

    def test_dispatch_unknown_kind_rejected(self):
        inc = self._inc("rogue_action_not_whitelisted")
        r = repairer.dispatch(inc)
        self.assertFalse(r.accepted)
        self.assertEqual(r.failure_code, "NOT_ON_WHITELIST")
        self.assertIn("whitelist", r.message.lower())

    def test_dispatch_invalid_schema_rejected(self):
        bad = {
            "kind": "Incident",
            # missing required metadata.id
            "metadata": {},
            "spec": {"kind": "stale_running", "severity": "high"},
        }
        r = repairer.dispatch(bad)
        self.assertFalse(r.accepted)
        self.assertEqual(r.failure_code, "SCHEMA_VALIDATION_ERROR")


# =============================================================================
# Incident schema validation
# =============================================================================
class TestIncidentSchema(unittest.TestCase):
    def setUp(self):
        self.validator = get_validator()

    def test_minimal_valid_incident(self):
        inc = {
            "api_version": PROTOCOL_VERSION,
            "kind": "Incident",
            "metadata": {"id": "x"},
            "spec": {"kind": "stale_running", "severity": "high"},
        }
        self.assertIsNone(self.validator.validate(inc))  # no exception

    def test_missing_metadata_id_rejected(self):
        inc = {
            "api_version": PROTOCOL_VERSION,
            "kind": "Incident",
            "metadata": {},  # missing id
            "spec": {"kind": "x", "severity": "y"},
        }
        with self.assertRaises(Exception):
            self.validator.validate(inc)

    def test_missing_spec_kind_rejected(self):
        inc = {
            "api_version": PROTOCOL_VERSION,
            "kind": "Incident",
            "metadata": {"id": "x"},
            "spec": {"severity": "y"},  # missing kind
        }
        with self.assertRaises(Exception):
            self.validator.validate(inc)

    def test_wrong_api_version_rejected(self):
        inc = {
            "api_version": "wrong.version",
            "kind": "Incident",
            "metadata": {"id": "x"},
            "spec": {"kind": "x", "severity": "y"},
        }
        with self.assertRaises(Exception):
            self.validator.validate(inc)

    def test_incident_class_validates_self(self):
        inc = Incident(
            metadata={"id": "i-1", "work_item_id": "wi-x"},
            spec={"kind": "stale_running", "severity": "warn"},
        )
        # Should not raise
        inc.validate()

    def test_incident_class_invalid_raises(self):
        inc = Incident(metadata={}, spec={"kind": "x"})  # missing metadata.id
        with self.assertRaises(Exception):
            inc.validate()


# =============================================================================
# Triager -> incident conversion
# =============================================================================
class TestTriagerToIncident(unittest.TestCase):
    def test_finding_to_incident_basic(self):
        finding = triager.Finding(
            severity="warn",
            code="HEARTBEAT_STALE",
            message="heartbeat stale: 90s old",
            evidence={"heartbeat_age_s": 90},
        )
        inc = triager._to_incident(finding, snapshot_work_item_id="wi-x")
        self.assertEqual(inc["api_version"], PROTOCOL_VERSION)
        self.assertEqual(inc["kind"], "Incident")
        self.assertIn("id", inc["metadata"])
        self.assertEqual(inc["metadata"]["work_item_id"], "wi-x")
        self.assertEqual(inc["spec"]["kind"], "stale_running")
        self.assertEqual(inc["spec"]["severity"], "warn")
        # evidence_refs should contain a numeric ref
        self.assertTrue(any("heartbeat_age_s" in r for r in inc["spec"]["evidence_refs"]))

    def test_finding_without_work_item_uses_sentinel(self):
        finding = triager.Finding(severity="critical", code="QUARANTINED", message="q")
        inc = triager._to_incident(finding)
        self.assertEqual(inc["metadata"]["work_item_id"], triager.SYSTEM_WORK_ITEM_ID)

    def test_triage_report_includes_incidents(self):
        snap = triager.observer.ObserverSnapshot()  # all defaults
        rep = triager.triage(snap)
        # Should have at least one finding and one incident for the hard-dead case
        self.assertGreater(len(rep.findings), 0)
        self.assertEqual(len(rep.incidents), len(rep.findings))
        for inc in rep.incidents:
            get_validator().validate(inc)  # all conform to schema

    def test_incident_evidence_refs_skip_none(self):
        finding = triager.Finding(
            severity="critical",
            code="AUTOPILOT_NOT_STARTED",
            message="x",
            evidence={"supervisor_pid_file": None},  # str(None) -> "None"
        )
        inc = triager._to_incident(finding)
        # "None" placeholder should be filtered out
        for ref in inc["spec"]["evidence_refs"]:
            self.assertNotEqual(ref, "None")
            self.assertNotEqual(ref.strip(), "")

    def test_health_snapshot_zero_incidents(self):
        snap = triager.observer.ObserverSnapshot(
            supervisor=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=10.0,
            autopilot=triager.observer.AutopilotState(state="running", state_file_exists=True),
            backoff=triager.observer.BackoffState(),
        )
        rep = triager.triage(snap)
        self.assertEqual(rep.verdict, "HEALTHY")
        self.assertEqual(rep.incidents, [])


# =============================================================================
# Integration: triager -> repairer end-to-end
# =============================================================================
class TestTriagerRepairerIntegration(unittest.TestCase):
    def test_quarantined_dispatch_to_mark_blocked(self):
        # Build a snapshot that the triager will mark as QUARANTINED
        snap = triager.observer.ObserverSnapshot(
            supervisor=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=10.0,
            autopilot=triager.observer.AutopilotState(state="quarantined", state_file_exists=True),
        )
        rep = triager.triage(snap)
        # Should emit a "manual_block" incident (mapped from QUARANTINED)
        kinds = [inc["spec"]["kind"] for inc in rep.incidents]
        self.assertIn("manual_block", kinds)
        # And the incident should be schema-valid
        for inc in rep.incidents:
            get_validator().validate(inc)

    def test_heartbeat_dead_dispatch_to_stale_running(self):
        snap = triager.observer.ObserverSnapshot(
            supervisor=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=300.0,  # dead
        )
        rep = triager.triage(snap)
        kinds = [inc["spec"]["kind"] for inc in rep.incidents]
        self.assertIn("stale_running", kinds)

    def test_daemon_dead_dispatch_to_stale_running(self):
        snap = triager.observer.ObserverSnapshot(
            supervisor=triager.observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=triager.observer.ProcessState(pid=999_999_999, alive=False, pid_file_exists=True),
            autopilot=triager.observer.AutopilotState(state="running", state_file_exists=True),
        )
        rep = triager.triage(snap)
        kinds = [inc["spec"]["kind"] for inc in rep.incidents]
        self.assertIn("stale_running", kinds)

    def test_end_to_end_reclaim_stale_running(self):
        # Construct snapshot, triage, then feed the resulting incident to dispatch
        with tempfile.TemporaryDirectory() as td:
            tmp = pathlib.Path(td)
            backlog = tmp / "backlog.json"
            _make_backlog([_make_running_task("wi-e2e-1", {"run_id": "L-e2e"})], backlog)
            # Override the default observer paths so snapshot() reads our temp
            snap = triager.observer.snapshot(
                logs_dir=tmp / "logs",
                backlog_path=backlog,
            )
            # Force a stale-running scenario by setting heartbeat_age_s
            snap.heartbeat_age_s = 300.0
            rep = triager.triage(snap)
            # The first stale_running incident should dispatch cleanly
            stale = next(
                (i for i in rep.incidents if i["spec"]["kind"] == "stale_running"),
                None,
            )
            self.assertIsNotNone(stale, "expected a stale_running incident")
            # Dispatch it (override backlog path)
            result = repairer.dispatch(stale, backlog_path=backlog)
            self.assertTrue(result.accepted, msg=result.message)
            data = json.loads(backlog.read_text())
            self.assertEqual(data["tasks"][0]["status"], "pending")

    def test_repairer_does_not_import_rdloop_or_iterate(self):
        """Whitelist guarantee: repairer is decoupled from the execution kernel.

        We do a fresh import in an isolated module dict, then assert that
        neither devkit.rdloop nor devkit.iterate was pulled into sys.modules
        as a side effect of importing devkit.repairer.
        """
        import importlib
        importlib.invalidate_caches()
        # Snapshot current sys.modules (everything that was already loaded
        # via the test runner), then clear any devkit.rdloop / devkit.iterate
        # that may have been pulled in transitively.
        pre = set(sys.modules.keys())
        for mod in list(sys.modules):
            if mod == "devkit.rdloop" or mod == "devkit.iterate":
                sys.modules.pop(mod, None)
        # Now import repairer (it's already imported via the top-of-file import,
        # but importlib.reload forces a fresh load).
        importlib.reload(repairer)
        post = set(sys.modules.keys())
        new = post - pre
        # Neither forbidden module should have been added
        self.assertFalse(
            any(m == "devkit.rdloop" or m == "devkit.iterate" for m in new),
            f"repairer imported a forbidden module: {[m for m in new if 'rdloop' in m or 'iterate' in m]}",
        )


if __name__ == "__main__":
    unittest.main()
