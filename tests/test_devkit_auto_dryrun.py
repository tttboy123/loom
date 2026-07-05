"""Unit tests for the pre-existing __main__.py + autoloop.py bug fixes
uncovered during Phase E integration smoke (2026-07-05).

Bug #1: devkit auto --dry-run / --as-json did not actually short-circuit
        before _autoloop.run_once() was called; the new --dry-run ordering
        we patched now lives at __main__.py:~1441.

Bug #2: autoloop.run_once() crashed on Phase B legacy flat-shape tasks
        whose `carrier` field is a string ("deepseek") instead of a dict.
        We normalize strings to apply to all 4 standard stages.

These tests pin both behaviours so they don't regress when the autopilot
is touched again.
"""

from __future__ import annotations

import json
import pathlib
import sys
import tempfile
import unittest


sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent))


class TestAutoloopRunOnce(unittest.TestCase):
    def test_carrier_string_applies_to_all_stages(self) -> None:
        """Phase B legacy flat-shape tasks carry `carrier` as a string,
        not a stage→carrier dict. run_once must not crash on string."""
        from devkit import autoloop
        out = autoloop.run_once({
            "task": "smoke",
            "stages": "plan,implement",
            "carrier": "deepseek",          # string, not dict
        })
        joined = " ".join(out["carriers"])
        # All 4 standard stages (plan/implement/verify/review) should
        # default to the supplied string.
        for stage in ("plan", "implement", "verify", "review"):
            self.assertIn(f"{stage}=deepseek", joined,
                          f"expected {stage}=deepseek in {joined!r}")
        # Default 4 stages means 4 carrier strings.
        self.assertEqual(len(out["carriers"]), 4)

    def test_carrier_dict_passes_through(self) -> None:
        """Phase C envelope-shape tasks carry `carrier` as a stage→name dict."""
        from devkit import autoloop
        out = autoloop.run_once({
            "task": "smoke",
            "stages": "plan,implement",
            "carrier": {"plan": "deepseek", "implement": "claude"},
        })
        joined = " ".join(out["carriers"])
        self.assertIn("plan=deepseek", joined)
        self.assertIn("implement=claude", joined)

    def test_carrier_none_falls_through(self) -> None:
        """carrier=None / missing should produce no carriers, not crash."""
        from devkit import autoloop
        out = autoloop.run_once({
            "task": "smoke",
            "stages": "plan",
        })
        self.assertIsInstance(out["carriers"], list)


class TestDevkitAutoDryRun(unittest.TestCase):
    """Smoke test: devkit auto --dry-run must NOT claim lease / call LLM."""

    def setUp(self) -> None:
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="dryrun_smoke_"))
        self.backlog = self.tmp / "backlog.json"
        self.backlog.write_text(json.dumps([
            {
                "id": "dryrun-smoke-ready",
                "status": "pending",
                "priority": "high",
                "deps": [],
                "task": "dry-run only",
                "stages": "plan",
                "carrier": "deepseek",
            },
        ]), encoding="utf-8")
        self.lease_path = self.tmp / "lease.json"

    def tearDown(self) -> None:
        import shutil
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_dry_run_does_not_create_lease_file(self) -> None:
        """`--dry-run --yes` should print task details and exit cleanly
        without claiming any lease or invoking an LLM."""
        import subprocess
        result = subprocess.run(
            [
                sys.executable, "-m", "devkit", "auto",
                "--backlog", str(self.backlog),
                "--lease-path", str(self.lease_path),
                "--dry-run", "--yes",
            ],
            cwd=pathlib.Path(__file__).resolve().parent.parent,
            capture_output=True, text=True, env={"PYTHONPATH": ".", "PATH": "/usr/bin:/usr/local/bin"},
            timeout=15,
        )
        # stdout should mention the picked task.
        self.assertIn("dryrun-smoke-ready", result.stdout,
                      f"stdout was: {result.stdout!r}")
        # Lease file MUST NOT exist after a dry-run.
        self.assertFalse(
            self.lease_path.exists(),
            f"dry-run should not claim a lease, but {self.lease_path} exists",
        )
        # Exit code should be 0 (clean exit).
        self.assertEqual(result.returncode, 0,
                         f"non-zero exit; stderr={result.stderr!r}")


if __name__ == "__main__":
    unittest.main()
