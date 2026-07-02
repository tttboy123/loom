"""从 run-log.md 聚合 carrier 性能指标（纯标准库）。"""
import re
from collections import Counter

_GATE_RE = re.compile(r'Gate:\s*(GO|NO-GO)')
_TOKENS_RE = re.compile(r'用量合计[：:]\s*(\d+)\s*tok')
_COST_RE = re.compile(r'费用[：:]\s*([\d.]+)\s*USD')
_CARRIER_RE = re.compile(r'实际\s*=\s*(\S+)')

_EMPTY_RESULT = {
    'gate': '',
    'tokens': 0,
    'cost_usd': 0.0,
    'carriers_used': [],
}

_EMPTY_AGG = {
    'total_runs': 0,
    'go_runs': 0,
    'no_go_runs': 0,
    'total_tokens': 0,
    'carrier_counts': {},
}

def parse_run_log(log_content: str) -> dict:
    """从 run-log.md 文本提取 {gate, tokens, cost_usd, carriers_used}。

    字段缺失一律走默认值，不抛异常。
    """
    if not log_content:
        return dict(_EMPTY_RESULT)

    gate_m = _GATE_RE.search(log_content)
    tok_m = _TOKENS_RE.search(log_content)
    cost_m = _COST_RE.search(log_content)
    carriers = _CARRIER_RE.findall(log_content)

    return {
        'gate': gate_m.group(1) if gate_m else '',
        'tokens': int(tok_m.group(1)) if tok_m else 0,
        'cost_usd': float(cost_m.group(1)) if cost_m else 0.0,
        'carriers_used': carriers,
    }

def aggregate(run_logs: list) -> dict:
    """批量聚合，返回 total_runs / go_runs / no_go_runs / total_tokens / carrier_counts。

    空列表走全 0 默认值。
    """
    if not run_logs:
        return dict(_EMPTY_AGG)

    parsed = [parse_run_log(log) for log in run_logs]
    go_runs = sum(1 for p in parsed if p['gate'] == 'GO')
    no_go_runs = sum(1 for p in parsed if p['gate'] == 'NO-GO')
    total_tokens = sum(p['tokens'] for p in parsed)
    carrier_counts = dict(
        Counter(c for p in parsed for c in p['carriers_used'])
    )

    return {
        'total_runs': len(parsed),
        'go_runs': go_runs,
        'no_go_runs': no_go_runs,
        'total_tokens': total_tokens,
        'carrier_counts': carrier_counts,
    }
