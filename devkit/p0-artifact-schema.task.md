# Task: P0 artifact.py — 锁定 schema 字段

## 背景
`devkit/artifact.py` 的 `make()` 目前只有 stage/role/title/body/fields 五个字段。
ROADMAP 要求在 P0 就把"后续所有 Plane 都要读"的字段槽留好，否则 Quota/Fitness/Learning 全要回填迁移。

## 目标：修改 `make()` 函数，新增以下 schema 字段（全部可选，缺省 None）

新的 `make()` 签名：
```python
def make(stage, role, title, body, fields=None, *,
         carrier=None, task_type=None,
         tokens=None, cost=None,
         verdict=None, tests_passed=None,
         window_used=None, budget_report=None):
```

返回 dict 新增这 8 个顶级字段（与 stage/role/title/body/fields 并列）：
```python
{
    "stage": stage,
    "role": role,
    "title": title,
    "body": body,
    "fields": fields,          # 已有
    "carrier": carrier,        # 用哪个载体跑的（如 "glm", "deepseek"）
    "task_type": task_type,    # 任务类型（"backend-fix"|"test-gen"|"review"|"refactor"|"feature"|"other"）
    "tokens": tokens,          # 本产物消耗的 token 数（int|None）
    "cost": cost,              # 本产物花费 USD（float|None）
    "verdict": verdict,        # "GO"|"NO-GO"|None
    "tests_passed": tests_passed,  # True|False|None
    "window_used": window_used,    # context window 实际使用 token 数（int|None）
    "budget_report": budget_report, # {"kept":[], "dropped":[], "used":int}|None
}
```

## 约束
- 只修改 `devkit/artifact.py`，不改其他文件
- 向后兼容：`make(stage, role, title, body)` 不传新字段时所有新字段为 None
- `fields` 的处理逻辑（浅拷贝）保持不变
- 不写 unittest 块
- 输出完整 `artifact.py` 文件（文件头注释 `# devkit/artifact.py`）

## 现有 artifact.py 全文
```python
# devkit/artifact.py
"""Loom 的结构化产物总线 v1"""

import re

PROTECTED = ("contract", "failure", "patch_targets")

def make(stage, role, title, body, fields=None):
    if fields is None:
        fields = {}
    else:
        fields = dict(fields)
    return {
        "stage": stage,
        "role": role,
        "title": title,
        "body": body,
        "fields": fields,
    }

def extract_fields(stage, body):
    if stage == "implement":
        pattern = re.compile(r"^#\s*(\S+\.\S+)\s*$")
        targets = []
        for line in body.splitlines():
            match = pattern.match(line.strip())
            if match:
                targets.append(match.group(1))
        return {"patch_targets": targets}
    if stage == "review":
        for line in body.splitlines():
            stripped = line.strip()
            if "NO-GO" in stripped or "需要修改" in stripped:
                return {"failure": stripped}
        return {}
    return {}

def protected(artifact):
    source_fields = artifact.get("fields", {})
    result = {}
    for key in PROTECTED:
        if key in source_fields:
            result[key] = source_fields[key]
    return result
```
