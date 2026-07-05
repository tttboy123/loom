"""Static guard for generated test imports.

Keep this module stdlib-only so sandbox tests can import it directly.
"""
from __future__ import annotations

from dataclasses import dataclass, field
import ast
import importlib
import pathlib
from typing import Iterable

TEST_IMPORT_ERROR = 1


@dataclass(frozen=True)
class MissingImport:
    module: str
    name: str | None = None
    file: pathlib.Path | None = None
    error: str = ""


@dataclass
class Report:
    ok: bool
    errors: list[str] = field(default_factory=list)
    exit_code: int = 0
    missing: list[MissingImport] = field(default_factory=list)
    parsed_files: int = 0


def expected_module_paths(module: str) -> list[str]:
    parts = [part for part in str(module or "").split(".") if part]
    if not parts:
        return []
    joined = "/".join(parts)
    return [f"{joined}.py", f"{joined}/__init__.py"]


def parse_imports(source: str) -> list[tuple[str, str | None]]:
    tree = ast.parse(source or "")
    refs: list[tuple[str, str | None]] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name:
                    refs.append((alias.name, None))
        elif isinstance(node, ast.ImportFrom):
            if node.level and node.level > 0:
                continue
            if not node.module:
                continue
            for alias in node.names:
                if alias.name == "*":
                    continue
                refs.append((node.module, alias.name))
    return refs


def _missing(module: str, name: str | None, *, file: pathlib.Path | None = None, error: str = "") -> MissingImport:
    return MissingImport(module=module, name=name, file=file, error=error)


def _report_from_missing(missing: list[MissingImport], parsed_files: int = 0) -> Report:
    errors: list[str] = []
    for item in missing:
        if item.name:
            msg = f"{item.module}.{item.name}"
        else:
            msg = item.module
        if item.error:
            msg += f": {item.error}"
        if item.file:
            msg = f"{item.file.name}: {msg}"
        errors.append(msg)
    return Report(
        ok=not missing,
        errors=errors,
        exit_code=0 if not missing else TEST_IMPORT_ERROR,
        missing=missing,
        parsed_files=parsed_files,
    )


def resolve_imports(refs: Iterable[tuple[str, str | None]]) -> list[MissingImport]:
    missing: list[MissingImport] = []
    for module, name in refs:
        try:
            imported = importlib.import_module(module)
        except Exception as exc:  # noqa: BLE001
            missing.append(_missing(module, name, error=f"{type(exc).__name__}: {exc}"))
            continue
        if name and not hasattr(imported, name):
            hint = ""
            upper_name = name.upper()
            if hasattr(imported, upper_name):
                hint = f"symbol missing; maybe {upper_name}"
            missing.append(_missing(module, name, error=hint or "symbol missing"))
    return missing


def check_imports(refs: Iterable[tuple[str, str | None]]) -> Report:
    return _report_from_missing(resolve_imports(refs))


def check_directory(root: str | pathlib.Path) -> Report:
    base = pathlib.Path(root)
    tests = sorted(base.rglob("test_*.py")) + sorted(base.rglob("*_test.py"))
    seen: set[pathlib.Path] = set()
    missing: list[MissingImport] = []
    parsed_files = 0
    for path in tests:
        if path in seen:
            continue
        seen.add(path)
        try:
            source = path.read_text(encoding="utf-8")
            tree = ast.parse(source, filename=str(path))
        except Exception as exc:  # noqa: BLE001
            missing.append(_missing(str(path), None, file=path, error=f"{type(exc).__name__}: {exc}"))
            continue
        parsed_files += 1
        refs: list[tuple[str, str | None]] = []
        for node in ast.walk(tree):
            if isinstance(node, ast.ImportFrom) and node.level and node.level > 0:
                missing.append(_missing(".", None, file=path, error="relative import unsupported"))
                continue
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name:
                        refs.append((alias.name, None))
            elif isinstance(node, ast.ImportFrom):
                if not node.module:
                    continue
                for alias in node.names:
                    if alias.name == "*":
                        continue
                    refs.append((node.module, alias.name))
        for item in resolve_imports(refs):
            missing.append(MissingImport(item.module, item.name, path, item.error))
    return _report_from_missing(missing, parsed_files=parsed_files)
