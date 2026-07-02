"""Run state machine: pending → running → done | failed; running → blocked → running. Stdlib only."""
from __future__ import annotations

_TRANSITIONS: dict[str, dict[str, str]] = {
    "pending":  {"start": "running"},
    "running":  {"succeed": "done", "fail": "failed", "block": "blocked"},
    "blocked":  {"retry": "running"},
    "done":     {},
    "failed":   {},
}

INITIAL_STATE = "pending"


def transition(state: str, event: str) -> str:
    """Apply event to state; return new state. Invalid transitions return original state."""
    return _TRANSITIONS.get(state, {}).get(event, state)


def valid_transitions(state: str) -> list[str]:
    """Return list of valid events for the given state."""
    return list(_TRANSITIONS.get(state, {}).keys())


def state_summary(history: list[dict]) -> dict:
    """Replay history entries {state, event} and return {final_state, steps, has_retry}."""
    final_state = INITIAL_STATE
    steps = 0
    has_retry = False
    for entry in history:
        if not isinstance(entry, dict):
            continue
        s = entry.get("state")
        ev = entry.get("event")
        if not isinstance(s, str) or not isinstance(ev, str):
            continue
        if s not in _TRANSITIONS:
            continue
        steps += 1
        if ev == "retry":
            has_retry = True
        final_state = transition(s, ev)
    return {"final_state": final_state, "steps": steps, "has_retry": has_retry}
