"""Monitor pipeline stage statuses. Stdlib only."""
from __future__ import annotations

_OK = frozenset({"ok", "done"})
_FAIL = frozenset({"failed"})
_BLOCK = frozenset({"blocked"})


def check_stage(stage: dict) -> dict:
    """Return {name, ok, issue} for a single stage."""
    status = stage.get("status", "")
    ok = status in _OK
    return {"name": stage.get("name", ""), "ok": ok, "issue": None if ok else status}


def supervise(stages: list[dict]) -> dict:
    """Aggregate {total, ok, failed, blocked, pending, healthy, summary}."""
    total = len(stages)
    ok = sum(1 for s in stages if s.get("status") in _OK)
    failed = sum(1 for s in stages if s.get("status") in _FAIL)
    blocked = sum(1 for s in stages if s.get("status") in _BLOCK)
    pending = total - ok - failed - blocked
    healthy = failed == 0 and blocked == 0
    return {
        "total": total,
        "ok": ok,
        "failed": failed,
        "blocked": blocked,
        "pending": pending,
        "healthy": healthy,
        "summary": f"{ok}/{total} ok",
    }


def pipeline_report(stages: list[dict]) -> str:
    """Return one line per stage: '[OK] name' or '[FAIL] name'."""
    if not stages:
        return "(no stages)"
    lines = []
    for s in stages:
        marker = "[OK]" if s.get("status") in _OK else "[FAIL]"
        lines.append(f"{marker} {s.get('name', '')}")
    return "\n".join(lines)
