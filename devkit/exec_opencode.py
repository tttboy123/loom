#!/usr/bin/env python3
"""OpenCode CLI executor adapter (stdlib only, no third-party deps).

Provides:
  - available() -> bool
  - run(task, cwd='.', timeout=300) -> dict {ok, output, exit_code}
  - version() -> str

All return values are type-stable so callers never have to guess:
  * available() always returns bool
  * run() always returns a dict with ok:bool, output:str, exit_code:int
  * version() always returns str (empty string when not installed / on error)
"""

from __future__ import annotations

import shutil
import subprocess

_CMD = "opencode"

def available() -> bool:
    """Return True iff the `opencode` command is found on PATH."""
    try:
        return shutil.which(_CMD) is not None
    except Exception:
        return False

def version() -> str:
    """Return the opencode version string.

    Returns "" when opencode is not installed, not on PATH, errors out,
    or times out. Never raises.
    """
    if not available():
        return ""
    try:
        proc = subprocess.run(
            [_CMD, "--version"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if proc.returncode == 0:
            out = (proc.stdout or "").strip()
            return out
        err = (proc.stderr or "").strip()
        return err
    except Exception:
        return ""

def run(task: str, cwd: str = ".", timeout: int = 300) -> dict:
    """Run an opencode task.

    Returns a dict with stable types:
      ok        : bool   -- True iff process exited 0
      output    : str    -- combined stdout+stderr (stripped)
      exit_code : int    -- process return code, or -1 on internal failure
    """
    if not available():
        return {
            "ok": False,
            "output": "opencode not available",
            "exit_code": -1,
        }

    try:
        proc = subprocess.run(
            [_CMD, task],
            cwd=cwd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        combined = (proc.stdout or "") + (proc.stderr or "")
        return {
            "ok": proc.returncode == 0,
            "output": combined.strip(),
            "exit_code": int(proc.returncode),
        }
    except subprocess.TimeoutExpired:
        return {
            "ok": False,
            "output": "timeout",
            "exit_code": -1,
        }
    except FileNotFoundError:
        return {
            "ok": False,
            "output": "opencode not available",
            "exit_code": -1,
        }
    except Exception as exc:
        return {
            "ok": False,
            "output": str(exc),
            "exit_code": -1,
        }

if __name__ == "__main__":
    print("available:", available())
    print("version:", repr(version()))
    print("run:", run("echo hello", cwd="/tmp", timeout=1))
