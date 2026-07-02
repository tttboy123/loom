"""Index the runs directory for fast querying. Stdlib only."""
from __future__ import annotations

import os


def build_index(runs_dir: str) -> dict:
    """Scan runs_dir subdirectories; return {run_id: {path, has_log, has_build, mtime}}."""
    if not os.path.isdir(runs_dir):
        return {}
    index = {}
    for entry in os.listdir(runs_dir):
        path = os.path.join(runs_dir, entry)
        if not os.path.isdir(path):
            continue
        index[entry] = {
            "path": path,
            "has_log": os.path.isfile(os.path.join(path, "run-log.md")),
            "has_build": os.path.isdir(os.path.join(path, "build")),
            "mtime": os.path.getmtime(path),
        }
    return index


def find_run(index: dict, run_id: str) -> dict | None:
    """Return index entry for run_id, or None if not found."""
    return index.get(run_id)


def latest_runs(index: dict, n: int = 5) -> list[str]:
    """Return up to n run_ids sorted by mtime descending."""
    return [
        run_id
        for run_id, _ in sorted(index.items(), key=lambda item: item[1]["mtime"], reverse=True)[:n]
    ]
