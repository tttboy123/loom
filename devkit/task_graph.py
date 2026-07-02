"""devkit/task_graph.py — backlog 依赖图分析器（纯标准库）"""

from __future__ import annotations


def build_graph(backlog: list[dict]) -> dict:
    """构建依赖图 {nodes, edges, roots, cycles}。"""
    nodes = [t["id"] for t in backlog if "id" in t]
    done_ids = {t["id"] for t in backlog if t.get("status") == "done"}
    edges = []
    for t in backlog:
        tid = t.get("id")
        if not tid:
            continue
        for dep in t.get("deps", []):
            edges.append({"from": dep, "to": tid})

    # roots = no deps, or all deps already done
    roots = []
    for t in backlog:
        tid = t.get("id")
        if not tid:
            continue
        deps = t.get("deps", [])
        if not deps or all(d in done_ids for d in deps):
            roots.append(tid)

    # simple cycle detection via DFS
    adj: dict[str, list[str]] = {n: [] for n in nodes}
    for e in edges:
        if e["from"] in adj:
            adj[e["from"]].append(e["to"])

    visited: set[str] = set()
    rec_stack: set[str] = set()
    cycles: list[list[str]] = []

    def dfs(node: str, path: list[str]) -> None:
        visited.add(node)
        rec_stack.add(node)
        for nb in adj.get(node, []):
            if nb not in visited:
                dfs(nb, path + [nb])
            elif nb in rec_stack:
                cycle_start = path.index(nb) if nb in path else 0
                cycles.append(path[cycle_start:])
        rec_stack.discard(node)

    for n in nodes:
        if n not in visited:
            dfs(n, [n])

    return {"nodes": nodes, "edges": edges, "roots": roots, "cycles": cycles}


def critical_path(graph: dict, backlog: list[dict]) -> list[str]:
    """返回最长依赖链（从根到叶）的 task id 列表。"""
    if not graph.get("nodes"):
        return []

    # build adjacency for longest path (topological ordering via memoization)
    adj: dict[str, list[str]] = {n: [] for n in graph["nodes"]}
    for e in graph.get("edges", []):
        if e["from"] in adj:
            adj[e["from"]].append(e["to"])

    memo: dict[str, list[str]] = {}

    def longest(node: str, visited: set[str]) -> list[str]:
        if node in memo:
            return memo[node]
        if node in visited:
            return [node]
        visited = visited | {node}
        best: list[str] = [node]
        for nb in adj.get(node, []):
            candidate = [node] + longest(nb, visited)
            if len(candidate) > len(best):
                best = candidate
        memo[node] = best
        return best

    best_path: list[str] = []
    for root in graph.get("roots", []) or graph["nodes"]:
        path = longest(root, set())
        if len(path) > len(best_path):
            best_path = path
    return best_path


def ready_tasks(backlog: list[dict]) -> list[dict]:
    """返回 status=pending 且所有 deps 已 done 的任务。"""
    done_ids = {t["id"] for t in backlog if t.get("status") == "done" and "id" in t}
    return [
        t for t in backlog
        if t.get("status") == "pending"
        and all(d in done_ids for d in t.get("deps", []))
    ]
