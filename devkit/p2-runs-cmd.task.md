# Task: P2 Task Command Center v1 — `devkit runs` 子命令

## 背景
`devkit/runs/` 目录下每次 R&D loop 跑后留有：
- `00-task.md` — 任务描述
- `run-log.md` — 各阶段汇总表 + Gate 建议
- `{i:02d}-{stage}.artifact.json` — per-stage 结构化产物（carrier/task_type/tokens/cost/verdict 等）
- `build/` — 物化代码文件

目标：在 `devkit/__main__.py` 新增 `devkit runs` 子命令，提供任务证据链视图。

## 需要修改的两个文件

### 文件 A：`devkit/__main__.py`

在 `main()` 里加路由（在 `if argv and argv[0] == "fitness":` 之后）：
```python
if argv and argv[0] == "runs":
    return _cmd_runs(argv[1:])
```

新增 `_cmd_runs(argv) -> int` 函数（加在 `_cmd_fitness` 之前）：

**无参数（列表模式）**：`devkit runs`
- 扫 `devkit/runs/` 下所有子目录，按时间逆序
- 读每个 `run-log.md` 提取：Gate 一行（`建议 GO` 或 `NO-GO`）、总 tokens、总$
- 读每个 `00-task.md` 取任务描述前 50 字
- 读任意一个 `.artifact.json` 取 `task_type`
- 输出表格，最多显示 20 行：
  ```
  run_id               gate            tokens     $       task_type  任务
  -------------------- --------------- ---------- ------- ---------- -------
  20260627-154230      建议 GO         2242       $0.000  feature    实现一个 Python 函数 word_count...
  20260627-150001      NO-GO(...)      6344       $0.000  feature    实现一个 Python 函数 add...
  ```

**带 run-id 参数（详情模式）**：`devkit runs <run-id>`
- 读该 run 目录的所有 `*.artifact.json`（按文件名排序）
- 展示每个阶段：stage / carrier / task_type / tokens / cost / verdict / tests_passed
- 若 `build/` 目录存在，列出物化文件名
- 格式：
  ```
  Run: 20260627-154230
  任务：实现一个 Python 函数 word_count...
  Gate：建议 GO（需人类最终确认）

  阶段      载体              tokens  $        verdict    tests_passed
  --------- ----------------- ------- -------- ---------- ------------
  implement deepseek          2242    $0.00000 GO         True
  verify    codex-sub         0       $0.00000 GO         True

  物化文件：devkit/word_count.py
  ```

### 文件 B：`devkit/insight.py`

新增辅助函数 `runs_list(runs_dir=None) -> list`（在 `model_fitness` 之后）：
```python
def runs_list(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> list:
    """返回 runs 列表（按时间逆序），每项：
    {"run_id", "gate", "tokens", "cost", "task_type", "task_snippet", "artifact_files"}
    缺文件的项自动跳过，不抛异常。"""
```

实现要点：
- `run-log.md` 里 Gate 行形如 `## Gate 建议\n\n建议 GO...` 或 `NO-GO(...)`，取该行
- tokens / cost 从 `## 用量合计` 那一行解析：形如 `**2242 tokens · $0.00000**`
- task_type 从第一个找到的 `*.artifact.json` 的 `task_type` 字段取
- task_snippet 从 `00-task.md` 取前 50 个非空字符（去掉 `# 任务` 标题行）
- artifact_files：该 run dir 下所有 `*.artifact.json` 路径列表

## 约束
- 只修改 `devkit/insight.py` 和 `devkit/__main__.py`
- 只用标准库
- 所有文件读取操作用异常兜底，缺文件/解析失败不抛出
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/insight.py` 和 `# devkit/__main__.py` 开头，产出完整文件
