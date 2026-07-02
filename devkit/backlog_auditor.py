# devkit/backlog_auditor.py
# Pure stdlib backlog task quality auditor.

from __future__ import annotations

from typing import Any

VALID_STATUSES = ("pending", "done", "failed", "running", "skipped")
VALID_PRIORITIES = ("low", "medium", "high", "critical")

def audit(backlog: list[dict]) -> dict:
    """Audit a backlog of task dicts and return a report.

    Returns
    -------
    dict
        ``{"total": int, "issues": list[dict], "healthy": bool}`` where each
        issue is ``{"id": <task id or "">", "issue": <reason>}``.
    """
    issues: list[dict] = []

    for task in backlog or []:
        if not isinstance(task, dict):
            issues.append({"id": "", "issue": "missing task"})
            continue

        task_id = task.get("id", "")

        # Rule 1: missing or empty 'task' field
        if not task.get("task"):
            issues.append({"id": task_id, "issue": "missing task"})

        # Rule 2: invalid status
        status = task.get("status")
        if status not in VALID_STATUSES:
            issues.append({"id": task_id, "issue": "invalid status"})

        # Rule 3: invalid priority
        priority = task.get("priority")
        if priority not in VALID_PRIORITIES:
            issues.append({"id": task_id, "issue": "invalid priority"})

    return {
        "total": len(backlog) if backlog is not None else 0,
        "issues": issues,
        "healthy": len(issues) == 0,
    }

def fix_status(task: dict) -> dict:
    """Return a shallow copy of ``task`` with ``status`` reset to 'pending'
    when the current value is not one of the valid statuses."""
    fixed = dict(task)  # shallow copy
    if fixed.get("status") not in VALID_STATUSES:
        fixed["status"] = "pending"
    return fixed

def audit_summary(result: dict) -> str:
    """Return a human-readable summary like ``'5 tasks, 0 issues'``."""
    total = result.get("total", 0)
    n = len(result.get("issues", []))
    return f"{total} tasks, {n} issues"
