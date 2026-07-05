"""Tests for devkit/protocol.py:write_run_protocol_bundle — rdloop Phase C bridge.

This is the function that ``devkit/rdloop.py`` calls (wrapped in
``try: ... except Exception: pass``) at the end of every run to emit
``run_dir/protocol_bundle.json``. The function used to be missing on
main, which made every run_loop end with a silent ImportError — the
bundle file was never written. These tests pin the contract.

Coverage targets:

  1. Synthetic run_dir (00-task.md + 99-gate.md) → bundle written,
     schema-valid, contains the objective text.
  2. Empty run_dir (no files) → bundle still written, no exception,
     all optional fields null.
  3. Round-trip: load JSON back, validate against the schema → pass.
  4. Atomic write: the .tmp + rename pattern is used (a simulated
     pre-rename crash must not leave a half-written target file).
  5. Keyword args match what rdloop.py actually passes (smoke).
  6. Tolerant of missing events.jsonl, blocked=None, etc.
  7. Schema is Draft 2020-12 and rejects an obviously broken payload.
"""
from __future__ import annotations

import json
import os
import pathlib
import sys
import tempfile
import unittest

# Make sure the schema/loader are importable as siblings of the other
# protocol_schemas modules.
from jsonschema import Draft202012Validator

from devkit import protocol
from devkit.protocol import (
    PROTOCOL_BUNDLE_FILENAME,
    PROTOCOL_BUNDLE_KIND,
    PROTOCOL_VERSION,
    ValidationFailed,
    _atomic_write_text,
    get_validator,
    validate,
    write_run_protocol_bundle,
)
from devkit.protocol_schemas import protocol_bundle_schema as _pbs_loader

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent


# ---------------------------------------------------------------------------- #
# Helpers
# ---------------------------------------------------------------------------- #
def _make_run_dir(parent: pathlib.Path, name: str = "run-test") -> pathlib.Path:
    rd = parent / name
    rd.mkdir(parents=True, exist_ok=True)
    return rd


def _load_schema_via_validator() -> Draft202012Validator:
    """Return the validator registered for ``PROTOCOL_BUNDLE_KIND``.

    Going through the cached registry ensures the schema file on disk
    matches the one the production validator loads.
    """
    return get_validator(PROTOCOL_BUNDLE_KIND)


# ---------------------------------------------------------------------------- #
# 1. Synthetic run_dir — happy path with both task + gate files
# ---------------------------------------------------------------------------- #
class TestSyntheticRunDir(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self._tmp.name)
        self.run_dir = _make_run_dir(self.tmp_path)
        self.run_id = "run-2026-07-05T00-00-00Z"
        self.objective_text = "Implement a green field with one oak tree."
        (self.run_dir / "00-task.md").write_text(
            f"# 任务\n\n{self.objective_text}\n",
            encoding="utf-8",
        )
        (self.run_dir / "99-gate.md").write_text(
            "# Gate 建议\n\n- gate: GO\n- status_code: ok\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_bundle_written_and_schema_valid(self) -> None:
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id=self.run_id,
            objective=self.objective_text,
            delivery_mode="apply",
            task_kind="feature",
            status_code="ok",
            gate="GO",
            candidate_state="materialized",
            review_text="Looks good.",
            blocked=None,
            tests_failed=False,
        )
        self.assertTrue(out.exists())
        self.assertEqual(out.name, PROTOCOL_BUNDLE_FILENAME)
        bundle = json.loads(out.read_text(encoding="utf-8"))
        self.assertEqual(bundle["api_version"], PROTOCOL_VERSION)
        self.assertEqual(bundle["kind"], PROTOCOL_BUNDLE_KIND)
        self.assertEqual(bundle["metadata"]["run_id"], self.run_id)
        # Objective text is in spec.objective (from in-memory arg).
        self.assertEqual(bundle["spec"]["objective"], self.objective_text)
        # sources point at the real files.
        self.assertEqual(bundle["spec"]["sources"]["task_path"], "00-task.md")
        self.assertEqual(bundle["spec"]["sources"]["gate_path"], "99-gate.md")
        self.assertIsNone(bundle["spec"]["sources"]["events_path"])
        # Validate against the schema.
        validate(PROTOCOL_BUNDLE_KIND, bundle)


# ---------------------------------------------------------------------------- #
# 2. Empty run_dir — every source missing → still writes, all optionals null
# ---------------------------------------------------------------------------- #
class TestEmptyRunDir(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self._tmp.name)
        self.run_dir = self.tmp_path / "empty-run"
        self.run_dir.mkdir(parents=True, exist_ok=True)
        self.run_id = "run-empty"

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_empty_run_dir_still_writes_bundle(self) -> None:
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id=self.run_id,
            objective=None,
        )
        self.assertTrue(out.exists())
        bundle = json.loads(out.read_text(encoding="utf-8"))
        # No 00-task.md → objective is None, objective_path None.
        self.assertIsNone(bundle["spec"]["objective"])
        self.assertIsNone(bundle["spec"]["objective_path"])
        # No sources present.
        self.assertEqual(
            bundle["spec"]["sources"],
            {"task_path": None, "gate_path": None, "events_path": None},
        )
        # No events either.
        self.assertEqual(bundle["spec"]["events"], [])
        # Default status fields.
        self.assertEqual(bundle["spec"]["status"]["blocked"], [])
        self.assertFalse(bundle["spec"]["status"]["tests_failed"])
        # Artifacts block is fully populated but every entry is None.
        for key in (
            "review_text",
            "gate_spec",
            "output_protocol",
            "artifact_manifest",
            "collect",
            "gate_result",
        ):
            self.assertIsNone(bundle["spec"]["artifacts"][key], msg=key)
        # Still schema-valid.
        validate(PROTOCOL_BUNDLE_KIND, bundle)


# ---------------------------------------------------------------------------- #
# 3. Round-trip — write, load JSON back, validate via Draft202012Validator
# ---------------------------------------------------------------------------- #
class TestRoundTripValidation(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self._tmp.name)
        self.run_dir = _make_run_dir(self.tmp_path, "round-trip")

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_round_trip_passes_draft_2020_12_validator(self) -> None:
        (self.run_dir / "00-task.md").write_text("Build a bridge.\n", encoding="utf-8")
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-roundtrip",
            objective="Build a bridge.",
            delivery_mode="apply",
            task_kind="feature",
            status_code="ok",
            gate="GO",
            candidate_state="materialized",
            review_text="Reviewed.",
            blocked=["budget_exceeded"],
            tests_failed=False,
            gate_spec={"mode": "artifact_json", "artifact_path": "build/x.json", "checks": []},
            output_protocol={"producer_evidence": {"ok": True}},
            artifact_manifest={"kind": "ArtifactManifest"},
            collect={"tests": [{"name": "t1", "outcome": "passed"}]},
            gate_result={"ok": True},
        )
        # Load raw JSON and revalidate with a *fresh* Draft202012Validator
        # built directly from the loader file — this proves the on-disk
        # schema matches what the function uses internally.
        payload = json.loads(out.read_text(encoding="utf-8"))
        direct_validator = Draft202012Validator(_pbs_loader.SCHEMA)
        errors = list(direct_validator.iter_errors(payload))
        self.assertEqual(errors, [], f"schema errors: {[e.message for e in errors]}")
        # And the cached validator from protocol.py also accepts it.
        _load_schema_via_validator().validate(payload)


# ---------------------------------------------------------------------------- #
# 4. Atomic write — .tmp + rename pattern
# ---------------------------------------------------------------------------- #
class TestAtomicWrite(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self._tmp.name)
        self.run_dir = _make_run_dir(self.tmp_path, "atomic-run")
        (self.run_dir / "00-task.md").write_text("Atomic run.\n", encoding="utf-8")

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_no_tmp_file_left_after_success(self) -> None:
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-atomic",
            objective="Atomic run.",
        )
        self.assertTrue(out.exists())
        # The .tmp sibling should be gone after the rename.
        tmp_sibling = out.with_name(f".{out.name}.tmp")
        self.assertFalse(tmp_sibling.exists())

    def test_simulated_crash_before_rename_leaves_no_half_written_target(self) -> None:
        """Patch the atomic helper to raise between write and rename, then
        verify the *target* file is NOT present (only a .tmp orphan may
        exist). The .tmp file itself is allowed because that's what a
        real crash would leave behind; the contract is that the target
        path is never written except via a successful rename."""
        from devkit import protocol as _proto

        original = _proto._atomic_write_text

        def _boom(path: pathlib.Path, text: str) -> None:
            # Simulate a crash after the tmp file is written but before
            # os.replace. We mimic the real implementation up to that
            # point so we leave a real .tmp on disk, then raise.
            path.parent.mkdir(parents=True, exist_ok=True)
            tmp = path.with_name(f".{path.name}.tmp")
            tmp.write_text(text, encoding="utf-8")
            raise RuntimeError("simulated crash mid-write")

        _proto._atomic_write_text = _boom  # type: ignore[assignment]
        try:
            with self.assertRaises(RuntimeError):
                write_run_protocol_bundle(
                    run_dir=self.run_dir,
                    run_id="run-crash",
                    objective="Crash run.",
                )
        finally:
            _proto._atomic_write_text = original  # type: ignore[assignment]

        target = self.run_dir / PROTOCOL_BUNDLE_FILENAME
        tmp_sibling = target.with_name(f".{target.name}.tmp")
        # Target must NOT exist after a failed write.
        self.assertFalse(target.exists(), "target bundle must not exist after failed write")
        # .tmp may or may not exist depending on the OS; either way the
        # invariant is: target is absent. We assert that.
        # (cleanup any leftover .tmp to keep the test directory tidy)
        if tmp_sibling.exists():
            tmp_sibling.unlink()


# ---------------------------------------------------------------------------- #
# 5. Tolerance — odd inputs from rdloop must not raise
# ---------------------------------------------------------------------------- #
class TestTolerance(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.tmp_path = pathlib.Path(self._tmp.name)
        self.run_dir = _make_run_dir(self.tmp_path, "tol-run")

    def tearDown(self) -> None:
        self._tmp.cleanup()

    def test_events_jsonl_is_parsed_when_present(self) -> None:
        events_path = self.run_dir / "events.jsonl"
        events_path.write_text(
            '{"ts": "2026-07-05T00:00:00Z", "kind": "stage_start"}\n'
            '{"ts": "2026-07-05T00:00:01Z", "kind": "stage_end"}\n',
            encoding="utf-8",
        )
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-ev",
            objective=None,
        )
        bundle = json.loads(out.read_text(encoding="utf-8"))
        self.assertEqual(len(bundle["spec"]["events"]), 2)
        self.assertEqual(bundle["spec"]["events"][0]["kind"], "stage_start")
        # sources.events_path is now populated
        self.assertEqual(bundle["spec"]["sources"]["events_path"], "events.jsonl")

    def test_blocked_accepts_none_str_or_iterable(self) -> None:
        # None → []
        out_none = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-bn",
            objective=None,
            blocked=None,
        )
        self.assertEqual(json.loads(out_none.read_text(encoding="utf-8"))["spec"]["status"]["blocked"], [])
        # str → [str] (whitespace-only → [])
        out_str = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-bs",
            objective=None,
            blocked="budget_exceeded",
        )
        self.assertEqual(
            json.loads(out_str.read_text(encoding="utf-8"))["spec"]["status"]["blocked"],
            ["budget_exceeded"],
        )
        out_blank = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-bb",
            objective=None,
            blocked="   ",
        )
        self.assertEqual(json.loads(out_blank.read_text(encoding="utf-8"))["spec"]["status"]["blocked"], [])
        # list → strings (None entries are filtered, numbers are stringified)
        out_lst = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-bl",
            objective=None,
            blocked=["a", "b", None, 42],
        )
        self.assertEqual(
            json.loads(out_lst.read_text(encoding="utf-8"))["spec"]["status"]["blocked"],
            ["a", "b", "42"],
        )

    def test_write_returns_absolute_path(self) -> None:
        out = write_run_protocol_bundle(
            run_dir=self.run_dir,
            run_id="run-path",
            objective=None,
        )
        self.assertTrue(out.is_absolute())
        self.assertEqual(out.parent, self.run_dir)


# ---------------------------------------------------------------------------- #
# 6. Schema self-check — a deliberately malformed bundle is rejected
# ---------------------------------------------------------------------------- #
class TestSchemaSelfCheck(unittest.TestCase):
    def test_schema_rejects_bogus_payload(self) -> None:
        # Missing required fields.
        with self.assertRaises(ValidationFailed):
            validate(PROTOCOL_BUNDLE_KIND, {"api_version": PROTOCOL_VERSION, "kind": PROTOCOL_BUNDLE_KIND})
        # Wrong kind.
        with self.assertRaises(ValidationFailed):
            validate(
                PROTOCOL_BUNDLE_KIND,
                {
                    "api_version": PROTOCOL_VERSION,
                    "kind": "NotABundle",
                    "metadata": {"id": "x", "run_id": "y"},
                    "spec": {"objective": None, "sources": {}, "status": {}},
                },
            )

    def test_schema_loader_exposes_draft_2020_12(self) -> None:
        # Sanity: the on-disk schema declares Draft 2020-12.
        self.assertEqual(
            _pbs_loader.SCHEMA.get("$schema"),
            "https://json-schema.org/draft/2020-12/schema",
        )
        self.assertEqual(_pbs_loader.SCHEMA["properties"]["kind"]["const"], PROTOCOL_BUNDLE_KIND)


if __name__ == "__main__":
    unittest.main(verbosity=2)