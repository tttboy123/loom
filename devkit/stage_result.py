"""Encapsulate a single stage execution result. Stdlib only."""
from __future__ import annotations


def make(stage: str, ok: bool, output: str, tokens: int, duration_s: float) -> dict:
    return {"stage": stage, "ok": ok, "output": output, "tokens": tokens, "duration_s": duration_s}


def is_ok(result: dict) -> bool:
    return bool(result.get("ok"))


def merge(results: list[dict]) -> dict:
    ok = all(r.get("ok", False) for r in results)
    total_tokens = sum(r.get("tokens", 0) for r in results)
    total_duration = sum(r.get("duration_s", 0.0) for r in results)
    stages = [r.get("stage", "") for r in results]
    outputs = [r.get("output", "") for r in results]
    return {"ok": ok, "total_tokens": total_tokens, "total_duration": total_duration,
            "stages": stages, "outputs": outputs}


def result_summary(result: dict) -> str:
    marker = "[OK]" if result.get("ok") else "[FAIL]"
    return f"{marker} {result.get('stage', '')}"
