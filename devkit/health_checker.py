# devkit/health_checker.py
"""综合系统健康检查（纯标准库）。

公共 API：
    check_python_version(min_version=(3, 9)) -> dict
    check_file_writable(path) -> dict
    check_json_parseable(text) -> dict
    full_check(write_path='/tmp') -> dict
"""
from __future__ import annotations

import json
import os
import sys
import tempfile
from typing import Tuple, Union

VersionTuple = Union[Tuple[int, ...], Tuple[int, int], Tuple[int, int, int]]

def check_python_version(min_version: VersionTuple = (3, 9)) -> dict:
    """比对 sys.version_info 是否满足最低版本要求。"""
    current = sys.version_info
    # namedtuple 与普通 tuple 支持逐元素比较；短元组视为前缀
    ok = bool(current >= min_version)  # type: ignore[operator]
    return {
        "ok": ok,
        "version": f"{current.major}.{current.minor}.{current.micro}",
        "min_version": ".".join(str(p) for p in min_version),
    }

def check_file_writable(path: str) -> dict:
    """通过写入并删除临时文件来验证 path 目录可写。"""
    result = {"ok": False, "path": path, "error": ""}
    # 1) 目录必须存在
    if not os.path.isdir(path):
        try:
            os.makedirs(path, exist_ok=True)
        except Exception as e:  # 权限或父目录不存在
            result["error"] = f"directory not writable or not creatable: {e}"
            return result

    tmp_path = None
    try:
        fd, tmp_path = tempfile.mkstemp(
            dir=path, prefix=".health_check_", suffix=".tmp"
        )
        try:
            os.write(fd, b"health_check")
            os.fsync(fd)
        finally:
            os.close(fd)
        # 2) 写入成功即视为可写；删除失败要降级为 ok=False 并报错
        try:
            os.remove(tmp_path)
        except Exception as e:
            result["error"] = f"tempfile cleanup failed: {e}"
            return result
        result["ok"] = True
        return result
    except Exception as e:
        result["error"] = f"write failed: {e}"
        # 尽力清理
        if tmp_path:
            try:
                os.remove(tmp_path)
            except OSError:
                pass
        return result

def check_json_parseable(text: str) -> dict:
    """尝试 json.loads(text)。"""
    try:
        json.loads(text)
        return {"ok": True, "error": ""}
    except (json.JSONDecodeError, ValueError, TypeError) as e:
        return {"ok": False, "error": str(e)}

def full_check(write_path: str = "/tmp") -> dict:
    """python + writable 联合检查。"""
    py = check_python_version()
    wr = check_file_writable(write_path)
    return {
        "python": py,
        "writable": wr,
        "overall_ok": bool(py.get("ok")) and bool(wr.get("ok")),
    }
