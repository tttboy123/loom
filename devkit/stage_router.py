"""devkit/stage_router.py — stage selection & routing utilities.

Pure standard library; no external dependencies.
"""

# -------------------- 常量 --------------------

# 默认的阶段列表字符串
DEFAULT_STAGES = "implement,verify"
# 默认的 carrier 模型标识
DEFAULT_CARRIER = "glm"

# -------------------- 公开接口 --------------------

def parse_stages(stages_str: str) -> list[str]:
    """解析逗号分隔的阶段字符串，去除空白，过滤空项。

    >>> parse_stages('implement,verify')
    ['implement', 'verify']

    >>> parse_stages('')
    []

    >>> parse_stages('implement')
    ['implement']
    """
    if not stages_str or not isinstance(stages_str, str):
        return []
    parts = stages_str.split(",")
    result = [part.strip() for part in parts if part.strip()]
    return result

def route(task: dict) -> list[str]:
    """从 task 中提取 / 计算实际需要执行的阶段列表。

    >>> route({'stages': 'implement,verify'})
    ['implement', 'verify']

    >>> route({})
    ['implement', 'verify']
    """
    raw = task.get("stages", DEFAULT_STAGES)
    return parse_stages(raw)

def should_skip(task: dict, stage: str) -> bool:
    """判断 task 是否要求跳过指定 stage。

    >>> should_skip({'skip_stages': ['verify']}, 'verify')
    True

    >>> should_skip({'skip_stages': ['verify']}, 'implement')
    False

    >>> should_skip({}, 'verify')
    False
    """
    skip_list = task.get("skip_stages")
    if skip_list is None or not isinstance(skip_list, list):
        return False
    return stage in skip_list

def stage_carrier(task: dict, stage: str) -> str:
    """获取 task 中指定 stage 对应的 carrier 模型标识。

    >>> stage_carrier({'carrier': {'implement': 'deepseek'}}, 'implement')
    'deepseek'

    >>> stage_carrier({}, 'implement')
    'glm'
    """
    carriers = task.get("carrier", {})
    if not isinstance(carriers, dict):
        return DEFAULT_CARRIER
    return carriers.get(stage, DEFAULT_CARRIER)
