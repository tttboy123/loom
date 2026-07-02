"""devkit/stage_scheduler.py — pure-stdlib stage scheduling."""
from __future__ import annotations

from typing import Iterable

def schedule(stages: list[str], deps: dict) -> list[str]:
    """Topological sort of stages by deps.

    deps: {stage: [dep_stages]} — keys' values list stages that must come BEFORE key.
    Returns sorted list; on cycle, returns the original `stages` order.
    """
    if not stages:
        return []

    stage_set = set(stages)
    # Normalize: only consider deps that are in the stage list
    norm_deps: dict[str, list[str]] = {s: [] for s in stages}
    for s, ds in deps.items():
        if s in stage_set:
            norm_deps[s] = [d for d in ds if d in stage_set]

    # DFS-based topological sort with cycle detection
    WHITE, GRAY, BLACK = 0, 1, 2
    color: dict[str, int] = {s: WHITE for s in stages}
    order: list[str] = []
    has_cycle = False

    def visit(node: str) -> None:
        nonlocal has_cycle
        if has_cycle:
            return
        if color[node] == GRAY:  # back edge -> cycle
            has_cycle = True
            return
        if color[node] == BLACK:
            return
        color[node] = GRAY
        for d in norm_deps.get(node, []):
            if d in color:
                visit(d)
        color[node] = BLACK
        order.append(node)

    for s in stages:
        if color[s] == WHITE:
            visit(s)
        if has_cycle:
            return list(stages)

    return order

def next_stage(scheduled: list[str], completed: set[str]) -> str | None:
    """Return the first stage not in `completed`, or None if all done."""
    for s in scheduled:
        if s not in completed:
            return s
    return None

def schedule_summary(stages: list[str], completed: set[str]) -> dict:
    """Return {total, done, remaining, next} for the schedule."""
    total = len(stages)
    # Count only completed items that are actually in the schedule
    done = sum(1 for s in stages if s in completed)
    remaining = total - done
    nxt = next_stage(stages, completed)
    return {"total": total, "done": done, "remaining": remaining, "next": nxt}
