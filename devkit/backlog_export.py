"""backlog_export.py

Export a backlog (list[dict]) to CSV / Markdown / JSON.
Pure standard library only.
"""
from __future__ import annotations

import csv
import io
import json
from typing import Optional

DEFAULT_FIELDS: list[str] = ['id', 'status', 'priority']

def to_csv(backlog: list[dict], fields: Optional[list[str]] = None) -> str:
    """Export backlog as CSV string with a header row.

    ``fields`` defaults to ``['id', 'status', 'priority']``. Fields missing on
    a given row are written as empty strings; extra keys in the row are
    ignored.
    """
    if fields is None:
        fields = list(DEFAULT_FIELDS)

    buf = io.StringIO()
    writer = csv.DictWriter(
        buf,
        fieldnames=fields,
        extrasaction='ignore',
        lineterminator='\n',
    )
    writer.writeheader()
    for row in backlog:
        normalized = {f: row.get(f, '') for f in fields}
        writer.writerow(normalized)
    return buf.getvalue()

def to_markdown(backlog: list[dict]) -> str:
    """Export backlog as a Markdown table with columns id | status | priority.

    An empty backlog returns the literal string ``'(empty)'``.
    """
    if not backlog:
        return '(empty)'

    cols = DEFAULT_FIELDS
    lines: list[str] = []
    lines.append('| ' + ' | '.join(cols) + ' |')
    lines.append('| ' + ' | '.join('---' for _ in cols) + ' |')
    for row in backlog:
        cells = [str(row.get(c, '')) for c in cols]
        lines.append('| ' + ' | '.join(cells) + ' |')
    return '\n'.join(lines) + '\n'

def to_json(backlog: list[dict]) -> str:
    """Return ``json.dumps(backlog, ensure_ascii=False, indent=2)``."""
    return json.dumps(backlog, ensure_ascii=False, indent=2)
