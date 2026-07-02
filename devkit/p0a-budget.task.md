实现一个纯标准库的 Python 模块 `budget.py`（Loom 上下文预算器 v1 的底层度量）。
只写这一个文件，不要新增依赖，不要写测试（测试由 Eval Gate 提供）。可以 `import math`。

## 背景（为什么）
不同载体的上下文窗口从 32k 到 128k 浮动。给中文估 token 时，绝不能用"字符数 / 3.5"
（那是英文比例）——中文 CJK 字符约 1 个字 ≈ 1 个 token，用 3.5 会把 token 数严重低估、
导致塞爆窗口。所以本模块要按 CJK / 非 CJK 分别估。

## 模块要求

1. 模块级常量：
   ```
   DEFAULT_WINDOW = 32768
   CARRIER_WINDOWS = {
       "claude": 128000, "claude-code-sub": 128000,
       "codex": 128000, "codex-sub": 128000,
       "glm": 128000, "deepseek": 65536, "minimax": 32768,
   }
   ```

2. `def carrier_window(carrier) -> int`
   返回该载体的上下文窗口大小：在 `CARRIER_WINDOWS` 里就返回对应值，否则返回 `DEFAULT_WINDOW`。

3. `def est_tokens(text) -> int`
   保守估计 text 的 token 数，按 CJK / 非 CJK 分别算：
   - CJK 字符：码点落在 `'一' <= ch <= '鿿'` 的字符，每个算 **1** 个 token。
   - 其余字符：字符数除以 **3.5**，再用 `math.ceil` **向上取整**。
   - 两部分相加返回 int。
   - 例：`est_tokens("")==0`；`est_tokens("hello world")==4`（11 个非 CJK 字符，ceil(11/3.5)=4）；
     `est_tokens("中文")==2`；`est_tokens("中文abc")==3`（2 + ceil(3/3.5)=2+1）。

4. `def budget_tokens(window, reserve=0.4) -> int`
   给输出留 `reserve` 比例的余量，返回可用于输入的 token 预算：`int(window * (1 - reserve))`。
   例：`budget_tokens(100000)==60000`；`budget_tokens(100000, 0.5)==50000`。

## 风格
- 纯标准库（`import math` 可用）。
- 函数短小、可读、加简短中文 docstring。
- 文件第一行用注释写出文件名：`# budget.py`
