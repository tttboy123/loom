# devkit/backoff_timer.py
"""
Exponential backoff timer (pure stdlib).

Functions:
    create(base, multiplier, max_delay) -> dict
    next_delay(timer) -> tuple[float, dict]
    reset(timer) -> dict
    timer_summary(timer) -> dict

Timer is an immutable-style dict: every mutating operation returns a new dict
so callers can compose operations without surprise aliasing.
"""

from __future__ import annotations

from typing import Tuple

__all__ = ["create", "next_delay", "reset", "timer_summary"]

def create(base: float = 1.0, multiplier: float = 2.0, max_delay: float = 60.0) -> dict:
    """Create a fresh backoff timer.

    Raises ValueError on invalid parameters:
      - base must be >= 0
      - multiplier must be > 1
      - max_delay must be > 0
    """
    if not isinstance(base, (int, float)) or base < 0:
        raise ValueError("base must be a non-negative number")
    if not isinstance(multiplier, (int, float)) or multiplier <= 1:
        raise ValueError("multiplier must be a number greater than 1")
    if not isinstance(max_delay, (int, float)) or max_delay <= 0:
        raise ValueError("max_delay must be a positive number")

    return {
        "base": float(base),
        "multiplier": float(multiplier),
        "max_delay": float(max_delay),
        "attempt": 0,
    }

def next_delay(timer: dict) -> Tuple[float, dict]:
    """Compute the next delay and return (delay, new_timer_with_attempt+1).

    delay = min(base * multiplier**attempt, max_delay)
    """
    _require_timer(timer)

    base = timer["base"]
    multiplier = timer["multiplier"]
    max_delay = timer["max_delay"]
    attempt = timer["attempt"]

    delay = min(base * (multiplier ** attempt), max_delay)
    new_timer = {
        "base": base,
        "multiplier": multiplier,
        "max_delay": max_delay,
        "attempt": attempt + 1,
    }
    return delay, new_timer

def reset(timer: dict) -> dict:
    """Return a new timer with attempt reset to 0; other fields unchanged."""
    _require_timer(timer)

    return {
        "base": timer["base"],
        "multiplier": timer["multiplier"],
        "max_delay": timer["max_delay"],
        "attempt": 0,
    }

def timer_summary(timer: dict) -> dict:
    """Return a snapshot describing the current state without advancing it."""
    _require_timer(timer)

    base = timer["base"]
    multiplier = timer["multiplier"]
    max_delay = timer["max_delay"]
    attempt = timer["attempt"]

    next_d = min(base * (multiplier ** attempt), max_delay)

    return {
        "attempt": int(attempt),
        "next_delay": float(next_d),
        "max_delay": float(max_delay),
    }

# -------- internal --------
def _require_timer(timer: dict) -> None:
    if not isinstance(timer, dict):
        raise TypeError("timer must be a dict produced by create()")
    required = {"base", "multiplier", "max_delay", "attempt"}
    if not required.issubset(timer.keys()):
        raise ValueError(f"timer missing keys: {required - timer.keys()}")
