"""devkit/triager.py — Loom autopilot issue triage (DESIGN-P0 #3a)

Classifies an ``ObserverSnapshot`` into a health verdict with a list of
findings (severity + evidence + recommendation text). Read-only — does NOT
take any repair action. Repairer lives in #3b.

Verdict ladder (best to worst):
    HEALTHY       — autopilot running, all checks pass
    WARN          — minor anomaly (stale heartbeat, one or two lease reclaims)
    DEGRADED      — supervisor up but daemon stuck, or backoff climbing
    STALLED       — no PENDING movement; consec_failures 3-4 (not yet 5)
    QUARANTINED   — watchdog state == quarantined
    HARD_DEAD     — no autopilot state file AND supervisor pid file missing

Each finding carries:
    severity      one of: info | warn | degraded | critical
    code          short stable identifier (e.g. "SUPERVISOR_DEAD")
    evidence      dict of source values that triggered the finding
    message       human-readable one-liner
    hint          text-only recommendation (no auto-action)
"""
from __future__ import annotations

import time
from dataclasses import dataclass, field, asdict
from typing import Optional

from devkit import observer
from devkit.observer import (
    ObserverSnapshot,
    HEARTBEAT_FRESH_S,
    HEARTBEAT_STALE_S,
)


VERDICT_ORDER = ["HEALTHY", "WARN", "DEGRADED", "STALLED", "QUARANTINED", "HARD_DEAD"]
VERDICT_RANK = {v: i for i, v in enumerate(VERDICT_ORDER)}

# ----------------------------------------------------------------------------
# Phase A — incident mapping
# ----------------------------------------------------------------------------
# Sentinel for incidents not tied to a specific work item (autopilot-wide).
# A real work item id, if any, comes from ObserverSnapshot.work_item_id.
SYSTEM_WORK_ITEM_ID = "system:autopilot"

# Maps triager finding code -> incident spec.kind (one of the whitelist
# dispatch keys in devkit/repairer.py). Codes not in this table do not
# produce an incident (transient noise that the watchdog handles on its own).
_FINDING_TO_INCIDENT_KIND: dict[str, str] = {
    "AUTOPILOT_NOT_STARTED": "repair_needed",
    "SUPERVISOR_DEAD": "repair_needed",
    "DAEMON_DEAD": "stale_running",
    "HEARTBEAT_DEAD": "stale_running",
    "HEARTBEAT_STALE": "stale_running",
    "BACKOFF_QUARANTINE_THRESHOLD": "manual_block",
    "BACKOFF_CLIMBING": "stale_running",
    "BACKOFF_MINOR": "stale_running",
    "QUARANTINED": "manual_block",
    "BACKLOG_LEASE_RECLAIMS_HIGH": "stale_running",
    "BACKLOG_LEASE_RECLAIMS": "stale_running",
    "WATCHDOG_HEALING_REPEATED": "stale_running",
}


def _to_incident(finding: "Finding", *, snapshot_work_item_id: Optional[str] = None) -> dict:
    """Convert a Finding to a dict that conforms to incident.schema.json.

    The returned dict has:
      - api_version, kind
      - metadata.id (required), metadata.work_item_id (optional)
      - spec.kind, spec.severity (required), spec.evidence_refs (optional)
    """
    spec_kind = _FINDING_TO_INCIDENT_KIND.get(finding.code)
    work_item_id = str(snapshot_work_item_id or "").strip() or SYSTEM_WORK_ITEM_ID
    # Generate a stable id; severity is appended so the same code at different
    # severities produces distinct incidents.
    incident_id = f"incident-{finding.code.lower()}-{finding.severity}"
    evidence_refs: list = []
    if isinstance(finding.evidence, dict):
        # Surface string values; numeric / None are dropped. Strings equal to
        # "None" (a common sentinel from str(None)) are also dropped.
        for k, v in finding.evidence.items():
            if isinstance(v, str):
                s = v.strip()
                if s and s != "None":
                    evidence_refs.append(s)
            elif isinstance(v, (int, float)):
                evidence_refs.append(f"{k}={v}")
    return {
        "api_version": "loom.dev/v1",
        "kind": "Incident",
        "metadata": {
            "id": incident_id,
            "work_item_id": work_item_id,
            "detected_by": "triager",
            "source_code": finding.code,
        },
        "spec": {
            "kind": spec_kind or "stale_running",
            "severity": finding.severity,
            "message": finding.message,
            "hint": finding.hint,
            "evidence_refs": evidence_refs,
        },
    }


@dataclass
class Finding:
    severity: str
    code: str
    message: str
    evidence: dict = field(default_factory=dict)
    hint: str = ""

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class TriageReport:
    verdict: str
    summary: str
    findings: list  # list[Finding]
    next_verdict_if_worse: Optional[str] = None
    generated_at: str = ""
    # Phase A — incident-compatible output. Each finding → one Incident
    # (see _to_incident). The list is empty when there are no findings.
    incidents: list = field(default_factory=list)  # list[dict] (Incident dicts)

    def to_dict(self) -> dict:
        d = asdict(self)
        return d


# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
def _rank(v: str) -> int:
    return VERDICT_RANK.get(v, 0)


def _escalate(current: str, candidate: str) -> str:
    return candidate if _rank(candidate) > _rank(current) else current


# ----------------------------------------------------------------------------
# Individual checks
# ----------------------------------------------------------------------------
def _check_supervisor(snap: ObserverSnapshot) -> Optional[Finding]:
    sup = snap.supervisor
    ap = snap.autopilot

    # Hard dead: no pid file AND no state file
    if not sup.pid_file_exists and not ap.state_file_exists:
        return Finding(
            severity="critical",
            code="AUTOPILOT_NOT_STARTED",
            message="autopilot never started: no supervisor pid file and no autopilot.state",
            evidence={"supervisor_pid_file": str(snap.source_paths.get("supervisor_pid"))},
            hint="run `./loom autopilot` to start the autopilot supervisor",
        )

    # Pid file present but process dead
    if sup.pid_file_exists and not sup.alive:
        return Finding(
            severity="critical",
            code="SUPERVISOR_DEAD",
            message=f"supervisor pid={sup.pid} not alive (pid file stale)",
            evidence={"supervisor_pid": sup.pid, "pid_file": str(snap.source_paths.get("supervisor_pid"))},
            hint=("watchdog should respawn within seconds; if not, run "
                  "`./loom autopilot` or inspect devkit/logs/watchdog.log"),
        )

    # Pid file missing but state file says running — drift
    if not sup.pid_file_exists and ap.state == "running":
        return Finding(
            severity="warn",
            code="SUPERVISOR_PID_MISSING",
            message="autopilot.state says 'running' but supervisor pid file is missing",
            evidence={"state_file": str(snap.source_paths.get("autopilot_state"))},
            hint="watchdog will detect and respawn; consider re-running `./loom autopilot`",
        )

    return None


def _check_daemon(snap: ObserverSnapshot) -> Optional[Finding]:
    dae = snap.daemon
    sup = snap.supervisor

    # Only complain if supervisor is alive — otherwise it's a SUPERVISOR_DEAD issue
    if not sup.alive:
        return None

    if dae.pid_file_exists and not dae.alive:
        return Finding(
            severity="degraded",
            code="DAEMON_DEAD",
            message=f"daemon pid={dae.pid} not alive despite supervisor running",
            evidence={"daemon_pid": dae.pid, "pid_file": str(snap.source_paths.get("daemon_pid"))},
            hint="watchdog should signal supervisor (SIGUSR1) to self-heal the daemon",
        )

    if not dae.pid_file_exists and snap.autopilot.state == "running":
        return Finding(
            severity="warn",
            code="DAEMON_PID_MISSING",
            message="autopilot running but daemon pid file missing (daemon may be mid-spawn)",
            evidence={"state": snap.autopilot.state},
            hint="if it doesn't appear in 30s, see watchdog.log",
        )

    return None


def _check_heartbeat(snap: ObserverSnapshot) -> Optional[Finding]:
    age = snap.heartbeat_age_s
    if age is None:
        return None

    if age > HEARTBEAT_STALE_S:
        return Finding(
            severity="critical",
            code="HEARTBEAT_DEAD",
            message=f"heartbeat stale: {age:.0f}s old (>{HEARTBEAT_STALE_S}s = dead)",
            evidence={"heartbeat_age_s": round(age, 1)},
            hint="watchdog should signal supervisor; if no respawn, daemon is hung",
        )
    if age > HEARTBEAT_FRESH_S:
        return Finding(
            severity="warn",
            code="HEARTBEAT_STALE",
            message=f"heartbeat stale: {age:.0f}s old (>{HEARTBEAT_FRESH_S}s = stale)",
            evidence={"heartbeat_age_s": round(age, 1)},
            hint="watchdog will backoff; check daemon log if persists",
        )
    return None


def _check_backoff(snap: ObserverSnapshot) -> Optional[Finding]:
    bo = snap.backoff
    if bo.consec_failures == 0:
        return None

    # Critical: at the quarantine threshold (5) or beyond
    if bo.consec_failures >= 5:
        return Finding(
            severity="critical",
            code="BACKOFF_QUARANTINE_THRESHOLD",
            message=f"consecutive failures = {bo.consec_failures} (quarantine threshold)",
            evidence={"consec_failures": bo.consec_failures, "last_reason": bo.last_reason},
            hint="watchdog will quarantine; run `./loom doctor` for diagnosis",
        )

    # Degraded: 3-4 failures (approaching quarantine)
    if bo.consec_failures >= 3:
        return Finding(
            severity="degraded",
            code="BACKOFF_CLIMBING",
            message=f"consecutive failures = {bo.consec_failures} (approaching quarantine at 5)",
            evidence={"consec_failures": bo.consec_failures, "last_reason": bo.last_reason},
            hint="watchdog will backoff; if reason is recurring, fix root cause",
        )

    # Warn: 1-2 failures (transient noise territory)
    return Finding(
        severity="warn",
        code="BACKOFF_MINOR",
        message=f"consecutive failures = {bo.consec_failures} (transient noise territory)",
        evidence={"consec_failures": bo.consec_failures, "last_reason": bo.last_reason},
        hint="watchdog will retry with backoff; should self-heal within a minute",
    )


def _check_quarantine(snap: ObserverSnapshot) -> Optional[Finding]:
    if snap.autopilot.state == "quarantined":
        return Finding(
            severity="critical",
            code="QUARANTINED",
            message=f"autopilot is quarantined: reason={snap.autopilot.reason!r}",
            evidence={
                "state": snap.autopilot.state,
                "reason": snap.autopilot.reason,
                "since": snap.autopilot.since,
            },
            hint="watchdog will NOT auto-restart; manual intervention required (run ./loom doctor)",
        )
    return None


def _check_backlog(snap: ObserverSnapshot) -> Optional[Finding]:
    bl = snap.backlog
    # Many lease reclaims = workers have been crashing
    if bl.lease_reclaimed >= 20:
        return Finding(
            severity="degraded",
            code="BACKLOG_LEASE_RECLAIMS_HIGH",
            message=f"{bl.lease_reclaimed} tasks have lease reclaim history (workers crashing?)",
            evidence={
                "lease_reclaimed": bl.lease_reclaimed,
                "by_reason": bl.last_lease_reclaim_reasons,
            },
            hint="check task-leak patterns; many owner_dead = worker crash loop",
        )
    if bl.lease_reclaimed >= 5:
        return Finding(
            severity="warn",
            code="BACKLOG_LEASE_RECLAIMS",
            message=f"{bl.lease_reclaimed} tasks reclaimed (some worker crashes)",
            evidence={
                "lease_reclaimed": bl.lease_reclaimed,
                "by_reason": bl.last_lease_reclaim_reasons,
            },
            hint="monitor; if growing per day, investigate worker stability",
        )
    return None


def _check_watchdog_activity(snap: ObserverSnapshot) -> Optional[Finding]:
    wd = snap.watchdog
    if wd.sigusr1_count_recent >= 3:
        return Finding(
            severity="degraded",
            code="WATCHDOG_HEALING_REPEATED",
            message=f"watchdog sent {wd.sigusr1_count_recent} SIGUSR1 signals recently (daemon keeps dying)",
            evidence={
                "sigusr1_count_recent": wd.sigusr1_count_recent,
                "heal_count_recent": wd.heal_count_recent,
                "recent_tail": wd.recent_lines[:3],
            },
            hint="frequent self-heal = underlying instability; see daemon log",
        )
    return None


# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------
def triage(snap: ObserverSnapshot) -> TriageReport:
    """Run all checks and return a verdict + findings."""
    from datetime import datetime, timezone

    findings: list = []

    # Order matters: critical first, so they bubble up to verdict
    for fn in (
        _check_quarantine,
        _check_supervisor,
        _check_heartbeat,
        _check_backoff,
        _check_daemon,
        _check_backlog,
        _check_watchdog_activity,
    ):
        f = fn(snap)
        if f:
            findings.append(f)

    # Verdict = worst-severity finding's semantic category
    verdict = "HEALTHY"
    if not snap.supervisor.pid_file_exists and not snap.autopilot.state_file_exists:
        verdict = "HARD_DEAD"
    if snap.autopilot.state == "quarantined":
        verdict = "QUARANTINED"
    for f in findings:
        sev = f.severity
        if sev == "critical":
            if f.code in ("DAEMON_DEAD",):
                verdict = _escalate(verdict, "DEGRADED")
            else:
                verdict = _escalate(verdict, "STALLED")
        elif sev == "degraded":
            verdict = _escalate(verdict, "DEGRADED")
        elif sev == "warn":
            verdict = _escalate(verdict, "WARN")

    # Stalled: consec_failures 3+ but not yet 5
    if 3 <= snap.backoff.consec_failures < 5:
        verdict = _escalate(verdict, "STALLED")

    summary = _build_summary(verdict, findings)
    next_worse = _next_worse(verdict)

    # Phase A: convert each finding into an incident (one per finding) for
    # the repairer to consume via dispatch(). Findings whose code is not in
    # the incident-kind map are intentionally skipped — they are watchdog
    # self-handled signals, not repair actions.
    snapshot_wiid = getattr(snap, "work_item_id", None)
    incidents = [
        _to_incident(f, snapshot_work_item_id=snapshot_wiid)
        for f in findings
        if f.code in _FINDING_TO_INCIDENT_KIND
    ]

    return TriageReport(
        verdict=verdict,
        summary=summary,
        findings=findings,
        next_verdict_if_worse=next_worse,
        generated_at=datetime.now(timezone.utc).isoformat(),
        incidents=incidents,
    )


def _build_summary(verdict: str, findings: list) -> str:
    if verdict == "HEALTHY":
        return "all systems nominal"
    if not findings:
        return f"verdict={verdict} (no findings recorded)"
    codes = ", ".join(f.code for f in findings)
    return f"{verdict}: {len(findings)} finding(s) — {codes}"


def _next_worse(verdict: str) -> Optional[str]:
    idx = VERDICT_RANK.get(verdict, 0)
    if idx + 1 < len(VERDICT_ORDER):
        return VERDICT_ORDER[idx + 1]
    return None


if __name__ == "__main__":
    import json as _json
    snap = observer.snapshot()
    rep = triage(snap)
    out = {
        "verdict": rep.verdict,
        "summary": rep.summary,
        "finding_count": len(rep.findings),
        "incident_count": len(rep.incidents),
        "incidents": rep.incidents,
        "next_verdict_if_worse": rep.next_verdict_if_worse,
    }
    print(_json.dumps(out, ensure_ascii=False, indent=2))