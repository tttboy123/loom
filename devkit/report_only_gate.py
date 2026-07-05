# devkit/report_only_gate.py
"""Report-only gate: decides GO/NO-GO based on evidence for diag/observe/audit tasks."""
from __future__ import annotations
import os
from typing import Dict, Any

_KEYWORDS = ("验收", "verified", "observed", "audit")
_SIZE = 32  # minimum non-empty evidence proxy (bytes of non-whitespace)

def _read_text(path: str) -> str:
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        return f.read()

def evaluate(task_type: str, evidence_path: str) -> Dict[str, Any]:
    # Rule 3: only diag/observe/audit allowed under report-only mode
    if task_type not in ("diag", "observe", "audit"):
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": (
                f"report-only 仅限 diag/observe/audit；got "
                f"{task_type!r}. Use L2 autonomous mode for non-report-only tasks."
            ),
        }

    # Rule 2: missing path
    if not evidence_path or not os.path.exists(evidence_path):
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": (
                f"evidence path missing for task_type={task_type!r}: "
                f"{evidence_path!r}"
            ),
        }

    try:
        text = _read_text(evidence_path)
    except Exception as e:
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": f"evidence unreadable ({e!r}) at {evidence_path!r}",
        }

    # Empty / whitespace-only file
    if not text or not text.strip():
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": (
                f"evidence file is empty for task_type={task_type!r}: "
                f"{evidence_path!r}"
            ),
        }

    if len(text.strip()) < _SIZE:
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": (
                f"evidence too small (<{_SIZE}B) for task_type={task_type!r}"
            ),
        }

    # Rule 1: keyword must appear
    lowered = text.lower()
    if not any(kw.lower() in lowered for kw in _KEYWORDS):
        return {
            "ok": False,
            "verdict": "NO-GO",
            "reason": (
                f"evidence lacks acceptance keyword "
                f"{[k for k in _KEYWORDS]} at {evidence_path!r}"
            ),
        }

    return {
        "ok": True,
        "verdict": "GO",
        "reason": (
            f"evidence accepted for task_type={task_type!r} at {evidence_path!r}"
        ),
    }
