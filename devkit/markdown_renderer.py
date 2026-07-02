# devkit/markdown_renderer.py
# Markdown 子集渲染为纯文本（纯标准库）。

import re

def render(md: str) -> str:
    """Markdown 子集 → 纯文本。

    支持：
      - ``# H1`` → 保留文本并附加换行
      - ``## H2`` / ``### H3`` → 保留文本
      - ``**bold**`` → 去掉 ``**``
      - ``*italic*`` → 去掉 ``*``
      - ```code``` → 去掉 `````
      - ``- item`` → 保留文本
      - ``> quote`` → 保留文本
    """
    if md is None:
        return ""

    lines = md.split("\n")
    out_lines = []

    for raw in lines:
        line = raw

        # > quote → 保留文本
        m_quote = re.match(r"^>\s?(.*)$", line)
        if m_quote is not None:
            line = m_quote.group(1)

        # # H1 / ## H2 / ### H3 → 保留文本，H1 附加换行
        m_head = re.match(r"^(#{1,6})\s+(.*)$", line)
        if m_head is not None:
            level = len(m_head.group(1))
            text = m_head.group(2)
            line = text + ("\n" if level == 1 else "")

        # - item → 保留文本
        m_item = re.match(r"^-\s+(.*)$", line)
        if m_item is not None:
            line = m_item.group(1)

        # 行内格式：`code` → 去反引号；**bold** → 去 **；*italic* → 去 *
        line = re.sub(r"`+", "", line)
        line = re.sub(r"\*+", "", line)

        out_lines.append(line)

    return "\n".join(out_lines)

def extract_headings(md: str) -> list[str]:
    """返回所有 # / ## / ### 标题的纯文本列表。"""
    if md is None:
        return []

    headings = []
    for line in md.split("\n"):
        m = re.match(r"^(#{1,3})\s+(.*)$", line)
        if m is not None:
            headings.append(m.group(2).strip())
    return headings

def extract_links(md: str) -> list[dict]:
    """提取 ``[text](url)`` 链接，返回 ``[{"text": ..., "url": ...}, ...]``。"""
    if md is None:
        return []

    pattern = re.compile(r"\[([^\]]*)\]\(([^)]+)\)")
    return [{"text": m.group(1), "url": m.group(2)} for m in pattern.finditer(md)]
