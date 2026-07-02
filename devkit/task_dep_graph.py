# devkit/task_dep_graph.py
from collections import defaultdict

def build_graph(backlog: list[dict]) -> dict:
    nodes = []
    edges = []
    id_to_entry = {}
    for entry in backlog:
        tid = entry['id']
        nodes.append(tid)
        id_to_entry[tid] = entry
    for entry in backlog:
        tid = entry['id']
        for dep in entry.get('deps', []):
            edges.append((dep, tid))
    # compute roots
    has_incoming = set()
    for src, dst in edges:
        has_incoming.add(dst)
    roots = [tid for tid in nodes if tid not in has_incoming]
    return {'nodes': nodes, 'edges': edges, 'roots': roots}

def render_ascii(graph: dict) -> str:
    if not graph['nodes']:
        return '(empty graph)'
    # build adjacency from parent to children
    children = defaultdict(list)
    for src, dst in graph['edges']:
        children[src].append(dst)
    lines = []

    def dfs(node_id: str, depth: int):
        lines.append('  ' * depth + node_id)
        for child in children.get(node_id, []):
            dfs(child, depth + 1)

    for root in graph['roots']:
        dfs(root, 0)
    return '\n'.join(lines)

def node_info(graph: dict, node_id: str) -> dict:
    exists = node_id in graph['nodes']
    in_degree = 0
    out_degree = 0
    if exists:
        for src, dst in graph['edges']:
            if dst == node_id:
                in_degree += 1
            if src == node_id:
                out_degree += 1
    return {'id': node_id, 'in_degree': in_degree, 'out_degree': out_degree, 'exists': exists}
