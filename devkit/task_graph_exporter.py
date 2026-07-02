"""devkit/task_graph_exporter.py

Pure-stdlib exporters for a task graph. A task graph is a dict:

    {
        "nodes": [node_id, ...],
        "edges": [(src, dst), ...],
    }

This module is the **draft** implementation; it is not yet wired into
any real repo. See the Loom Charter: report-only, tests-first.
"""
from __future__ import annotations

import json
from typing import Dict, List, Tuple

__all__ = ["to_dot", "to_adjacency_list", "to_json"]

def to_dot(graph: dict) -> str:
    """Return a Graphviz DOT representation of the task graph.

    The returned string starts with 'digraph {' and ends with '}'.
    """
    nodes: List[str] = list(graph.get("nodes", []))
    edges: List[Tuple[str, str]] = list(graph.get("edges", []))

    lines: List[str] = ["digraph {"]
    lines.append("  // nodes")
    for n in nodes:
        lines.append(f'  "{n}";')
    lines.append("  // edges")
    for src, dst in edges:
        lines.append(f'  "{src}" -> "{dst}";')
    lines.append("}")
    return "\n".join(lines)

def to_adjacency_list(graph: dict) -> Dict[str, List[str]]:
    """Return {node_id: [neighbor_ids, ...]}.

    Every node in `graph["nodes"]` is guaranteed to have a key,
    even if it has no outgoing edges.
    """
    nodes: List[str] = list(graph.get("nodes", []))
    edges: List[Tuple[str, str]] = list(graph.get("edges", []))

    adj: Dict[str, List[str]] = {n: [] for n in nodes}
    for src, dst in edges:
        # If the source is not declared in `nodes`, we still surface it
        # so the adjacency list is a faithful view of the edges.
        adj.setdefault(src, []).append(dst)
    return adj

def to_json(graph: dict) -> str:
    """Return a JSON string serialization of the task graph."""
    return json.dumps({
        "nodes": list(graph.get("nodes", [])),
        "edges": [list(e) for e in graph.get("edges", [])],
    })
