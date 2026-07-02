实现纯标准库新模块 `evidence.py`（长时自治 Agent 的"默认失败契约 / 物理证据门"核心）。
只写这一个文件，不要新增依赖，不要写测试（测试由 Eval Gate 提供）。

## 背景
模型爱"报喜不报忧"，把半成品当完成品。所以默认一律 NO-GO，**只有拿出真实证据才准翻 GO**。

## 里程碑（验收点）
- M1：`def gate(record) -> dict`
  `record` 是 dict，含这些键：
  - `has_test_output`（bool）：是否真的产生过测试输出
  - `tests_passed`（bool 或 None）：测试是否通过
  - `has_codex_verdict`（bool）：是否有 codex 验证结论
  - `codex_verdict`（str 或 None）：codex 的结论，可能是 "GO"/"NO-GO"/None
- M2：判定规则（**默认失败**）——满足以下任一条才返回 GO，否则一律 NO-GO：
  1. `has_test_output is True` 且 `tests_passed is True`
  2. `has_codex_verdict is True` 且 `codex_verdict == "GO"`
- M3：返回 `{"verdict": "GO" 或 "NO-GO", "reason": <一句话英文/中文说明，如 'tests passed' / 'no evidence' / 'tests failed' / 'codex NO-GO'>}`。
  - 完全没有证据（has_test_output False 且 has_codex_verdict False）时 reason 要体现"no evidence"。

## 风格
纯标准库，函数短小，加简短中文 docstring，文件第一行 `# evidence.py`。
