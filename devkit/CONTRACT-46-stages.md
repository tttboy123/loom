# Sprint Contract #46 — Usage by-stage cost pivot (`devkit stages`)

**Why:** Of the 4 user-approved token-opt items, "usage by-stage" is the last unbuilt one.
Loom already aggregates run-log rows **by backend** (`run_stats_by_backend` → `score_report`).
This adds the orthogonal pivot: **by pipeline stage** (brainstorm/plan/implement/verify/review/
iterate-impl/…), so the user can see *where* tokens & cost actually go and target compaction /
cheaper carriers at the heaviest stage. Pure read-side aggregation over existing on-disk logs —
no new model calls, no schema migration.

**Data source (existing, unchanged):** `devkit/runs/<ts>/run-log.md` markdown tables, row format
(the SOLE writer is `rdloop.py:489` `log_rows.append`):
`| {st.key} | {carrier} | {served|-} | {OK|BLOCKED} | {dt:.1f}s | {tokens} | ${cost:.5f} |`
The first column = `st.key`. `_ROW` (insight.py:248) currently **drops** it (`[^|]+?` non-captured).
Every real `run-log.md` also has a **header row** `| 阶段 | 载体 | … |` and a **separator row**
`| --- | --- | … |` that must NOT be aggregated.
**Stage keys are the real `st.key` values of the active roles pipeline** (default
brainstorm/plan/implement/verify/review). NOTE (verified): iterate/cascade re-runs write to
separate `90-implement-r{N}.md` / `91-review-r{N}.md` files via `_exec_stage`, **not** to the
`run-log.md` `log_rows` table — so no `-r{N}`/pseudo-stage keys appear in the table today. No fixed
enum is assumed: any custom roles-config key is aggregated as-is.

## Acceptance criteria (machine-checkable)

1. **`insight.run_stats_by_stage(runs_dir=DEFAULT) -> dict`** — parses every `*/run-log.md` row and
   aggregates by the **first column (stage key)**, returning
   `{stage_key: {"stage": k, "uses": int, "ok": int, "lat": float, "tokens": int, "cost": float}}`.
   Mirrors `run_stats_by_backend` accumulation semantics exactly (uses +1, ok +1 iff status==OK,
   lat/tokens/cost summed).
2. **Stage-key capture — pinned regex-group invariant.** Implement EITHER:
   (a) add a sibling `_ROW_STAGE` regex capturing **7** groups
   `(stage, carrier, served, status, dt, toks, cost)` and leave `_ROW` **byte-for-byte unchanged**;
   OR (b) widen `_ROW` to capture group-1 AND update `run_stats_by_backend`'s `m.groups()`
   unpacking in the same change. Any change that alters the number/order of groups
   `run_stats_by_backend` reads **without** updating that callsite is forbidden. The stage column
   uses `\s*([^|]+?)\s*` (non-greedy, trimmed) — matching house trimming style and ensuring the
   header (`| 阶段 |`) and separator (`| --- |`) rows still fail to match (they lack `OK|BLOCKED`
   and a `[\d.]+s` cell). Existing `run_stats_by_backend` + `ScoreReportTest` stay green.
3. **Robustness (identical to `run_stats_by_backend`):** missing `runs_dir` → `{}`; a line that
   doesn't match the row regex is skipped (this includes the markdown **header row** and
   **separator row** present in every real log — they must be excluded from aggregation); an
   unreadable file is skipped (try/except continue). No exception escapes on malformed input.
   **BLOCKED rows count toward `uses` and their `tokens`/`cost` sums (uses +1, ok +0)** — parity
   with insight.py:272-276 — so a stage with only BLOCKED rows still appears with `ok_rate` 0%.
4. **`insight.stage_report(runs_dir=DEFAULT) -> dict`** returns
   `{"rows": [ {stage, uses, ok_rate|None, avg_lat|None, avg_tokens|None, avg_cost|None,
   total_tokens, total_cost, pct_cost} ... ], "totals": {"tokens": int, "cost": float, "uses": int}}`.
   `pct_cost` = round(total_cost_of_stage / grand_total_cost * 100, 1), or 0.0 when grand total is 0
   (no divide-by-zero). Rows sorted by `total_cost` desc. Stages preserve their real `st.key`
   values (not a fixed enum). `avg_*` mirror `score_report` shape (insight.py:370-372) — never None
   in practice since a stage only appears with ≥1 use; `stage_report` over a missing OR empty dir
   both yield `{"rows": [], "totals": {"tokens": 0, "cost": 0.0, "uses": 0}}`.
5. **CLI + dispatch (3 concrete file edits, all required):**
   (a) `__main__.py` — `if argv[0] == "stages":` branch in `main()` that calls a `cmd_stages` printing
       a readable table (stage, uses, ok%, avg tok, avg $, total $, % of cost) + a TOTAL line; empty
       data prints a friendly "还没有跑过 / no runs yet" line and exits 0. Plus an `epilog` help line
       near __main__.py:73.
   (b) `loom` (bash) — a new `stages) python3 -m devkit stages "$@" ;;` case arm (no generic
       passthrough exists) AND a help-text line in the `*)` catch-all heredoc.
   Verifiable: `./loom stages --runs-dir <tmp>` exits 0 on both empty and populated dirs.
6. **Tests (in `devkit/test_features.py`):** a `StageReportTest` (≥4 assertions) using a temp dir
   with a synthetic `run-log.md` that includes the **header row, separator row**, ≥2 stages, an OK
   row with **non-zero cost**, an all-BLOCKED stage ($0.00000), and a malformed line — asserting:
   header/separator/malformed rows are NOT counted; correct per-stage uses/ok/tokens/cost; the
   all-BLOCKED stage appears with ok_rate 0%; `pct_cost` sums to ~100 (±0.2) **(fixture has a
   non-zero-cost row so this branch executes)**; missing-dir AND empty-dir both →
   `{"rows": [], "totals": {…0}}` with no exception; rows sorted by total_cost desc. Full suite
   (`./loom test`) stays green; **test-method count goes 66 → ≥68** (net ≥+2 from `StageReportTest`).

## Explicitly OUT of scope (anti-gold-plating)
- No console panel/endpoint in this contract (CLI only; console can follow as a separate item).
- No new persisted format — only parse what `run-log.md` already contains.
- No change to what `rdloop.py` writes.

## Definition of done
All 6 criteria met; the three dispatch edits (criterion 5a/5b) in place; `./loom stages` shows a
real pivot over the existing `devkit/runs/`; full test suite green (≥68 cases); independent
code-review subagent ACCEPTS against this contract.

---
**Contract negotiated** with independent code-review subagent (round 1): REQUEST-CHANGES → all 5
REQUIRED changes applied (corrected test baseline 66→≥68; pinned 7-group `_ROW_STAGE` invariant;
removed false iterate-pseudo-stage claim; explicit header/separator skip-test; concrete 3-file
dispatch deliverable). Contract tightened, not weakened.
