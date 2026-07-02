"""devkit/task_validator.py — 纯标准库 · 验证 backlog 任务结构合法性。

公开 API:
  - validate_task(task)     -> list[str]   每条错误一条消息；空列表 = 合法
  - validate_backlog(...)   -> dict        聚合统计 + 错误明细
  - is_valid(task)          -> bool        validate_task 返回空列表时为 True

字段约束
--------
必填:
  id     : 非空字符串
  status : {"pending", "done", "failed", "running", "stopped"} 之一
可选（存在时类型必须正确）:
  deps     : list
  priority : str
  stages   : str
  task_kind : str
  delivery_mode : {"autonomous", "report-only", "apply-required", "apply-git"}
  apply_target  : str
  apply_git     : str
  apply_branch  : str
  allowed_artifact_paths   : list[str]
  forbidden_artifact_paths : list[str]
"""

from __future__ import annotations

from typing import Any

from devkit.delivery_mode import VALID_DELIVERY_MODES

VALID_STATUSES = frozenset({"pending", "done", "failed", "running", "stopped"})

def validate_task(task: dict) -> list[str]:
    """对单个任务做结构校验，返回错误消息列表。空列表表示合法。"""
    errors: list[str] = []

    # --- id: 必填 + 非空字符串 ---
    if "id" not in task:
        errors.append("missing required field: id")
    else:
        id_val = task["id"]
        if not isinstance(id_val, str) or id_val == "":
            errors.append("id must be a non-empty string")

    # --- status: 必填 + 枚举值 ---
    if "status" not in task:
        errors.append("missing required field: status")
    else:
        status = task["status"]
        if status not in VALID_STATUSES:
            errors.append(
                f"status must be one of {sorted(VALID_STATUSES)}, got {status!r}"
            )

    # --- 可选字段: 类型约束 ---
    if "deps" in task and not isinstance(task["deps"], list):
        errors.append("deps must be a list")

    if "priority" in task and not isinstance(task["priority"], str):
        errors.append("priority must be a string")

    if "stages" in task and not isinstance(task["stages"], str):
        errors.append("stages must be a string")

    if "task_kind" in task and not isinstance(task["task_kind"], str):
        errors.append("task_kind must be a string")

    if "delivery_mode" in task:
        mode = task["delivery_mode"]
        if not isinstance(mode, str):
            errors.append("delivery_mode must be a string")
        elif mode not in VALID_DELIVERY_MODES:
            errors.append(
                f"delivery_mode must be one of {sorted(VALID_DELIVERY_MODES)}, got {mode!r}"
            )

    for key in ("apply_target", "apply_git", "apply_branch"):
        if key in task and not isinstance(task[key], str):
            errors.append(f"{key} must be a string")

    for key in ("allowed_artifact_paths", "forbidden_artifact_paths"):
        if key in task:
            value = task[key]
            if not isinstance(value, list):
                errors.append(f"{key} must be a list")
            elif not all(isinstance(item, str) for item in value):
                errors.append(f"{key} must contain only strings")

    return errors

def validate_backlog(backlog: list[dict]) -> dict:
    """校验整个 backlog，返回 {valid, invalid, errors}。"""
    valid = 0
    invalid = 0
    errors: list[dict] = []

    for task in backlog:
        if not isinstance(task, dict):
            invalid += 1
            errors.append({"id": None, "errors": ["task must be a dict"]})
            continue

        task_errors = validate_task(task)
        if task_errors:
            invalid += 1
            # id 缺失或类型错时记 None，便于定位
            raw_id = task.get("id")
            entry_id = raw_id if isinstance(raw_id, str) and raw_id else None
            errors.append({"id": entry_id, "errors": task_errors})
        else:
            valid += 1

    return {"valid": valid, "invalid": invalid, "errors": errors}

def is_valid(task: dict) -> bool:
    """validate_task 返回空列表时为 True。"""
    return len(validate_task(task)) == 0

# ---------------------------------------------------------------------------
# TDD 验证入口: 10 条 golden + 合约/非 happy-path。运行即得 PASS/FAIL 汇总。
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    import sys

    _join = "\n".join

    golden: list[tuple[str, bool]] = [
        ("g1_valid_minimal",         validate_task({"id": "a", "status": "done"}) == []),
        ("g2_missing_required",      len(validate_task({})) > 0),
        ("g3_empty_id",              len(validate_task({"id": "", "status": "done"})) > 0),
        ("g4_invalid_status",        len(validate_task({"id": "a", "status": "invalid"})) > 0),
        ("g5_is_valid_true",         is_valid({"id": "a", "status": "pending"}) is True),
        ("g6_is_valid_false",        is_valid({}) is False),
        ("g7_empty_backlog_valid0",  validate_backlog([])["valid"] == 0),
        ("g8_single_valid",          validate_backlog([{"id": "a", "status": "done"}])["valid"] == 1),
        ("g9_mixed_invalid_one",     validate_backlog([{"id": "a", "status": "done"}, {}])["invalid"] == 1),
        ("g10_errors_is_list",       isinstance(validate_backlog([])["errors"], list)),
    ]

    contract: list[tuple[str, bool]] = [
        # 可选字段类型错误
        ("c_deps_type",         "deps must be a list" in _join(validate_task({"id": "x", "status": "pending", "deps": "nope"}))),
        ("c_priority_type",     "priority must be a string" in _join(validate_task({"id": "x", "status": "pending", "priority": 5}))),
        ("c_stages_type",       "stages must be a string" in _join(validate_task({"id": "x", "status": "pending", "stages": ["a"]}))),
        # 全部合法 status 被接受
        ("c_all_statuses_ok",   all(validate_task({"id": "x", "status": s}) == [] for s in ("pending", "done", "failed", "running", "stopped"))),
        # 可选字段类型正确时无错误
        ("c_optional_clean",    validate_task({"id": "x", "status": "pending", "deps": ["a"], "priority": "P1", "stages": "dev,qa"}) == []),
        # 多条错误聚合（非 happy-path）
        ("c_multi_errors",      len(validate_task({"id": "", "status": "weird", "deps": "x"})) >= 3),
        # 聚合 valid 计数
        ("c_backlog_valid_cnt", validate_backlog([{"id": "a", "status": "done"}, {"id": "b", "status": "pending"}, {}])["valid"] == 2),
        # errors 只列无效任务
        ("c_errors_only_bad",   len(validate_backlog([{"id": "a", "status": "done"}, {"id": "b", "status": "broken"}])["errors"]) == 1),
        # 非 dict 任务被记录
        ("c_non_dict_caught",   validate_backlog([{"id": "a", "status": "done"}, "junk"])["invalid"] == 1),
    ]

    all_checks = golden + contract
    failures = [name for name, ok in all_checks if not ok]
    for name, ok in all_checks:
        print(f"{'PASS' if ok else 'FAIL'}  {name}")
    print(f"\nTotal: {len(all_checks)}  Passed: {len(all_checks) - len(failures)}  Failed: {len(failures)}")
    sys.exit(1 if failures else 0)
