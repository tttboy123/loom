"""
改动预览：把一次运行的 build/ 产物与另一次运行做 unified diff。

控制台（console/server.py）与 CLI（python3 -m devkit diff）共用这一份实现。
runs_dir 作为参数传入，便于单测指向临时目录。
"""
from __future__ import annotations

import difflib
import pathlib
from typing import List, Optional


def list_build_files(build: pathlib.Path) -> List[str]:
    """列出 build/ 下的代码文件相对路径，排除 _deps / __pycache__ / .* / *.pyc 等内部产物。"""
    out = []
    for f in sorted(build.rglob("*")):
        rel = f.relative_to(build)
        if (not f.is_file() or f.suffix == ".pyc"
                or any(p.startswith("_") or p.startswith(".") or p == "__pycache__"
                       for p in rel.parts)):
            continue
        out.append(str(rel))
    return out


def prev_run_with_build(runs_dir: pathlib.Path, ts: str) -> Optional[str]:
    """按时间戳找比 ts 更早、且有 build/ 的最近一次运行（配合「改一改重跑」看改动）。"""
    if not runs_dir.exists():
        return None
    cands = sorted(p.name for p in runs_dir.iterdir()
                   if p.is_dir() and (p / "build").is_dir())
    earlier = [c for c in cands if c < ts]
    return earlier[-1] if earlier else None


def diff_runs(runs_dir: pathlib.Path, ts: str, against: str = "") -> dict:
    """本次运行 build/ 对比另一次（默认上一次带 build 的）的 unified diff。

    返回 {ts, against, files:[{name,status,diff}], changed, total} 或 {error}。
    status ∈ new | changed | deleted | same。
    """
    if not ts or ".." in ts or "/" in ts:
        return {"error": "bad ts"}
    cur = runs_dir / ts / "build"
    if not cur.is_dir():
        return {"error": "该运行没有 build 产物（implement 未产出代码）"}
    base = against or prev_run_with_build(runs_dir, ts)
    if not base:
        return {"error": "没有更早的、带 build 的运行可对比"}
    if ".." in base or "/" in base:
        return {"error": "bad against"}
    bdir = runs_dir / base / "build"
    if not bdir.is_dir():
        return {"error": f"{base} 没有 build 产物"}
    a_files, b_files = set(list_build_files(bdir)), set(list_build_files(cur))
    out = []
    for name in sorted(a_files | b_files):
        a = (bdir / name).read_text().splitlines(keepends=True) if name in a_files else []
        b = (cur / name).read_text().splitlines(keepends=True) if name in b_files else []
        if a == b:
            out.append({"name": name, "status": "same", "diff": ""})
            continue
        status = "new" if not a else ("deleted" if not b else "changed")
        diff = "".join(difflib.unified_diff(
            a, b, fromfile=f"{base}/{name}", tofile=f"{ts}/{name}"))
        out.append({"name": name, "status": status, "diff": diff[:8000]})
    return {"ts": ts, "against": base, "files": out,
            "changed": sum(1 for f in out if f["status"] != "same"), "total": len(out)}
