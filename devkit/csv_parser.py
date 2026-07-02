"""CSV parser / serializer using only the Python standard library."""

from __future__ import annotations

import csv
import io
from typing import Iterable

def parse(text: str, delimiter: str = ",") -> list[dict]:
    """Parse CSV text (first row is header) into a list of dicts."""
    if not text:
        return []
    reader = csv.DictReader(io.StringIO(text), delimiter=delimiter)
    return [dict(row) for row in reader]

def to_csv(rows: list[dict], delimiter: str = ",") -> str:
    """Serialize a list of dicts to a CSV string (header included)."""
    if not rows:
        return ""
    # 从第一行推导字段顺序，保证表头稳定
    fieldnames: Iterable[str] = rows[0].keys()
    buf = io.StringIO()
    writer = csv.DictWriter(buf, fieldnames=list(fieldnames), delimiter=delimiter)
    writer.writeheader()
    for row in rows:
        writer.writerow({k: ("" if v is None else v) for k, v in row.items()})
    return buf.getvalue().rstrip("\r\n")

def filter_rows(rows: list[dict], key: str, value: str) -> list[dict]:
    """Return rows where row[key] == value."""
    return [row for row in rows if row.get(key) == value]
