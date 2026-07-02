实现新模块 `artifact_bus.py`（结构化 Artifact 交接总线）。只写这一个文件，不写测试文件，文件第一行 `# artifact_bus.py`。

## 背景
Loom 各阶段产出物（Plan/Patch/VerifyReport）目前以 markdown 字符串拼接传递，容易丢失结构。
artifact_bus.py 提供两个转换函数，使阶段间可以用结构化 JSON 交接而非纯文本。

## 接口

### `to_bus_message(art: dict) -> dict`
把 artifact.make() 的产物字典序列化成 Bus 消息格式：
```python
{
  "type": "artifact",
  "stage": art["stage"],      # 如 "plan" / "implement" / "verify"
  "role": art["role"],
  "title": art["title"],
  "carrier": art.get("carrier",""),
  "task_type": art.get("task_type",""),
  "tokens": art.get("tokens") or 0,
  "cost": art.get("cost") or 0.0,
  "verdict": art.get("verdict"),          # "GO" / "NO-GO" / None
  "tests_passed": art.get("tests_passed"),
  "body_digest": _digest(art.get("body",""))  # SHA-256 前 8 hex，用于快速去重
}
```

### `from_bus_message(msg: dict) -> dict`
把 Bus 消息还原为 artifact 字典（body 字段不在 bus 消息里，需调用方自己补）。
返回与 artifact.make() 相同结构的 dict（body 默认为 ""）。

### `_digest(text: str) -> str`
内部函数：对 text UTF-8 编码后 SHA-256，取前 8 个 hex 字符。

### `merge(arts: list[dict]) -> dict`
把多个阶段的 artifact 合并成一个汇总 dict：
- `stages`: list of stage names
- `total_tokens`: sum
- `total_cost`: sum
- `go_count` / `nogo_count`: 各阶段 verdict 统计
- `verdicts`: {stage: verdict} mapping

## 风格
纯标准库（hashlib），函数短小，中文 docstring，不写测试文件。
