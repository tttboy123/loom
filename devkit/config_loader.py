# devkit/config_loader.py
"""Load and merge devkit configuration dicts. Standard library only."""
from __future__ import annotations

import json


def load_json(path: str, default: dict = None) -> dict:
    if default is None:
        default = {}
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return default


def merge_configs(*configs: dict) -> dict:
    result: dict = {}
    for c in configs:
        result.update(c)
    return result


def get(config: dict, key: str, default=None):
    parts = key.split(".")
    cur = config
    for p in parts:
        if not isinstance(cur, dict) or p not in cur:
            return default
        cur = cur[p]
    return cur


def set_nested(config: dict, key: str, value) -> dict:
    parts = key.split(".")
    cur = config
    for p in parts[:-1]:
        if p not in cur or not isinstance(cur[p], dict):
            cur[p] = {}
        cur = cur[p]
    cur[parts[-1]] = value
    return config
