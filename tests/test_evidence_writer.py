"""Tests for devkit/evidence_writer.py (Phase D bridge).

Coverage:

1. Synthetic run_dir → evidence written to
   ``devkit/evidence/<run-id>/evidence_packet.json`` (use tmpdir).
2. The written file validates against ``evidence_packet.schema.json``.
3. Empty run_dir → evidence still written, no exception, optional fields null.
4. Integration with gatekeeper:
   ``gatekeeper.evaluate_final_gate(run_id, work_item_id, pathlib.Path("devkit/evidence"))``
   reads the file written by the writer — returns ``passed=True`` for a
   happy-path evidence.
5. Missing inputs (``run_id``/``work_item_id``) → ``ValueError``.
6. Atomic write: parent directory is created (``parents=True, exist_ok=True``).
7. Backward-compat fallback: gatekeeper still reads legacy
   ``<evidence_dir>/evidence.json`` when ``evidence_packet.json`` is absent.
"""
from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from devkit import evidence_writer as ew
from devkit import gatekeeper
from devkit.evidence_writer import (
    SCHEMA_PATH,
    PROTOCOL_VERSION as EW_PROTOCOL_VERSION,
    EVIDENCE_KIND,
    DEFAULT_EVIDENCE_ROOT,
    get_validator,
    reset_validator_cache,
)
from devkit.gatekeeper import (
    EVIDENCE_INNER_SANDBOX,
)


def _write(p: pathlib.Path, content) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(content, (dict, list)):
        content = json.dumps(content, ensure_ascii=False)
    p.write_text(content, encoding="utf-8")


# ============================================================================
# Test 1: synthetic run_dir → evidence written under tmpdir evidence_root
# ============================================================================
class TestWriteRunEvidenceHappy(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-ev"
        self.run_dir.mkdir()
        # lay down inputs that the writer should pick up
        (self.run_dir / "00-task.md").write_text(
            "# Task\n\nImplement HTTP 200 detection.\n",
            encoding="utf-8",
        )
        (self.run_dir / "99-gate.md").write_text(
            "# Gate 建议\n\ngate: GO\nstatus_code: suggested_go\n",
            encoding="utf-8",
        )
        self.evidence_root = pathlib.Path(self.tmp.name) / "evidence_root"

    def test_writes_to_run_id_subdir(self):
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-ev",
            work_item_id="wi-ev",
            evidence_root=self.evidence_root,
            status_code="suggested_go",
            gate="GO",
            cost_usd=0.001234,
            budget_cap_usd=1.00,
            tests_passed=10,
            tests_failed=0,
            artifact_manifest={"source": EVIDENCE_INNER_SANDBOX},
            evidence_source=EVIDENCE_INNER_SANDBOX,
        )
        self.assertTrue(out.exists())
        self.assertEqual(
            out,
            self.evidence_root / "r-ev" / "evidence_packet.json",
        )
        # parent dirs created automatically
        self.assertTrue((self.evidence_root / "r-ev").is_dir())

    def test_payload_shape(self):
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-ev",
            work_item_id="wi-ev",
            evidence_root=self.evidence_root,
            status_code="suggested_go",
            gate="GO",
            cost_usd=0.5,
            budget_cap_usd=5.0,
            tests_passed=3,
            tests_failed=0,
            artifact_manifest={"source": EVIDENCE_INNER_SANDBOX},
            evidence_source=EVIDENCE_INNER_SANDBOX,
        )
        payload = json.loads(out.read_text(encoding="utf-8"))
        self.assertEqual(payload["api_version"], EW_PROTOCOL_VERSION)
        self.assertEqual(payload["kind"], EVIDENCE_KIND)
        self.assertEqual(payload["metadata"]["run_id"], "r-ev")
        self.assertEqual(payload["metadata"]["work_item_id"], "wi-ev")
        self.assertEqual(payload["spec"]["source"], EVIDENCE_INNER_SANDBOX)
        # summary is a string (per evidence_packet.schema.json)
        self.assertIsInstance(payload["spec"]["summary"], str)
        self.assertIn("tests 3/3", payload["spec"]["summary"])
        # metrics carries the structured counters
        self.assertEqual(payload["spec"]["metrics"]["cost_usd"], 0.5)
        self.assertEqual(payload["spec"]["metrics"]["tests_passed"], 3)
        self.assertEqual(payload["spec"]["metrics"]["tests_failed"], 0)
        self.assertEqual(
            payload["spec"]["artifact_manifest"]["source"],
            EVIDENCE_INNER_SANDBOX,
        )
        # task excerpt surfaced from 00-task.md
        self.assertIn("HTTP 200", payload["spec"]["metrics"]["run_log_excerpt"])

    def test_excerpts_capped(self):
        # 00-task.md much longer than 600 chars
        long_task = "header\n" + ("long line " * 200) + "\n"
        (self.run_dir / "00-task.md").write_text(long_task, encoding="utf-8")
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-ev",
            work_item_id="wi-ev",
            evidence_root=self.evidence_root,
            status_code="suggested_go",
            gate="GO",
        )
        payload = json.loads(out.read_text(encoding="utf-8"))
        excerpt = payload["spec"]["metrics"]["run_log_excerpt"]
        self.assertLessEqual(len(excerpt), 600)


# ============================================================================
# Test 2: schema validation
# ============================================================================
class TestSchemaValidation(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-schema"
        self.run_dir.mkdir()

    def test_validates_against_evidence_packet_schema(self):
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-schema",
            work_item_id="wi-schema",
            status_code="suggested_go",
            gate="GO",
            cost_usd=0.1,
            budget_cap_usd=1.0,
            tests_passed=1,
            tests_failed=0,
            evidence_source=EVIDENCE_INNER_SANDBOX,
        )
        payload = json.loads(out.read_text(encoding="utf-8"))
        # Must not raise
        ew.validate_evidence_packet(payload)
        # And the global validator should accept it too
        ew.get_validator().validate(payload)
        self.assertTrue(SCHEMA_PATH.exists())


# ============================================================================
# Test 3: empty run_dir
# ============================================================================
class TestEmptyRunDir(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-empty"
        self.run_dir.mkdir()
        self.evidence_root = pathlib.Path(self.tmp.name) / "evidence_root"

    def test_no_exception_and_optional_fields_null(self):
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-empty",
            work_item_id="wi-empty",
            evidence_root=self.evidence_root,
        )
        # no exception, file present
        self.assertTrue(out.exists())
        payload = json.loads(out.read_text(encoding="utf-8"))
        # The optional fields collapse to null in the metrics dict.
        metrics = payload["spec"]["metrics"]
        self.assertIsNone(metrics.get("cost_usd"))
        self.assertIsNone(metrics.get("budget_cap_usd"))
        self.assertIsNone(metrics.get("tests_passed"))
        self.assertIsNone(metrics.get("tests_failed"))
        self.assertNotIn("gate_inputs", metrics)
        self.assertNotIn("run_log_excerpt", metrics)
        self.assertNotIn("gate_decision_excerpt", metrics)
        # summary still has a placeholder string
        self.assertIsInstance(payload["spec"]["summary"], str)
        self.assertTrue(payload["spec"]["summary"])
        # spec.source is absent (defaults to "unknown" at the gatekeeper only)
        self.assertNotIn("source", payload["spec"])

    def test_garbage_io_errors_dont_raise(self):
        # make run_dir read-restricted to ensure we tolerate missing files
        import os
        if hasattr(os, "chmod"):
            os.chmod(self.run_dir, 0o000)
            self.addCleanup(lambda: os.chmod(self.run_dir, 0o755))
        out = ew.write_run_evidence(
            self.run_dir,
            run_id="r-garbage",
            work_item_id="wi-garbage",
            evidence_root=self.evidence_root,
        )
        # Even if reading the task file fails, the packet must still write.
        self.assertTrue(out.exists())


# ============================================================================
# Test 4: integration with gatekeeper — happy path
# ============================================================================
class TestGatekeeperIntegration(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.run_dir = pathlib.Path(self.tmp.name) / "r-int"
        self.run_dir.mkdir()
        self.evidence_root = pathlib.Path(self.tmp.name) / "evidence"

    def test_gate_reads_packet_and_passes(self):
        ew.write_run_evidence(
            self.run_dir,
            run_id="r-int",
            work_item_id="wi-int",
            evidence_root=self.evidence_root,
            status_code="suggested_go",
            gate="GO",
            cost_usd=0.5,
            budget_cap_usd=5.0,
            tests_passed=10,
            tests_failed=0,
            artifact_manifest={"source": EVIDENCE_INNER_SANDBOX},
            evidence_source=EVIDENCE_INNER_SANDBOX,
        )
        # gatekeeper is pointed at the evidence root (matches the production
        # wiring where callers pass evidence_dir=devkit/evidence)
        verdict = gatekeeper.evaluate_final_gate(
            "r-int",
            "wi-int",
            self.evidence_root,
        )
        self.assertTrue(verdict.passed, msg=f"verdict={verdict}")
        self.assertEqual(verdict.failure_codes, [])
        self.assertEqual(verdict.evidence_source, EVIDENCE_INNER_SANDBOX)
        # diagnostic: gatekeeper used the per-run packet
        self.assertEqual(
            verdict.spec.get("lineage", {}).get("evidence_source_kind"),
            "evidence_packet",
        )

    def test_gate_reports_missing_when_neither_path_exists(self):
        verdict = gatekeeper.evaluate_final_gate(
            "r-missing",
            "wi-x",
            self.evidence_root,
        )
        self.assertFalse(verdict.passed)
        self.assertIn("EVIDENCE_MISSING", verdict.failure_codes)

    def test_gate_legacy_fallback(self):
        # Place a legacy evidence.json at the root (no per-run packet). Gate
        # must still read it.
        legacy = self.evidence_root / "evidence.json"
        legacy.parent.mkdir(parents=True, exist_ok=True)
        legacy.write_text(
            json.dumps(
                {
                    "summary": {
                        "tests_passed": 5,
                        "tests_failed": 0,
                        "cost_usd": 0.25,
                        "budget_cap_usd": 5.0,
                    },
                    "spec": {"source": EVIDENCE_INNER_SANDBOX},
                    "artifact_manifest": {"source": EVIDENCE_INNER_SANDBOX},
                }
            ),
            encoding="utf-8",
        )
        verdict = gatekeeper.evaluate_final_gate(
            "anything",
            "wi-x",
            self.evidence_root,
        )
        self.assertTrue(verdict.passed)
        self.assertEqual(
            verdict.spec.get("lineage", {}).get("evidence_source_kind"),
            "evidence_legacy",
        )


# ============================================================================
# Test 5: input validation
# ============================================================================
class TestInputValidation(unittest.TestCase):
    def test_missing_run_id_raises(self):
        with tempfile.TemporaryDirectory() as td:
            run_dir = pathlib.Path(td) / "r"
            run_dir.mkdir()
            with self.assertRaises(ValueError):
                ew.write_run_evidence(
                    run_dir,
                    run_id="",
                    work_item_id="wi",
                )

    def test_missing_work_item_id_raises(self):
        with tempfile.TemporaryDirectory() as td:
            run_dir = pathlib.Path(td) / "r"
            run_dir.mkdir()
            with self.assertRaises(ValueError):
                ew.write_run_evidence(
                    run_dir,
                    run_id="r",
                    work_item_id="",
                )


# ============================================================================
# Test 6: atomic write — partial writes won't leave a stale file
# ============================================================================
class TestAtomicWrite(unittest.TestCase):
    def test_existing_packet_overwritten_atomically(self):
        with tempfile.TemporaryDirectory() as td:
            run_dir = pathlib.Path(td) / "r"
            run_dir.mkdir()
            evidence_root = pathlib.Path(td) / "evidence"
            # First write
            out1 = ew.write_run_evidence(
                run_dir,
                run_id="r",
                work_item_id="wi",
                evidence_root=evidence_root,
                status_code="suggested_go",
                gate="GO",
            )
            payload1 = json.loads(out1.read_text(encoding="utf-8"))
            self.assertEqual(payload1["spec"]["metrics"].get("cost_usd"), None)
            # Second write overwrites cleanly
            out2 = ew.write_run_evidence(
                run_dir,
                run_id="r",
                work_item_id="wi",
                evidence_root=evidence_root,
                status_code="suggested_go",
                gate="GO",
                cost_usd=0.99,
            )
            payload2 = json.loads(out2.read_text(encoding="utf-8"))
            self.assertEqual(out1, out2)
            self.assertEqual(payload2["spec"]["metrics"]["cost_usd"], 0.99)
            # No leftover tmp file
            tmp_path = out2.with_name(".evidence_packet.json.tmp")
            self.assertFalse(tmp_path.exists())


if __name__ == "__main__":
    unittest.main()
