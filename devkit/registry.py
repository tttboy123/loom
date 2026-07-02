# registry.py
"""Stage Registry：结构化阶段注册表，从 loom.roles.toml / loom-roles.yaml 读取并校验。"""
from __future__ import annotations

import pathlib
import sys
from typing import Optional

_DEFAULTS = {"trust_level": 1, "max_cost_per_run": 0.05, "allowed_executors": ["chat"]}
_ROOT = pathlib.Path(__file__).parent.parent


def _find_config() -> Optional[pathlib.Path]:
    """查找 loom.roles.toml 或 loom-roles.yaml，优先 toml。"""
    for name in ("loom.roles.toml", "loom-roles.yaml", "loom-roles.yml"):
        p = _ROOT / name
        if p.exists():
            return p
        p2 = _ROOT / "devkit" / name
        if p2.exists():
            return p2
    return None


def _load_toml(path: pathlib.Path) -> list[dict]:
    text = path.read_text(encoding="utf-8")
    if sys.version_info >= (3, 11):
        import tomllib
        data = tomllib.loads(text)
    else:
        try:
            import tomli
            data = tomli.loads(text)
        except ImportError:
            return []
    stages = data.get("stage") or data.get("stages") or []
    if isinstance(stages, dict):
        stages = [dict(v, key=k) for k, v in stages.items()]
    return stages if isinstance(stages, list) else []


def _load_yaml(path: pathlib.Path) -> list[dict]:
    try:
        import yaml  # type: ignore[import]
        data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        stages = data.get("stages") or data.get("stage") or []
        return stages if isinstance(stages, list) else []
    except ImportError:
        return []


def _with_defaults(entry: dict) -> dict:
    out = dict(entry)
    for k, v in _DEFAULTS.items():
        if k not in out:
            out[k] = v
    return out


def load(path: Optional[str] = None) -> list[dict]:
    """从 loom.roles.toml 或 loom-roles.yaml 加载 stage 列表，补齐默认值。"""
    cfg = pathlib.Path(path) if path else _find_config()
    if not cfg or not cfg.exists():
        return []
    suffix = cfg.suffix.lower()
    if suffix == ".toml":
        stages = _load_toml(cfg)
    elif suffix in (".yaml", ".yml"):
        stages = _load_yaml(cfg)
    else:
        return []
    return [_with_defaults(s) for s in stages]


def get(key: str, path: Optional[str] = None) -> Optional[dict]:
    """按 stage key 返回单条注册信息，不存在返回 None。"""
    for entry in load(path):
        if entry.get("key") == key:
            return entry
    return None


def validate(entry: dict) -> dict:
    """校验一条 stage 配置，返回 {ok: bool, errors: list[str]}。"""
    errors: list[str] = []
    if not entry.get("key") or not isinstance(entry.get("key"), str):
        errors.append("key 必须是非空字符串")
    tl = entry.get("trust_level")
    if tl not in (1, 2, 3):
        errors.append("trust_level 必须是 1/2/3")
    mc = entry.get("max_cost_per_run")
    if not isinstance(mc, (int, float)) or mc <= 0:
        errors.append("max_cost_per_run 必须 > 0")
    ae = entry.get("allowed_executors")
    if not isinstance(ae, list) or len(ae) == 0:
        errors.append("allowed_executors 必须是非空列表")
    return {"ok": len(errors) == 0, "errors": errors}
