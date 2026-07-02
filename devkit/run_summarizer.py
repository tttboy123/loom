"""
devkit/run_summarizer.py

纯标准库：根据 run 字典生成摘要。TDD 实现的最小集。
不依赖任何第三方包。
"""

def _classify_tokens(tokens: int) -> str:
    """<1000 -> 'light'；<5000 -> 'medium'；否则 'heavy'。"""
    if tokens < 1000:
        return 'light'
    if tokens < 5000:
        return 'medium'
    return 'heavy'

def _verdict_from_gate(gate: str) -> str:
    """gate == 'GO' -> 'pass'；否则 'fail'。"""
    return 'pass' if gate == 'GO' else 'fail'

def summarize(run: dict) -> dict:
    """
    输入 run: {id, gate, tokens, duration_s, stages: list}
    返回: {id, gate, token_class, stage_count, verdict}
    """
    return {
        'id': run['id'],
        'gate': run['gate'],
        'token_class': _classify_tokens(run['tokens']),
        'stage_count': len(run['stages']),
        'verdict': _verdict_from_gate(run['gate']),
    }

def batch_summarize(runs: list) -> list:
    """对每个 run 调用 summarize，返回摘要列表（保序）。"""
    return [summarize(r) for r in runs]

def summary_stats(summaries: list) -> dict:
    """
    返回 {total, pass_count, fail_count, pass_rate}。
    total == 0 时 pass_rate = 0.0（避免 ZeroDivisionError）。
    """
    total = len(summaries)
    pass_count = sum(1 for s in summaries if s['verdict'] == 'pass')
    fail_count = total - pass_count
    pass_rate = (pass_count / total) if total > 0 else 0.0
    return {
        'total': total,
        'pass_count': pass_count,
        'fail_count': fail_count,
        'pass_rate': pass_rate,
    }
