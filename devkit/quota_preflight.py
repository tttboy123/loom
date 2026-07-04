"""
devkit.quota_preflight
======================

Quota Preflight — DESIGN-P0 P0-a component.

Estimate whether a planned task fits the current budget/quota envelope
BEFORE running it. Pure stdlib; reads historical run data from
``devkit/logs/decisions.jsonl`` and the backlog; produces a structured
``PreflightReport``.

CLI
---
    ./loom quota simulate "<task description>"
    python3 -m devkit quota_preflight simulate --task "..." --carrier minimax
    python3 -m devkit quota_preflight simulate --no-task  # report only on history

Verdict levels
--------------
- ``Safe``         — enough budget; safe to run
- ``Risky``        — likely to run but may exhaust budget
- ``Insufficient`` — history shows cost > remaining budget
- ``Unknown``      — no history; we cannot estimate

Design rules
------------
1. **Zero dependencies** — stdlib only.
2. **Deterministic** — same inputs → same output.
3. **Best-effort** — never blocks runs; if uncertain, return ``Unknown``.
4. **Tunable** — env vars override defaults: ``LOOM_QUOTA_SOFT_RATIO``,
   ``LOOM_QUOTA_HARD_RATIO``, ``LOOM_QUOTA_REMAINING_USD``.

Heuristic
---------
For each stage (brainstorm / plan / implement / verify / review), the mean
cost is computed from decisions.jsonl records that have a non-null
``cost_usd`` for that stage. Mean cost is aggregated by task_type when
possible (so a "review" task only pays review costs, not the full pipeline).
If no history exists, returns ``Unknown`` (does not invent a number).
"""

from __future__ import annotations

import json
import os
import re
import statistics
import sys
from collections import defaultdict
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Any, Iterable

# Defaults — override with env vars
DEFAULT_SOFT_RATIO = float(os.environ.get("LOOM_QUOTA_SOFT_RATIO", "0.7"))
DEFAULT_HARD_RATIO = float(os.environ.get("LOOM_QUOTA_HARD_RATIO", "0.95"))

# Verdict constants
SAFE = "Safe"
RISKY = "Risky"
INSUFFICIENT = "Insufficient"
UNKNOWN = "Unknown"

# Default stage pipeline (if user doesn't specify)
DEFAULT_STAGES = ("brainstorm", "plan", "implement", "verify", "review")


@dataclass
class PreflightReport:
    """Structured output of a quota preflight check."""

    verdict: str
    estimated_cost_usd: float | None
    remaining_usd: float | None
    utilization: float | None            # estimated / remaining, capped at 1.0
    by_stage: dict[str, float] = field(default_factory=dict)
    sample_size: int = 0
    basis: str = "history"               # "history" | "unknown" | "override"
    notes: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        d = asdict(self)
        return d

    def human(self) -> str:
        badge = {
            SAFE: "✓ Safe",
            RISKY: "⚠ Risky",
            INSUFFICIENT: "✗ Insufficient",
            UNKNOWN: "? Unknown",
        }.get(self.verdict, self.verdict)
        lines = [
            f"Quota preflight: {badge}",
            f"  estimated:    {fmt_money(self.estimated_cost_usd)}",
            f"  remaining:    {fmt_money(self.remaining_usd)}",
            f"  utilization:  {fmt_pct(self.utilization)}",
        ]
        if self.by_stage:
            lines.append("  by stage:")
            for stage, cost in self.by_stage.items():
                lines.append(f"    {stage:<12s} {fmt_money(cost)}")
        if self.notes:
            lines.append("  notes:")
            for n in self.notes:
                lines.append(f"    - {n}")
        return "\n".join(lines)


def fmt_money(v: float | None) -> str:
    if v is None:
        return "n/a"
    return f"${v:.4f}"


def fmt_pct(v: float | None) -> str:
    if v is None:
        return "n/a"
    return f"{v*100:.1f}%"


# ----------------------------------------------------------------------------
# Decision log loader
# ----------------------------------------------------------------------------
def _resolve_decisions_log(backlog_path: Path | str | None = None) -> Path:
    """Resolve the path to decisions.jsonl. Default = devkit/decisions.jsonl
    relative to the repo root, but caller can override via env or arg.
    """
    if backlog_path:
        bp = Path(backlog_path)
        if bp.is_file():
            return bp.parent / "decisions.jsonl"
    env = os.environ.get("LOOM_DECISIONS_LOG")
    if env:
        return Path(env)
    # Default: walk up to find devkit/decisions.jsonl
    for p in [Path.cwd(), *Path.cwd().parents]:
        candidate = p / "devkit" / "decisions.jsonl"
        if candidate.exists():
            return candidate
    return Path("devkit/decisions.jsonl")


def _load_decisions(log_path: Path) -> list[dict]:
    """Parse decisions.jsonl into a list of dicts. Skip lines that don't parse."""
    if not log_path.exists():
        return []
    out: list[dict] = []
    try:
        with log_path.open(encoding="utf-8", errors="replace") as fp:
            for line in fp:
                line = line.strip()
                if not line:
                    continue
                try:
                    out.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
    except OSError:
        return []
    return out


# ----------------------------------------------------------------------------
# Secondary source: parse cost data out of run artifact files (markdown)
# This handles the case where decisions.jsonl lacks stage_costs but each
# run's stage .md file does record it (e.g. "# cost: $0.0123" in header).
# ----------------------------------------------------------------------------
_COST_RE = re.compile(
    r"^\s*#?\s*(?:cost|花费|费用)\s*[:=\s]?\s*\$?\s*(\d+(?:\.\d+)?)",
    re.IGNORECASE | re.MULTILINE,
)
_STAGE_RE = re.compile(
    r"^\[(\d+/\d+)\]\s+(\w+)\b",
    re.MULTILINE,
)


def _scan_runs_dir_for_costs(runs_dir: Path) -> dict[str, list[float]]:
    """Walk runs_dir/<run-id>/NN-<stage>.md, parse cost from header.

    Returns {stage: [cost, ...]} aggregating across all runs.
    """
    if not runs_dir.is_dir():
        return defaultdict(list)
    by_stage: dict[str, list[float]] = defaultdict(list)
    for run_md in runs_dir.glob("*/[0-9][0-9]-*.md"):
        try:
            text = run_md.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        # Try to find stage from filename first (NN-stage.md)
        # e.g. "01-implement.md" → "implement"
        m = re.match(r"\d{2}-(\w+)\.md$", run_md.name)
        stage = m.group(1) if m else None
        # If not, scan for "[N/M] stage" pattern
        if not stage:
            sm = _STAGE_RE.search(text)
            if sm:
                stage = sm.group(2)
        if not stage:
            continue
        # Find cost value
        cm = _COST_RE.search(text)
        if cm:
            try:
                by_stage[stage.lower()].append(float(cm.group(1)))
            except ValueError:
                pass
    return by_stage


# ----------------------------------------------------------------------------
# Quota source — pull remaining budget
# ----------------------------------------------------------------------------
def _read_quota_report() -> dict | None:
    """Try to find a quota report. Returns a dict like {provider: remaining_usd}
    or None if no source found.
    """
    # Env override
    env_remaining = os.environ.get("LOOM_QUOTA_REMAINING_USD")
    if env_remaining:
        try:
            return {"__total__": float(env_remaining)}
        except ValueError:
            pass
    # Try known files
    candidates = [
        Path("devkit/quota_report.json"),
        Path("devkit/logs/quota.json"),
    ]
    for p in candidates:
        if p.is_file():
            try:
                d = json.loads(p.read_text(encoding="utf-8", errors="replace"))
                if isinstance(d, dict) and "remaining_usd" in d:
                    return {"__total__": float(d["remaining_usd"])}
                if isinstance(d, dict):
                    return {k: float(v) for k, v in d.items() if isinstance(v, (int, float))}
            except (OSError, json.JSONDecodeError, ValueError):
                pass
    return None


# ----------------------------------------------------------------------------
# Stage cost extraction
# ----------------------------------------------------------------------------
def _normalize_stage(s: str) -> str:
    return (s or "").strip().lower()


def _extract_stage_costs(records: Iterable[dict]) -> dict[str, list[float]]:
    """Return {stage: [cost_usd, ...]} from decision records.

    A decision record represents a task run. Costs are tracked per-stage
    in nested fields. We try:
      - rec.get("stage_costs") -> {stage: cost}
      - rec.get("stages") -> list of {name, cost_usd}
      - rec.get("cost_usd") with rec.get("stage") as a single stage
    """
    by_stage: dict[str, list[float]] = defaultdict(list)
    for rec in records:
        if not isinstance(rec, dict):
            continue
        # Pattern 1: explicit stage_costs map
        sc = rec.get("stage_costs")
        if isinstance(sc, dict):
            for stage, cost in sc.items():
                if isinstance(cost, (int, float)):
                    by_stage[_normalize_stage(stage)].append(float(cost))
            continue
        # Pattern 2: list of stages with cost_usd
        stages = rec.get("stages")
        if isinstance(stages, list):
            for s in stages:
                if isinstance(s, dict) and s.get("name") and s.get("cost_usd") is not None:
                    by_stage[_normalize_stage(s["name"])].append(float(s["cost_usd"]))
            continue
        # Pattern 3: a single cost_usd attributed to one stage
        if "cost_usd" in rec and "stage" in rec:
            try:
                by_stage[_normalize_stage(rec["stage"])].append(float(rec["cost_usd"]))
            except (TypeError, ValueError):
                pass
    return by_stage


# ----------------------------------------------------------------------------
# Public API
# ----------------------------------------------------------------------------
@dataclass
class PreflightInput:
    task: str = ""
    stages: tuple[str, ...] = DEFAULT_STAGES
    carrier: str = ""
    decisions_log: Path | None = None
    soft_ratio: float = DEFAULT_SOFT_RATIO
    hard_ratio: float = DEFAULT_HARD_RATIO


def preflight(inp: PreflightInput) -> PreflightReport:
    """Compute a PreflightReport from the given input."""
    notes: list[str] = []
    log_path = inp.decisions_log or _resolve_decisions_log()
    records = _load_decisions(log_path)

    # Primary source: decisions.jsonl
    by_stage = _extract_stage_costs(records) if records else defaultdict(list)

    # Secondary source: scan run artifacts (markdown) for "# cost: $X" lines.
    # This bridges the gap when decisions.jsonl doesn't carry stage_costs.
    runs_dir = (log_path.parent.parent / "runs") if log_path.parent.name == "devkit" else None
    if runs_dir is None:
        # fall back: find "devkit/runs" relative to log_path
        candidate = log_path.parent
        for _ in range(4):
            candidate = candidate.parent
            if (candidate / "devkit" / "runs").is_dir():
                runs_dir = candidate / "devkit" / "runs"
                break
    if runs_dir is None and (log_path.parent / "runs").is_dir():
        runs_dir = log_path.parent / "runs"
    if runs_dir is not None:
        artifact_costs = _scan_runs_dir_for_costs(runs_dir)
        for stage, costs in artifact_costs.items():
            by_stage[stage].extend(costs)
        if artifact_costs:
            notes.append(f"scanned {sum(len(v) for v in artifact_costs.values())} cost records from {runs_dir.name}/")

    if not by_stage:
        notes.append(f"no history at {log_path} and no run artifacts")
        return PreflightReport(
            verdict=UNKNOWN,
            estimated_cost_usd=None,
            remaining_usd=None,
            utilization=None,
            sample_size=0,
            basis="unknown",
            notes=notes,
        )

    # Filter to selected stages
    target = tuple(_normalize_stage(s) for s in inp.stages)
    per_stage: dict[str, float] = {}
    for stage in target:
        samples = by_stage.get(stage) or []
        if not samples:
            notes.append(f"no history for stage '{stage}'")
            continue
        # Trim outliers (top 10%) for robustness
        if len(samples) >= 5:
            samples = sorted(samples)[: int(len(samples) * 0.9) or 1]
        per_stage[stage] = statistics.mean(samples)
    if not per_stage:
        notes.append("no per-stage cost data for selected stages")
        return PreflightReport(
            verdict=UNKNOWN,
            estimated_cost_usd=None,
            remaining_usd=None,
            utilization=None,
            sample_size=sum(len(v) for v in by_stage.values()),
            basis="unknown",
            notes=notes,
        )

    estimated = sum(per_stage.values())
    sample_size = sum(len(by_stage.get(s) or []) for s in target)

    quota = _read_quota_report()
    remaining: float | None = None
    if quota:
        # Use the smallest remaining across all providers (worst case)
        non_total = [v for k, v in quota.items() if k != "__total__"]
        if non_total:
            remaining = min(non_total) if non_total else quota.get("__total__")
        else:
            remaining = quota.get("__total__")

    utilization = None
    verdict = UNKNOWN
    if remaining is not None and remaining > 0:
        utilization = min(estimated / remaining, 1.0) if remaining else 0.0
        if estimated > remaining:
            verdict = INSUFFICIENT
        elif utilization >= inp.hard_ratio:
            verdict = INSUFFICIENT
        elif utilization >= inp.soft_ratio:
            verdict = RISKY
        else:
            verdict = SAFE
    else:
        notes.append("no quota source (set LOOM_QUOTA_REMAINING_USD or write devkit/quota_report.json)")
        verdict = UNKNOWN

    return PreflightReport(
        verdict=verdict,
        estimated_cost_usd=round(estimated, 6),
        remaining_usd=round(remaining, 6) if remaining is not None else None,
        utilization=round(utilization, 4) if utilization is not None else None,
        by_stage={k: round(v, 6) for k, v in per_stage.items()},
        sample_size=sample_size,
        basis="history",
        notes=notes,
    )


def simulate(
    task: str = "",
    stages: Iterable[str] = DEFAULT_STAGES,
    carrier: str = "",
    decisions_log: Path | str | None = None,
) -> PreflightReport:
    """Convenience wrapper around :func:`preflight`."""
    inp = PreflightInput(
        task=task,
        stages=tuple(stages) if not isinstance(stages, tuple) else stages,
        carrier=carrier,
        decisions_log=Path(decisions_log) if decisions_log else None,
    )
    return preflight(inp)


# ----------------------------------------------------------------------------
# CLI
# ----------------------------------------------------------------------------
def _print_human(report: PreflightReport) -> None:
    print(report.human())


def _print_json(report: PreflightReport) -> None:
    print(json.dumps(report.to_dict(), ensure_ascii=False, indent=2))


def _cli(argv: list[str]) -> int:
    args = list(argv)
    if not args or args[0] in ("-h", "--help"):
        print("Usage: python -m devkit.quota_preflight simulate [opts] [task]")
        print("Options:")
        print("  --stages a,b,c         Stages to estimate (default: brainstorm,plan,implement,verify,review)")
        print("  --carrier NAME         Hint for which carrier (informational only)")
        print("  --log PATH             Override decisions.jsonl path")
        print("  --json                 JSON output")
        print("  --soft-ratio N         Override soft threshold (default 0.7)")
        print("  --hard-ratio N         Override hard threshold (default 0.95)")
        print("  --remaining-usd N      Override remaining budget (default: read from file)")
        return 0

    if args[0] != "simulate":
        print(f"unknown subcommand: {args[0]}", file=sys.stderr)
        return 2

    args = args[1:]
    json_out = "--json" in args
    if json_out:
        args.remove("--json")

    # Parse simple flags
    task = ""
    stages: list[str] = list(DEFAULT_STAGES)
    carrier = ""
    log: Path | None = None
    soft = DEFAULT_SOFT_RATIO
    hard = DEFAULT_HARD_RATIO
    remaining_override: float | None = None

    i = 0
    while i < len(args):
        a = args[i]
        if a == "--stages":
            stages = args[i + 1].split(",")
            i += 2
        elif a == "--carrier":
            carrier = args[i + 1]
            i += 2
        elif a == "--log":
            log = Path(args[i + 1])
            i += 2
        elif a == "--soft-ratio":
            soft = float(args[i + 1])
            i += 2
        elif a == "--hard-ratio":
            hard = float(args[i + 1])
            i += 2
        elif a == "--remaining-usd":
            remaining_override = float(args[i + 1])
            i += 2
        elif a.startswith("-"):
            print(f"unknown flag: {a}", file=sys.stderr)
            return 2
        else:
            task = a
            i += 1

    if remaining_override is not None:
        os.environ["LOOM_QUOTA_REMAINING_USD"] = str(remaining_override)

    report = simulate(task=task, stages=stages, carrier=carrier, decisions_log=log)
    # Override ratios in the report (in case of pre-computed via simulate())
    if report.basis == "history":
        if report.utilization is not None:
            if report.utilization >= hard:
                report.verdict = INSUFFICIENT
            elif report.utilization >= soft:
                report.verdict = RISKY
            else:
                report.verdict = SAFE

    if json_out:
        _print_json(report)
    else:
        _print_human(report)
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(_cli(sys.argv[1:]))