# devkit/html_stripper.py
"""HTML cleaning utilities — pure stdlib, no external deps.

Three public functions:
    strip_tags(html)        -> str
    extract_links(html)     -> list[str]
    extract_text(html, tag) -> list[str]
"""
from __future__ import annotations

import re
from html.parser import HTMLParser

__all__ = ["strip_tags", "extract_links", "extract_text"]

# ---------------------------------------------------------------------------
# strip_tags
# ---------------------------------------------------------------------------
_TAG_RE = re.compile(r"<[^>]*>")

def strip_tags(html: str) -> str:
    """Remove all HTML tags and return the plain text.

    Strategy: strip every "<...>" run, then collapse runs of whitespace only
    when they sit *between* stripped tags, preserving the original whitespace
    structure of natural text.
    """
    if not html:
        return ""
    return _TAG_RE.sub("", html)

# ---------------------------------------------------------------------------
# extract_links
# ---------------------------------------------------------------------------
class _LinkCollector(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.links: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag.lower() != "a":
            return
        for name, value in attrs:
            if name == "href" and value is not None:
                self.links.append(value)
                return

def extract_links(html: str) -> list[str]:
    """Return the list of href URLs from <a href="..."> elements."""
    if not html:
        return []
    parser = _LinkCollector()
    parser.feed(html)
    parser.close()
    return parser.links

# ---------------------------------------------------------------------------
# extract_text
# ---------------------------------------------------------------------------
class _TextExtractor(HTMLParser):
    def __init__(self, target_tag: str) -> None:
        super().__init__(convert_charrefs=True)
        self._target = target_tag.lower()
        self._depth = 0
        self._buf: list[str] = []
        self._results: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag.lower() == self._target:
            if self._depth == 0:
                self._buf = []
            self._depth += 1

    def handle_endtag(self, tag: str) -> None:
        if tag.lower() == self._target and self._depth > 0:
            self._depth -= 1
            if self._depth == 0:
                self._results.append("".join(self._buf))
                self._buf = []

    def handle_data(self, data: str) -> None:
        if self._depth > 0:
            self._buf.append(data)

def extract_text(html: str, tag: str) -> list[str]:
    """Return the list of text contents for each <tag>...</tag> element."""
    if not html or not tag:
        return []
    parser = _TextExtractor(tag)
    parser.feed(html)
    parser.close()
    return parser._results
