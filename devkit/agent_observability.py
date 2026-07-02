"""Unified Agent observability snapshot for Loom autonomy."""
from __future__ import annotations

import json
import os
import pathlib
import re
import subprocess
from datetime import datetime

ROOT = pathlib.Path(__file__).resolve().parent.parent
_RUN_ROW_RE = re.compile(
    r"^\|\s*(?P<stage>\w+)\s*\|\s*(?P<carrier>[^|]+)\|\s*(?P<served>[^|]+)\|\s*"
    r"(?P<status>[^|]+)\|\s*(?P<dt>[\d.]+s)\s*\|\s*(?P<tokens>\d+)\s*\|\s*\$(?P<cost>[\d.]+)\s*\|",
    re.MULTILINE,
)
_TASK_LINE_RE = re.compile(r"^-\s*任务[:：]\s*(.+)$", re.MULTILINE)
_GATE_LINE_RE = re.compile(r"^## Gate 建议\s*\n+([^\n]+)", re.MULTILINE)
_FAILURE_CODE_RE = re.compile(r"(?:失败码[:：]\*{0,2}|failure_code[=: ]+)([A-Z][A-Z0-9_]{4,})", re.IGNORECASE)


def _safe_json(path: pathlib.Path, default):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return default


def _iter_backlog_items(data) -> list[dict]:
    if isinstance(data, list):
        return [x for x in data if isinstance(x, dict)]
    if isinstance(data, dict):
        items = data.get("tasks", data.get("items", []))
        if isinstance(items, list):
            return [x for x in items if isinstance(x, dict)]
    return []


def _task_preview(item: dict, limit: int = 96) -> str:
    text = str(item.get("task") or item.get("title") or "").replace("\n", " ").strip()
    return text[:limit]


def _is_pid_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def _read_pid(path: pathlib.Path) -> dict:
    if not path.exists():
        return {"pid": None, "alive": False}
    try:
        pid = int(path.read_text(encoding="utf-8").strip())
    except Exception:
        return {"pid": None, "alive": False}
    return {"pid": pid, "alive": _is_pid_alive(pid)}


def _tmux_session_exists(name: str) -> bool:
    try:
        proc = subprocess.run(
            ["tmux", "has-session", "-t", name],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        return proc.returncode == 0
    except Exception:
        return False


def _latest_run(runs_dir: pathlib.Path) -> dict:
    if not runs_dir.exists():
        return {"id": None, "task": None, "gate": None, "failure_code": None, "silent_zero": False, "suspicious_stages": []}
    dirs = [p for p in runs_dir.iterdir() if p.is_dir()]
    if not dirs:
        return {"id": None, "task": None, "gate": None, "failure_code": None, "silent_zero": False, "suspicious_stages": []}
    auto_dirs = [p for p in dirs if p.name.startswith("auto-")]
    latest_pool = auto_dirs or dirs
    latest = max(latest_pool, key=lambda p: p.stat().st_mtime)
    run_log = latest / "run-log.md"
    if not run_log.exists():
        return {"id": latest.name, "task": None, "gate": None, "failure_code": None, "silent_zero": False, "suspicious_stages": []}
    text = run_log.read_text(encoding="utf-8", errors="replace")
    task_match = _TASK_LINE_RE.search(text)
    gate_match = _GATE_LINE_RE.search(text)
    failure_match = _FAILURE_CODE_RE.search(text)
    suspicious = []
    for row in _RUN_ROW_RE.finditer(text):
        status = row.group("status").strip()
        tokens = int(row.group("tokens"))
        dt = row.group("dt").strip()
        if status == "OK" and tokens == 0 and dt == "0.0s":
            suspicious.append(row.group("stage").strip())
    return {
        "id": latest.name,
        "task": task_match.group(1).strip()[:160] if task_match else None,
        "gate": gate_match.group(1).strip() if gate_match else None,
        "failure_code": failure_match.group(1).strip() if failure_match else None,
        "silent_zero": bool(suspicious),
        "suspicious_stages": suspicious,
    }


def _decision_tail(path: pathlib.Path, limit: int = 5) -> list[dict]:
    if not path.exists():
        return []
    rows = []
    try:
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    except OSError:
        return []
    out = []
    for row in rows[-limit:]:
        out.append({
            "ts": row.get("ts"),
            "task_id": row.get("task_id"),
            "outcome": row.get("outcome"),
            "score": row.get("score"),
        })
    return out


def _queue_summary(backlog_path: pathlib.Path) -> dict:
    items = _iter_backlog_items(_safe_json(backlog_path, []))
    totals = {
        "total": len(items),
        "running": 0,
        "pending": 0,
        "failed": 0,
        "done": 0,
        "stopped": 0,
        "ready": 0,
        "contract_blocked": 0,
        "human_required": 0,
    }
    running = []
    failed = []
    for idx, item in enumerate(items, start=1):
        status = str(item.get("status", "")).lower() or "unknown"
        stop_reason = str(item.get("stop_reason", "")).strip()
        human_required_reason = str(item.get("human_required_reason", "")).strip()
        failure_code = str(item.get("failure_code", "")).strip()
        contract_blocked = failure_code == "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN"
        record = {
            "index": idx,
            "id": str(item.get("id", f"no-id-{idx}")),
            "priority": item.get("priority"),
            "status": status,
            "delivery_mode": str(item.get("delivery_mode", "report-only") or "report-only"),
            "task_kind": str(item.get("task_kind", "")).strip() or None,
            "stop_reason": stop_reason or None,
            "human_required_reason": human_required_reason or None,
            "failure_code": failure_code or None,
            "contract_blocked": contract_blocked,
            "task": _task_preview(item),
        }
        if status == "running":
            totals["running"] += 1
            running.append(record)
        elif status == "pending":
            totals["pending"] += 1
            if not item.get("deps"):
                totals["ready"] += 1
        elif status == "failed":
            totals["failed"] += 1
            failed.append(record)
        elif status == "done":
            totals["done"] += 1
        elif status == "stopped":
            totals["stopped"] += 1
        if contract_blocked:
            totals["contract_blocked"] += 1
        if stop_reason == "human_required_report_only":
            totals["human_required"] += 1
    return {"totals": totals, "running": running, "failed_top": failed[:10]}


def collect(
    backlog_path: str = "devkit/backlog.json",
    runs_dir: str = "devkit/runs",
    decisions_path: str = "devkit/decisions.jsonl",
    iterate_log_path: str = "devkit/logs/iterate-daemon.log",
    supervisor_log_path: str = "devkit/logs/iterate-supervisor.log",
    worker_pid_path: str = "devkit/logs/loom-iterate-daemon.pid",
    tmux_session: str = "loom-autopilot",
) -> dict:
    backlog_file = ROOT / backlog_path if not pathlib.Path(backlog_path).is_absolute() else pathlib.Path(backlog_path)
    runs_root = ROOT / runs_dir if not pathlib.Path(runs_dir).is_absolute() else pathlib.Path(runs_dir)
    decisions_file = ROOT / decisions_path if not pathlib.Path(decisions_path).is_absolute() else pathlib.Path(decisions_path)
    iterate_log = ROOT / iterate_log_path if not pathlib.Path(iterate_log_path).is_absolute() else pathlib.Path(iterate_log_path)
    supervisor_log = ROOT / supervisor_log_path if not pathlib.Path(supervisor_log_path).is_absolute() else pathlib.Path(supervisor_log_path)
    worker_pid = ROOT / worker_pid_path if not pathlib.Path(worker_pid_path).is_absolute() else pathlib.Path(worker_pid_path)

    queue = _queue_summary(backlog_file)
    latest = _latest_run(runs_root)
    pid = _read_pid(worker_pid)
    return {
        "ts": datetime.now().isoformat(timespec="seconds"),
        "autopilot": {
            "tmux_session": tmux_session,
            "tmux_alive": _tmux_session_exists(tmux_session),
            "worker": pid,
            "iterate_log_exists": iterate_log.exists(),
            "supervisor_log_exists": supervisor_log.exists(),
        },
        "queue": queue,
        "latest_run": latest,
        "recent_decisions": _decision_tail(decisions_file),
        "alerts": {
            "silent_zero_suspected": latest["silent_zero"],
            "worker_down": not pid["alive"],
            "queue_failed_nonzero": queue["totals"]["failed"] > 0,
        },
    }
