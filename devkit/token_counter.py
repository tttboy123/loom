"""Token usage estimation and budget helpers.

Pure standard library. Intentionally simple — this is an *estimate*,
not a tokenizer. See docstrings per function for the exact contract.
"""

def count_tokens(text: str) -> int:
    """Rough token estimate: word count × 1.3, truncated to int, min 0.

    Examples
    --------
    >>> count_tokens("")
    0
    >>> count_tokens("hello world")
    2
    """
    if not text:
        return 0
    words = len(text.split())
    return int(words * 1.3)

def count_stage_tokens(stages: list[dict]) -> dict:
    """Aggregate token counts across pipeline stages.

    Parameters
    ----------
    stages : list[dict]
        Each item must have ``name`` (str) and ``tokens`` (int).

    Returns
    -------
    dict
        ``{"total": int, "by_stage": {name: tokens}, "max_stage": str|None}``

    On empty input, ``total == 0`` and ``max_stage is None``.
    On ties for the maximum, the first stage in the list wins.
    """
    by_stage = {s["name"]: s["tokens"] for s in stages}
    total = sum(by_stage.values())

    max_stage = None
    if stages:
        # max() with a stable key: first encountered wins on ties
        # because we compare (value, -index) — no, simpler: iterate.
        max_stage = stages[0]["name"]
        max_tokens = stages[0]["tokens"]
        for s in stages[1:]:
            if s["tokens"] > max_tokens:
                max_tokens = s["tokens"]
                max_stage = s["name"]

    return {"total": total, "by_stage": by_stage, "max_stage": max_stage}

def token_budget_status(used: int, budget: int) -> dict:
    """Compute remaining budget and whether usage is within limits.

    Parameters
    ----------
    used : int
    budget : int

    Returns
    -------
    dict
        ``{"used", "budget", "remaining", "pct", "ok"}``

    Notes
    -----
    - ``remaining = budget - used`` (may be negative when over).
    - ``pct = used / budget * 100`` as a float; ``0.0`` when ``budget == 0``
      (avoids ZeroDivisionError; caller can still inspect ``used``/``ok``).
    - ``ok = used <= budget``.
    """
    if budget == 0:
        pct = 0.0
    else:
        pct = used / budget * 100

    return {
        "used": used,
        "budget": budget,
        "remaining": budget - used,
        "pct": pct,
        "ok": used <= budget,
    }
