"""devkit/run_summary.py — 最近 N 个 run 的摘要（纯标准库）。"""
import os
import pathlib
import re

_GATE_RE = re.compile(r"^## Gate 建议\s*\n+([^\n]+)", re.MULTILINE)
_TOTAL_RE = re.compile(r"^- 用量合计：\*\*(\d+)\s+tokens\s+·\s+\$([\d.]+)\*\*", re.MULTILINE)
_STAGE_ROW_RE = re.compile(
    r"^\|\s*[^|]+\|\s*[^|]+\|\s*[^|]+\|\s*[^|]+\|\s*[\d.]+s\s*\|\s*(\d+)\s*\|\s*\$([\d.]+)\s*\|",
    re.MULTILINE,
)

def recent_runs(n=10, runs_dir='runs'):
    """返回最新 n 个 run 的摘要列表，优先展示 auto-* run。"""
    if not os.path.isdir(runs_dir):
        return []

    summaries = []
    for entry in os.listdir(runs_dir):
        run_path = os.path.join(runs_dir, entry)
        if not os.path.isdir(run_path):
            continue
        log_path = os.path.join(run_path, 'run-log.md')
        if not os.path.isfile(log_path):
            continue

        summary = {'run_id': entry, 'gate': '', 'tokens': 0, 'cost_usd': 0.0, 'duration_s': 0.0, 'stages': 0}
        try:
            content = pathlib.Path(log_path).read_text(encoding='utf-8')
        except OSError:
            pass
        else:
            patterns = {
                'gate': (r'^# gate:\s*(.+)$', str),
                'tokens': (r'^# tokens:\s*(\d+)$', int),
                'cost_usd': (r'^# cost_usd:\s*([\d.]+)$', float),
                'duration_s': (r'^# duration_s:\s*([\d.]+)$', float),
                'stages': (r'^# stages:\s*(\d+)$', int),
            }
            for key, (pat, cast) in patterns.items():
                m = re.search(pat, content, re.MULTILINE)
                if m:
                    try:
                        summary[key] = cast(m.group(1))
                    except (ValueError, TypeError):
                        pass
            if not summary['gate']:
                m = _GATE_RE.search(content)
                if m:
                    summary['gate'] = m.group(1).strip()
            if summary['tokens'] == 0 and summary['cost_usd'] == 0.0:
                m = _TOTAL_RE.search(content)
                if m:
                    try:
                        summary['tokens'] = int(m.group(1))
                        summary['cost_usd'] = float(m.group(2))
                    except (ValueError, TypeError):
                        pass
            if summary['stages'] == 0:
                rows = list(_STAGE_ROW_RE.finditer(content))
                if rows:
                    summary['stages'] = len(rows)

        summaries.append(summary)

    auto_rows = [r for r in summaries if str(r.get('run_id', '')).startswith('auto-')]
    pool = auto_rows or summaries
    pool.sort(key=lambda x: x['run_id'], reverse=True)
    return pool[:n]

def format_table(rows):
    """输出固定宽度文本表格，列：run_id | gate | tokens | cost | stages。"""
    if not rows:
        return '(no runs)'

    headers = ['run_id', 'gate', 'tokens', 'cost', 'stages']
    col_widths = {h: len(h) for h in headers}
    for r in rows:
        for h in headers:
            col_widths[h] = max(col_widths[h], len(str(r.get(h, ''))))

    sep = ' | '.join('-' * col_widths[h] for h in headers)
    header_line = ' | '.join(h.ljust(col_widths[h]) for h in headers)
    lines = [sep, header_line, sep]
    for r in rows:
        lines.append(' | '.join(str(r.get(h, '')).ljust(col_widths[h]) for h in headers))
    lines.append(sep)
    return '\n'.join(lines)
