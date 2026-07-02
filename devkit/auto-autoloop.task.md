实现纯标准库新模块 `autoloop.py`（自治驱动循环纯逻辑核心）。
只写这一个文件，不要新增依赖，不要写测试文件。

## 背景
Loom 长时自治 Agent 需要一个驱动循环：从 backlog 取下一个任务、推进状态机、结构化单次运行参数。
这是纯逻辑层（无 IO、无 LLM 调用），便于测试和独立验证。

## 里程碑

### M1: `pick_next(backlog: list[dict]) -> dict | None`
从 backlog 中选下一个可执行任务。
- backlog item 结构：`{"id": str, "status": str, "deps": list[str], ...}`
- status 可取值：`"pending"` / `"running"` / `"done"` / `"failed"` / `"stopped"`
- 选取规则：status == "pending" **且** 所有 deps 在 backlog 中 status == "done"
- 按 backlog 列表顺序取第一个满足条件的；无则返回 None

### M2: `advance_state(state: str, event: str) -> str`
任务状态机推进。
- 转移规则：
  - `"pending"` + `"start"` → `"running"`
  - `"running"` + `"success"` → `"done"`
  - `"running"` + `"failure"` → `"failed"`
  - `"running"` + `"stop"` → `"stopped"`
  - 终止状态（done/failed/stopped）+ 任意事件 → 保持不变
  - 非法 state/event → 返回 state 不变（容错）

### M3: `run_once(task_spec: dict) -> dict`
将任务 spec 结构化为一次 devkit 运行的参数字典。
- 输入键：`"task"`(str)、`"stages"`(str 可选)、`"carrier"`(dict 可选)、`"run_id"`(str 可选)
- 输出：`{"task": str, "stages": str, "carriers": list[str], "run_id": str}`
  - stages 默认 `"plan,implement,verify"`
  - carriers 由 task_spec["carrier"] dict 展开：`{"implement": "deepseek"}` → `["implement=deepseek"]`
  - run_id 默认 `"auto-" + datetime.now().strftime("%Y%m%d-%H%M%S")`

## 风格
纯标准库（datetime 允许），函数短小，加简短中文 docstring，文件第一行 `# autoloop.py`。
