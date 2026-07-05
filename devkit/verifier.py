from __future__ import annotations

import os
import py_compile
import subprocess
import sys
from typing import Callable, Iterable, Optional

PREFLIGHT_GATE_NAME = "IMPLEMENT_PREFLIGHT"


def _is_python_file(path: str) -> bool:
    return path.endswith(".py")


def _is_test_file(path: str) -> bool:
    name = os.path.basename(path)
    return _is_python_file(path) and (name.startswith("test_") or name.endswith("_test.py"))


def _syntax_error_result(path: str, exc: py_compile.PyCompileError) -> dict:
    exc_value = getattr(exc, "exc_value", None)
    line = getattr(exc, "lineno", None) or getattr(exc_value, "lineno", None)
    filename = getattr(exc, "filename", None) or getattr(exc_value, "filename", None) or path
    return {
        "ok": False,
        "gate": PREFLIGHT_GATE_NAME,
        "error": "SYNTAX_ERROR",
        "file": filename,
        "line": line,
        "message": str(exc),
    }


def _collect_error_result(path: str, proc: subprocess.CompletedProcess[str]) -> dict:
    return {
        "ok": False,
        "gate": PREFLIGHT_GATE_NAME,
        "error": "COLLECT_ERROR",
        "file": path,
        "line": None,
        "message": ((proc.stdout or "") + "\n" + (proc.stderr or "")).strip(),
    }


def _run_collect(path: str, timeout: int) -> Optional[dict]:
    proc = subprocess.run(
        [sys.executable, "-m", "pytest", "--collect-only", "-q", path],
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    if proc.returncode == 0:
        return None
    combined = ((proc.stdout or "") + "\n" + (proc.stderr or "")).strip()
    if "No module named pytest" not in combined:
        return _collect_error_result(path, proc)

    parent = os.path.dirname(path) or "."
    fallback = subprocess.run(
        [sys.executable, "-m", "unittest", "discover", "-s", parent, "-p", os.path.basename(path), "-t", parent],
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    fallback_output = ((fallback.stdout or "") + "\n" + (fallback.stderr or "")).strip()
    if fallback.returncode == 0 or "Ran 0 tests" in fallback_output or "NO TESTS RAN" in fallback_output:
        return None
    return _collect_error_result(path, fallback)


def _implement_preflight(new_files: Iterable[str], timeout: int = 120) -> dict:
    checked: list[str] = []
    test_files: list[str] = []

    for raw_path in new_files:
        path = os.fspath(raw_path)
        if not _is_python_file(path):
            continue
        checked.append(path)
        if _is_test_file(path):
            test_files.append(path)
        try:
            py_compile.compile(path, doraise=True)
        except py_compile.PyCompileError as exc:
            return _syntax_error_result(path, exc)

    for path in test_files:
        failed = _run_collect(path, timeout)
        if failed is not None:
            return failed

    return {"ok": True, "gate": PREFLIGHT_GATE_NAME, "checked": checked}


def run_verify(
    stage: str,
    new_files: Optional[Iterable[str]] = None,
    *,
    runner: Optional[Callable[..., dict]] = None,
    **kwargs,
) -> dict:
    if stage == "implement":
        gate = _implement_preflight(new_files or [])
        if not gate["ok"]:
            return gate
    if runner is None:
        return {"ok": True, "gate": PREFLIGHT_GATE_NAME, "stage": stage, "skipped": True}
    return runner(stage=stage, new_files=list(new_files or []), **kwargs)
