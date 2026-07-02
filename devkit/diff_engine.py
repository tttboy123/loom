# devkit/diff_engine.py
# 纯标准库实现的字典差异计算引擎。
# 本模块无任何外部依赖，且不修改入参 dict（所有操作返回新 dict）。

from __future__ import annotations
from typing import Any, Dict

def diff(old: Dict[str, Any], new: Dict[str, Any]) -> Dict[str, Dict[str, Any]]:
    """
    对比两个字典的差异。

    Args:
        old: 旧字典。
        new: 新字典。

    Returns:
        一个 dict，包含四个键：
          - added:     在 new 中有但 old 中没有的 {key: value}
          - removed:   在 old 中有但 new 中没有的 {key: value}
          - changed:   两者都有但值不同的 {key: {"old": old_v, "new": new_v}}
          - unchanged: 两者都存在且值相同的 {key: value}
    """
    old_keys = set(old.keys())
    new_keys = set(new.keys())

    added_keys = new_keys - old_keys
    removed_keys = old_keys - new_keys
    common_keys = old_keys & new_keys

    added = {k: new[k] for k in added_keys}
    removed = {k: old[k] for k in removed_keys}

    changed: Dict[str, Dict[str, Any]] = {}
    unchanged: Dict[str, Any] = {}
    for k in common_keys:
        if old[k] == new[k]:
            unchanged[k] = new[k]
        else:
            changed[k] = {"old": old[k], "new": new[k]}

    return {
        "added": added,
        "removed": removed,
        "changed": changed,
        "unchanged": unchanged,
    }

def apply(base: Dict[str, Any], patch: Dict[str, Any]) -> Dict[str, Any]:
    """
    将 patch 合并到 base 的副本上。
    - patch 中值为 None 的键：表示从结果中删除该键（无论其是否存在于 base）。
    - 其他键：覆盖 base 中的同名键（或新增）。
    - 不修改入参 base；返回新 dict。

    Args:
        base:  基础字典。
        patch: 补丁字典，值可为 None（表示删除）。

    Returns:
        合并后的新字典。
    """
    result = dict(base)  # 不污染入参
    for k, v in patch.items():
        if v is None:
            # 删除键，若不存在也保持不存在（dict.pop 的默认行为）
            result.pop(k, None)
        else:
            result[k] = v
    return result

def diff_summary(d: Dict[str, Dict[str, Any]]) -> Dict[str, int]:
    """
    统计 diff 结果中各类条目的数量。

    Args:
        d: diff() 的返回结果。

    Returns:
        {"added": int, "removed": int, "changed": int, "unchanged": int}
    """
    return {
        "added": len(d.get("added", {})),
        "removed": len(d.get("removed", {})),
        "changed": len(d.get("changed", {})),
        "unchanged": len(d.get("unchanged", {})),
    }
