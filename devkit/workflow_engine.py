"""
devkit/workflow_engine.py
Pure stdlib sequential step workflow engine.

State shape:
    {
        "steps":   list[str],
        "current": int,            # index of the *next* step to run
        "status":  "pending" | "running" | "done",
        "results": dict[int, dict] # results keyed by step index
    }
"""

from __future__ import annotations
from typing import Any

_STATUS_PENDING = "pending"
_STATUS_RUNNING = "running"
_STATUS_DONE = "done"

def create(steps: list[str]) -> dict:
    """Initialize a workflow with the given step names."""
    if not isinstance(steps, list):
        raise TypeError("steps must be a list of strings")
    return {
        "steps": list(steps),
        "current": 0,
        "status": _STATUS_PENDING,
        "results": {},
    }

def advance(wf: dict, step_result: dict) -> dict:
    """
    Record the result of the current step and move forward.
    Returns a *new* state dict; does not mutate the input.
    """
    if not isinstance(wf, dict):
        raise TypeError("wf must be a dict")
    if wf.get("status") == _STATUS_DONE:
        # Past the end: stay done, no new result recorded.
        return {
            "steps": list(wf["steps"]),
            "current": wf["current"],
            "status": _STATUS_DONE,
            "results": dict(wf["results"]),
        }

    new_results = dict(wf.get("results", {}))
    idx = wf["current"]
    new_results[idx] = dict(step_result) if step_result is not None else {}

    new_current = idx + 1
    steps = wf["steps"]
    if new_current >= len(steps):
        new_status = _STATUS_DONE
    else:
        new_status = _STATUS_RUNNING

    return {
        "steps": list(steps),
        "current": new_current,
        "status": new_status,
        "results": new_results,
    }

def current_step(wf: dict) -> str | None:
    """Return the name of the current step, or None when done."""
    if wf.get("status") == _STATUS_DONE:
        return None
    steps = wf.get("steps", [])
    i = wf.get("current", 0)
    if 0 <= i < len(steps):
        return steps[i]
    return None

def wf_summary(wf: dict) -> dict:
    """Return a small summary dict describing workflow progress."""
    total = len(wf.get("steps", []))
    completed = len(wf.get("results", {}))
    return {
        "total": total,
        "completed": completed,
        "status": wf.get("status", _STATUS_PENDING),
        "current_step": current_step(wf),
    }

__all__ = ["create", "advance", "current_step", "wf_summary"]
