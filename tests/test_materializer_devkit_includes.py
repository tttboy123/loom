"""
Focused unittest for devkit sandbox materializer's include rule.

Contract under test:
  - is_included_by_globs(path, include_globs=("devkit/*.py",))
    MUST match every top-level devkit/*.py source file
    (not just devkit/test_*.py), including:
        preflight.py, ponytail.py, wallet.py, retry.py
  - MUST NOT match files outside the devkit/ prefix
  - MUST NOT recursively match devkit/sub/* (unless glob allows it)
"""
import os
import tempfile
import unittest
from pathlib import Path

from harness.sandbox.materializer import is_included_by_globs

class TestIsIncludedByGlobs(unittest.TestCase):
    def setUp(self):
        # Realistic on-disk tree so we can also sanity-check glob semantics,
        # though the unit under test is the pure matcher.
        self.tmp = tempfile.mkdtemp(prefix="devkit_mat_")
        self.devkit = Path(self.tmp) / "devkit"
        self.devkit.mkdir()
        for name in ("preflight.py", "ponytail.py", "wallet.py", "retry.py",
                     "test_preflight.py", "test_ponytail.py"):
            (self.devkit / name).write_text("# stub\n")
        (self.devkit / "__init__.py").write_text("")
        (self.devkit / "sub").mkdir()
        (self.devkit / "sub" / "nested.py").write_text("# stub\n")
        # A non-devkit sibling to prove the prefix matters.
        (Path(self.tmp) / "other.py").write_text("# stub\n")

        self.include = ("devkit/*.py",)

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmp, ignore_errors=True)

    # ---- flagship: all top-level devkit sources must be included ----
    def test_includes_all_top_level_devkit_py_sources(self):
        """Flagship: materializer must NOT limit itself to devkit/test_*.py."""
        expected = {
            "devkit/preflight.py",
            "devkit/ponytail.py",
            "devkit/wallet.py",
            "devkit/retry.py",
            "devkit/__init__.py",  # any *.py at top level
        }
        for p in expected:
            with self.subTest(path=p):
                self.assertTrue(
                    is_included_by_globs(p, self.include),
                    f"{p} should be matched by {self.include!r}",
                )

    # ---- non-happy-path: previously broken case must now pass ----
    def test_includes_preflight_ponytail_wallet_retry_explicitly(self):
        """Regression for the reported bug: these four were missing."""
        for p in ("devkit/preflight.py", "devkit/ponytail.py",
                  "devkit/wallet.py", "devkit/retry.py"):
            self.assertTrue(is_included_by_globs(p, self.include), p)

    def test_excludes_files_outside_devkit_prefix(self):
        self.assertFalse(is_included_by_globs("other.py", self.include))
        self.assertFalse(is_included_by_globs("src/devkit/preflight.py",
                                              self.include))

    def test_default_include_glob_is_devkit_star_py(self):
        """Default-arg contract: callers who omit include_globs still get
        devkit/*.py semantics, not devkit/test_*.py."""
        for p in ("devkit/preflight.py", "devkit/retry.py"):
            self.assertTrue(is_included_by_globs(p), p)
        self.assertTrue(is_included_by_globs("devkit/__init__.py"))

class TestMaterializerAcceptanceLayout(unittest.TestCase):
    """
    Acceptance #1 (structural): build/<run_id>/devkit/ materialized with
    at least preflight.py, ponytail.py, wallet.py, retry.py present.

    This is enforced by glob rule + a filesystem assertion on the
    materializer's output for a synthetic source tree.
    """

    def test_layout_after_materialize_lists_required_sources(self):
        from harness.sandbox.materializer import materialize

        with tempfile.TemporaryDirectory() as src_root, \
             tempfile.TemporaryDirectory() as build_root:
            src_devkit = Path(src_root) / "devkit"
            src_devkit.mkdir()
            for name in ("preflight.py", "ponytail.py", "wallet.py",
                         "retry.py", "__init__.py"):
                (src_devkit / name).write_text("# stub\n")

            run_id = "auto-test"
            materialize(
                src_root=src_root,
                build_root=build_root,
                run_id=run_id,
                include_globs=("devkit/*.py",),
            )

            materialized_dir = Path(build_root) / run_id / "devkit"
            py_files = sorted(p.name for p in materialized_dir.glob("*.py"))
            for required in ("preflight.py", "ponytail.py",
                             "wallet.py", "retry.py"):
                self.assertIn(required, py_files,
                              f"missing {required} in {py_files}")

if __name__ == "__main__":
    unittest.main()
