# devkit/decisions_log.py
# 纯标准库 JSONL 决策日志记录器

import json
import os

DEFAULT_LOG_PATH = "/tmp/loom_decisions.jsonl"

def append(log_path, record: dict) -> None:
    """把 record 追加写入 log_path（JSONL 格式）
    不存在时自动创建，失败不报错（静默）
    """
    try:
        os.makedirs(os.path.dirname(log_path) or ".", exist_ok=True)
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(json.dumps(record, ensure_ascii=False) + "\n")
    except Exception:
        pass

def read_all(log_path=None) -> list[dict]:
    """读取所有行，跳过空行和无效 JSON，文件不存在返回 []"""
    if log_path is None:
        log_path = DEFAULT_LOG_PATH

    try:
        if not os.path.exists(log_path):
            return []

        results = []
        with open(log_path, "r", encoding="utf-8") as f:
            for line in f:
                stripped = line.strip()
                if not stripped:
                    continue
                try:
                    results.append(json.loads(stripped))
                except json.JSONDecodeError:
                    continue
        return results
    except Exception:
        return []

def last_n(log_path, n: int = 10) -> list[dict]:
    """返回最后 n 条记录"""
    all_records = read_all(log_path)
    return all_records[-n:] if all_records else []

def clear(log_path) -> None:
    """清空文件（截断），文件不存在时创建空文件"""
    try:
        os.makedirs(os.path.dirname(log_path) or ".", exist_ok=True)
        with open(log_path, "w", encoding="utf-8") as f:
            f.write("")
    except Exception:
        pass
