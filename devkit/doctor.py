"""devkit/doctor.py — `loom doctor --autopilot` diagnostic command.

Glues observer + triager + a human-readable report. Used by ``./loom doctor``
shell wrapper. Read-only: collects state and prints findings. NO repairs
(repairer lives in #3b).
"""
from __future__ import annotations

import argparse
import json
import sys

from devkit.observer import snapshot
from devkit.triager import triage


# ----------------------------------------------------------------------------
# Formatting helpers
# ----------------------------------------------------------------------------
SEVERITY_GLYPH = {
    "info": "  · ",
    "warn": "  ⚠  ",
    "degraded": "  ⚠  ",
    "critical": "  ✗ ",
}


def _fmt_age(secs: float | None) -> str:
    if secs is None:
        return "n/a"
    if secs < 60:
        return f"{secs:.0f}s"
    if secs < 3600:
        return f"{secs / 60:.0f}m{secs % 60:.0f}s"
    return f"{secs / 3600:.1f}h"


def _fmt_pid(p: dict | None) -> str:
    if not p:
        return "n/a"
    pid = p.get("pid")
    alive = p.get("alive")
    exists = p.get("pid_file_exists")
    if not exists:
        return "no pid file"
    if not pid:
        return "pid file (unparseable)"
    state = "alive" if alive else "DEAD"
    return f"pid={pid} {state}"


def render_text(snap_dict: dict, report_dict: dict) -> str:
    """Render a human-readable diagnostic report."""
    ap = snap_dict["autopilot"]
    sup = snap_dict["supervisor"]
    dae = snap_dict["daemon"]
    hb = snap_dict.get("heartbeat_age_s")
    bo = snap_dict["backoff"]
    bl = snap_dict["backlog"]
    wd = snap_dict["watchdog"]

    verdict = report_dict["verdict"]
    summary = report_dict["summary"]
    findings = report_dict["findings"]

    lines: list[str] = []
    lines.append("=" * 64)
    lines.append(f"  Loom autopilot diagnostic — verdict: {verdict}")
    lines.append(f"  {summary}")
    lines.append("=" * 64)
    lines.append("")

    # State block
    lines.append("Process state:")
    lines.append(f"  supervisor : {_fmt_pid(sup)}")
    lines.append(f"  daemon     : {_fmt_pid(dae)}")
    lines.append(f"  heartbeat  : {_fmt_age(hb)} ago")
    lines.append(f"  autopilot  : state={ap.get('state')!r}  reason={ap.get('reason')!r}")
    lines.append(f"  backoff    : consec_failures={bo['consec_failures']}  last_reason={bo['last_reason']!r}")
    lines.append("")

    # Backlog
    by_status = bl.get("by_status", {})
    lines.append(f"Backlog ({bl.get('total', 0)} tasks):")
    for s in ("pending", "running", "done", "failed", "stopped", "blocked", "skipped"):
        if s in by_status:
            lines.append(f"  {s:<10s} {by_status[s]}")
    if bl.get("lease_reclaimed"):
        lines.append(f"  lease reclaimed: {bl['lease_reclaimed']}  reasons={bl.get('last_lease_reclaim_reasons')}")
    lines.append("")

    # Watchdog recent
    if wd.get("recent_lines"):
        lines.append(f"Watchdog (last {len(wd['recent_lines'])} lines):")
        for line in wd["recent_lines"][-5:]:
            lines.append(f"  | {line[:100]}")
        lines.append("")

    # Findings
    if not findings:
        lines.append("Findings: none — no anomalies detected.")
    else:
        lines.append(f"Findings ({len(findings)}):")
        for f in findings:
            glyph = SEVERITY_GLYPH.get(f["severity"], "  · ")
            lines.append(f"{glyph}[{f['severity']}] {f['code']}")
            lines.append(f"      {f['message']}")
            if f.get("hint"):
                lines.append(f"      hint: {f['hint']}")
            lines.append("")

    # Next verdict
    next_w = report_dict.get("next_verdict_if_worse")
    if next_w:
        lines.append(f"Next worse verdict: {next_w}")

    return "\n".join(lines) + "\n"


# ----------------------------------------------------------------------------
# CLI
# ----------------------------------------------------------------------------
def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(
        prog="devkit.doctor",
        description="Loom autopilot diagnostic (observer + triager)",
    )
    p.add_argument("--autopilot", action="store_true",
                   help="Run autopilot diagnostic (default behavior)")
    p.add_argument("--json", action="store_true",
                   help="Output JSON instead of human-readable text")
    p.add_argument("--quiet", action="store_true",
                   help="Exit 0 if HEALTHY, 1 otherwise (for scripts)")
    args = p.parse_args(argv)

    snap = snapshot()
    report = triage(snap)

    if args.json:
        out = {
            "snapshot": snap.to_dict(),
            "report": report.to_dict(),
        }
        print(json.dumps(out, indent=2, ensure_ascii=False, default=str))
    else:
        print(render_text(snap.to_dict(), report.to_dict()))

    if args.quiet:
        return 0 if report.verdict == "HEALTHY" else 1
    return 0


if __name__ == "__main__":
    sys.exit(main())