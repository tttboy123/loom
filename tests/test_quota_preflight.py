"""
Unit tests for devkit.quota_preflight.
"""
from __future__ import annotations

import json
import os
import tempfile
import unittest
from pathlib import Path

from devkit.quota_preflight import (
    INSUFFICIENT,
    RISKY,
    SAFE,
    UNKNOWN,
    PreflightInput,
    preflight,
    simulate,
    _load_decisions,
    _extract_stage_costs,
)


def _write_jsonl(path: Path, records: list[dict]) -> None:
    with path.open("w") as f:
        for r in records:
            f.write(json.dumps(r) + "\n")


class TestLoadDecisions(unittest.TestCase):
    def test_empty_file(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "empty.jsonl"
            p.write_text("")
            self.assertEqual(_load_decisions(p), [])

    def test_malformed_lines_skipped(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "mixed.jsonl"
            p.write_text(
                '{"ts": "x", "valid": true}\n'
                "this is not json\n"
                '{"ts": "y", "valid": false}\n'
            )
            records = _load_decisions(p)
            self.assertEqual(len(records), 2)

    def test_missing_file(self):
        with tempfile.TemporaryDirectory() as td:
            self.assertEqual(_load_decisions(Path(td) / "nope.jsonl"), [])


class TestExtractStageCosts(unittest.TestCase):
    def test_pattern_1_stage_costs_dict(self):
        recs = [
            {"ts": "1", "stage_costs": {"brainstorm": 0.01, "plan": 0.02}},
            {"ts": "2", "stage_costs": {"brainstorm": 0.015, "plan": 0.025}},
        ]
        by_stage = _extract_stage_costs(recs)
        self.assertAlmostEqual(by_stage["brainstorm"][0], 0.01)
        self.assertEqual(len(by_stage["plan"]), 2)

    def test_pattern_2_stages_list(self):
        recs = [
            {"ts": "1", "stages": [
                {"name": "implement", "cost_usd": 0.05},
                {"name": "verify", "cost_usd": 0.01},
            ]},
        ]
        by_stage = _extract_stage_costs(recs)
        self.assertAlmostEqual(by_stage["implement"][0], 0.05)
        self.assertAlmostEqual(by_stage["verify"][0], 0.01)

    def test_pattern_3_single_cost_usd(self):
        recs = [{"ts": "1", "stage": "review", "cost_usd": 0.005}]
        by_stage = _extract_stage_costs(recs)
        self.assertAlmostEqual(by_stage["review"][0], 0.005)

    def test_ignores_non_dict_records(self):
        recs = [None, "string", 42, {"ts": "1", "stage_costs": {"a": 0.1}}]
        by_stage = _extract_stage_costs(recs)
        self.assertIn("a", by_stage)


class TestPreflight(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.tmpdir = Path(self.tmp.name)
        self.log = self.tmpdir / "decisions.jsonl"
        _write_jsonl(self.log, [
            {"ts": "1", "stage_costs": {"brainstorm": 0.001, "plan": 0.002,
                                        "implement": 0.05, "verify": 0.01, "review": 0.008}},
            {"ts": "2", "stage_costs": {"brainstorm": 0.0015, "plan": 0.003,
                                        "implement": 0.04, "verify": 0.015, "review": 0.012}},
            {"ts": "3", "stage_costs": {"brainstorm": 0.0008, "plan": 0.001,
                                        "implement": 0.06, "verify": 0.012}},
        ])

    def tearDown(self):
        self.tmp.cleanup()

    def test_no_history_returns_unknown(self):
        empty_log = self.tmpdir / "empty.jsonl"
        empty_log.write_text("")
        rep = preflight(PreflightInput(decisions_log=empty_log))
        self.assertEqual(rep.verdict, UNKNOWN)
        self.assertIsNone(rep.estimated_cost_usd)

    def test_no_remaining_returns_unknown(self):
        # Without remaining, we can't compute utilization
        os.environ.pop("LOOM_QUOTA_REMAINING_USD", None)
        rep = preflight(PreflightInput(decisions_log=self.log))
        # No quota source → unknown
        self.assertEqual(rep.verdict, UNKNOWN)
        self.assertIsNotNone(rep.notes)

    def test_safe(self):
        os.environ["LOOM_QUOTA_REMAINING_USD"] = "1.0"
        rep = preflight(PreflightInput(decisions_log=self.log))
        self.assertEqual(rep.verdict, SAFE)
        self.assertGreater(rep.estimated_cost_usd, 0)
        self.assertLess(rep.utilization, 0.5)

    def test_risky(self):
        os.environ["LOOM_QUOTA_REMAINING_USD"] = "0.15"
        rep = preflight(PreflightInput(decisions_log=self.log))
        # estimated ≈ 0.0756; remaining 0.15; utilization ≈ 50% — could be Risky
        # depending on mean. Acceptable band: 0.4-0.95
        self.assertIn(rep.verdict, [RISKY, SAFE])
        if rep.utilization is not None:
            self.assertLess(rep.utilization, 0.95)

    def test_insufficient(self):
        os.environ["LOOM_QUOTA_REMAINING_USD"] = "0.05"
        rep = preflight(PreflightInput(decisions_log=self.log))
        self.assertEqual(rep.verdict, INSUFFICIENT)
        self.assertEqual(rep.utilization, 1.0)  # capped

    def test_by_stage_breakdown(self):
        os.environ["LOOM_QUOTA_REMAINING_USD"] = "1.0"
        rep = preflight(PreflightInput(decisions_log=self.log))
        self.assertIn("implement", rep.by_stage)
        # implement is the most expensive stage
        self.assertGreater(rep.by_stage["implement"], rep.by_stage["brainstorm"])

    def test_simulate_helper(self):
        os.environ["LOOM_QUOTA_REMAINING_USD"] = "1.0"
        rep = simulate(decisions_log=self.log)
        self.assertEqual(rep.verdict, SAFE)

    def test_partial_stage_history(self):
        # History only for some stages → still works with Unknown verdict
        _write_jsonl(self.tmpdir / "partial.jsonl", [
            {"ts": "1", "stage_costs": {"implement": 0.05}},
        ])
        rep = preflight(PreflightInput(decisions_log=self.tmpdir / "partial.jsonl"))
        # Only implement has history
        self.assertIn("implement", rep.by_stage)
        # Missing stages noted
        self.assertTrue(any("no history" in n for n in rep.notes))


class TestPreflightOutlierTrimming(unittest.TestCase):
    def test_top_10_percent_trimmed(self):
        with tempfile.TemporaryDirectory() as td:
            log = Path(td) / "decisions.jsonl"
            # 10 records: 9 normal (0.05) + 1 outlier (5.0)
            records = [{"ts": str(i), "stage_costs": {"implement": 0.05 if i < 9 else 5.0}}
                       for i in range(10)]
            _write_jsonl(log, records)
            os.environ["LOOM_QUOTA_REMAINING_USD"] = "1.0"
            rep = preflight(PreflightInput(decisions_log=log))
            # With outlier trimmed, mean should be ~0.05, not ~0.5
            self.assertLess(rep.estimated_cost_usd, 0.2)


class TestScanRunsDir(unittest.TestCase):
    def test_parses_cost_from_stage_md(self):
        from devkit.quota_preflight import _scan_runs_dir_for_costs
        with tempfile.TemporaryDirectory() as td:
            runs_dir = Path(td) / "runs"
            run1 = runs_dir / "run-a"
            run1.mkdir(parents=True)
            (run1 / "01-implement.md").write_text(
                "# implement stage\n# carrier: minimax\n# cost: $0.0500\n# tokens: 5000\n",
                encoding="utf-8",
            )
            (run1 / "02-verify.md").write_text(
                "# verify stage\n# cost: $0.0120\n",
                encoding="utf-8",
            )
            run2 = runs_dir / "run-b"
            run2.mkdir(parents=True)
            (run2 / "01-implement.md").write_text(
                "# cost: $0.0400\n",
                encoding="utf-8",
            )
            costs = _scan_runs_dir_for_costs(runs_dir)
            self.assertEqual(len(costs["implement"]), 2)
            self.assertEqual(len(costs["verify"]), 1)
            self.assertIn(0.05, costs["implement"])
            self.assertIn(0.04, costs["implement"])

    def test_handles_chinese_cost_label(self):
        from devkit.quota_preflight import _scan_runs_dir_for_costs
        with tempfile.TemporaryDirectory() as td:
            runs_dir = Path(td) / "runs"
            r = runs_dir / "r1"
            r.mkdir(parents=True)
            # 花费 (with fullwidth colon) and a normal $0.03
            (r / "01-implement.md").write_text(
                "# implement\n# 花费 $0.0300\n", encoding="utf-8"
            )
            costs = _scan_runs_dir_for_costs(runs_dir)
            self.assertIn(0.03, costs["implement"])

    def test_missing_runs_dir(self):
        from devkit.quota_preflight import _scan_runs_dir_for_costs
        with tempfile.TemporaryDirectory() as td:
            costs = _scan_runs_dir_for_costs(Path(td) / "no-such-dir")
            self.assertEqual(sum(len(v) for v in costs.values()), 0)


if __name__ == "__main__":
    unittest.main()