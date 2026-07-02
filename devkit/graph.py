"""devkit/graph.py — Pure-stdlib directed graph with BFS/DFS.

Design notes:
- nodes stored as list[str] (JSON-serializable; preserves insertion order)
- edges stored as dict[str, list[str]]; neighbor lists are deduped & ordered
- All mutators return a NEW graph (no input mutation) — contract tested.
"""

from __future__ import annotations
from collections import deque
from typing import Dict, List

def create() -> dict:
    """Return a fresh empty graph: {nodes: [], edges: {}}."""
    return {"nodes": [], "edges": {}}

def _add_node_inplace(g: dict, node: str) -> None:
    if node not in g["nodes"]:
        g["nodes"].append(node)

def add_node(g: dict, node: str) -> dict:
    """Return a NEW graph with `node` added."""
    new_g = {"nodes": list(g["nodes"]), "edges": {k: list(v) for k, v in g["edges"].items()}}
    _add_node_inplace(new_g, node)
    return new_g

def add_edge(g: dict, src: str, dst: str) -> dict:
    """Return a NEW graph with directed edge src -> dst (auto-creates nodes)."""
    new_g = {"nodes": list(g["nodes"]), "edges": {k: list(v) for k, v in g["edges"].items()}}
    _add_node_inplace(new_g, src)
    _add_node_inplace(new_g, dst)
    neighbors = new_g["edges"].setdefault(src, [])
    if dst not in neighbors:
        neighbors.append(dst)
    return new_g

def bfs(g: dict, start: str) -> List[str]:
    """Breadth-first traversal from `start`. Returns [] if start not in graph."""
    if start not in g["nodes"]:
        return []
    visited = []
    seen = {start}
    queue = deque([start])
    while queue:
        u = queue.popleft()
        visited.append(u)
        for v in g["edges"].get(u, []):
            if v not in seen:
                seen.add(v)
                queue.append(v)
    return visited

def dfs(g: dict, start: str) -> List[str]:
    """Depth-first traversal from `start`. Returns [] if start not in graph."""
    if start not in g["nodes"]:
        return []
    visited = []
    seen = {start}
    stack = [start]
    while stack:
        u = stack.pop()
        visited.append(u)
        # reverse to keep insertion order when traversing neighbors
        for v in reversed(g["edges"].get(u, [])):
            if v not in seen:
                seen.add(v)
                stack.append(v)
    return visited
