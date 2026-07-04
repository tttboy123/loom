"""devkit/observer.py — Loom autopilot state observer (DESIGN-P0 #3a)

Read-only: collects a snapshot of Loom autopilot state from well-known files
under ``devkit/logs/`` and ``devkit/backlog.json``. Used by ``loom doctor``
and (in #3b) by the repairer to decide what to fix.

NOT thread-safe; called as a one-shot snapshot, not as a watcher.
"""
from __future__ import annotations

import json
import os
import pathlib
import re
import subprocess
import time
from dataclasses import dataclass, field, asdict
from typing import Optional

# ----------------------------------------------------------------------------
# Defaults — paths are Loom conventions
# ----------------------------------------------------------------------------
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
LOGS_DIR = REPO_ROOT / "devkit" / "logs"
BACKLOG_PATH = REPO_ROOT / "devkit" / "backlog.json"
WATCHDOG_LOG = LOGS_DIR / "watchdog.log"
SUPERVISOR_PID = LOGS_DIR / "loom-iterate-supervisor.pid"
DAEMON_PID = LOGS_DIR / "loom-iterate-daemon.pid"
HEARTBEAT_DAEMON = LOGS_DIR / "heartbeat.daemon"
AUTOPILOT_STATE = LOGS_DIR / "autopilot.state"
BACKOFF_PATH = LOGS_DIR / "backoff.json"

# Heartbeat freshness thresholds
HEARTBEAT_FRESH_S = 60         # < 60s = fresh
HEARTBEAT_STALE_S = 180        # 60-180s = stale; > 180s = dead
WATCHDOG_LOG_TAIL_LINES = 20   # how many watchdog events to summarize


# ----------------------------------------------------------------------------
# Data model
# ----------------------------------------------------------------------------
@dataclass
class AutopilotState:
    """Raw state snapshot of the autopilot subprocess chain."""
    state: Optional[str] = None       # running / quarantined / dead
    reason: Optional[str] = None
    since: Optional[str] = None
    supervisor_pid: Optional[str] = None
    daemon_pid: Optional[str] = None
    last_heartbeat: Optional[str] = None
    state_file_exists: bool = False


@dataclass
class BackoffState:
    consec_failures: int = 0
    last_reason: Optional[str] = None
    last_attempt_at: Optional[str] = None


@dataclass
class ProcessState:
    pid: Optional[int] = None
    alive: bool = False
    pid_file_exists: bool = False


@dataclass
class BacklogState:
    total: int = 0
    by_status: dict = field(default_factory=dict)
    by_priority: dict = field(default_factory=dict)
    lease_reclaimed: int = 0
    last_lease_reclaim_reasons: dict = field(default_factory=dict)


@dataclass
class WatchdogEvents:
    recent_lines: list = field(default_factory=list)  # raw last N lines
    heal_count_recent: int = 0     # "restarted" events in last N lines
    sigusr1_count_recent: int = 0  # SIGUSR1 signals in last N lines


@dataclass
class ObserverSnapshot:
    """Aggregated, read-only state of the Loom autopilot."""
    autopilot: AutopilotState = field(default_factory=AutopilotState)
    backoff: BackoffState = field(default_factory=BackoffState)
    supervisor: ProcessState = field(default_factory=ProcessState)
    daemon: ProcessState = field(default_factory=ProcessState)
    heartbeat_age_s: Optional[float] = None
    backlog: BacklogState = field(default_factory=BacklogState)
    watchdog: WatchdogEvents = field(default_factory=WatchdogEvents)
    collected_at: str = ""
    source_paths: dict = field(default_factory=dict)
    # Phase A — primary work item this snapshot is "about". Triager uses this
    # to attach metadata.work_item_id to the incidents it produces. ``None``
    # means the snapshot is autopilot-wide (e.g. quarantine / supervisor
    # missing) and the triager falls back to a system sentinel.
    work_item_id: Optional[str] = None

    def to_dict(self) -> dict:
        d = asdict(self)
        return d


# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
def _pid_alive(pid: int) -> bool:
    """Cross-platform pid alive check (kill -0 semantics)."""
    try:
        if os.name == "posix":
            os.kill(pid, 0)
            return True
    except (ProcessLookupError, PermissionError):
        return False
    except Exception:  # noqa: BLE001
        return False
    return False


def _read_json(path: pathlib.Path) -> Optional[dict]:
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None


def _parse_iso_age(iso: Optional[str]) -> Optional[float]:
    """Return age in seconds for an ISO-8601 timestamp; None if unparseable.

    Naive timestamps are interpreted as local time (matches what
    ``state_writer.now_iso()`` writes — its ``datetime.now()`` is local).
    """
    if not iso:
        return None
    try:
        cleaned = iso.replace("Z", "+00:00")
        from datetime import datetime
        ts = datetime.fromisoformat(cleaned)
        if ts.tzinfo:
            from datetime import timezone
            now = datetime.now(tz=ts.tzinfo)
        else:
            # Naive → assume local
            now = datetime.now()
        return (now - ts).total_seconds()
    except Exception:  # noqa: BLE001
        return None


def _parse_pid_file(path: pathlib.Path) -> tuple[Optional[int], bool]:
    if not path.exists():
        return None, False
    try:
        content = path.read_text(encoding="utf-8").strip()
        pid = int(content.split()[0]) if content else None
        return pid, pid is not None
    except (OSError, ValueError):
        return None, True


# ----------------------------------------------------------------------------
# Per-source collectors
# ----------------------------------------------------------------------------
def collect_autopilot_state(state_path: pathlib.Path = AUTOPILOT_STATE) -> AutopilotState:
    data = _read_json(state_path)
    if not data:
        return AutopilotState(state_file_exists=state_path.exists())
    return AutopilotState(
        state=data.get("state"),
        reason=data.get("reason"),
        since=data.get("since"),
        supervisor_pid=data.get("supervisor_pid"),
        daemon_pid=data.get("daemon_pid"),
        last_heartbeat=data.get("last_heartbeat"),
        state_file_exists=True,
    )


def collect_backoff(path: pathlib.Path = BACKOFF_PATH) -> BackoffState:
    data = _read_json(path)
    if not data:
        return BackoffState()
    return BackoffState(
        consec_failures=int(data.get("consec_failures", 0) or 0),
        last_reason=data.get("last_reason"),
        last_attempt_at=data.get("last_attempt_at"),
    )


def collect_process(pid_path: pathlib.Path) -> ProcessState:
    pid, exists = _parse_pid_file(pid_path)
    return ProcessState(pid=pid, alive=_pid_alive(pid) if pid else False, pid_file_exists=exists)


def collect_backlog(path: pathlib.Path = BACKLOG_PATH) -> BacklogState:
    data = _read_json(path)
    if not data:
        return BacklogState()
    # Backlog may be a bare list or {"tasks": [...]}
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        tasks = data["tasks"]
    elif isinstance(data, list):
        tasks = data
    else:
        tasks = []
    by_status: dict = {}
    by_priority: dict = {}
    lease_reasons: dict = {}
    for t in tasks:
        if not isinstance(t, dict):
            continue
        s = t.get("status", "unknown")
        by_status[s] = by_status.get(s, 0) + 1
        p = t.get("priority", "unknown")
        by_priority[p] = by_priority.get(p, 0) + 1
        reason = t.get("_lease_reclaim_reason")
        if reason:
            lease_reasons[reason] = lease_reasons.get(reason, 0) + 1
    return BacklogState(
        total=len(tasks),
        by_status=by_status,
        by_priority=by_priority,
        lease_reclaimed=sum(lease_reasons.values()),
        last_lease_reclaim_reasons=lease_reasons,
    )


def _first_recent_running_work_item_id(path: pathlib.Path) -> Optional[str]:
    """Return the id of the most recently seen ``running`` task, if any.

    Phase A helper: the triager needs a primary work item to attach to the
    incidents it emits. Prefer running > blocked > failed. If the backlog is
    a bare list (not a dict wrapper), that path is also handled.
    """
    data = _read_json(path)
    if not data:
        return None
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        items = data["tasks"]
    elif isinstance(data, list):
        items = data
    else:
        return None
    # Priority order: running first, then blocked, then failed, then anything
    for target_status in ("running", "blocked", "failed"):
        for t in items:
            if not isinstance(t, dict):
                continue
            if str(t.get("status", "")).lower() == target_status:
                wid = str(t.get("id", "")).strip()
                if wid:
                    return wid
    return None


def collect_watchdog(log_path: pathlib.Path = WATCHDOG_LOG,
                     tail_lines: int = WATCHDOG_LOG_TAIL_LINES) -> WatchdogEvents:
    if not log_path.exists():
        return WatchdogEvents()
    try:
        # tail -n on POSIX
        out = subprocess.run(
            ["tail", "-n", str(tail_lines), str(log_path)],
            capture_output=True, text=True, timeout=5,
        )
        lines = [l for l in out.stdout.splitlines() if l.strip()]
    except Exception:  # noqa: BLE001
        return WatchdogEvents()
    heal_count = sum(1 for l in lines if "restarted" in l.lower())
    sigusr1_count = sum(1 for l in lines if "SIGUSR1" in l)
    return WatchdogEvents(
        recent_lines=lines,
        heal_count_recent=heal_count,
        sigusr1_count_recent=sigusr1_count,
    )


# ----------------------------------------------------------------------------
# Main entry
# ----------------------------------------------------------------------------
def snapshot(
    *,
    logs_dir: Optional[pathlib.Path] = None,
    backlog_path: Optional[pathlib.Path] = None,
) -> ObserverSnapshot:
    """Collect a one-shot read-only snapshot of Loom autopilot state.

    Paths default to Loom's standard layout but can be overridden for tests
    (resolved at call time, not definition time, so tests can patch).
    """
    from datetime import datetime, timezone

    logs_dir = pathlib.Path(logs_dir) if logs_dir is not None else LOGS_DIR
    backlog_path = pathlib.Path(backlog_path) if backlog_path is not None else BACKLOG_PATH

    ap = collect_autopilot_state(logs_dir / "autopilot.state")
    bo = collect_backoff(logs_dir / "backoff.json")
    sup = collect_process(logs_dir / "loom-iterate-supervisor.pid")
    dae = collect_process(logs_dir / "loom-iterate-daemon.pid")

    # Heartbeat age: prefer daemon heartbeat file (numeric epoch)
    heartbeat_age: Optional[float] = None
    hb_path = logs_dir / "heartbeat.daemon"
    if hb_path.exists():
        try:
            content = hb_path.read_text(encoding="utf-8").strip()
            if content:
                ts = float(content.split()[0])
                heartbeat_age = max(0.0, time.time() - ts)
        except (OSError, ValueError):
            heartbeat_age = None
    if heartbeat_age is None and ap.last_heartbeat:
        heartbeat_age = _parse_iso_age(ap.last_heartbeat)

    bl = collect_backlog(backlog_path)
    wd = collect_watchdog(logs_dir / "watchdog.log")
    work_item_id = _first_recent_running_work_item_id(backlog_path)

    return ObserverSnapshot(
        autopilot=ap,
        backoff=bo,
        supervisor=sup,
        daemon=dae,
        heartbeat_age_s=heartbeat_age,
        backlog=bl,
        watchdog=wd,
        collected_at=datetime.now(timezone.utc).isoformat(),
        source_paths={
            "autopilot_state": str(logs_dir / "autopilot.state"),
            "backoff": str(logs_dir / "backoff.json"),
            "supervisor_pid": str(logs_dir / "loom-iterate-supervisor.pid"),
            "daemon_pid": str(logs_dir / "loom-iterate-daemon.pid"),
            "heartbeat": str(hb_path),
            "backlog": str(backlog_path),
            "watchdog_log": str(logs_dir / "watchdog.log"),
        },
        work_item_id=work_item_id,
    )