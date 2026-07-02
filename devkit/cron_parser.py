# devkit/cron_parser.py
# Pure-stdlib 5-field cron expression parser.
# Fields: minute hour day month weekday

from __future__ import annotations

from typing import Dict

# ---------- Field bounds ----------
_FIELD_RANGES: Dict[str, tuple] = {
    "minute":   (0, 59),
    "hour":     (0, 23),
    "day":      (1, 31),
    "month":   (1, 12),
    "weekday": (0, 6),   # 0 = Sunday
}

_FIELD_NAMES = ("minute", "hour", "day", "month", "weekday")

# ---------- Public API ----------

def parse(expr: str) -> Dict[str, str]:
    """Parse a 5-field cron expression.

    Returns a dict with keys: minute, hour, day, month, weekday.
    Each value is the raw field string (unchanged), e.g. "*", "0", "*/5".
    """
    _assert_five_fields(expr)
    fields = expr.strip().split()
    return dict(zip(_FIELD_NAMES, fields))

def is_valid(expr: str) -> bool:
    """True iff `expr` is a syntactically valid 5-field cron expression.

    Checks structure (5 fields) and per-field syntactic shape
    (numbers, ranges, steps, lists, and ranges within bounds).
    """
    if not isinstance(expr, str):
        return False
    parts = expr.strip().split()
    if len(parts) != 5:
        return False
    return all(_valid_field(p, r) for p, r in zip(parts, _FIELD_RANGES.values()))

def describe(expr: str) -> str:
    """Return a human-readable description of the cron expression."""
    parsed = parse(expr)  # may raise ValueError; intentional per spec
    f = (parsed["minute"], parsed["hour"], parsed["day"],
         parsed["month"], parsed["weekday"])
    return _describe(f)

# ---------- Internals ----------

def _assert_five_fields(expr: str) -> None:
    if not isinstance(expr, str):
        raise ValueError(f"cron expression must be a string, got {type(expr).__name__}")
    parts = expr.strip().split()
    if len(parts) != 5:
        raise ValueError(
            f"cron expression must have 5 fields (minute hour day month weekday), "
            f"got {len(parts)}: {expr!r}"
        )

def _valid_field(field: str, bounds: tuple) -> bool:
    """Validate one field against inclusive (lo, hi) bounds."""
    lo, hi = bounds
    # comma-separated list
    for piece in field.split(","):
        if not _valid_piece(piece, lo, hi):
            return False
    return True

def _valid_piece(piece: str, lo: int, hi: int) -> bool:
    # step piece: "a/b" or "*/b"
    if "/" in piece:
        base, _, step_str = piece.partition("/")
        if not step_str.isdigit() or int(step_str) <= 0:
            return False
        # base must itself be valid (including as a range or "*")
        if base == "*":
            return True
        return _valid_range_or_number(base, lo, hi)
    # plain "*"
    if piece == "*":
        return True
    # range "a-b"
    if "-" in piece:
        return _valid_range_or_number(piece, lo, hi)
    # single number
    return piece.isdigit() and lo <= int(piece) <= hi

def _valid_range_or_number(token: str, lo: int, hi: int) -> bool:
    # token is either a single integer "n" or a range "a-b"
    if token.isdigit():
        return lo <= int(token) <= hi
    if "-" in token:
        a_str, _, b_str = token.partition("-")
        if not (a_str.lstrip("-").isdigit() and b_str.lstrip("-").isdigit()):
            return False
        a, b = int(a_str), int(b_str)
        return lo <= a <= hi and lo <= b <= hi and a <= b
    return False

# ---------- Human-readable description ----------

_MONTH_NAMES = ["", "January", "February", "March", "April", "May", "June",
                "July", "August", "September", "October", "November", "December"]
_WEEKDAY_NAMES = ["Sunday", "Monday", "Tuesday", "Wednesday",
                  "Thursday", "Friday", "Saturday"]

def _describe(fields) -> str:
    minute, hour, day, month, weekday = fields

    # common shortcut: every minute
    if (minute, hour, day, month, weekday) == ("*", "*", "*", "*", "*"):
        return "every minute"

    parts = []

    # minute / hour phrase
    parts.append(_time_phrase(minute, hour))

    # month
    if month != "*":
        parts.append(f"in {_month_phrase(month)}")

    # day of month
    if day != "*":
        parts.append(f"on day {day}")

    # weekday
    if weekday != "*":
        parts.append(f"on {_weekday_phrase(weekday)}")

    return ", ".join(parts)

def _time_phrase(minute: str, hour: str) -> str:
    if minute == "*" and hour == "*":
        return "every minute"
    if minute == "*":
        return f"at every minute of hour {hour}"
    if hour == "*":
        return f"at minute {minute} of every hour"
    return f"at {hour}:{minute.zfill(2)}"

def _month_phrase(month: str) -> str:
    names = []
    for piece in month.split(","):
        if piece.isdigit():
            idx = int(piece)
            if 1 <= idx <= 12:
                names.append(_MONTH_NAMES[idx])
    return ", ".join(names) if names else month

def _weekday_phrase(weekday: str) -> str:
    names = []
    for piece in weekday.split(","):
        if piece.isdigit():
            idx = int(piece) % 7
            names.append(_WEEKDAY_NAMES[idx])
    return ", ".join(names) if names else weekday
