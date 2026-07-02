# devkit/run_finalizer.py
"""Collect and archive artifacts from a single run directory. Standard library only."""
from __future__ import annotations

import json
import os


def collect_artifacts(run_dir: str) -> list[str]:
    """Return relative paths of all .md and .json files under run_dir (recursive), sorted."""
    if not os.path.isdir(run_dir):
        return []
    result: list[str] = []
    for dirpath, _, filenames in os.walk(run_dir):
        for fname in filenames:
            if fname.endswith(".md") or fname.endswith(".json"):
                full = os.path.join(dirpath, fname)
                result.append(os.path.relpath(full, run_dir))
    return sorted(result)


def write_summary(run_dir: str, summary: dict) -> bool:
    """Write summary dict as JSON to run_dir/summary.json. Returns False if dir missing."""
    if not os.path.isdir(run_dir):
        return False
    path = os.path.join(run_dir, "summary.json")
    try:
        with open(path, "w", encoding="utf-8") as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)
        return True
    except OSError:
        return False


def finalize(run_dir: str) -> dict:
    """Collect artifacts and return summary dict for run_dir.

    Returns {run_id, artifacts, artifact_count, ok}.
    ok=True when run_dir exists and has at least 1 artifact.
    """
    run_id = os.path.basename(run_dir.rstrip("/\\"))
    if not os.path.isdir(run_dir):
        return {"run_id": run_id, "artifacts": [], "artifact_count": 0, "ok": False}
    artifacts = collect_artifacts(run_dir)
    return {
        "run_id": run_id,
        "artifacts": artifacts,
        "artifact_count": len(artifacts),
        "ok": len(artifacts) > 0,
    }
