实现纯标准库新模块 `stopcheck.py`（长时自治 Agent 的"死循环检测 / AGENT_STOP"核心）。
只写这一个文件，不要新增依赖，不要写测试。

## 背景
自治体可能卡在同一个错误上反复空烧 token。检测到"同一错误连续重复"就该停下挂起等人。

## 里程碑（验收点）
- M1：`def should_stop(signatures, max_repeats=2) -> dict`
  `signatures` 是字符串列表，每轮一个：非空串=该轮的错误/失败签名，空串 `""`=该轮通过（不是错误）。
- M2：判定——当**列表末尾连续 `max_repeats` 个签名全部相等且非空**时，停（返回 stop=True）；否则不停。
  - 例：`["e1","e1"]` → stop（末尾 2 个都是 "e1"）；`["e1","e2"]` → 不停；
    `["a","e1","e1"]` → stop；`["e1","e1","e2"]` → 不停（末尾是 e1,e2 不等）；
    `[]` → 不停；`["",""]` → 不停（空串是通过，不算错误）。
- M3：返回 `{"stop": bool, "reason": str}`，reason 一句话（如 'same error repeated' / 'ok'）。

## 风格
纯标准库，函数短小，加简短中文 docstring，文件第一行 `# stopcheck.py`。
