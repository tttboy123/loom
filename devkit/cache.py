"""
精确响应缓存（stdlib sqlite3）—— 相同请求不重复计费。
线程安全：每次操作新开短连接（不跨线程共享 connection），WAL + busy_timeout。
任何 sqlite 故障一律吞掉、当 miss/no-store（绝不让缓存层崩到调用方）。
schema 版本不符自愈（重建表），TTL 惰性清理 + 硬上限按最旧淘汰，无后台线程。
"""
from __future__ import annotations

import json
import pathlib
import sqlite3
import time
from typing import Optional

SCHEMA = "1"      # 改键/值格式就 bump，老行自动失效


def _conn(db_path) -> sqlite3.Connection:
    p = pathlib.Path(db_path)
    p.parent.mkdir(parents=True, exist_ok=True)
    c = sqlite3.connect(str(p), timeout=5, check_same_thread=False)
    try:                                  # PRAGMA/CREATE 在坏库上会抛 → 先关连接再抛，避免泄漏
        c.execute("PRAGMA journal_mode=WAL")
        c.execute("PRAGMA busy_timeout=5000")
        c.execute("CREATE TABLE IF NOT EXISTS cache "
                  "(key TEXT PRIMARY KEY, val TEXT, ts REAL, ver TEXT)")
    except Exception:
        c.close()
        raise
    return c


def get(db_path, key: str, ttl: float, now: Optional[float] = None):
    """命中且未过期返回 dict；否则 None。任何异常 → None（当 miss）。"""
    now = time.time() if now is None else now
    try:
        c = _conn(db_path)
        try:
            row = c.execute("SELECT val, ts FROM cache WHERE key=? AND ver=?",
                            (key, SCHEMA)).fetchone()
            if not row:
                return None
            val, ts = row
            if now - ts > ttl:                       # 过期 → 惰性删除
                c.execute("DELETE FROM cache WHERE key=?", (key,))
                c.commit()
                return None
            obj = json.loads(val)
            return obj if isinstance(obj, dict) else None   # 只认 dict（防外部篡改成别的形状崩调用方）
        finally:
            c.close()
    except Exception:  # noqa: BLE001
        return None


def put(db_path, key: str, value: dict, max_rows: int, now: Optional[float] = None) -> None:
    """写入；超硬上限按 ts 最旧淘汰。任何异常 → 静默不存。"""
    now = time.time() if now is None else now
    try:
        c = _conn(db_path)
        try:
            c.execute("INSERT OR REPLACE INTO cache (key, val, ts, ver) VALUES (?,?,?,?)",
                      (key, json.dumps(value, ensure_ascii=False), now, SCHEMA))
            n = c.execute("SELECT COUNT(*) FROM cache").fetchone()[0]
            if n > max_rows:
                c.execute("DELETE FROM cache WHERE key IN "
                          "(SELECT key FROM cache ORDER BY ts ASC LIMIT ?)", (n - max_rows,))
            c.commit()
        finally:
            c.close()
    except Exception:  # noqa: BLE001
        return


def delete(db_path, key: str) -> None:
    """删除指定缓存键。任何异常 → 静默忽略。"""
    try:
        c = _conn(db_path)
        try:
            c.execute("DELETE FROM cache WHERE key=?", (key,))
            c.commit()
        finally:
            c.close()
    except Exception:  # noqa: BLE001
        return
