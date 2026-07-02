# safety_preset.py
"""Declarative safety presets: minimal / standard / strict.

A scan result (from ``safety.scan_build()``) is expected to be a dict that
contains a ``"violations"`` key — a list of dicts each with at least a
``"level"`` key (``error`` / ``warn`` / ``info``), and typically ``"rule"``
and ``"message"``.
"""

_PRESETS = {
    "minimal": {
        "block_levels": frozenset({"error"}),
        "warn_levels": frozenset(),
        "description": "Only error-level violations block.",
    },
    "standard": {
        "block_levels": frozenset({"error"}),
        "warn_levels": frozenset({"warn"}),
        "description": "Error-level violations block; warn-level produce warnings.",
    },
    "strict": {
        # None == "no threshold": any violation blocks.
        "block_levels": None,
        "warn_levels": frozenset(),
        "description": "Any violation blocks.",
    },
}

_VALID_LEVELS = frozenset(_PRESETS.keys())

def get_preset(level: str) -> dict:
    """Return a copy of the rule configuration for the given safety level.

    Raises ``ValueError`` if *level* is not one of minimal / standard / strict.
    """
    if level not in _PRESETS:
        raise ValueError(
            f"Unknown safety preset level: {level!r}. "
            f"Expected one of: {sorted(_VALID_LEVELS)}"
        )
    src = _PRESETS[level]
    return {
        "block_levels": (
            set(src["block_levels"]) if src["block_levels"] is not None else None
        ),
        "warn_levels": set(src["warn_levels"]),
        "description": src["description"],
    }

def apply_preset(level: str, scan_result: dict) -> dict:
    """Apply a safety preset to a scan-build result.

    Returns ``{"block": bool, "warnings": list[str], "reason": str}``.
    """
    if not isinstance(scan_result, dict):
        raise TypeError(
            f"scan_result must be a dict, got {type(scan_result).__name__}"
        )

    preset = get_preset(level)
    block_levels = preset["block_levels"]      # set[str] | None
    warn_levels = preset["warn_levels"]        # set[str]

    violations = scan_result.get("violations") or []

    block = False
    warnings: list[str] = []
    block_details: list[str] = []

    for v in violations:
        if not isinstance(v, dict):
            # Defensive: skip malformed entries rather than crash.
            continue

        v_level = v.get("level", "info")
        rule = v.get("rule", "unknown")
        message = v.get("message", "")
        detail = f"[{rule}] {message}".strip()
        if not detail:
            detail = f"[{rule}] (no message)"

        # strict: any violation blocks, regardless of level.
        if block_levels is None:
            block = True
            block_details.append(detail)
            continue

        if v_level in block_levels:
            block = True
            block_details.append(detail)
        elif v_level in warn_levels:
            warnings.append(detail)
        # else: info / unknown levels are silently ignored at this preset.

    if block:
        reason = f"[{level}] blocked — {'; '.join(block_details)}"
    elif warnings:
        reason = f"[{level}] passed with warnings — {'; '.join(warnings)}"
    else:
        reason = f"[{level}] passed — no violations"

    return {"block": block, "warnings": warnings, "reason": reason}
