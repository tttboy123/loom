"""Validate stage output format. Stdlib only."""
from __future__ import annotations

import re

_FENCE_PATTERN = re.compile(r"```[^\n]*\n(.*?)```", re.DOTALL)


def check_nonempty(output: str) -> bool:
    return bool(output.strip())


def check_has_code(output: str) -> bool:
    return "```" in output


def extract_code_blocks(output: str) -> list[str]:
    return _FENCE_PATTERN.findall(output)


def validate_output(output: str, rules: list[str]) -> dict:
    """Return {valid, violations} against named rules."""
    violations: list[str] = []
    for rule in rules:
        if rule == "nonempty" and not check_nonempty(output):
            violations.append("nonempty")
        elif rule == "has_code" and not check_has_code(output):
            violations.append("has_code")
        elif rule == "has_python" and "```python" not in output:
            violations.append("has_python")
    return {"valid": len(violations) == 0, "violations": violations}
