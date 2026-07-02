# devkit/run_profiler.py
# 纯标准库实现：分析 run 性能数据。
# 三个公共 API:
#   - profile(run_data) -> dict
#   - compare_profiles(a, b) -> dict
#   - profile_summary(profile) -> str

def profile(run_data: dict) -> dict:
    """
    分析一次 run 的阶段性能数据。

    参数:
        run_data: {
            'stages': list[{'name': str, 'tokens': int, 'duration_s': float}],
            'total_tokens': int
        }

    返回:
        {
            'slowest_stage':     str | None,
            'fastest_stage':     str | None,
            'token_distribution': dict[str, float],   # 各 stage 占比 0-100
            'total_duration':    float                # 所有阶段 duration_s 之和
        }
    """
    stages = run_data.get("stages", []) or []
    total_tokens = run_data.get("total_tokens", 0) or 0

    # 1) 找最慢 / 最快 stage（按 duration_s）
    if stages:
        slowest_stage = max(stages, key=lambda s: s.get("duration_s", 0.0))["name"]
        fastest_stage = min(stages, key=lambda s: s.get("duration_s", 0.0))["name"]
    else:
        slowest_stage = None
        fastest_stage = None

    # 2) 构造 token 分布（百分比 0-100）
    token_distribution: dict = {}
    for stage in stages:
        name = stage.get("name", "")
        tokens = stage.get("tokens", 0) or 0
        if total_tokens > 0:
            pct = (tokens / total_tokens) * 100.0
        else:
            pct = 0.0
        token_distribution[name] = pct

    # 3) 总时长
    total_duration = sum(float(s.get("duration_s", 0.0) or 0.0) for s in stages)

    return {
        "slowest_stage": slowest_stage,
        "fastest_stage": fastest_stage,
        "token_distribution": token_distribution,
        "total_duration": total_duration,
    }

def compare_profiles(a: dict, b: dict) -> dict:
    """
    对比两次 profile 的结果。

    参数 a, b 至少包含键: 'total_duration' (float), 'total_tokens' (int)
    返回: {
        'faster':         'a' | 'b',     # duration 更小者胜；相等时 'b'
        'token_delta':    int,           # a.total_tokens - b.total_tokens
        'duration_delta': float,         # a.total_duration - b.total_duration
    }
    """
    a_duration = a.get("total_duration", 0.0) or 0.0
    b_duration = b.get("total_duration", 0.0) or 0.0

    faster = "a" if a_duration < b_duration else "b"

    a_tokens = a.get("total_tokens", 0) or 0
    b_tokens = b.get("total_tokens", 0) or 0

    return {
        "faster": faster,
        "token_delta": int(a_tokens) - int(b_tokens),
        "duration_delta": float(a_duration) - float(b_duration),
    }

def profile_summary(profile: dict) -> str:
    """
    把 profile 结果格式化成一行摘要字符串:
        slowest=<name|None> fastest=<name|None> duration=<total_duration:.1f>s
    """
    slowest = profile.get("slowest_stage")
    fastest = profile.get("fastest_stage")
    total_duration = float(profile.get("total_duration", 0.0) or 0.0)
    return (
        f"slowest={slowest} "
        f"fastest={fastest} "
        f"duration={total_duration:.1f}s"
    )
