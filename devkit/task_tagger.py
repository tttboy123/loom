"""
devkit/task_tagger.py
为 backlog 任务打标签 / 搜索标签。纯标准库。

合同（contract）：
  - add_tag    : 原地追加 tag 并去重；自动初始化 task['tags']；返回 task。
  - remove_tag : 原地移除 tag；不存在或 task['tags'] 缺失时静默；返回 task。
  - has_tag    : task['tags'] 含 tag 时返回 True；缺失键视为 False。
  - filter_by_tag : 返回 backlog 中含 tag 的任务子集（保留原对象）。
  - all_tags   : 返回所有任务标签的去重、按字典序排序的列表。
"""

from __future__ import annotations
from typing import Any

def _ensure_tags_list(task: dict) -> list:
    """确保 task['tags'] 是一个 list；缺失或为 None 时新建。"""
    tags = task.get("tags")
    if tags is None:
        tags = []
        task["tags"] = tags
    return tags

def add_tag(task: dict, tag: str) -> dict:
    """向 task['tags'] 追加 tag（去重）；原地修改，返回 task。"""
    tags = _ensure_tags_list(task)
    if tag not in tags:
        tags.append(tag)
    return task

def remove_tag(task: dict, tag: str) -> dict:
    """从 task['tags'] 移除 tag（不存在时静默）；原地修改，返回 task。"""
    tags = _ensure_tags_list(task)
    if tag in tags:
        tags.remove(tag)
    return task

def has_tag(task: dict, tag: str) -> bool:
    """task['tags'] 中含 tag 时返回 True。缺失键视为 False。"""
    tags = task.get("tags") or []
    return tag in tags

def filter_by_tag(backlog: list, tag: str) -> list:
    """返回 backlog 中含 tag 的任务列表（保持原顺序、保留原对象）。"""
    return [t for t in backlog if has_tag(t, tag)]

def all_tags(backlog: list) -> list:
    """返回所有任务标签的去重、按字典序排序的有序列表。"""
    seen: set = set()
    for t in backlog:
        for tag in (t.get("tags") or []):
            seen.add(tag)
    return sorted(seen)

__all__ = [
    "add_tag",
    "remove_tag",
    "has_tag",
    "filter_by_tag",
    "all_tags",
]
