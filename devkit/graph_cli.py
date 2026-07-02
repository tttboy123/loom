"""devkit/graph_cli.py — backlog 依赖图 ASCII 可视化（纯标准库）。"""
from __future__ import annotations
from typing import List, Dict, Set

_KNOWN_STATUSES = ('done', 'pending', 'failed', 'running')

def _status_letter(status: str) -> str:
    s = (status or '').strip().lower()
    mapping = {'done': 'D', 'pending': 'P', 'failed': 'F', 'running': 'R'}
    return mapping.get(s, s[:1].upper() if s else '?')

def _build_lookup(backlog: List[Dict]) -> Dict[str, Dict]:
    return {item.get('id', ''): item for item in backlog if item.get('id') is not None}

def _find_roots(backlog: List[Dict], lookup: Dict[str, Dict]) -> List[str]:
    roots = []
    for item in backlog:
        tid = item.get('id', '')
        deps = item.get('deps') or []
        if not deps or not any(d in lookup for d in deps):
            roots.append(tid)
    return roots

def _walk(node_id: str, lookup: Dict[str, Dict], depth: int, max_depth: int,
          visited: Set[str], lines: List[str]) -> None:
    if depth > max_depth:
        lines.append('  ' * depth + '...')
        return
    if node_id in visited:
        lines.append('  ' * depth + '[cycle] ' + node_id)
        return
    item = lookup.get(node_id)
    if item is None:
        return
    letter = _status_letter(item.get('status', ''))
    lines.append(f"{'  ' * depth}[{letter}] {node_id}")
    deps = item.get('deps') or []
    if deps:
        visited.add(node_id)
        for dep_id in deps:
            _walk(dep_id, lookup, depth + 1, max_depth, visited, lines)
        visited.discard(node_id)

def ascii_tree(backlog: List[Dict], max_depth: int = 5) -> str:
    """渲染依赖图为缩进 ASCII 树。backlog=[] 返回 '(empty backlog)'。"""
    if not backlog:
        return '(empty backlog)'
    lookup = _build_lookup(backlog)
    roots = _find_roots(backlog, lookup)
    if not roots:
        roots = [t.get('id', '') for t in backlog if t.get('id')]
    lines: List[str] = []
    for root_id in roots:
        _walk(root_id, lookup, 0, max_depth, set(), lines)
    return '\n'.join(lines) if lines else '(empty backlog)'

def summary_line(backlog: List[Dict]) -> str:
    """单行摘要：'N tasks: X done, Y pending, Z failed'。"""
    done = sum(1 for t in backlog if t.get('status') == 'done')
    pending = sum(1 for t in backlog if t.get('status') == 'pending')
    failed = sum(1 for t in backlog if t.get('status') == 'failed')
    total = done + pending + failed
    return f'{total} tasks: {done} done, {pending} pending, {failed} failed'
