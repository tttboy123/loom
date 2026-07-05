"""Harness gate：决定 task candidate 是否 GO。

- task_type='report-only' + candidate_state='materialized'：
    不再要求 pytest 收集到测试；要求 evidence 文件存在且非空，
    且 frontmatter/正文中包含任务验收点对应的关键字。
- task_type='impl'（默认）：维持原 pytest 路径。
"""
from __future__ import annotations

import os
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Sequence

REPORT_ONLY_TASK_TYPES = {"report-only", "diag", "observe", "audit"}

@dataclass
class GateResult:
    decision: str   # 'GO' | 'NO-GO'
    reason: str = ""
    details: dict | None = None

def _candidate_evidence_paths(runs_dir: Path, task_id: str) -> list[Path]:
    return [
        runs_dir / "evidence" / f"{task_id}.md",
        runs_dir / f"diag-{task_id}.md",
    ]

_KEYWORD_RE_CACHE: dict[str, re.Pattern[str]] = {}

def _keyword_pattern(keywords: Iterable[str]) -> re.Pattern[str]:
    keys = tuple(k.lower() for k in keywords if k)
    pat = _KEYWORD_RE_CACHE.get(keys)
    if pat is None:
        if not keys:
            pat = re.compile(r".+", re.IGNORECASE | re.DOTALL)
        else:
            pat = re.compile(r"|".join(re.escape(k) for k in keys), re.IGNORECASE)
        _KEYWORD_RE_CACHE[keys] = pat
    return pat

def _evidence_has_keywords(path: Path, keywords: Sequence[str]) -> bool:
    text = path.read_text(encoding="utf-8", errors="ignore")
    # 取 frontmatter + 正文统一检索
    fm_match = re.match(r"^---\n(.*?)\n---\n?", text, flags=re.DOTALL)
    haystack = (fm_match.group(1) if fm_match else "") + "\n" + text
    return bool(_keyword_pattern(keywords).search(haystack))

def _is_report_only(task: dict) -> bool:
    tt = (task.get("task_type") or "impl").lower()
    return tt in REPORT_ONLY_TASK_TYPES

def evaluate_gate(task: dict, *, runs_dir: str | os.PathLike = "runs") -> GateResult:
    """判定 gate。

    Args:
        task: dict with keys
            - task_id: str
            - task_type: 'impl' (default) | 'report-only' | 'diag' | 'observe' | 'audit'
            - candidate_state: e.g. 'materialized'
            - acceptance_keywords: Sequence[str]（验收点关键字）
        runs_dir: 运行时产物根目录，默认 'runs'。

    Returns:
        GateResult(decision, reason, details)
    """
    runs_root = Path(runs_dir)
    task_id = task["task_id"]

    # --- 分支 1：report-only / diag / observe / audit ---
    if _is_report_only(task) and task.get("candidate_state") == "materialized":
        keywords = task.get("acceptance_keywords") or []
        candidates = _candidate_evidence_paths(runs_root, task_id)

        for path in candidates:
            if path.exists() and path.stat().st_size > 0:
                if _evidence_has_keywords(path, keywords):
                    return GateResult("GO", details={"evidence": str(path)})
                return GateResult(
                    "NO-GO",
                    reason="evidence_keywords_missing",
                    details={"evidence": str(path), "keywords": list(keywords)},
                )
        return GateResult(
            "NO-GO",
            reason="evidence_missing",
            details={"checked": [str(p) for p in candidates]},
        )

    # --- 分支 2：test-impl（原 pytest 路径） ---
    # 仅契约：实际实现由现有 pytest 收集器 + junit 解析给出
    outcome = os.environ.get("DEVKIT_PYTEST_OUTCOME", "failed")
    if outcome == "passed":
        return GateResult("GO", details={"path": "pytest"})
    return GateResult("NO-GO", reason="tests_failed", details={"path": "pytest"})
