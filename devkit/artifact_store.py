# devkit/artifact_store.py
"""
artifact_store.py —— 构建产物的纯标准库存储。

契约：
  - 持久结构: {"artifacts": dict[str, object]}
  - 所有 mutator（put / remove）**不修改入参**，返回新 store。
  - get 对缺键返回 `default`（默认 None）。
  - keys 始终返回**排序后**的列表。
  - remove 对缺键**静默**处理。
"""

from __future__ import annotations
from typing import Any, Dict, List, Optional

__all__ = ["create", "put", "get", "keys", "remove"]

Store = Dict[str, Dict[str, Any]]

def create() -> Store:
    """创建一个空 store。"""
    return {"artifacts": {}}

def put(store: Store, key: str, value: Any) -> Store:
    """返回包含新键值对的**新** store；原 store 不变。"""
    return {"artifacts": {**store["artifacts"], key: value}}

def get(store: Store, key: str, default: Any = None) -> Any:
    """返回 `key` 对应的值；不存在时返回 `default`。"""
    return store["artifacts"].get(key, default)

def keys(store: Store) -> List[str]:
    """返回 store 中所有键的**排序后**列表。"""
    return sorted(store["artifacts"].keys())

def remove(store: Store, key: str) -> Store:
    """移除 `key` 并返回新 store；缺键时静默不报错。"""
    if key not in store["artifacts"]:
        # 仍返回新 store（保持「不修改输入」的契约一致）
        return {"artifacts": dict(store["artifacts"])}
    return {"artifacts": {k: v for k, v in store["artifacts"].items() if k != key}}
