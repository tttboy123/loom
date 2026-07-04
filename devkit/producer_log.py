"""Per-run producer command log capture for the rdloop artifact stage.

When one of the ``artifact_producer_commands`` runs fails, ``capture()``
records the command + its tail (stdout / stderr) into a JSON diff blob the
rdloop evidence block can write alongside the artefact manifest.

Designed to be safe to call on absent files (returns empty fields) — the
rdloop path that invokes this only fires after a non-zero exit, but a stale
filesystem (e.g. quota preflight wiped the run_dir) should not crash the
loop.

API
---
``capture(command, *, cwd, log_dir, label="producer", tail_bytes=4096)``

Returns a dict with:

* ``command``        — the command line string for forensic echoing
* ``cwd``            — working directory it was attempted from
* ``log_dir``        — per-run directory the log files live in
* ``label``          — short tag, default ``"producer"``
* ``stdout_path``    — absolute path to the captured stdout file
* ``stderr_path``    — absolute path to the captured stderr file
* ``last_stdout_tail``  — at most ``tail_bytes`` from the *tail* of stdout
* ``last_stderr_tail``  — at most ``tail_bytes`` from the *tail* of stderr
* ``captured_at``    — ISO-8601 UTC timestamp

The module is intentionally dependency-free (only stdlib) so it never imposes
on rdloop's import graph.
"""
from __future__ import annotations

import datetime as _dt
import pathlib
from typing import Any

DEFAULT_TAIL_BYTES = 4096


def _now_iso() -> str:
    return _dt.datetime.now(_dt.timezone.utc).isoformat()


def _tail(path: pathlib.Path, limit: int) -> str:
    """Return at most ``limit`` bytes from the tail of ``path``.

    Treats missing / unreadable files as empty strings — callers should not
    distinguish "file does not exist" from "file is empty" in evidence.
    """
    if limit <= 0 or not path.exists():
        return ""
    try:
        size = path.stat().st_size
        if size <= limit:
            data = path.read_bytes()
        else:
            with path.open("rb") as fh:
                fh.seek(-limit, 2)
                data = fh.read()
    except OSError:
        return ""
    try:
        return data.decode("utf-8", errors="replace")
    except Exception:
        return ""


def _resolve(name: str, log_dir: str | pathlib.Path) -> pathlib.Path:
    """Return the abs Path for ``name`` relative to ``log_dir`` (or itself)."""
    p = pathlib.Path(name)
    if p.is_absolute():
        return p
    if log_dir:
        return pathlib.Path(log_dir) / name
    return p


def capture(
    command: str,
    *,
    cwd: str | pathlib.Path,
    log_dir: str | pathlib.Path,
    label: str = "producer",
    tail_bytes: int = DEFAULT_TAIL_BYTES,
) -> dict[str, Any]:
    """Capture stdout / stderr tails produced by a failed producer command.

    ``command`` is the shell command line that just failed. ``cwd`` is the
    directory it ran in (recorded verbatim so a reviewer can replicate). The
    stdout / stderr file basenames (e.g. ``artifact-producer-03.stdout.txt``)
    are resolved against ``log_dir`` to absolute paths.
    """
    log_dir_path = pathlib.Path(log_dir) if log_dir else pathlib.Path(".")
    stdout_path = _resolve(f"{label}.stdout.txt", log_dir_path)
    stderr_path = _resolve(f"{label}.stderr.txt", log_dir_path)
    return {
        "command": command,
        "cwd": str(cwd),
        "log_dir": str(log_dir_path),
        "label": label,
        "stdout_path": str(stdout_path),
        "stderr_path": str(stderr_path),
        "last_stdout_tail": _tail(stdout_path, tail_bytes),
        "last_stderr_tail": _tail(stderr_path, tail_bytes),
        "captured_at": _now_iso(),
    }
