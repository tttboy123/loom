# budget.py

import math
import os
import pathlib

DEFAULT_WINDOW = 32768
CARRIER_WINDOWS = {
    "claude": 128000, "claude-code-sub": 128000,
    "codex": 128000, "codex-sub": 128000,
    "glm": 128000, "deepseek": 65536, "minimax": 32768,
}

# Reasoning models burn budget on <think> traces; 900 tok default causes empty content.
CARRIER_MAX_TOKENS: dict[str, int] = {
    "glm": 8000,
    "deepseek": 8000,
    "minimax": 8000,
}

def carrier_window(carrier: str) -> int:
    return CARRIER_WINDOWS.get(carrier, DEFAULT_WINDOW)

def carrier_max_tokens(carrier: str) -> int | None:
    """Return per-carrier minimum max_tokens for reasoning models, or None for default."""
    return CARRIER_MAX_TOKENS.get(carrier, None)

def est_tokens(text: str) -> int:
    cjk_count = sum(1 for ch in text if '一' <= ch <= '鿿')
    non_cjk_count = len(text) - cjk_count
    return cjk_count + math.ceil(non_cjk_count / 3.5)

def budget_tokens(window: int, reserve: float = 0.4) -> int:
    return int(window * (1 - reserve))

DEFAULT_COST_LIMIT_USD = float(os.environ.get("LOOM_COST_LIMIT_USD", "5.00"))


class BudgetExceeded(RuntimeError):
    """Raised when a state transition would exceed the per-call cost limit."""

    def __init__(self, message: str, *, task_id: str, cost_usd: float, limit_usd: float) -> None:
        super().__init__(message)
        self.task_id = str(task_id)
        self.cost_usd = float(cost_usd)
        self.limit_usd = float(limit_usd)


def check(task_id: str, cost_usd: float, *, limit_usd: float | None = None) -> dict:
    """Enforce a per-task USD budget on a state transition.

    Returns ``{"ok": True, "task_id": ..., "cost_usd": ..., "limit_usd": ...}``
    when the cost fits; raises :class:`BudgetExceeded` otherwise. The limit
    defaults to ``DEFAULT_COST_LIMIT_USD`` (``$5.00``) and can be overridden
    via the ``LOOM_COST_LIMIT_USD`` environment variable or ``limit_usd=``.

    ``check()`` is intentionally stateless — the post-call accounting is the
    caller's responsibility. The state_writer hook uses this as a *gate*
    before persisting an expensive transition.
    """
    if cost_usd is None:
        return {
            "ok": True,
            "task_id": str(task_id),
            "cost_usd": 0.0,
            "limit_usd": float(limit_usd) if limit_usd is not None else DEFAULT_COST_LIMIT_USD,
            "checked": False,
        }
    cost = float(cost_usd)
    limit = float(limit_usd) if limit_usd is not None else DEFAULT_COST_LIMIT_USD
    if cost < 0:
        raise ValueError(f"cost_usd must be non-negative, got {cost}")
    if cost > limit:
        raise BudgetExceeded(
            f"task {task_id!r} cost ${cost:.4f} exceeds per-call limit ${limit:.4f}",
            task_id=str(task_id),
            cost_usd=cost,
            limit_usd=limit,
        )
    return {
        "ok": True,
        "task_id": str(task_id),
        "cost_usd": cost,
        "limit_usd": limit,
        "checked": True,
    }


def pack(blocks, budget, est=est_tokens):
    """
    根据预算和优先级打包文本块。
    
    1. 所有 protected=True 的块无条件保留（即使总 token 超预算）。
    2. 其余块按 prio 升序排列，依次尝试加入，直到预算用尽。
    3. 返回结果包含保留的块名、丢弃的块名、使用的 token 总数以及保留块的文本。
    """
    # 分离 protected 和非 protected 块
    protected_blocks = [block for block in blocks if block["protected"]]
    non_protected_blocks = [block for block in blocks if not block["protected"]]
    
    # 按 prio 升序排列非 protected 块
    non_protected_blocks.sort(key=lambda x: x["prio"])
    
    # 计算已保留的 token 总数
    used = sum(est(block["text"]) for block in protected_blocks)
    
    # 初始化保留和丢弃的块名列表
    kept = [block["name"] for block in protected_blocks]
    dropped = []
    
    # 依次尝试加入非 protected 块
    for block in non_protected_blocks:
        est_value = est(block["text"])
        if used + est_value <= budget:
            kept.append(block["name"])
            used += est_value
        else:
            dropped.append(block["name"])
    
    # 按原始顺序排列保留的块名
    kept_sorted = []
    for block in blocks:
        if block["name"] in kept:
            kept_sorted.append(block["name"])
    
    # 按保留顺序拼接文本
    text = "\n\n".join(block["text"] for block in blocks if block["name"] in kept)
    
    return {
        "kept": kept_sorted,
        "dropped": dropped,
        "used": used,
        "text": text
    }
