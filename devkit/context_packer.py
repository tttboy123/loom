# devkit/context_packer.py
"""Pack multiple text sections into an LLM context within a char budget. Stdlib only."""
from __future__ import annotations


def truncate_section(content: str, max_chars: int) -> str:
    return content[:max_chars]


def pack(sections: list[dict], max_chars: int = 4000) -> str:
    sorted_sections = sorted(sections, key=lambda s: s.get("priority", 0), reverse=True)
    parts: list[str] = []
    used = 0
    for s in sorted_sections:
        title = s.get("title", "")
        content = s.get("content", "")
        block = f"## {title}\n{content}\n\n"
        if used + len(block) > max_chars:
            remaining = max_chars - used
            if remaining <= len(f"## {title}\n\n"):
                break
            content = truncate_section(content, remaining - len(f"## {title}\n\n"))
            block = f"## {title}\n{content}\n\n"
        parts.append(block)
        used += len(block)
        if used >= max_chars:
            break
    return "".join(parts).rstrip()


def estimate_fit(sections: list[dict], max_chars: int = 4000) -> dict:
    sorted_sections = sorted(sections, key=lambda s: s.get("priority", 0), reverse=True)
    running = 0
    included = 0
    excluded = 0
    all_chars = 0
    for s in sorted_sections:
        title = s.get("title", "")
        content = s.get("content", "")
        block_len = len(f"## {title}\n{content}\n\n")
        all_chars += block_len
        if running + block_len <= max_chars:
            included += 1
            running += block_len
        else:
            excluded += 1
    return {
        "included": included,
        "excluded": excluded,
        "total_chars": all_chars,
        "fits": all_chars <= max_chars,
    }
