# devkit/executors.py
"""
可插拔执行器（Executor）—— 把某个阶段交给不同的「执行后端」跑。

- chat     ：默认。经 LiteLLM 网关做一次扁平对话（见 rdloop.gateway_chat）。
- hermes   ：Nous Hermes Agent（带工具的 agent，`hermes -z` 一次性模式）。
- openclaw ：OpenClaw agent（同类，开源 github.com/openclaw/openclaw）。
- codex    ：Codex 验证执行器，经 LiteLLM 网关 codex-sub 载体做验证。
- codex-runner ：Codex 真执行器 — 在 sandbox 真跑 pytest/ruff（DESIGN-P0 P0-b）。
- opencode ：OpenCode CLI。

设计原则（对齐 Constitution）：
- agentic 执行器在**隔离 sandbox 目录**里跑（每阶段一个），不碰真实仓库；
  apply（把 sandbox 产物落进真实仓库）是**人类门**，本套件不自动做。
- agentic 执行器复用 Loom 网关 → 用 loom 载体（统一计费/降级），不污染用户自身的 CLI 配置（仅注入临时 env）。
- 缺失的执行器（如未安装 openclaw）**优雅降级**：返回清晰的「未就绪 + 如何启用」，不崩。
"""
from __future__ import annotations

import json
import os
import pathlib
import shutil
import subprocess
import sys
import urllib.request
from typing import Tuple

from devkit.model_aliases import normalize_model_name

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
    """各执行器是否可用（chat 永远可用；hermes/openclaw/opencode 看是否在 PATH）。"""
    return {
        "chat": True,
        "hermes": shutil.which("hermes") is not None,
        "openclaw": shutil.which("openclaw") is not None,
        "codex": True,  # codex 无需本地二进制，依赖网关
        "opencode": shutil.which("opencode") is not None,
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


# ----------------------------------------------------------------------------
# codex_runner — DESIGN-P0 P0-b "codex executor" (real sandbox execution)
#
# Unlike ``run_codex`` (which talks to a chat-only codex-sub model), this
# executor ACTUALLY RUNS the code in a sandbox: pytest, ruff, and emits a
# structured VerifyReport. Designed to be the "verify" stage's default.
#
# Behavior:
#   1. If `codex` CLI is on PATH, run it non-interactively
#   2. Otherwise, fall back to `pytest -q --tb=short` + `ruff check` (if installed)
#   3. Parse output into VerifyReport (verdict, tests_passed, failing, repro)
#
# Returns: (ok, output, executor_name, verify_report_dict)
# ----------------------------------------------------------------------------
def _parse_pytest_output(out: str) -> dict:
    """Parse pytest -q output to extract pass/fail counts and failing tests.

    Recognizes lines like:
      '5 passed in 0.12s'
      '3 passed, 2 failed in 1.23s'
      '1 failed, 2 passed, 1 error in 2.5s'
    """
    import re as _re
    report = {
        "verdict": "UNKNOWN",
        "tests_passed": None,
        "tests_failed": 0,
        "tests_collected": 0,
        "failing": [],
        "repro": out[:1500],
    }
    # Look for the summary line (last "X passed" or "X failed" line)
    summary_re = _re.compile(
        r"(?:(\d+)\s+failed)?[,\s]*(?:(\d+)\s+passed)?[,\s]*(?:(\d+)\s+error)?[,\s]*in\s+[\d.]+s",
        _re.IGNORECASE,
    )
    for line in out.splitlines()[::-1]:  # search from end
        line = line.strip()
        m = summary_re.search(line)
        if m:
            failed = int(m.group(1) or 0)
            passed = int(m.group(2) or 0)
            errored = int(m.group(3) or 0)
            report["tests_failed"] = failed + errored
            report["tests_passed"] = (passed > 0 and failed == 0 and errored == 0)
            report["tests_collected"] = passed + failed + errored
            if report["tests_passed"]:
                report["verdict"] = "GO"
            elif failed > 0 or errored > 0:
                report["verdict"] = "NO-GO"
            break
    # Capture FAILURE / ERROR lines (for repro context)
    fail_re = _re.compile(r"^FAILED\s+(\S+?)(?:\s*-\s*(.+))?$", _re.MULTILINE)
    err_re = _re.compile(r"^ERROR\s+(\S+?)(?:\s*-\s*(.+))?$", _re.MULTILINE)
    for m in fail_re.finditer(out):
        report["failing"].append({"name": m.group(1), "reason": (m.group(2) or "").strip()})
    for m in err_re.finditer(out):
        report["failing"].append({"name": m.group(1), "reason": (m.group(2) or "").strip()})
    return report


def run_codex_runner(sandbox: pathlib.Path, *, timeout: int = 300) -> Result:
    """Run verification in sandbox. Prefers `codex` CLI; falls back to pytest+ruff.

    Returns: (ok, output_text, "codex-runner")

    A False return here means "we could not even attempt verification"
    (no executor on PATH). A True return means we ran something; callers
    parse the output for verdict.
    """
    pytest_bin = shutil.which("pytest")
    codex_bin = shutil.which("codex")
    ruff_bin = shutil.which("ruff")

    # 1. Try `codex` CLI first (it's the "designed" executor)
    codex_attempted = False
    if codex_bin:
        codex_attempted = True
        # Conservative invocation — Codex CLI variants differ
        cmd = [codex_bin, "verify", "--json", "."]
        code, out = _run(cmd, cwd=sandbox, env=os.environ.copy(), timeout=timeout)
        if code == 0:
            return True, f"[codex CLI] {out[:1500]}", "codex-runner"
        # else fall through to pytest

    # 2. Fallback: pytest (required) + ruff (optional)
    if not pytest_bin:
        if codex_attempted:
            return False, ("codex CLI failed and `pytest` is not on PATH; "
                           "install pytest or fix the codex invocation to enable verify."), "codex-runner"
        return False, ("no `pytest` and no `codex` CLI on PATH; "
                       "cannot verify."), "codex-runner"

    parts: list[str] = []
    code, out = _run(
        ["pytest", "-q", "--tb=short", "--no-header"],
        cwd=sandbox, env=os.environ.copy(), timeout=timeout,
    )
    parts.append(f"[pytest rc={code}] {out[:1500]}")

    if ruff_bin:
        code, out = _run(
            ["ruff", "check", "."],
            cwd=sandbox, env=os.environ.copy(), timeout=60,
        )
        parts.append(f"[ruff rc={code}] {out[:500]}")

    output = "\n".join(parts)
    return True, output, "codex-runner"

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

def _parse_verify_report(content: str) -> dict:
    """将 codex 载体返回内容解析为结构化验证报告。

    优先级：
    1) NO-GO / FAIL / ❌ → verdict="NO-GO", tests_passed=False
    2) GO / PASS / ✅（且未命中 1）→ verdict="GO", tests_passed=True
    3) 其他 → verdict="UNKNOWN", tests_passed=None
    """
    import re as _re
    low = content.lower()
    # 1) 失败标记（优先）
    if any(marker in low for marker in ("no-go", "fail", "❌")):
        return {"verdict": "NO-GO", "tests_passed": False, "summary": content[:500]}
    # 2) 成功标记（"go" 用词边界避免误匹配 ago/logo 等）
    if "✅" in content or "pass" in low or _re.search(r'\bgo\b', low):
        return {"verdict": "GO", "tests_passed": True, "summary": content[:500]}
    # 3) 未识别
    return {"verdict": "UNKNOWN", "tests_passed": None, "summary": content[:500]}

def run_codex(prompt: str, model: str, sandbox: pathlib.Path,
              gateway: str, api_key: str, timeout: int = 60, os_sandbox: bool = False) -> Result:
    """通过 LiteLLM 网关调用 codex-sub 载体做代码验证。

    - 发送 prompt 到 {gateway}/v1/chat/completions
    - 模型优先使用传入的 model，若为空或以 loom-* 开头则回退到 "codex-sub"
    - system 角色预设为代码验证助手
    - 返回 (成功与否, 解析后的 content, "codex")
    """
    # 模型回退
    effective_model = normalize_model_name(model)
    if not effective_model or effective_model.startswith("loom-"):
        effective_model = "codex-sub"

    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}",
    }
    payload = {
        "model": effective_model,
        "messages": [
            {"role": "system", "content": "你是代码验证助手。分析给定的实现和测试输出，判断是否通过验证。"},
            {"role": "user", "content": prompt},
        ],
        "max_tokens": 2000,
    }

    try:
        req = urllib.request.Request(
            f"{gateway.rstrip('/')}/v1/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers=headers,
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        return False, f"{type(e).__name__}: {e}"[:400], "codex"

    content = body.get("choices", [{}])[0].get("message", {}).get("content", "")
    # 用 _parse_verify_report 提取结构化信息，但保留原始字符串作为 output_text
    report = _parse_verify_report(content)
    # 返回值保持 (bool, str, str)：成功调用网关即为 True
    return True, content[:8000], "codex"

def run_opencode(prompt: str, sandbox: pathlib.Path, timeout: int = 120) -> Result:
    """通过子进程调用 opencode CLI 执行编码任务。

    命令：opencode run --non-interactive --print-output "<prompt>"
    成功：(True, stdout[:8000], "opencode")
    失败：(False, stderr[:400] 或错误说明, "opencode")
    """
    try:
        result = subprocess.run(
            ["opencode", "run", "--non-interactive", "--print-output", prompt],
            cwd=str(sandbox),
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        if result.returncode == 0:
            return True, result.stdout[:8000], "opencode"
        return False, (result.stderr or result.stdout)[:400], "opencode"
    except FileNotFoundError:
        return False, "opencode 未安装（brew install opencode 或 npm i -g opencode）", "opencode"
    except subprocess.TimeoutExpired:
        return False, f"opencode 超时（{timeout}s）", "opencode"
    except Exception as e:  # noqa: BLE001
        return False, f"{type(e).__name__}: {e}"[:400], "opencode"


def run(executor: str, prompt: str, model: str, sandbox: pathlib.Path,
        gateway: str, api_key: str, timeout: int = 300, os_sandbox: bool = False) -> Result:
    """统一入口：按名字派发到对应执行器（chat 由 rdloop 直接处理，不走这里）。"""
    if executor == "hermes":
        return run_hermes(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
    if executor == "openclaw":
        return run_openclaw(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
    if executor == "codex":
        return run_codex(prompt, model, sandbox, gateway, api_key, timeout, os_sandbox)
    if executor == "codex-runner":
        return run_codex_runner(sandbox, timeout=timeout)
    if executor == "opencode":
        return run_opencode(prompt, sandbox, timeout=timeout)
    return False, f"未知执行器：{executor}（可选 chat / hermes / openclaw / codex / codex-runner / opencode）", executor
