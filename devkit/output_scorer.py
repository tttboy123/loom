# output_scorer.py - devkit/output_scorer.py

import re

def score(output: str, criteria: list[str]) -> dict:
    """
    Score an output string against a list of criteria.
    Returns {score, passed, failed}.
    """
    passed = []
    failed = []

    for criterion in criteria:
        if criterion == 'nonempty':
            if output.strip() != '':
                passed.append('nonempty')
            else:
                failed.append('nonempty')
        elif criterion == 'has_code':
            if re.search(r'```', output):
                passed.append('has_code')
            else:
                failed.append('has_code')
        elif criterion == 'has_python':
            if re.search(r'```python', output):
                passed.append('has_python')
            else:
                failed.append('has_python')
        elif criterion == 'concise':
            if len(output) <= 2000:
                passed.append('concise')
            else:
                failed.append('concise')
        else:
            # Unknown criteria are treated as failing
            failed.append(criterion)

    total = len(criteria)
    if total == 0:
        score_val = 1.0
    else:
        score_val = len(passed) / total

    return {
        'score': score_val,
        'passed': passed,
        'failed': failed,
    }

def batch_score(outputs: list[str], criteria: list[str]) -> dict:
    """
    Score a list of outputs, returning aggregate stats.
    """
    scores = []
    for output in outputs:
        scores.append(score(output, criteria)['score'])

    if not scores:
        return {'scores': [], 'avg': 0.0, 'min': 0.0, 'max': 0.0}

    avg = sum(scores) / len(scores)
    min_val = min(scores)
    max_val = max(scores)

    return {
        'scores': scores,
        'avg': avg,
        'min': min_val,
        'max': max_val,
    }

def top_outputs(outputs: list[str], criteria: list[str], n: int) -> list[str]:
    """
    Return the top n outputs by score, sorted descending.
    """
    if n <= 0 or not outputs:
        return []

    scored = [(score(out, criteria)['score'], out) for out in outputs]
    scored.sort(key=lambda x: -x[0])

    return [out for _, out in scored[:n]]
