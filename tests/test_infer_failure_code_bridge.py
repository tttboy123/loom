"""Tests for devkit/iterate.py — Phase F: infer_failure_code GateVerdict bridge.

These tests cover the Phase F change that lets ``infer_failure_code``
consume a typed ``gate_verdict`` (Phase D GateVerdict) instead of only
scraping free-text run logs.

Spec note
---------

The Phase F task spec lists test inputs using CamelCase names
(``TestFail`` / ``BudgetExceed`` / ``CompileFail``) which the spec
treats as canonical repairer codes. The actual Loom codebase uses
SCREAMING_SNAKE Phase A repairer codes (e.g.
``SCHEMA_VALIDATION_ERROR``, ``BUDGET_EXCEEDED``, ``NOT_ON_WHITELIST``)
— see :data:`devkit.failure_codes.PHASE_A_REPAIRER_CODES` and
:data:`devkit.failure_codes.PHASE_D_TO_PHASE_A`. The Phase F contract
explicitly forbids introducing new failure-code strings, so these
tests use the **actual** Phase A codes (SCREAMING_SNAKE) for the
text-path assertions and rely on
:func:`devkit.failure_codes.phase_d_to_phase_a` for the verdict-path
assertions. The deliverable.md documents this divergence.
"""
from __future__ import annotations

import pathlib
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import failure_codes as fc  # noqa: E402
from devkit import iterate  # noqa: E402


class TestBackwardCompatTextPath(unittest.TestCase):
    """Old behavior — when gate_verdict is None, scan free-text only."""

    def test_old_path_scrapes_phase_a_code_from_text(self):
        """infer_failure_code('SCHEMA_VALIDATION_ERROR happened') → SCHEMA_VALIDATION_ERROR.

        Old regex path is preserved verbatim. The token in the text is
        the *actual* Phase A repairer code (SCREAMING_SNAKE), which the
        regex matches.
        """
        self.assertEqual(
            iterate.infer_failure_code("SCHEMA_VALIDATION_ERROR happened"),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_old_path_returns_empty_when_nothing_matches(self):
        """infer_failure_code('nothing here') → '' (no code, no verdict)."""
        self.assertEqual(iterate.infer_failure_code("nothing here"), "")

    def test_old_path_multiple_texts_returns_first_match(self):
        """Iterates texts in order, returns first regex match per legacy semantics."""
        self.assertEqual(
            iterate.infer_failure_code(
                "no code here",
                "BUDGET_EXCEEDED in second text",
                "NOT_ON_WHITELIST in third",
            ),
            "BUDGET_EXCEEDED",
        )

    def test_old_path_excludes_meta_tokens(self):
        """REQUEST / CHANGES / STALLED_NO_READY_TASK are skipped (legacy exclusion set)."""
        self.assertEqual(
            iterate.infer_failure_code("REQUEST was sent, STALLED_NO_READY_TASK at idle"),
            "",  # both excluded; nothing else matches
        )


class TestVerdictPath(unittest.TestCase):
    """New path — when gate_verdict is provided, Phase D code wins."""

    def test_test_regression_translates_to_phase_a(self):
        """TEST_REGRESSION verdict → SCHEMA_VALIDATION_ERROR (via phase_d_to_phase_a)."""
        self.assertEqual(
            iterate.infer_failure_code(
                gate_verdict={
                    "spec": {"passed": False, "failure_codes": ["TEST_REGRESSION"]},
                },
            ),
            fc.phase_d_to_phase_a("TEST_REGRESSION"),
        )
        # Explicit value: TEST_REGRESSION → SCHEMA_VALIDATION_ERROR
        self.assertEqual(
            iterate.infer_failure_code(
                gate_verdict={
                    "spec": {"passed": False, "failure_codes": ["TEST_REGRESSION"]},
                },
            ),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_budget_exceeded_translates_to_phase_a(self):
        """BUDGET_EXCEEDED verdict → BUDGET_EXCEEDED (Phase A identity mapping)."""
        self.assertEqual(
            iterate.infer_failure_code(
                gate_verdict={
                    "spec": {"passed": False, "failure_codes": ["BUDGET_EXCEEDED"]},
                },
            ),
            "BUDGET_EXCEEDED",
        )

    def test_passed_true_with_no_failure_codes_returns_empty(self):
        """passed=True verdict → ''. The verdict wins, no text fallback."""
        self.assertEqual(
            iterate.infer_failure_code(
                "SCHEMA_VALIDATION_ERROR happens here too",
                gate_verdict={"spec": {"passed": True, "failure_codes": []}},
            ),
            "",
        )

    def test_unmappable_phase_d_returns_empty_when_verdict_wins(self):
        """Verdict wins: EVIDENCE_MISSING has no Phase A mapping → ''.

        Even when text contains a mappable Phase A code, the verdict
        path takes priority. EVIDENCE_MISSING → None → verdict
        returns "" without falling through.
        """
        self.assertEqual(
            iterate.infer_failure_code(
                "SCHEMA_VALIDATION_ERROR and other things",
                gate_verdict={
                    "spec": {"passed": False, "failure_codes": ["EVIDENCE_MISSING"]},
                },
            ),
            "",
        )

    def test_none_verdict_preserves_old_path(self):
        """gate_verdict=None (explicit) → old regex path unchanged."""
        self.assertEqual(
            iterate.infer_failure_code("BUDGET_EXCEEDED", gate_verdict=None),
            "BUDGET_EXCEEDED",
        )

    def test_verdict_path_picks_first_mappable_code(self):
        """When verdict has multiple failure_codes, return the first mappable one.

        Order matters: BUDGET_EXCEEDED first, then EVIDENCE_MISSING
        (un-mappable) → BUDGET_EXCEEDED wins.
        """
        self.assertEqual(
            iterate.infer_failure_code(
                gate_verdict={
                    "spec": {
                        "passed": False,
                        "failure_codes": ["BUDGET_EXCEEDED", "EVIDENCE_MISSING"],
                    },
                },
            ),
            "BUDGET_EXCEEDED",
        )


class TestGateVerdictDataclassShape(unittest.TestCase):
    """Bonus: the function accepts a real GateVerdict dataclass, not just dicts."""

    def test_gateverdict_dataclass_passed_through(self):
        """Caller can pass a GateVerdict dataclass directly (no .to_dict() needed)."""
        from devkit.gatekeeper import GateVerdict  # noqa: E402

        gv = GateVerdict.build(
            run_id="r1",
            work_item_id="w1",
            evidence_source="inner_sandbox",
            passed=False,
            reason="tests_failed",
            failure_codes=["TEST_REGRESSION"],
        )
        self.assertEqual(
            iterate.infer_failure_code(gate_verdict=gv),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_gateverdict_dataclass_passed_true(self):
        """GateVerdict dataclass with passed=True → '' (no text fallback)."""
        from devkit.gatekeeper import GateVerdict  # noqa: E402

        gv = GateVerdict.build(
            run_id="r2",
            work_item_id="w2",
            evidence_source="inner_sandbox",
            passed=True,
            reason="all_gates_passed",
            failure_codes=[],
        )
        self.assertEqual(
            iterate.infer_failure_code(
                "SCHEMA_VALIDATION_ERROR happens here",
                gate_verdict=gv,
            ),
            "",
        )


class TestLoadRunVerdictHelper(unittest.TestCase):
    """devkit.__main__._load_run_verdict — fail-open helper for verdict.json."""

    def test_returns_none_when_no_verdict_file(self):
        """No verdict.json → None (legacy path remains active)."""
        from devkit.__main__ import _load_run_verdict  # noqa: E402

        with tempfile.TemporaryDirectory() as tmp:
            self.assertIsNone(_load_run_verdict(pathlib.Path(tmp)))

    def test_returns_none_for_none_run_dir(self):
        """None run_dir → None (fail-open)."""
        from devkit.__main__ import _load_run_verdict  # noqa: E402

        self.assertIsNone(_load_run_verdict(None))
        self.assertIsNone(_load_run_verdict(""))

    def test_loads_verdict_payload_when_present(self):
        """Verdict.json present → dict payload (wire shape)."""
        from devkit.__main__ import _load_run_verdict  # noqa: E402

        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp) / "verdict.json"
            p.write_text(
                '{"spec": {"passed": false, "failure_codes": ["BUDGET_EXCEEDED"]}}',
                encoding="utf-8",
            )
            verdict = _load_run_verdict(pathlib.Path(tmp))
            self.assertIsNotNone(verdict)
            self.assertEqual(verdict["spec"]["failure_codes"], ["BUDGET_EXCEEDED"])

    def test_corrupt_verdict_returns_none(self):
        """Unparseable verdict.json → None (fail-open)."""
        from devkit.__main__ import _load_run_verdict  # noqa: E402

        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp) / "verdict.json"
            p.write_text("not valid json", encoding="utf-8")
            self.assertIsNone(_load_run_verdict(pathlib.Path(tmp)))

    def test_load_then_infer_integration(self):
        """End-to-end: verdict.json on disk → infer_failure_code returns Phase A code."""
        from devkit.__main__ import _load_run_verdict  # noqa: E402

        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp) / "verdict.json"
            p.write_text(
                '{"spec": {"passed": false, "failure_codes": ["TEST_REGRESSION"]}}',
                encoding="utf-8",
            )
            verdict = _load_run_verdict(pathlib.Path(tmp))
            self.assertEqual(
                iterate.infer_failure_code(
                    "some text that doesn't matter",
                    gate_verdict=verdict,
                ),
                "SCHEMA_VALIDATION_ERROR",
            )


if __name__ == "__main__":
    unittest.main()