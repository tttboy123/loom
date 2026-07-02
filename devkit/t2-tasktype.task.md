实现一个纯标准库的 Python 模块 `tasktype.py`（Loom 的启发式任务类型分类器）。
只写这一个文件，不要新增依赖，**不要写任何测试代码**（不要 `unittest`、不要 `if __name__ == "__main__"` 测试块；测试由 Eval Gate 提供）。

## 背景（为什么）
这个模块给 schema 字段 `task_type` 提供取值，供路线图里的 "Model-Fitness-by-task"（按任务类型挑模型）使用。
输入是一段（中文或英文）任务描述文本，输出是单一的任务类型字符串。

## 模块要求

### M1：函数签名与返回值集合
```
def infer_task_type(text) -> str
```
对（中文或英文）任务文本做关键词启发式匹配，返回且仅返回以下六个值之一：
`"backend-fix"`、`"test-gen"`、`"review"`、`"refactor"`、`"feature"`、`"other"`。

### M2：规则与优先级（按此顺序检查，**第一个命中即返回**，first match wins）
1. 文本含 `修复` / `fix` / `bug`（英文不区分大小写）中的任意一个 → 返回 `"backend-fix"`
2. 文本含 `测试` / `test` / `golden` 中的任意一个 → 返回 `"test-gen"`
3. 文本含 `审查` / `review` / `评审` 中的任意一个 → 返回 `"review"`
4. 文本含 `重构` / `refactor` 中的任意一个 → 返回 `"refactor"`
5. 文本含 `实现` / `新增` / `add` / `feature` / `build` 中的任意一个 → 返回 `"feature"`
6. 以上都不命中 → 返回 `"other"`

注意优先级：例如 `"fix and add validation"` 同时含 `fix` 和 `add`，但 `fix` 规则在前，应返回 `"backend-fix"`。

### M3：大小写与风格
- 英文关键词匹配**不区分大小写**（`Fix`、`FIX`、`Review`、`REFACTOR` 等都要命中）。中文关键词按原样子串匹配即可。
- 纯标准库，不 import 任何第三方包。
- 函数短小、可读、加简短 docstring。
- 文件第一行用注释写出文件名：`# tasktype.py`
- 模块里**不得包含任何测试代码**（没有 `unittest`、没有测试用的 `if __name__ == "__main__"` 块、没有 `assert` 测试）。
