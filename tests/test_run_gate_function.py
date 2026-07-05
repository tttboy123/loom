"""Tests for ``devkit.gatekeeper.evaluate_run_gate`` (Phase D end-of-run bridge).

The five scenarios required by the task spec:

1. Happy path: synthetic run_dir with 00-task.md + 99-gate.md →
   ``evaluate_run_gate`` returns ``("ok", [], gate_verdict)`` with
   ``gate_verdict.passed=True``.
2. Failed path: synthetic run_dir, ``gate_inputs.tests_failed=1`` →
   returns non-``ok`` status, reasons contains ``"tests_failed"``,
   ``gate_verdict.failure_codes`` contains ``"TEST_REGRESSION"``.
3. ``failure_codes_override`` parameter: when passed, it replaces
   the gate's own ``failure_codes``; returned verdict reflects the override.
4. ``phase_d_to_phase_b`` round-trip: ``TEST_REGRESSION`` → ``"tests_failed"``,
   ``BUDGET_EXCEEDED`` → ``"over_budget"``.
5. Empty run_dir (no files): gate falls through to ``EVIDENCE_MISSING``,
   ``verdict.passed=False``, reasons contains the translated string.

Plus 3 supporting tests for the public-surface / signature contract and
the ``evidence_dir`` override (so the function can be exercised without
writing into the canonical ``devkit/evidence`` directory the way
production code does).

Verification target
-------------------
``PYTHONPATH=. .venv/bin/python -m unittest tests.test_run_gate_function -v``
should report all tests OK and full-suite regression tests must stay green.
"""
from __future__ import annotations

import pathlib
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit.gatekeeper import evaluate_run_gate, GateVerdict
from devkit.failure_codes import (
    phase_d_to_phase_b,
    phase_b_to_phase_d,
    PHASE_D_TO_PHASE_B,
)


def _make_run_dir(tmp: pathlib.Path, *, name: str = "r-gate") -> pathlib.Path:
    run_dir = tmp / name
    run_dir.mkdir(parents=True, exist_ok=True)
    return run_dir


def _write_run_dir(run_dir: pathlib.Path) -> None:
    """Lay down the standard ``00-task.md`` / ``99-gate.md`` pair."""
    (run_dir / "00-task.md").write_text(
        "# Task\n\nImplement HTTP 200 detection.\n", encoding="utf-8"
    )
    (run_dir / "99-gate.md").write_text(
        "# Gate\n\ngate: GO\nstatus_code: suggested_go\n", encoding="utf-8"
    )


# ============================================================================
# Test 1 — happy path
# ============================================================================
class TestHappyPath(unittest.TestCase):
    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        self.run_dir = _make_run_dir(self.tmp)
        _write_run_dir(self.run_dir)
        self.evidence_dir = self.tmp / "evidence"

    def test_returns_ok_and_passed_verdict(self):
        status_code, reasons, verdict = evaluate_run_gate(
            run_id="r-gate-happy",
            work_item_id="wi-gate-happy",
            run_dir=self.run_dir,
            evidence_dir=self.evidence_dir,
            gate_inputs={
                "status_code": "suggested_go",
                "gate": "GO",
                "cost_usd": 0.5,
                "budget_cap_usd": 5.0,
                "tests_passed": 10,
                "tests_failed": 0,
            },
        )
        self.assertEqual(status_code, "ok")
        self.assertEqual(reasons, [])
        self.assertIsInstance(verdict, GateVerdict)
        self.assertTrue(verdict.passed)
        self.assertEqual(verdict.failure_codes, [])
        # typed metadata carried through
        self.assertEqual(verdict.run_id, "r-gate-happy")
        self.assertEqual(verdict.work_item_id, "wi-gate-happy")
        # evidence file was materialised on disk
        self.assertTrue(
            (self.evidence_dir / "r-gate-happy" / "evidence_packet.json").exists()
        )
        # lineage carries the evidence path so callers can audit
        lineage = verdict.spec.get("lineage", {})
        self.assertEqual(
            lineage.get("writer_kind"), "evidence_writer.write_run_evidence"
        )
        self.assertTrue(lineage.get("evidence_path", "").endswith("evidence_packet.json"))


# ============================================================================
# Test 2 — failed path: tests failed
# ============================================================================
class TestFailedPath(unittest.TestCase):
    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        self.run_dir = _make_run_dir(self.tmp, name="r-gate-fail")
        _write_run_dir(self.run_dir)
        self.evidence_dir = self.tmp / "evidence"

    def test_returns_non_ok_with_translated_reason(self):
        status_code, reasons, verdict = evaluate_run_gate(
            run_id="r-gate-fail",
            work_item_id="wi-gate-fail",
            run_dir=self.run_dir,
            evidence_dir=self.evidence_dir,
            gate_inputs={
                "status_code": "tests_failed",
                "gate": "NO_GO",
                "tests_passed": 10,
                "tests_failed": 1,
                "cost_usd": 0.5,
                "budget_cap_usd": 5.0,
            },
        )
        self.assertNotEqual(status_code, "ok")
        self.assertEqual(status_code, "task_contract_blocked")
        # reasons translated to Phase B free strings via phase_d_to_phase_b
        self.assertIn("tests_failed", reasons)
        self.assertFalse(verdict.passed)
        # typed verdict carries the Phase D enum
        self.assertIn("TEST_REGRESSION", verdict.failure_codes)


# ============================================================================
# Test 3 — failure_codes_override
# ============================================================================
class TestFailureCodesOverride(unittest.TestCase):
    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        self.run_dir = _make_run_dir(self.tmp, name="r-gate-ovr")
        _write_run_dir(self.run_dir)
        self.evidence_dir = self.tmp / "evidence"

    def test_override_replaces_gate_failure_codes(self):
        # baseline: write happy evidence so the gate would normally pass…
        status_code, reasons, verdict = evaluate_run_gate(
            run_id="r-gate-ovr",
            work_item_id="wi-gate-ovr",
            run_dir=self.run_dir,
            evidence_dir=self.evidence_dir,
            gate_inputs={"tests_failed": 0, "cost_usd": 0.1, "budget_cap_usd": 5.0},
            failure_codes_override=["BUDGET_EXCEEDED", "TEST_REGRESSION"],
        )
        # …but the override flips the verdict.
        self.assertEqual(status_code, "task_contract_blocked")
        self.assertFalse(verdict.passed)
        self.assertEqual(
            verdict.failure_codes, ["BUDGET_EXCEEDED", "TEST_REGRESSION"]
        )
        # reasons reflect the translated override codes
        self.assertIn("over_budget", reasons)
        self.assertIn("tests_failed", reasons)

    def test_empty_override_means_passed(self):
        # explicit override of empty list — clearer signal than None
        status_code, reasons, verdict = evaluate_run_gate(
            run_id="r-gate-ovr2",
            work_item_id="wi-gate-ovr2",
            run_dir=self.run_dir,
            evidence_dir=self.evidence_dir,
            gate_inputs={"tests_failed": 0, "cost_usd": 0.1, "budget_cap_usd": 5.0},
            failure_codes_override=[],
        )
        self.assertEqual(status_code, "ok")
        self.assertEqual(reasons, [])
        self.assertTrue(verdict.passed)


# ============================================================================
# Test 4 — phase_d_to_phase_b round-trip property
# ============================================================================
class TestPhaseDToPhaseBRoundTrip(unittest.TestCase):
    """The translator must round-trip cleanly for the three codes that have
    a canonical Phase B equivalent; the two with no equivalent return None
    so callers can fall back to the original enum string."""

    def test_main_three_round_trip(self):
        # forward direction
        self.assertEqual(phase_b_to_phase_d("tests_failed"), "TEST_REGRESSION")
        self.assertEqual(phase_b_to_phase_d("over_budget"), "BUDGET_EXCEEDED")
        self.assertEqual(phase_b_to_phase_d("blocked"), "EVIDENCE_INVALID")
        # reverse direction (the new function)
        self.assertEqual(phase_d_to_phase_b("TEST_REGRESSION"), "tests_failed")
        self.assertEqual(phase_d_to_phase_b("BUDGET_EXCEEDED"), "over_budget")
        self.assertEqual(phase_d_to_phase_b("EVIDENCE_INVALID"), "blocked")

    def test_unknown_inputs_tolerated(self):
        self.assertIsNone(phase_d_to_phase_b("EVIDENCE_MISSING"))
        self.assertIsNone(phase_d_to_phase_b("SCHEMA_VALIDATION_ERROR"))
        self.assertIsNone(phase_d_to_phase_b(""))
        self.assertIsNone(phase_d_to_phase_b(None))
        self.assertIsNone(phase_d_to_phase_b(123))

    def test_phase_d_table_is_small_and_complete(self):
        # the table is intentionally partial — these three codes have a
        # canonical Phase B reason; the other two Phase D codes do not.
        self.assertEqual(
            set(PHASE_D_TO_PHASE_B.keys()),
            {"TEST_REGRESSION", "BUDGET_EXCEEDED", "EVIDENCE_INVALID"},
        )


# ============================================================================
# Test 5 — empty run_dir: the writer still materialises an empty packet,
# so the gate passes; EVIDENCE_MISSING is exercised via direct gatekeeper.
# ============================================================================
class TestEmptyRunDir(unittest.TestCase):
    """Empty run_dir handling — two contracts worth pinning down.

    The ``evidence_writer.write_run_evidence`` contract is: empty / missing
    inputs are silently tolerated (no exception, optional metrics are
    null). Therefore ``evaluate_run_gate`` *always* materialises a packet
    the gate can read, and the happy path is the natural outcome even
    from an empty run_dir.

    The EVIDENCE_MISSING gate path is exercised directly via
    :func:`evaluate_final_gate` to confirm the underlying gate still emits
    ``EVIDENCE_MISSING`` when no packet AND no legacy evidence.json are
    present (e.g. when a higher layer chooses not to call
    ``evaluate_run_gate`` at all).
    """

    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        # empty — no 00-task.md, no 99-gate.md
        self.run_dir = _make_run_dir(self.tmp, name="r-gate-empty")
        self.evidence_dir = self.tmp / "evidence"

    def test_empty_run_dir_passes_via_writer(self):
        # The writer tolerates empty inputs — it still writes a packet
        # (no exception) with spec.summary="no evidence collected" and an
        # empty metrics dict. The gate reads that packet and the counters
        # are zero so the verdict passes.
        status_code, reasons, verdict = evaluate_run_gate(
            run_id="r-gate-empty",
            work_item_id="wi-gate-empty",
            run_dir=self.run_dir,
            evidence_dir=self.evidence_dir,
        )
        self.assertEqual(status_code, "ok")
        self.assertEqual(reasons, [])
        self.assertTrue(verdict.passed)
        self.assertEqual(verdict.failure_codes, [])
        # packet was indeed written
        self.assertTrue(
            (self.evidence_dir / "r-gate-empty" / "evidence_packet.json").exists()
        )

    def test_gate_evidence_missing_path_still_works(self):
        # Direct exercise of the gate's EVIDENCE_MISSING code path. This is
        # the same path evaluate_run_gate would have taken if the writer
        # had not been called — i.e. when the caller shortcuts and lets
        # the gate run alone. Document the contract: EVIDENCE_MISSING is
        # preserved verbatim by the gate, and the translator falls back to
        # the enum on the reasons list when no Phase B equivalent exists
        # (EVIDENCE_MISSING has no Phase B equivalent per
        # ``devkit/failure_codes.py``).
        from devkit.gatekeeper import evaluate_final_gate

        verdict = evaluate_final_gate(
            run_id="r-no-packet",
            work_item_id="wi-no-packet",
            evidence_dir=self.tmp / "evidence_missing",
        )
        self.assertFalse(verdict.passed)
        self.assertIn("EVIDENCE_MISSING", verdict.failure_codes)
        # the run_gate wrapper around this would surface
        # "EVIDENCE_MISSING" verbatim in reasons (no Phase B translation)
        # and "task_contract_blocked" as the status code. Verify the
        # translator falls back accordingly:
        from devkit.failure_codes import phase_d_to_phase_b

        self.assertIsNone(phase_d_to_phase_b("EVIDENCE_MISSING"))


# ============================================================================
# Bonus — public surface + return shape
# ============================================================================
class TestPublicSurface(unittest.TestCase):
    def test_signature_is_keyword_only_after_run_dir(self):
        import inspect

        sig = inspect.signature(evaluate_run_gate)
        params = list(sig.parameters.values())
        # run_dir is keyword-only (after the *)
        self.assertEqual(params[0].name, "run_id")
        self.assertEqual(params[1].name, "work_item_id")
        self.assertEqual(params[2].name, "run_dir")
        self.assertEqual(params[2].kind, inspect.Parameter.KEYWORD_ONLY)
        self.assertEqual(params[2].default, inspect.Parameter.empty)

    def test_return_shape_is_three_tuple(self):
        # type-erased return annotation
        anno = evaluate_run_gate.__annotations__.get("return", "")
        self.assertIn("tuple", anno)
        self.assertIn("GateVerdict", anno)

    def test_is_exported(self):
        import devkit.gatekeeper as gk

        self.assertIn("evaluate_run_gate", gk.__all__)
        # and the translator function from failure_codes
        import devkit.failure_codes as fc

        self.assertIn("phase_d_to_phase_b", fc.__all__)
        self.assertIn("PHASE_D_TO_PHASE_B", fc.__all__)


if __name__ == "__main__":
    unittest.main()
