# devkit/event_bus.py
"""
Pure-stdlib publish/subscribe event bus.
All mutators return a NEW bus; originals are never mutated.
publish() is a pure query: returns registered handler names + count,
does NOT invoke handlers (per spec).
"""

# ---------------------------------------------------------------------------
# Tests (TDD: written first; run before the impl below to see them go green).
# To run:  python devkit/event_bus.py
# ---------------------------------------------------------------------------

import copy

def _run_tests() -> None:
    import sys
    import traceback

    failures = 0
    # Golden cases (mapped 1:1 from the spec).
    cases = []

    # 1. create()['listeners'] == {}
    cases.append((
        "G1 create empty",
        lambda: create()["listeners"] == {},
    ))

    # 2. subscribe(create(),'run.done','h1')['listeners']['run.done'] == ['h1']
    cases.append((
        "G2 subscribe single",
        lambda: subscribe(create(), "run.done", "h1")["listeners"]["run.done"]
        == ["h1"],
    ))

    # 3. nested subscribe -> len == 2
    cases.append((
        "G3 subscribe twice -> len 2",
        lambda: len(
            subscribe(
                subscribe(create(), "e", "h1"), "e", "h2"
            )["listeners"]["e"]
        ) == 2,
    ))

    # 4. publish on empty bus -> count 0
    cases.append((
        "G4 publish empty -> count 0",
        lambda: publish(create(), "run.done", {"gate": "GO"})["count"] == 0,
    ))

    # 5. publish after one subscribe -> count 1
    cases.append((
        "G5 publish one handler -> count 1",
        lambda: publish(
            subscribe(create(), "run.done", "h1"), "run.done", {}
        )["count"] == 1,
    ))

    # 6. publish after one subscribe -> handlers == ['h1']
    cases.append((
        "G6 publish one handler -> handlers ['h1']",
        lambda: publish(
            subscribe(create(), "run.done", "h1"), "run.done", {}
        )["handlers"] == ["h1"],
    ))

    # 7. publish unknown event -> event echoed
    cases.append((
        "G7 publish unknown event echoes name",
        lambda: publish(create(), "other", {})["event"] == "other",
    ))

    # 8. unsubscribe removes handler; .get returns []
    cases.append((
        "G8 unsubscribe empties list",
        lambda: unsubscribe(
            subscribe(create(), "e", "h1"), "e", "h1"
        )["listeners"].get("e", []) == [],
    ))

    # 9. publish data is a dict (any value)
    cases.append((
        "G9 publish data is dict",
        lambda: isinstance(publish(create(), "x", {})["data"], dict),
    ))

    # 10. first element of subscribed list is handler name
    cases.append((
        "G10 first handler equals registered name",
        lambda: subscribe(create(), "e", "h1")["listeners"]["e"][0] == "h1",
    ))

    # --- Non-happy-path additions ---
    cases.append((
        "N1 subscribe does not mutate original bus",
        lambda: (
            (lambda b1, b2: b1["listeners"] != b2["listeners"]
                          and b1["listeners"] == {})
            (create(), subscribe(create(), "e", "h1"))
        ),
    ))
    cases.append((
        "N2 unsubscribe unknown handler is a no-op (no crash)",
        lambda: (
            lambda b: b["listeners"].get("e") == ["h1"]
            (unsubscribe(subscribe(create(), "e", "h1"), "e", "ghost"))
        ),
    ))
    cases.append((
        "N3 unsubscribe unknown event does not crash",
        lambda: (
            lambda b: b["listeners"] == {}
            (unsubscribe(create(), "never", "h1"))
        ),
    ))
    cases.append((
        "N4 publish carries the data dict through",
        lambda: publish(create(), "x", {"k": 1, "n": 2})["data"]
        == {"k": 1, "n": 2},
    ))
    cases.append((
        "N5 chaining subscribes preserves order",
        lambda: subscribe(
            subscribe(
                subscribe(create(), "e", "h1"), "e", "h2"
            ), "e", "h3"
        )["listeners"]["e"] == ["h1", "h2", "h3"],
    ))

    for name, fn in cases:
        try:
            ok = bool(fn())
        except Exception:  # noqa: BLE001
            ok = False
            traceback.print_exc()
        status = "PASS" if ok else "FAIL"
        if not ok:
            failures += 1
        print(f"[{status}] {name}")

    if failures:
        print(f"\n{failures} test(s) failed.")
        sys.exit(1)
    print(f"\nAll {len(cases)} tests passed.")

# ---------------------------------------------------------------------------
# Implementation (minimal, stdlib-only).
# ---------------------------------------------------------------------------

def create() -> dict:
    """Return a fresh, empty event bus."""
    return {"listeners": {}}

def subscribe(bus: dict, event: str, handler_name: str) -> dict:
    """Return a NEW bus with `handler_name` registered for `event`."""
    new_bus = copy.deepcopy(bus)
    handlers = new_bus["listeners"].get(event)
    if handlers is None:
        handlers = []
        new_bus["listeners"][event] = handlers
    handlers.append(handler_name)
    return new_bus

def publish(bus: dict, event: str, data: dict) -> dict:
    """Return {event, data, handlers, count}; does NOT invoke handlers."""
    handlers = bus["listeners"].get(event, [])
    # Return a copy of the handlers list so callers cannot mutate bus state
    # through the result.
    return {
        "event": event,
        "data": data,
        "handlers": list(handlers),
        "count": len(handlers),
    }

def unsubscribe(bus: dict, event: str, handler_name: str) -> dict:
    """Return a NEW bus with `handler_name` removed for `event`."""
    new_bus = copy.deepcopy(bus)
    handlers = new_bus["listeners"].get(event)
    if handlers is not None:
        # remove only the first occurrence; tolerate missing handler (idempotent)
        try:
            handlers.remove(handler_name)
        except ValueError:
            pass
    return new_bus

# ---------------------------------------------------------------------------
# Entrypoint: run tests when the module is executed directly.
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    _run_tests()
