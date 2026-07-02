# schema_validator.py —— 纯标准库，JSON Schema 子集验证器
from __future__ import annotations
from typing import Any

# 支持的 type → Python 类型映射
_TYPE_MAP = {
    "str":   str,
    "int":   int,
    "float": float,
    "bool":  bool,
    "list":  list,
    "dict":  dict,
    "null":  type(None),
}

def _err(errors: list[str], msg: str) -> None:
    errors.append(msg)

def validate(data: Any, schema: dict) -> dict:
    """按 schema 校验 data，返回 {valid: bool, errors: list[str]}。"""
    errors: list[str] = []

    # --- type ---
    tname = schema.get("type")
    if tname is not None:
        py_t = _TYPE_MAP.get(tname)
        if py_t is None:
            _err(errors, f"unsupported type: {tname!r}")
        else:
            # 注意：bool 是 int 的子类，单独判断时不能让 True/False 通过 int
            if py_t is int:
                if not isinstance(data, int) or isinstance(data, bool):
                    _err(errors, f"expected int, got {type(data).__name__}")
            else:
                if not isinstance(data, py_t):
                    _err(errors, f"expected {tname}, got {type(data).__name__}")

    # --- type 已失败时，剩余约束意义不大，跳过以避免噪声 ---
    if errors:
        return {"valid": False, "errors": errors}

    # --- required (dict) ---
    req = schema.get("required")
    if isinstance(req, list):
        if not isinstance(data, dict):
            _err(errors, "'required' only applies to dict")
        else:
            for key in req:
                if not isinstance(key, str):
                    _err(errors, "required keys must be strings")
                    break
                if key not in data:
                    _err(errors, f"missing required key: {key!r}")

    # --- minLength / maxLength (str) ---
    if "minLength" in schema or "maxLength" in schema:
        if not isinstance(data, str):
            _err(errors, "minLength/maxLength only applies to str")
        else:
            mn = schema.get("minLength")
            mx = schema.get("maxLength")
            if mn is not None and len(data) < mn:
                _err(errors, f"length {len(data)} < minLength {mn}")
            if mx is not None and len(data) > mx:
                _err(errors, f"length {len(data)} > maxLength {mx}")

    # --- minimum / maximum (数值) ---
    if "minimum" in schema or "maximum" in schema:
        if not isinstance(data, (int, float)) or isinstance(data, bool):
            _err(errors, "minimum/maximum only applies to numbers")
        else:
            mn = schema.get("minimum")
            mx = schema.get("maximum")
            if mn is not None and data < mn:
                _err(errors, f"value {data} < minimum {mn}")
            if mx is not None and data > mx:
                _err(errors, f"value {data} > maximum {mx}")

    return {"valid": len(errors) == 0, "errors": errors}

def is_valid(data: Any, schema: dict) -> bool:
    """validate() 的 valid 字段封装。"""
    return validate(data, schema)["valid"]
