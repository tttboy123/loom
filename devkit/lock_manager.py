"""
lock_manager.py — 纯标准库资源锁管理器

不可变风格：每次 acquire / release 返回新 manager，原对象不被修改。
时间戳使用 UTC ISO-8601（带 'Z' 后缀），便于跨时区比对与日志。
"""
from datetime import datetime, timezone
from typing import Dict, Tuple, List

def _now_iso() -> str:
    """UTC 当前时间的 ISO-8601 字符串（带 Z 后缀）。"""
    return datetime.now(timezone.utc).isoformat(timespec="seconds").replace(
        "+00:00", "Z"
    )

def create() -> dict:
    """创建一个空的锁管理器。"""
    return {"locks": {}}

def acquire(manager: dict, resource: str, owner: str) -> Tuple[bool, dict]:
    """
    尝试为 owner 获取 resource 的锁。
    - 未锁定：写入锁记录，返回 (True, new_manager)
    - 已被同一 owner 持有：幂等成功，返回 (True, manager)
    - 被他人持有：返回 (False, manager)
    """
    locks = manager.get("locks", {})
    if resource in locks:
        if locks[resource]["owner"] == owner:
            return True, manager
        return False, manager

    new_manager = {"locks": {**locks, resource: {"owner": owner, "acquired_at": _now_iso()}}}
    return True, new_manager

def release(manager: dict, resource: str, owner: str) -> Tuple[bool, dict]:
    """
    释放 resource 的锁。仅当持有者 == owner 时删除并返回成功。
    否则返回 (False, manager)，状态不变。
    """
    locks = manager.get("locks", {})
    if resource not in locks:
        return False, manager
    if locks[resource]["owner"] != owner:
        return False, manager

    new_locks = {k: v for k, v in locks.items() if k != resource}
    return True, {"locks": new_locks}

def is_locked(manager: dict, resource: str) -> bool:
    """resource 是否处于锁定状态。"""
    return resource in manager.get("locks", {})

def manager_summary(manager: dict) -> dict:
    """返回 {total_locked, resources} 摘要。"""
    locks = manager.get("locks", {})
    return {
        "total_locked": len(locks),
        "resources": list(locks.keys()),
    }
