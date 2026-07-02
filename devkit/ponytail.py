# ponytail.py
"""PonyTail 过工程审查门：防止便宜模型"为完成任务而过度膨胀 diff"。

用法（CLI）：
    devkit "任务" --ponytail [--recipe cheap-dev ...]

原理：替换 review 阶段的 system prompt 为 PonyTail 审查契约，其余不变。
review 阶段若产出 REQUEST-CHANGES，rdloop 已有 _wants_changes() 检测 → 触发 iterate。
"""
from __future__ import annotations

PONYTAIL_SYSTEM = """\
你是【PonyTail 过工程审查员】。专职：防止便宜生成模型"为完成任务而膨胀 diff"。

逐条审查（打 PASS / FAIL + 一句证据）：
1. 最小 diff：patch 只改任务要求的内容，无多余重构/重命名/格式化。
2. 零新依赖：若新增 import 或第三方包，必须有任务明确要求的理由。
3. 无多余抽象：无新增类/接口/层/工具函数 —— 除非任务明确要求封装。
4. 约束遵守：任务 spec 的所有限制（只标准库/不改函数签名/etc）均得到遵守。
5. 旗舰用例可追溯：产物里有可检验证据（测试/输出/日志）说明核心用例能跑通。

打分后输出：
- 全部 PASS → **APPROVE**
- 任意 FAIL → **REQUEST-CHANGES**（一句话：违反哪条 + 要求重实现更小 patch）

不给代码修改建议，只给 APPROVE 或 REQUEST-CHANGES + 理由。\
"""


def score_patch(diff_lines: list[str]) -> dict:
    """统计 diff 指标。返回 {added, removed, new_imports, new_files}。"""
    added = 0
    removed = 0
    new_imports = 0
    new_files = 0
    for line in diff_lines:
        if line.startswith("+++ b/"):
            new_files += 1
        elif line.startswith("+") and not line.startswith("+++"):
            added += 1
            stripped = line[1:].lstrip()
            if stripped.startswith("import ") or stripped.startswith("from "):
                new_imports += 1
        elif line.startswith("-") and not line.startswith("---"):
            removed += 1
    return {"added": added, "removed": removed, "new_imports": new_imports, "new_files": new_files}


def gate(diff_lines: list[str], max_added: int = 200, max_new_imports: int = 5) -> dict:
    """反过度工程判定。返回 {ok: bool, verdict: str, reason: str}。"""
    if not diff_lines:
        return {"ok": True, "verdict": "GO", "reason": "无改动"}
    s = score_patch(diff_lines)
    if s["added"] > max_added:
        return {"ok": False, "verdict": "NO-GO",
                "reason": f"新增行数 {s['added']} 超过上限 {max_added}"}
    if s["new_imports"] > max_new_imports:
        return {"ok": False, "verdict": "NO-GO",
                "reason": f"新引入 import 数 {s['new_imports']} 超过上限 {max_new_imports}"}
    return {"ok": True, "verdict": "GO",
            "reason": f"改动在合理范围内（+{s['added']} 行，{s['new_imports']} import）"}


def apply(stages: list) -> list:
    """把 review 阶段的 system prompt 替换成 PonyTail 审查契约，返回新 stages 列表。
    无 review 阶段则原样返回（PonyTail 只作用于 review）。"""
    import dataclasses
    result = []
    for st in stages:
        if st.key == "review":
            st = dataclasses.replace(st, system=PONYTAIL_SYSTEM)
        result.append(st)
    return result
