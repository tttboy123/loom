"""rdloop ↔ gate 的桥接：在 materialize 时把任务标记为 'report-only'。"""
from typing import Any

# 历史 rdloop 在调用 evaluate_gate 时不传 task_type → 此处补默认值
def call_gate_for_task(task: dict, *, runs_dir: str = "runs"):
    # 默认 'impl'；diag/observe/audit 类由 caller 在 materialize 时设置 'report-only'
    payload: dict[str, Any] = {
        "task_id": task["task_id"],
        "task_type": task.get("task_type", "impl"),
        "candidate_state": task.get("candidate_state", "materialized"),
        "acceptance_keywords": task.get("acceptance_keywords", []),
    }
    # 延迟导入，避免 gate 模块缺位时 rdloop 整体崩溃
    from devkit import harness_gate
    return harness_gate.evaluate_gate(payload, runs_dir=runs_dir)
