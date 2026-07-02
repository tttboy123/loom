"""devkit/wallet.py — Provider 余额查询适配器（纯标准库）。"""
from __future__ import annotations

import json
import urllib.error
import urllib.request


def check_deepseek(api_key: str, timeout: int = 10) -> dict:
    """调用 DeepSeek 余额 API。返回 {ok, balance_usd, currency, error}。"""
    if not api_key:
        return {"ok": False, "balance_usd": 0.0, "currency": "USD", "error": "no api_key"}
    req = urllib.request.Request(
        "https://api.deepseek.com/user/balance",
        headers={"Authorization": f"Bearer {api_key}", "Accept": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            data = json.loads(r.read())
        infos = data.get("balance_infos") or []
        if not infos:
            return {"ok": False, "balance_usd": 0.0, "currency": "USD", "error": "no balance_infos"}
        total = float(infos[0].get("total_balance", "0") or "0")
        currency = infos[0].get("currency", "USD")
        return {"ok": True, "balance_usd": total, "currency": currency, "error": ""}
    except urllib.error.HTTPError as e:
        return {"ok": False, "balance_usd": 0.0, "currency": "USD", "error": f"HTTP {e.code}"}
    except Exception as e:
        return {"ok": False, "balance_usd": 0.0, "currency": "USD", "error": str(e)}


def estimate_balance(provider: str, api_key: str = "", timeout: int = 10) -> dict:
    """统一余额查询入口。返回 {ok, balance_usd, source, error}。"""
    if provider == "deepseek":
        r = check_deepseek(api_key, timeout=timeout)
        return {"ok": r["ok"], "balance_usd": r["balance_usd"], "source": "api", "error": r["error"]}
    return {"ok": True, "balance_usd": -1.0, "source": "unknown", "error": ""}


def summary(providers: list[str], api_keys: dict) -> list[dict]:
    """批量查询多个 provider 余额。"""
    results = []
    for p in providers:
        key = api_keys.get(p, "")
        r = estimate_balance(p, api_key=key)
        results.append({"provider": p, **r})
    return results
