# devkit/circuit_breaker.py
"""
Circuit breaker state machine (pure stdlib).

States: closed -> open -> half-open -> closed
- closed:    normal; failures counted, success resets counter.
- open:      tripped; calls should be rejected (see is_open).
- half-open: recovery probe; next success closes, failure re-opens.

This module is a draft for the devkit. It does not auto-advance to
half-open based on wall-clock; the caller (or a higher layer) is
responsible for transitioning open -> half-open after `reset_after`.
"""
from __future__ import annotations

from copy import deepcopy
from datetime import datetime, timezone
from typing import Any, Dict

def create(threshold: int = 3, reset_after: int = 60) -> Dict[str, Any]:
    """Create a fresh circuit breaker in the 'closed' state."""
    return {
        "state": "closed",
        "failures": 0,
        "threshold": threshold,
        "reset_after": reset_after,
        "opened_at": None,
    }

def record_success(cb: Dict[str, Any]) -> Dict[str, Any]:
    """Record a successful call. Returns a new cb dict."""
    new = deepcopy(cb)
    if new["state"] == "half-open":
        new["state"] = "closed"
        new["failures"] = 0
    elif new["state"] == "closed":
        new["failures"] = 0
    # state == 'open': do nothing (caller shouldn't be calling through)
    return new

def record_failure(cb: Dict[str, Any]) -> Dict[str, Any]:
    """Record a failed call. Returns a new cb dict."""
    new = deepcopy(cb)
    new["failures"] = new["failures"] + 1
    if new["state"] != "open" and new["failures"] >= new["threshold"]:
        new["state"] = "open"
        new["opened_at"] = datetime.now(timezone.utc).isoformat()
    return new

def is_open(cb: Dict[str, Any]) -> bool:
    """True iff the breaker is in the 'open' state."""
    return cb["state"] == "open"

def cb_summary(cb: Dict[str, Any]) -> Dict[str, Any]:
    """Return a compact view: {state, failures, threshold, is_open}."""
    return {
        "state": cb["state"],
        "failures": cb["failures"],
        "threshold": cb["threshold"],
        "is_open": is_open(cb),
    }
