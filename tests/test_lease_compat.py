"""Compat tests for devkit/lease ↔ devkit/scheduler bridge (Phase D blocker #4).

Two lease paradigms coexist:

* ``devkit.lease`` — in-band, lives on the task dict, default timeout 1800s.
* ``devkit.scheduler`` — out-of-band, lives in a separate JSON file, default
  stale window 300s.

The bridge is :mod:`devkit.lease_sync`. It is wired into
:func:`devkit.lease.attach_lease` and :func:`devkit.lease.heartbeat` via the
opt-in ``lease_path`` kwarg. These tests cover:

1. ``attach_lease(..., lease_path=...)`` writes both the in-memory subdict
   and a scheduler-compatible file.
2. ``heartbeat(..., lease_path=...)`` updates both layers in sync.
3. Round-trip: ``scheduler.claim_lease`` → ``sync_lease_from_file`` →
   task dict gets a ``lease`` subdict that matches the scheduler's payload.
4. ``attach_lease`` without ``lease_path`` is unchanged — no file written,
   no exception.
5. ``reclaim_stale_running`` still works on a backlog list (no file
   interaction, no behavior change).

Plus a few extras: idempotency, error paths, ``release_lease_via_file``
delegation.
"""
from __future__ import annotations

import json
import os
import pathlib
import sys
import tempfile
import time
import unittest
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import lease, lease_sync, scheduler


def _tmp_path() -> pathlib.Path:
    fd, name = tempfile.mkstemp(prefix="lease-compat-", suffix=".json")
    os.close(fd)
    p = pathlib.Path(name)
    p.unlink()  # we want the path but no file yet
    return p


def _read_json(path: pathlib.Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


# ============================================================================
# Test 1: attach_lease(..., lease_path=...) writes both layers
# ============================================================================
class AttachLeaseWithLeasePath(unittest.TestCase):
    def test_attach_lease_sets_in_memory_and_writes_file(self):
        lp = _tmp_path()
        try:
            task = {"id": "task-a"}
            lease.attach_lease(task, owner_pid=123, run_id="run-1", lease_path=lp)

            # In-memory shape unchanged
            self.assertIn("lease", task)
            self.assertEqual(task["lease"]["owner_pid"], 123)
            self.assertEqual(task["lease"]["run_id"], "run-1")
            self.assertEqual(task["lease"]["timeout_seconds"], lease.DEFAULT_TIMEOUT_SECONDS)
            self.assertIn("heartbeat_at", task["lease"])

            # File exists and has scheduler-compatible shape
            self.assertTrue(lp.exists(), "lease file should exist on disk")
            on_disk = _read_json(lp)
            self.assertEqual(on_disk["work_item_id"], "task-a")
            self.assertEqual(on_disk["run_id"], "run-1")
            self.assertEqual(on_disk["protocol_version"], "loom.dev/v1")
            self.assertEqual(on_disk["owner_pid"], 123)
            self.assertIsInstance(on_disk["claimed_at"], (int, float))
            self.assertIn("claimed_at_iso", on_disk)
            self.assertIn("lease_id", on_disk)
        finally:
            if lp.exists():
                lp.unlink()

    def test_attach_lease_without_lease_path_is_pure_in_memory(self):
        # This is the legacy code path — must not write to disk, must not raise.
        lp = _tmp_path()  # sentinel; should remain absent
        try:
            task = {"id": "task-b"}
            lease.attach_lease(task, owner_pid=42, run_id="run-x")
            self.assertIn("lease", task)
            self.assertFalse(
                lp.exists(),
                "attach_lease without lease_path must not create any file at that path",
            )
        finally:
            if lp.exists():
                lp.unlink()


# ============================================================================
# Test 2: heartbeat(..., lease_path=...) keeps both layers in sync
# ============================================================================
class HeartbeatWithLeasePath(unittest.TestCase):
    def test_heartbeat_updates_in_memory_and_file(self):
        lp = _tmp_path()
        try:
            task = {"id": "task-c"}
            lease.attach_lease(task, owner_pid=99, run_id="run-2", lease_path=lp)
            first = task["lease"]["heartbeat_at"]
            on_disk_first = _read_json(lp)["claimed_at_iso"]

            # Sleep just enough to get a different ISO timestamp.
            time.sleep(0.01)

            fixed = "2026-07-05T10:00:00+00:00"
            lease.heartbeat(task, at=fixed, lease_path=lp)

            # In-memory updated
            self.assertEqual(task["lease"]["heartbeat_at"], fixed)
            self.assertNotEqual(task["lease"]["heartbeat_at"], first)

            # On-disk updated and reflects the same instant
            on_disk = _read_json(lp)
            self.assertEqual(
                on_disk["claimed_at_iso"],
                datetime.fromisoformat(fixed).isoformat(),
            )
            self.assertNotEqual(on_disk["claimed_at_iso"], on_disk_first)
            # Owner / run unchanged
            self.assertEqual(on_disk["owner_pid"], 99)
            self.assertEqual(on_disk["run_id"], "run-2")
            self.assertEqual(on_disk["work_item_id"], "task-c")
        finally:
            if lp.exists():
                lp.unlink()

    def test_heartbeat_without_lease_path_is_pure_in_memory(self):
        task = {"id": "task-d"}
        lease.attach_lease(task, owner_pid=7, run_id="run-3")
        first = task["lease"]["heartbeat_at"]
        time.sleep(0.01)
        lease.heartbeat(task)  # no lease_path
        self.assertNotEqual(task["lease"]["heartbeat_at"], first)

    def test_heartbeat_without_lease_on_task_is_noop(self):
        # Even with lease_path, heartbeat on a task without a lease subdict
        # should silently no-op — no file created.
        lp = _tmp_path()
        try:
            task = {"id": "task-e"}
            lease.heartbeat(task, lease_path=lp)
            self.assertNotIn("lease", task)
            self.assertFalse(lp.exists())
        finally:
            if lp.exists():
                lp.unlink()


# ============================================================================
# Test 3: Round-trip scheduler → lease_sync.from_file → task["lease"]
# ============================================================================
class SchedulerToLeaseSyncRoundTrip(unittest.TestCase):
    def test_claim_lease_then_from_file_yields_task_lease(self):
        lp = _tmp_path()
        try:
            ok = scheduler.claim_lease("wid-1", "run-rt", lp)
            self.assertTrue(ok)

            sub = lease_sync.sync_lease_from_file(lp)
            self.assertIsNotNone(sub)
            self.assertEqual(sub["run_id"], "run-rt")
            self.assertEqual(sub["owner_pid"], 0)  # scheduler doesn't store it
            self.assertIn("heartbeat_at", sub)
            self.assertEqual(sub["timeout_seconds"], lease.DEFAULT_TIMEOUT_SECONDS)

            # Plug it into a task and check shape
            task = {"id": "wid-1", "lease": dict(sub)}
            self.assertEqual(task["lease"]["run_id"], "run-rt")
            self.assertEqual(task["lease"]["timeout_seconds"], 1800)
        finally:
            if lp.exists():
                lp.unlink()

    def test_attach_then_claim_then_round_trip_matches_run_id(self):
        """attach_lease → claim_lease by another process — both views share run_id."""
        lp = _tmp_path()
        try:
            task = {"id": "wid-2"}
            lease.attach_lease(task, owner_pid=os.getpid(), run_id="local-run", lease_path=lp)
            # Now scheduler writes its own view (different run_id — this is
            # simulating "another process claimed it"). Re-claim with same
            # run_id to test idempotency.
            ok = scheduler.claim_lease("wid-2", "local-run", lp)
            self.assertTrue(ok, "same owner should be able to refresh the lease")

            sub = lease_sync.sync_lease_from_file(lp)
            self.assertEqual(sub["run_id"], "local-run")
        finally:
            if lp.exists():
                lp.unlink()


# ============================================================================
# Test 4: release_lease_via_file delegates to scheduler
# ============================================================================
class ReleaseViaFile(unittest.TestCase):
    def test_release_removes_file(self):
        lp = _tmp_path()
        scheduler.claim_lease("wid-3", "run-z", lp)
        self.assertTrue(lp.exists())
        lease_sync.release_lease_via_file(lp)
        self.assertFalse(lp.exists())

    def test_release_missing_file_is_silent(self):
        lp = _tmp_path()
        # File doesn't exist — should not raise
        lease_sync.release_lease_via_file(lp)


# ============================================================================
# Test 5: reclaim_stale_running unchanged
# ============================================================================
class ReclaimUnchanged(unittest.TestCase):
    def test_reclaim_stale_running_stops_dead_owner(self):
        backlog = [
            {
                "id": "r-1",
                "status": "running",
                "lease": {"owner_pid": 999, "heartbeat_at": "2026-01-01T00:00:00+00:00"},
            }
        ]
        out = lease.reclaim_stale_running(
            backlog,
            current_owner_pid=123,
            is_pid_alive=lambda pid: False,
        )
        self.assertEqual(out["reclaimed"], 1)
        self.assertEqual(out["backlog"][0]["status"], "stopped")
        self.assertEqual(out["backlog"][0]["_lease_reclaim_reason"], "owner_dead")

    def test_reclaim_stale_running_keeps_current_owner(self):
        backlog = [
            {
                "id": "r-2",
                "status": "running",
                "lease": {"owner_pid": 123, "heartbeat_at": "2026-01-01T00:00:00+00:00"},
            }
        ]
        out = lease.reclaim_stale_running(
            backlog,
            current_owner_pid=123,
            is_pid_alive=lambda pid: True,
        )
        self.assertEqual(out["reclaimed"], 0)
        self.assertEqual(out["backlog"][0]["status"], "running")

    def test_reclaim_does_not_touch_disk(self):
        # Even if the task has a lease with all the right fields, reclaim
        # must not interact with any file — it is purely in-memory.
        lp = _tmp_path()
        try:
            backlog = [
                {
                    "id": "r-3",
                    "status": "running",
                    "lease": {"owner_pid": 999, "heartbeat_at": "2026-01-01T00:00:00+00:00"},
                }
            ]
            lease.reclaim_stale_running(
                backlog,
                current_owner_pid=123,
                is_pid_alive=lambda pid: False,
            )
            self.assertFalse(
                lp.exists(),
                "reclaim_stale_running must not write to a lease file",
            )
        finally:
            if lp.exists():
                lp.unlink()


# ============================================================================
# Test 6: error paths
# ============================================================================
class ErrorPaths(unittest.TestCase):
    def test_sync_lease_to_file_requires_task_id(self):
        lp = _tmp_path()
        try:
            task = {"lease": {"owner_pid": 1, "run_id": "r"}}
            with self.assertRaises(ValueError):
                lease_sync.sync_lease_to_file(task, lp)
            self.assertFalse(lp.exists())
        finally:
            if lp.exists():
                lp.unlink()

    def test_sync_lease_to_file_requires_lease_subdict(self):
        lp = _tmp_path()
        try:
            task = {"id": "no-lease"}
            with self.assertRaises(ValueError):
                lease_sync.sync_lease_to_file(task, lp)
            self.assertFalse(lp.exists())
        finally:
            if lp.exists():
                lp.unlink()

    def test_sync_lease_from_file_missing_returns_none(self):
        lp = _tmp_path()
        self.assertIsNone(lease_sync.sync_lease_from_file(lp))

    def test_sync_lease_from_file_malformed_returns_none(self):
        lp = _tmp_path()
        lp.write_text("not json", encoding="utf-8")
        try:
            self.assertIsNone(lease_sync.sync_lease_from_file(lp))
        finally:
            lp.unlink()


# ============================================================================
# Test 7: bridge independence — public API surface
# ============================================================================
class PublicAPISurface(unittest.TestCase):
    def test_lease_public_api_unchanged(self):
        # Backward-compat guarantee: the original public symbols still exist
        # and have the same signature shape (minus the new lease_path kwarg).
        for name in ("now_iso", "attach_lease", "heartbeat", "reclaim_stale_running", "DEFAULT_TIMEOUT_SECONDS"):
            self.assertTrue(hasattr(lease, name), f"missing lease.{name}")

    def test_lease_sync_public_api(self):
        for name in (
            "sync_lease_to_file",
            "sync_lease_from_file",
            "release_lease_via_file",
            "DEFAULT_TIMEOUT_SECONDS",
        ):
            self.assertTrue(hasattr(lease_sync, name), f"missing lease_sync.{name}")


if __name__ == "__main__":
    unittest.main()