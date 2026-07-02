# Task: P0-b 在 devkit/executors.py 新增 "codex" 执行器分支

## 目标
在 `devkit/executors.py` 里新增：
1. 内部函数 `_parse_verify_report(content: str) -> dict`
2. 函数 `run_codex(...)`（调 LiteLLM 网关 codex-sub 载体做验证）
3. 在 `run()` 的 executor 分支加 `"codex"` 入口

**产物**：输出完整的 `executors.py` 文件（代码块标注 `# devkit/executors.py`）。

---

## 现有 executors.py 全文（不要删改已有内容，只增加）

```python
# devkit/executors.py
"""
可插拔执行器（Executor）—— 把某个阶段交给不同的「执行后端」跑。

- chat     ：默认。经 LiteLLM 网关做一次扁平对话（见 rdloop.gateway_chat）。
- hermes   ：Nous Hermes Agent（带工具的 agent，`hermes -z` 一次性模式）。
- openclaw ：OpenClaw agent（同类，开源 github.com/openclaw/openclaw）。

两者可在**一次运行里按阶段同时启用**：
    --executor implement=hermes --executor verify=chat --executor review=openclaw

设计原则（对齐 Constitution）：
- agentic 执行器在**隔离 sandbox 目录**里跑（每阶段一个），不碰真实仓库；
  apply（把 sandbox 产物落进真实仓库）是**人类门**，本套件不自动做。
- agentic 执行器复用 Loom 网关 → 用 loom 载体（统一计费/降级），不污染用户自身的 CLI 配置（仅注入临时 env）。
- 缺失的执行器（如未安装 openclaw）**优雅降级**：返回清晰的「未就绪 + 如何启用」，不崩。
"""
from __future__ import annotations

import os
import pathlib
import shutil
import subprocess
import sys
from typing import Tuple

ROOT = pathlib.Path(__file__).resolve().parent.parent  # agent-platform/

# 返回：(ok, output_text, executor_name)
Result = Tuple[bool, str, str]

# hermes provider 前缀 -> (我们 .env 里的 key 名, hermes 期望的 env 名)
_HERMES_PROVIDER_KEY = {
    "deepseek": ("DEEPSEEK_API_KEY", "DEEPSEEK_API_KEY"),
    "glm": ("ZHIPU_API_KEY", "GLM_API_KEY"),
    "minimax": ("MINIMAX_API_KEY", "MINIMAX_API_KEY"),
}


def _env_val(name: str) -> str:
    """从 agent-platform/.env 读一个变量（hermes 原生 provider 复用我们已填的 key）。"""
    if os.environ.get(name):
        return os.environ[name]
    f = ROOT / ".env"
    if f.exists():
        for line in f.read_text().splitlines():
            if line.startswith(name + "="):
                return line.split("=", 1)[1].strip()
    return ""


def available() -> dict:
    """各执行器是否可用（chat 永远可用；hermes/openclaw 看是否在 PATH）。"""
    return {
        "chat": True,
        "hermes": shutil.which("hermes") is not None,
        "openclaw": shutil.which("openclaw") is not None,
    }


def sandbox_dir(run_dir: pathlib.Path, stage: str) -> pathlib.Path:
    d = pathlib.Path(run_dir) / f"sandbox-{stage}"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _sandbox_wrap(cmd: list, sandbox_dir: pathlib.Path) -> Tuple[list, bool]:
    """macOS sandbox-exec：默认放行，但**写盘只许沙箱 + agent 状态目录 + tmp**（限制 blast radius）。
    非 macOS 或无 sandbox-exec 时原样返回（不阻塞）。"""
    if sys.platform != "darwin" or not shutil.which("sandbox-exec"):
        return cmd, False
    home = os.path.expanduser("~")
    sd = str(pathlib.Path(sandbox_dir).resolve())
    allow = "\n  ".join(f'(subpath "{p}")' for p in [
        sd, f"{home}/.hermes", f"{home}/.openclaw", f"{home}/.npm",
        f"{home}/.cache", f"{home}/.config", "/private/tmp", "/private/var/folders",
        "/tmp", "/dev"])
    profile = f'(version 1)\n(allow default)\n(deny file-write*)\n(allow file-write*\n  {allow})'
    return ["sandbox-exec", "-p", profile] + cmd, True


def _run(cmd, cwd, env, timeout) -> Tuple[int, str]:
    try:
        r = subprocess.run(cmd, cwd=str(cwd), env=env,
                           capture_output=True, text=True, timeout=timeout)
        out = (r.stdout or "").strip() or (r.stderr or "").strip()
        return r.returncode, out
    except subprocess.TimeoutExpired:
        return 124, f"（执行超时 > {timeout}s）"
    except FileNotFoundError:
        return 127, "（命令未找到）"
    except Exception as e:  # noqa: BLE001
        return 1, f"（调用异常：{type(e).__name__}: {e}）"


def run_hermes(prompt: str, model: str, sandbox: pathlib.Path,
               gateway: str, api_key: str, timeout: int = 300, os_sandbox: bool = False) -> Result:
    """用 Hermes Agent 跑（带工具、在 sandbox 里），走 hermes **原生 provider**。"""
    if not shutil.which("hermes"):
        return False, "hermes 未安装 —— 适配器已就绪，装好（Nous Hermes Agent）即用。", "hermes"
    hmodel = model if ("/" in model and not model.startswith("loom-")) else "deepseek/deepseek-chat"
    provider = hmodel.split("/", 1)[0]
    env = dict(os.environ)
    if provider in _HERMES_PROVIDER_KEY:
        ours, hname = _HERMES_PROVIDER_KEY[provider]
        v = _env_val(ours)
        if v:
            env[hname] = v
        else:
            return False, f"hermes：缺 {provider} 的 key（在 agent-platform/.env 填 {ours}）。", "hermes"
    cmd = ["hermes", "-z", prompt, "-m", hmodel, "--cli", "--yolo"]
    if os_sandbox:
        cmd, _ = _sandbox_wrap(cmd, sandbox)
    code, out = _run(cmd, sandbox, env, timeout)
    bad = (code != 0) or (not out) or ("no final response" in out) or ("agent failed" in out.lower())
    if bad:
        return False, f"hermes 未产出终稿（{hmodel}）。原文：{out[:400]}", "hermes"
    return True, out[:8000], "hermes"


def run_openclaw(prompt: str, model: str, sandbox: pathlib.Path,
                 gateway: str, api_key: str, timeout: int = 300, os_sandbox: bool = False) -> Result:
    """用 OpenClaw 的 `agent --local` 一次性跑（与 hermes 同源，原生 provider + 我们的 key）。"""
    if not shutil.which("openclaw"):
        return False, ("openclaw 未安装 —— 适配器已就绪。`npm i -g openclaw` 即可启用"
                       "（github.com/openclaw/openclaw，与 hermes 同源）。"), "openclaw"
    omodel = model if ("/" in model and not model.startswith("loom-")) else "deepseek/deepseek-chat"
    provider = omodel.split("/", 1)[0]
    env = dict(os.environ)
    if provider in _HERMES_PROVIDER_KEY:
        ours, hname = _HERMES_PROVIDER_KEY[provider]
        v = _env_val(ours)
        if v:
            env[hname] = v
    cmd = ["openclaw", "agent", "--local", "--session-key", "loom-devkit",
           "-m", omodel, "--message", prompt]
    if os_sandbox:
        cmd, _ = _sandbox_wrap(cmd, sandbox)
    code, out = _run(cmd, sandbox, env, timeout)
    low = out.lower()
    if "providerautherror" in low or "no api key" in low or "missing-provider-auth" in low:
        return False, ("openclaw 已安装/接线，但其 --local 用自有 auth store（非 env key）。"
                       "先跑一次 `openclaw onboard`/登录授权一个 provider，即可启用。"), "openclaw"
    if code != 0 or not out:
        return False, f"openclaw 未产出（退出码 {code}，模型 {omodel}）。原文：{out[:400]}", "openclaw"
    return True, out[:8000], "openclaw"


def run(executor: str, prompt: str, model: str, sandbox: pathlib.Path,
        gateway: str, api_key: str, timeout: int = 300, os_sandbox: bool = False) -> Result:
    """统一入口：按名字派发到对应执行器（chat 由 rdloop 直接处理，不走这里）。"""
    if executor == "hermes":
        return run_hermes(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
    if executor == "openclaw":
        return run_openclaw(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
    return False, f"未知执行器：{executor}（可选 chat / hermes / openclaw）", executor
```

---

## 需要新增的内容

### A. `_parse_verify_report(content: str) -> dict`
放在 `run_openclaw` 之后、`run()` 之前。

判断规则（严格按此顺序）：
1. 先查 NO-GO / FAIL / ❌ → verdict="NO-GO", tests_passed=False
2. 再查 GO / PASS / ✅（且步骤1未触发）→ verdict="GO", tests_passed=True
3. 否则 → verdict="UNKNOWN", tests_passed=None

返回：
```python
{"verdict": str, "tests_passed": bool|None, "summary": content[:500]}
```

### B. `run_codex(...) -> Result`
放在 `_parse_verify_report` 之后、`run()` 之前。

签名：
```python
def run_codex(prompt: str, model: str, sandbox: pathlib.Path,
              gateway: str, api_key: str, timeout: int = 60, os_sandbox: bool = False) -> Result:
```

实现：
- 用 `json` + `urllib.request` 调 `{gateway}/v1/chat/completions`
- model 用传入的 `model`，fallback 到 `"codex-sub"` 若 model 为空或 loom-*
- system: `"你是代码验证助手。分析给定的实现和测试输出，判断是否通过验证。"`
- user: `prompt`
- max_tokens: 2000
- timeout: 参数 timeout
- 成功 → 提取 choices[0].message.content，调 `_parse_verify_report`，返回 `(True, content, "codex")`
- 失败/异常 → `(False, str(e)[:400], "codex")`
- 不在 meta 字典里塞 verify_report（保持 Result=(bool,str,str) 签名一致）

### C. 更新 `run()` 末尾
在 `if executor == "openclaw":` 之后、`return False, ...` 之前加：
```python
    if executor == "codex":
        return run_codex(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
```
同时更新最后的错误信息：`（可选 chat / hermes / openclaw / codex）`

---

## 约束
- 输出完整 `executors.py`（文件头注释 `# devkit/executors.py`）
- 只用标准库（json, urllib.request）
- 不写 unittest 块，不改已有函数
- 错误时返回 (False, error_str, "codex")，不抛异常
