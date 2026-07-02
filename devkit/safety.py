# devkit/safety.py
"""轻量安全扫描：对 build/ 产物检查硬编码密钥 / shell 注入 / SQL 注入模式。"""

import re
import pathlib
from typing import Dict, List, Any

# 扫描规则（按 rule_id 排序，IGNORECASE）
RULES = [
    {
        "rule_id": "S001",
        "name": "hardcoded-secret",
        "severity": "error",
        "pattern": re.compile(
            r"[A-Z_]{3,}_(?:KEY|TOKEN|SECRET|PASSWORD)\s*=\s*['\"][A-Za-z0-9+/=_\-]{20,}['\"]",
            re.IGNORECASE,
        ),
    },
    {
        "rule_id": "S002",
        "name": "shell-injection",
        "severity": "error",
        "pattern": re.compile(
            r"subprocess\.(?:run|call|Popen|check_output)\([^)]*shell\s*=\s*True",
            re.IGNORECASE,
        ),
    },
    {
        "rule_id": "S003",
        "name": "os-system",
        "severity": "error",
        "pattern": re.compile(r"os\.system\s*\(", re.IGNORECASE),
    },
    {
        "rule_id": "S004",
        "name": "sql-concat",
        "severity": "warn",
        "pattern": re.compile(r"(?:execute|executemany)\s*\([^)]*\+", re.IGNORECASE),
    },
    {
        "rule_id": "S005",
        "name": "eval-injection",
        "severity": "warn",
        "pattern": re.compile(r"eval\s*\(", re.IGNORECASE),
    },
    {
        "rule_id": "S006",
        "name": "pickle-load",
        "severity": "warn",
        "pattern": re.compile(r"pickle\.loads?\s*\(", re.IGNORECASE),
    },
]


def scan_build(build_dir: pathlib.Path) -> Dict[str, Any]:
    """扫描 build_dir 下所有 .py 文件，返回结果 dict。"""
    violations: List[Dict[str, Any]] = []
    scanned_files = 0
    rules_applied = len(RULES)

    if not build_dir.is_dir():
        return {
            "ok": True,
            "violations": [],
            "has_errors": False,
            "scanned_files": 0,
            "rules_applied": rules_applied,
        }

    for py_file in build_dir.rglob("*.py"):
        if "__pycache__" in py_file.parts:
            continue
        scanned_files += 1
        try:
            with open(py_file, "r", encoding="utf-8", errors="replace") as f:
                lines = f.readlines()
        except Exception:  # noqa: BLE001
            continue

        rel_path = str(py_file.relative_to(build_dir))
        for lineno, line in enumerate(lines, start=1):
            for rule in RULES:
                if rule["pattern"].search(line):
                    violations.append({
                        "file": rel_path,
                        "line": lineno,
                        "rule": rule["rule_id"],
                        "severity": rule["severity"],
                        "snippet": line.rstrip("\n")[:120].strip(),
                    })

    return {
        "ok": len(violations) == 0,
        "violations": violations,
        "has_errors": any(v["severity"] == "error" for v in violations),
        "scanned_files": scanned_files,
        "rules_applied": rules_applied,
    }
