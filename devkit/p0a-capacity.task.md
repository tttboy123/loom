实现一个纯标准库的新模块 `capacity.py`（Loom 额度容量预判 v1 的纯逻辑，不联网）。
只写这一个文件，不要新增依赖，不要写测试。

## 背景
任务开始前，根据"预计成本"和"剩余额度"判断能不能跑完。订阅型后端查不到余额时记 Unknown。

## 里程碑（验收点）
- M1：`def estimate(stage_avgs) -> float`
  `stage_avgs` 是各阶段平均成本的浮点列表。返回 `round(sum(stage_avgs, 0.0), 5)`
  （用 0.0 起始保证空列表返回 `0.0` 而不是 `0`）。
- M2：`def verdict(est_cost, remaining) -> str`
  按下面**顺序**判定并返回字符串：
  1. `remaining is None` → `"Unknown"`（订阅型查不到余额）
  2. `est_cost <= 0` → `"Safe"`
  3. `est_cost > remaining` → `"Insufficient"`
  4. `est_cost <= remaining * 0.5` → `"Safe"`
  5. 其它（即 `0.5*remaining < est_cost <= remaining`）→ `"Risky"`
  例：`verdict(3, 10)=="Safe"`；`verdict(7, 10)=="Risky"`；`verdict(12, 10)=="Insufficient"`；
  `verdict(0.0, 10)=="Safe"`；`verdict(5, None)=="Unknown"`。

## 风格
- 纯标准库，函数短小、加简短中文 docstring，文件第一行 `# capacity.py`。
