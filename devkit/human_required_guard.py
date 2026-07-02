"""Guard for tasks that cannot succeed in report-only autonomy mode."""

from __future__ import annotations

import re
from typing import Any

from devkit.delivery_mode import resolve_delivery_mode

_PATH_RE = re.compile(r"\b((?:devkit|tests|console|scripts|docs|litellm|runs|build)/[\w./-]+\.[A-Za-z]+)\b")
_REPORT_ONLY_PREFIXES = ("runs/", "build/")
_WRITE_SIGNALS = (
    "修复",
    "修改",
    "新增",
    "创建",
    "实现",
    "增加",
    "接入",
    "改写",
    "rename",
    "fix",
    "patch",
    "edit",
    "update",
    "create",
    "add",
    "implement",
    "write",
)
_HUMAN_ONLY_SIGNALS = (
    "人类运维",
    "不接受 implement 自动化",
    "需人工 apply",
    "人工 apply",
    "人工处理",
    "人工运维",
    "human only",
    "human-only",
    "human ops",
    "manual apply",
)
def _task_text(item: Any) -> str:
    if not isinstance(item, dict):
        return ""
    for key in ("task", "text", "title", "body"):
        value = item.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def delivery_mode(item: Any) -> str:
    if not isinstance(item, dict):
        return resolve_delivery_mode()
    return resolve_delivery_mode(
        delivery_mode=item.get("delivery_mode"),
        apply_target=item.get("apply_target"),
        apply_git=item.get("apply_git"),
    )


def _carrier_values(item: Any) -> list[str]:
    if not isinstance(item, dict):
        return []
    carrier = item.get("carrier")
    if not isinstance(carrier, dict):
        return []
    out: list[str] = []
    for value in carrier.values():
        if isinstance(value, str) and value.strip():
            out.append(value.strip())
    return out


def referenced_paths(item: Any) -> list[str]:
    text = _task_text(item)
    return list(dict.fromkeys(_PATH_RE.findall(text)))


def _looks_like_write_task(text: str) -> bool:
    lowered = text.lower()
    return any(sig in text for sig in _WRITE_SIGNALS if not sig.isascii()) or any(
        sig in lowered for sig in _WRITE_SIGNALS if sig.isascii()
    )


def human_required_reason(item: Any) -> str | None:
    text = _task_text(item)
    lowered = text.lower()
    for sig in _HUMAN_ONLY_SIGNALS:
        if (sig.isascii() and sig in lowered) or (not sig.isascii() and sig in text):
            return f"explicit-human-signal:{sig}"

    for carrier in _carrier_values(item):
        if carrier.lower().startswith("human-"):
            return f"human-carrier:{carrier}"

    mode = delivery_mode(item)
    paths = referenced_paths(item)
    if paths and _looks_like_write_task(text):
        repo_targets = [p for p in paths if not p.startswith(_REPORT_ONLY_PREFIXES)]
        if repo_targets and mode == "report-only":
            return "report-only-cannot-apply:" + ",".join(repo_targets[:3])
    return None


def is_human_only(item: Any) -> bool:
    return human_required_reason(item) is not None
