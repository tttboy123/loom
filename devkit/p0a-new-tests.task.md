# Task: 为 budget.carrier_max_tokens 和 0文件构建修复添加单元测试

## 背景
`devkit/budget.py` 新增了 `carrier_max_tokens(carrier)` 函数和 `CARRIER_MAX_TOKENS` 字典。
`devkit/rdloop.py` 修复了一个 bug：当 `files=[]`（0 文件构建）时 `tests_failed` 现在是 True。

## 任务
在 `devkit/test_features.py` 末尾追加两个 TestCase 类，为上述两处新增/修复行为写单元测试。

## 你需要添加的内容（只允许追加，不能改动已有代码）

### TestCase 1: BudgetCarrierMaxTokensTest
放在文件末尾，测试 `devkit.budget.carrier_max_tokens`：

- test_reasoning_models_return_8000: 断言 carrier_max_tokens("glm") == 8000, carrier_max_tokens("deepseek") == 8000, carrier_max_tokens("minimax") == 8000
- test_unknown_carrier_returns_none: 断言 carrier_max_tokens("claude") is None, carrier_max_tokens("no-such") is None
- test_stage_max_fallback_logic: 模拟 `getattr(st, "max_tokens", None) or carrier_max_tokens(carrier) or 900` 的三档优先级：① stage 写了 max_tokens=4000 → 用 4000；② stage 无 max_tokens 但 carrier="glm" → 用 8000；③ stage 无 max_tokens 且 carrier="claude" → 用 900（run 级默认）

### TestCase 2: ZeroFileBugTest
放在 TestCase 1 之后，测试 0 文件构建应 tests_failed=True 的逻辑：

- test_zero_files_means_failed: 直接测试 `(tpassed is False) or not files` 的逻辑：当 files=[], tpassed=None → 结果 True；当 files=["a.py"], tpassed=True → 结果 False；当 files=["a.py"], tpassed=False → 结果 True
- test_gate_reasons_include_0file_when_empty: 模拟 reasons 列表：当 files=[] 时 reasons 应包含 0文件相关项，gate 以 NO-GO 开头（用字符串模拟 gate 逻辑 `f"NO-GO（{...}）" if reasons else "建议 GO"`）

## 约束
- 只追加到 test_features.py 末尾，不修改已有代码
- 只用 Python 标准库（unittest, pathlib, tempfile 等）
- 不写任何 `if __name__ == "__main__"` 块（已有）
- 不写任何多余的 print/注释
- 测试必须不依赖真实网关（全部 offline 可跑）
- 追加内容是完整的 TestCase 类代码，以 `\n\nclass BudgetCarrierMaxTokensTest...` 开头
