"""Shared delivery mode semantics for CLI, backlog, and runtime."""
from __future__ import annotations

import pathlib

VALID_DELIVERY_MODES = frozenset({"autonomous", "report-only", "apply-required", "apply-git"})
DEFAULT_DELIVERY_MODE = "autonomous"


def normalize_delivery_mode(value: str | None, default: str = DEFAULT_DELIVERY_MODE) -> str:
    mode = str(value or "").strip().lower()
    if not mode:
        return default
    if mode in VALID_DELIVERY_MODES:
        return mode
    raise ValueError(f"invalid delivery_mode: {value!r}")


def resolve_delivery_mode(
    *,
    delivery_mode: str | None = None,
    apply_target: str | None = None,
    apply_git: str | None = None,
    default: str = DEFAULT_DELIVERY_MODE,
) -> str:
    if delivery_mode:
        return normalize_delivery_mode(delivery_mode, default=default)
    if isinstance(apply_git, str) and apply_git.strip():
        return "apply-git"
    if isinstance(apply_target, str) and apply_target.strip():
        return "apply-required"
    return default


def resolved_targets(
    *,
    mode: str,
    repo_root: pathlib.Path,
    apply_target: str | None = None,
    apply_git: str | None = None,
) -> tuple[str | None, str | None]:
    normalized_mode = normalize_delivery_mode(mode)
    target = apply_target.strip() if isinstance(apply_target, str) and apply_target.strip() else None
    git_repo = apply_git.strip() if isinstance(apply_git, str) and apply_git.strip() else None

    if normalized_mode in {"autonomous", "apply-required"} and not target:
        target = str(repo_root)
    if normalized_mode == "apply-git" and not git_repo and (repo_root / ".git").exists():
        git_repo = str(repo_root)
    return target, git_repo


def display_label(mode: str) -> str:
    normalized = normalize_delivery_mode(mode)
    if normalized == "report-only":
        return "L1 / report-only"
    if normalized == "autonomous":
        return "L2 / autonomous"
    if normalized == "apply-git":
        return "L2 / apply-git"
    return "L2 / apply-required"


def is_report_only(mode: str) -> bool:
    return normalize_delivery_mode(mode) == "report-only"
