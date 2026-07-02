"""devkit/setup.py — Loom 一键设置向导。

纯标准库，零第三方依赖。
"""
from __future__ import annotations

import os
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Optional

DEFAULT_ENV_PATH = str(Path(__file__).parent.parent / "agent-platform" / ".env")

_ENV_CANDIDATES = (
    os.environ.get("LOOM_ENV_FILE", ""),
    str(Path(__file__).parent.parent / "agent-platform" / ".env"),
    ".env",
    ".env.local",
)


# ── Docker ────────────────────────────────────────────────────────────────────

def detect_docker() -> bool:
    try:
        r = subprocess.run(["docker", "info"], capture_output=True, timeout=10)
        return r.returncode == 0
    except Exception:
        return False


# ── Key detection ─────────────────────────────────────────────────────────────

def _parse_env_line(line: str) -> Optional[tuple[str, str]]:
    s = line.strip()
    if not s or s.startswith("#") or "=" not in s:
        return None
    k, v = s.split("=", 1)
    k = k.strip()
    v = v.strip().strip('"').strip("'")
    return (k, v) if k else None


def detect_env_key(key_name: str) -> str:
    v = os.environ.get(key_name)
    if v:
        return v
    for candidate in _ENV_CANDIDATES:
        if not candidate:
            continue
        p = Path(candidate)
        if not p.is_file():
            continue
        try:
            for line in p.read_text(encoding="utf-8").splitlines():
                parsed = _parse_env_line(line)
                if parsed and parsed[0] == key_name:
                    return parsed[1]
        except OSError:
            continue
    return ""


# ── .env write ────────────────────────────────────────────────────────────────

def write_env_file(keys: dict, env_path: str | None = None) -> None:
    if env_path is None:
        env_path = DEFAULT_ENV_PATH
    path = Path(env_path)
    path.parent.mkdir(parents=True, exist_ok=True)

    existing: list[str] = []
    if path.is_file():
        existing = path.read_text(encoding="utf-8").splitlines()

    updated: dict[str, bool] = {k: False for k in keys}
    new_lines: list[str] = []
    for line in existing:
        parsed = _parse_env_line(line)
        if parsed and parsed[0] in keys:
            new_lines.append(f"{parsed[0]}={keys[parsed[0]]}")
            updated[parsed[0]] = True
        else:
            new_lines.append(line)

    for k, v in keys.items():
        if not updated[k]:
            new_lines.append(f"{k}={v}")

    path.write_text("\n".join(new_lines) + "\n", encoding="utf-8")


# ── LiteLLM ───────────────────────────────────────────────────────────────────

def start_litellm(compose_file: str | None = None) -> dict:
    if not detect_docker():
        return {"ok": False, "output": "Docker 不可用"}
    cmd = ["docker", "compose"]
    if compose_file:
        cmd += ["-f", compose_file]
    cmd += ["up", "-d", "litellm"]
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
        ok = r.returncode == 0
        output = (r.stdout + r.stderr).strip()
        return {"ok": ok, "output": output}
    except subprocess.TimeoutExpired:
        return {"ok": False, "output": "超时"}
    except Exception as e:
        return {"ok": False, "output": str(e)}


# ── Gateway check ─────────────────────────────────────────────────────────────

def check_gateway(base_url: str = "http://localhost:4000", timeout: int = 10) -> dict:
    url = base_url.rstrip("/") + "/health"
    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            latency_ms = (time.monotonic() - t0) * 1000
            return {"ok": resp.status < 400, "latency_ms": round(latency_ms, 2), "error": ""}
    except Exception as e:
        return {"ok": False, "latency_ms": 0.0, "error": str(e)}


# ── Main setup flow ───────────────────────────────────────────────────────────

_DEFAULT_KEY_NAMES = (
    "MINIMAX_API_KEY",
    "ANTHROPIC_API_KEY",
    "ZHIPU_API_KEY",
    "DEEPSEEK_API_KEY",
)


def run_setup(interactive: bool = True, keys: dict | None = None) -> dict:
    steps: list[dict] = []
    merged: dict[str, str] = dict(keys or {})

    # Step 1: detect existing keys
    detected: dict[str, str] = {}
    for k in _DEFAULT_KEY_NAMES:
        v = detect_env_key(k)
        if v:
            detected[k] = v
    for k, v in detected.items():
        if k not in merged:
            merged[k] = v
    steps.append({"name": "detect_keys", "ok": True,
                   "message": f"检测到 {len(detected)} 个已有 key"})

    # Step 2: interactive prompt for missing keys
    if interactive:
        for k in _DEFAULT_KEY_NAMES:
            if not merged.get(k):
                try:
                    val = input(f"  请输入 {k} (留空跳过): ").strip()
                    if val:
                        merged[k] = val
                except EOFError:
                    pass

    # Step 3: write .env
    if merged:
        try:
            write_env_file(merged)
            steps.append({"name": "write_env", "ok": True,
                           "message": f"写入 {len(merged)} 个 key 到 .env"})
        except Exception as e:
            steps.append({"name": "write_env", "ok": False, "message": str(e)})
    else:
        steps.append({"name": "write_env", "ok": True, "message": "无 key 可写"})

    # Step 4: start LiteLLM
    litellm_result = start_litellm()
    steps.append({"name": "start_litellm", "ok": litellm_result["ok"],
                   "message": litellm_result.get("output", "")})

    # Step 5: check gateway
    gw = check_gateway()
    steps.append({"name": "check_gateway", "ok": gw["ok"],
                   "message": f"latency={gw['latency_ms']}ms" if gw["ok"] else gw["error"]})

    overall_ok = all(s["ok"] for s in steps)
    return {"ok": overall_ok, "steps": steps}
