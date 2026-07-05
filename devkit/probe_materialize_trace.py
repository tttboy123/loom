"""
Probe：直接调用 harness/materialize.py 的 write_file 物化 fixture，
用 pathlib 在调用前后 stat 目标路径，落盘到 runs/probe_<ts>/materialize_trace.json。

设计原则：
  - 不与 harness 强耦合：通过 _get_harness() 注入，测试可替换
  - 调用前后都 stat，字段齐全（含 error / mtime / size）
  - 异常被捕获并写入 trace，不让脚本崩溃
"""

from __future__ import annotations

import importlib
import json
import os
import traceback
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Union

PathLike = Union[str, Path]

# ---------------------------------------------------------------------------
# Harness 注入层（便于单测；生产环境会真实 import harness.materialize）
# ---------------------------------------------------------------------------

def _get_harness():
    """
    真实实现：import harness.materialize 并返回该模块。
    这里允许通过环境变量 PROBE_HARNESS_OVERRIDE 指向别的符号，
    默认走 write_file。
    """
    mod = importlib.import_module("harness.materialize")
    return mod

# ---------------------------------------------------------------------------
# 核心：跑一次 probe
# ---------------------------------------------------------------------------

def _stat_snapshot(path: Path) -> Dict[str, Any]:
    try:
        st = path.stat()
        return {
            "exists": path.exists(),
            "size_bytes": st.st_size,
            "mtime": st.st_mtime,
        }
    except FileNotFoundError:
        return {"exists": False, "size_bytes": 0, "mtime": None}
    except OSError as e:
        return {"exists": path.exists(), "size_bytes": 0, "mtime": None, "stat_error": str(e)}

def run_probe(
    *,
    fixture_rel: PathLike,
    target_rel: PathLike,
    runs_dir: PathLike,
    harness_entry: str = "write_file",
    now: datetime | None = None,
) -> Dict[str, Any]:
    """
    跑一次物化 + 探测，返回 dict（含 trace_path）。
    异常被捕获并写入 trace，不抛出。
    """
    fixture = Path(fixture_rel).resolve()
    target = Path(target_rel).resolve()
    runs = Path(runs_dir)

    if not fixture.exists():
        raise FileNotFoundError(f"fixture not found: {fixture}")

    runs.mkdir(parents=True, exist_ok=True)
    ts = (now or datetime.now(timezone.utc)).strftime("%Y%m%dT%H%M%S%fZ")
    out_dir = runs / f"probe_{ts}"
    out_dir.mkdir(parents=True, exist_ok=True)
    trace_path = out_dir / "materialize_trace.json"

    # 调用前 stat
    before = _stat_snapshot(target)
    payload: Dict[str, Any] = {
        "timestamp": ts,
        "fixture": str(fixture),
        "target": str(target),
        "harness_entry": harness_entry,
        "exists_before": before["exists"],
        "exists_after": False,
        "size_bytes": 0,
        "mtime": None,
        "error": None,
    }

    # 真正调用 harness
    try:
        harness = _get_harness()
        fn = getattr(harness, harness_entry)
        fn(str(fixture), str(target))  # 与 harness.write_file(src, dst) 契约一致
    except Exception as e:  # noqa: BLE001 - 故意捕获，记录到 trace
        payload["error"] = f"{type(e).__name__}: {e}"
        payload["traceback"] = traceback.format_exc(limit=4)
    finally:
        # 调用后 stat（即便失败也尽量记录）
        after = _stat_snapshot(target)
        payload["exists_after"] = after["exists"]
        payload["size_bytes"] = after["size_bytes"]
        payload["mtime"] = after["mtime"]

    trace_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    payload["trace_path"] = str(trace_path)
    return payload

# ---------------------------------------------------------------------------
# CLI 入口
# ---------------------------------------------------------------------------

def _default_paths() -> Dict[str, str]:
    cwd = Path(os.getcwd())
    return {
        "fixture": "tests/_probe/polluted_input.md",
        "target": "build/tests/_probe/polluted_input.md",
        "runs": "runs",
    }

def main() -> int:
    paths = _default_paths()
    result = run_probe(
        fixture_rel=paths["fixture"],
        target_rel=paths["target"],
        runs_dir=paths["runs"],
    )
    print(json.dumps({k: v for k, v in result.items() if k != "traceback"}, ensure_ascii=False, indent=2))
    return 0 if result.get("error") is None else 1

if __name__ == "__main__":
    raise SystemExit(main())
