"""Stage retry policy — pure stdlib.

Decides whether a stage should be retried, picks the next carrier in a
cascade, and summarises a sequence of attempts.
"""
from __future__ import annotations

from typing import Iterable

# Statuses that are eligible for a retry attempt.
_RETRYABLE_STATUSES = frozenset({"failed", "blocked"})

def should_retry(stage: dict, attempt: int, max_attempts: int = 3) -> bool:
    """Return True if the stage may be retried for this attempt.

    A stage is retried when its status is in the retryable set AND the
    attempt counter is still below ``max_attempts``.
    """
    if not isinstance(stage, dict):
        return False
    status = stage.get("status")
    if status not in _RETRYABLE_STATUSES:
        return False
    return attempt < max_attempts

def next_carrier(cascade: list[str], attempt: int) -> str | None:
    """Return the carrier at ``attempt`` index, or None if out of range."""
    if not cascade:
        return None
    if attempt < 0 or attempt >= len(cascade):
        return None
    return cascade[attempt]

def retry_summary(attempts: list[dict]) -> dict:
    """Summarise an attempt log into counts and the last carrier used."""
    total = len(attempts)
    success = 0
    failed = 0
    last_carrier: str | None = None

    for entry in attempts:
        if not isinstance(entry, dict):
            continue
        status = entry.get("status")
        if status == "ok":
            success += 1
        else:
            # Any non-ok terminal attempt (failed / blocked / unknown) counts
            # as failed for the purpose of the summary.
            failed += 1
        carrier = entry.get("carrier")
        if carrier is not None:
            last_carrier = carrier

    return {
        "total": total,
        "success": success,
        "failed": failed,
        "last_carrier": last_carrier,
    }
