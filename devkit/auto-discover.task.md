实现纯标准库新模块 `discover.py`（从历史数据发现下一个该建的候选任务）。
只写这一个文件，不要新增依赖，不要写测试文件。

## 背景
Loom 自治体需要"自我发现"：从 model_fitness/learn 的历史数据里找出能力缺口，
生成候选任务，送给 valuer 评分后再让人类排序。这是纯分析层（只读文件，无 LLM 调用）。

## 里程碑

### M1: `from_fitness(fitness_rows: list[dict]) -> list[dict]`
从 model_fitness 行找出能力缺口候选。
- `fitness_rows` 结构：每项有 `"task_type"` / `"backend"` / `"ok_rate"` (0-100) / `"runs"` (int)
- 规则：
  - `ok_rate < 50` 且 `runs >= 2` → 生成候选 `{"type":"improve_carrier","task_type":t,"backend":b,"ok_rate":r}`
  - `runs == 0` 的 task_type → 生成候选 `{"type":"add_coverage","task_type":t}`
- 去重（同 type+task_type+backend 只保留 ok_rate 最低的）
- 按 ok_rate 升序排列（最差的排前面）

### M2: `from_suggestions(suggestions: list[dict]) -> list[dict]`
从 learn.analyze() 的 suggestions 列表提取可行任务候选。
- `suggestions` 结构：每项有 `"type"` / `"detail"` / `"priority"` (high/medium/low)
- 映射规则：
  - `type == "carrier"` → `{"type":"switch_carrier","detail":detail,"priority":priority}`
  - `type == "golden"` → `{"type":"fix_golden","detail":detail,"priority":priority}`
  - `type == "quota"` → `{"type":"manage_quota","detail":detail,"priority":priority}`
  - 其他 → 原样透传，补充 `"type"` 字段
- 只保留 priority 为 "high" 或 "medium" 的

### M3: `merge(candidates: list[list[dict]], max_total: int = 10) -> list[dict]`
合并多个候选来源，去重，截取前 max_total 个。
- 各列表依次 round-robin 交织（先取第一个列表的第一项，再取第二个的第一项，...）
- 已见 `{"type","task_type","backend"}` 三元组相同的跳过（缺失键视为空串比较）
- 最多返回 max_total 项

## 风格
纯标准库，函数短小，加简短中文 docstring，文件第一行 `# discover.py`。
