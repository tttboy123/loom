# devkit/snapshot_manager.py
"""Lightweight JSON snapshot persistence — pure stdlib.

Public API:
    save(path, data)          -> None
    load(path, default=None)  -> dict
    list_snapshots(directory) -> list[str]
    delete(path)              -> bool
"""
from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any, Dict, List

__all__ = ["save", "load", "list_snapshots", "delete"]

# Treat None as "no default supplied → use {}".
_MISSING: Any = object()

def save(path: str, data: Dict[str, Any]) -> None:
    """Serialize ``data`` as JSON to ``path``; create parent dirs if needed."""
    path_obj = Path(path)
    path_obj.parent.mkdir(parents=True, exist_ok=True)
    with path_obj.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2, sort_keys=True)
        f.write("\n")

def load(path: str, default: Dict[str, Any] = None) -> Dict[str, Any]:
    """Load JSON from ``path``; on missing/unreadable/corrupt return ``default`` (or ``{}``)."""
    if default is None:
        default = {}
    try:
        with open(path, "r", encoding="utf-8") as f:
            loaded = json.load(f)
    except (FileNotFoundError, IsADirectoryError, PermissionError, json.JSONDecodeError):
        return default
    # If the file contains valid JSON but isn't a dict (e.g. a list, scalar),
    # fall back to default — the contract says load() returns a dict.
    if not isinstance(loaded, dict):
        return default
    return loaded

def list_snapshots(directory: str) -> List[str]:
    """Return sorted ``.json`` basenames in ``directory``; ``[]`` if directory is missing."""
    if not os.path.isdir(directory):
        return []
    return sorted(
        entry.name
        for entry in os.scandir(directory)
        if entry.is_file() and entry.name.endswith(".json")
    )

def delete(path: str) -> bool:
    """Delete file at ``path``; ``True`` if removed, ``False`` if it did not exist."""
    try:
        os.remove(path)
        return True
    except FileNotFoundError:
        return False
