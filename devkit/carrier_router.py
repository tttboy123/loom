# carrier_router.py
"""多 Carrier 负载均衡路由：基于历史 ok_rate + 成本加权选出最优 carrier，支持 fallback 链。

评分公式：score = ok_rate * 100 - avg_cost * 1000
适用范围：ok_rate 较高（>0.3）且重试成本可忽略的场景。
低 ok_rate 场景下真实期望成本 = avg_cost / ok_rate，该公式会低估重试开销，请结合 carrier_health
做二次过滤。

无历史数据时默认 ok_rate=0.5 / avg_cost=0.002 / runs=0（中性先验）。
平分时按 candidates 传入顺序的原始索引作为 tiebreak（稳定确定性排序）。
"""
from __future__ import annotations

_DEFAULT_OK_RATE = 0.5
_DEFAULT_AVG_COST = 0.002


def score_carrier(carrier: str, stage: str, history_rows: list,
                  task_type: "str | None" = None) -> dict:
    """计算单个 carrier 在某 stage 的评分。

    返回 {carrier, ok_rate, avg_cost, runs, score}。
    """
    # 优先用同 task_type 的历史，无匹配则退到全局
    if task_type:
        typed = [r for r in history_rows
                 if r.get("carrier") == carrier and r.get("stage") == stage
                 and r.get("task_type") == task_type]
        matches = typed or [r for r in history_rows
                            if r.get("carrier") == carrier and r.get("stage") == stage]
    else:
        matches = [r for r in history_rows
                   if r.get("carrier") == carrier and r.get("stage") == stage]
    if matches:
        ok_rate = sum(r.get("ok_rate", 0.0) for r in matches) / len(matches)
        avg_cost = sum(r.get("avg_cost", 0.0) for r in matches) / len(matches)
        runs = sum(r.get("runs", 1) for r in matches)
    else:
        ok_rate = _DEFAULT_OK_RATE
        avg_cost = _DEFAULT_AVG_COST
        runs = 0

    score = ok_rate * 100 - avg_cost * 1000
    return {"carrier": carrier, "ok_rate": ok_rate, "avg_cost": avg_cost,
            "runs": runs, "score": score}


def fallback_chain(stage: str, candidates: list, history_rows: list,
                   task_type: "str | None" = None) -> list:
    """返回 candidates 按优先级排列的列表（最优在前）。

    empty candidates → raises ValueError（与 select 保持一致）。
    平分时保持 candidates 原始顺序（稳定 tiebreak）。
    """
    if not candidates:
        raise ValueError("candidates 不能为空")
    scored = [
        (i, score_carrier(c, stage, history_rows, task_type))
        for i, c in enumerate(candidates)
    ]
    scored.sort(key=lambda x: (-x[1]["score"], x[0]))  # 高分优先，平分按原始下标
    return [s["carrier"] for _, s in scored]


def select(stage: str, candidates: list, history_rows: list,
           task_type: "str | None" = None) -> str:
    """从候选 carriers 中选出最优 carrier。

    empty candidates → raises ValueError。
    """
    if not candidates:
        raise ValueError("candidates 不能为空")
    return fallback_chain(stage, candidates, history_rows, task_type)[0]
