# Task: carrier_router.py — 多 Carrier 负载均衡路由

## 背景
Loom 研发循环 (rdloop.py) 目前对每个 stage 只能静态指定一个 carrier（如 glm / deepseek / minimax）。
历史上 GLM 对某些 task_type 成功率偏低，MiniMax 更贵但质量更高。
需要一个路由模块，基于历史 ok_rate + 成本自动选出最优 carrier，并在主 carrier 失败时自动切换。

## 要实现的文件
`devkit/carrier_router.py`

## 函数签名（必须完全符合）

```python
def select(stage: str, candidates: list[str], history_rows: list[dict]) -> str:
    """从候选 carriers 中选出最优 carrier。
    
    - 按 ok_rate 降序排列候选
    - ok_rate 相同时按 avg_cost 升序（便宜优先）
    - 若 history_rows 中某 carrier 无数据，视为 ok_rate=0.5（中性默认）
    - candidates 为空时 raise ValueError
    - 返回被选中的 carrier 名
    """

def fallback_chain(stage: str, candidates: list[str], history_rows: list[dict]) -> list[str]:
    """返回 carriers 按优先级排列的列表（select() 的结果在最前）。
    
    - 长度等于 candidates
    - 顺序：最优 → 次优 → ...
    - 若 candidates 只有一个，返回 [that_carrier]
    """

def score_carrier(carrier: str, stage: str, history_rows: list[dict]) -> dict:
    """计算单个 carrier 在某 stage 的评分，返回 {carrier, ok_rate, avg_cost, runs, score}。
    
    - score = ok_rate * 100 - avg_cost * 1000（加权公式，数值越高越好）
    - runs = 历史中匹配 (carrier, stage) 的记录数
    - 无历史数据：ok_rate=0.5, avg_cost=0.002, runs=0
    """
```

## history_rows 格式（每行一个字典）
```json
{"carrier": "glm", "stage": "implement", "ok_rate": 0.8, "avg_cost": 0.001, "runs": 10}
```

## 测试 golden 案例

实现完成后，以下断言必须全部通过：

1. `select("implement", ["glm", "deepseek"], [{"carrier":"glm","stage":"implement","ok_rate":0.9,"avg_cost":0.001,"runs":5}, {"carrier":"deepseek","stage":"implement","ok_rate":0.7,"avg_cost":0.002,"runs":3}])` → `"glm"`
2. `select("implement", ["glm", "deepseek"], [])` → 返回某个 str（无历史时默认选第一个或随机，但必须在 candidates 中）
3. `fallback_chain("implement", ["glm", "deepseek", "minimax"], [{"carrier":"minimax","stage":"implement","ok_rate":0.95,"avg_cost":0.005,"runs":10}])` → 第一个元素是 `"minimax"`
4. `len(fallback_chain("plan", ["glm", "deepseek"], []))` → `2`
5. `select("plan", [], [])` → 抛出 `ValueError`
6. `score_carrier("glm", "implement", [{"carrier":"glm","stage":"implement","ok_rate":0.8,"avg_cost":0.001,"runs":5}])["ok_rate"]` → `0.8`
7. `score_carrier("new_model", "implement", [])["runs"]` → `0`
8. `score_carrier("new_model", "implement", [])["ok_rate"]` → `0.5`
9. `fallback_chain("impl", ["only"], [])` → `["only"]`
10. 高 ok_rate 低 cost 的 carrier score > 低 ok_rate 高 cost 的 carrier score

## 约束
- 纯标准库，不依赖第三方包
- 不依赖 devkit 其他模块（standalone）
- 函数签名必须与上述完全一致
- 只输出 `carrier_router.py` 文件内容，不需要测试文件
