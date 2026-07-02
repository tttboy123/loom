# Task: P2 Safety Gate Hard Mode — violations → NO-GO + 更多规则

## 背景
`devkit/safety.py` 已有 4 条规则和 `scan_build(build_dir)` 函数。
目前 `--safety` flag 只打印警告，不影响最终 gate。
目标：
1. 在 `devkit/safety.py` 新增 2 条规则（S005 / S006）
2. 新增 `severity` 字段（"error" | "warn"）：S001/S002/S003 = error，S004/S005/S006 = warn
3. 在 `scan_build()` 返回值增加 `"has_errors": bool`（是否有 error 级违规）
4. 在 `devkit/__main__.py` 的 `_cmd_run()` 里：若 `--safety` 且 `has_errors=True`，打印 NO-GO 并让返回码变 1

## 只修改一个文件：`devkit/safety.py`

### 新增规则

| rule_id | 名称 | 正则 | severity |
|---|---|---|---|
| S005 | eval-injection | `eval\s*\(` | warn |
| S006 | pickle-load | `pickle\.loads?\s*\(` | warn |

把 `severity` 加进每条规则：
- S001 hardcoded-secret → error
- S002 shell-injection → error
- S003 os-system → error
- S004 sql-concat → warn
- S005 eval-injection → warn
- S006 pickle-load → warn

### 修改 `scan_build()` 返回值

violations 每项增加 `"severity"` 字段（来自该 rule 的 severity）：
```python
violations.append({
    "file": rel_path,
    "line": lineno,
    "rule": rule["rule_id"],
    "severity": rule["severity"],
    "snippet": line.rstrip("\n")[:120].strip(),
})
```

返回值增加：
```python
"has_errors": any(v["severity"] == "error" for v in violations),
"rules_applied": len(RULES),   # 现在是 6
```

## 约束
- **只修改 `devkit/safety.py`**，不改其他文件
- 只用标准库（re / pathlib）
- 所有文件读取异常在内部捕获，不抛出
- `rules_applied` 固定返回 `len(RULES)`（规则数量）
- 不写 unittest 块
- 输出一个代码块，以 `# devkit/safety.py` 开头，产出完整文件
- 网关：http://localhost:4000
- 级别：L1 / report-only
