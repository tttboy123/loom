"""Phase E end-to-end smoke test — verifies the integration seam.

Goal: prove the 9 phase-E tasks actually wire together — not just that
each individual task passes its own test suite. We construct a
synthetic backlog + run_dir, walk the entire path:

  select_next_pending → claim_lease → fake run_loop output →
  evidence_writer.write_run_evidence → evaluate_run_gate →
  verdict.json on disk → release_lease

If any of these seams is broken (a contract mismatch between modules,
a missing translation, a missing dispatch), this smoke test fails
loudly. Per-task unit tests cover the internals; this one covers the
contracts.
"""

from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest

# Ensure devkit is importable when this test is run from the worktree root.
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent))

from devkit import evidence_writer, gatekeeper, scheduler


class TestPhaseEIntegration(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="phase_e_smoke_"))
        self.run_dir = self.tmp / "runs" / "smoke-001"
        self.run_dir.mkdir(parents=True, exist_ok=True)
        self.evidence_dir = self.tmp / "evidence"
        self.evidence_dir.mkdir(parents=True, exist_ok=True)
        self.lease_path = self.tmp / "auto-lease.json"
        # Stand up a fake run_dir that evidence_writer can ingest.
        (self.run_dir / "00-task.md").write_text(
            "# task\nsmoke integration test", encoding="utf-8"
        )
        (self.run_dir / "99-gate.md").write_text(
            "# gate\nok", encoding="utf-8"
        )
        (self.run_dir / "events.jsonl").write_text(
            json.dumps({"event": "smoke_started", "ts": "2026-07-05T15:00:00+08:00"}) + "\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        import shutil
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_full_integration_seam(self) -> None:
        """Walk the entire Phase E pipeline end-to-end."""
        run_id = "smoke-001"
        work_item_id = "wi-smoke-001"

        # 1. Synthetic backlog with one ready task.
        backlog_path = self.tmp / "backlog.json"
        backlog_path.write_text(json.dumps({
            "tasks": [
                {
                    "id": work_item_id,
                    "status": "pending",
                    "priority": "high",
                    "deps": [],
                },
            ],
        }), encoding="utf-8")

        # 2. Scheduler picks the ready task.
        decision = scheduler.select_next_pending(backlog_path)
        self.assertIsNotNone(decision, "scheduler should pick the ready task")
        self.assertTrue(decision.is_actionable(), "decision should be actionable")
        self.assertEqual(decision.work_item_id, work_item_id)

        # 3. Claim lease.
        self.assertTrue(
            scheduler.claim_lease(work_item_id, run_id, self.lease_path),
            "lease claim should succeed on a fresh lease path",
        )

        # 4. Evidence writer materializes the evidence packet.
        # Pass evidence_root=self.evidence_dir so writer + gatekeeper agree
        # on the path. Without this, the writer's cwd-relative default
        # (`devkit/evidence/`) and the gate's evidence_dir would not match.
        # artifact_manifest.source must be a valid Phase D enum
        # (inner_sandbox / materialized_repo / external_signal / unknown).
        evidence_path = evidence_writer.write_run_evidence(
            run_dir=self.run_dir,
            run_id=run_id,
            work_item_id=work_item_id,
            tests_passed=10,
            tests_failed=0,
            cost_usd=0.42,
            budget_cap_usd=1.0,
            artifact_manifest={"source": "inner_sandbox"},
            evidence_source="inner_sandbox",
            evidence_root=self.evidence_dir,
        )
        self.assertTrue(evidence_path.exists(), "evidence packet should be on disk")
        # Path unification: writer writes under <evidence_root>/<run-id>/.
        self.assertEqual(
            evidence_path.parent.name, run_id,
            "evidence packet should sit under <root>/<run_id>/",
        )

        # 5. Gatekeeper reads the same path and returns a typed verdict.
        verdict = gatekeeper.evaluate_final_gate(
            run_id=run_id,
            work_item_id=work_item_id,
            evidence_dir=self.evidence_dir,
        )
        self.assertTrue(verdict.passed, f"smoke evidence should pass; got {verdict}")
        self.assertEqual(verdict.evidence_source, "inner_sandbox")

        # 6. Round-trip the verdict through write_verdict / load_verdict.
        verdict_json_path = self.run_dir / "verdict.json"
        gatekeeper.write_verdict(verdict, verdict_json_path)
        self.assertTrue(verdict_json_path.exists(), "verdict.json should be on disk")
        reloaded = gatekeeper.load_verdict(verdict_json_path)
        self.assertIsNotNone(reloaded)
        self.assertEqual(reloaded.verdict_id, verdict.verdict_id)
        self.assertEqual(reloaded.passed, verdict.passed)

        # 7. evaluate_run_gate (the typed end-of-run entry point from T6a)
        #    accepts the rdloop-style kwargs dict the autopilot passes.
        status_code, reasons, run_gate_verdict = gatekeeper.evaluate_run_gate(
            run_id=run_id,
            work_item_id=work_item_id,
            run_dir=self.run_dir,
            gate_inputs={
                "blocked": [],
                "review_blocked": False,
                "review_requested_changes": False,
                "tests_failed": False,
                "over_budget": False,
                "status_code": "ok",
                "gate": "GO",
                "tot_cost": 0.42,
                "tot_tokens": 1000,
            },
            evidence_dir=self.evidence_dir,
        )
        self.assertEqual(status_code, "ok")
        self.assertEqual(reasons, [])
        self.assertTrue(run_gate_verdict.passed)

        # 8. Release lease.
        scheduler.release_lease(self.lease_path)
        self.assertFalse(
            self.lease_path.exists(),
            "release_lease should remove the lease file",
        )

    def test_failure_path_translates_through_sanitizer(self) -> None:
        """Verify the T6a _sanitize_gate_inputs bridge: rdloop-style
        bool flags → Phase D enum codes, no TypeError on the writer."""
        from devkit.gatekeeper import (
            _sanitize_gate_inputs,
            _PHASE_B_GATE_FLAG_TO_PHASE_D,
        )
        writer_kwargs, derived = _sanitize_gate_inputs({
            "tests_failed": True,
            "over_budget": True,
            "blocked": True,
            "review_blocked": False,
            "tests_passed": 5,
            "cost_usd": 5.5,
            "budget_cap_usd": 1.0,
            "some_internal": "ignored",
        })
        # tests_failed True → TEST_REGRESSION; over_budget True →
        # BUDGET_EXCEEDED; blocked True → EVIDENCE_INVALID.
        self.assertIn("TEST_REGRESSION", derived)
        self.assertIn("BUDGET_EXCEEDED", derived)
        self.assertIn("EVIDENCE_INVALID", derived)
        # Writer-native scalars pass through.
        self.assertEqual(writer_kwargs.get("tests_passed"), 5)
        self.assertEqual(writer_kwargs.get("cost_usd"), 5.5)
        # Unknown keys dropped.
        self.assertNotIn("some_internal", writer_kwargs)
        # Phase-B flag → Phase D enum mapping is populated.
        self.assertEqual(
            _PHASE_B_GATE_FLAG_TO_PHASE_D["tests_failed"], "TEST_REGRESSION",
        )
        self.assertEqual(
            _PHASE_B_GATE_FLAG_TO_PHASE_D["over_budget"], "BUDGET_EXCEEDED",
        )


if __name__ == "__main__":
    unittest.main()