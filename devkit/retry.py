"""devkit/retry.py — Backlog 任务重试策略。

纯标准库，无第三方依赖。
"""
from __future__ import annotations
from typing import List


def should_retry(item: dict, max_retries: int = 2) -> bool:
    """判断 failed 任务是否应重试。"""
    if item.get("status") != "failed":
        return False
    return item.get("_attempts", 0) < max_retries


def record_attempt(item: dict) -> dict:
    """在 item 上记录一次尝试（原地修改并返回）。"""
    item["_attempts"] = item.get("_attempts", 0) + 1
    return item


def reset_for_retry(item: dict) -> dict:
    """将 failed 任务重置为 pending 以便重试（原地修改并返回）。"""
    if item.get("status") == "failed":
        item["status"] = "pending"
        record_attempt(item)
    return item


def filter_retryable(backlog: List[dict], max_retries: int = 2) -> List[dict]:
    """从 backlog 中找出所有可重试的任务，返回列表（不修改原数据）。"""
    return [item for item in backlog if should_retry(item, max_retries)]
