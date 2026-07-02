在现有模块 `budget.py` 中**新增**一个函数 `pack`。其余已有代码（carrier_window / est_tokens /
budget_tokens / 常量）必须**原样保留、一字不改**。只输出这一个完整的 `budget.py` 文件，纯标准库。

## 当前 budget.py（保留这些，在末尾加 pack）
```python
# budget.py

import math

DEFAULT_WINDOW = 32768
CARRIER_WINDOWS = {
    "claude": 128000, "claude-code-sub": 128000,
    "codex": 128000, "codex-sub": 128000,
    "glm": 128000, "deepseek": 65536, "minimax": 32768,
}

def carrier_window(carrier: str) -> int:
    return CARRIER_WINDOWS.get(carrier, DEFAULT_WINDOW)

def est_tokens(text: str) -> int:
    cjk_count = sum(1 for ch in text if '一' <= ch <= '鿿')
    non_cjk_count = len(text) - cjk_count
    return cjk_count + math.ceil(non_cjk_count / 3.5)

def budget_tokens(window: int, reserve: float = 0.4) -> int:
    return int(window * (1 - reserve))
```

## 里程碑（验收点）
- M1：`def pack(blocks, budget, est=est_tokens) -> dict`。`blocks` 是 dict 列表，每项
  `{"name": str, "text": str, "prio": int, "protected": bool}`。
- M2：所有 `protected=True` 的块**无条件保留**（即使单独就超预算）。
- M3：其余块按 `prio` **升序**（数字小=优先级高）依次尝试加入；当“已保留块的 est 总和 + 该块 est ≤ budget”
  时保留，否则丢弃该块并**继续看下一个**（不中断）。
- M4：返回 `{"kept": [...名字...], "dropped": [...名字...], "used": int, "text": str}`。
  `kept`/`text` 按**输入列表的原始顺序**排列（不是按 prio）；`text` 用 `"\n\n"` 连接保留块的 text；
  `used` = 保留块 est 之和（int）。
- M5：`est` 是估算 token 的函数，默认用本模块 `est_tokens`；测试会注入 `est=len`，逻辑必须照用注入的函数。

## 风格
- 纯标准库，函数短小、加简短中文 docstring，文件第一行 `# budget.py`。
