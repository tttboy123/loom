"""devkit/compact_log.py — 压缩长 Markdown 为摘要，节省 LLM 上下文窗口。"""
from __future__ import annotations

import re

_HEADER_RE = re.compile(r"^#{2,3}\s+(.+)$", re.MULTILINE)
_FENCE_RE = re.compile(r"```[^\n]*\n(.*?)```", re.DOTALL)
_FILENAME_RE = re.compile(r"^#\s*(\S+\.py)\s*$")

def extract_headers(md: str) -> list[str]:
    """提取 ## 和 ### 标题文本（不含 # 号和空格）。"""
    return _HEADER_RE.findall(md or "")

def extract_code_files(md: str) -> list[str]:
    """提取所有围栏代码块内、首行形如 `# path/to/file.py` 的文件名列表。"""
    out: list[str] = []
    for m in _FENCE_RE.finditer(md or ""):
        first_line = m.group(1).split("\n", 1)[0].strip()
        fm = _FILENAME_RE.match(first_line)
        if fm:
            out.append(fm.group(1))
    return out

def compress(md: str, max_chars: int = 500) -> str:
    """原文本已够短则原样返回，否则返回不超过 max_chars 的摘要。"""
    if len(md) <= max_chars:
        return md
    summary = "\n".join(extract_headers(md) + extract_code_files(md))
    return summary[:max_chars]
