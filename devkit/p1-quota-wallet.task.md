# Task: P1 Quota Wallet — provider_balance() + quota_report() 实时余额接入

## 背景
`devkit/insight.py` 现有 `quota_report()` 的 `remaining_usd` 来自静态 `loom.quota.toml`（`free_usd - used`）。
目标：新增 `provider_balance(backend, provider_key)` 函数，对支持余额 API 的 provider 尝试拉取实时余额，`quota_report()` 优先用实时数据。

## 只修改一个文件：`devkit/insight.py`

### 新增函数 `provider_balance`

在 `quota_report` 函数之前加：

```python
def provider_balance(backend: str, provider_key: str | None) -> dict:
    """尝试从 provider 的余额 API 拉取实时可用余额。
    
    支持：
    - deepseek：GET https://api.deepseek.com/user/balance
      响应形如：{"balance_infos": [{"currency": "CNY", "total_balance": "X", "granted_balance": "Y", "topped_up_balance": "Z"}]}
      将 granted_balance (免费赠送) + topped_up_balance (充值) 的 CNY 数相加，
      粗略换算为 USD（1 USD ≈ 7.2 CNY），保留 5 位小数。
    - 其余 backend：不支持，返回 available_usd=None，source="unsupported"
    
    若 provider_key 为 None/空，返回 available_usd=None，source="no_key"
    若网络失败/解析失败，返回 available_usd=None，source="error"
    
    返回：
    {
        "backend": str,
        "available_usd": float | None,
        "available_cny": float | None,   # 原始 CNY 金额（仅 deepseek）
        "source": "api" | "no_key" | "unsupported" | "error",
    }
    """
```

### 修改 `quota_report`

在 `quota_report()` 里，对每个 backend，当 `cfg.get(b, {}).get("provider_key")` 存在时，
调用 `provider_balance(b, provider_key)` 拿到 `available_usd`；
若 `available_usd is not None`，则用它覆盖 `remaining_usd`（而非静态 `free_usd - used`），
并在该行的 `note` 字段追加 `"[实时]"` 标记。

即：
```python
# 在 for b in BACKEND_ORDER 循环里
c = cfg.get(b, {}) if isinstance(cfg, dict) else {}
provider_key = c.get("provider_key")  # 用户在 loom.quota.toml 里填的 provider 原始 API key
if provider_key:
    pb = provider_balance(b, provider_key)
    if pb["available_usd"] is not None:
        # 用实时余额覆盖静态计算
        kind = "免费额度"
        remaining = pb["available_usd"]
        free = pb["available_usd"] + float(spend.get(b, {}).get("spend", 0.0))  # 估算 free_usd
        pct = round(min(float(spend.get(b, {}).get("spend", 0.0)) / free * 100, 100), 1) if free > 0 else 0.0
        note = c.get("note", "") + " [实时]"
        rows.append(...)
        continue  # 跳过下方的静态计算分支
# 原有静态计算保持不变
```

## 约束
- 只修改 `devkit/insight.py`
- 只用标准库（urllib.request、json）
- 所有异常在 `provider_balance` 内部捕获，绝不往外抛
- 不写 unittest 块
- 输出一个代码块，以 `# devkit/insight.py` 开头，产出完整文件

## 关键数据结构（不要弄错）
- `stage_report()` 返回 `{"rows": [...], "totals": {...}}` —— 不是 `{"stages": {...}}`
- `quota_report()` 返回 `{"rows": [...], "recommend": ..., "gateway_ok": ..., "configured": ...}` —— 顶层没有 remaining_usd
- `provider_balance` 是新增函数，不影响上述返回结构
