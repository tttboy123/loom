# devkit/run_health_monitor.py
"""Monitor run health metrics - pure standard library."""

def check_run_health(run_data: dict) -> dict:
    """Check health of a single run.

    Args:
        run_data: {tokens, duration_s, gate, iterations}

    Returns:
        {healthy: bool, warnings: list[str], score: float}
    """
    warnings = []

    if run_data.get('tokens', 0) > 50000:
        warnings.append('high token usage')

    if run_data.get('duration_s', 0) > 300:
        warnings.append('slow run')

    if run_data.get('iterations', 0) > 3:
        warnings.append('many iterations')

    if run_data.get('gate') not in ('GO', '建议 GO'):
        warnings.append('gate failed')

    score = max(0.0, 1.0 - 0.1 * len(warnings))
    healthy = score >= 0.9
    return {'healthy': healthy, 'warnings': warnings, 'score': score}

def aggregate_health(runs: list[dict]) -> dict:
    """Aggregate health stats across multiple runs.

    Args:
        runs: list of run_data dicts

    Returns:
        {total, healthy_count, avg_score, issues}
    """
    total = len(runs)
    if total == 0:
        return {'total': 0, 'healthy_count': 0, 'avg_score': 0.0, 'issues': []}

    healthy_count = 0
    total_score = 0.0
    all_warnings = set()

    for run in runs:
        result = check_run_health(run)
        if result['healthy']:
            healthy_count += 1
        total_score += result['score']
        all_warnings.update(result['warnings'])

    avg_score = total_score / total if total > 0 else 0.0
    issues = sorted(all_warnings)

    return {'total': total, 'healthy_count': healthy_count,
            'avg_score': avg_score, 'issues': issues}
