from __future__ import annotations

import json
import os
import pathlib
import time
from dataclasses import dataclass
from typing import Iterable


REPORT_ONLY_KEYWORDS: tuple[str, ...] = (
    "诊断",
    "grep",
    "只输出",
    "只读",
    "report-only",
    "不落源码",
)
REPORT_ONLY_THRESHOLD: int = 2
REPORT_ONLY_SCAN_WINDOW: int = 512

_TEXT_EVIDENCE_EXTS = {".md", ".markdown", ".txt", ".log", ".html", ".csv", ".yaml", ".yml"}


def _normalize_for_scan(text: str) -> str:
    return (text or "").lower()


def keyword_hits(text: str, keywords: Iterable[str] = REPORT_ONLY_KEYWORDS) -> list[str]:
    norm = _normalize_for_scan(text)
    head = norm[:REPORT_ONLY_SCAN_WINDOW]
    return [kw for kw in keywords if kw.lower() in head]


def keyword_report_only(text: str) -> bool:
    if not text:
        return False
    return len(keyword_hits(text)) >= REPORT_ONLY_THRESHOLD


def task_prefers_report_only(
    task: str,
    *,
    normalized_delivery_mode: str = "",
    explicit_signals: Iterable[str] = (),
) -> bool:
    text = task or ""
    lowered = text.lower()
    if normalized_delivery_mode == "report-only":
        return True
    for sig in explicit_signals:
        if not sig:
            continue
        if sig.isascii():
            if sig in lowered:
                return True
        elif sig in text:
            return True
    return keyword_report_only(text)


def report_only_task_type(task_kind: str, gate_mode: str) -> str:
    kind = str(task_kind or "").strip().lower()
    mode = str(gate_mode or "").strip().lower()
    if kind in {"diag", "audit", "observe"}:
        return kind
    if mode == "manual_review":
        return "audit"
    return "report-only"


def prepare_report_only_evidence(
    *,
    build_dir: pathlib.Path,
    task_id: str,
    files: list[str],
    impl_text: str,
) -> pathlib.Path:
    candidates: list[pathlib.Path] = []
    normalized_files = [
        str(path or "").strip().replace("\\", "/").lstrip("./")
        for path in (files or [])
        if str(path or "").strip()
    ]
    preferred = [
        f"runs/evidence/{task_id}.md",
        f"runs/diag-{task_id}.md",
        "run-log.md",
    ]
    for rel in preferred + normalized_files:
        path = build_dir / rel
        if path.is_file() and path not in candidates:
            candidates.append(path)
    source = candidates[0] if candidates else None
    target = build_dir / "runs" / "evidence" / f"{task_id}.md"
    target.parent.mkdir(parents=True, exist_ok=True)
    if source and source.resolve() == target.resolve():
        return target
    if source and source.suffix.lower() in _TEXT_EVIDENCE_EXTS:
        body = source.read_text(encoding="utf-8", errors="ignore").strip()
    elif source and source.suffix.lower() == ".json":
        body = "```json\n" + source.read_text(encoding="utf-8", errors="ignore").strip() + "\n```"
    elif source:
        body = source.read_text(encoding="utf-8", errors="ignore").strip()
    else:
        body = str(impl_text or "").strip()
    content = (
        f"---\n"
        f"task_id: {task_id}\n"
        f"source: {(str(source.relative_to(build_dir)) if source else 'implement-text')}\n"
        f"generated_at_epoch: {time.time():.3f}\n"
        f"---\n\n"
        f"{body}\n"
    )
    target.write_text(content, encoding="utf-8")
    return target


def bridge_gate_to_runtime_result(bridge_result, *, mode: str, success_output: str) -> tuple[object, dict]:
    from devkit import gates as _gates

    decision = _gates.Decision.GO if bridge_result.decision == "GO" else _gates.Decision.NO_GO
    failure_code = None
    if decision == _gates.Decision.NO_GO:
        if bridge_result.reason == "evidence_missing":
            failure_code = "REPORT_ONLY_EVIDENCE_MISSING"
        elif bridge_result.reason == "evidence_keywords_missing":
            failure_code = "REPORT_ONLY_EVIDENCE_KEYWORDS_MISSING"
        else:
            failure_code = "REPORT_ONLY_GATE_FAILED"
    details = dict(bridge_result.details or {})
    reason_text = bridge_result.reason or ("report-only evidence ok" if bridge_result.decision == "GO" else "report-only evidence gate failed")
    if details and decision == _gates.Decision.NO_GO:
        reason_text = reason_text + " | " + json.dumps(details, ensure_ascii=False, sort_keys=True)
    collect = {
        "ok": decision == _gates.Decision.GO,
        "runner": mode,
        "collected": 0,
        "output": success_output if decision == _gates.Decision.GO else reason_text,
        "failure_code": failure_code,
    }
    gate_result = _gates.GateResult(
        decision,
        mode,
        [reason_text] if reason_text else [],
        checked=list(details.get("checked") or []),
        missing=[] if decision == _gates.Decision.GO else [bridge_result.reason or "report_only_evidence"],
        failure_code=failure_code,
    )
    return gate_result, collect


@dataclass(frozen=True)
class MaterializeGateResult:
    allow: bool
    reason: str
    files: tuple[str, ...] = ()


def materialize_gate(
    *,
    build_dir: str | os.PathLike[str],
    task_type: str,
    stage_marker: str = "",
) -> MaterializeGateResult:
    import ast

    root = pathlib.Path(build_dir)
    if not root.is_dir():
        return MaterializeGateResult(False, "no-artifact")
    files = sorted(str(path.relative_to(root)).replace("\\", "/") for path in root.rglob("*") if path.is_file())
    if not files:
        return MaterializeGateResult(False, "no-artifact")

    nonempty = [path for path in files if (root / path).stat().st_size > 0]
    if not nonempty:
        return MaterializeGateResult(False, "empty-artifact", tuple(files))

    lowered_task_type = str(task_type or "").strip().lower()
    lowered_stage_marker = str(stage_marker or "").strip().lower()
    report_only_types = {
        "audit",
        "diag",
        "diagnose",
        "diagnostic",
        "manual-review",
        "observe",
        "report",
        "report-only",
        "verify-reverify",
    }
    if lowered_task_type in report_only_types or lowered_stage_marker in {"verify", "report-only"} and lowered_task_type in report_only_types:
        allowed = [
            path for path in nonempty
            if pathlib.Path(path).suffix.lower() in _TEXT_EVIDENCE_EXTS | {".json"}
        ]
        if not allowed:
            return MaterializeGateResult(False, "empty-artifact", tuple(files))
        return MaterializeGateResult(True, "report-only-skip-ast", tuple(allowed))

    parseable = []
    for rel in nonempty:
        path = root / rel
        if path.suffix.lower() != ".py":
            continue
        try:
            ast.parse(path.read_text(encoding="utf-8"))
        except SyntaxError:
            continue
        parseable.append(rel)
    if not parseable:
        return MaterializeGateResult(False, "no-python", tuple(files))
    return MaterializeGateResult(True, "ok", tuple(parseable))
