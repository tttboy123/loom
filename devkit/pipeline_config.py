# devkit/pipeline_config.py
# 纯标准库实现：pipeline 级别配置管理

def default_pipeline_config():
    """返回 pipeline 的默认配置。"""
    return {
        "max_iterations": 3,
        "cascade": "minimax,glm,deepseek",
        "timeout": 600,
        "iterate": True,
        "blind_review": False,
    }

def apply_overrides(config, overrides):
    """浅合并 overrides 到 config 副本，返回新 dict，不修改原始。"""
    if config is None:
        config = {}
    if overrides is None:
        overrides = {}
    result = dict(config)
    for key, value in overrides.items():
        result[key] = value
    return result

def config_diff(base, current):
    """比较 base 与 current，返回 {changed, added, removed}。

    - changed: 键在两者都有，但值不同的项 (key -> current value)
    - added:   current 有但 base 没有的项 (key -> current value)
    - removed: base 有但 current 没有的键列表
    """
    if base is None:
        base = {}
    if current is None:
        current = {}

    base_keys = set(base.keys())
    current_keys = set(current.keys())

    changed = {k: current[k] for k in base_keys & current_keys if base[k] != current[k]}
    added = {k: current[k] for k in current_keys - base_keys}
    removed = [k for k in base_keys - current_keys]

    return {"changed": changed, "added": added, "removed": removed}
