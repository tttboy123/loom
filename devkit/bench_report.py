# devkit/bench_report.py
"""devkit/bench_report.py - Bench report generator (pure stdlib).

Reads devkit/carrier_bench.json (if present) and produces a single-line text
summary suitable for console / log output.

Functions:
  load_bench(path=None) -> list[dict]
  top_carriers(rows, metric='ok_rate', n=3) -> list[dict]
  format_bench(rows) -> str
"""

import json

DEFAULT_BENCH_PATH = 'devkit/carrier_bench.json'

def load_bench(path=None):
    """Load bench rows from a JSON file. Returns [] when missing or invalid.

    Args:
        path: Filesystem path to a JSON list. Defaults to 'devkit/carrier_bench.json'.

    Returns:
        list[dict]: The parsed list, or [] on missing / unreadable / non-list input.
    """
    if path is None:
        path = DEFAULT_BENCH_PATH
    try:
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
    except (FileNotFoundError, IsADirectoryError, PermissionError, OSError,
            json.JSONDecodeError, ValueError):
        return []
    if not isinstance(data, list):
        return []
    return data

def top_carriers(rows, metric='ok_rate', n=3):
    """Return the top-n rows sorted by ``metric`` descending.

    Missing, None, or non-numeric ``metric`` values are treated as 0.
    Non-dict entries sort as 0.  ``n`` < 0 is clamped to 0.
    """
    def _key(r):
        if not isinstance(r, dict):
            return 0.0
        val = r.get(metric, 0)
        if val is None:
            return 0.0
        try:
            return float(val)
        except (TypeError, ValueError):
            return 0.0

    try:
        n_int = int(n)
    except (TypeError, ValueError):
        n_int = 0
    if n_int < 0:
        n_int = 0
    return sorted(rows, key=_key, reverse=True)[:n_int]

def format_bench(rows):
    """Format rows as a single-line summary.

    Format: 'N 条记录；top carrier: X (ok_rate=Y)'
    Empty input returns '(no bench data)'.
    """
    if not rows:
        return '(no bench data)'
    n = len(rows)
    top = top_carriers(rows, metric='ok_rate', n=1)
    if not top:
        return f'{n} 条记录；top carrier: (none)'
    top_row = top[0]
    carrier = top_row.get('carrier', 'unknown')
    ok_rate = top_row.get('ok_rate', 0)
    return f'{n} 条记录；top carrier: {carrier} (ok_rate={ok_rate})'
