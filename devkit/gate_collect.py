"""Helpers for verify-command gates when pytest collection is intentionally zero."""

from __future__ import annotations

import os
import re
from pathlib import Path

_PYTEST_CMD_RE = re.compile(r"(^|\s)(pytest|python(?:3)?\s+-m\s+pytest)\b", re.IGNORECASE)
_NON_TEST_PY_RE = re.compile(r"(^|/)(?!test_)[^/]+\.py$", re.IGNORECASE)
_ARTIFACT_PATH_RE = re.compile(r"([A-Za-z0-9_./-]+\.(?:txt|json|md|csv|yaml|yml|log))\b")
_NONEMPTY_HINTS = ("非空", "存在且非空", "non-empty", "nonempty", "size>0", "size > 0")


def is_pytest_command(command: str) -> bool:
    return bool(_PYTEST_CMD_RE.search(str(command or "").strip()))


def should_skip_pytest(verify_cmd: str, materialized_files: list[str]) -> bool:
    command = str(verify_cmd or "").strip()
    if not command or is_pytest_command(command):
        return False
    for raw_path in materialized_files or []:
        path = str(raw_path or "").strip().replace("\\", "/").lstrip("./")
        if not path.endswith(".py"):
            continue
        name = os.path.basename(path)
        if name.startswith("test_") or name.endswith("_test.py"):
            continue
        if _NON_TEST_PY_RE.search(path):
            return True
    return False


def extract_required_artifact_paths(task_text: str) -> list[str]:
    found: list[str] = []
    seen: set[str] = set()
    for line in str(task_text or "").splitlines():
        lowered = line.lower()
        if not any(hint in line or hint in lowered for hint in _NONEMPTY_HINTS):
            continue
        for match in _ARTIFACT_PATH_RE.finditer(line):
            path = match.group(1).replace("\\", "/").lstrip("./")
            if path not in seen:
                seen.add(path)
                found.append(path)
    return found


def resolve_workspace_artifact_path(workspace: Path, artifact_path: str) -> Path:
    raw = str(artifact_path or "").strip().replace("\\", "/").lstrip("./")
    parts = Path(raw).parts
    if parts and parts[0] == "build":
        parts = parts[1:]
    rel = Path(*parts) if parts else Path(raw)
    return Path(workspace) / rel
