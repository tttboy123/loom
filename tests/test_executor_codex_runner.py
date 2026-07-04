"""Tests for devkit/executors.py — especially the new ``codex-runner``
(DESIGN-P0 P0-b: real pytest/ruff execution in sandbox, distinct from
the chat-only ``codex`` executor which talks to codex-sub via gateway)."""
from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest import mock

from devkit import executors


# ----------------------------------------------------------------------------
# _parse_pytest_output
# ----------------------------------------------------------------------------
class TestParsePytestOutput(unittest.TestCase):
    def test_all_pass(self):
        out = "test_a.py::test_x PASSED\ntest_b.py::test_y PASSED\n\n2 passed in 0.10s"
        r = executors._parse_pytest_output(out)
        self.assertEqual(r["verdict"], "GO")
        self.assertTrue(r["tests_passed"])
        self.assertEqual(r["tests_failed"], 0)
        self.assertEqual(r["tests_collected"], 2)

    def test_some_fail(self):
        out = (
            "test_a.py::test_x FAILED\n"
            "test_b.py::test_y PASSED\n\n"
            "FAILED test_a.py::test_x - assert 0 == 1\n"
            "1 failed, 1 passed in 0.20s"
        )
        r = executors._parse_pytest_output(out)
        self.assertEqual(r["verdict"], "NO-GO")
        self.assertFalse(r["tests_passed"])
        self.assertEqual(r["tests_failed"], 1)
        self.assertEqual(r["tests_collected"], 2)
        self.assertEqual(len(r["failing"]), 1)
        self.assertEqual(r["failing"][0]["name"], "test_a.py::test_x")

    def test_error_counted_as_failure(self):
        out = "1 error in 0.50s"
        r = executors._parse_pytest_output(out)
        self.assertEqual(r["verdict"], "NO-GO")
        self.assertEqual(r["tests_failed"], 1)

    def test_unparseable_returns_unknown(self):
        out = "garbage output that doesn't look like pytest"
        r = executors._parse_pytest_output(out)
        self.assertEqual(r["verdict"], "UNKNOWN")
        self.assertIsNone(r["tests_passed"])


# ----------------------------------------------------------------------------
# run_codex_runner — happy path (mocked, since pytest may not be on global PATH)
# ----------------------------------------------------------------------------
class TestRunCodexRunnerHappy(unittest.TestCase):
    def _write_passing_pytest(self, sandbox: pathlib.Path) -> None:
        (sandbox / "tests").mkdir()
        (sandbox / "tests" / "test_ok.py").write_text(
            textwrap.dedent("""
                def test_one():
                    assert 1 + 1 == 2

                def test_two():
                    assert "ab" + "cd" == "abcd"
            """).strip() + "\n",
            encoding="utf-8",
        )

    def _write_failing_pytest(self, sandbox: pathlib.Path) -> None:
        (sandbox / "tests").mkdir()
        (sandbox / "tests" / "test_bad.py").write_text(
            textwrap.dedent("""
                def test_one():
                    assert 1 + 1 == 3
            """).strip() + "\n",
            encoding="utf-8",
        )

    def test_passing_suite_returns_ok(self):
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)
            self._write_passing_pytest(sandbox)
            with mock.patch("shutil.which", return_value="/usr/bin/pytest"), \
                 mock.patch("shutil.which") as m_which:
                # pytest yes, codex no, ruff no
                m_which.side_effect = lambda n: "/usr/bin/pytest" if n == "pytest" else None
                with mock.patch.object(executors, "_run") as m_run:
                    m_run.return_value = (0, "2 passed in 0.10s")
                    ok, out, name = executors.run_codex_runner(sandbox, timeout=30)
            self.assertEqual(name, "codex-runner")
            self.assertTrue(ok)
            self.assertIn("passed", out.lower())

    def test_failing_suite_still_returns_ok_with_output(self):
        # The executor's "ok" means "we successfully ran tests" — the report
        # says whether they passed.
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)
            self._write_failing_pytest(sandbox)
            with mock.patch("shutil.which") as m_which:
                m_which.side_effect = lambda n: "/usr/bin/pytest" if n == "pytest" else None
                with mock.patch.object(executors, "_run") as m_run:
                    m_run.return_value = (1, "1 failed in 0.10s")
                    ok, out, name = executors.run_codex_runner(sandbox, timeout=30)
            self.assertEqual(name, "codex-runner")
            self.assertTrue(ok)
            self.assertIn("failed", out.lower())


# ----------------------------------------------------------------------------
# run_codex_runner — graceful degradation
# ----------------------------------------------------------------------------
class TestRunCodexRunnerDegraded(unittest.TestCase):
    def test_no_pytest_no_codex(self):
        """If neither `pytest` nor `codex` is on PATH, return clear error."""
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)
            with mock.patch("shutil.which", return_value=None):
                ok, out, name = executors.run_codex_runner(sandbox, timeout=10)
        self.assertEqual(name, "codex-runner")
        self.assertFalse(ok)
        self.assertIn("pytest", out.lower())
        self.assertIn("codex", out.lower())

    def test_codex_cli_attempted_first(self):
        """If `codex` CLI exists, it's tried first (we don't assert outcome
        because the CLI may exit non-zero on a vanilla repo)."""
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)
            (sandbox / "tests").mkdir()
            (sandbox / "tests" / "test_ok.py").write_text(
                "def test_x():\n    assert True\n", encoding="utf-8"
            )

            calls = []

            def fake_which(name: str):
                return "/usr/local/bin/codex" if name == "codex" else None

            def fake_run(cmd, cwd, env, timeout):
                calls.append(cmd[0])
                return (0, "[codex] ok")  # pretend codex succeeded

            with mock.patch("shutil.which", side_effect=fake_which), \
                 mock.patch.object(executors, "_run", side_effect=fake_run):
                ok, out, name = executors.run_codex_runner(sandbox, timeout=10)

            self.assertEqual(name, "codex-runner")
            self.assertTrue(ok)
            self.assertEqual(calls, ["/usr/local/bin/codex"])

    def test_codex_cli_failing_falls_through_to_pytest(self):
        """If `codex` CLI exists but returns non-zero, fall through to pytest."""
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)
            (sandbox / "tests").mkdir()
            (sandbox / "tests" / "test_ok.py").write_text(
                "def test_x():\n    assert True\n", encoding="utf-8"
            )

            calls = []

            def fake_which(name: str):
                if name == "codex":
                    return "/usr/local/bin/codex"
                if name == "pytest":
                    return "/usr/bin/pytest"
                return None

            def fake_run(cmd, cwd, env, timeout):
                calls.append(cmd[0])
                if "codex" in cmd[0]:
                    return (1, "codex failed")
                # pytest
                return (0, "1 passed in 0.01s")

            with mock.patch("shutil.which", side_effect=fake_which), \
                 mock.patch.object(executors, "_run", side_effect=fake_run):
                ok, out, name = executors.run_codex_runner(sandbox, timeout=10)

            self.assertTrue(ok)
            # codex first, then pytest
            self.assertIn("codex", calls[0])
            self.assertEqual(calls[1], "pytest")

    def test_codex_cli_fails_and_no_pytest(self):
        """If codex CLI exists but fails AND pytest is missing — graceful fail."""
        with tempfile.TemporaryDirectory() as td:
            sandbox = pathlib.Path(td)

            def fake_which(name: str):
                return "/usr/local/bin/codex" if name == "codex" else None

            def fake_run(cmd, cwd, env, timeout):
                return (1, "codex failed")

            with mock.patch("shutil.which", side_effect=fake_which), \
                 mock.patch.object(executors, "_run", side_effect=fake_run):
                ok, out, name = executors.run_codex_runner(sandbox, timeout=10)

            self.assertFalse(ok)
            self.assertIn("pytest", out.lower())
            self.assertNotIn("no codex", out.lower())  # message should not say "no codex"


# ----------------------------------------------------------------------------
# dispatch
# ----------------------------------------------------------------------------
class TestExecutorDispatch(unittest.TestCase):
    def test_codex_runner_dispatches(self):
        with mock.patch.object(
            executors, "run_codex_runner", return_value=(True, "ok", "codex-runner")
        ) as mock_run:
            ok, out, name = executors.run(
                "codex-runner", "prompt", "model",
                sandbox=pathlib.Path("."), gateway="http://g", api_key="k",
            )
        self.assertTrue(ok)
        mock_run.assert_called_once()

    def test_unknown_executor(self):
        ok, out, name = executors.run(
            "nope", "p", "m", sandbox=pathlib.Path("."),
            gateway="http://g", api_key="k",
        )
        self.assertFalse(ok)
        self.assertIn("未知执行器", out)
        self.assertIn("codex-runner", out)


if __name__ == "__main__":
    unittest.main()