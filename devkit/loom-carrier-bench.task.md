# Task: carrier_bench.py + devkit bench 子命令

## 背景
Loom 现在有多个 carrier（minimax/glm/deepseek），需要一个基准测试工具，
量化各 carrier 在不同 stage 类型（plan/implement/verify）上的 ok_rate/latency/cost，
结果写入本地缓存供 carrier_router 参考。

## 要实现的文件
`devkit/carrier_bench.py`（纯标准库，不依赖第三方包）

## 函数签名（必须完全符合）

```python
def load_tasks(tasks_path: str | None = None) -> list[dict]:
    """从 devkit/bench_tasks.json 读取 benchmark 任务列表。
    
    文件不存在时返回内置默认任务列表（至少 3 条，覆盖 plan/implement/verify）。
    每条任务格式：{"id": str, "stage": str, "prompt": str}
    """

def run_bench(
    carriers: list[str],
    base_url: str,
    api_key: str,
    tasks: list[dict] | None = None,
    timeout: int = 60,
) -> dict:
    """对每个 carrier 依次跑所有 benchmark 任务，收集结果。
    
    返回 {carrier: {stage: {ok_rate: float, avg_latency_ms: float, avg_cost: float, runs: int}}}
    - 每个 (carrier, stage) 组合跑所有该 stage 的任务
    - ok_rate = 成功请求数 / 总请求数
    - 不依赖 LiteLLM 网关，直接用 carrier_health.probe() 做可达性判断
    - 实际请求通过 urllib 发送到 base_url/chat/completions
    """

def save_results(results: dict, out_path: str | None = None) -> None:
    """将结果写入 devkit/carrier_bench.json，带时间戳。"""

def load_results(out_path: str | None = None) -> dict:
    """读取上次 bench 结果，文件不存在返回 {}。"""

def print_table(results: dict) -> None:
    """打印对比表：每行一个 carrier，每列一个 stage，显示 ok_rate/latency/cost。
    
    格式示例：
    carrier       plan(ok/lat/cost)   implement(ok/lat/cost)   verify(ok/lat/cost)
    ──────────────────────────────────────────────────────────────────────────────
    minimax       100%/1.2s/$0.00     90%/3.5s/$0.00           95%/2.1s/$0.00
    glm           80%/0.8s/$0.00      70%/2.1s/$0.00           75%/1.5s/$0.00
    """
```

## 测试 golden 案例（必须全部通过）

1. `load_tasks()` 返回非空 list
2. `load_tasks()` 每条有 id/stage/prompt 字段
3. `load_results('/tmp/nonexistent_bench_xyz.json')` → `{}`
4. `save_results({'glm': {}}, '/tmp/test_bench_save.json')` 后 `load_results('/tmp/test_bench_save.json')['glm']` → `{}`
5. `run_bench([], 'http://x', 'k')` → `{}`（空 carriers 返回空结果）
6. `run_bench(['bad_carrier'], 'http://127.0.0.1:19999', 'k', timeout=1)` 返回 dict（不抛异常）
7. `run_bench(['bad_carrier'], 'http://127.0.0.1:19999', 'k', timeout=1)['bad_carrier']` 包含至少一个 stage 键
8. 每个 stage 结果包含 ok_rate/avg_latency_ms/avg_cost/runs 四个字段
9. ok_rate 在 [0.0, 1.0] 范围内
10. `print_table({})` 不抛异常（空结果静默）

## 约束
- 纯标准库（urllib），不依赖第三方包
- 不依赖 devkit 其他模块，除了 carrier_health.probe（可以直接复制 probe 逻辑而非 import）
- 只输出 `carrier_bench.py` 文件内容，不需要测试文件
