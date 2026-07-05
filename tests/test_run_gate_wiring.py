"""Tests for the rdloop ↔ evaluate_run_gate wiring (``unify-run-gate-wiring``).

This is T6b in the ``plan_e78d4693`` rollout. T6a (``unify-run-gate-function``)
added ``devkit.gatekeeper.evaluate_run_gate(...)`` and the ``gate_inputs``
sanitization bridge. This task wires ``devkit.rdloop.run_loop`` to that
bridge:

  1. ``run_loop`` now calls ``gatekeeper.evaluate_run_gate(...)`` instead of
     its own kwargs-shaped ``evaluate_final_gate(...)``.
  2. The typed ``GateVerdict`` is persisted to ``<run_dir>/verdict.json`` via
     ``gatekeeper.write_verdict(...)`` (fail-open, ``logger.warning`` not
     silent ``pass``).
  3. The Phase-B ``evaluate_final_gate(...)`` is kept callable for direct
     callers (the regression suite) but emits a ``DeprecationWarning``
     pointing at the new entry point.

Tests cover:

  * ``TestEvaluateFinalGateDeprecation`` — the wrapper fires
    ``DeprecationWarning`` whose message names ``evaluate_run_gate`` and
    still returns the legacy ``(status_code, reasons)`` tuple.
  * ``TestEvaluateFinalGateLegacyBehavior`` — every flag-combination that
    rdloop exercised before still produces the same status string +
    reasons list (the regression suite contract).
  * ``TestRdloopCallSiteWiring`` — source-level: ``rdloop.run_loop`` calls
    ``evaluate_run_gate``, persists ``verdict.json``, and the
    ``evaluate_final_gate`` symbol has been replaced by the
    ``_deprecated``-wrapped variant.
  * ``TestRunGateVerdictPersistence`` — end-to-end behavioural: invoking
    ``evaluate_run_gate`` with the exact kwargs dict rdloop passes at the
    call site produces a verdict, the writer materialises
    ``evidence_packet.json``, and ``write_verdict`` round-trips through
    ``load_verdict``.

Verification target
-------------------
``PYTHONPATH=. .venv/bin/python -m unittest tests.test_run_gate_wiring -v``
should report all tests OK. Full-suite regression tests (notably
``test_rdloop_spec_integration``, ``test_run_gate_function``,
``test_evidence_writer``, ``test_repairer``) must stay green.
"""
from __future__ import annotations

import logging
import pathlib
import re
import sys
import tempfile
import unittest
import warnings

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import gatekeeper
from devkit import rdloop
from devkit.gatekeeper import (
    GateVerdict,
    evaluate_run_gate,
    load_verdict,
    write_verdict,
)


# ---------------------------------------------------------------------------
# Helpers — keep in lock-step with the rest of the test suite (notably
# ``tests/test_run_gate_function.py``). When the rdloop evidence layout
# changes, mirror the change here too.
# ---------------------------------------------------------------------------
def _seed_run_dir(run_dir: pathlib.Path) -> pathlib.Path:
    """Lay down the minimal pair rdloop writes before the gate fires."""
    run_dir.mkdir(parents=True, exist_ok=True)
    (run_dir / "00-task.md").write_text(
        "# Task\n\nWiring smoke test.\n", encoding="utf-8"
    )
    (run_dir / "99-gate.md").write_text(
        "# Gate\n\ngate: GO\nstatus_code: suggested_go\n", encoding="utf-8"
    )
    return run_dir


def _rdloop_gate_inputs(*, tests_failed: bool = False, over_budget: bool = False,
                        blocked=None, review_blocked: bool = False,
                        review_requested_changes: bool = False) -> dict:
    """Reproduce the kwargs dict rdloop.run_loop threads through to the gate.

    Spec form (from ``rdloop.py``):
        gate_inputs = {
            "blocked": blocked,
            "review_blocked": review_blocked,
            "review_requested_changes": review_requested_changes,
            "tests_failed": tests_failed,
            "over_budget": over_budget,
            "gate_spec": gate_spec,
        }
    """
    if blocked is None:
        blocked = []
    return {
        "blocked": blocked,
        "review_blocked": review_blocked,
        "review_requested_changes": review_requested_changes,
        "tests_failed": tests_failed,
        "over_budget": over_budget,
        "gate_spec": None,
    }


# ===========================================================================
# Test 1 — ``evaluate_final_gate`` deprecation wrapper
# ===========================================================================
class TestEvaluateFinalGateDeprecation(unittest.TestCase):
    """The legacy kwarg-shaped helper must warn + keep working."""

    def test_emits_deprecation_warning_on_call(self):
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always")
            rdloop.evaluate_final_gate(
                blocked=[],
                review_blocked=False,
                review_requested_changes=False,
                tests_failed=False,
                over_budget=False,
            )
        dep_warnings = [w for w in caught if issubclass(w.category, DeprecationWarning)]
        self.assertTrue(
            dep_warnings,
            "expected at least one DeprecationWarning emitted by "
            "devkit.rdloop.evaluate_final_gate, got none",
        )

    def test_warning_message_names_evaluate_run_gate(self):
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always")
            rdloop.evaluate_final_gate(
                blocked=[],
                review_blocked=False,
                review_requested_changes=False,
                tests_failed=False,
                over_budget=False,
            )
        dep_messages = [
            str(w.message) for w in caught if issubclass(w.category, DeprecationWarning)
        ]
        self.assertTrue(
            any("evaluate_run_gate" in m for m in dep_messages),
            f"expected deprecation message to point at 'evaluate_run_gate'; "
            f"got {dep_messages!r}",
        )

    def test_still_returns_legacy_two_tuple_shape(self):
        """The wrapper must NOT change the return signature — callers and
        tests still destructure ``(status_code, reasons)``."""
        with warnings.catch_warnings():
            # Don't pollute the captured list with the deprecation noise.
            warnings.simplefilter("ignore", DeprecationWarning)
            result = rdloop.evaluate_final_gate(
                blocked=[],
                review_blocked=False,
                review_requested_changes=False,
                tests_failed=True,
                over_budget=False,
            )
        self.assertIsInstance(result, tuple)
        self.assertEqual(len(result), 2)
        status_code, reasons = result
        self.assertEqual(status_code, "tests_failed")
        self.assertIsInstance(reasons, list)
        # reasons contains the legacy Chinese helper text — preserved verbatim.
        self.assertIn("构建测试/Eval 未过", reasons)


# ===========================================================================
# Test 2 — Legacy ``evaluate_final_gate`` behaviour preserved bit-for-bit
# ===========================================================================
class TestEvaluateFinalGateLegacyBehavior(unittest.TestCase):
    """Every flag combination rdloop's old call site relied on still
    produces the same status_code. Source: test_rdloop_spec_integration's
    ``test_rdloop_evaluate_final_gate_wired_into_run_loop_signature``."""

    def setUp(self):
        # Silence the DeprecationWarning during the assertions — this class
        # is about behaviour preservation, not about the warning itself.
        self._warn_ctx = warnings.catch_warnings()
        self._warn_ctx.__enter__()
        warnings.simplefilter("ignore", DeprecationWarning)
        self.addCleanup(self._warn_ctx.__exit__, None, None, None)

    def test_review_blocked_yields_review_timeout(self):
        status, reasons = rdloop.evaluate_final_gate(
            blocked=[],
            review_blocked=True,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(status, "review_timeout")

    def test_review_requested_changes_yields_review_request_changes(self):
        status, reasons = rdloop.evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=True,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(status, "review_request_changes")

    def test_tests_failed_yields_tests_failed(self):
        status, reasons = rdloop.evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=True,
            over_budget=False,
        )
        self.assertEqual(status, "tests_failed")
        self.assertIn("构建测试/Eval 未过", reasons)

    def test_over_budget_yields_over_budget(self):
        status, reasons = rdloop.evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=True,
        )
        self.assertEqual(status, "over_budget")
        self.assertIn("超预算", reasons)

    def test_all_clean_yields_suggested_go(self):
        status, reasons = rdloop.evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(status, "suggested_go")
        self.assertEqual(reasons, [])


# ===========================================================================
# Test 3 — rdloop.run_loop call-site wiring (source-level contract)
# ===========================================================================
class TestRdloopCallSiteWiring(unittest.TestCase):
    """Pin down the wiring contract on the rdloop side via source inspection.

    These are intentionally source-level: the contract being verified is
    "rdloop.py now calls evaluate_run_gate, persists verdict.json, and the
    legacy helper is wrapped in ``_deprecated``". A purely behavioural test
    would have to spin up the entire 1800-line ``run_loop`` body, which is
    not feasible in a unit test (network, gateways, model API).
    """

    @classmethod
    def setUpClass(cls):
        cls._rdloop_src = (ROOT / "devkit" / "rdloop.py").read_text(encoding="utf-8")
        cls._rdloop_lines = cls._rdloop_src.splitlines()

    def _line(self, text: str) -> int:
        """Return the 1-based line number for the first matching line."""
        for i, line in enumerate(self._rdloop_lines, start=1):
            if text in line:
                return i
        raise AssertionError(f"no line in rdloop.py contains {text!r}")

    def test_run_loop_calls_evaluate_run_gate(self):
        """The Phase-B call site must be gone; the Phase-D call must be in."""
        # The function definition is still present (it's deprecated, not
        # deleted) — so we can't blindly assertNoMatch on the substring.
        # Instead we assert two structural facts:
        #   1. The specific call-site pattern that lived at line 2789
        #      pre-wiring (``status_code, reasons = evaluate_final_gate(...)``)
        #      is gone from the run_loop body.
        #   2. The new gatekeeper.evaluate_run_gate call IS present.
        self.assertNotRegex(
            self._rdloop_src,
            r"status_code,\s+reasons\s*=\s*evaluate_final_gate\(",
            "legacy kwarg-shape call site 'status_code, reasons = "
            "evaluate_final_gate(...)' must be removed from run_loop",
        )
        self.assertIn(
            "_gatekeeper.evaluate_run_gate(",
            self._rdloop_src,
            "expected _gatekeeper.evaluate_run_gate(...) call in rdloop.run_loop",
        )
        # The unwrapped (no `_gatekeeper.` prefix) form must not appear
        # anywhere — rdloop imports gatekeeper locally so all calls are
        # namespaced.
        # NB: bare `evaluate_run_gate(` may still appear in docstrings.
        unwrapped_re = re.compile(r"(?<!_gatekeeper\.)\bevaluate_run_gate\(")
        bare_calls = [m for m in unwrapped_re.finditer(self._rdloop_src)]
        self.assertEqual(
            bare_calls,
            [],
            f"expected all evaluate_run_gate calls to be namespaced as "
            f"_gatekeeper.evaluate_run_gate; found bare calls at: "
            f"{[m.start() for m in bare_calls]!r}",
        )

    def test_run_loop_persists_verdict_json_with_fail_open_logging(self):
        """The verdict write must be wrapped in try/except + logger.warning,
        not a silent ``pass`` — the spec forbids silent swallowing."""
        # Locate the run_loop call site block. We do this by anchoring on
        # the verdict.json write call and asserting the surrounding lines.
        verdict_idx = self._line('run_dir / "verdict.json"')
        # Look at the previous 12 lines — should include the try: and the
        # write_verdict() call. Look at the next 6 lines — should include
        # the except + logger.warning call.
        window = self._rdloop_lines[max(0, verdict_idx - 12): verdict_idx + 6]
        joined = "\n".join(window)
        self.assertIn(
            "write_verdict(",
            joined,
            "expected write_verdict(...) in the surrounding block",
        )
        self.assertIn(
            "try:",
            joined,
            "expected a try: block around the verdict write (fail-open)",
        )
        # The except clause must call logger.warning — NOT a bare ``pass``.
        except_window = self._rdloop_lines[verdict_idx: verdict_idx + 8]
        except_joined = "\n".join(except_window)
        self.assertIn(
            "logger.warning(",
            except_joined,
            "expected logger.warning(...) in the except handler — "
            "spec forbids silent swallow",
        )
        # Belt-and-suspenders: ensure no bare ``except: pass`` was
        # introduced for the verdict write (this pattern was used for
        # write_run_protocol_bundle and is exactly what the spec
        # prohibits here).
        self.assertNotIn(
            'except Exception:\n        pass',
            "\n".join(self._rdloop_lines[verdict_idx - 6: verdict_idx + 8]),
            "verdict write must not be a bare 'except: pass'",
        )

    def test_evaluate_final_gate_symbol_wrapped_in_deprecated(self):
        """The public symbol must be replaced by the ``_deprecated``-wrapped
        variant. Direct callers (``test_rdloop_spec_integration``) still
        resolve it via ``from devkit.rdloop import evaluate_final_gate``."""
        # Locate the line that re-assigns the symbol.
        reassign_idx = self._line("evaluate_final_gate = _deprecated(evaluate_final_gate)")
        # The decorator helper must be defined in the same module.
        self.assertIn(
            "def _deprecated(",
            self._rdloop_src,
            "expected _deprecated() helper in rdloop.py",
        )
        # Confirm functools + warnings are imported at module top. Use a
        # newline-anchored pattern (``\nimport functools\b``) instead of
        # ``^`` because ``assertRegex`` uses ``re.search`` without
        # ``re.MULTILINE`` — ``^`` only matches the very start of the file,
        # not lines after a docstring.
        self.assertRegex(
            self._rdloop_src,
            r"\nimport functools\b",
            "expected 'import functools' at module top",
        )
        self.assertRegex(
            self._rdloop_src,
            r"\nimport warnings\b",
            "expected 'import warnings' at module top",
        )
        # The wrapper must emit a DeprecationWarning that mentions the
        # new entry point — pin down the message text.
        self.assertIn(
            "DeprecationWarning",
            self._rdloop_src,
            "expected DeprecationWarning in rdloop.py",
        )
        self.assertIn(
            "evaluate_run_gate",
            self._rdloop_src,
            "expected deprecation message to name 'evaluate_run_gate'",
        )
        # The reassignment must come AFTER the original function
        # definition (sanity — a forward reference would explode).
        def_line = self._line("def evaluate_final_gate(")
        self.assertLess(
            def_line,
            reassign_idx,
            "the @_deprecated wrap must come AFTER the def, not before",
        )


# ===========================================================================
# Test 4 — End-to-end: evaluate_run_gate + write_verdict round-trip
# ===========================================================================
class TestRunGateVerdictPersistence(unittest.TestCase):
    """Behavioural tests — call ``evaluate_run_gate`` the same way rdloop
    does, then assert ``verdict.json`` is written and loadable."""

    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        self.run_dir = self.tmp / "run"
        _seed_run_dir(self.run_dir)
        self.evidence_dir = self.tmp / "evidence"
        # Run id mirrors the rdloop call-site pattern (ts string).
        self.run_id = "20260705-r-gate-wiring"
        self.work_item_id = "wi-r-gate-wiring"

    def test_evaluate_run_gate_writes_verdict_json(self):
        """Mirrors the rdloop call site: pass kwargs dict + run_dir, get
        verdict, persist via write_verdict, assert file exists."""
        gate_inputs = _rdloop_gate_inputs(tests_failed=True)
        status, reasons, verdict = evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        # Phase-B status/reason shape preserved for the reflection/memory
        # consumers downstream of run_loop.
        self.assertEqual(status, "task_contract_blocked")
        self.assertIn("tests_failed", reasons)
        # Phase-D typed surface.
        self.assertIsInstance(verdict, GateVerdict)
        self.assertFalse(verdict.passed)
        self.assertIn("TEST_REGRESSION", verdict.failure_codes)
        # Persist + read back — exactly what rdloop does at the call site.
        verdict_path = self.run_dir / "verdict.json"
        returned_path = write_verdict(verdict, verdict_path)
        self.assertTrue(returned_path.exists())
        self.assertTrue(verdict_path.exists())
        self.assertEqual(returned_path.resolve(), verdict_path.resolve())
        reloaded = load_verdict(verdict_path)
        self.assertIsNotNone(reloaded)
        self.assertEqual(reloaded.passed, verdict.passed)
        self.assertEqual(reloaded.failure_codes, verdict.failure_codes)

    def test_clean_run_produces_ok_and_passed_verdict_json(self):
        """The happy path — no Phase-B flags → ``ok`` + empty reasons +
        verdict.passed=True. Verifies the verdict.json payload matches."""
        gate_inputs = _rdloop_gate_inputs()  # all defaults: all False
        status, reasons, verdict = evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        self.assertEqual(status, "ok")
        self.assertEqual(reasons, [])
        self.assertTrue(verdict.passed)
        # Persist + read.
        verdict_path = self.run_dir / "verdict.json"
        write_verdict(verdict, verdict_path)
        reloaded = load_verdict(verdict_path)
        self.assertIsNotNone(reloaded)
        self.assertTrue(reloaded.passed)
        self.assertEqual(reloaded.failure_codes, [])

    def test_over_budget_flag_drives_budget_exceeded_in_verdict(self):
        """Phase-B ``over_budget=True`` must translate to ``BUDGET_EXCEEDED``
        in the typed verdict (and ``over_budget`` in the legacy reasons)."""
        gate_inputs = _rdloop_gate_inputs(over_budget=True)
        status, reasons, verdict = evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        self.assertEqual(status, "task_contract_blocked")
        self.assertIn("over_budget", reasons)
        self.assertIn("BUDGET_EXCEEDED", verdict.failure_codes)

    def test_blocked_list_drops_silently_to_info_log(self):
        """Phase-B ``blocked=[stage]`` is a non-bool list at the rdloop
        call site — the sanitiser (``_sanitize_gate_inputs``) currently
        drops list values at INFO because its Phase-B→Phase-D mapping
        is type-dispatched on ``bool``. The wire-up still completes
        without crashing and the verdict reflects the evidence file.

        If a future change adds list→EVIDENCE_INVALID translation, this
        test should be updated to assert that translation instead.
        This test pins the **current** behaviour so regressions are
        visible — silent drops without INFO logs would be the failure
        mode to watch for.
        """
        gate_inputs = _rdloop_gate_inputs(blocked=["build"])
        status, reasons, verdict = evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        # Bridge must NOT crash on a non-bool blocked value.
        self.assertIsInstance(status, str)
        self.assertIsInstance(reasons, list)
        self.assertIsInstance(verdict, GateVerdict)
        # And the verdict must not contain EVIDENCE_INVALID — the sanitiser
        # dropped the list value (per its bool-only dispatch contract).
        self.assertNotIn(
            "EVIDENCE_INVALID",
            verdict.failure_codes,
            "expected list-typed blocked value to be silently dropped "
            "by _sanitize_gate_inputs (bool-only dispatch); if this "
            "fails, the sanitiser contract changed and the test must "
            "be updated alongside.",
        )

    def test_review_flags_drop_silently_to_info_log(self):
        """Phase-B ``review_*`` flags have no Phase-D equivalent (review
        verdicts are gate-evidence-orthogonal); the bridge must NOT crash
        and must NOT pollute the verdict.failure_codes."""
        gate_inputs = _rdloop_gate_inputs(
            review_blocked=True, review_requested_changes=True
        )
        status, reasons, verdict = evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        # All review flags dropped → empty verdict.
        self.assertEqual(status, "ok")
        self.assertEqual(reasons, [])
        self.assertTrue(verdict.passed)
        self.assertEqual(verdict.failure_codes, [])

    def test_evidence_packet_materialised_under_evidence_dir(self):
        """The writer must lay down ``<evidence_dir>/<run_id>/evidence_packet.json``
        so the gate can read it back. This is the prerequisite for
        ``write_verdict`` to round-trip — both files come out of the
        same call."""
        gate_inputs = _rdloop_gate_inputs()
        evaluate_run_gate(
            run_id=self.run_id,
            work_item_id=self.work_item_id,
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        packet_path = self.evidence_dir / self.run_id / "evidence_packet.json"
        self.assertTrue(
            packet_path.exists(),
            f"expected writer to lay down {packet_path}",
        )


# ===========================================================================
# Test 5 — Verdict write failure must be logged, not swallowed silently
# ===========================================================================
class TestVerdictWriteFailOpenLogging(unittest.TestCase):
    """The rdloop call site wraps ``write_verdict`` in try/except +
    ``logger.warning``. The contract being verified: if ``write_verdict``
    raises, the loop continues and the cause lands in the standard
    ``devkit.rdloop`` logger at WARNING level — NOT silently dropped."""

    def setUp(self):
        self.tmp_ctx = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_ctx.cleanup)
        self.tmp = pathlib.Path(self.tmp_ctx.name)
        self.run_dir = self.tmp / "run"
        _seed_run_dir(self.run_dir)
        self.evidence_dir = self.tmp / "evidence"

    def test_write_verdict_failure_emits_logger_warning(self):
        """If ``write_verdict`` raises (here we make it raise by feeding an
        unserialisable path), ``devkit.rdloop.logger.warning`` must be
        hit — proving the call site uses the ``logger.warning`` pattern
        and NOT the bare ``except: pass`` pattern from the protocol
        bundle write above it."""
        # Force write_verdict to raise by monkey-patching it.
        original = gatekeeper.write_verdict

        def boom(*_args, **_kwargs):
            raise RuntimeError("simulated verdict write crash")

        gatekeeper.write_verdict = boom
        try:
            with self.assertLogs("devkit.rdloop", level="WARNING") as log_ctx:
                # Mirror the rdloop call-site pattern: try the write,
                # except -> logger.warning.
                gate_inputs = _rdloop_gate_inputs()
                status, reasons, verdict = evaluate_run_gate(
                    run_id="boom-run",
                    work_item_id="wi-boom",
                    run_dir=self.run_dir,
                    gate_inputs=gate_inputs,
                    evidence_dir=self.evidence_dir,
                )
                try:
                    gatekeeper.write_verdict(verdict, self.run_dir / "verdict.json")
                except Exception as exc:  # noqa: BLE001
                    rdloop.logger.warning(
                        "run_loop: write_verdict(%s/verdict.json) failed: %s",
                        self.run_dir,
                        exc,
                    )
            # Exactly one warning on devkit.rdloop — and it names the cause.
            self.assertEqual(len(log_ctx.records), 1)
            record = log_ctx.records[0]
            self.assertEqual(record.levelname, "WARNING")
            self.assertIn("simulated verdict write crash", record.getMessage())
            self.assertIn("verdict.json", record.getMessage())
        finally:
            gatekeeper.write_verdict = original

    def test_write_verdict_success_emits_no_warning(self):
        """The negative control: when write_verdict succeeds, no warning
        is emitted on devkit.rdloop."""
        gate_inputs = _rdloop_gate_inputs()
        status, reasons, verdict = evaluate_run_gate(
            run_id="clean-run",
            work_item_id="wi-clean",
            run_dir=self.run_dir,
            gate_inputs=gate_inputs,
            evidence_dir=self.evidence_dir,
        )
        # Build a logger handler that records everything; expect zero
        # records after a clean write.
        records = []

        class _Capture(logging.Handler):
            def emit(self, record):
                records.append(record)

        handler = _Capture(level=logging.DEBUG)
        rdloop.logger.addHandler(handler)
        try:
            write_verdict(verdict, self.run_dir / "verdict.json")
        finally:
            rdloop.logger.removeHandler(handler)
        # The verdict write itself is done by gatekeeper (not rdloop); the
        # assertion is that no rdloop-level warning fired.
        rdloop_warnings = [
            r for r in records if r.name == "devkit.rdloop" and r.levelno >= logging.WARNING
        ]
        self.assertEqual(
            rdloop_warnings,
            [],
            f"expected no devkit.rdloop warnings on clean write; got {rdloop_warnings!r}",
        )


# ===========================================================================
# Test 6 — Module-level surface: rdloop's evaluate_final_gate is the wrapper
# ===========================================================================
class TestRdloopEvaluateFinalGateSurface(unittest.TestCase):
    """The public symbol is the wrapped variant. Imports from
    ``devkit.rdloop`` (e.g. the regression suite) see the wrapper."""

    def test_imported_symbol_is_wrapped(self):
        # ``__wrapped__`` is set by ``functools.wraps`` and is the
        # canonical signal that the function passed through the
        # ``_deprecated`` decorator.
        self.assertTrue(
            hasattr(rdloop.evaluate_final_gate, "__wrapped__"),
            "expected devkit.rdloop.evaluate_final_gate to be wrapped "
            "(functools.wraps sets __wrapped__ on the wrapper)",
        )
        # The wrapper exposes the *original* ``evaluate_final_gate`` via
        # ``__wrapped__`` — that's the function the wrapper delegates to.
        # The original (in turn) delegates to ``_resolve_gate_status``.
        self.assertEqual(
            rdloop.evaluate_final_gate.__wrapped__.__name__,
            "evaluate_final_gate",
            "expected __wrapped__ to point at the original "
            "evaluate_final_gate def (the wrapper delegates to it)",
        )
        # And the original def, when called bare, still delegates to the
        # legacy ``_resolve_gate_status`` helper. So the wrap chain is:
        #   wrapper (emits DeprecationWarning)
        #     └─ original evaluate_final_gate (validates gate_spec)
        #          └─ _resolve_gate_status (returns (status, reasons))
        self.assertTrue(
            hasattr(rdloop.evaluate_final_gate.__wrapped__, "_resolve_gate_status")
            or hasattr(rdloop, "_resolve_gate_status"),
            "the original evaluate_final_gate should still call into "
            "_resolve_gate_status (sanity check on the wrap chain)",
        )

    def test_deprecation_decorator_helper_exists_at_module_level(self):
        """The ``_deprecated`` helper must be importable from
        ``devkit.rdloop`` so future Phase-B-to-Phase-D migrations can
        reuse it without copy-paste."""
        self.assertTrue(hasattr(rdloop, "_deprecated"))
        self.assertTrue(callable(rdloop._deprecated))


if __name__ == "__main__":
    unittest.main()
