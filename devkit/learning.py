"""devkit/learning.py — read-only learning sidecar for devkit runs.

Parses stage markdown files to extract events, then generates
suggestions for human Accept/Reject. Default read-only, stdlib only.
"""
from __future__ import annotations

import re
from pathlib import Path
from typing import Union

PathLike = Union[str, Path]

_GATE_RE = re.compile(r"(?im)\bGate\s*[:=]\s*(GO|NO[\-_ ]?GO)\b")
_GATE_BARE_RE = re.compile(r"(?im)\b(NO[\-_ ]?GO|GO)\b")
_CARRIER_RE = re.compile(r"(?im)\bCarrier\s*[:=]\s*([A-Za-z][\w\-]*)")
_MODEL_RE = re.compile(r"(?im)\bModel\s*[:=]\s*([A-Za-z][\w\-]*)")
_TOKENS_RE = re.compile(r"(?im)\bTokens?\s*[:=]\s*(\d+)")


def _norm_gate(raw: str) -> str:
    s = raw.strip().upper().replace("_", "-").replace(" ", "-")
    return "NO-GO" if s.startswith("NO") else "GO"


def _parse_md(content: str, file_path: Path) -> dict:
    stage = file_path.stem.lower()
    gate = ""
    m = _GATE_RE.search(content)
    if m:
        gate = _norm_gate(m.group(1))
    elif (m := _GATE_BARE_RE.search(content)):
        gate = _norm_gate(m.group(1))

    carrier = ""
    m = _CARRIER_RE.search(content)
    if m:
        carrier = m.group(1).strip().lower()
    elif (m := _MODEL_RE.search(content)):
        carrier = m.group(1).strip().lower()

    tokens = 0
    m = _TOKENS_RE.search(content)
    if m:
        try:
            tokens = int(m.group(1))
        except ValueError:
            tokens = 0

    return {"stage": stage, "gate": gate, "carrier": carrier, "tokens": tokens, "file": file_path.name}


def extract_events(run_dir: PathLike) -> list[dict]:
    """从 run_dir 提取关键事件。目录不存在或无 md → 返回 []。"""
    p = Path(run_dir)
    if not p.exists() or not p.is_dir():
        return []
    events: list[dict] = []
    for md_file in sorted(p.glob("*.md")):
        if not md_file.is_file():
            continue
        try:
            content = md_file.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        events.append(_parse_md(content, md_file))
    return events


def suggest(events: list[dict]) -> list[dict]:
    """基于事件序列生成建议列表。"""
    if not events:
        return []

    suggestions: list[dict] = []

    # Rule 1: carrier with multiple NO-GO → suggest switching
    no_go_by: dict[str, list[dict]] = {}
    for ev in events:
        if ev.get("gate") == "NO-GO" and (ev.get("tokens") or 0) > 0:
            c = ev.get("carrier") or "unknown"
            no_go_by.setdefault(c, []).append(ev)
    for carrier, evs in no_go_by.items():
        if len(evs) >= 2:
            suggestions.append({
                "type": "switch_carrier",
                "reason": f"carrier '{carrier}' 在 {len(evs)} 个阶段中 gate=NO-GO，建议切换",
                "confidence": min(0.5 + len(evs) * 0.1, 0.9),
            })

    # Rule 2: all GO → suggest increasing automation
    non_empty = [ev for ev in events if ev.get("gate")]
    if non_empty and all(ev.get("gate") == "GO" for ev in non_empty):
        suggestions.append({
            "type": "increase_automation",
            "reason": "所有阶段 gate=GO，此任务类型可提高自动化程度",
            "confidence": 0.7,
        })

    return suggestions


def top_suggestions(run_dirs, max_suggestions: int = 5) -> list[dict]:
    """批量处理多个 run_dir，合并建议，按 confidence 降序返回最多 max_suggestions 条。"""
    if not run_dirs:
        return []
    all_events: list[dict] = []
    for d in run_dirs:
        all_events.extend(extract_events(d))
    if not all_events:
        return []
    raw = suggest(all_events)
    seen: dict[str, dict] = {}
    for s in raw:
        key = s["type"] + "|" + s["reason"]
        if key not in seen or s["confidence"] > seen[key]["confidence"]:
            seen[key] = s
    return sorted(seen.values(), key=lambda x: x["confidence"], reverse=True)[:max_suggestions]
