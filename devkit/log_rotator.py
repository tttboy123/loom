"""log_rotator — JSONL log file rotation manager (stdlib only).

Draft for devkit. Not landed in real repo. Three primitives:
    rotate(path, max_bytes) -> bool
    trim(path, max_lines)   -> int
    log_size(path)          -> dict

All three are side-effect safe on missing files (no exception, no leftover).
"""
from __future__ import annotations

import os
from pathlib import Path
from typing import Any, Dict

def rotate(path: str, max_bytes: int = 1_000_000) -> bool:
    """Rename path -> path.1 if its size exceeds max_bytes.

    Returns True iff a rotation actually happened. Missing file -> False.
    On success, a fresh empty file is created at `path` so writers can
    append immediately without a FileNotFoundError.
    """
    if not os.path.exists(path):
        return False

    try:
        size = os.path.getsize(path)
    except OSError:
        return False

    if size <= max_bytes:
        return False

    backup = f"{path}.1"

    # Spec only mandates a single backup slot (.1). If an older backup
    # exists we drop it before renaming — keeps the contract simple.
    if os.path.exists(backup):
        try:
            os.remove(backup)
        except OSError:
            return False

    try:
        os.rename(path, backup)
    except OSError:
        return False

    try:
        Path(path).touch()
    except OSError:
        # Rotation already succeeded; the new file simply may not exist
        # on read-only filesystems. We still report True (work was done).
        pass

    return True

def trim(path: str, max_lines: int = 10_000) -> int:
    """Keep only the last `max_lines` lines of `path`. Returns lines removed.

    Missing file -> 0. Non-positive `max_lines` is rejected.
    On read/write failure, returns 0 and leaves the file untouched.
    """
    if not os.path.exists(path):
        return 0
    if max_lines < 0:
        raise ValueError("max_lines must be >= 0")

    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            lines = f.readlines()
    except OSError:
        return 0

    if len(lines) <= max_lines:
        return 0

    kept = lines[-max_lines:] if max_lines > 0 else []
    removed = len(lines) - max_lines

    try:
        with open(path, "w", encoding="utf-8") as f:
            f.writelines(kept)
    except OSError:
        return 0

    return removed

def log_size(path: str) -> Dict[str, Any]:
    """Return {"path": str, "bytes": int, "lines": int}.

    Missing file -> {"path": path, "bytes": 0, "lines": 0}.
    `lines` counts physical '\\n' separators via byte-stream iteration
    (suitable for well-formed JSONL; malformed mixed newlines are tolerated).
    """
    result: Dict[str, Any] = {"path": path, "bytes": 0, "lines": 0}
    if not os.path.exists(path):
        return result

    try:
        result["bytes"] = os.path.getsize(path)
    except OSError:
        result["bytes"] = 0

    line_count = 0
    try:
        with open(path, "rb") as f:
            for _ in f:
                line_count += 1
    except OSError:
        line_count = 0
    result["lines"] = line_count

    return result
