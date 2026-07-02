"""Resolve backlog task dependency order. Stdlib only."""
from __future__ import annotations

from collections import defaultdict, deque
from typing import Any, Dict, List, Sequence


def _index_tasks(backlog: Sequence[Dict[str, Any]]) -> Dict[str, Dict[str, Any]]:
    index: Dict[str, Dict[str, Any]] = {}
    for task in backlog or ():
        if isinstance(task, dict) and isinstance(task.get("id"), str):
            index[task["id"]] = task
    return index


def _safe_deps(task: Dict[str, Any]) -> List[str]:
    raw = task.get("deps", []) or []
    seen: set[str] = set()
    out: List[str] = []
    for d in raw:
        if isinstance(d, str) and d not in seen:
            seen.add(d)
            out.append(d)
    return out


def missing_deps(backlog: list[dict]) -> dict:
    """Return {task_id: [missing_dep_id, ...]} for tasks referencing unknown ids."""
    index = _index_tasks(backlog)
    known_ids = set(index.keys())
    result: Dict[str, List[str]] = {}
    for tid, task in index.items():
        missing: List[str] = []
        for d in _safe_deps(task):
            if d not in known_ids and d != tid:
                missing.append(d)
        if missing:
            result[tid] = missing
    return result


def resolve_order(backlog: list[dict]) -> list[str]:
    """Topological sort via Kahn's algorithm. Cycles appended at end without error."""
    index = _index_tasks(backlog)
    if not index:
        return []

    in_deg: Dict[str, int] = {tid: 0 for tid in index}
    adj: Dict[str, List[str]] = defaultdict(list)
    order_index: Dict[str, int] = {tid: i for i, tid in enumerate(index)}

    for tid, task in index.items():
        for d in _safe_deps(task):
            if d in index and d != tid:
                in_deg[tid] += 1
                adj[d].append(tid)

    ready: List[str] = sorted(
        (tid for tid, deg in in_deg.items() if deg == 0),
        key=lambda x: order_index[x],
    )
    ready_q: deque[str] = deque(ready)
    result: List[str] = []
    while ready_q:
        u = ready_q.popleft()
        result.append(u)
        for v in adj.get(u, ()):
            in_deg[v] -= 1
            if in_deg[v] == 0:
                ready_q.append(v)

    if len(result) < len(index):
        emitted = set(result)
        for tid in order_index:
            if tid not in emitted:
                result.append(tid)

    return result


def find_cycles(backlog: list[dict]) -> list[list[str]]:
    """Return all dependency cycles as [a, b, ..., a] paths."""
    index = _index_tasks(backlog)
    if not index:
        return []

    adj: Dict[str, List[str]] = defaultdict(list)
    for tid, task in index.items():
        for d in _safe_deps(task):
            if d in index:
                adj[tid].append(d)

    WHITE, GRAY, BLACK = 0, 1, 2
    color: Dict[str, int] = {tid: WHITE for tid in index}
    parent: Dict[str, str | None] = {tid: None for tid in index}
    cycles: list[list[str]] = []
    seen_signatures: set[tuple[str, ...]] = set()

    def _sig(path: list[str]) -> tuple[str, ...]:
        nodes = path[:-1]
        if not nodes:
            return ()
        m = min(range(len(nodes)), key=lambda i: nodes[i])
        return tuple(nodes[m:] + nodes[:m])

    def dfs(u: str) -> None:
        color[u] = GRAY
        for v in adj.get(u, ()):
            if color[v] == GRAY:
                path: list[str] = [v]
                cur: str | None = u
                while cur is not None and cur != v:
                    path.append(cur)
                    cur = parent[cur]
                path.append(v)
                sig = _sig(path)
                if sig and sig not in seen_signatures:
                    seen_signatures.add(sig)
                    cycles.append(path)
            elif color[v] == WHITE:
                parent[v] = u
                dfs(v)
        color[u] = BLACK

    for tid in index:
        if color[tid] == WHITE:
            dfs(tid)

    return cycles


__all__ = ["resolve_order", "find_cycles", "missing_deps"]
