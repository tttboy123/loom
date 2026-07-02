# devkit/result_formatter.py
"""Format run results into different text forms. Pure stdlib only."""
from typing import Iterable, Mapping

# ---------- to_text ----------

def to_text(result: Mapping[str, object]) -> str:
    """Format a single run result as one human-readable line."""
    rid = result.get('id', '')
    gate = result.get('gate', '')
    tokens = result.get('tokens', 0)
    duration_s = float(result.get('duration_s', 0.0))
    return f'Run {rid}: {gate} | {tokens} tokens | {duration_s:.1f}s'

# ---------- to_table ----------

_TABLE_HEADER = ('id', 'gate', 'tokens', 'duration_s')

def _row(result: Mapping[str, object]) -> list[str]:
    duration_s = float(result.get('duration_s', 0.0))
    return [
        str(result.get('id', '')),
        str(result.get('gate', '')),
        str(result.get('tokens', '')),
        f'{duration_s:.1f}',
    ]

def to_table(results: Iterable[Mapping[str, object]]) -> str:
    """Render results as an aligned text table.

    Empty input returns the literal marker '(no results)'.
    """
    results = list(results)
    if not results:
        return '(no results)'

    rows = [_row(r) for r in results]

    # column widths based on header + all rows
    widths = [len(h) for h in _TABLE_HEADER]
    for row in rows:
        for i, cell in enumerate(row):
            if len(cell) > widths[i]:
                widths[i] = len(cell)

    def _fmt(cells: list[str]) -> str:
        # left-align id/gate, right-align numeric columns
        parts = []
        aligns = ('left', 'left', 'right', 'right')
        for h, cell, w, align in zip(_TABLE_HEADER, cells, widths, aligns):
            if align == 'right':
                parts.append(cell.rjust(w))
            else:
                parts.append(cell.ljust(w))
        return '  '.join(parts)

    header_line = _fmt(list(_TABLE_HEADER))
    sep_line = '  '.join('-' * w for w in widths)
    body_lines = [_fmt(r) for r in rows]
    return '\n'.join([header_line, sep_line, *body_lines])

# ---------- to_summary ----------

def to_summary(results: Iterable[Mapping[str, object]]) -> str:
    """Return a short tally line: '{n} runs: {go} GO, {nogo} NO-GO'."""
    results = list(results)
    n = len(results)
    go = sum(1 for r in results if r.get('gate') == 'GO')
    nogo = sum(1 for r in results if r.get('gate') == 'NO-GO')
    return f'{n} runs: {go} GO, {nogo} NO-GO'
