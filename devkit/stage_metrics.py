"""
stage_metrics.py - 收集 stage 执行指标（纯标准库）

提供：
  - collect(stages)        -> dict   汇总指标
  - percentile(values, p)  -> float  百分位计算
  - metrics_report(metrics)-> str    单行报告

设计要点：
  * 纯标准库，不引入任何第三方依赖；
  * 空集合情况下，count=0、ok_rate=0.0、avg_*=0.0、total_tokens=0；
  * percentile 的 p 裁剪到 [0,100]；空列表返回 0.0；
  * metrics_report 只格式化非空语义字段，遵循 golden 期望的字符串形式。
"""

from __future__ import annotations

from typing import Any

def collect(stages: list[dict]) -> dict:
    """
    汇总一组 stage 的执行指标。

    输入每个 stage 字段：name(str), tokens(int), duration_s(float), ok(bool)
    返回字段：count(int), ok_rate(float), avg_tokens(float),
             avg_duration(float), total_tokens(int)

    空列表时全部为 0/0.0，避免除零。
    """
    count = len(stages)

    if count == 0:
        return {
            "count": 0,
            "ok_rate": 0.0,
            "avg_tokens": 0.0,
            "avg_duration": 0.0,
            "total_tokens": 0,
        }

    ok_count = sum(1 for s in stages if s.get("ok"))
    total_tokens = sum(int(s.get("tokens", 0)) for s in stages)
    total_duration = sum(float(s.get("duration_s", 0.0)) for s in stages)

    return {
        "count": count,
        "ok_rate": ok_count / count,
        "avg_tokens": total_tokens / count,
        "avg_duration": total_duration / count,
        "total_tokens": total_tokens,
    }

def percentile(values: list[float], p: float) -> float:
    """
    返回第 p 百分位（p ∈ [0, 100]）。
    - 空列表：返回 0.0（golden 6）
    - p 越界：裁剪到 [0, 100]
    - 使用线性插值（与 numpy.percentile 默认 linear 一致），
      以保证 [1,2,3,4,5] 的端点恰好是 1.0 和 5.0（golden 7, 8）
    """
    if not values:
        return 0.0

    # 裁剪 p 到 [0, 100]
    p = max(0.0, min(100.0, float(p)))

    sorted_vals = sorted(float(v) for v in values)
    n = len(sorted_vals)

    if n == 1:
        return sorted_vals[0]

    # 将百分位 [0, 100] 映射到索引空间 [0, n-1]
    rank = (p / 100.0) * (n - 1)
    lower_idx = int(rank)  # 向下取整
    upper_idx = min(lower_idx + 1, n - 1)
    frac = rank - lower_idx

    return sorted_vals[lower_idx] + (sorted_vals[upper_idx] - sorted_vals[lower_idx]) * frac

def metrics_report(metrics: dict) -> str:
    """
    单行指标报告：
      'count={count} ok={ok_rate:.0%} avg_tokens={avg_tokens:.0f}'

    设计为对空指标（count=0）也合法：ok_rate=0.0 -> '0%'。
    """
    count = metrics.get("count", 0)
    ok_rate = metrics.get("ok_rate", 0.0)
    avg_tokens = metrics.get("avg_tokens", 0.0)

    return f"count={count} ok={ok_rate:.0%} avg_tokens={avg_tokens:.0f}"
