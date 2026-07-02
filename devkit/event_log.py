"""devkit/event_log.py — Append-only structured event log (JSONL).

Pure standard library. One JSON object per line. Append-friendly, readable
in reverse order via ``read_events(..., n=...)``, filterable by ``type``.

Schema of each line written by ``append_event``::

    {"ts": <float>, "type": <str>, **payload}
"""

from __future__ import annotations

import json
import os
import time


def append_event(path: str, event_type: str, payload: dict = {}) -> None:
    """Append one JSONL event line to ``path``."""
    event = {**payload, "ts": time.time(), "type": event_type}
    parent = os.path.dirname(path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    with open(path, "a", encoding="utf-8") as f:
        f.write(json.dumps(event, ensure_ascii=False) + "\n")


def read_events(path: str, n: int = 100) -> list[dict]:
    """Return the last ``n`` events from the JSONL file; [] if file missing."""
    if not os.path.exists(path):
        return []
    events: list[dict] = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError:
                continue
            if isinstance(obj, dict):
                events.append(obj)
    if n <= 0:
        return []
    return events[-n:]


def filter_events(events: list[dict], event_type: str) -> list[dict]:
    """Return events whose ``type`` equals ``event_type``."""
    return [e for e in events if isinstance(e, dict) and e.get("type") == event_type]
