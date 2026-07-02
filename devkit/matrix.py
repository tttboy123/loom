# devkit/matrix.py
"""二维矩阵运算（纯标准库，嵌套 list 表示）。

设计要点：
- 所有"返回新矩阵"的函数均保证**不可变**：不修改入参、不与入参共享内层 list。
- 形状不匹配时抛 ValueError（而非静默截断），符合"不确定就明说"。
"""

def create(rows: int, cols: int, default=0):
    """返回 rows×cols 的嵌套列表，每行是独立 list，初始值为 default。"""
    if rows < 0 or cols < 0:
        raise ValueError("rows and cols must be non-negative")
    return [[default] * cols for _ in range(rows)]

def get(m, row: int, col: int):
    return m[row][col]

def set_val(m, row: int, col: int, val):
    """返回新矩阵，[row][col] 设为 val；不修改原矩阵。"""
    rows = len(m)
    cols = len(m[0]) if rows > 0 else 0
    if not (0 <= row < rows and 0 <= col < cols):
        raise IndexError(f"index ({row},{col}) out of shape ({rows},{cols})")
    # 深拷贝一层 + 单点修改
    return [row_list[:] for row_list in m] and _with_set(m, row, col, val)

def _with_set(m, row, col, val):
    # 单独抽出便于 set_val 表达（避免上面 and 链的可读性坑）
    new_m = [r[:] for r in m]
    new_m[row][col] = val
    return new_m

def add(a, b):
    """矩阵加法；要求 a、b 形状一致。"""
    if len(a) != len(b):
        raise ValueError(f"row count mismatch: {len(a)} vs {len(b)}")
    if len(a) == 0:
        return []
    if len(a[0]) != len(b[0]):
        raise ValueError(f"col count mismatch: {len(a[0])} vs {len(b[0])}")
    return [[a[i][j] + b[i][j] for j in range(len(a[0]))] for i in range(len(a))]

def transpose(m):
    """转置矩阵；不修改入参。"""
    if len(m) == 0:
        return []
    cols = len(m[0])
    # 用 * 解包保证新行是独立 list
    return [list(col) for col in zip(*m)]
