# devkit/token_bucket.py
import time

def create(window_size: int = 60, max_tokens: int = 1000) -> dict:
    return {
        "window_size": window_size,
        "max_tokens": max_tokens,
        "buckets": {},
        "total_consumed": 0
    }

def consume(tb: dict, key: str, amount: int, timestamp: int) -> tuple[bool, dict]:
    if amount > tb["max_tokens"]:
        return (False, tb)
    
    window_start = timestamp - tb["window_size"]
    
    # 清理过期记录
    if key in tb["buckets"]:
        tb["buckets"][key] = [(ts, amt) for ts, amt in tb["buckets"][key] if ts >= window_start]
    
    # 计算当前窗口已消耗
    current_usage = sum(amt for _, amt in tb["buckets"].get(key, []))
    
    if current_usage + amount > tb["max_tokens"]:
        return (False, tb)
    
    # 记录消耗
    if key not in tb["buckets"]:
        tb["buckets"][key] = []
    tb["buckets"][key].append((timestamp, amount))
    tb["total_consumed"] += amount
    
    return (True, tb)

def usage(tb: dict, key: str, timestamp: int) -> int:
    window_start = timestamp - tb["window_size"]
    if key not in tb["buckets"]:
        return 0
    return sum(amt for ts, amt in tb["buckets"][key] if ts >= window_start)

def tb_summary(tb: dict) -> dict:
    return {
        "keys": list(tb["buckets"].keys()),
        "total_consumed": tb["total_consumed"],
        "max_tokens": tb["max_tokens"]
    }
