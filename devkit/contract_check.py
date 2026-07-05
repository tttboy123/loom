"""
AST symbol contract check.

在 rdloop verify 阶段之前，对 impl 产物做静态符号合约扫描，
缺符号时 gate = NO-GO，并由调用方把 'MISSING_SYMBOL:<name>' 写入 stage-error.md。
"""
from __future__ import annotations

import ast
from pathlib import Path
from typing import Any, Dict, Iterable, List

GATE_GO = "GO"
GATE_NO_GO = "NO-GO"

# 验收 #1 的默认合约：只盯 export-key-env-names.py 里的 KEY_ENV_NAMES。
DEFAULT_CONTRACTS: Dict[str, List[str]] = {
    "export-key-env-names.py": ["KEY_ENV_NAMES"],
}

def _collect_module_symbols(tree: ast.Module) -> set[str]:
    """仅模块级：FunctionDef / AsyncFunctionDef / ClassDef / Assign / AnnAssign / Import / ImportFrom。"""
    names: set[str] = set()
    for node in tree.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            names.add(node.name)
        elif isinstance(node, ast.Assign):
            for tgt in node.targets:
                if isinstance(tgt, ast.Name):
                    names.add(tgt.id)
                elif isinstance(tgt, (ast.Tuple, ast.List)):
                    for elt in tgt.elts:
                        if isinstance(elt, ast.Name):
                            names.add(elt.id)
        elif isinstance(node, ast.AnnAssign):
            if isinstance(node.target, ast.Name):
                names.add(node.target.id)
        elif isinstance(node, ast.Import):
            for alias in node.names:
                names.add(alias.asname or alias.name.split(".")[0])
        elif isinstance(node, ast.ImportFrom):
            for alias in node.names:
                # 只取 asname 或顶级包名（与上游风险说明一致：a.b.c -> a）
                if alias.asname:
                    names.add(alias.asname)
                else:
                    base = (node.module or "").split("." if node.module else "")[0]
                    # 当 from . import x 时 module 为 None/''，回退到名字本身
                    names.add(base or alias.name)
    return names

def check_symbols(py_path: str, expected_symbols: Iterable[str]) -> Dict[str, List[str]]:
    """
    扫描单个 .py 文件，返回：
      {"missing": [...], "present": [...]}
    规则：
      - 文件不存在 -> 全部 missing
      - 语法错误   -> 全部 missing
      - 只看模块级符号
    """
    expected = list(expected_symbols)
    p = Path(py_path)
    if not p.is_file():
        return {"missing": list(expected), "present": []}

    try:
        src = p.read_text(encoding="utf-8")
        tree = ast.parse(src, filename=str(p))
    except (OSError, SyntaxError):
        return {"missing": list(expected), "present": []}

    present_set = _collect_module_symbols(tree)
    present: List[str] = []
    missing: List[str] = []
    for name in expected:
        if name in present_set:
            present.append(name)
        else:
            missing.append(name)
    return {"missing": missing, "present": present}

def verify_impl_contracts(
    impl_dir: str,
    contract_map: Dict[str, List[str]],
) -> Dict[str, Any]:
    """
    批量校验 impl_dir 下所有 contract_map 命中的实现文件。
    返回：
      {
        "all_passed": bool,
        "missing_symbols": ["<rel_path>:<symbol>", ...],
        "errors": ["<rel_path>: <reason>", ...],
      }
    """
    base = Path(impl_dir)
    missing_symbols: List[str] = []
    errors: List[str] = []

    if not base.is_dir():
        return {
            "all_passed": False,
            "missing_symbols": [f"__impl_dir__:{s}" for s in sum(contract_map.values(), [])],
            "errors": [f"impl_dir not found: {impl_dir}"],
        }

    for rel_path, expected in contract_map.items():
        full = base / rel_path
        rel_key = rel_path.replace("\\", "/")
        if not full.is_file():
            for s in expected:
                missing_symbols.append(f"{rel_key}:{s}")
            errors.append(f"{rel_key}: file not found")
            continue
        try:
            tree = ast.parse(full.read_text(encoding="utf-8"), filename=str(full))
        except (OSError, SyntaxError) as e:
            for s in expected:
                missing_symbols.append(f"{rel_key}:{s}")
            errors.append(f"{rel_key}: parse error: {e}")
            continue
        present_set = _collect_module_symbols(tree)
        for s in expected:
            if s not in present_set:
                missing_symbols.append(f"{rel_key}:{s}")

    return {
        "all_passed": len(missing_symbols) == 0 and len(errors) == 0,
        "missing_symbols": missing_symbols,
        "errors": errors,
    }

def gate_decision(verify_result: Dict[str, Any]) -> str:
    """verify_impl_contracts 结果 -> 闸门决定。"""
    return GATE_GO if verify_result.get("all_passed") else GATE_NO_GO

def format_stage_error_lines(missing_symbols: Iterable[str]) -> List[str]:
    """把 '<rel>:<symbol>' 渲染成 stage-error.md 需要的 'MISSING_SYMBOL:<symbol>' 行。"""
    out: List[str] = []
    for item in missing_symbols:
        # item 形如 "export-key-env-names.py:KEY_ENV_NAMES"
        _, _, sym = item.rpartition(":")
        out.append(f"MISSING_SYMBOL:{sym}")
    return out
