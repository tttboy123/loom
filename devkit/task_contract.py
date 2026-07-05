"""任务契约：把自然语言任务的关键边界收成结构化约束。"""
from __future__ import annotations

from dataclasses import dataclass
import os
import re

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
_MANUAL_REVIEW_SIGNALS = (
    "人工复核",
    "人工审核",
    "human gate",
    "manual review",
    "manual-review",
)
_DEFAULT_REPORT_ONLY_FORBIDDEN_PREFIXES = (
    "tests/",
    "devkit/test",
    ".github/",
)
_SAFE_REPORT_TEST_PREFIXES = (
    "tests/test_diag",
)
_JSON_PATH_RE = re.compile(r"([A-Za-z0-9_\-./\\]+?\.json)\b")
_FIELD_RE = re.compile(r"(?:含|包含|字段)\s*[`\"']?([A-Za-z_][A-Za-z0-9_]*)[`\"']?")


@dataclass(frozen=True)
class TaskContract:
    delivery_mode: str
    task_kind: str
    allow_report_tests: bool
    allowed_artifact_paths: tuple[str, ...]
    forbidden_artifact_paths: tuple[str, ...]


@dataclass(frozen=True)
class GateSpec:
    mode: str
    artifact_path: str | None = None
    checks: tuple[dict, ...] = ()


def task_prefers_report_only(task: str, contract: TaskContract | None = None) -> bool:
    from devkit import report_only_policy as _report_only

    text = task or ""
    normalized_delivery = normalize_delivery_mode(getattr(contract, "delivery_mode", "") or "")
    return _report_only.task_prefers_report_only(
        text,
        normalized_delivery_mode=normalized_delivery,
        explicit_signals=_REPORT_ONLY_SIGNALS,
    )


def files_look_report_only(files: list[str] | None) -> bool:
    normalized_files = [
        str(path or "").strip().replace("\\", "/").lstrip("./")
        for path in (files or [])
        if str(path or "").strip()
    ]
    if not normalized_files:
        return False
    exts = {os.path.splitext(path)[1].lower() for path in normalized_files}
    if any(os.path.basename(path).startswith("test_") and path.endswith(".py") for path in normalized_files):
        return False
    if exts and exts.issubset({".md", ".markdown", ".txt", ".json", ".html", ".csv", ".yaml", ".yml"}):
        return True
    code_files = [path for path in normalized_files if os.path.splitext(path)[1].lower() in {".py", ".js", ".jsx", ".ts", ".tsx", ".go", ".rs", ".sh", ".bash"}]
    if not code_files:
        return False
    return all(path.split("/", 1)[0] == "runs" for path in code_files)


def task_prefers_manual_review(task: str) -> bool:
    text = task or ""
    lowered = text.lower()
    return (
        any(sig in text for sig in _MANUAL_REVIEW_SIGNALS if not sig.isascii())
        or any(sig in lowered for sig in _MANUAL_REVIEW_SIGNALS if sig.isascii())
    )


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


def build_gate_spec(task: str, contract: TaskContract, files: list[str] | None = None) -> GateSpec:
    normalized_files = [
        str(path or "").strip().replace("\\", "/").lstrip("./")
        for path in (files or [])
        if str(path or "").strip()
    ]
    text = str(task or "")
    lowered = text.lower()
    json_files = [path for path in normalized_files if path.lower().endswith(".json")]
    report_files = [path for path in normalized_files if os.path.splitext(path)[1].lower() in {".md", ".markdown", ".txt", ".html", ".csv", ".yaml", ".yml"}]
    json_path = _extract_json_path(text) or (json_files[0] if json_files else None)
    field_names = _extract_field_names(text)
    if task_prefers_manual_review(text):
        return GateSpec(mode="manual_review")
    if json_path and ("json" in lowered or "json.load" in lowered or "json 文件" in text):
        checks = [{"type": "exists"}, {"type": "min_size", "bytes": 10}, {"type": "json_load"}]
        checks.extend({"type": "field_present", "field": name} for name in field_names)
        return GateSpec(mode="artifact_json", artifact_path=json_path, checks=tuple(checks))
    if task_prefers_report_only(text, contract) and (report_files or json_files or files_look_report_only(normalized_files) or contract.task_kind in {"diag", "report", "audit"}):
        return GateSpec(mode="report_only")
    return GateSpec(mode="pytest")


def _extract_json_path(text: str) -> str | None:
    match = _JSON_PATH_RE.search(text or "")
    if not match:
        return None
    return match.group(1).replace("\\", "/").lstrip("./")


def _extract_field_names(text: str) -> tuple[str, ...]:
    seen: list[str] = []
    for match in _FIELD_RE.finditer(text or ""):
        field = match.group(1)
        if field.lower() in {"json", "file"}:
            continue
        if field not in seen:
            seen.append(field)
    return tuple(seen)
