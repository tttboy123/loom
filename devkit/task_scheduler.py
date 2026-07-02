# devkit/task_scheduler.py
"""Priority + dependency-aware task scheduler. Standard library only."""
from __future__ import annotations

_PRIORITY = {"high": 0, "medium": 1, "low": 2}


def priority_order(priority: str) -> int:
    return _PRIORITY.get(priority.lower() if priority else "", 3)


def ready_tasks(backlog: list[dict]) -> list[dict]:
    """Pending tasks whose deps are all done, sorted by priority."""
    done_ids = {t["id"] for t in backlog if t.get("status") == "done"}
    result = [
        t for t in backlog
        if t.get("status") == "pending"
        and all(dep in done_ids for dep in t.get("deps", []))
    ]
    return sorted(result, key=lambda t: priority_order(t.get("priority", "")))


def next_task(backlog: list[dict]) -> dict | None:
    ready = ready_tasks(backlog)
    return ready[0] if ready else None


def schedule_order(backlog: list[dict]) -> list[str]:
    """Topological + priority order of all reachable pending tasks."""
    order: list[str] = []
    done_ids = {t["id"] for t in backlog if t.get("status") == "done"}
    remaining = [t for t in backlog if t.get("status") == "pending"]

    for _ in range(len(remaining) + 1):
        if not remaining:
            break
        done_set = set(done_ids) | set(order)
        wave = sorted(
            [t for t in remaining if all(d in done_set for d in t.get("deps", []))],
            key=lambda t: priority_order(t.get("priority", "")),
        )
        if not wave:
            break
        for t in wave:
            order.append(t["id"])
            remaining.remove(t)

    return order
