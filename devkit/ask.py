"""
@ask-model（借鉴 Kode）：临时问某个/某些载体一句，复用网关计费，不开整条 loop。

控制台（console/server.py）与 CLI（python3 -m devkit ask）共用这一份实现，避免漂移。
注意：内部用 `rdloop.gateway_chat` 走模块属性（非 from-import），便于单测打桩。
"""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from typing import List

from devkit.model_aliases import normalize_model_name
from devkit import rdloop

ASK_SYS = "你是被临时咨询的专家模型，简洁、直接、只答要点。"
_CONTROL_PLANE_MODELS = {"codex-sub", "loom-product", "loom-orchestrator", "loom-reviewer"}
_CONTROL_PLANE_FALLBACKS = {
    "codex-sub": ["minimax-m27-highspeed", "minimax-m27", "minimax", "glm", "deepseek"],
    "loom-product": ["minimax-m27-highspeed", "minimax-m27", "minimax", "glm", "deepseek"],
    "loom-orchestrator": ["minimax-m27-highspeed", "minimax-m27", "minimax", "glm", "deepseek"],
    "loom-reviewer": ["minimax-m27-highspeed", "minimax-m27", "minimax", "glm", "deepseek"],
}


def _normalize_model(model: str) -> str:
    return normalize_model_name(model)


def _is_retryable_blocked_error(error: str) -> bool:
    text = (error or "").lower()
    return (
        "at capacity" in text
        or "please try a different model" in text
        or "rate limit" in text
        or "too many requests" in text
        or "service unavailable" in text
        or "timed out" in text
        or "timeout" in text
        or "overloaded" in text
        or "empty response" in text
        or "http 500" in text
        or "http 502" in text
        or "http 503" in text
        or "http 504" in text
        or "429" in text
    )


def _default_timeout(model: str) -> int:
    model = _normalize_model(model)
    if model in _CONTROL_PLANE_MODELS:
        return 75
    if "minimax" in model.lower():
        return 180
    return 120


def _default_fallback_models(model: str) -> list[str]:
    model = _normalize_model(model)
    extras = _CONTROL_PLANE_FALLBACKS.get(model, [])
    out: list[str] = []
    seen: set[str] = set()
    for name in [model, *extras]:
        if not name or name in seen:
            continue
        seen.add(name)
        out.append(name)
    return out


def _failure_code_from_error(error: str) -> str:
    text = (error or "").lower()
    if "http 429" in text or "rate limit" in text or "too many requests" in text:
        return "RATE_LIMIT"
    if "http 500" in text:
        return "PROVIDER_HTTP_500"
    if "http 502" in text:
        return "PROVIDER_HTTP_502"
    if "http 503" in text:
        return "PROVIDER_HTTP_503"
    if "http 504" in text:
        return "PROVIDER_HTTP_504"
    if "timed out" in text or "timeout" in text:
        return "PROVIDER_TIMEOUT"
    if "at capacity" in text or "service unavailable" in text or "overloaded" in text:
        return "PROVIDER_CAPACITY"
    if "empty response" in text:
        return "EMPTY_RESPONSE"
    if "invalid model name" in text:
        return "INVALID_MODEL"
    return "ASK_FAILED"


def ask_one(model: str, prompt: str, base_url: str, api_key: str,
            max_tokens: int = 700, tag: str = "ask", timeout: int | None = None,
            extra_tags: list[str] | None = None) -> dict:
    """问单个载体/后端，返回 {model, ok, served, tokens, cost, content|error}。"""
    try:
        model = _normalize_model(model)
        timeout = timeout or _default_timeout(model)
        tags = [tag, f"model:{model}"]
        if extra_tags:
            tags.extend(extra_tags)
        try:
            ok, content, served, tokens, cost = rdloop.gateway_chat(
                base_url, api_key, model, ASK_SYS, prompt,
                max_tokens, timeout=timeout, tags=tags)
        except TypeError as e:
            # 兼容旧测试桩 / 旧 monkeypatch：未接受 timeout 关键字时退回老签名。
            if "timeout" not in str(e):
                raise
            ok, content, served, tokens, cost = rdloop.gateway_chat(
                base_url, api_key, model, ASK_SYS, prompt,
                max_tokens, tags=tags)
        if not ok:
            error = str(content)[:300]
            return {"model": model, "ok": False, "error": error,
                    "failure_code": _failure_code_from_error(error)}
        if not str(content or "").strip():
            return {"model": model, "ok": False, "error": "empty response",
                    "failure_code": "EMPTY_RESPONSE"}
        return {"model": model, "ok": True, "served": served,
                "tokens": tokens, "cost": cost, "content": content}
    except Exception as e:  # noqa: BLE001
        error = f"{type(e).__name__}: {e}"
        return {"model": model, "ok": False, "error": error,
                "failure_code": _failure_code_from_error(error)}


def ask_models(models: List[str], prompt: str, base_url: str, api_key: str,
               max_tokens: int = 700, tag: str = "ask", timeout: int | None = None,
               extra_tags: list[str] | None = None) -> List[dict]:
    """问一个或多个载体；多个时线程池并行、按输入顺序返回。"""
    models = [m.strip() for m in models if m and m.strip()]
    if not models:
        return []
    if len(models) == 1:
        return [ask_one_with_fallback(
            [models[0]], prompt, base_url, api_key,
            max_tokens=max_tokens, tag=tag, timeout=timeout, extra_tags=extra_tags,
        )]
    with ThreadPoolExecutor(max_workers=min(len(models), 6)) as ex:
        results = list(ex.map(
            lambda m: ask_one(m, prompt, base_url, api_key, max_tokens, tag, timeout, extra_tags), models))
    order = {m: i for i, m in enumerate(models)}
    results.sort(key=lambda r: order.get(r["model"], 99))
    return results


def ask_one_with_fallback(
    models: List[str],
    prompt: str,
    base_url: str,
    api_key: str,
    max_tokens: int = 700,
    tag: str = "ask",
    timeout: int | None = None,
    extra_tags: list[str] | None = None,
) -> dict:
    """按序尝试多个模型；遇到可恢复错误（如容量不足）会自动尝试下一个。"""
    candidates: list[str] = []
    seen: set[str] = set()
    for raw in models:
        for model in _default_fallback_models(raw):
            if model in seen:
                continue
            seen.add(model)
            candidates.append(model)
    if not candidates:
        return {"model": "", "ok": False, "error": "no model specified"}

    last_resp = {"model": candidates[-1], "ok": False, "error": "all models failed", "attempted_models": candidates}
    for idx, model in enumerate(candidates):
        resp = ask_one(
            model, prompt, base_url, api_key,
            max_tokens=max_tokens, tag=tag, timeout=timeout, extra_tags=extra_tags,
        )
        if resp.get("ok"):
            resp["attempted_models"] = candidates[:idx + 1]
            return resp

        last_resp = resp
        last_resp["attempted_models"] = candidates[:idx + 1]
        if idx == len(candidates) - 1:
            break
        if not _is_retryable_blocked_error(str(resp.get("error", ""))):
            break

    last_resp["error"] = f"ask fallback exhausted; attempted {', '.join(candidates)}; last_error={last_resp.get('error','')}"
    last_resp["failure_code"] = last_resp.get("failure_code") or _failure_code_from_error(last_resp["error"])
    return last_resp
