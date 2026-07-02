# devkit/run_scorer.py
# 纯标准库，对 run 质量综合打分

from __future__ import annotations
from typing import Any

DEFAULT_WEIGHTS: dict[str, float] = {
    'gate': 0.4,
    'ok_rate': 0.4,
    'efficiency': 0.2,
}

def _gate_score(gate: Any) -> float:
    return 1.0 if gate == 'GO' else 0.0

def _efficiency_score(tokens: Any) -> float:
    # 数值安全：tokens 非法时按 0 处理（最高效率）
    try:
        t = max(0, float(tokens))
    except (TypeError, ValueError):
        t = 0.0
    return 1.0 / (1.0 + t / 1000.0)

def _ok_score(ok_rate: Any) -> float:
    try:
        v = float(ok_rate)
    except (TypeError, ValueError):
        v = 0.0
    # 裁剪到 [0, 1]，避免越界
    if v < 0.0:
        return 0.0
    if v > 1.0:
        return 1.0
    return v

def score_run(run: dict, weights: dict | None = None) -> float:
    """综合打分，返回 [0.0, 1.0] 范围内的加权和。"""
    w = weights if weights is not None else DEFAULT_WEIGHTS
    w_gate = float(w.get('gate', DEFAULT_WEIGHTS['gate']))
    w_ok = float(w.get('ok_rate', DEFAULT_WEIGHTS['ok_rate']))
    w_eff = float(w.get('efficiency', DEFAULT_WEIGHTS['efficiency']))

    g = _gate_score(run.get('gate'))
    o = _ok_score(run.get('ok_rate'))
    e = _efficiency_score(run.get('tokens', 0))

    s = w_gate * g + w_ok * o + w_eff * e
    # 数值稳定裁剪
    if s < 0.0:
        return 0.0
    if s > 1.0:
        return 1.0
    return s

def rank_runs(runs: list[dict]) -> list[dict]:
    """按 score_run 降序排序，返回带 score 字段的新列表（不修改入参）。"""
    scored = [{**r, 'score': score_run(r)} for r in runs]
    scored.sort(key=lambda r: r['score'], reverse=True)
    return scored

def score_summary(runs: list[dict]) -> dict:
    """汇总：count / avg_score / best_id。"""
    if not runs:
        return {'count': 0, 'avg_score': 0.0, 'best_id': None}

    best = max(runs, key=lambda r: score_run(r))
    avg = sum(score_run(r) for r in runs) / len(runs)
    return {
        'count': len(runs),
        'avg_score': float(avg),
        'best_id': best.get('id'),
    }
