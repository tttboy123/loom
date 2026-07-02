# 任务：实现 devkit/learn.py — Learning Sidecar（只读分析）

## 目标
新建 `devkit/learn.py`，实现一个只读的"学习侧车"：读取历史 run 数据，输出结构化建议（carrier 优化 / quota 警告 / safety 热点），供 `devkit learn` CLI 展示。

**不得**修改任何已有文件。只交付 `devkit/learn.py`。

---

## 接口要求

### `analyze(runs_dir: pathlib.Path | None = None) -> dict`

读取历史 run，返回：
```python
{
    "suggestions": [Suggestion, ...],   # 建议列表（见下方）
    "summary": {
        "total_runs": int,
        "go_rate": float,               # GO 率 0–1
        "top_task_type": str | None,    # 最多任务类型
        "avg_cost_usd": float,          # 均次成本
        "total_cost_usd": float,
    }
}
```

### Suggestion 结构
```python
{
    "type": str,        # "carrier" | "quota" | "safety" | "golden"
    "confidence": float,  # 0–1
    "reason": str,      # 一句中文说明
    "action": str,      # 具体建议（一句话，可直接显示给用户）
    "data": dict,       # 原始证据（backend, ok_rate, task_type, count 等）
}
```

### `suggest_carrier(task_type: str, runs_dir=None) -> Suggestion | None`

针对特定 task_type 给出最优 carrier 建议：
- 读 model_fitness rows，找 ok_rate 最高的 backend
- ok_rate < 0.5 且样本 ≥ 3 → 返回警告
- 无数据 → 返回 None

### `quota_trend(runs_dir=None) -> dict`

统计最近 10 次 run 的成本趋势：
```python
{
    "recent_10_cost": float,    # 最近10次总成本
    "avg_cost": float,
    "max_cost": float,
    "trend": "rising" | "stable" | "falling",  # 均值对比前半/后半
}
```

---

## 实现要求

1. **只用标准库** + devkit 已有模块（`insight.runs_list`, `insight.model_fitness`）
2. `runs_dir` 默认值：`pathlib.Path(__file__).resolve().parent.parent / "devkit" / "runs"`
3. `analyze()` 的 suggestions 由以下规则生成：
   - **carrier 建议**：对每个 task_type，若 model_fitness 有两个以上 backend 有数据，且最优 backend 的 ok_rate 比最差高 ≥ 20pp → 生成 carrier 建议
   - **quota 警告**：最近 5 次 run 均成本 > 最近 20 次均成本的 1.5 倍 → 生成 quota 警告
   - **safety 热点**：若 run 中含 safety 违规记录（run-log.md 含"Safety NO-GO"字样）→ 生成 safety 建议
4. confidence 规则：样本 ≥ 10 → 0.9，≥ 5 → 0.7，≥ 2 → 0.5，< 2 → 0.3
5. 所有函数容忍文件不存在 / 格式错误，默认返回空结果，不抛异常

---

## 不允许
- 不得写入任何文件
- 不得修改已有文件（artifact.py / rdloop.py / insight.py / __main__.py 等）
- 不得添加第三方依赖
- 不得添加 print 语句（调用方自己决定是否打印）
