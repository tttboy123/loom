# stage_config.py
# 纯标准库，管理 stage 配置。
# 注意：bool 是 int 的子类，"正整数"校验需显式排除 bool。

CARRIER = 'minimax'
DEFAULT_TIMEOUT = 300
DEFAULT_MAX_TOKENS = 8000

def default_config(stage: str) -> dict:
    """返回 stage 的默认配置 dict。"""
    return {
        'stage': stage,
        'carrier': CARRIER,
        'timeout': DEFAULT_TIMEOUT,
        'max_tokens': DEFAULT_MAX_TOKENS,
        'enabled': True,
    }

def merge_config(base: dict, overrides: dict) -> dict:
    """将 overrides 浅拷贝叠加到 base 的浅拷贝上，返回新 dict（不修改入参）。"""
    out = dict(base)            # 浅拷贝 base
    out.update(dict(overrides)) # 浅拷贝 overrides 并叠加
    return out

def _is_positive_int(value) -> bool:
    """True 当且仅当 value 是 int 且 > 0。bool 视为非 int（显式排除）。"""
    return isinstance(value, int) and not isinstance(value, bool) and value > 0

def _is_non_empty_str(value) -> bool:
    """True 当且仅当 value 是 str 且非空。"""
    return isinstance(value, str) and value != ''

def validate_config(config: dict) -> dict:
    """校验 config，返回 {valid: bool, errors: list[str]}。"""
    errors: list[str] = []

    if 'carrier' not in config or not _is_non_empty_str(config.get('carrier')):
        errors.append('carrier must be a non-empty string')

    if 'timeout' not in config or not _is_positive_int(config.get('timeout')):
        errors.append('timeout must be a positive integer')

    if 'max_tokens' not in config or not _is_positive_int(config.get('max_tokens')):
        errors.append('max_tokens must be a positive integer')

    return {'valid': len(errors) == 0, 'errors': errors}
