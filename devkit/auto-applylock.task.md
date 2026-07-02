实现纯标准库新模块 `applylock.py`（长时自治 Agent 的"自我修改护栏 / 文件锁分类"核心）。
只写这一个文件，不要新增依赖，不要写测试。

## 背景
自治体改 Loom 自己时，绝不能让它自动改掉约束自己的 harness。这些文件必须走人类 apply 门。

## 里程碑（验收点）
- M1：模块级集合 `CRITICAL_BASENAMES = {"rdloop.py", "evals.py", "autoloop.py", "evidence.py", "ratchet.py", "stopcheck.py", "applylock.py"}`
- M2：`def requires_human(path) -> bool` —— 满足任一条返回 True（必须人类 apply），否则 False（可自动 apply）：
  1. 文件名（basename）在 `CRITICAL_BASENAMES` 里
  2. 文件名匹配 `test_*.py`（以 `test_` 开头、`.py` 结尾）
  3. 路径以 `.golden.json` 结尾
  用 `os.path.basename` 取文件名。
- M3：`def classify(path) -> str` —— `requires_human` 为真返回 `"human"`，否则 `"auto"`。

## 风格
纯标准库（可 `import os`），函数短小，加简短中文 docstring，文件第一行 `# applylock.py`。
