"""
devkit 运行总台账的 contract 测试（无需 pytest，直接 `python3 devkit/test_runs_log.py`）。

源自 dogfood loop 里 cross-vendor reviewer(Codex) 的验收清单：
表头、6 列、摘要截断、carrier→model、BLOCKED 回填、用量列、相对路径、追加不覆盖、fail-open。
"""
import pathlib
import tempfile

from devkit.rdloop import ROOT, append_run_ledger


def _rows(p: pathlib.Path):
    # 仅数据行：排除表头与 markdown 分隔行(| --- | --- |)
    return [l for l in p.read_text(encoding="utf-8").splitlines()
            if l.startswith("| ") and "时间戳" not in l and "---" not in l]


def main() -> int:
    with tempfile.TemporaryDirectory() as d:
        ledger = pathlib.Path(d) / "RUNS.md"

        # 1) 首次写入建表头；摘要超长截断；OK→实际模型，非OK→BLOCKED；相对路径
        append_run_ledger(
            task="X" * 120,
            stage_meta=[("brainstorm", "loom-product", "loom-product", "OK"),
                        ("verify", "loom-tester", "-", "BLOCKED")],
            gate="建议 GO（需人类最终确认）",
            run_dir=ROOT / "devkit" / "runs" / "20260101-000000",
            tot_tokens=1234, tot_cost=0.0021,
            ledger=ledger,
        )
        txt = ledger.read_text(encoding="utf-8")
        assert txt.startswith("# devkit 运行总台账"), "缺表头"
        rows = _rows(ledger)
        assert len(rows) == 1, f"应有 1 行数据，实得 {len(rows)}"
        cols = [c.strip() for c in rows[0].split("|")[1:-1]]
        assert len(cols) == 6, f"应有 6 列，实得 {len(cols)}: {cols}"
        assert len(cols[1]) <= 60, "摘要未截断到 60"
        assert "loom-product→loom-product" in cols[2], "OK 阶段应记实际模型"
        assert "verify:loom-tester→BLOCKED" in cols[2], "非 OK 阶段应记 BLOCKED"
        assert cols[3] == "GO", "Gate 应为 GO"
        assert "tok" in cols[4] and "$" in cols[4], f"用量列应含 tokens/花费: {cols[4]}"
        assert not cols[5].startswith(("/", "\\")), f"产物目录应为相对路径: {cols[5]}"

        # 2) 再写一次 → 追加不覆盖（2 行数据）；NO-GO 正确
        append_run_ledger(
            task="short", stage_meta=[("review", "codex-sub", "codex-sub", "OK")],
            gate="NO-GO（有阶段未跑通）", run_dir=ROOT / "x", ledger=ledger,
        )
        assert len(_rows(ledger)) == 2, "第二次应为追加而非覆盖"
        assert _rows(ledger)[1].split("|")[4].strip() == "NO-GO"

        # 3) fail-open：脏输入不得抛异常
        append_run_ledger(task=None, stage_meta=None, gate=None, run_dir=ledger, ledger=ledger)
        append_run_ledger(task=123, stage_meta=[("a",)], gate="GO", run_dir=ROOT, ledger=ledger)

    print("✓ test_runs_log: ALL PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
