# capacity.py
"""运行前容量预检：预估 token/成本，与 budget 对比，建议可删减的 stages。"""
from __future__ import annotations

_FALLBACK = {"plan": 1000, "implement": 3000, "verify": 1500,
             "review": 1500, "brainstorm": 800}
_OPTIONAL = ("brainstorm", "verify")


def _stage_estimate(stage: str, carrier: str, history_rows: list) -> tuple:
    """从历史行找 (stage, carrier) 的均值，找不到用兜底值。"""
    matches = [r for r in history_rows
               if r.get("stage") == stage and
               (not carrier or r.get("carrier", "") == carrier)]
    if not matches:
        matches = [r for r in history_rows if r.get("stage") == stage]
    if matches:
        avg_tok = sum(r.get("avg_tokens", 0) for r in matches) / len(matches)
        avg_cost = sum(r.get("avg_cost", 0.0) for r in matches) / len(matches)
        return max(1, int(avg_tok)), float(avg_cost)
    fallback_tok = _FALLBACK.get(stage, 1000)
    return fallback_tok, fallback_tok * 0.000002


def estimate_run(stages: list, carrier_map: dict, history_rows: list) -> dict:
    """预估一次 run 的 token 和成本。

    返回 {estimated_tokens: int, estimated_cost: float, per_stage: {stage: {tokens, cost}}}
    """
    per_stage: dict = {}
    total_tok, total_cost = 0, 0.0
    for stage in stages:
        carrier = carrier_map.get(stage, "") if isinstance(carrier_map, dict) else ""
        tok, cost = _stage_estimate(stage, carrier, history_rows if isinstance(history_rows, list) else [])
        per_stage[stage] = {"tokens": tok, "cost": cost}
        total_tok += tok
        total_cost += cost
    return {"estimated_tokens": total_tok, "estimated_cost": total_cost, "per_stage": per_stage}


def preflight_check(stages: list, carrier_map: dict,
                    history_rows: list, budget) -> dict:
    """检查 budget 限制下是否可运行。

    返回 {ok: bool, warning: str}
    """
    rows = history_rows if isinstance(history_rows, list) else []
    est = estimate_run(stages, carrier_map if isinstance(carrier_map, dict) else {}, rows)
    cost = est["estimated_cost"]
    no_history = not rows

    if budget is None:
        warning = "历史数据不足，成本预估基于兜底值" if no_history else ""
        return {"ok": True, "warning": warning}

    if cost > budget:
        return {"ok": False,
                "warning": f"预估 ${cost:.5f} 超出预算 ${budget:.5f}"}

    warning = "历史数据不足，成本预估基于兜底值" if no_history else ""
    return {"ok": True, "warning": warning}


def suggest_cheaper(stages: list, carrier_map: dict,
                    history_rows: list, budget: float) -> list:
    """超预算时建议可删减的 optional stages（仅 brainstorm/verify）。"""
    return [s for s in stages if s in _OPTIONAL]
