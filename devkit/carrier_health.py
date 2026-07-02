# carrier_health.py
"""Carrier 健康探针：向 LiteLLM 网关探测各 carrier 可达性和延迟，结果缓存到本地文件。"""
from __future__ import annotations

import json
import pathlib
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone

_DEFAULT_CACHE = pathlib.Path(__file__).parent / "carrier_health.json"


def probe(carrier: str, base_url: str, api_key: str, timeout: int = 5) -> dict:
    """探测单个 carrier 健康状态。

    向 LiteLLM 发送最小请求（max_tokens=1），测量延迟。
    返回 {ok: bool, latency_ms: float, error: str}。
    """
    payload_dict = {
        "model": carrier,
        "messages": [{"role": "user", "content": "ping"}],
        "max_tokens": 1,
    }
    try:
        from devkit import rdloop as _rdloop
        payload_dict = _rdloop._apply_model_specific_request_fields(payload_dict, carrier)
    except Exception:
        pass
    payload = json.dumps(payload_dict).encode("utf-8")
    url = base_url.rstrip("/") + "/chat/completions"
    auth_key = api_key
    try:
        if getattr(_rdloop, "_is_minimax_reasoning_model")(carrier) and base_url.rstrip("/") in {"http://localhost:4000/v1", "http://127.0.0.1:4000/v1"}:
            minimax_key = _rdloop.load_env_key("MINIMAX_API_KEY")
            if minimax_key:
                url = "https://api.minimaxi.com/v1/chat/completions"
                auth_key = minimax_key
    except Exception:
        pass
    req = urllib.request.Request(
        url,
        data=payload,
        headers={"Content-Type": "application/json",
                 "Authorization": f"Bearer {auth_key}"},
        method="POST",
    )
    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=timeout):
            latency_ms = (time.monotonic() - t0) * 1000
            return {"ok": True, "latency_ms": round(latency_ms, 1), "error": ""}
    except urllib.error.HTTPError as e:
        return {"ok": False, "latency_ms": 0.0, "error": f"HTTP {e.code}: {e.reason}"}
    except Exception as e:  # noqa: BLE001 — timeout / connection refused etc.
        return {"ok": False, "latency_ms": 0.0, "error": str(e)[:120]}


def probe_all(carriers: list, base_url: str, api_key: str, timeout: int = 5) -> dict:
    """批量串行探测多个 carriers，返回 {carrier: {ok, latency_ms, error}}。"""
    if not carriers:
        return {}
    return {c: probe(c, base_url, api_key, timeout) for c in carriers}


def healthy_carriers(results: dict) -> list:
    """筛出 ok=True 的 carriers，按 latency_ms 升序排列。"""
    ok = [(name, r) for name, r in results.items() if r.get("ok")]
    ok.sort(key=lambda x: x[1].get("latency_ms", 0.0))
    return [name for name, _ in ok]


def load_cache(cache_path: "str | None" = None) -> dict:
    """从缓存文件读取上次探测结果，不存在返回 {}。"""
    path = pathlib.Path(cache_path) if cache_path else _DEFAULT_CACHE
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return {}


def save_cache(results: dict, cache_path: "str | None" = None) -> None:
    """将探测结果（带时间戳）写入缓存文件。"""
    path = pathlib.Path(cache_path) if cache_path else _DEFAULT_CACHE
    ts = datetime.now(tz=timezone.utc).isoformat()
    stamped = {k: dict(v, ts=ts) for k, v in results.items()}
    path.write_text(json.dumps(stamped, ensure_ascii=False, indent=2), encoding="utf-8")
