
from __future__ import annotations

from dataclasses import dataclass

from devkit import report_only_policy as _report_only

REPORT_ONLY_KEYWORDS = _report_only.REPORT_ONLY_KEYWORDS
REPORT_ONLY_THRESHOLD = _report_only.REPORT_ONLY_THRESHOLD
REPORT_ONLY_SCAN_WINDOW = _report_only.REPORT_ONLY_SCAN_WINDOW

def is_report_only(task_text: str) -> bool:
    """扫描 task_text 前 512 字符，命中关键词 >= 2 则判为 report-only。

    契约（用于 contract test）：
      - 输入 task_text 为 None/'' 时返回 False（保守走 materialize，避免误吞）
      - 命中数 >= REPORT_ONLY_THRESHOLD(=2) 返回 True
      - 否则 False
    """
    return _report_only.keyword_report_only(task_text)

# --- run.log 最小行结构（L1 / report-only） ---
@dataclass(frozen=True)
class RunLogFields:
    run_id: str
    mode: str          # 'report-only' | 'materialize'
    token: str         # '0' for report-only per spec
    carrier: str       # 'local-fallback' for report-only per spec
    reason: str = ""
    skipped: str = ""

def format_run_log_line(f: RunLogFields) -> str:
    parts = [
        f"[run] run_id={f.run_id}",
        f"[run] mode={f.mode}",
        f"[run] token={f.token}",
        f"[run] carrier={f.carrier}",
    ]
    if f.reason:
        parts.append(f"[run] reason=keyword-hit: {f.reason}")
    if f.skipped:
        parts.append(f"[run] skipped={f.skipped}")
    return "\n".join(parts) + "\n"


MaterializeGateResult = _report_only.MaterializeGateResult
materialize_gate = _report_only.materialize_gate
