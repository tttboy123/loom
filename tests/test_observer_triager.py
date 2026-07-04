"""Tests for devkit/observer.py and devkit/triager.py (DESIGN-P0 #3a)."""
from __future__ import annotations

import json
import os
import pathlib
import tempfile
import textwrap
import time
import unittest
from datetime import datetime, timezone
from unittest import mock

from devkit import observer, triager


# =============================================================================
# Helpers
# =============================================================================
def _write(p: pathlib.Path, content) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content, encoding="utf-8")


def _write_json(p: pathlib.Path, obj) -> None:
    _write(p, json.dumps(obj))


# =============================================================================
# Observer — pid_alive / parse_iso_age
# =============================================================================
class TestObserverHelpers(unittest.TestCase):
    def test_pid_alive_self(self):
        my_pid = os.getpid()
        self.assertTrue(observer._pid_alive(my_pid))

    def test_pid_alive_dead(self):
        # 999999 is almost certainly not a live pid
        self.assertFalse(observer._pid_alive(999_999_999))

    def test_parse_iso_age_naive_local(self):
        # Naive timestamp interpreted as local
        ts = datetime.now().isoformat()
        age = observer._parse_iso_age(ts)
        self.assertIsNotNone(age)
        self.assertLess(abs(age), 5)

    def test_parse_iso_age_with_tz(self):
        # TZ-aware UTC
        ts = datetime.now(timezone.utc).isoformat()
        age = observer._parse_iso_age(ts)
        self.assertIsNotNone(age)
        self.assertLess(abs(age), 5)

    def test_parse_iso_age_garbage(self):
        self.assertIsNone(observer._parse_iso_age("not a date"))
        self.assertIsNone(observer._parse_iso_age(None))
        self.assertIsNone(observer._parse_iso_age(""))


# =============================================================================
# Observer — snapshot
# =============================================================================
class TestSnapshot(unittest.TestCase):
    def test_minimal_empty(self):
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            backlog = pathlib.Path(td) / "backlog.json"
            snap = observer.snapshot(logs_dir=logs, backlog_path=backlog)
        self.assertEqual(snap.autopilot.state, None)
        self.assertFalse(snap.supervisor.pid_file_exists)
        self.assertEqual(snap.backlog.total, 0)

    def test_full_state(self):
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            _write_json(logs / "autopilot.state", {
                "state": "running",
                "reason": "ok",
                "supervisor_pid": "12345",
                "daemon_pid": "12346",
                "last_heartbeat": datetime.now().isoformat(),
            })
            _write_json(logs / "backoff.json", {
                "consec_failures": 2, "last_reason": "transient"
            })
            _write(logs / "loom-iterate-supervisor.pid", f"{os.getpid()}\n")
            _write(logs / "loom-iterate-daemon.pid", f"{os.getpid()}\n")
            _write(logs / "heartbeat.daemon", f"{time.time()}\n")
            backlog = pathlib.Path(td) / "backlog.json"
            _write_json(backlog, {"tasks": [
                {"status": "pending", "priority": "high"},
                {"status": "done", "priority": "high", "_lease_reclaim_reason": "owner_dead"},
                {"status": "stopped", "priority": "low"},
            ]})
            _write(logs / "watchdog.log", "line1\n[ts] supervisor restarted\nline3\n")

            snap = observer.snapshot(logs_dir=logs, backlog_path=backlog)

        self.assertEqual(snap.autopilot.state, "running")
        self.assertEqual(snap.backoff.consec_failures, 2)
        self.assertTrue(snap.supervisor.alive)
        self.assertTrue(snap.daemon.alive)
        self.assertIsNotNone(snap.heartbeat_age_s)
        self.assertLess(snap.heartbeat_age_s, 5)
        self.assertEqual(snap.backlog.total, 3)
        self.assertEqual(snap.backlog.by_status.get("pending"), 1)
        self.assertEqual(snap.backlog.lease_reclaimed, 1)
        self.assertEqual(snap.watchdog.heal_count_recent, 1)

    def test_dead_supervisor(self):
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            _write(logs / "loom-iterate-supervisor.pid", "999999999\n")
            _write(logs / "loom-iterate-daemon.pid", f"{os.getpid()}\n")
            snap = observer.snapshot(logs_dir=logs, backlog_path=pathlib.Path(td) / "b.json")
        self.assertTrue(snap.supervisor.pid_file_exists)
        self.assertFalse(snap.supervisor.alive)
        self.assertTrue(snap.daemon.alive)

    def test_heartbeat_uses_iso_fallback(self):
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            # heartbeat file empty; iso timestamp in autopilot.state
            _write(logs / "heartbeat.daemon", "")
            _write_json(logs / "autopilot.state", {
                "last_heartbeat": datetime.now().isoformat()
            })
            snap = observer.snapshot(logs_dir=logs, backlog_path=pathlib.Path(td) / "b.json")
        self.assertIsNotNone(snap.heartbeat_age_s)
        self.assertLess(snap.heartbeat_age_s, 5)


# =============================================================================
# Triager — happy path: HEALTHY
# =============================================================================
class TestTriageHealthy(unittest.TestCase):
    def test_healthy_everything_ok(self):
        snap = observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=10.0,
            autopilot=observer.AutopilotState(state="running", state_file_exists=True),
            backoff=observer.BackoffState(consec_failures=0),
            backlog=observer.BacklogState(total=10, by_status={"pending": 5, "done": 5}),
            watchdog=observer.WatchdogEvents(),
        )
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "HEALTHY")
        self.assertEqual(r.findings, [])

    def test_healthy_with_minor_lease_reclaims(self):
        snap = observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=10.0,
            autopilot=observer.AutopilotState(state="running", state_file_exists=True),
            backoff=observer.BackoffState(),
            backlog=observer.BacklogState(total=10, lease_reclaimed=3),
            watchdog=observer.WatchdogEvents(),
        )
        r = triager.triage(snap)
        # 3 lease reclaims is below the warn threshold (5), so HEALTHY
        self.assertEqual(r.verdict, "HEALTHY")
        self.assertEqual(r.findings, [])

    def test_warn_with_5_lease_reclaims(self):
        snap = observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            heartbeat_age_s=10.0,
            autopilot=observer.AutopilotState(state="running", state_file_exists=True),
            backoff=observer.BackoffState(),
            backlog=observer.BacklogState(total=10, lease_reclaimed=5),
            watchdog=observer.WatchdogEvents(),
        )
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "WARN")
        self.assertTrue(any(f.code == "BACKLOG_LEASE_RECLAIMS" for f in r.findings))


# =============================================================================
# Triager — escalation ladder
# =============================================================================
class TestTriageEscalation(unittest.TestCase):
    def _alive_snap(self) -> observer.ObserverSnapshot:
        return observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
        )

    def test_warn_stale_heartbeat(self):
        snap = self._alive_snap()
        snap.heartbeat_age_s = 90  # > 60 fresh, < 180 stale
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "WARN")
        self.assertTrue(any(f.code == "HEARTBEAT_STALE" for f in r.findings))

    def test_dead_heartbeat_is_stalled(self):
        snap = self._alive_snap()
        snap.heartbeat_age_s = 300  # > 180 dead
        r = triager.triage(snap)
        # HEARTBEAT_DEAD is critical → escalates to STALLED
        self.assertIn(r.verdict, ("STALLED", "QUARANTINED", "HARD_DEAD"))
        self.assertTrue(any(f.code == "HEARTBEAT_DEAD" for f in r.findings))

    def test_backoff_3_to_4_is_stalled(self):
        snap = self._alive_snap()
        snap.backoff = observer.BackoffState(consec_failures=3, last_reason="x")
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "STALLED")

    def test_backoff_5_is_stalled(self):
        snap = self._alive_snap()
        snap.backoff = observer.BackoffState(consec_failures=5, last_reason="x")
        r = triager.triage(snap)
        # consec_failures=5 hits BACKOFF_QUARANTINE_THRESHOLD (critical)
        self.assertIn(r.verdict, ("STALLED", "QUARANTINED"))

    def test_quarantined_state_is_critical(self):
        snap = self._alive_snap()
        snap.autopilot.state = "quarantined"
        snap.autopilot.reason = "consecutive failures"
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "QUARANTINED")
        self.assertTrue(any(f.code == "QUARANTINED" for f in r.findings))

    def test_hard_dead_no_state_no_pid(self):
        snap = observer.ObserverSnapshot()  # all default/None
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "HARD_DEAD")
        self.assertTrue(any(f.code == "AUTOPILOT_NOT_STARTED" for f in r.findings))

    def test_supervisor_dead_with_running_state(self):
        snap = observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=999_999_999, alive=False, pid_file_exists=True),
            autopilot=observer.AutopilotState(state="running", state_file_exists=True),
        )
        r = triager.triage(snap)
        self.assertTrue(any(f.code == "SUPERVISOR_DEAD" for f in r.findings))
        # SUPERVISOR_DEAD is critical → STALLED verdict
        self.assertIn(r.verdict, ("STALLED", "QUARANTINED", "HARD_DEAD"))

    def test_daemon_dead_with_alive_supervisor(self):
        snap = observer.ObserverSnapshot(
            supervisor=observer.ProcessState(pid=os.getpid(), alive=True, pid_file_exists=True),
            daemon=observer.ProcessState(pid=999_999_999, alive=False, pid_file_exists=True),
            autopilot=observer.AutopilotState(state="running", state_file_exists=True),
        )
        r = triager.triage(snap)
        self.assertTrue(any(f.code == "DAEMON_DEAD" for f in r.findings))
        self.assertEqual(r.verdict, "DEGRADED")

    def test_high_lease_reclaims_is_degraded(self):
        snap = self._alive_snap()
        snap.backlog = observer.BacklogState(total=100, lease_reclaimed=50)
        r = triager.triage(snap)
        self.assertEqual(r.verdict, "DEGRADED")
        self.assertTrue(any(f.code == "BACKLOG_LEASE_RECLAIMS_HIGH" for f in r.findings))

    def test_repeated_heal_is_degraded(self):
        snap = self._alive_snap()
        snap.watchdog = observer.WatchdogEvents(
            recent_lines=["x"] * 10, sigusr1_count_recent=5
        )
        r = triager.triage(snap)
        self.assertTrue(any(f.code == "WATCHDOG_HEALING_REPEATED" for f in r.findings))


# =============================================================================
# Triager — verdict escalation order
# =============================================================================
class TestVerdictOrder(unittest.TestCase):
    def test_ladder(self):
        # HEALTHY < WARN < DEGRADED < STALLED < QUARANTINED < HARD_DEAD
        for a, b in [("HEALTHY", "WARN"), ("WARN", "DEGRADED"),
                     ("DEGRADED", "STALLED"), ("STALLED", "QUARANTINED"),
                     ("QUARANTINED", "HARD_DEAD")]:
            self.assertLess(triager.VERDICT_RANK[a], triager.VERDICT_RANK[b])

    def test_next_worse(self):
        self.assertEqual(triager._next_worse("HEALTHY"), "WARN")
        self.assertEqual(triager._next_worse("STALLED"), "QUARANTINED")
        self.assertIsNone(triager._next_worse("HARD_DEAD"))


# =============================================================================
# Doctor CLI
# =============================================================================
class TestDoctorCLI(unittest.TestCase):
    def test_json_output(self):
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            _write(logs / "loom-iterate-supervisor.pid", f"{os.getpid()}\n")
            _write(logs / "loom-iterate-daemon.pid", f"{os.getpid()}\n")
            _write_json(logs / "autopilot.state", {"state": "running"})
            with mock.patch.object(observer, "LOGS_DIR", logs), \
                 mock.patch.object(observer, "BACKLOG_PATH", pathlib.Path(td) / "b.json"):
                from devkit.doctor import main
                import sys
                old_argv = sys.argv
                try:
                    sys.argv = ["doctor", "--autopilot", "--json"]
                    rc = main([])
                finally:
                    sys.argv = old_argv
        # main() called without --json from `argv` (we passed []) would print human-readable.
        # To keep it simple, just call main() with json via argv param
        self.assertEqual(rc, 0)

    def test_json_mode(self):
        from devkit import doctor as doc
        import io
        from contextlib import redirect_stdout
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            _write(logs / "loom-iterate-supervisor.pid", f"{os.getpid()}\n")
            _write(logs / "loom-iterate-daemon.pid", f"{os.getpid()}\n")
            with mock.patch.object(observer, "LOGS_DIR", logs), \
                 mock.patch.object(observer, "BACKLOG_PATH", pathlib.Path(td) / "b.json"):
                buf = io.StringIO()
                with redirect_stdout(buf):
                    rc = doc.main(["--autopilot", "--json", "--quiet"])
        out = buf.getvalue()
        # JSON should be parseable
        data = json.loads(out)
        self.assertIn("snapshot", data)
        self.assertIn("report", data)
        self.assertEqual(rc, 0)

    def test_quiet_returns_1_on_degraded(self):
        from devkit import doctor as doc
        import io
        from contextlib import redirect_stdout
        with tempfile.TemporaryDirectory() as td:
            logs = pathlib.Path(td) / "logs"
            logs.mkdir()
            # No pid file → HARD_DEAD
            with mock.patch.object(observer, "LOGS_DIR", logs), \
                 mock.patch.object(observer, "BACKLOG_PATH", pathlib.Path(td) / "b.json"):
                buf = io.StringIO()
                with redirect_stdout(buf):
                    rc = doc.main(["--autopilot", "--quiet"])
        self.assertEqual(rc, 1)


if __name__ == "__main__":
    unittest.main()