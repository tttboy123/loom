# Task: P0 Quota Preflight —— `quota_simulate()` + CLI `devkit quota simulate`

## 背景
`devkit/insight.py` 已有 `stage_report()` 和 `quota_report()`。
目标是新增 `quota_simulate(stages, base_url, key, runs_dir=None)` 函数，让用户在跑任务前预判额度是否够用。

## 需要修改的两个文件

### 文件 A：`devkit/insight.py`
在 `quota_report()` 函数之后加一个新函数：

```python
def quota_simulate(stages: list, base_url: str, key: str,
                   runs_dir=None) -> dict:
    """
    预判运行给定 stages 是否有足够额度。
    
    逻辑：
    1. 从 stage_report(runs_dir) 取各阶段的 avg_cost（历史均值）
    2. 对 stages 里每个阶段，取 avg_cost；若无历史数据则为 None
    3. estimated_total = sum(非 None 的 avg_cost)
    4. 从 quota_report(base_url, key) 取 remaining_usd（免费额度剩余）
       和 subscription 状态（订阅=无限=不计费）
    5. 判断 verdict：
       - 有任何 stage 无历史数据 → "Unknown"（信息不足）
       - estimated_total == 0 或所有后端是订阅 → "Safe"（不花钱）
       - estimated_total <= remaining_usd * 0.5 → "Safe"
       - estimated_total <= remaining_usd → "Risky"（会用掉超过 50% 剩余额度）
       - estimated_total > remaining_usd → "Insufficient"
       - remaining_usd is None（无免费额度/仅订阅）→ "Safe"（订阅不限制）
    
    返回：
    {
        "stages": stages,                    # 输入的阶段列表
        "stage_costs": {stage: avg_cost},    # 各阶段预估成本（None = 无历史）
        "estimated_total": float,            # 预估总成本（仅计非 None 的）
        "remaining_usd": float|None,         # 可用免费额度
        "verdict": "Safe"|"Risky"|"Insufficient"|"Unknown",
        "missing_stages": [...]              # 无历史数据的阶段名
    }
    """
```

### 文件 B：`devkit/__main__.py`
在 `_cmd_quota()` 函数里，加 `simulate` 子命令支持：

当 `argv` 第一个参数是 `"simulate"` 时，解析 `<stages>` 参数（逗号分隔，如 `"implement,verify,review"` 或 `"implement"`），调用 `quota_simulate()`，打印结果。

```
用法：devkit quota simulate implement,verify,review
输出示例：
  阶段预估成本：
    implement  $0.00012（历史均值）
    verify     $0.00008（历史均值）
    review     $0.00010（历史均值）
  预估总计：$0.00030
  可用额度：$4.23100（deepseek 免费）
  结论：✅ Safe
```

若某阶段无历史数据，显示 `$?（无历史）`，verdict 为 `Unknown`。

## 约束
- 只修改 `devkit/insight.py` 和 `devkit/__main__.py`
- 只用标准库
- `quota_simulate()` 内部处理所有异常（网关不可达时 remaining_usd=None → verdict=Unknown）
- 不写 unittest 块
- 输出两个代码块：分别以 `# devkit/insight.py` 和 `# devkit/__main__.py` 开头
