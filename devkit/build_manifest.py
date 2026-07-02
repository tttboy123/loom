# devkit/build_manifest.py
"""Build manifest generation, diffing, and summary (stdlib only).

Loom 宪章 §2: report-only 草案；§3: TDD——本文件在 tests/test_build_manifest.py
通过前是失败的实现，通过后即最小可接受实现。
"""
from __future__ import annotations

import os
from datetime import datetime, timezone
from typing import Dict, List

def generate(build_dir: str) -> dict:
    """生成 build 产物清单。

    契约：
      - 目录不存在或不可读 -> files=[], py_count=0, test_count=0, total=0
      - files 排序、去重
      - generated_at 为 ISO-8601 字符串（UTC, 末尾 'Z'）
    """
    files: List[str] = []
    if os.path.isdir(build_dir):
        try:
            entries = os.listdir(build_dir)
        except OSError:
            entries = []
        files = sorted(set(entries))

    py_count = sum(1 for f in files if f.endswith(".py"))
    test_count = sum(
        1 for f in files if f.endswith(".py") and (f.startswith("test_") or "_test.py" in f)
    )

    return {
        "files": files,
        "py_count": py_count,
        "test_count": test_count,
        "total": len(files),
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }

def diff_manifests(old: dict, new: dict) -> dict:
    """对比两份清单，返回 added/removed/unchanged。

    契约：
      - 只看 'files' 字段，按集合语义比较
      - 缺失键视作空列表
    """
    old_set = set((old or {}).get("files", []) or [])
    new_set = set((new or {}).get("files", []) or [])
    return {
        "added": sorted(new_set - old_set),
        "removed": sorted(old_set - new_set),
        "unchanged": len(old_set & new_set),
    }

def manifest_summary(manifest: dict) -> str:
    """单行人类可读摘要。"""
    m = manifest or {}
    return f"{m.get('total', 0)} files ({m.get('py_count', 0)} py, {m.get('test_count', 0)} tests)"
