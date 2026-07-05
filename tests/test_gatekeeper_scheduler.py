"""Tests for devkit/gatekeeper.py and devkit/scheduler.py (Phase D).

Coverage targets (>=30):
  - classify_evidence_source (5: 4 enum + unknown + alias mapping)
  - evaluate_final_gate happy paths (3: inner_sandbox, materialized_repo, external_signal)
  - evaluate_final_gate failure paths (5: 4 failure_codes + happy explicit passed=True)
  - write_verdict / load_verdict round trip (3)
  - GateVerdict build constraints (3)
  - select_next_pending priority sort (3)
  - select_next_pending dep blocking (3)
  - select_next_pending lease blocking (2)
  - select_next_pending budget blocking (2)
  - select_next_pending missing backlog (2)
  - list_blocked (2)
  - claim_lease / release_lease / is_lease_stale (5)
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
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import gatekeeper
from devkit import scheduler
from devkit.gatekeeper import (
    GateVerdict,
    EVIDENCE_INNER_SANDBOX,
    EVIDENCE_MATERIALIZED_REPO,
    EVIDENCE_EXTERNAL_SIGNAL,
    EVIDENCE_UNKNOWN,
    FC_EVIDENCE_MISSING,
    FC_TEST_REGRESSION,
    FC_BUDGET_EXCEEDED,
    FC_EVIDENCE_INVALID,
    FC_SCHEMA_VALIDATION_ERROR,
    FAILURE_CODES,
    EVIDENCE_SOURCES,
    PROTOCOL_VERSION as GK_PROTOCOL_VERSION,
    SCHEMA_PATH as GK_SCHEMA_PATH,
    get_validator as gk_get_validator,
    reset_validator_cache as gk_reset_validator_cache,
)
from devkit.scheduler import (
    ScheduleDecision,
    PROTOCOL_VERSION as SCH_PROTOCOL_VERSION,
    DEFAULT_LEASE_MAX_AGE_S,
    select_next_pending,
    list_blocked,
    claim_lease,
    release_lease,
    is_lease_stale,
)


# ============================================================================
# Helpers
# ============================================================================
def _write(p: pathlib.Path, content) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(content, (dict, list)):
        content = json.dumps(content, ensure_ascii=False)
    p.write_text(content, encoding="utf-8")


def _make_evidence(
    *,
    tests_passed: int = 5,
    tests_failed: int = 0,
    cost_usd: float = 0.50,
    budget_cap_usd: float = 5.00,
    artifact_source: str | None = "inner_sandbox",
    include_artifact: bool = True,
    summary_extras: dict | None = None,
) -> dict:
    """Build a synthetic evidence.json payload."""
    summary = {
        "tests_passed": tests_passed,
        "tests_failed": tests_failed,
        "cost_usd": cost_usd,
        "budget_cap_usd": budget_cap_usd,
    }
    if summary_extras:
        summary.update(summary_extras)
    payload: dict = {"kind": "Evidence", "summary": summary}
    if include_artifact:
        payload["artifact_manifest"] = {"source": artifact_source}
    return payload


def _make_backlog(tasks: list[dict], path: pathlib.Path) -> None:
    """Write a bare-list backlog (Phase B fix-backlog-pending style)."""
    _write(path, tasks)


# ============================================================================
# Gatekeeper — classify_evidence_source
# ============================================================================
class TestClassifyEvidenceSource(unittest.TestCase):
    def test_inner_sandbox(self):
        self.assertEqual(
            gatekeeper.classify_evidence_source({"spec": {"source": "inner_sandbox"}}),
            EVIDENCE_INNER_SANDBOX,
        )

    def test_materialized_repo(self):
        self.assertEqual(
            gatekeeper.classify_evidence_source({"spec": {"source": "materialized_repo"}}),
            EVIDENCE_MATERIALIZED_REPO,
        )

    def test_external_signal(self):
        self.assertEqual(
            gatekeeper.classify_evidence_source({"spec": {"source": "external_signal"}}),
            EVIDENCE_EXTERNAL_SIGNAL,
        )

    def test_unknown_fallback(self):
        self.assertEqual(
            gatekeeper.classify_evidence_source({"spec": {"source": "banana"}}),
            EVIDENCE_UNKNOWN,
        )

    def test_missing_or_empty_returns_unknown(self):
        self.assertEqual(gatekeeper.classify_evidence_source({}), EVIDENCE_UNKNOWN)
        self.assertEqual(
            gatekeeper.classify_evidence_source({"spec": {}}),
            EVIDENCE_UNKNOWN,
        )
        self.assertEqual(gatekeeper.classify_evidence_source(None), EVIDENCE_UNKNOWN)
        # bogus non-dict input
        self.assertEqual(gatekeeper.classify_evidence_source("garbage"), EVIDENCE_UNKNOWN)
        self.assertEqual(gatekeeper.classify_evidence_source(42), EVIDENCE_UNKNOWN)


# ============================================================================
# Gatekeeper — evaluate_final_gate happy paths
# ============================================================================
class TestEvaluateFinalGateHappy(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-happy"
        self.run_dir.mkdir()

    def _write_evidence_with_source(self, source: str) -> None:
        ev = _make_evidence()
        ev["artifact_manifest"]["source"] = source
        ev["spec"] = {"source": source}
        _write(self.run_dir / "evidence.json", ev)

    def test_happy_inner_sandbox(self):
        self._write_evidence_with_source(EVIDENCE_INNER_SANDBOX)
        v = gatekeeper.evaluate_final_gate("r-happy", "wi-1", self.run_dir)
        self.assertTrue(v.passed)
        self.assertEqual(v.evidence_source, EVIDENCE_INNER_SANDBOX)
        self.assertEqual(v.failure_codes, [])
        self.assertEqual(v.reason, "all_gates_passed")

    def test_happy_materialized_repo(self):
        self._write_evidence_with_source(EVIDENCE_MATERIALIZED_REPO)
        v = gatekeeper.evaluate_final_gate("r-happy", "wi-2", self.run_dir)
        self.assertTrue(v.passed)
        self.assertEqual(v.evidence_source, EVIDENCE_MATERIALIZED_REPO)

    def test_happy_external_signal(self):
        self._write_evidence_with_source(EVIDENCE_EXTERNAL_SIGNAL)
        v = gatekeeper.evaluate_final_gate("r-happy", "wi-3", self.run_dir)
        self.assertTrue(v.passed)
        self.assertEqual(v.evidence_source, EVIDENCE_EXTERNAL_SIGNAL)


# ============================================================================
# Gatekeeper — evaluate_final_gate failure paths (one per failure_code)
# ============================================================================
class TestEvaluateFinalGateFailures(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-fail"
        self.run_dir.mkdir()

    def test_evidence_missing(self):
        # no evidence.json at all
        v = gatekeeper.evaluate_final_gate("r-fail", "wi-x", self.run_dir)
        self.assertFalse(v.passed)
        self.assertIn(FC_EVIDENCE_MISSING, v.failure_codes)
        self.assertEqual(v.evidence_source, EVIDENCE_UNKNOWN)

    def test_test_regression(self):
        _write(self.run_dir / "evidence.json", _make_evidence(tests_failed=2))
        v = gatekeeper.evaluate_final_gate("r-fail", "wi-x", self.run_dir)
        self.assertFalse(v.passed)
        self.assertIn(FC_TEST_REGRESSION, v.failure_codes)
        self.assertTrue(v.passed is False)

    def test_budget_exceeded(self):
        _write(
            self.run_dir / "evidence.json",
            _make_evidence(cost_usd=12.34, budget_cap_usd=5.00),
        )
        v = gatekeeper.evaluate_final_gate("r-fail", "wi-x", self.run_dir)
        self.assertFalse(v.passed)
        self.assertIn(FC_BUDGET_EXCEEDED, v.failure_codes)

    def test_evidence_invalid_artifact_source(self):
        _write(
            self.run_dir / "evidence.json",
            _make_evidence(artifact_source="banana"),
        )
        v = gatekeeper.evaluate_final_gate("r-fail", "wi-x", self.run_dir)
        self.assertFalse(v.passed)
        self.assertIn(FC_EVIDENCE_INVALID, v.failure_codes)

    def test_evidence_invalid_missing_artifact_source(self):
        _write(
            self.run_dir / "evidence.json",
            _make_evidence(artifact_source=""),
        )
        v = gatekeeper.evaluate_final_gate("r-fail", "wi-x", self.run_dir)
        self.assertFalse(v.passed)
        self.assertIn(FC_EVIDENCE_INVALID, v.failure_codes)


# ============================================================================
# Gatekeeper — verdict write/load round trip
# ============================================================================
class TestVerdictRoundTrip(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.tdir = pathlib.Path(self.tmp.name)

    def test_round_trip_pass(self):
        verdict = GateVerdict.build(
            run_id="r-rt",
            work_item_id="wi-rt",
            evidence_source=EVIDENCE_INNER_SANDBOX,
            passed=True,
            reason="all_gates_passed",
            failure_codes=[],
        )
        path = self.tdir / "verdict.json"
        gatekeeper.write_verdict(verdict, path)
        self.assertTrue(path.exists())
        loaded = gatekeeper.load_verdict(path)
        self.assertIsNotNone(loaded)
        self.assertEqual(loaded.work_item_id, "wi-rt")
        self.assertEqual(loaded.run_id, "r-rt")
        self.assertTrue(loaded.passed)
        self.assertEqual(loaded.evidence_source, EVIDENCE_INNER_SANDBOX)
        self.assertEqual(loaded.failure_codes, [])

    def test_round_trip_fail(self):
        verdict = GateVerdict.build(
            run_id="r-fail",
            work_item_id="wi-x",
            evidence_source=EVIDENCE_MATERIALIZED_REPO,
            passed=False,
            reason="tests failed",
            failure_codes=[FC_TEST_REGRESSION, FC_BUDGET_EXCEEDED],
        )
        path = self.tdir / "verdict.json"
        gatekeeper.write_verdict(verdict, path)
        loaded = gatekeeper.load_verdict(path)
        self.assertIsNotNone(loaded)
        self.assertFalse(loaded.passed)
        self.assertEqual(set(loaded.failure_codes), {FC_TEST_REGRESSION, FC_BUDGET_EXCEEDED})

    def test_load_missing_returns_none(self):
        self.assertIsNone(gatekeeper.load_verdict(self.tdir / "no-such.json"))

    def test_load_malformed_returns_none(self):
        path = self.tdir / "bad.json"
        path.write_text("{not valid json", encoding="utf-8")
        self.assertIsNone(gatekeeper.load_verdict(path))


# ============================================================================
# Gatekeeper — GateVerdict build constraints
# ============================================================================
class TestGateVerdictBuild(unittest.TestCase):
    def test_passed_true_requires_empty_failure_codes(self):
        with self.assertRaises(ValueError):
            GateVerdict.build(
                run_id="r",
                work_item_id="w",
                evidence_source=EVIDENCE_INNER_SANDBOX,
                passed=True,
                reason="x",
                failure_codes=[FC_TEST_REGRESSION],
            )

    def test_passed_false_requires_codes(self):
        with self.assertRaises(ValueError):
            GateVerdict.build(
                run_id="r",
                work_item_id="w",
                evidence_source=EVIDENCE_INNER_SANDBOX,
                passed=False,
                reason="x",
                failure_codes=[],
            )

    def test_unknown_failure_code_rejected(self):
        with self.assertRaises(ValueError):
            GateVerdict.build(
                run_id="r",
                work_item_id="w",
                evidence_source=EVIDENCE_INNER_SANDBOX,
                passed=False,
                reason="x",
                failure_codes=["NOT_A_CODE"],
            )

    def test_invalid_evidence_source_rejected(self):
        with self.assertRaises(ValueError):
            GateVerdict.build(
                run_id="r",
                work_item_id="w",
                evidence_source="banana",
                passed=False,
                reason="x",
                failure_codes=[FC_EVIDENCE_MISSING],
            )

    def test_constants_match_schema_enum(self):
        # schema enum should match what the dataclass / API accept
        self.assertEqual(
            set(EVIDENCE_SOURCES),
            {EVIDENCE_INNER_SANDBOX, EVIDENCE_MATERIALIZED_REPO,
             EVIDENCE_EXTERNAL_SIGNAL, EVIDENCE_UNKNOWN},
        )
        self.assertEqual(
            set(FAILURE_CODES),
            {FC_EVIDENCE_MISSING, FC_TEST_REGRESSION, FC_BUDGET_EXCEEDED,
             FC_EVIDENCE_INVALID, FC_SCHEMA_VALIDATION_ERROR},
        )

    def test_schema_validator_accepts_build(self):
        verdict = GateVerdict.build(
            run_id="r",
            work_item_id="w",
            evidence_source=EVIDENCE_INNER_SANDBOX,
            passed=True,
            reason="all_gates_passed",
        )
        # must not raise
        gk_get_validator().validate(verdict.to_dict())


# ============================================================================
# Scheduler — select_next_pending priority sort
# ============================================================================
class TestSchedulerPrioritySort(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"

    def test_high_chosen_first(self):
        _make_backlog(
            [
                {"id": "low-1", "status": "pending", "priority": "low", "deps": []},
                {"id": "high-1", "status": "pending", "priority": "high", "deps": []},
                {"id": "med-1", "status": "pending", "priority": "medium", "deps": []},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        self.assertIsNotNone(d)
        self.assertEqual(d.work_item_id, "high-1")
        self.assertTrue(d.is_actionable())
        self.assertEqual(d.priority, "high")

    def test_medium_beats_low(self):
        _make_backlog(
            [
                {"id": "low-1", "status": "pending", "priority": "low", "deps": []},
                {"id": "med-1", "status": "pending", "priority": "medium", "deps": []},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        self.assertEqual(d.work_item_id, "med-1")

    def test_tie_breaks_on_id_stable_order(self):
        _make_backlog(
            [
                {"id": "high-b", "status": "pending", "priority": "high", "deps": []},
                {"id": "high-a", "status": "pending", "priority": "high", "deps": []},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        # priority rank is the same, fall back to id lexicographic order
        self.assertEqual(d.work_item_id, "high-a")


# ============================================================================
# Scheduler — select_next_pending dep blocking
# ============================================================================
class TestSchedulerDepBlocking(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"

    def test_dep_pending_blocks(self):
        _make_backlog(
            [
                {"id": "dep-1", "status": "pending", "priority": "high", "deps": []},
                {"id": "child-1", "status": "pending", "priority": "high",
                 "deps": ["dep-1"]},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        self.assertEqual(d.work_item_id, "dep-1")
        # child-1 is blocked by dep-1 → not picked
        self.assertNotEqual(d.work_item_id, "child-1")

    def test_dep_done_unblocks(self):
        _make_backlog(
            [
                {"id": "dep-1", "status": "done", "priority": "high", "deps": []},
                {"id": "child-1", "status": "pending", "priority": "high",
                 "deps": ["dep-1"]},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        self.assertEqual(d.work_item_id, "child-1")

    def test_dep_skipped_unblocks(self):
        _make_backlog(
            [
                {"id": "dep-1", "status": "skipped", "priority": "high", "deps": []},
                {"id": "child-1", "status": "pending", "priority": "high",
                 "deps": ["dep-1"]},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        self.assertEqual(d.work_item_id, "child-1")

    def test_dep_failed_still_blocks(self):
        # failed is NOT in DEP_SATISFIED_STATUSES → child stays blocked
        _make_backlog(
            [
                {"id": "dep-1", "status": "failed", "priority": "high", "deps": []},
                {"id": "child-1", "status": "pending", "priority": "high",
                 "deps": ["dep-1"]},
                {"id": "independent", "status": "pending", "priority": "medium",
                 "deps": []},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp)
        # dep-1 is failed (not pending) and child-1 is blocked → only
        # 'independent' is selectable.
        self.assertIsNotNone(d)
        self.assertEqual(d.work_item_id, "independent")
        blocked = list_blocked(self.bp)
        blocked_ids = {b.work_item_id for b in blocked}
        self.assertIn("child-1", blocked_ids)
        self.assertEqual(blocked[0].blocked_by, ["dep-1"])


# ============================================================================
# Scheduler — select_next_pending lease blocking
# ============================================================================
class TestSchedulerLeaseBlocking(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"
        self.lp = pathlib.Path(self.tmp.name) / "lease.json"

    def test_leased_work_item_excluded(self):
        _make_backlog(
            [
                {"id": "a", "status": "pending", "priority": "high", "deps": []},
                {"id": "b", "status": "pending", "priority": "high", "deps": []},
            ],
            self.bp,
        )
        # claim a's lease with a fresh timestamp so it's not stale
        self.assertTrue(claim_lease("a", "run-x", self.lp))
        d = select_next_pending(self.bp, self.lp)
        # a is leased → scheduler must skip it
        self.assertIsNotNone(d)
        self.assertEqual(d.work_item_id, "b")

    def test_stale_lease_does_not_block(self):
        _make_backlog(
            [
                {"id": "a", "status": "pending", "priority": "high", "deps": []},
                {"id": "b", "status": "pending", "priority": "medium", "deps": []},
            ],
            self.bp,
        )
        # write a lease whose claimed_at is far in the past
        old_payload = {
            "work_item_id": "a",
            "run_id": "run-old",
            "claimed_at": time.time() - 999,
            "claimed_at_iso": "2020-01-01T00:00:00+00:00",
        }
        _write(self.lp, old_payload)
        self.assertTrue(is_lease_stale(self.lp))
        d = select_next_pending(self.bp, self.lp)
        self.assertEqual(d.work_item_id, "a")


# ============================================================================
# Scheduler — select_next_pending budget blocking
# ============================================================================
class TestSchedulerBudgetBlocking(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"

    def test_over_budget_excluded(self):
        _make_backlog(
            [
                {"id": "cheap", "status": "pending", "priority": "low",
                 "deps": [], "budget_usd": 0.5},
                {"id": "expensive", "status": "pending", "priority": "high",
                 "deps": [], "budget_usd": 999.0},
            ],
            self.bp,
        )
        d = select_next_pending(self.bp, budget_cap_usd=1.0)
        self.assertEqual(d.work_item_id, "cheap")

    def test_zero_cap_excludes_everything(self):
        _make_backlog(
            [
                {"id": "x", "status": "pending", "priority": "high",
                 "deps": [], "budget_usd": 0.5},
            ],
            self.bp,
        )
        # cap_usd=0.0 means "no work" — even a $0.50 task is rejected
        d = select_next_pending(self.bp, budget_cap_usd=0.0)
        self.assertIsNone(d)


# ============================================================================
# Scheduler — missing / empty backlog
# ============================================================================
class TestSchedulerBacklogMissing(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def test_missing_backlog_returns_none(self):
        bp = pathlib.Path(self.tmp.name) / "no-such.json"
        self.assertIsNone(select_next_pending(bp))

    def test_empty_backlog_returns_none(self):
        bp = pathlib.Path(self.tmp.name) / "empty.json"
        _write(bp, [])
        self.assertIsNone(select_next_pending(bp))
        self.assertEqual(list_blocked(bp), [])

    def test_no_pending_returns_none(self):
        bp = pathlib.Path(self.tmp.name) / "all-done.json"
        _make_backlog(
            [
                {"id": "x", "status": "done", "priority": "high", "deps": []},
                {"id": "y", "status": "running", "priority": "high", "deps": []},
            ],
            bp,
        )
        self.assertIsNone(select_next_pending(bp))


# ============================================================================
# Scheduler — list_blocked
# ============================================================================
class TestSchedulerListBlocked(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"
        self.lp = pathlib.Path(self.tmp.name) / "lease.json"

    def test_blocked_lists_deps_lease_budget(self):
        _make_backlog(
            [
                {"id": "ready", "status": "pending", "priority": "high", "deps": []},
                {"id": "by-dep", "status": "pending", "priority": "medium",
                 "deps": ["ready"]},
                {"id": "by-budget", "status": "pending", "priority": "medium",
                 "deps": [], "budget_usd": 99.0},
            ],
            self.bp,
        )
        blocked = list_blocked(self.bp, budget_cap_usd=1.0)
        by_id = {b.work_item_id: b for b in blocked}
        self.assertIn("by-dep", by_id)
        self.assertIn("by-budget", by_id)
        self.assertEqual(by_id["by-dep"].reason, "blocked")
        self.assertIn("ready", by_id["by-dep"].blocked_by)
        self.assertEqual(by_id["by-budget"].reason, "over_budget")

    def test_list_blocked_includes_leased(self):
        _make_backlog(
            [
                {"id": "a", "status": "pending", "priority": "high", "deps": []},
            ],
            self.bp,
        )
        self.assertTrue(claim_lease("a", "run-x", self.lp))
        blocked = list_blocked(self.bp, self.lp)
        self.assertEqual(len(blocked), 1)
        self.assertEqual(blocked[0].work_item_id, "a")
        self.assertEqual(blocked[0].reason, "leased")


# ============================================================================
# Scheduler — claim_lease / release_lease / is_lease_stale
# ============================================================================
class TestSchedulerLeaseLifecycle(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.lp = pathlib.Path(self.tmp.name) / "lease.json"

    def test_claim_succeeds_when_no_lease(self):
        self.assertTrue(claim_lease("a", "run-1", self.lp))
        self.assertTrue(self.lp.exists())

    def test_claim_refused_when_active_lease_held_by_other(self):
        self.assertTrue(claim_lease("a", "run-1", self.lp))
        self.assertFalse(claim_lease("b", "run-2", self.lp))

    def test_claim_succeeds_after_release(self):
        self.assertTrue(claim_lease("a", "run-1", self.lp))
        release_lease(self.lp)
        self.assertFalse(self.lp.exists())
        self.assertTrue(claim_lease("b", "run-2", self.lp))

    def test_idempotent_re_claim_by_same_owner(self):
        self.assertTrue(claim_lease("a", "run-1", self.lp))
        # same owner re-claiming refreshes timestamp and still returns True
        self.assertTrue(claim_lease("a", "run-1", self.lp))

    def test_stale_lease_can_be_stolen(self):
        # write a clearly-stale lease directly
        payload = {
            "work_item_id": "a",
            "run_id": "run-old",
            "claimed_at": time.time() - 9999,
        }
        self.lp.parent.mkdir(parents=True, exist_ok=True)
        _write(self.lp, payload)
        self.assertTrue(is_lease_stale(self.lp))
        # different owner can steal a stale lease
        self.assertTrue(claim_lease("b", "run-new", self.lp))

    def test_release_missing_file_is_noop(self):
        # file does not exist → no raise
        release_lease(self.lp)

    def test_is_lease_stale_missing_file(self):
        self.assertTrue(is_lease_stale(self.lp))

    def test_is_lease_stale_fresh(self):
        claim_lease("a", "run-1", self.lp)
        self.assertFalse(is_lease_stale(self.lp))

    def test_is_lease_stale_respects_max_age(self):
        # write a lease that's only 1s old; max_age=0 ⇒ stale immediately
        payload = {"claimed_at": time.time() - 1.0}
        _write(self.lp, payload)
        self.assertTrue(is_lease_stale(self.lp, max_age_s=0))


# ============================================================================
# Scheduler — ScheduleDecision dataclass
# ============================================================================
class TestScheduleDecision(unittest.TestCase):
    def test_is_actionable_only_when_ready(self):
        ready = ScheduleDecision(work_item_id="a", reason="ready")
        self.assertTrue(ready.is_actionable())
        blocked = ScheduleDecision(work_item_id="a", reason="blocked", blocked_by=["b"])
        self.assertFalse(blocked.is_actionable())
        empty = ScheduleDecision()
        self.assertFalse(empty.is_actionable())

    def test_round_trip_dict(self):
        d = ScheduleDecision(
            work_item_id="x",
            reason="blocked",
            blocked_by=["y", "z"],
            estimated_cost_usd=1.23,
            priority="high",
        )
        as_dict = d.to_dict()
        again = ScheduleDecision.from_dict(as_dict)
        self.assertEqual(again.work_item_id, "x")
        self.assertEqual(again.reason, "blocked")
        self.assertEqual(again.blocked_by, ["y", "z"])
        self.assertAlmostEqual(again.estimated_cost_usd, 1.23)
        self.assertEqual(again.priority, "high")

    def test_priority_normalized_to_known(self):
        d = ScheduleDecision(priority="URGENT")  # unknown → medium
        self.assertEqual(d.priority, "medium")


# ============================================================================
# Cross-module integration smoke
# ============================================================================
class TestIntegrationSmoke(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-int"
        self.run_dir.mkdir()
        self.bp = pathlib.Path(self.tmp.name) / "backlog.json"
        self.lp = pathlib.Path(self.tmp.name) / "lease.json"

    def test_scheduler_then_gatekeeper(self):
        # 1) scheduler picks the only ready task
        _make_backlog(
            [{"id": "wi-int", "status": "pending", "priority": "high", "deps": []}],
            self.bp,
        )
        decision = select_next_pending(self.bp, self.lp)
        self.assertIsNotNone(decision)
        self.assertTrue(claim_lease(decision.work_item_id, "run-int", self.lp))

        # 2) gatekeeper evaluates the run's evidence
        _write(self.run_dir / "evidence.json", _make_evidence(tests_failed=1))
        verdict = gatekeeper.evaluate_final_gate(
            "run-int", decision.work_item_id, self.run_dir
        )
        self.assertFalse(verdict.passed)
        self.assertIn(FC_TEST_REGRESSION, verdict.failure_codes)

        # 3) verdict survives a round trip
        vp = self.run_dir / "verdict.json"
        gatekeeper.write_verdict(verdict, vp)
        loaded = gatekeeper.load_verdict(vp)
        self.assertIsNotNone(loaded)
        self.assertEqual(loaded.work_item_id, decision.work_item_id)


if __name__ == "__main__":
    unittest.main()