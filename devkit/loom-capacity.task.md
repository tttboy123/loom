实现新模块 `capacity.py`（运行前容量预检）。只写这一个文件，不写测试文件，文件第一行 `# capacity.py`。

## 背景
在 run_loop 开始前，根据历史数据预估本次 run 的 token/成本，与 --budget 对比，若超出则提示用户缩减 stages 或换便宜模型。纯分析层，不调用 LLM，不修改任何 harness 文件。

## 接口

### `estimate_run(stages: list[str], carrier_map: dict, history_rows: list[dict]) -> dict`
预估一次 run 的 token 和成本。
- `stages`: stage key 列表，如 ["plan","implement","verify"]
- `carrier_map`: {stage_key: carrier_name}，未指定的 stage 按历史均值估
- `history_rows`: 来自 insight.run_stats_by_stage() 的 row 列表
  每行含: `{"stage": str, "carrier": str, "avg_tokens": float, "avg_cost": float, "count": int}`
  若为空则按兜底值估（plan=1000, implement=3000, verify=1500, review=1500）
- 返回：`{"estimated_tokens": int, "estimated_cost": float, "per_stage": {stage: {tokens, cost}}}`

### `preflight_check(stages: list[str], carrier_map: dict, history_rows: list[dict], budget: float | None) -> dict`
在 budget 限制下检查是否可以运行。
- 调用 estimate_run() 得到预估
- 若 budget 为 None 或预估成本 <= budget → `{"ok": True, "warning": ""}`
- 若预估成本 > budget → `{"ok": False, "warning": f"预估 ${estimated:.5f} 超出预算 ${budget:.5f}"}`
- 若历史数据不足（history_rows 空） → `{"ok": True, "warning": "历史数据不足，成本预估基于兜底值"}`

### `suggest_cheaper(stages: list[str], carrier_map: dict, history_rows: list[dict], budget: float) -> list[str]`
当超预算时，建议可以删减的 stages 以降低成本（返回可安全删减的 stage key 列表，优先删 brainstorm，其次 verify）。
删减建议只推荐 optional stages（brainstorm / verify），不推荐删 plan / implement / review。

## 风格
纯标准库，函数短小，中文 docstring，无 LLM 调用。
