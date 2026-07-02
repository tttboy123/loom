"""log_formatter.py

纯标准库实现的日志格式化工具。
"""

from __future__ import annotations

import re
from typing import List, Mapping, Optional

_LEVEL_RE = re.compile(r"^\[([^\]]+)\]")

def format_line(
    level: str,
    msg: str,
    context: Optional[Mapping[str, object]] = None,
) -> str:
    """格式化单条日志行。

    格式：
        '[{LEVEL}] {msg}'
        '[{LEVEL}] {msg} | k1=v1,k2=v2,...'   (按 key 排序)

    - level 会被转为大写；
    - context 为空或为 None 时不附加 '| ...' 段。
    """
    head = f"[{level.upper()}] {msg}"
    if not context:
        return head

    parts = [f"{k}={context[k]}" for k in sorted(context.keys())]
    return f"{head} | {','.join(parts)}"

def format_batch(entries: List[Mapping[str, object]]) -> str:
    """批量格式化日志条目。

    每项必须包含 'level' 与 'msg' 两个键。
    返回的字符串每行一条日志；空列表返回空字符串。
    """
    if not entries:
        return ""

    lines = [format_line(str(entry["level"]), str(entry["msg"])) for entry in entries]
    return "\n".join(lines)

def parse_level(line: str) -> str:
    """从形如 '[INFO] xxx' 的行首解析出 level。

    未匹配到 '[...]' 时返回 'UNKNOWN'。
    """
    if not isinstance(line, str):
        return "UNKNOWN"
    m = _LEVEL_RE.match(line)
    if not m:
        return "UNKNOWN"
    return m.group(1).upper()

__all__ = ["format_line", "format_batch", "parse_level"]
