# devkit/pipeline_trace.py
"""Pipeline trace recording & replay. Pure stdlib (time only).

API:
    new_trace(task_id) -> dict
    add_step(trace, stage, carrier, ok, tokens=0) -> dict
    trace_summary(trace) -> dict
    format_trace(trace) -> str
"""
import time

def new_trace(task_id: str) -> dict:
    """Create a fresh, empty trace object for the given task."""
    return {
        "task_id": task_id,
        "steps": [],
        "started_at": time.time(),
    }

def add_step(trace: dict, stage: str, carrier: str, ok: bool, tokens: int = 0) -> dict:
    """Append a step to trace['steps'] (in place) and return the trace.

    A step is: {stage, carrier, ok (bool), tokens (int), ts (float)}.
    """
    trace["steps"].append({
        "stage": stage,
        "carrier": carrier,
        "ok": bool(ok),
        "tokens": int(tokens),
        "ts": time.time(),
    })
    return trace

def trace_summary(trace: dict) -> dict:
    """Aggregate stats. carriers is a deduped list preserving first-occurrence order.

    Safe on a malformed/empty input (treats missing keys as empty defaults).
    """
    steps = (trace or {}).get("steps") or []
    carriers_seen, seen = [], set()
    ok_steps = failed_steps = total_tokens = 0
    for s in steps:
        c = s.get("carrier")
        if c not in seen:
            seen.add(c)
            carriers_seen.append(c)
        if s.get("ok"):
            ok_steps += 1
        else:
            failed_steps += 1
        total_tokens += s.get("tokens", 0) or 0
    return {
        "task_id": (trace or {}).get("task_id", ""),
        "total_steps": len(steps),
        "ok_steps": ok_steps,
        "failed_steps": failed_steps,
        "total_tokens": total_tokens,
        "carriers": carriers_seen,
    }

def format_trace(trace: dict) -> str:
    """Render steps as '  [OK/FAIL] stage via carrier (N tok)' lines.

    Empty / malformed input -> '(empty trace)'.
    """
    steps = (trace or {}).get("steps") or []
    if not steps:
        return "(empty trace)"
    lines = []
    for s in steps:
        status = "OK" if s.get("ok") else "FAIL"
        lines.append(
            f"  [{status}] {s.get('stage', '?')} via "
            f"{s.get('carrier', '?')} ({s.get('tokens', 0)} tok)"
        )
    return "\n".join(lines)
