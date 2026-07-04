"""Phase B spec integration tests for rdloop.

These tests verify that the six protocol schemas that were previously
"decorative" are now actually "functional" inside rdloop:

  1. ``validate_goal_spec`` rejects malformed GoalSpec payloads via
     ``jsonschema`` against ``devkit/protocol_schemas/goal_spec.schema.json``.
  2. ``validate_work_item`` / ``pick_next_pending`` enforce the WorkItem
     contract at backlog iteration time.
  3. ``evaluate_final_gate`` loads ``gate_spec.schema.json`` as a baseline
     and produces the same verdict as the legacy ``_resolve_gate_status``.
  4. ``evidence_packet_artifact_source`` classifies each artifact with a
     ``source: 'inner_sandbox' | 'materialized_repo' | 'runtime_support' | 'declared'``.
  5. ``devkit.state_writer.transition_task`` consults ``devkit.budget.check``
     and rejects state mutations that would exceed the per-call USD limit.

Total: 24 tests across 6 classes.
"""
from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest
from typing import Any
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))


# --------------------------------------------------------------------------- #
# Fixtures
# --------------------------------------------------------------------------- #
def _valid_goal_spec(goal_id: str = "g-test-01") -> dict[str, Any]:
    return {
        "api_version": "loom.dev/v1",
        "kind": "GoalSpec",
        "metadata": {"id": goal_id, "created_by": "test"},
        "spec": {
            "objective": "add JSON validation to rdloop entry",
            "scope": "repo",
            "owner_role": "orchestrator",
            "priority": "medium",
            "acceptance": ["validate_goal_spec raises on bad input"],
            "source": "human",
        },
    }


def _valid_work_item(wi_id: str = "w-test-01", status: str = "pending") -> dict[str, Any]:
    return {
        "api_version": "loom.dev/v1",
        "kind": "WorkItem",
        "metadata": {"id": wi_id, "goal_id": "g-test-01"},
        "spec": {
            "objective": "add validate_goal_spec to rdloop",
            "owner_role": "implementer",
            "status": status,
            "acceptance": ["validate_goal_spec raises on bad input"],
            "input_refs": ["loom://goals/g-test-01"],
        },
    }


# --------------------------------------------------------------------------- #
# 1. GoalSpec validation (3 tests)
# --------------------------------------------------------------------------- #
class GoalSpecValidationTest(unittest.TestCase):
    def test_validate_goal_spec_accepts_valid_payload(self):
        from devkit.rdloop import validate_goal_spec

        payload = _valid_goal_spec()
        result = validate_goal_spec(payload)
        self.assertEqual(result["kind"], "GoalSpec")
        self.assertEqual(result["metadata"]["id"], "g-test-01")
        # Ensure the returned object is the same normalized dict.
        self.assertEqual(result["spec"]["objective"], payload["spec"]["objective"])

    def test_validate_goal_spec_rejects_missing_objective(self):
        from devkit.rdloop import validate_goal_spec, SpecValidationError

        bad = {
            "api_version": "loom.dev/v1",
            "kind": "GoalSpec",
            "metadata": {"id": "g-bad"},
            "spec": {"scope": "repo"},  # objective missing
        }
        with self.assertRaises(SpecValidationError) as ctx:
            validate_goal_spec(bad)
        self.assertEqual(ctx.exception.kind, "GoalSpec")
        # The jsonschema error message should mention 'objective'.
        self.assertIn("objective", str(ctx.exception))

    def test_validate_goal_spec_rejects_wrong_kind_constant(self):
        from devkit.rdloop import validate_goal_spec, SpecValidationError

        bad = {
            "api_version": "loom.dev/v1",
            "kind": "SomethingElse",  # must be const "GoalSpec"
            "metadata": {"id": "g-bad"},
            "spec": {"objective": "x"},
        }
        with self.assertRaises(SpecValidationError):
            validate_goal_spec(bad)


# --------------------------------------------------------------------------- #
# 2. WorkItem validation (3 tests)
# --------------------------------------------------------------------------- #
class WorkItemValidationTest(unittest.TestCase):
    def test_validate_work_item_accepts_valid_payload(self):
        from devkit.rdloop import validate_work_item

        result = validate_work_item(_valid_work_item())
        self.assertEqual(result["kind"], "WorkItem")
        self.assertEqual(result["metadata"]["goal_id"], "g-test-01")

    def test_validate_work_item_rejects_missing_objective(self):
        from devkit.rdloop import validate_work_item, SpecValidationError

        bad = {
            "api_version": "loom.dev/v1",
            "kind": "WorkItem",
            "metadata": {"id": "w-bad"},
            "spec": {"status": "pending"},
        }
        with self.assertRaises(SpecValidationError) as ctx:
            validate_work_item(bad)
        self.assertEqual(ctx.exception.kind, "WorkItem")
        self.assertIn("objective", str(ctx.exception))

    def test_pick_next_pending_returns_first_pending_work_item(self):
        from devkit.rdloop import pick_next_pending

        items = [
            {"id": "a", "status": "done"},
            _valid_work_item("w1", "running"),
            _valid_work_item("w2", "pending"),
        ]
        picked = pick_next_pending(items)
        self.assertIsNotNone(picked)
        self.assertEqual(picked["metadata"]["id"], "w2")
        self.assertEqual(picked["spec"]["status"], "pending")

    def test_pick_next_pending_returns_none_when_empty(self):
        from devkit.rdloop import pick_next_pending

        self.assertIsNone(pick_next_pending([]))
        self.assertIsNone(pick_next_pending(None))
        # All in non-pending states
        items = [
            _valid_work_item("w1", "done"),
            _valid_work_item("w2", "failed"),
        ]
        self.assertIsNone(pick_next_pending(items))


# --------------------------------------------------------------------------- #
# 3. GateSpec schema-aware final gate (3 tests)
# --------------------------------------------------------------------------- #
class GateSpecSchemaTest(unittest.TestCase):
    def test_evaluate_final_gate_matches_resolve_gate_status(self):
        """The new evaluate_final_gate must delegate to the legacy helper
        with bit-for-bit identical output (no behaviour drift)."""
        from devkit.rdloop import evaluate_final_gate, _resolve_gate_status

        kwargs = dict(
            blocked=["verify"],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        new_status, new_reasons = evaluate_final_gate(**kwargs)
        old_status, old_reasons = _resolve_gate_status(**kwargs)
        self.assertEqual(new_status, old_status)
        self.assertEqual(new_reasons, old_reasons)

    def test_evaluate_final_gate_loads_gate_spec_schema_as_baseline(self):
        """Even when no gate_spec is provided, evaluate_final_gate must
        actually read gate_spec.schema.json (decorative → functional)."""
        from devkit.rdloop import _load_schema, evaluate_final_gate

        # The schema is loaded lazily; this also returns it for assertions.
        schema = _load_schema("GateSpec")
        self.assertEqual(schema.get("title"), "GateSpec")
        # And evaluate_final_gate still works without a gate_spec argument.
        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(verdict, "suggested_go")
        self.assertEqual(reasons, [])

    def test_evaluate_final_gate_validates_provided_gate_spec_dict(self):
        """When a GateSpec dict is passed, evaluate_final_gate must validate
        it against the schema. A valid GateSpec dict passes; a malformed
        one raises SpecValidationError."""
        from devkit.rdloop import evaluate_final_gate, SpecValidationError

        valid_gate = {
            "api_version": "loom.dev/v1",
            "kind": "GateSpec",
            "metadata": {"id": "gs-1"},
            "spec": {"mode": "pytest", "verdict": "go"},
        }
        status, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
            gate_spec=valid_gate,
        )
        # tests_failed=False, no blocked → goes to the success path
        self.assertEqual(status, "suggested_go")
        self.assertEqual(reasons, [])

        bad_gate = {
            "api_version": "loom.dev/v1",
            "kind": "GateSpec",
            "metadata": {"id": "gs-bad"},
            "spec": {"verdict": "go"},  # 'mode' is required by schema
        }
        with self.assertRaises(SpecValidationError):
            evaluate_final_gate(
                blocked=[],
                review_blocked=False,
                review_requested_changes=False,
                tests_failed=False,
                over_budget=False,
                gate_spec=bad_gate,
            )


# --------------------------------------------------------------------------- #
# 4. EvidencePacket source classification (3 tests)
# --------------------------------------------------------------------------- #
class EvidencePacketSourceTest(unittest.TestCase):
    def test_evidence_packet_artifact_source_inner_sandbox(self):
        from devkit.rdloop import evidence_packet_artifact_source

        with tempfile.TemporaryDirectory() as d:
            workspace = pathlib.Path(d)
            build = workspace / "build"
            build.mkdir()
            (build / "pkg.py").write_text("X = 1", encoding="utf-8")
            self.assertEqual(
                evidence_packet_artifact_source("pkg.py", workspace=workspace, build_dir=build),
                "inner_sandbox",
            )

    def test_evidence_packet_artifact_source_runtime_support(self):
        from devkit.rdloop import evidence_packet_artifact_source

        with tempfile.TemporaryDirectory() as d:
            workspace = pathlib.Path(d)
            build = workspace / "build"
            build.mkdir()
            # harness/, verify/, devkit/ are runtime support by convention.
            for rel in ("harness/render.py", "verify/check.py", "devkit/foo.py"):
                (build / rel).parent.mkdir(parents=True, exist_ok=True)
                (build / rel).write_text("", encoding="utf-8")
                self.assertEqual(
                    evidence_packet_artifact_source(rel, workspace=workspace, build_dir=build),
                    "runtime_support",
                    f"expected runtime_support for {rel}",
                )

    def test_evidence_packet_artifact_source_materialized_repo(self):
        """When build_dir == workspace, files are materialized to the repo
        (i.e. applied in-place). Source should be 'materialized_repo'."""
        from devkit.rdloop import evidence_packet_artifact_source

        with tempfile.TemporaryDirectory() as d:
            ws = pathlib.Path(d)
            (ws / "module.py").write_text("Y = 2", encoding="utf-8")
            self.assertEqual(
                evidence_packet_artifact_source("module.py", workspace=ws, build_dir=ws),
                "materialized_repo",
            )


# --------------------------------------------------------------------------- #
# 5. Budget hook in state_writer (4 tests)
# --------------------------------------------------------------------------- #
class BudgetHookTest(unittest.TestCase):
    def _make_backlog(self, tmp: pathlib.Path) -> pathlib.Path:
        path = tmp / "backlog.json"
        from devkit import state_writer
        state_writer.enqueue_task(
            backlog_path=path,
            item={"id": "t-budget", "task": "demo", "status": "pending"},
            actor="tester", source_task_id="t-budget", reason="setup",
        )
        return path

    def test_budget_check_within_limit_returns_ok(self):
        from devkit.budget import check
        result = check("t-x", 1.0, limit_usd=5.0)
        self.assertTrue(result["ok"])
        self.assertEqual(result["task_id"], "t-x")
        self.assertTrue(result["checked"])
        self.assertEqual(result["limit_usd"], 5.0)

    def test_budget_check_over_limit_raises(self):
        from devkit.budget import check, BudgetExceeded
        with self.assertRaises(BudgetExceeded) as ctx:
            check("t-y", 10.0, limit_usd=2.0)
        self.assertEqual(ctx.exception.task_id, "t-y")
        self.assertEqual(ctx.exception.cost_usd, 10.0)
        self.assertEqual(ctx.exception.limit_usd, 2.0)

    def test_transition_task_within_budget_accepted(self):
        from devkit import state_writer
        with tempfile.TemporaryDirectory() as d:
            path = self._make_backlog(pathlib.Path(d))
            rec = state_writer.transition_task(
                backlog_path=path, task_id="t-budget", to_status="running",
                actor="tester", source_task_id="t-budget", reason="start",
                estimated_cost_usd=0.5, cost_limit_usd=2.0,
            )
            self.assertEqual(rec["outcome"], "accepted")

    def test_transition_task_over_budget_rejected(self):
        from devkit import state_writer
        with tempfile.TemporaryDirectory() as d:
            path = self._make_backlog(pathlib.Path(d))
            with self.assertRaises(state_writer.TransitionError) as ctx:
                state_writer.transition_task(
                    backlog_path=path, task_id="t-budget", to_status="running",
                    actor="tester", source_task_id="t-budget", reason="too_expensive",
                    estimated_cost_usd=100.0, cost_limit_usd=2.0,
                )
            self.assertEqual(ctx.exception.failure_code, "BUDGET_EXCEEDED")

    def test_transition_task_without_cost_param_unchanged(self):
        """Backward compatibility: callers that omit estimated_cost_usd see
        the same behaviour as before Phase B."""
        from devkit import state_writer
        with tempfile.TemporaryDirectory() as d:
            path = self._make_backlog(pathlib.Path(d))
            rec = state_writer.transition_task(
                backlog_path=path, task_id="t-budget", to_status="running",
                actor="tester", source_task_id="t-budget", reason="start",
            )
            self.assertEqual(rec["outcome"], "accepted")


# --------------------------------------------------------------------------- #
# 6. Integration: end-to-end (4 tests)
# --------------------------------------------------------------------------- #
class SpecIntegrationTest(unittest.TestCase):
    def test_run_once_validates_goal_spec_payload(self):
        """autoloop.run_once() must call validate_goal_spec on GoalSpec input."""
        from devkit.autoloop import run_once
        from devkit.rdloop import SpecValidationError

        bad = {
            "kind": "GoalSpec", "api_version": "loom.dev/v1",
            "metadata": {"id": "g-bad"},
            "spec": {},  # objective missing
        }
        with self.assertRaises(SpecValidationError):
            run_once(bad)

    def test_run_once_passes_through_non_goal_spec_dict(self):
        """Backward compatibility: plain task dicts (no GoalSpec shape) skip
        schema validation entirely."""
        from devkit.autoloop import run_once

        out = run_once({"task": "demo", "stages": "implement", "carrier": {}})
        self.assertEqual(out["task"], "demo")
        self.assertEqual(out["stages"], "implement")

    def test_artifact_manifest_build_manifest_produces_schema_aligned_dict(self):
        """The Phase B schema-aligned build_manifest must produce a dict
        that Draft202012Validator accepts."""
        from devkit.artifact_manifest import build_manifest, validate_manifest

        manifest = build_manifest(
            manifest_id="m1",
            entries=[{"path": "src/pkg.py"}, "docs/readme.md"],
            run_id="r1",
            workspace_path=".",
            candidate_path="build",
            lineage={"run_id": "r1"},
            source="loom_runtime",
        )
        self.assertEqual(manifest["kind"], "ArtifactManifest")
        self.assertEqual(manifest["api_version"], "loom.dev/v1")
        self.assertEqual(manifest["metadata"]["id"], "m1")
        # Must round-trip the schema validator.
        validate_manifest(manifest)  # raises on failure

    def test_artifact_manifest_rejects_invalid_source(self):
        from devkit.artifact_manifest import build_manifest, ManifestBuildError

        with self.assertRaises(ManifestBuildError):
            build_manifest(manifest_id="m-bad", entries=["x.py"], source="unknown")

    def test_build_manifest_accepts_rel_path_key(self):
        """Input alias: ``rel_path`` is accepted as a more descriptive synonym
        for ``path`` on input dicts. The schema-validated output must still
        round-trip — i.e. the dict produced contains ``path`` (not
        ``rel_path``) but is otherwise unchanged."""
        from devkit.artifact_manifest import build_manifest, validate_manifest

        manifest = build_manifest(
            manifest_id="m-relpath",
            entries=[{"rel_path": "src/pkg.py", "source": "inner_sandbox"}],
            run_id="r-relpath",
        )
        # Output uses 'path', not 'rel_path'.
        entry = manifest["spec"]["entries"][0]
        self.assertEqual(entry["path"], "src/pkg.py")
        self.assertNotIn("rel_path", entry)
        # Other metadata keys flow through unchanged.
        self.assertEqual(entry["source"], "inner_sandbox")
        # And the whole manifest still validates against the schema.
        validate_manifest(manifest)

    def test_build_manifest_normalizes_rel_path_to_path(self):
        """Even when callers pass both keys (or only ``rel_path``) the output
        dict must contain exactly one canonical ``path`` key — never both
        ``path`` and ``rel_path`` simultaneously."""
        from devkit.artifact_manifest import build_manifest

        # Mixed input — strings, dicts with path, dicts with rel_path, dicts
        # with both keys (the latter is a caller bug, but we still produce a
        # single, normalized output).
        manifest = build_manifest(
            manifest_id="m-norm",
            entries=[
                "bare.txt",
                {"path": "with_path.txt"},
                {"rel_path": "with_relpath.txt"},
                {"path": "winner.txt", "rel_path": "ignored.txt"},
            ],
        )
        paths = [e["path"] for e in manifest["spec"]["entries"]]
        self.assertEqual(
            paths,
            ["bare.txt", "with_path.txt", "with_relpath.txt", "winner.txt"],
        )
        for entry in manifest["spec"]["entries"]:
            # No entry should carry both keys; with the last fixture, the
            # 'path' value wins (matches the original 'path' precedence).
            self.assertNotIn("rel_path", entry, f"unexpected rel_path in {entry!r}")

    def test_rdloop_evaluate_final_gate_wired_into_run_loop_signature(self):
        """Smoke test: evaluate_final_gate is exported from rdloop, callable
        with the same kwargs run_loop uses, and produces a valid verdict."""
        from devkit.rdloop import evaluate_final_gate

        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=True,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(verdict, "review_timeout")

        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=True,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(verdict, "review_request_changes")

        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=True,
            over_budget=False,
        )
        self.assertEqual(verdict, "tests_failed")
        self.assertIn("构建测试/Eval 未过", reasons)

        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=True,
        )
        self.assertEqual(verdict, "over_budget")
        self.assertIn("超预算", reasons)

        verdict, reasons = evaluate_final_gate(
            blocked=[],
            review_blocked=False,
            review_requested_changes=False,
            tests_failed=False,
            over_budget=False,
        )
        self.assertEqual(verdict, "suggested_go")
        self.assertEqual(reasons, [])


if __name__ == "__main__":
    unittest.main()
