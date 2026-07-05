"""Resolve queue items whose control-plane capabilities are already landed."""
from __future__ import annotations

import pathlib
from typing import Callable


def sweep_resolved_control_plane_tasks(backlog: list[dict]) -> dict:
    resolved_ids = _resolved_task_ids()
    updated: list[dict] = []
    stopped: list[dict] = []
    for item in backlog or []:
        if not isinstance(item, dict):
            continue
        row = dict(item)
        task_id = str(row.get("id", "")).strip()
        if task_id in resolved_ids and str(row.get("status", "")).strip().lower() in {"pending", "failed"}:
            row["status"] = "stopped"
            row["stop_reason"] = "implemented_in_control_plane_migration_2026_07_04"
            row["resolved_by"] = resolved_ids[task_id]
            stopped.append({"id": task_id, "resolved_by": resolved_ids[task_id]})
        updated.append(row)
    return {"backlog": updated, "stopped": stopped}


def _resolved_task_ids() -> dict[str, str]:
    resolved: dict[str, str] = {}
    if _goal_spec_landed():
        for task_id in (
            "goal-spec-v1",
            "goal-spec-v1-retest-contract",
            "unblock-goal-spec-v1-no-applylock",
            "goal-spec-v1-rerun",
            "requeue-goal-spec-v1-attempt2",
        ):
            resolved[task_id] = "goal-spec + tests landed"
    if _goal_submit_landed():
        resolved["goal-submit-cli-v1"] = "goal submit CLI landed"
    if _protocol_read_surface_landed():
        resolved["loom-mcp-read-resources-v1"] = "protocol read resources landed"
    if _protocol_write_surface_landed():
        resolved["loom-mcp-gated-tools-v1"] = "protocol gated tools landed"
    if _cockpit_projection_landed():
        resolved["loom-cockpit-protocol-projection-v1"] = "cockpit protocol projection landed"
    return resolved


def _repo_root() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parent.parent


def _contains(path: str, needle: str) -> bool:
    file_path = _repo_root() / path
    if not file_path.exists():
        return False
    return needle in file_path.read_text(encoding="utf-8", errors="replace")


def _goal_spec_landed() -> bool:
    return _contains("devkit/protocol.py", "class GoalSpec(") and _contains("tests/test_protocol_runtime.py", "test_goal_spec_defaults_and_roundtrip")


def _goal_submit_landed() -> bool:
    return _contains("devkit/goal_submit.py", "def submit_goal_spec(") and _contains("devkit/__main__.py", "def _cmd_goal(")


def _protocol_read_surface_landed() -> bool:
    return _contains("devkit/protocol.py", 'if parsed.path == "/resources"') and _contains("devkit/protocol.py", 'if parsed.path == "/backlog"')


def _protocol_write_surface_landed() -> bool:
    return _contains("devkit/protocol.py", "loom.acquire_lease") and _contains("devkit/protocol.py", "loom.evaluate_gate")


def _cockpit_projection_landed() -> bool:
    return _contains("devkit/protocol.py", "def cockpit_projection(") and _contains("devkit/protocol.py", "latest_artifact_id")
