# -*- coding: utf-8 -*-
"""Build / test gate with `inspect_mode` 分支。

约束：纯标准库，无新依赖。

判定逻辑：
  1) `artifact_json` 显式走 inspect_mode
  2) `report_only` / `manual_review` 走显式 gate
  3) 其余走 pytest 路径
"""
from __future__ import annotations

import json
import re
import subprocess
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import List, Sequence

class Decision(str, Enum):
    GO = "GO"
    NO_GO = "NO-GO"

# 字段点名：从 acceptance 行中提取 "含 <field>" 或 "含 <field> 数组/对象/字段"
_FIELD_PATTERNS = [
    re.compile(r"含\s*[`\"\']?([A-Za-z_][A-Za-z0-9_]*)[`\"\']?\s*(?:数组|对象|字段|列表)?"),
    re.compile(r"包含\s*[`\"\']?([A-Za-z_][A-Za-z0-9_]*)[`\"\']?"),
    re.compile(r"字段\s*[`\"\']?([A-Za-z_][A-Za-z0-9_]*)[`\"\']?"),
]

# 路径提取：从 acceptance 行里识别 "xxx.json" / "path/to/x.json"
_PATH_PATTERN = re.compile(
    r"([A-Za-z0-9_\-./\\]+?\.(?:json|JSON))\b"
)

# "文件存在且非空" 的显式条件
_EXISTS_NONEMPTY = re.compile(r"文件存在(且非空)?")

def _extract_path_from_acceptance(acceptance: Sequence[str], workspace: Path) -> Path | None:
    """取 acceptance[0] 里第一个出现的文件路径；若相对，相对于 workspace。"""
    if not acceptance:
        return None
    first = acceptance[0]
    m = _PATH_PATTERN.search(first)
    if not m:
        return None
    p = Path(m.group(1))
    if not p.is_absolute():
        p = workspace / p
    return p

def _extract_required_fields(acceptance: Sequence[str]) -> List[str]:
    """从 acceptance 全集中抽取被点名的字段名（去重，保序）。"""
    seen: List[str] = []
    for line in acceptance:
        for pat in _FIELD_PATTERNS:
            for m in pat.finditer(line):
                name = m.group(1)
                # 过滤掉明显非字段名的命中（如 "JSON"）
                if name.lower() in {"json", "file"}:
                    continue
                if name not in seen:
                    seen.append(name)
    return seen

# ---------- 结果类型 ----------
@dataclass
class GateResult:
    decision: Decision
    mode: str                       # "inspect" | "pytest"
    reasons: List[str] = field(default_factory=list)
    checked: List[str] = field(default_factory=list)
    missing: List[str] = field(default_factory=list)
    failure_code: str | None = None

    def is_go(self) -> bool:
        return self.decision == Decision.GO


@dataclass(frozen=True)
class GateSpec:
    mode: str
    artifact_path: str | None = None
    checks: tuple[dict, ...] = ()


def run_command_artifact_gate(
    *,
    task_text: str,
    workspace: Path,
    verify_commands: Sequence[str],
) -> GateResult:
    from devkit import gate_collect as _gate_collect

    commands = [str(cmd or "").strip() for cmd in verify_commands if str(cmd or "").strip()]
    if not commands:
        return GateResult(
            Decision.NO_GO,
            "verify_command",
            ["verify_command gate missing executable command"],
            missing=["verify_command"],
            failure_code="VERIFY_COMMAND_MISSING",
        )

    output_chunks: list[str] = []
    for command in commands:
        proc = subprocess.run(
            command,
            cwd=str(workspace),
            shell=True,
            executable="/bin/bash",
            capture_output=True,
            text=True,
        )
        combined = ((proc.stdout or "") + (proc.stderr or "")).strip()
        if combined:
            output_chunks.append(f"$ {command}\n{combined}")
        else:
            output_chunks.append(f"$ {command}")
        if proc.returncode != 0:
            return GateResult(
                Decision.NO_GO,
                "verify_command",
                [f"verify command failed rc={proc.returncode}: {command}", combined[-1000:] if combined else ""],
                checked=["verify_command"],
                missing=["verify_command_exit_zero"],
                failure_code="VERIFY_COMMAND_FAILED",
            )

    required = _gate_collect.extract_required_artifact_paths(task_text)
    missing: list[str] = []
    checked = ["verify_command"]
    for rel in required:
        checked.append(f"artifact:{rel}")
        target = _gate_collect.resolve_workspace_artifact_path(workspace, rel)
        if not target.is_file() or target.stat().st_size <= 0:
            missing.append(rel)
    if missing:
        return GateResult(
            Decision.NO_GO,
            "verify_command",
            ["verify command finished but required artifacts are missing or empty: " + ", ".join(missing)] + output_chunks[-2:],
            checked=checked,
            missing=[f"artifact:{item}" for item in missing],
            failure_code="VERIFY_ARTIFACT_MISSING",
        )

    reason = "verify command passed"
    if required:
        reason += f"; artifacts ok: {', '.join(required)}"
    return GateResult(
        Decision.GO,
        "verify_command",
        [reason] + output_chunks[-2:],
        checked=checked,
    )


def _infer_report_only_task_type(task_text: str, gate_mode: str) -> str:
    from devkit import task_contract as _task_contract

    kind = _task_contract.infer_task_kind(task_text or "")
    mode = str(gate_mode or "").strip().lower()
    if kind in {"diag", "audit", "observe"}:
        return kind
    if mode == "manual_review":
        return "audit"
    if _task_contract.task_prefers_report_only(task_text or ""):
        return "report-only"
    return "impl"


def _resolve_report_only_evidence_path(gate_spec: GateSpec, workspace: Path) -> Path | None:
    candidates: list[Path] = []
    if gate_spec.artifact_path:
        explicit = Path(gate_spec.artifact_path)
        if not explicit.is_absolute():
            explicit = workspace / explicit
        candidates.append(explicit)
    candidates.append(workspace / "run-log.md")
    for path in candidates:
        if path.is_file() and path.stat().st_size > 0:
            return path
    return candidates[0] if candidates else None


def _evaluate_report_only_gate_spec(
    gate_spec: GateSpec,
    *,
    task_text: str,
    workspace: Path,
    mode: str,
) -> GateResult:
    task_type = _infer_report_only_task_type(task_text, mode)
    if task_type not in {"report-only", "diag", "observe", "audit"}:
        return GateResult(
            Decision.NO_GO,
            mode,
            [f"{mode} gate only supports report/diag/audit tasks; inferred task_type={task_type}"],
            checked=[mode],
            missing=["report_only_task_type"],
            failure_code="REPORT_ONLY_UNSUPPORTED_TASK_TYPE",
        )

    evidence_path = _resolve_report_only_evidence_path(gate_spec, workspace)
    if evidence_path is None or not evidence_path.is_file() or evidence_path.stat().st_size <= 0:
        missing = str(evidence_path) if evidence_path is not None else "(missing)"
        return GateResult(
            Decision.NO_GO,
            mode,
            [f"{mode} evidence missing or empty: {missing}"],
            checked=[mode],
            missing=["report_only_evidence"],
            failure_code="REPORT_ONLY_EVIDENCE_MISSING",
        )

    return GateResult(
        Decision.GO,
        mode,
        [f"{mode} evidence present: {evidence_path}"],
        checked=[mode, "report_only_evidence"],
    )

# ---------- inspect_mode 核心 ----------
def inspect_mode(task_text: str, acceptance: Sequence[str], workspace: Path) -> GateResult:
    """按 acceptance 第 1 条指定的文件做 4 项检查（任务描述 #1.1~#1.4）。"""
    reasons: List[str] = []

    # 1) 定位 artifact
    target = _extract_path_from_acceptance(acceptance, workspace)
    if target is None:
        return GateResult(
            Decision.NO_GO, "inspect",
            ["inspect_mode: acceptance 第 1 条未指定 JSON 文件路径"],
            missing=["artifact_path"],
            failure_code="ARTIFACT_JSON_GATE_FAILED",
        )

    # 1.1 文件存在且非空（>10 bytes）
    if not target.exists():
        reasons.append(
            f"未满足 acceptance[0]: artifact 缺失 {target}（文件不存在）"
        )
        return GateResult(Decision.NO_GO, "inspect", reasons, checked=["exists"], missing=["exists"], failure_code="ARTIFACT_JSON_GATE_FAILED")

    size = target.stat().st_size
    if size <= 10:
        reasons.append(
            f"未满足 acceptance[0]: {target} 存在但为空或过小(size={size} bytes, 需 >10)"
        )
        return GateResult(Decision.NO_GO, "inspect", reasons, checked=["exists", "min_size"], missing=["min_size"], failure_code="ARTIFACT_JSON_GATE_FAILED")

    # 1.2 json.load 不抛异常
    try:
        with target.open("r", encoding="utf-8") as f:
            parsed = json.load(f)
    except (json.JSONDecodeError, UnicodeDecodeError, OSError) as e:
        reasons.append(
            f"未满足 acceptance: {target} JSON 损坏 — json.load 失败: {e!s}"
        )
        return GateResult(Decision.NO_GO, "inspect", reasons, checked=["exists", "min_size", "json_load"], missing=["json_load"], failure_code="ARTIFACT_JSON_GATE_FAILED")

    # 1.3 acceptance 中点名的字段必须出现
    required = _extract_required_fields(acceptance)
    missing = [
        f for f in required
        if not _field_present(parsed, f)
    ]
    if missing:
        reasons.append(
            "未满足 acceptance 字段点名: 缺少 " + ", ".join(missing)
        )
        return GateResult(
            Decision.NO_GO,
            "inspect",
            reasons,
            checked=["exists", "min_size", "json_load"],
            missing=[f"field:{name}" for name in missing],
            failure_code="ARTIFACT_JSON_GATE_FAILED",
        )

    # 全部通过
    reasons.append(
        f"inspect_mode 全部通过: {target} size={size}B, 字段 {required or '[]'} 已就位"
    )
    checked = ["exists", "min_size", "json_load"] + [f"field:{name}" for name in required]
    return GateResult(Decision.GO, "inspect", reasons, checked=checked)

def _field_present(obj, name: str) -> bool:
    """字段存在性：dict 直接命中；list 检查非空且元素为 dict 时第一项含该键。"""
    if isinstance(obj, dict):
        return name in obj
    if isinstance(obj, list):
        if not obj:
            return False
        first = obj[0]
        if isinstance(first, dict):
            return name in first
        return False
    return False

# ---------- 原 pytest 路径（不改既有语义） ----------
def run_gate(
    *,
    gate_spec: GateSpec | None = None,
    task_text: str,
    acceptance: Sequence[str],
    workspace: Path,
    pytest_collected: int,
    pytest_passed: int = 0,
    pytest_failed: int = 0,
) -> GateResult:
    """Gate 入口。显式 GateSpec 优先；否则走 pytest 路径。

    pytest 路径的 GO 条件：collected > 0 且 failed == 0 且 passed == collected。
    collect=0 在 pytest 路径下视为 NO-GO（保留原行为）。
    """
    if gate_spec is not None:
        return evaluate_gate_spec(gate_spec, task_text=task_text, acceptance=acceptance, workspace=workspace,
                                  pytest_collected=pytest_collected, pytest_passed=pytest_passed, pytest_failed=pytest_failed)

    reasons: List[str] = []
    if pytest_collected <= 0:
        reasons.append("pytest 收集到 0 条用例（collect=0 视为失败）")
        return GateResult(Decision.NO_GO, "pytest", reasons, checked=["pytest_collect"], missing=["pytest_collect"], failure_code="TEST_COLLECT_NONE")
    if pytest_failed > 0:
        reasons.append(f"pytest 有 {pytest_failed} 条失败")
        return GateResult(Decision.NO_GO, "pytest", reasons, checked=["pytest_collect", "pytest_pass"], missing=["pytest_pass"], failure_code="TESTS_FAILED")
    if pytest_passed != pytest_collected:
        reasons.append(
            f"pytest 通过 {pytest_passed}/{pytest_collected} 不匹配"
        )
        return GateResult(Decision.NO_GO, "pytest", reasons, checked=["pytest_collect", "pytest_pass"], missing=["pytest_pass"], failure_code="TESTS_FAILED")

    reasons.append(f"pytest 全绿 {pytest_passed}/{pytest_collected}")
    return GateResult(Decision.GO, "pytest", reasons, checked=["pytest_collect", "pytest_pass"])


def evaluate_gate_spec(
    gate_spec: GateSpec,
    *,
    task_text: str,
    acceptance: Sequence[str],
    workspace: Path,
    pytest_collected: int,
    pytest_passed: int = 0,
    pytest_failed: int = 0,
) -> GateResult:
    if gate_spec.mode == "artifact_json":
        artifact_rel = gate_spec.artifact_path or ""
        checks = list(gate_spec.checks or ())
        art_acceptance = [artifact_rel] + [f"字段 `{c['field']}`" for c in checks if c.get("type") == "field_present"]
        result = inspect_mode(task_text or artifact_rel, art_acceptance, workspace)
        return GateResult(
            result.decision,
            "artifact_json",
            list(result.reasons),
            checked=list(result.checked),
            missing=list(result.missing),
            failure_code=result.failure_code,
        )
    if gate_spec.mode == "report_only":
        return _evaluate_report_only_gate_spec(
            gate_spec,
            task_text=task_text,
            workspace=workspace,
            mode="report_only",
        )
    if gate_spec.mode == "manual_review":
        return _evaluate_report_only_gate_spec(
            gate_spec,
            task_text=task_text,
            workspace=workspace,
            mode="manual_review",
        )
    if gate_spec.mode == "verify_command":
        raise ValueError("verify_command gate must be executed via run_command_artifact_gate()")
    return run_gate(task_text=task_text, acceptance=acceptance, workspace=workspace,
                    pytest_collected=pytest_collected, pytest_passed=pytest_passed, pytest_failed=pytest_failed)

GateVerdict = GateResult

__all__ = [
    "Decision",
    "GateResult",
    "GateVerdict",
    "GateSpec",
    "evaluate_gate_spec",
    "inspect_mode",
    "run_command_artifact_gate",
    "run_gate",
]
