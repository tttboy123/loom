"""devkit 统一输出格式化（纯标准库）。

四个函数：
  - fmt_table   固定宽度文本表格
  - fmt_status  ✓/✗ 状态标识
  - fmt_progress 进度文案
  - fmt_json    ensure_ascii=False 的 JSON 序列化
"""
from __future__ import annotations

import json
from typing import Any, Dict, Iterable, List

def fmt_table(rows: List[Dict[str, Any]], cols: List[str]) -> str:
    """按 cols 列顺序输出固定宽度文本表格（含表头行和分隔线）。

    - 缺失字段显示为 '-'。
    - cols 为空时返回 '(no columns)'。
    - 每列宽度 = max(len(表头), max(len(各行该列值))).
    """
    if not cols:
        return '(no columns)'

    # 1) 计算每列宽度
    widths: List[int] = []
    for col in cols:
        w = len(col)
        for row in rows:
            val = row.get(col, '-')
            w = max(w, len(str(val)))
        widths.append(w)

    # 2) 行渲染：左对齐，列间用 ' | ' 分隔
    def render_row(values: Iterable[Any]) -> str:
        cells = [str(v).ljust(widths[i]) for i, v in enumerate(values)]
        return ' | '.join(cells)

    sep = '-+-'.join('-' * w for w in widths)
    lines = [
        render_row(cols),
        sep,
    ]
    for row in rows:
        lines.append(render_row(row.get(col, '-') for col in cols))

    return '\n'.join(lines)

def fmt_status(label: str, ok: bool) -> str:
    """ok=True → '✓ {label}'；ok=False → '✗ {label}'。"""
    mark = '✓' if ok else '✗'
    return f'{mark} {label}'

def fmt_progress(done: int, total: int) -> str:
    """返回 '{done}/{total} ({pct:.0f}%)'；total=0 返回 '0/0 (-%)'。"""
    if total == 0:
        return '0/0 (-%)'
    pct = (done / total) * 100
    return f'{done}/{total} ({pct:.0f}%)'

def fmt_json(obj: Any, indent: int = 2) -> str:
    """json.dumps 的薄封装：ensure_ascii=False 以保留中文/Unicode 字面。"""
    return json.dumps(obj, ensure_ascii=False, indent=indent)
