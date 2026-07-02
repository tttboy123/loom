"""devkit.model_selector —— 纯标准库，按任务属性选最优模型。

公开 API：
    capability_score(model) -> int
    select_model(task, available) -> str
    rank_models(models) -> list[str]
    model_info(model) -> dict
"""

from __future__ import annotations

# ---- 能力分（base） ----------------------------------------------------------
_BASE_SCORES: dict[str, int] = {
    "claude": 100,
    "deepseek": 80,
    "minimax": 75,
    "glm": 70,
}
_DEFAULT_SCORE = 50

# ---- high priority 加权 ------------------------------------------------------
# 高优任务仅给头部两个模型加权；加到 base 上。
_HIGH_PRIORITY_BONUS: dict[str, int] = {
    "claude": 50,     # 100 → 150
    "deepseek": 20,   #  80 → 100
}

def capability_score(model: str) -> int:
    """模型名 → 能力分。未知模型返回 50。"""
    return _BASE_SCORES.get(model, _DEFAULT_SCORE)

def _effective_score(model: str, high_priority: bool) -> int:
    """高优任务时给头部模型加权。"""
    score = capability_score(model)
    if high_priority:
        score += _HIGH_PRIORITY_BONUS.get(model, 0)
    return score

def select_model(task: dict, available: list[str]) -> str:
    """从 available 中选能力分最高的模型。

    - task.get('priority') == 'high' 时头部模型加权。
    - available 为空返回 ''。
    - 并列时取首个（max 的稳定行为）。
    """
    if not available:
        return ""
    high_priority = task.get("priority") == "high"
    return max(available, key=lambda m: _effective_score(m, high_priority))

def rank_models(models: list[str]) -> list[str]:
    """按 capability_score 降序；同分保持原序（sorted 稳定）。"""
    return sorted(models, key=capability_score, reverse=True)

def model_info(model: str) -> dict:
    """{model, score, tier}；tier 由 score 决定。"""
    score = capability_score(model)
    if score >= 100:
        tier = "premium"
    elif score >= 70:
        tier = "standard"
    else:
        tier = "basic"
    return {"model": model, "score": score, "tier": tier}
