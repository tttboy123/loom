实现新模块 `registry.py`（Stage Registry：结构化阶段注册表）。只写这一个文件，不写测试文件，文件第一行 `# registry.py`。

## 背景
loom.roles.toml / loom-roles.yaml 定义了各阶段（stage）的角色。当前缺少对 trust_level、max_cost_per_run、allowed_executors 等结构化字段的标准化读取和校验。

## 接口

### `load(path=None) -> list[dict]`
读取 loom.roles.toml 或 loom-roles.yaml（按此顺序查找，path 可显式指定）。
每个 stage 条目补齐默认值后返回列表：
- `trust_level`: int，默认 1（1=低信任，2=中，3=高）
- `max_cost_per_run`: float，默认 0.05（USD）
- `allowed_executors`: list[str]，默认 ["chat"]
- 原有字段（key, title, role, carrier, system 等）原样保留

### `get(key: str, path=None) -> dict | None`
按 stage key 返回单条注册信息，不存在返回 None。

### `validate(entry: dict) -> dict`
校验一条 stage 配置，返回 `{"ok": bool, "errors": list[str]}`。
规则：
- key 必须是非空字符串
- trust_level 必须是 1/2/3
- max_cost_per_run 必须 > 0
- allowed_executors 必须是非空列表

## 风格
纯标准库（tomllib / tomlib 只在 Python ≥ 3.11 可用；3.10 用 tomli 第三方包；fallback 到读 yaml 或返回空列表），函数短小，加简短中文 docstring。
