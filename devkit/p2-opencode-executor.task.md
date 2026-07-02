# Task: P2 OpenCode Executor — `executor=opencode` 接入

## 背景
`devkit/executors.py` 已有 `chat / hermes / openclaw / codex` 四种执行器。
目标：新增 `opencode` executor stub，让 `--executor implement=opencode` 能工作。

OpenCode 是一个 CLI 编码工具（https://opencode.ai），通过子进程调用：
```
opencode run --non-interactive --print-output "<prompt>"
```
若命令不存在（未安装），应优雅降级返回 `(False, "opencode 未安装（brew install opencode 或 npm i -g opencode）", "opencode")`

## 只修改一个文件：`devkit/executors.py`

### 新增 `run_opencode` 函数

在 `run_codex` 函数之后加：

```python
def run_opencode(prompt: str, sandbox: pathlib.Path,
                 timeout: int = 120) -> Result:
    """通过子进程调用 opencode CLI 执行编码任务。
    
    命令：opencode run --non-interactive --print-output "<prompt>"
    sandbox：opencode 的工作目录（cwd）
    
    成功：(True, stdout[:8000], "opencode")
    失败：(False, stderr[:400] 或 "opencode 未安装...", "opencode")
    
    若 opencode 未安装（FileNotFoundError）：
        返回 (False, "opencode 未安装（brew install opencode 或 npm i -g opencode）", "opencode")
    若超时（subprocess.TimeoutExpired）：
        返回 (False, f"opencode 超时（{timeout}s）", "opencode")
    """
```

实现要求：
- 用 `subprocess.run(["opencode", "run", "--non-interactive", "--print-output", prompt], ...)`
- `cwd=sandbox`，`capture_output=True`，`text=True`，`timeout=timeout`
- 成功：returncode==0 → `(True, result.stdout[:8000], "opencode")`
- 非零：`(False, (result.stderr or result.stdout)[:400], "opencode")`
- FileNotFoundError → 优雅提示
- TimeoutExpired → 超时提示
- 其他 Exception → `(False, f"{type(e).__name__}: {e}"[:400], "opencode")`

### 修改 `run` 函数

在现有 `if executor == "codex":` 分支之后加：
```python
if executor == "opencode":
    return run_opencode(prompt, sandbox, timeout=timeout)
```

更新最后的 `return False, f"未知执行器..."` 那行，把 `opencode` 加进去：
```python
return False, f"未知执行器：{executor}（可选 chat / hermes / openclaw / codex / opencode）", executor
```

### 修改 `devkit/roles.py`

在 `EXECUTORS` tuple 里加 `"opencode"`：
```python
EXECUTORS = ("chat", "hermes", "openclaw", "codex", "opencode")
```

## 约束
- 修改 `devkit/executors.py` 和 `devkit/roles.py`
- 只用标准库（subprocess、pathlib）
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/executors.py` 和 `# devkit/roles.py` 开头，产出完整文件
