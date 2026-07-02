"""
Eval Gate —— golden 集质量回归（对齐 RD-LOOP「unit 测拦不住 LLM 质量回归」）。

golden 文件是 JSON 数组，每条一个用例，几种形式：
  1) Python 表达式：{"name":"...", "import":"from reverse import reverse",
                     "expr":"reverse('abc')", "expect":"cba"}
     期望抛异常：   {..., "expr":"f(-1)", "raises":"ValueError"}
  2) 子进程：       {"name":"...", "cmd":"python cli.py", "stdin":"abc",
                     "expect_contains":"cba", "expect_exit":0}
  3) 真机 Web：     {"name":"...", "web":{"start":["python","app.py"],"port":8000,
                     "path":"/","status":200,"expect_contains":"<html"}}
                    —— 启动应用 → 真实 HTTP 请求 → 校验（纯标准库，无浏览器依赖）。
  4) 真浏览器(可选)：{"name":"...", "playwright":"check.py", "web":{...start...}}
                    —— 装了 playwright 才跑；脚本用 LOOM_BASE_URL 拿应用地址；没装则⏭跳过。

在「物化后的 build/ 沙箱」里跑；任一失败 → Gate NO-GO（跳过不算失败）。fail-open（异常计未过）。
"""
from __future__ import annotations

import json
import os
import pathlib
import signal
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from typing import List, Optional, Tuple


def _start_app(build: pathlib.Path, spec: dict, wait: int = 30):
    """按 spec.start 启动应用，轮询 spec.port 直到就绪。返回 (proc, err)。"""
    start = spec.get("start")
    port = int(spec.get("port", 8000))
    proc = None
    if start:
        cmd = start if isinstance(start, list) else str(start).split()
        proc = subprocess.Popen(cmd, cwd=str(build), stdout=subprocess.DEVNULL,
                                stderr=subprocess.DEVNULL, start_new_session=True)
    deadline = time.time() + wait
    while time.time() < deadline:
        if proc and proc.poll() is not None:
            return proc, f"应用进程提前退出（exit={proc.returncode}）"
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=1):
                return proc, ""
        except OSError:
            time.sleep(0.3)
    return proc, f"端口 {port} 未就绪（应用没起来？）"


def _kill(proc) -> None:
    if proc and proc.poll() is None:
        try:
            os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        except Exception:  # noqa: BLE001
            proc.terminate()
        try:
            proc.wait(timeout=3)         # 回收，避免僵尸 / ResourceWarning
        except Exception:  # noqa: BLE001
            pass


def _run_web(build: pathlib.Path, c: dict) -> Tuple[Optional[bool], str]:
    """真机验证：启动应用 → 真实 HTTP 请求 → 校验 status / body 含子串（纯标准库）。"""
    spec = c.get("web", {})
    proc, err = _start_app(build, spec)
    try:
        if err:
            return False, err
        url = f"http://127.0.0.1:{int(spec.get('port', 8000))}{spec.get('path', '/')}"
        req = urllib.request.Request(url, method=str(spec.get("method", "GET")))
        try:
            with urllib.request.urlopen(req, timeout=6) as r:
                code, body = r.status, r.read(40000).decode("utf-8", "replace")
        except urllib.error.HTTPError as e:
            code, body = e.code, e.read(40000).decode("utf-8", "replace")
        if "status" in spec and int(code) != int(spec["status"]):
            return False, f"status={code} want={spec['status']}"
        exp = spec.get("expect_contains")
        if exp and str(exp) not in body:
            return False, f"body 不含 {exp!r}（status={code}）"
        return True, f"status={code} ✓ 真机验证"
    finally:
        _kill(proc)


def _run_playwright(build: pathlib.Path, c: dict, timeout: int = 90) -> Tuple[Optional[bool], str]:
    """可选：装了 playwright 才跑真浏览器脚本（用 LOOM_BASE_URL 指向起好的应用）；没装则跳过。"""
    import importlib.util
    if importlib.util.find_spec("playwright") is None:
        return None, "playwright 未安装，跳过（pip install playwright && playwright install chromium）"
    spec = c.get("web", {})
    proc, err = _start_app(build, spec) if spec.get("start") else (None, "")
    try:
        if err:
            return False, err
        env = dict(os.environ, LOOM_BASE_URL=f"http://127.0.0.1:{int(spec.get('port', 8000))}")
        r = subprocess.run([sys.executable, str(build / c["playwright"])], cwd=str(build),
                           env=env, capture_output=True, text=True, timeout=timeout)
        if r.returncode == 0:
            return True, "playwright ✓ 真浏览器验证"
        tail = ((r.stderr or r.stdout or "").strip().splitlines() or ["脚本失败"])[-1]
        return False, tail[:120]
    finally:
        _kill(proc)


def _run_case(build: pathlib.Path, c: dict, timeout: int = 30) -> Tuple[Optional[bool], str]:
    try:
        if "playwright" in c:        # 必须先于 web：playwright 用例也带 web 启动 spec
            return _run_playwright(build, c)
        if "web" in c:
            return _run_web(build, c)
        if "expr" in c:
            code = (str(c.get("import", "")) + "\n_r=(" + str(c["expr"]) + ")\nprint(repr(_r))")
            r = subprocess.run([sys.executable, "-c", code], cwd=str(build),
                               capture_output=True, text=True, timeout=timeout)
            raises = c.get("raises")
            if raises:                       # 期望抛指定异常（非 happy-path 的正确表达）
                if r.returncode != 0:
                    ok = str(raises) in (r.stderr or "")
                    last = ((r.stderr or "").strip().splitlines() or [""])[-1][:120]
                    return ok, (f"raised {raises} ✓" if ok else f"{last}  want raises {raises}")
                return False, f"未抛异常 got={r.stdout.strip()}  want raises {raises}"
            if r.returncode != 0:
                err = ((r.stderr or "").strip().splitlines() or ["运行错误"])[-1][:140]
                return False, f"{err}  want={repr(c.get('expect'))}"  # 异常也带期望值，便于迭代修复
            got, want = r.stdout.strip(), repr(c.get("expect"))
            return (got == want), f"got={got} want={want}"
        if "cmd" in c:
            cmd = c["cmd"] if isinstance(c["cmd"], list) else str(c["cmd"]).split()
            r = subprocess.run(cmd, cwd=str(build), input=str(c.get("stdin", "")),
                               capture_output=True, text=True, timeout=timeout)
            out = ((r.stdout or "") + (r.stderr or "")).strip()
            if "expect_exit" in c and r.returncode != c["expect_exit"]:
                return False, f"exit={r.returncode} want={c['expect_exit']}"
            exp = c.get("expect_contains")
            if exp is not None:
                return (exp in out), f"out~={out[:120]}"
            return (r.returncode == 0), out[:120]
        return False, "未知用例格式（需 expr 或 cmd）"
    except subprocess.TimeoutExpired:
        return False, f"超时 > {timeout}s"
    except Exception as e:  # noqa: BLE001
        return False, f"{type(e).__name__}: {e}"


def run_golden(build: pathlib.Path, golden_path: str) -> Tuple[bool, str]:
    """跑 golden 集，返回 (全过?, markdown 摘要)。"""
    p = pathlib.Path(golden_path)
    if not p.exists():
        return False, f"golden 文件不存在：{golden_path}"
    try:
        cases = json.loads(p.read_text(encoding="utf-8"))
        assert isinstance(cases, list)
    except Exception as e:  # noqa: BLE001
        return False, f"golden 文件解析失败：{e}"
    rows: List[str] = []
    all_ok, skipped = True, 0
    for i, c in enumerate(cases):
        ok, detail = _run_case(build, c)
        if ok is None:                         # 跳过（如 playwright 未装）→ 不拉低 gate
            skipped += 1
            mark = "⏭"
        else:
            all_ok = all_ok and ok
            mark = "✅" if ok else "❌"
        rows.append(f"| {c.get('name', f'case{i+1}')} | {mark} | {str(detail).replace('|', '│')[:80]} |")
    tail = f"（{len(cases)} 例" + (f"，{skipped} 跳过" if skipped else "") + "）"
    summary = ("| 用例 | 结果 | 详情 |\n| --- | --- | --- |\n" + "\n".join(rows)
               + f"\n\n**Eval Gate：{'✅ 全过' if all_ok else '❌ 有失败'}{tail}**")
    return all_ok, summary
