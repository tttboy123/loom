"""devkit/run_archive.py — 旧 run 归档工具（纯标准库）"""

from __future__ import annotations

import os
import shutil

def list_old_runs(runs_dir: str = "runs", keep_latest: int = 20) -> list[str]:
    """返回按名称排序后、除最新 keep_latest 个外的所有 run_id 列表。

    runs_dir 不存在返回 []。
    """
    if not os.path.isdir(runs_dir):
        return []

    # 只取目录
    entries = [
        e for e in os.listdir(runs_dir)
        if os.path.isdir(os.path.join(runs_dir, e))
    ]
    entries.sort()

    if len(entries) <= keep_latest:
        return []

    return entries[:-keep_latest]

def archive_run(
    run_id: str,
    runs_dir: str = "runs",
    archive_dir: str = "runs/archive",
) -> bool:
    """把 runs/{run_id} 目录移动到 runs/archive/{run_id}，返回 True/False。

    archive_dir 不存在时自动创建。
    """
    src = os.path.join(runs_dir, run_id)
    if not os.path.isdir(src):
        return False

    os.makedirs(archive_dir, exist_ok=True)
    dst = os.path.join(archive_dir, run_id)

    # 如果目标已存在，先移除（允许覆盖旧的归档）
    if os.path.exists(dst):
        shutil.rmtree(dst)

    shutil.move(src, dst)
    return True

def prune(
    runs_dir: str = "runs",
    keep_latest: int = 20,
    archive_dir: str = "runs/archive",
) -> int:
    """把超出 keep_latest 的旧 run 全部归档，返回归档数量。

    若 runs_dir 不存在返回 0。
    """
    if not os.path.isdir(runs_dir):
        return 0

    old_runs = list_old_runs(runs_dir, keep_latest=keep_latest)
    count = 0
    for run_id in old_runs:
        if archive_run(run_id, runs_dir, archive_dir):
            count += 1
    return count
