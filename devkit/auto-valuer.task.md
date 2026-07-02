实现纯标准库新模块 `valuer.py`（候选任务价值评分器）。
只写这一个文件，不要新增依赖，不要写测试文件。

## 背景
Loom 自治体从 discover 拿到候选任务后，需要自动评分，让最有价值的任务先跑。
评分必须引用真实信号（ok_rate/runs/priority），不能靠 LLM 编造分数。

### M1: `score(candidate: dict, evidence: dict) -> dict`
给单个候选打分，返回含分数和理由的结果。
- `candidate` 键：`"type"` / `"task_type"` / `"backend"` / `"detail"` / `"priority"`
- `evidence` 键（从 fitness/learn 来，全部可选）：
  - `"ok_rate"`: 当前 ok_rate（0-100），越低越值得改（越急迫）
  - `"runs"`: 历史运行次数，越多证据越充分
  - `"priority"`: "high"/"medium"/"low"
- 评分公式（0-100 整数，**不得依赖随机**）：
  - 基础分 = 50
  - ok_rate < 30 → +25；30-50 → +15；50-70 → +5；>70 → -10
  - priority == "high" → +20；"medium" → +5；"low" → -10
  - runs >= 5 → +10（充分证据加分）；runs == 0 → -15（无历史数据减分）
  - 最终 clamp 到 [0, 100]
- 返回：`{"score": int, "reason": str, "candidate": candidate}`
  - reason 简述得分依据（英文或中文均可）

### M2: `rank(candidates: list[dict], evidences: list[dict]) -> list[dict]`
批量评分后按分数降序排列，返回评分后的列表。
- candidates 和 evidences 一一对应（长度可不同，多余的 evidence 或 candidate 补空 dict）
- 按 `score` 字段降序排列，分数相同保持原有顺序（稳定排序）

### M3: `top_n(candidates: list[dict], evidences: list[dict], n: int = 3) -> list[dict]`
返回评分最高的前 n 个候选（含分数字段）。

## 风格
纯标准库，函数短小，加简短中文 docstring，文件第一行 `# valuer.py`。
