"""
跨运行的轻量「学习记忆」（Wayland 认知记忆的简化版）。

- record()：每次运行后把 {任务, Gate, 审查要点} 追加到 devkit/MEMORY.md。
- recall()：下次运行把最近 N 条作为「过往教训」注入 brainstorm/plan 上下文，
  让流程「越跑越聪明」、少重复同类问题。

fail-open：记忆相关任何异常都不影响 loop 主流程。
"""
from __future__ import annotations

import pathlib
import re
from datetime import datetime

ROOT = pathlib.Path(__file__).resolve().parent.parent  # agent-platform/
MEM = ROOT / "devkit" / "MEMORY.md"

_HEADER = ("# Loom 学习记忆（跨运行）\n\n"
           "> 每条 = 一次运行的任务 / Gate / 审查要点。新运行会把最近若干条作为"
           "「过往教训」注入 brainstorm / plan 上下文，避免重复同类问题。\n\n")


def _review_lesson(review_text: str) -> str:
    if not review_text:
        return ""
    verdict = ""
    mv = re.search(r"(REQUEST-CHANGES|APPROVE|NO-GO|GO)", review_text)
    if mv:
        verdict = mv.group(1)
    # 取审查里前几条有信息量的行
    lines = [l.strip("-* 　") for l in review_text.splitlines()
             if l.strip() and not l.startswith("#") and len(l.strip()) > 6][:3]
    body = " / ".join(lines)[:240]
    return (f"{verdict}：" if verdict else "") + body


def record(task: str, gate: str, review_text: str = "") -> None:
    try:
        brief = " ".join(str(task).split())[:70] or "UNKNOWN"
        brief = brief.replace("|", "│").replace("\n", " ")
        lesson = _review_lesson(review_text).replace("\n", " ")
        ts = datetime.now().strftime("%Y-%m-%d %H:%M")
        entry = f"- [{ts}] **{brief}** — Gate: {gate}" + (f"；要点：{lesson}" if lesson else "") + "\n"
        if not MEM.exists():
            MEM.parent.mkdir(parents=True, exist_ok=True)
            MEM.write_text(_HEADER, encoding="utf-8")
        with MEM.open("a", encoding="utf-8") as f:
            f.write(entry)
    except Exception:  # noqa: BLE001
        return


def recall(n: int = 5) -> str:
    """返回最近 n 条记忆，作为可注入的「过往教训」前言；无则空串。"""
    try:
        if not MEM.exists():
            return ""
        entries = [l for l in MEM.read_text(encoding="utf-8").splitlines() if l.startswith("- [")]
        recent = entries[-n:]
        if not recent:
            return ""
        return ("【过往运行回顾（供参考，避免重复同类问题；不是本次任务要求）】\n"
                + "\n".join(recent) + "\n\n")
    except Exception:  # noqa: BLE001
        return ""
