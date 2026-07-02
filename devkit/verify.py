# verify.py
"""T16 物理验证：在干净子进程里导入模块 + 运行 golden 用例，与 evals.py 互补。

evals.py 在当前进程内 eval，verify.py 用 subprocess 完全隔离，抓住：
  - 导入期副作用 / 循环导入 / 缺失 __init__
  - 仅在 loom 运行环境里才通过的隐式依赖
  - 真实用户安装时会遇到的 ImportError
"""
from __future__ import annotations

import datetime as _dt
import json
import os
import pathlib
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from typing import Any, Iterable, Optional

_DEFAULT_DENIED_PREFIXES: tuple[str, ...] = (
    "tests/",
    "tests/contract/",
    "devkit/",
    ".github/",
)
_DEFAULT_REDIRECT_ROOT = "runs/{run_id}/_verify_tmp/"


@dataclass(frozen=True)
class PathDecision:
    final_path: str
    rewritten: bool
    reason: str | None


class MaterializePathForbidden(Exception):
    """物化目标命中受保护前缀。"""


def _discover_repo_root(start: pathlib.Path) -> pathlib.Path | None:
    cur = pathlib.Path(start).resolve()
    for cand in [cur, *cur.parents]:
        if (cand / "devkit" / "__init__.py").is_file():
            return cand
    return None


def _normalize_path(path: pathlib.Path | str) -> str:
    norm = str(path).replace("\\", "/").strip()
    if norm.startswith("./"):
        norm = norm[2:]
    return norm


def _is_denied(path: str, denied: tuple[str, ...]) -> bool:
    return any(path == prefix.rstrip("/") or path.startswith(prefix) for prefix in denied)


def resolve_target_path(
    declared: str,
    run_id: str,
    *,
    denied_prefixes: tuple[str, ...] = _DEFAULT_DENIED_PREFIXES,
    redirect_root: str = _DEFAULT_REDIRECT_ROOT,
) -> PathDecision:
    """将 verify 物化目标重写到 runs/<run_id>/_verify_tmp/ 下。"""
    if not run_id:
        raise ValueError("run_id must be a non-empty string")

    original = declared or ""
    declared_norm = os.path.normpath(original).replace(os.sep, "/")
    unsafe = (
        not declared_norm
        or declared_norm in {".", ""}
        or os.path.isabs(original)
        or declared_norm.startswith("..")
    )
    if unsafe or _is_denied(declared_norm, denied_prefixes):
        base = redirect_root.format(run_id=run_id).rstrip("/") + "/"
        if os.path.isabs(original):
            tail = declared_norm.lstrip("/") or "_empty"
            final = base + "_absolute/" + tail
        else:
            tail = declared_norm.lstrip("./") or "_empty"
            final = base + tail
        return PathDecision(
            final_path=os.path.normpath(final).replace(os.sep, "/"),
            rewritten=True,
            reason=f"materialize-path-rewritten: {original} -> {os.path.normpath(final).replace(os.sep, '/')}",
        )
    return PathDecision(final_path=declared_norm, rewritten=False, reason=None)


def assert_path_allowed(target: pathlib.Path | str) -> None:
    """仅允许 verify 临时产物写入 runs/... 空间。"""
    s = _normalize_path(target)
    if _is_denied(s, _DEFAULT_DENIED_PREFIXES):
        raise MaterializePathForbidden(f"materialize target '{s}' hits forbidden prefix")


def _append_decision_log(
    decision_log_path: pathlib.Path,
    *,
    task_id: str,
    stage: str,
    decision: str,
    target_path: str,
    reason: str,
) -> None:
    rec = {
        "ts": _dt.datetime.now(_dt.UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "task_id": task_id,
        "stage": stage,
        "decision": decision,
        "target_path": target_path,
        "reason": reason,
    }
    decision_log_path.parent.mkdir(parents=True, exist_ok=True)
    with decision_log_path.open("a", encoding="utf-8") as fp:
        fp.write(json.dumps(rec, ensure_ascii=False) + "\n")


def collect_test_files(*args, **kwargs):
    raise NotImplementedError


def run_materialize_step(
    task: dict[str, Any],
    *,
    decision_log_path: pathlib.Path,
) -> dict[str, Any]:
    task_id = str(task.get("task_id", "<unknown>"))
    target_raw = str(task.get("materialize", {}).get("target", "") or "")
    try:
        assert_path_allowed(target_raw)
    except MaterializePathForbidden as e:
        _append_decision_log(
            decision_log_path,
            task_id=task_id,
            stage="materialize",
            decision="blocked-applylock",
            target_path=_normalize_path(target_raw),
            reason=f"{type(e).__name__}: {e}",
        )
        return {"status": "blocked-applylock", "task_id": task_id, "target": _normalize_path(target_raw)}
    raise NotImplementedError("existing materialize body intentionally not run in this patch")


def rdloop_status_counts(decision_log_path: pathlib.Path) -> dict[str, int]:
    counts: dict[str, int] = {}
    if not decision_log_path.exists():
        return counts
    for line in decision_log_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
        except json.JSONDecodeError:
            continue
        decision = str(rec.get("decision", "<unknown>"))
        counts[decision] = counts.get(decision, 0) + 1
    return counts


def smoke_import(build_dir: pathlib.Path | str, timeout: int = 30) -> dict:
    """尝试在子进程里 import 每个非测试 .py 文件，返回汇总。

    返回：{"ok": bool, "errors": [{"file": str, "error": str}], "imported": [str]}
    """
    build_dir = pathlib.Path(build_dir)
    errors: list[dict] = []
    imported: list[str] = []
    repo_root = _discover_repo_root(build_dir)

    py_files = [
        f for f in sorted(build_dir.rglob("*.py"))
        if f.is_file()
        and not f.name.startswith("test_")
        and not f.name.startswith("_")
        and "__pycache__" not in f.parts
    ]

    for py in py_files:
        rel = py.relative_to(build_dir)
        module_expr = str(rel).replace(os.sep, "/").replace(".py", "").replace("/", ".")
        path_inits = [f"sys.path.insert(0, {str(build_dir)!r})"]
        if repo_root is not None:
            path_inits.append(f"sys.path.insert(0, {str(repo_root)!r})")
        script = (
            "import sys; "
            + "; ".join(path_inits)
            + f"; import importlib; importlib.import_module({module_expr!r})"
        )
        try:
            r = subprocess.run(
                [sys.executable, "-c", script],
                capture_output=True, text=True, timeout=timeout,
            )
            if r.returncode == 0:
                imported.append(str(rel))
            else:
                err = (r.stderr or r.stdout or "unknown error").strip().splitlines()[-1]
                errors.append({"file": str(rel), "error": err})
        except subprocess.TimeoutExpired:
            errors.append({"file": str(rel), "error": f"import timeout >{timeout}s"})
        except Exception as e:  # noqa: BLE001
            errors.append({"file": str(rel), "error": str(e)})

    return {"ok": len(errors) == 0, "errors": errors, "imported": imported}


def run_golden_subprocess(
    build_dir: pathlib.Path | str,
    golden_path: str,
    timeout: int = 60,
) -> dict:
    """在独立子进程里跑 golden 用例（与 evals.py 的 in-process eval 互补）。

    每条用例独立 subprocess：确保没有跨用例状态污染。
    返回：{"ok": bool, "passed": int, "failed": int, "rows": [{"name", "ok", "detail"}]}
    """
    build_dir = pathlib.Path(build_dir)
    golden_file = pathlib.Path(golden_path)
    if not golden_file.exists():
        return {"ok": False, "passed": 0, "failed": 0, "rows": [],
                "error": f"golden not found: {golden_path}"}
    try:
        cases = json.loads(golden_file.read_text(encoding="utf-8"))
    except Exception as e:  # noqa: BLE001
        return {"ok": False, "passed": 0, "failed": 0, "rows": [], "error": str(e)}

    rows = []
    for case in cases:
        name = case.get("name", "?")
        imp = case.get("import", "")
        expr = case.get("expr", "")
        expected = case.get("expect")
        raises = case.get("raises")

        if raises:
            script = (
                f"import sys; sys.path.insert(0, {str(build_dir)!r})\n"
                f"{imp}\n"
                f"try:\n"
                f"    result = {expr}\n"
                f"    print('NO_RAISE')\n"
                f"except {raises}:\n"
                f"    print('RAISED')\n"
                f"except Exception as e:\n"
                f"    print(f'WRONG_EXC:{{type(e).__name__}}')\n"
            )
        else:
            script = (
                f"import sys, json; sys.path.insert(0, {str(build_dir)!r})\n"
                f"{imp}\n"
                f"result = {expr}\n"
                f"print(json.dumps(result))\n"
            )

        try:
            r = subprocess.run(
                [sys.executable, "-c", script],
                capture_output=True, text=True, timeout=timeout,
            )
            stdout = (r.stdout or "").strip()
            if r.returncode != 0:
                err = (r.stderr or r.stdout or "error").strip().splitlines()[-1]
                rows.append({"name": name, "ok": False, "detail": f"exit {r.returncode}: {err}"})
                continue

            if raises:
                ok = stdout == "RAISED"
                rows.append({"name": name, "ok": ok,
                             "detail": "" if ok else f"expected {raises}, got: {stdout}"})
            else:
                try:
                    actual = json.loads(stdout)
                    ok = actual == expected
                    rows.append({"name": name, "ok": ok,
                                 "detail": "" if ok else f"got {actual!r}, want {expected!r}"})
                except json.JSONDecodeError:
                    rows.append({"name": name, "ok": False,
                                 "detail": f"non-JSON output: {stdout[:120]}"})
        except subprocess.TimeoutExpired:
            rows.append({"name": name, "ok": False, "detail": f"timeout >{timeout}s"})
        except Exception as e:  # noqa: BLE001
            rows.append({"name": name, "ok": False, "detail": str(e)})

    passed = sum(1 for r in rows if r["ok"])
    failed = len(rows) - passed
    return {"ok": failed == 0 and bool(rows), "passed": passed, "failed": failed, "rows": rows}


def summarize(result: dict) -> str:
    """把 run_golden_subprocess 或 smoke_import 结果格式化成人读字符串。"""
    if "imported" in result:
        ok = result["ok"]
        lines = [f"smoke_import: {'✅ 全部可导入' if ok else '❌ 有导入错误'}"]
        for e in result.get("errors", []):
            lines.append(f"  ✗ {e['file']}: {e['error']}")
        for f in result.get("imported", []):
            lines.append(f"  ✓ {f}")
        return "\n".join(lines)

    rows = result.get("rows", [])
    lines = [f"verify: {result.get('passed',0)}/{len(rows)} 通过"
             + (" ✅" if result.get("ok") else " ❌")]
    for r in rows:
        mark = "✅" if r["ok"] else "❌"
        detail = f"  {mark} {r['name']}"
        if r.get("detail"):
            detail += f"  — {r['detail']}"
        lines.append(detail)
    return "\n".join(lines)
