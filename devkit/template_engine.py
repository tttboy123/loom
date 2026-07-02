"""Simple template rendering engine — pure standard library."""

import re

# Matches {{ key }} with optional surrounding whitespace inside the braces.
# Does NOT match doubled braces like {{{key}}} as a substitution; only {{...}}.
_PATTERN = re.compile(r"\{\{\s*([^{}]+?)\s*\}\}")

def render(template: str, context: dict) -> str:
    """Replace {{key}} with context[key]; leave {{key}} unchanged if missing.

    Whitespace inside braces is tolerated ({{ key }}, {{key }} both work).
    Keys are looked up via str(key) — no attribute/index access, no expressions.
    """
    if not template:
        return template

    def _replace(match: re.Match) -> str:
        key = match.group(1).strip()
        if key in context:
            return str(context[key])
        return match.group(0)  # leave original {{key}} untouched

    return _PATTERN.sub(_replace, template)

def render_list(template: str, items: list[dict]) -> list[str]:
    """Render the template once per item dict; returns list of strings."""
    return [render(template, item) for item in items]

def extract_vars(template: str) -> list[str]:
    """Return all {{key}} keys in the template, deduplicated, preserving order."""
    seen: set[str] = set()
    result: list[str] = []
    for match in _PATTERN.finditer(template):
        key = match.group(1).strip()
        if key not in seen:
            seen.add(key)
            result.append(key)
    return result
