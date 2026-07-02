from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parent / "fix_backlog_pending.py"


def _run_in_tmp(
    backlog: dict | list,
    decision_rows: list[dict] | None = None,
    *,
    dry_run: bool = False,
    raw_decision_log: str | None = None,
) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory() as tmp_dir:
        root = Path(tmp_dir)
        (root / "backlog.json").write_text(json.dumps(backlog), encoding="utf-8")
        if raw_decision_log is not None:
            (root / "decision_log.jsonl").write_text(raw_decision_log, encoding="utf-8")
        elif decision_rows is not None:
            with (root / "decision_log.jsonl").open("w", encoding="utf-8") as fh:
                for row in decision_rows:
                    fh.write(json.dumps(row) + "\n")
        args = [sys.executable, str(SCRIPT)]
        if dry_run:
            args.append("--dry-run")
        return subprocess.run(args, cwd=root, capture_output=True, text=True)


class FixBacklogPendingTest(unittest.TestCase):
    def test_no_pending_entries_synced_zero(self):
        proc = _run_in_tmp(
            backlog={"tasks": [{"id": "t-1", "status": "done"}]},
            decision_rows=[{"task_id": "t-1", "status": "done"}],
            dry_run=True,
        )
        self.assertEqual(proc.returncode, 0)
        self.assertIn("synced=0", proc.stdout)

    def test_pending_task_id_not_in_backlog(self):
        proc = _run_in_tmp(
            backlog={"tasks": [{"id": "t-1", "status": "done"}]},
            decision_rows=[{"task_id": "ghost", "status": "pending"}],
        )
        self.assertEqual(proc.returncode, 0)
        self.assertIn("synced=0", proc.stdout)

    def test_pending_synced(self):
        proc = _run_in_tmp(
            backlog={"tasks": [{"id": "t-1", "status": "failed"}, {"id": "t-2", "status": "done"}]},
            decision_rows=[{"task_id": "t-1", "status": "pending"}, {"task_id": "t-2", "status": "pending"}],
        )
        self.assertEqual(proc.returncode, 0)
        self.assertIn("synced=2", proc.stdout)

    def test_io_error_returns_nonzero(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            (root / "backlog.json").write_text(json.dumps({"tasks": []}), encoding="utf-8")
            blocker = root / "decision_log.jsonl"
            blocker.mkdir()
            proc = subprocess.run([sys.executable, str(SCRIPT)], cwd=root, capture_output=True, text=True)
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn("error:", proc.stderr)

    def test_invalid_json_returns_nonzero(self):
        proc = _run_in_tmp(
            backlog={"tasks": []},
            raw_decision_log="{not valid json\n",
        )
        self.assertNotEqual(proc.returncode, 0)
        self.assertIn("error:", proc.stderr)

    def test_empty_decision_log_returns_zero(self):
        proc = _run_in_tmp(
            backlog={"tasks": [{"id": "t-1", "status": "done"}]},
            raw_decision_log="",
        )
        self.assertEqual(proc.returncode, 0)
        self.assertIn("synced=0", proc.stdout)


if __name__ == "__main__":
    unittest.main()
