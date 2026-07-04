"""
Unit tests for devkit.gate.opensource.

Each test builds a tiny synthetic repo in a tmp dir and exercises one or more
checks, asserting the verdict and details.
"""

from __future__ import annotations

import textwrap
import unittest
from pathlib import Path

from devkit.gate.opensource import (
    BLOCKER,
    OPEN_GO,
    OPEN_GO_WITH_WARNINGS,
    OPEN_NO_GO,
    WARNING,
    OpenSourceCheck,
    default_checks,
    evaluate_opensource_gate,
)


class _RepoFixture:
    """Tiny helper to build a synthetic repo dir."""

    def __init__(self, root: Path):
        self.root = root
        # Create .gitignore as a starting baseline (tests override as needed)
        (root / ".gitignore").write_text(
            textwrap.dedent(
                """\
                .DS_Store
                __pycache__/
                *.pyc
                .env
                """
            ),
            encoding="utf-8",
        )

    def write(self, rel: str, content: str) -> Path:
        p = self.root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(content, encoding="utf-8")
        return p


def _good_license() -> str:
    """A non-trivial LICENSE so LicensePresent passes."""
    return (
        "MIT License\n\n"
        "Copyright (c) 2026 Loom Contributors\n\n"
        "Permission is hereby granted, free of charge, to any person obtaining a copy "
        "of this software and associated documentation files (the \"Software\"), to deal "
        "in the Software without restriction, including without limitation the rights "
        "to use, copy, modify, merge, publish, distribute, sublicense, and/or sell "
        "copies of the Software, and to permit persons to whom the Software is "
        "furnished to do so, subject to the following conditions:\n\n"
        "The above copyright notice and this permission notice shall be included in all "
        "copies or substantial portions of the Software.\n\n"
        "THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND.\n"
    )


class TestGateBaseline(unittest.TestCase):
    def test_empty_repo_is_nogo(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            result = evaluate_opensource_gate(root)
            self.assertEqual(result["verdict"], OPEN_NO_GO)
            # Should fail on the absolute basics.
            # PathsScrubbed passes vacuously on an empty repo (nothing to leak).
            names = {c["name"] for c in result["blockers"]}
            for required in (
                "LicensePresent",
                "ContributingPresent",
                "CodeOfConductPresent",
                "SecurityPresent",
                "GithubTemplatesPresent",
                "CIWorkflowPresent",
                "GitignoreComplete",
            ):
                self.assertIn(required, names, f"missing blocker: {required}")

    def test_fully_compliant_repo_passes_blockers(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            fx = _RepoFixture(root)
            fx.write("LICENSE", _good_license())
            fx.write("CONTRIBUTING.md", "# Contributing\n\nStart here.\n" * 20)
            fx.write("CODE_OF_CONDUCT.md", "# CoC\n\nBe kind.\n" * 20)
            fx.write(
                "SECURITY.md",
                "# Security\n\nSubscription proxies may violate provider ToS. "
                "Users opt in.\n",
            )
            (root / ".github" / "ISSUE_TEMPLATE").mkdir(parents=True, exist_ok=True)
            fx.write(".github/ISSUE_TEMPLATE/bug.md", "# Bug\n")
            fx.write(".github/PULL_REQUEST_TEMPLATE.md", "# PR\n")
            (root / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            fx.write(".github/workflows/ci.yml", "name: ci\non: push\n")
            # Required .gitignore patterns
            gi_extra = textwrap.dedent(
                """\

                .schemathesis/
                .hypothesis/
                _diag/
                *.log
                devkit/MEMORY.md
                devkit/RUNS.md
                devkit/runs/
                devkit/logs/
                """
            )
            (root / ".gitignore").write_text(
                (root / ".gitignore").read_text(encoding="utf-8") + gi_extra,
                encoding="utf-8",
            )
            result = evaluate_opensource_gate(root)
            self.assertNotEqual(
                result["verdict"],
                OPEN_NO_GO,
                msg=f"unexpected blockers: {result['blockers']}",
            )


class TestLicensePresent(unittest.TestCase):
    def test_missing(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            r = evaluate_opensource_gate(Path(td))
            failed = next(c for c in r["blockers"] if c["name"] == "LicensePresent")
            self.assertIn("No LICENSE", failed["detail"])

    def test_short_placeholder_blocked(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            fx = _RepoFixture(Path(td))
            fx.write("LICENSE", "TODO\n")
            r = evaluate_opensource_gate(Path(td))
            failed = next(c for c in r["blockers"] if c["name"] == "LicensePresent")
            self.assertIn("suspiciously short", failed["detail"])

    def test_full_mit_passes(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            fx = _RepoFixture(Path(td))
            fx.write("LICENSE", _good_license())
            r = evaluate_opensource_gate(Path(td))
            self.assertNotIn("LicensePresent", [c["name"] for c in r["blockers"]])


class TestGitignoreComplete(unittest.TestCase):
    def test_missing_patterns_listed(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            Path(td, ".gitignore").write_text("*.pyc\n", encoding="utf-8")
            r = evaluate_opensource_gate(Path(td))
            failed = next(c for c in r["blockers"] if c["name"] == "GitignoreComplete")
            for needle in (".schemathesis/", "_diag/", "*.log", "devkit/MEMORY.md"):
                self.assertIn(needle, failed["detail"])


class TestPathsScrubbed(unittest.TestCase):
    def test_personal_path_blocked(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            fx = _RepoFixture(Path(td))
            fx.write("README.md", "Run me at /Users/lune/Code/repo/foo.py\n")
            r = evaluate_opensource_gate(Path(td))
            failed = next(c for c in r["blockers"] if c["name"] == "PathsScrubbed")
            self.assertIn("README.md", failed["detail"])
            self.assertIn("/Users/lune", failed["detail"])

    def test_relative_paths_pass(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            fx = _RepoFixture(Path(td))
            fx.write("README.md", "Run me at ./scripts/foo.py or $REPO_ROOT/devkit\n")
            # Add a full LICENSE / template so only PathsScrubbed matters here
            fx.write("LICENSE", _good_license())
            fx.write("CONTRIBUTING.md", "# Contributing\n" * 20)
            fx.write("CODE_OF_CONDUCT.md", "# CoC\n" * 20)
            fx.write(
                "SECURITY.md",
                "# Security\n\nSubscription proxies: ToS risk; users opt in.\n",
            )
            (Path(td) / ".github" / "ISSUE_TEMPLATE").mkdir(parents=True, exist_ok=True)
            fx.write(".github/ISSUE_TEMPLATE/bug.md", "# Bug\n")
            fx.write(".github/PULL_REQUEST_TEMPLATE.md", "# PR\n")
            (Path(td) / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            fx.write(".github/workflows/ci.yml", "name: ci\n")
            # gitignore patterns
            gi_extra = "\n.schemathesis/\n.hypothesis/\n_diag/\n*.log\ndevkit/MEMORY.md\ndevkit/RUNS.md\ndevkit/runs/\ndevkit/logs/\n"
            (Path(td) / ".gitignore").write_text(
                (Path(td) / ".gitignore").read_text(encoding="utf-8") + gi_extra,
                encoding="utf-8",
            )
            r = evaluate_opensource_gate(Path(td))
            self.assertNotIn("PathsScrubbed", [c["name"] for c in r["blockers"]])


class TestMockMode(unittest.TestCase):
    def test_missing_mock_warns_not_blocks(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            r = evaluate_opensource_gate(Path(td))
            self.assertNotIn("MockModeRunnable", [c["name"] for c in r["blockers"]])
            self.assertIn("MockModeRunnable", [c["name"] for c in r["warnings"]])

    def test_existing_mock_warns_passes(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            fx = _RepoFixture(Path(td))
            fx.write(
                "litellm/config.mock.yaml",
                "model_list:\n  - model_name: mock\n    litellm_params:\n      model: openai/mock\n      mock_response: ok\n",
            )
            r = evaluate_opensource_gate(Path(td))
            self.assertNotIn("MockModeRunnable", [c["name"] for c in r["warnings"]])
            self.assertNotIn("MockModeRunnable", [c["name"] for c in r["blockers"]])


class TestWorktreeClean(unittest.TestCase):
    def test_non_git_repo_does_not_warn(self):
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            r = evaluate_opensource_gate(Path(td))
            wt = next(c for c in r["checks"] if c["name"] == "WorktreeClean")
            self.assertTrue(wt["passed"])


class TestDefaultChecks(unittest.TestCase):
    def test_every_check_has_unique_name(self):
        checks = default_checks()
        names = [c.name for c in checks]
        self.assertEqual(len(names), len(set(names)))

    def test_blockers_are_subset_for_go_path(self):
        checks = default_checks()
        blockers = [c for c in checks if c.severity == BLOCKER]
        self.assertGreaterEqual(len(blockers), 6)

    def test_custom_check_can_be_injected(self):
        # The evaluator accepts an iterable of checks; ensure injection works.
        sentinel = OpenSourceCheck(
            name="SentinelCheck",
            severity=WARNING,
            run=lambda _root: __import__(
                "devkit.gate.opensource", fromlist=["CheckResult"]
            ).CheckResult(
                name="SentinelCheck",
                severity=WARNING,
                passed=False,
                detail="forced",
            ),
        )
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            r = evaluate_opensource_gate(Path(td), checks=[sentinel])
            self.assertEqual(r["verdict"], OPEN_GO_WITH_WARNINGS)
            self.assertTrue(
                any(c["name"] == "SentinelCheck" for c in r["warnings"])
            )


if __name__ == "__main__":
    unittest.main()