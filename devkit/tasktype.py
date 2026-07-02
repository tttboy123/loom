# tasktype.py
"""Loom 启发式任务类型分类器。

提供 infer_task_type(text)，返回以下之一：
backend-fix / test-gen / review / refactor / feature / other。
"""

__all__ = ["infer_task_type"]

_RULES = (
    ("backend-fix", ("修复", "fix", "bug")),
    ("test-gen", ("测试", "test", "golden")),
    ("review", ("审查", "review", "评审")),
    ("refactor", ("重构", "refactor")),
    ("feature", ("实现", "新增", "add", "feature", "build")),
)

def infer_task_type(text: str) -> str:
    """根据任务描述文本启发式推断任务类型。

    英文关键词不区分大小写，中文关键词按原样子串匹配；
    按规则顺序第一个命中即返回。
    """
    lowered = text.lower()
    for task_type, keywords in _RULES:
        if any(keyword in lowered for keyword in keywords):
            return task_type
    return "other"
