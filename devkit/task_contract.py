"""任务契约：把自然语言任务的关键边界收成结构化约束。"""
from __future__ import annotations

from dataclasses import dataclass
import os

from devkit.delivery_mode import normalize_delivery_mode

_REPORT_ONLY_SIGNALS = (
    "report-only",
    "report only",
    "read-only",
    "readonly",
    "只读",
    "无写操作",
)
_DIAG_TASK_SIGNALS = (
    "诊断",
    "审计",
    "audit",
    "diagnose",
    "diagnostic",
    "report",
    "报告",
)
_ALLOW_REPORT_TEST_SIGNALS = (
    "允许生成验证测试文件",
    "允许生成测试文件",
    "allow verification tests",
    "allow generated tests",
)
_DEFAULT_REPORT_ONLY_FORBIDDEN_PREFIXES = (
    "tests/",
    "devkit/test",
    ".github/",
)
_SAFE_REPORT_TEST_PREFIXES = (
    "tests/test_diag",
)


@dataclass(frozen=True)
class TaskContract:
    delivery_mode: str
    task_kind: str
    allow_report_tests: bool
    allowed_artifact_paths: tuple[str, ...]
    forbidden_artifact_paths: tuple[str, ...]


def _normalize_prefixes(values) -> tuple[str, ...]:
    if not values:
        return ()
    out: list[str] = []
    for value in values:
        if not isinstance(value, str):
            continue
        norm = value.strip().replace("\\", "/").lstrip("./")
        if norm and norm not in out:
            out.append(norm)
    return tuple(out)


def infer_task_kind(task: str, delivery_mode: str = "autonomous", explicit: str | None = None) -> str:
    if explicit:
        return str(explicit).strip().lower()
    text = task or ""
    lowered = text.lower()
    if any(sig in text for sig in _DIAG_TASK_SIGNALS if not sig.isascii()):
        return "diag"
    if any(sig in lowered for sig in _DIAG_TASK_SIGNALS if sig.isascii()):
        return "diag"
    return "code"


def build_task_contract(
    task: str,
    *,
    delivery_mode: str = "autonomous",
    task_kind: str | None = None,
    allowed_artifact_paths=None,
    forbidden_artifact_paths=None,
) -> TaskContract:
    normalized_delivery = normalize_delivery_mode(delivery_mode)
    normalized_kind = infer_task_kind(task, normalized_delivery, task_kind)
    allowed = _normalize_prefixes(allowed_artifact_paths)
    forbidden = _normalize_prefixes(forbidden_artifact_paths)
    lowered = (task or "").lower()
    allow_report_tests = any(sig in (task or "") for sig in _ALLOW_REPORT_TEST_SIGNALS if not sig.isascii())
    allow_report_tests = allow_report_tests or any(sig in lowered for sig in _ALLOW_REPORT_TEST_SIGNALS if sig.isascii())
    allow_report_tests = allow_report_tests or any(
        prefix == "tests" or prefix.startswith("tests/") for prefix in allowed
    )

    if not forbidden and normalized_delivery == "report-only":
        if normalized_kind in {"diag", "report", "audit"}:
            forbidden = _DEFAULT_REPORT_ONLY_FORBIDDEN_PREFIXES

    return TaskContract(
        delivery_mode=normalized_delivery,
        task_kind=normalized_kind,
        allow_report_tests=allow_report_tests,
        allowed_artifact_paths=allowed,
        forbidden_artifact_paths=forbidden,
    )


def validate_materialized_paths(contract: TaskContract, files: list[str]) -> dict[str, object]:
    from devkit import applylock as _applylock

    normalized_files = [
        str(path or "").strip().replace("\\", "/").lstrip("./")
        for path in (files or [])
        if str(path or "").strip()
    ]
    blocked: list[str] = []
    for path in normalized_files:
        base = os.path.basename(path)
        is_test_file = base.startswith("test_") and base.endswith(".py")
        if path in _applylock.env_allowed_paths():
            continue
        if any(path == allow.rstrip("/") or path.startswith(allow) for allow in contract.allowed_artifact_paths):
            continue
        if contract.allow_report_tests and path.startswith("tests/"):
            continue
        if any(path.startswith(prefix) for prefix in _SAFE_REPORT_TEST_PREFIXES):
            continue
        if contract.delivery_mode == "report-only" and contract.task_kind in {"diag", "report", "audit"} and is_test_file:
            blocked.append(path)
            continue
        if any(path == deny.rstrip("/") or path.startswith(deny) for deny in contract.forbidden_artifact_paths):
            blocked.append(path)
    return {
        "ok": not blocked,
        "blocked": blocked,
        "reason": (
            None
            if not blocked
            else f"task-contract-forbidden-artifact-paths:{','.join(blocked)}"
        ),
    }
