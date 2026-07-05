"""Tests for devkit/failure_codes.py — Phase B/D/A vocabulary translator.

Coverage targets (>=6):

1. Each Phase B reason maps to its Phase D equivalent (or None).
2. Each Phase D enum maps to its Phase A equivalent (or None).
3. Composition chain: phase_b → phase_d → phase_a for known pairs.
4. Unknown reason returns None (no exception).
5. ``all_phase_a_for_phase_d`` returns multiple when applicable.
6. Round-trip property: tests_failed / over_budget reach a non-empty Phase A.
Plus extras: vectorised helpers, full chain, render, dict-shape, public API.
"""
from __future__ import annotations

import pathlib
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import failure_codes as fc  # noqa: E402


class TestConstants(unittest.TestCase):
    def test_phase_d_codes_match_gatekeeper_enum(self):
        """PHASE_D_CODES must align with gatekeeper.FAILURE_CODES."""
        from devkit import gatekeeper

        # gatekeeper.FAILURE_CODES is a tuple; PHASE_D_CODES is a frozenset.
        self.assertEqual(set(fc.PHASE_D_CODES), set(gatekeeper.FAILURE_CODES))

    def test_phase_b_reasons_are_all_in_table(self):
        """Every entry in PHASE_B_REASONS must have a PHASE_B_TO_PHASE_D entry."""
        for reason in fc.PHASE_B_REASONS:
            self.assertIn(reason, fc.PHASE_B_TO_PHASE_D)

    def test_phase_a_codes_are_non_empty(self):
        self.assertGreater(len(fc.PHASE_A_REPAIRER_CODES), 0)

    def test_phase_d_phase_a_table_is_well_typed(self):
        """Every Phase D code should have either a string or None value."""
        for code, mapped in fc.PHASE_D_TO_PHASE_A.items():
            self.assertTrue(mapped is None or isinstance(mapped, str))


class TestPhaseBToPhaseD(unittest.TestCase):
    """1. Each Phase B reason maps to its Phase D equivalent (or None)."""

    def test_tests_failed_maps_to_test_regression(self):
        self.assertEqual(fc.phase_b_to_phase_d("tests_failed"), "TEST_REGRESSION")

    def test_over_budget_maps_to_budget_exceeded(self):
        self.assertEqual(fc.phase_b_to_phase_d("over_budget"), "BUDGET_EXCEEDED")

    def test_blocked_maps_to_evidence_invalid(self):
        self.assertEqual(fc.phase_b_to_phase_d("blocked"), "EVIDENCE_INVALID")

    def test_task_contract_blocked_maps_to_evidence_invalid(self):
        self.assertEqual(
            fc.phase_b_to_phase_d("task_contract_blocked"),
            "EVIDENCE_INVALID",
        )

    def test_review_requested_changes_has_no_phase_d(self):
        self.assertIsNone(fc.phase_b_to_phase_d("review_requested_changes"))

    def test_review_request_changes_has_no_phase_d(self):
        self.assertIsNone(fc.phase_b_to_phase_d("review_request_changes"))

    def test_review_timeout_has_no_phase_d(self):
        self.assertIsNone(fc.phase_b_to_phase_d("review_timeout"))

    def test_suggested_go_has_no_phase_d(self):
        """Success path → no failure code."""
        self.assertIsNone(fc.phase_b_to_phase_d("suggested_go"))

    def test_blocked_no_detail_has_no_phase_d(self):
        self.assertIsNone(fc.phase_b_to_phase_d("blocked_no_detail"))

    def test_no_detail_has_no_phase_d(self):
        self.assertIsNone(fc.phase_b_to_phase_d("no_detail"))


class TestPhaseDToPhaseA(unittest.TestCase):
    """2. Each Phase D enum maps to its Phase A equivalent (or None)."""

    def test_test_regression_maps_to_schema_validation_error(self):
        self.assertEqual(
            fc.phase_d_to_phase_a("TEST_REGRESSION"),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_budget_exceeded_maps_to_budget_exceeded(self):
        self.assertEqual(
            fc.phase_d_to_phase_a("BUDGET_EXCEEDED"),
            "BUDGET_EXCEEDED",
        )

    def test_evidence_missing_has_no_phase_a(self):
        self.assertIsNone(fc.phase_d_to_phase_a("EVIDENCE_MISSING"))

    def test_evidence_invalid_maps_to_schema_validation_error(self):
        self.assertEqual(
            fc.phase_d_to_phase_a("EVIDENCE_INVALID"),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_schema_validation_error_maps_to_itself(self):
        self.assertEqual(
            fc.phase_d_to_phase_a("SCHEMA_VALIDATION_ERROR"),
            "SCHEMA_VALIDATION_ERROR",
        )


class TestCompositionChain(unittest.TestCase):
    """3. Composition chain: phase_b → phase_d → phase_a for known pairs."""

    def test_tests_failed_full_chain(self):
        self.assertEqual(fc.phase_b_to_phase_a("tests_failed"), "SCHEMA_VALIDATION_ERROR")

    def test_over_budget_full_chain(self):
        self.assertEqual(fc.phase_b_to_phase_a("over_budget"), "BUDGET_EXCEEDED")

    def test_blocked_full_chain(self):
        self.assertEqual(fc.phase_b_to_phase_a("blocked"), "SCHEMA_VALIDATION_ERROR")

    def test_task_contract_blocked_full_chain(self):
        self.assertEqual(
            fc.phase_b_to_phase_a("task_contract_blocked"),
            "SCHEMA_VALIDATION_ERROR",
        )

    def test_review_requested_changes_returns_none(self):
        """No Phase D equivalent → chain breaks → None."""
        self.assertIsNone(fc.phase_b_to_phase_a("review_requested_changes"))

    def test_suggested_go_returns_none(self):
        """Success → no Phase D → no Phase A."""
        self.assertIsNone(fc.phase_b_to_phase_a("suggested_go"))


class TestUnknownInputs(unittest.TestCase):
    """4. Unknown reason returns None (no exception)."""

    def test_unknown_phase_b_reason_returns_none(self):
        self.assertIsNone(fc.phase_b_to_phase_d("totally_made_up"))

    def test_unknown_phase_d_returns_none(self):
        self.assertIsNone(fc.phase_d_to_phase_a("TOTALLY_MADE_UP"))

    def test_empty_string_returns_none(self):
        self.assertIsNone(fc.phase_b_to_phase_d(""))
        self.assertIsNone(fc.phase_d_to_phase_a(""))

    def test_non_string_phase_b_returns_none(self):
        self.assertIsNone(fc.phase_b_to_phase_d(None))  # type: ignore[arg-type]
        self.assertIsNone(fc.phase_b_to_phase_d(123))  # type: ignore[arg-type]
        self.assertIsNone(fc.phase_b_to_phase_d(["blocked"]))  # type: ignore[arg-type]

    def test_non_string_phase_d_returns_none(self):
        self.assertIsNone(fc.phase_d_to_phase_a(None))  # type: ignore[arg-type]
        self.assertIsNone(fc.phase_d_to_phase_a(42))  # type: ignore[arg-type]


class TestAllPhaseAForPhaseD(unittest.TestCase):
    """5. ``all_phase_a_for_phase_d`` returns multiple when applicable."""

    def test_test_regression_has_two_candidates(self):
        candidates = fc.all_phase_a_for_phase_d("TEST_REGRESSION")
        self.assertGreaterEqual(len(candidates), 2)
        self.assertIn("SCHEMA_VALIDATION_ERROR", candidates)
        self.assertIn("INVALID_TASK_SPEC", candidates)

    def test_budget_exceeded_has_one_candidate(self):
        candidates = fc.all_phase_a_for_phase_d("BUDGET_EXCEEDED")
        self.assertEqual(candidates, ["BUDGET_EXCEEDED"])

    def test_evidence_missing_has_empty_candidates(self):
        self.assertEqual(fc.all_phase_a_for_phase_d("EVIDENCE_MISSING"), [])

    def test_evidence_invalid_has_two_candidates(self):
        candidates = fc.all_phase_a_for_phase_d("EVIDENCE_INVALID")
        self.assertIn("SCHEMA_VALIDATION_ERROR", candidates)
        self.assertIn("INVALID_TASK_SPEC", candidates)

    def test_unknown_phase_d_returns_empty_list(self):
        self.assertEqual(fc.all_phase_a_for_phase_d("UNKNOWN_THING"), [])

    def test_all_phase_a_for_phase_b_uses_chain(self):
        """Phase B → Phase D → all Phase A candidates."""
        candidates = fc.all_phase_a_for_phase_b("tests_failed")
        self.assertGreater(len(candidates), 0)
        self.assertIn("SCHEMA_VALIDATION_ERROR", candidates)

    def test_all_phase_a_for_phase_b_blocked_returns_candidates(self):
        candidates = fc.all_phase_a_for_phase_b("blocked")
        self.assertGreater(len(candidates), 0)
        self.assertIn("SCHEMA_VALIDATION_ERROR", candidates)

    def test_all_phase_a_for_phase_b_review_returns_empty(self):
        """review_requested_changes has no Phase D → empty chain result."""
        self.assertEqual(fc.all_phase_a_for_phase_b("review_requested_changes"), [])


class TestRoundTripProperty(unittest.TestCase):
    """6. Round-trip property: tests_failed / over_budget reach a non-empty Phase A."""

    def test_main_three_phase_b_reasons_each_reach_a_phase_a(self):
        """The 'main three' from the prompt: tests_failed, over_budget, blocked."""
        for reason in ("tests_failed", "over_budget", "blocked"):
            with self.subTest(reason=reason):
                d = fc.phase_b_to_phase_d(reason)
                self.assertIsNotNone(d, f"phase_d missing for {reason!r}")
                a = fc.phase_d_to_phase_a(d)
                self.assertIsNotNone(a, f"phase_a missing for {d!r}")
                candidates = fc.all_phase_a_for_phase_d(d)
                self.assertGreater(
                    len(candidates),
                    0,
                    f"no Phase A candidates for {d!r}",
                )

    def test_chain_result_is_actionable_for_main_three(self):
        for reason in ("tests_failed", "over_budget", "blocked"):
            result = fc.translate_chain(reason)
            with self.subTest(reason=reason):
                self.assertTrue(
                    result.is_actionable(),
                    f"chain not actionable for {reason!r}: {result}",
                )


class TestTranslateChain(unittest.TestCase):
    """Full-chain TranslationResult behaviour."""

    def test_chain_result_for_tests_failed(self):
        r = fc.translate_chain("tests_failed")
        self.assertEqual(r.reason, "tests_failed")
        self.assertEqual(r.phase_d, "TEST_REGRESSION")
        self.assertEqual(r.phase_a, "SCHEMA_VALIDATION_ERROR")
        self.assertIn("SCHEMA_VALIDATION_ERROR", r.phase_a_candidates)
        self.assertIn("INVALID_TASK_SPEC", r.phase_a_candidates)
        self.assertTrue(r.is_actionable())

    def test_chain_result_for_review_returns_none_phase(self):
        r = fc.translate_chain("review_requested_changes")
        self.assertIsNone(r.phase_d)
        self.assertIsNone(r.phase_a)
        self.assertEqual(r.phase_a_candidates, ())
        self.assertFalse(r.is_actionable())

    def test_chain_result_to_dict_shape(self):
        r = fc.translate_chain("over_budget")
        d = r.to_dict()
        self.assertEqual(d["reason"], "over_budget")
        self.assertEqual(d["phase_d"], "BUDGET_EXCEEDED")
        self.assertEqual(d["phase_a"], "BUDGET_EXCEEDED")
        self.assertIsInstance(d["phase_a_candidates"], list)

    def test_chain_result_render_string(self):
        r = fc.translate_chain("blocked")
        s = r.render()
        self.assertIn("blocked", s)
        self.assertIn("EVIDENCE_INVALID", s)
        self.assertIn("SCHEMA_VALIDATION_ERROR", s)

    def test_chain_result_for_unknown_reason_does_not_raise(self):
        # Should not raise; should produce None hops.
        r = fc.translate_chain("not_a_real_thing")
        self.assertEqual(r.reason, "not_a_real_thing")
        self.assertIsNone(r.phase_d)
        self.assertIsNone(r.phase_a)
        self.assertEqual(r.phase_a_candidates, ())


class TestBulkHelpers(unittest.TestCase):
    def test_translate_phase_b_to_phase_d_preserves_order(self):
        reasons = ["blocked", "tests_failed", "over_budget", "review_timeout"]
        out = fc.translate_phase_b_to_phase_d(reasons)
        self.assertEqual(
            out,
            ["EVIDENCE_INVALID", "TEST_REGRESSION", "BUDGET_EXCEEDED", None],
        )

    def test_translate_phase_d_to_phase_a_preserves_order(self):
        enums = ["TEST_REGRESSION", "BUDGET_EXCEEDED", "EVIDENCE_MISSING"]
        out = fc.translate_phase_d_to_phase_a(enums)
        self.assertEqual(
            out,
            ["SCHEMA_VALIDATION_ERROR", "BUDGET_EXCEEDED", None],
        )


class TestPublicAPI(unittest.TestCase):
    """Sanity-check the public surface that downstream modules import."""

    EXPECTED_NAMES = [
        "PHASE_D_CODES",
        "PHASE_B_REASONS",
        "PHASE_A_REPAIRER_CODES",
        "PHASE_B_TO_PHASE_D",
        "PHASE_D_TO_PHASE_A",
        "ALL_PHASE_A_FOR_PHASE_D",
        "phase_b_to_phase_d",
        "phase_d_to_phase_a",
        "phase_b_to_phase_a",
        "all_phase_a_for_phase_d",
        "all_phase_a_for_phase_b",
        "translate_phase_b_to_phase_d",
        "translate_phase_d_to_phase_a",
        "translate_chain",
        "TranslationResult",
    ]

    def test_all_expected_names_are_exported(self):
        for name in self.EXPECTED_NAMES:
            with self.subTest(name=name):
                self.assertTrue(
                    hasattr(fc, name),
                    f"devkit.failure_codes is missing {name!r}",
                )

    def test_module_is_importable(self):
        import importlib

        m = importlib.import_module("devkit.failure_codes")
        self.assertIs(m, fc)


if __name__ == "__main__":
    unittest.main()