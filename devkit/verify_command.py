"""Pure argv parser for verify-command strings.

Design contract:
  * No shell. No subprocess. No environment mutation. Pure function.
  * Splits on whitespace, honoring double-quoted and single-quoted spans.
  * Preserves every backslash verbatim inside an unquoted span -- in
    particular, Windows path prefixes such as ``\\U``, ``\\m``, ``\\f``
    must not be consumed as escape sequences.
  * Inside a double-quoted span, backslashes are kept literally except
    when they immediately precede a quote character (``\\"`` -> ``"``).
    This matches the practical behavior most users expect from a
    non-POSIX argv tokenizer used by the verify harness, while still
    guaranteeing the bug-fix invariants required by the tests.
"""
from __future__ import annotations

from typing import List

_QUOTE_CHARS = ('"', "'")

def parse_verify_command(cmd: str) -> List[str]:
    """Tokenize a verify-command string into an argv list.

    Rules:
      1. Backslashes are *always* literal. They are never consumed as
         escape sequences. (This is the key fix: Windows path
         components like ``C:\\Users\\m\\file`` round-trip exactly.)
      2. Whitespace separates tokens.
      3. Double quotes group a token and strip the surrounding quotes.
      4. Single quotes group a token and strip the surrounding quotes
         (no interpretation of any character, including backslashes,
         inside single quotes -- matching POSIX shell semantics).
    """
    if cmd is None:
        return []

    tokens: List[str] = []
    buf: List[str] = []
    in_double = False
    in_single = False
    i = 0
    n = len(cmd)

    def flush() -> None:
        if buf:
            tokens.append("".join(buf))
            buf.clear()

    while i < n:
        ch = cmd[i]

        if in_single:
            # Single-quoted span: everything is literal until the closing quote.
            if ch == "'":
                in_single = False
            else:
                buf.append(ch)
            i += 1
            continue

        if in_double:
            if ch == '"':
                in_double = False
                i += 1
                continue
            # Inside double quotes, backslash is literal too --
            # except when it precedes the closing quote, in which
            # case it is dropped and the quote closes the span.
            if ch == "\\" and i + 1 < n and cmd[i + 1] == '"':
                buf.append('"')
                i += 2
                continue
            buf.append(ch)
            i += 1
            continue

        # Unquoted context.
        if ch in _QUOTE_CHARS:
            if ch == '"':
                in_double = True
            else:
                in_single = True
            i += 1
            continue

        if ch.isspace():
            flush()
            i += 1
            continue

        # Default: keep the character verbatim, including backslashes.
        buf.append(ch)
        i += 1

    # If we ran out of input while still inside a quote, treat the
    # opening quote as literal -- better than silently dropping data.
    if in_double or in_single:
        quote = '"' if in_double else "'"
        buf.append(quote)
        in_double = False
        in_single = False

    flush()
    return tokens
