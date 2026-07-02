"""devkit/run_monitor.py

运行状态监控器（纯标准库）。
"""

from __future__ import annotations

import os
import time
from pathlib import Path
from typing import Any

DEFAULT_RUNS_DIR = "runs"
RUN_LOG_NAME = "run-log.md"
TASK_NAME = "00-task.md"

def _safe_iter_files(run_dir: Path) -> list[str]:
    try:
        if not run_dir.is_dir():
            return []
        return [f.name for f in run_dir.iterdir() if f.is_file()]
    except OSError:
        return []

def _classify(files: list[str]) -> str:
    if RUN_LOG_NAME in files:
        return "done"
    stage_files = [f for f in files if f.endswith(".md") and f != TASK_NAME]
    if stage_files:
        return "running"
    return "empty"

def _latest_stage(files: list[str]) -> str | None:
    md_files = sorted(f for f in files if f.endswith(".md"))
    return md_files[-1] if md_files else None

def _age_minutes(run_dir: Path) -> float:
    latest_mtime: float | None = None
    try:
        for f in run_dir.iterdir():
            try:
                if f.is_file():
                    mt = f.stat().st_mtime
                    if latest_mtime is None or mt > latest_mtime:
                        latest_mtime = mt
            except OSError:
                continue
    except OSError:
        return 0.0
    if latest_mtime is None:
        return 0.0
    return (time.time() - latest_mtime) / 60.0

def list_runs(
    runs_dir: str | os.PathLike | None = None,
) -> list[dict[str, Any]]:
    base = Path(runs_dir) if runs_dir is not None else Path(DEFAULT_RUNS_DIR)
    try:
        if not base.is_dir():
            return []
        sub_iter = list(base.iterdir())
    except OSError:
        return []

    out: list[dict[str, Any]] = []
    for entry in sub_iter:
        if not entry.is_dir():
            continue
        files = _safe_iter_files(entry)
        if not files:
            continue
        status = _classify(files)
        out.append({
            "run_id": entry.name,
            "status": status,
            "stage": _latest_stage(files),
            "age_minutes": float(_age_minutes(entry)),
            "files": files,
        })
    return out

def stale_runs(
    runs_dir: str | os.PathLike | None = None,
    threshold_minutes: float = 30.0,
) -> list[dict[str, Any]]:
    return [
        r for r in list_runs(runs_dir)
        if r["status"] == "running" and r["age_minutes"] > threshold_minutes
    ]

def active_run(
    runs_dir: str | os.PathLike | None = None,
) -> dict[str, Any] | None:
    running = [r for r in list_runs(runs_dir) if r["status"] == "running"]
    if not running:
        return None
    return min(running, key=lambda r: r["age_minutes"])
