# devkit/carrier_selector.py
"""carrier_selector: 根据历史成功率选择最合适的 carrier（纯标准库）。

公开 API:
    select(candidates, history) -> str | None
    rank_carriers(candidates, history) -> list[dict]
    fallback_chain(primary, fallbacks) -> list[str]
"""

from __future__ import annotations

from typing import Iterable

__all__ = ["select", "rank_carriers", "fallback_chain"]

# 中性先验：当某个 candidate 在历史中没有任何记录时，给一个 0.5 的先验
# 成功率（count=0），避免在「零信息」情况下被误判为最差。
NEUTRAL_OK_RATE = 0.5

def _aggregate(candidates: Iterable[str], history: list[dict]) -> dict[str, dict]:
    """聚合每个 carrier 的 ok / total 计数。"""
    stats: dict[str, dict] = {
        c: {"ok": 0, "total": 0} for c in candidates
    }
    for entry in history:
        carrier = entry.get("carrier")
        if carrier is None:
            continue
        if carrier not in stats:
            # 历史中出现了但不在候选里：忽略，不影响选择。
            continue
        ok = bool(entry.get("ok"))
        stats[carrier]["total"] += 1
        if ok:
            stats[carrier]["ok"] += 1
    return stats

def rank_carriers(candidates: list[str], history: list[dict]) -> list[dict]:
    """返回 [{carrier, ok_rate, count}, ...]，按 ok_rate 降序。

    - 排序键：ok_rate 降序；相同 ok_rate 时保持原 candidates 顺序（稳定）。
    - history 中无记录的 carrier：ok_rate = 0.5（中性），count = 0。
    """
    stats = _aggregate(candidates, history)
    ranked: list[dict] = []
    for c in candidates:
        s = stats[c]
        total = s["total"]
        if total == 0:
            ok_rate: float = NEUTRAL_OK_RATE
        else:
            ok_rate = s["ok"] / total
        ranked.append({"carrier": c, "ok_rate": ok_rate, "count": total})

    # 用 enumerate 取得原顺序作为 tie-breaker，保证稳定排序。
    indexed = list(enumerate(ranked))
    indexed.sort(key=lambda kv: (-kv[1]["ok_rate"], kv[0]))
    return [item for _, item in indexed]

def select(candidates: list[str], history: list[dict]) -> str | None:
    """从 candidates 中选最优 carrier。

    - 空 candidates -> None
    - 空 history    -> candidates[0]
    - 否则          -> rank_carriers 排序后第一名
    """
    if not candidates:
        return None
    if not history:
        return candidates[0]
    ranked = rank_carriers(candidates, history)
    return ranked[0]["carrier"]

def fallback_chain(primary: str, fallbacks: list[str]) -> list[str]:
    """返回 [primary] + fallbacks，去重并保持原顺序。"""
    seen: set[str] = set()
    chain: list[str] = []
    for name in [primary, *fallbacks]:
        if name in seen:
            continue
        seen.add(name)
        chain.append(name)
    return chain
