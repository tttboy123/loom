# Task: 为 executors._parse_verify_report 和 codex 分支写单元测试

## 背景
`devkit/executors.py` 新增了：
- `_parse_verify_report(content) -> dict` — 内部函数
- `run_codex(...)` — 调网关，可通过 mock
- `run()` 新增 "codex" 分支

## 任务
在 `devkit/test_features.py` **末尾** 追加一个 TestCase 类 `CodexExecutorTest`。

## 需要测试的内容

```python
class CodexExecutorTest(unittest.TestCase):
    """codex executor 和 _parse_verify_report 的单元测试。"""

    def test_parse_go(self):
        # "GO" 关键词 → verdict GO, tests_passed True
        ...

    def test_parse_nogo(self):
        # "NO-GO" 或 "❌" → verdict NO-GO, tests_passed False

    def test_parse_fail(self):
        # "fail" → verdict NO-GO

    def test_parse_unknown(self):
        # 无任何关键词 → verdict UNKNOWN, tests_passed None

    def test_parse_nogo_overrides_go(self):
        # 同时含 "PASS" 和 "NO-GO" → NO-GO 优先

    def test_parse_pass(self):
        # "PASS" → GO

    def test_parse_emoji_pass(self):
        # "✅" → GO

    def test_run_dispatches_codex(self):
        # run("codex", ...) 不走 hermes/openclaw 分支
        # 用 unittest.mock.patch 打桩 run_codex，验证 run() 确实调了它
        from unittest.mock import patch
        from devkit.executors import run
        import pathlib
        with patch("devkit.executors.run_codex", return_value=(True, "GO", "codex")) as mock_rc:
            ok, content, name = run("codex", "prompt", "codex-sub",
                                    pathlib.Path("/tmp"), "http://localhost:4000", "key")
        mock_rc.assert_called_once()
        self.assertTrue(ok)
        self.assertEqual(name, "codex")

    def test_run_unknown_executor(self):
        # run("unknown", ...) 返回 (False, ..., "unknown")
        from devkit.executors import run
        import pathlib
        ok, msg, name = run("unknown", "p", "m", pathlib.Path("/tmp"), "gw", "k")
        self.assertFalse(ok)
        self.assertIn("未知", msg)
```

## 约束
- 只追加到 `devkit/test_features.py` 末尾（在现有 `if __name__ == "__main__":` **之前**）
- 用 `from devkit.executors import _parse_verify_report` 导入（带 devkit. 前缀）
- 所有测试 offline（不发真实 HTTP）
- 不写额外 import 块（unittest 已在文件顶部）
- 不改已有代码
- 输出格式：一个完整的 Python 代码块，以 `class CodexExecutorTest(unittest.TestCase):` 开头
