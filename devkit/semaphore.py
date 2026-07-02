"""
devkit.semaphore — 纯标准库实现的计数信号量。

约定：
  - 函数式风格：输入 dict 不被改写，返回新的 dict。
  - 计数不会越界：acquire 不会变负；release 不会超过 max_count。
  - release 同时承担"归还名额 + 唤醒队首 waiter"两步：
    * 若存在 waiters，移除队首（count 不变，等价于把名额"直接给" waiter）。
    * 若没有 waiters 且 count < max_count，count += 1。
"""

from typing import Tuple, List, Dict

Semaphore = Dict  # {"max_count": int, "count": int, "waiters": List[str]}

def create(max_count: int) -> dict:
    """创建计数信号量，初始 count = max_count，waiters 为空。"""
    return {
        "max_count": max_count,
        "count": max_count,
        "waiters": [],
    }

def acquire(sem: dict, owner: str) -> Tuple[bool, dict]:
    """
    申请一个名额。

    - 若 count > 0：count -= 1，返回 (True, new_sem)。
    - 否则：owner 进入 waiters，返回 (False, new_sem)。
    """
    if sem["count"] > 0:
        new_sem = {
            "max_count": sem["max_count"],
            "count": sem["count"] - 1,
            "waiters": list(sem["waiters"]),
        }
        return True, new_sem

    new_waiters = list(sem["waiters"])
    new_waiters.append(owner)
    new_sem = {
        "max_count": sem["max_count"],
        "count": sem["count"],
        "waiters": new_waiters,
    }
    return False, new_sem

def release(sem: dict, owner: str) -> dict:
    """
    释放一个名额。

    - 若存在 waiters：移除队首（FIFO），名额保留语义等价于 count 不变；
      但为了与"释放一次 / 名额回 1"的直觉一致，这里把 count 视作可用名额，
      唤醒时显式把名额交给 waiter —— 因此实现为：先按"无 waiter 时 count += 1"，
      再把队首移除，使 (count + 等候等待唤醒的 waiter) 总可用名额守恒。

    简化语义（与任务规约一致）：
      - 若有 waiters：移除第一个 waiter，count 不变（名额已"预分配"给队首）。
      - 否则若 count < max_count：count += 1。
    """
    new_waiters: List[str] = list(sem["waiters"])
    if new_waiters:
        new_waiters.pop(0)
        new_count = sem["count"]
    else:
        new_count = sem["count"] + 1 if sem["count"] < sem["max_count"] else sem["count"]

    return {
        "max_count": sem["max_count"],
        "count": new_count,
        "waiters": new_waiters,
    }

def sem_summary(sem: dict) -> dict:
    """返回 {max_count, available, waiters, utilization}。"""
    max_count = sem["max_count"]
    count = sem["count"]
    waiters = len(sem["waiters"])
    available = count
    utilization = (max_count - count) / max_count if max_count > 0 else 0.0
    return {
        "max_count": max_count,
        "available": available,
        "waiters": waiters,
        "utilization": utilization,
    }
