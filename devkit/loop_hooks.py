# devkit/loop_hooks.py
"""Autoloop lifecycle hook system (pure stdlib).

Tiny event-bus primitive used by the autoloop:
  * register(event, fn)   — append callback (multiple per event allowed).
  * fire(event, payload)  — invoke callbacks in FIFO order, swallow per-callback
                            exceptions, return list of return values.
  * clear(event='')       — drop hooks for one event, or all if event==''.

State is module-level (intentional: a single bus per process). Tests should
call clear() in setUp to isolate.
"""

# event name -> list[callable], FIFO
_hooks: dict = {}

def register(event: str, fn) -> None:
    """Register ``fn`` as a callback for ``event`` (appended in order)."""
    _hooks.setdefault(event, []).append(fn)

def fire(event: str, payload: dict = None) -> list:
    """Fire ``event``; call each registered fn with ``payload``; collect returns.

    Unknown events return ``[]``. Per-callback exceptions are silently
    skipped — the spec explicitly requires fire() to never raise because
    of a single bad hook.
    """
    if payload is None:
        payload = {}
    results: list = []
    # snapshot the list so callbacks that re-register don't mutate mid-iteration
    for fn in list(_hooks.get(event, ())):
        try:
            results.append(fn(payload))
        except Exception:
            # Per spec: ignore exceptions.
            continue
    return results

def clear(event: str = "") -> None:
    """Clear hooks for ``event``; clear ALL hooks when ``event == ''``."""
    if event == "":
        _hooks.clear()
    else:
        _hooks.pop(event, None)
