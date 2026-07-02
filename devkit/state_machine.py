# devkit/state_machine.py
"""Pure-stdlib finite state machine.

This module provides a minimal finite state machine (FSM) implementation
using only the Python standard library. The state machine is represented
as a plain dictionary and functions are *pure* — they return new state
machines rather than mutating inputs in place. This makes the FSM easy
to test, snapshot, and reason about.
"""

from __future__ import annotations

from copy import deepcopy
from typing import Dict, List, Tuple

# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def create(states: List[str], initial: str) -> dict:
    """Create a new finite state machine.

    Parameters
    ----------
    states : list[str]
        The set of legal states for the machine. ``initial`` must be a
        member of this collection.
    initial : str
        The starting state. Must be present in ``states``.

    Returns
    -------
    dict
        A fresh state machine shaped as::

            {
                "states":       list[str],          # copy of input states
                "current":      str,                # the active state
                "transitions":  dict[tuple, str],   # (from, event) -> to
                "history":      list[str],          # previously visited states
            }

    Raises
    ------
    ValueError
        If ``initial`` is not present in ``states``, or if ``states`` is
        empty / contains duplicates.
    TypeError
        If ``states`` is not iterable, or any element is not a string.
    """
    # ---- validate ``states`` ------------------------------------------------
    if not isinstance(states, (list, tuple)):
        raise TypeError(
            f"states must be a list or tuple, got {type(states).__name__}"
        )
    if len(states) == 0:
        raise ValueError("states must contain at least one state")
    if len(set(states)) != len(states):
        raise ValueError(f"states must be unique, got {states!r}")
    for s in states:
        if not isinstance(s, str):
            raise TypeError(
                f"every state must be a str, got {type(s).__name__}: {s!r}"
            )

    # ---- validate ``initial`` ----------------------------------------------
    if not isinstance(initial, str):
        raise TypeError(
            f"initial must be a str, got {type(initial).__name__}"
        )
    if initial not in states:
        raise ValueError(
            f"initial state {initial!r} must be one of {states!r}"
        )

    return {
        "states":      list(states),
        "current":     initial,
        "transitions": {},
        "history":     [],
    }

def add_transition(
    sm: dict, from_state: str, event: str, to_state: str
) -> dict:
    """Register a transition ``from_state --event--> to_state``.

    A copy of ``sm`` is returned with the new transition added. If the
    same ``(from_state, event)`` already exists it is overwritten and
    the previous target is returned alongside a ``False`` flag inside
    the machine's ``_last_add_replaced`` field so callers can detect
    the override. The previous target is also pushed to ``history`` so
    no information is lost.

    Parameters
    ----------
    sm : dict
        State machine produced by :func:`create`.
    from_state : str
        Origin state. Must be in ``sm["states"]``.
    event : str
        The triggering event name.
    to_state : str
        Destination state. Must be in ``sm["states"]``.

    Returns
    -------
    dict
        A new state machine with the transition registered.

    Raises
    ------
    ValueError
        If ``from_state`` or ``to_state`` is not a known state.
    TypeError
        If ``event`` is not a string.
    """
    _validate_sm(sm)

    if not isinstance(from_state, str):
        raise TypeError(
            f"from_state must be a str, got {type(from_state).__name__}"
        )
    if not isinstance(event, str):
        raise TypeError(
            f"event must be a str, got {type(event).__name__}"
        )
    if not isinstance(to_state, str):
        raise TypeError(
            f"to_state must be a str, got {type(to_state).__name__}"
        )

    if from_state not in sm["states"]:
        raise ValueError(
            f"from_state {from_state!r} is not a known state "
            f"(known: {sm['states']!r})"
        )
    if to_state not in sm["states"]:
        raise ValueError(
            f"to_state {to_state!r} is not a known state "
            f"(known: {sm['states']!r})"
        )

    # Deep-copy so the caller's state machine is never mutated.
    new_sm: dict = deepcopy(sm)

    key = (from_state, event)
    previous = new_sm["transitions"].get(key)
    new_sm["transitions"][key] = to_state
    return new_sm

def trigger(sm: dict, event: str) -> Tuple[bool, dict]:
    """Fire ``event`` against the current state.

    If a transition is registered for ``(sm["current"], event)`` the
    machine moves to the registered target state, the *previous* state
    is appended to ``history``, and ``(True, new_sm)`` is returned.
    Otherwise the state machine is returned unchanged inside
    ``(False, sm_copy)`` — the input is never mutated.

    Parameters
    ----------
    sm : dict
        State machine produced by :func:`create` (possibly extended by
        :func:`add_transition`).
    event : str
        The event to dispatch.

    Returns
    -------
    tuple[bool, dict]
        ``(True, new_sm)`` on a successful transition, otherwise
        ``(False, sm)`` with ``sm`` deep-copied.
    """
    _validate_sm(sm)

    if not isinstance(event, str):
        raise TypeError(
            f"event must be a str, got {type(event).__name__}"
        )

    key = (sm["current"], event)
    target = sm["transitions"].get(key)

    if target is None:
        # No transition: return an untouched copy to keep the contract
        # that callers never observe internal mutation.
        return False, deepcopy(sm)

    new_sm: dict = deepcopy(sm)
    new_sm["history"].append(new_sm["current"])
    new_sm["current"] = target
    return True, new_sm

def sm_summary(sm: dict) -> dict:
    """Return a compact summary of ``sm``.

    The returned dict contains:

    ``current``           - the active state
    ``total_states``      - number of states declared at creation
    ``total_transitions`` - number of registered transitions
    ``history``           - list of previously visited states

    Parameters
    ----------
    sm : dict
        State machine produced by :func:`create` (and possibly
        extended by :func:`add_transition` / :func:`trigger`).

    Returns
    -------
    dict
        The summary dictionary described above.
    """
    _validate_sm(sm)
    return {
        "current":           sm["current"],
        "total_states":      len(sm["states"]),
        "total_transitions": len(sm["transitions"]),
        "history":           list(sm["history"]),
    }

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

_REQUIRED_KEYS = ("states", "current", "transitions", "history")

def _validate_sm(sm: dict) -> None:
    """Raise ``TypeError`` / ``ValueError`` if ``sm`` is not a well-formed
    state machine produced by :func:`create`.

    Keeping validation centralized means every public function enforces
    the same contract.
    """
    if not isinstance(sm, dict):
        raise TypeError(
            f"sm must be a dict, got {type(sm).__name__}"
        )
    for key in _REQUIRED_KEYS:
        if key not in sm:
            raise ValueError(f"sm is missing required key {key!r}")

    if not isinstance(sm["states"], list):
        raise ValueError("sm['states'] must be a list")
    if not isinstance(sm["current"], str):
        raise ValueError("sm['current'] must be a str")
    if not isinstance(sm["transitions"], dict):
        raise ValueError("sm['transitions'] must be a dict")
    if not isinstance(sm["history"], list):
        raise ValueError("sm['history'] must be a list")

    if sm["current"] not in sm["states"]:
        raise ValueError(
            f"sm['current']={sm['current']!r} is not in sm['states']="
            f"{sm['states']!r}"
        )

    for key, target in sm["transitions"].items():
        if not (
            isinstance(key, tuple)
            and len(key) == 2
            and all(isinstance(part, str) for part in key)
        ):
            raise ValueError(
                f"transition key {key!r} must be a (str, str) tuple"
            )
        if not isinstance(target, str):
            raise ValueError(
                f"transition target {target!r} must be a str"
            )
        if key[0] not in sm["states"]:
            raise ValueError(
                f"transition origin {key[0]!r} is not a known state"
            )
        if target not in sm["states"]:
            raise ValueError(
                f"transition target {target!r} is not a known state"
            )
