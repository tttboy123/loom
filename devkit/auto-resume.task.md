实现纯标准库新模块 `resume.py`（断点续跑：从 run 目录推断已完成阶段）。
只写这一个文件，不要新增依赖，不要写测试文件。

## 背景
Loom 自治循环可能中断（token 耗尽/超时/崩溃）。续跑时需要知道哪些阶段已完成，避免重复跑。
这是纯 IO 层（只读文件系统），无 LLM 调用。

## 里程碑

### M1: `done_stages(run_dir) -> list[str]`
读取 run_dir（pathlib.Path 或 str）下的文件，推断已完成的阶段名列表。
- 扫描规则：查找文件名匹配 `[0-9][0-9]-<name>.md` 的文件
  - **跳过** `00-task.md`（任务定义）
  - **跳过** `90-*` / `91-*` / `99-*`（迭代重跑/总结）
  - 提取 name 部分：`01-plan.md` → `"plan"`，`02-implement.md` → `"implement"`
- run_dir 不存在 → 返回 `[]`
- 按文件名数字前缀升序排序后返回

### M2: `pending_stages(run_dir, all_stages: list[str]) -> list[str]`
返回尚未完成的阶段列表。
- `all_stages` 中去掉 `done_stages(run_dir)` 已有的，保持 all_stages 原有顺序

### M3: `is_complete(run_dir) -> bool`
判断一次 run 是否已彻底完成。
- 条件：run_dir 存在 **且** 包含 `run-log.md` 文件

## 风格
纯标准库（pathlib 允许），函数短小，加简短中文 docstring，文件第一行 `# resume.py`。
