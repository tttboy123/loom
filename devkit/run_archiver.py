# devkit/run_archiver.py
"""Run archive record management — pure stdlib."""
from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

def make_archive_entry(run_id: str, gate: str, tokens: int) -> dict[str, Any]:
    """Create a single archive record with an ISO-8601 timestamp."""
    return {
        "run_id": run_id,
        "gate": gate,
        "tokens": tokens,
        "archived_at": datetime.now(timezone.utc).isoformat(),
    }

def filter_by_gate(entries: list[dict[str, Any]], gate: str) -> list[dict[str, Any]]:
    """Return entries whose gate matches the given gate string."""
    return [e for e in entries if e.get("gate") == gate]

def archive_stats(entries: list[dict[str, Any]] | dict[str, Any]) -> dict[str, int]:
    """Aggregate counters over a list (or dict) of archive entries.

    Accepts list for the documented contract; also tolerates a dict
    (e.g. ``{run_id: entry}``) to stay robust against caller variation.
    """
    items = list(entries.values()) if isinstance(entries, dict) else list(entries)
    go = sum(1 for e in items if e.get("gate") == "GO")
    nogo = sum(1 for e in items if e.get("gate") == "NO-GO")
    total_tokens = sum(int(e.get("tokens", 0)) for e in items)
    return {
        "total": len(items),
        "go_count": go,
        "nogo_count": nogo,
        "total_tokens": total_tokens,
    }

def latest(entries: list[dict[str, Any]], n: int) -> list[dict[str, Any]]:
    """Return up to ``n`` most-recent entries, newest first.

    Entries lacking a parseable ``archived_at`` are treated as oldest
    (sorted to the end) so they don't poison the top of the result.
    """
    if n <= 0:
        return []

    def _key(e: dict[str, Any]) -> str:
        return str(e.get("archived_at", ""))

    return sorted(entries, key=_key, reverse=True)[:n]
