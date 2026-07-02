# resume.py
"""断点续跑：从 run 目录推断已完成阶段。纯 IO 层，只读文件系统。"""
from __future__ import annotations

import re
from pathlib import Path

# 文件名格式：两位数字-名称.md
_STAGE_RE = re.compile(r"^(\d{2})-(.+)\.md$")
# 需跳过的特殊前缀
_SKIP_PREFIXES = {"00", "90", "91", "99"}

def done_stages(run_dir) -> list[str]:
    """扫描 run_dir，返回已完成阶段名列表（按数字前缀升序）。"""
    p = Path(run_dir)
    if not p.is_dir():
        return []

    found: list[tuple[str, str]] = []
    for f in p.iterdir():
        if not f.is_file():
            continue
        m = _STAGE_RE.match(f.name)
        if not m:
            continue
        num, name = m.group(1), m.group(2)
        if num in _SKIP_PREFIXES:
            continue
        found.append((num, name))

    found.sort(key=lambda x: (x[0], x[1]))
    return [name for _, name in found]

def pending_stages(run_dir, all_stages: list[str]) -> list[str]:
    """返回尚未完成的阶段列表，保持 all_stages 原有顺序。"""
    done = set(done_stages(run_dir))
    return [s for s in all_stages if s not in done]

def is_complete(run_dir) -> bool:
    """判断 run 是否彻底完成：run_dir 存在且包含 run-log.md。"""
    p = Path(run_dir)
    if not p.is_dir():
        return False
    return (p / "run-log.md").is_file()
