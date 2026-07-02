"""Pure-stdlib context window manager.

All mutating-style operations return a NEW dict; inputs are never modified.
"""
from __future__ import annotations

from typing import Any

def create(max_tokens: int) -> dict:
    """Create a fresh, empty context window."""
    return {"max_tokens": max_tokens, "used": 0, "messages": []}

def add_message(ctx: dict, role: str, content: str, tokens: int) -> dict:
    """Append a message and return a new ctx with updated `used`."""
    new_msg = {"role": role, "content": content, "tokens": tokens}
    return {
        "max_tokens": ctx["max_tokens"],
        "used": ctx["used"] + tokens,
        "messages": ctx["messages"] + [new_msg],
    }

def fits(ctx: dict, tokens: int) -> bool:
    """True iff adding `tokens` keeps usage within the budget."""
    return ctx["used"] + tokens <= ctx["max_tokens"]

def trim(ctx: dict, target_tokens: int) -> dict:
    """Drop oldest messages until `used` <= target_tokens; return new ctx."""
    messages = list(ctx["messages"])
    used = ctx["used"]
    # Drop from the front (oldest first) while over budget.
    while used > target_tokens and messages:
        dropped = messages.pop(0)
        used -= dropped["tokens"]
    return {
        "max_tokens": ctx["max_tokens"],
        "used": used,
        "messages": messages,
    }
