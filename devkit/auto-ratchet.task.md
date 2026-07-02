实现纯标准库新模块 `ratchet.py`（长时自治 Agent 的"测试棘轮 / Test Ratchet"核心）。
只写这一个文件，不要新增依赖，不要写测试。

## 背景
自治体为了凑"全部通过"，会偷偷删/改失败的测试用例。棘轮 = 测试只增不减，绝不许弱化。

## 里程碑（验收点）
- 辅助：把"raises 用例数"定义为列表里含真值 `raises` 键的用例个数。
- M1：`def is_weakened(old_cases, new_cases) -> bool`
  `old_cases` / `new_cases` 都是 golden 用例 dict 列表。**被弱化**（返回 True）当且仅当：
  `len(new_cases) < len(old_cases)`  或  `新的 raises 用例数 < 旧的 raises 用例数`。否则 False。
- M2：`def check(old_cases, new_cases) -> dict`
  返回 `{"weakened": bool, "old_count": int, "new_count": int, "old_raises": int, "new_raises": int, "reason": str}`。
  `reason` 一句话说明（如 'ok' / 'cases dropped' / 'raises dropped'）。

## 风格
纯标准库，函数短小，加简短中文 docstring，文件第一行 `# ratchet.py`。
