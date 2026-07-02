# Task: P2 Learning Loop — `recommend_model()` + `devkit recommend`

## 背景
`devkit/insight.py` 已有 `model_fitness(runs_dir)` 函数，返回按 (backend, task_type) 分桶的历史成功率。
目标：新增 `recommend_model(task, runs_dir)` 函数，从历史数据里自动推荐最适合当前任务的 backend；
并在 `devkit/__main__.py` 新增 `devkit recommend "<任务>"` 子命令。

## 需要修改的两个文件

### 文件 A：`devkit/insight.py`

在 `model_fitness` 函数之后、`runs_list` 函数之前新增：

```python
def recommend_model(task: str,
                    runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """根据历史 model_fitness 数据推荐最适合当前任务的 backend。

    流程：
    1. 从 task 文本推断 task_type（调用 devkit.tasktype.infer_task_type）
    2. 调用 model_fitness(runs_dir) 获取历史分桶数据
    3. 筛选 task_type 匹配的行（uses >= 1）
    4. 按 ok_rate 降序选最优 backend
    5. 若无历史数据，返回 backend=None

    返回：
    {
        "task_type": str,
        "backend": str | None,       # 推荐后端，无数据时为 None
        "ok_rate": int | None,       # 该后端在此类任务的历史成功率（%）
        "avg_cost": float | None,    # 该后端在此类任务的历史均成本
        "uses": int,                 # 历史样本数（0 表示无数据）
        "reason": str,               # 一句推荐理由（或"无历史数据"）
    }
    """
```

实现要点：
- 调用 `from devkit.tasktype import infer_task_type`，若导入失败则 task_type="unknown"
- 调用 `model_fitness(runs_dir)["rows"]`，所有异常在函数内部捕获
- 无匹配行时：`{"task_type": ..., "backend": None, "ok_rate": None, "avg_cost": None, "uses": 0, "reason": "无历史数据"}`
- 有匹配行时选 ok_rate 最高的，reason 形如 `"历史 3 次，成功率 85%，均 $0.00123"`

### 文件 B：`devkit/__main__.py`

在 `main()` 里加路由（在 `if argv and argv[0] == "runs":` 之后）：
```python
if argv and argv[0] == "recommend":
    return _cmd_recommend(argv[1:])
```

新增 `_cmd_recommend(argv) -> int` 函数（加在 `_cmd_runs` 之前）：

```python
def _cmd_recommend(argv) -> int:
    """推荐最适合当前任务的 backend（基于历史 model_fitness 数据）。"""
    from devkit import insight
    p = argparse.ArgumentParser(prog="devkit recommend",
                                description="根据历史数据推荐最适合当前任务的 backend")
    p.add_argument("task", help="任务描述（用于推断任务类型）")
    p.add_argument("--runs-dir")
    a = p.parse_args(argv)
    r = (insight.recommend_model(a.task, pathlib.Path(a.runs_dir))
         if a.runs_dir else insight.recommend_model(a.task))
    print(f"任务类型：{r['task_type']}")
    if r["backend"] is None:
        print("推荐：无历史数据，建议先用 deepseek（便宜）跑几次积累 runs。")
        print("提示：`devkit fitness` 查看积累后的分桶数据。")
    else:
        ok = f"{r['ok_rate']}%" if r["ok_rate"] is not None else "—"
        cost = f"${r['avg_cost']:.5f}" if r["avg_cost"] is not None else "—"
        print(f"推荐：{r['backend']}  （成功率 {ok} · 均成本 {cost} · 样本 {r['uses']} 次）")
        print(f"理由：{r['reason']}")
        print(f"\n用法：devkit \"任务\" --carrier implement={r['backend']}")
    return 0
```

## 约束
- 只修改 `devkit/insight.py` 和 `devkit/__main__.py`
- 只用标准库（devkit.tasktype 已有，可直接 import）
- 所有异常在函数内部捕获，绝不往外抛
- **绝对不能改 `model_fitness()`、`runs_list()` 或任何已有函数的签名**
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/insight.py` 和 `# devkit/__main__.py` 开头，产出完整文件
- 网关：http://localhost:4000
- 级别：L1 / report-only
