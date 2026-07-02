"""Normalize legacy provider/model strings to stable Loom carrier aliases."""
from __future__ import annotations

from typing import Iterable

_STAGE_DEFAULTS = {
    "brainstorm": "loom-product",
    "plan": "loom-orchestrator",
    "review": "loom-reviewer",
}

_EXACT_ALIASES = {
    "codex-sub": "codex-sub",
    "loom-product": "loom-product",
    "loom-orchestrator": "loom-orchestrator",
    "loom-reviewer": "loom-reviewer",
    "loom-dev": "loom-dev",
    "loom-tester": "loom-tester",
    "minimax": "minimax",
    "minimax-m27": "minimax-m27",
    "minimax-m27-highspeed": "minimax-m27-highspeed",
    "glm": "glm",
    "deepseek": "deepseek",
}

_RAW_MODEL_ALIASES = {
    "gpt-5.4": "codex-sub",
    "gpt5.4": "codex-sub",
    "gpt-5.4-codex": "codex-sub",
    "gpt5.4-codex": "codex-sub",
    "gpt-5.5": "codex-sub",
    "gpt5.5": "codex-sub",
    "openai/gpt-5.4": "codex-sub",
    "openai/gpt-5.5": "codex-sub",
    "minimax-m3": "minimax",
    "minimax/minimax-m3": "minimax",
    "m3": "minimax",
    "minimax-m2.7": "minimax-m27",
    "minimax/minimax-m2.7": "minimax-m27",
    "m2.7": "minimax-m27",
    "minimax-m2.7-highspeed": "minimax-m27-highspeed",
    "minimax/minimax-m2.7-highspeed": "minimax-m27-highspeed",
    "m2.7-highspeed": "minimax-m27-highspeed",
}


def normalize_model_name(model: str, *, stage: str | None = None) -> str:
    raw = (model or "").strip()
    if not raw:
        return ""
    lowered = raw.lower()
    if lowered in _EXACT_ALIASES:
        return _EXACT_ALIASES[lowered]
    if lowered in _RAW_MODEL_ALIASES:
        mapped = _RAW_MODEL_ALIASES[lowered]
        default = _STAGE_DEFAULTS.get((stage or "").strip().lower())
        if mapped == "codex-sub" and default:
            return default
        return mapped
    if lowered.startswith("glm") or "/glm" in lowered or "chatglm" in lowered:
        return "glm"
    if lowered.startswith("deepseek") or "/deepseek" in lowered:
        return "deepseek"
    if lowered.startswith("gpt-5") or lowered.startswith("openai/gpt-5"):
        return _STAGE_DEFAULTS.get((stage or "").strip().lower(), "codex-sub")
    if lowered.startswith("minimax"):
        if "m2.7-highspeed" in lowered:
            return "minimax-m27-highspeed"
        if "m2.7" in lowered:
            return "minimax-m27"
        return "minimax"
    return raw


def normalize_model_list(models: Iterable[str], *, stage: str | None = None) -> list[str]:
    out: list[str] = []
    seen: set[str] = set()
    for model in models:
        normalized = normalize_model_name(model, stage=stage)
        if not normalized or normalized in seen:
            continue
        seen.add(normalized)
        out.append(normalized)
    return out
