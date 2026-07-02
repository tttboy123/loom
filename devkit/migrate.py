# devkit/migrate.py — Workflow Migration: import configs from other AI coding tools
from __future__ import annotations

import pathlib
from typing import Optional

# Tool detection: (tool_name, description, file_patterns)
_TOOL_SIGNATURES: list[tuple[str, str, list[str]]] = [
    ("claude-code",  "Claude Code",   ["CLAUDE.md", ".claude/CLAUDE.md", ".claude/settings.json"]),
    ("codex",        "OpenAI Codex",  [".codex/", "codex.json"]),
    ("aider",        "Aider",         [".aider.conf.yml", ".aider.model.metadata.json"]),
    ("cline",        "Cline",         [".clinerules", ".cline/"]),
    ("roo",          "Roo Code",      [".roomodes", ".roo/"]),
    ("cursor",       "Cursor",        [".cursorrules", ".cursor/"]),
    ("continue",     "Continue.dev",  [".continue/", ".continuerc.json"]),
    ("openclaw",     "OpenClaw",      ["openclaw.json", ".openclaw/"]),
    ("hermes",       "Hermes",        ["hermes.config.json", ".hermes/"]),
]


def detect(search_dir: Optional[pathlib.Path] = None) -> dict[str, dict]:
    """
    Detect which AI coding tools have configs in the directory.

    Returns {tool_name: {"found": bool, "files": [...], "description": str}}.
    """
    base = pathlib.Path(search_dir) if search_dir else pathlib.Path.cwd()
    result = {}
    for tool, desc, patterns in _TOOL_SIGNATURES:
        found_files = []
        for pat in patterns:
            candidate = base / pat
            if candidate.exists():
                found_files.append(str(candidate.relative_to(base)))
        result[tool] = {
            "description": desc,
            "found": bool(found_files),
            "files": found_files,
        }
    return result


def _read_file_safe(path: pathlib.Path, max_bytes: int = 32000) -> str:
    try:
        return path.read_bytes()[:max_bytes].decode("utf-8", errors="replace").strip()
    except Exception:
        return ""


def migrate_tool(
    tool_name: str,
    source_dir: Optional[pathlib.Path] = None,
    assets_path: Optional[pathlib.Path] = None,
) -> list[dict]:
    """
    Import configs from a specific tool as assets. Returns list of created assets.
    Supported tools: claude-code, aider, cline, roo, cursor, codex, continue.
    """
    from devkit import asset as A

    base = pathlib.Path(source_dir) if source_dir else pathlib.Path.cwd()
    created: list[dict] = []

    if tool_name == "claude-code":
        for fname in ("CLAUDE.md", ".claude/CLAUDE.md"):
            p = base / fname
            if p.is_file():
                content = _read_file_safe(p)
                if content:
                    a = A.add_asset(
                        name="claude-code-rules",
                        asset_type="rule",
                        content=content,
                        tags=["imported", "claude-code", "rules"],
                        path=assets_path,
                        trust_level=1,
                    )
                    created.append(a)
                    break
        # Also import settings if present
        settings_p = base / ".claude" / "settings.json"
        if settings_p.is_file():
            content = _read_file_safe(settings_p)
            if content:
                a = A.add_asset(
                    name="claude-code-settings",
                    asset_type="rule",
                    content=f"# Claude Code settings.json\n```json\n{content}\n```",
                    tags=["imported", "claude-code", "settings"],
                    path=assets_path,
                    trust_level=1,
                )
                created.append(a)

    elif tool_name == "aider":
        p = base / ".aider.conf.yml"
        if p.is_file():
            content = _read_file_safe(p)
            if content:
                a = A.add_asset(
                    name="aider-config",
                    asset_type="skill",
                    content=f"# Aider 配置约束（.aider.conf.yml）\n```yaml\n{content}\n```",
                    tags=["imported", "aider"],
                    path=assets_path,
                    trust_level=1,
                )
                created.append(a)

    elif tool_name == "cline":
        p = base / ".clinerules"
        if p.is_file():
            content = _read_file_safe(p)
            if content:
                a = A.add_asset(
                    name="cline-rules",
                    asset_type="rule",
                    content=content,
                    tags=["imported", "cline", "rules"],
                    path=assets_path,
                    trust_level=1,
                )
                created.append(a)

    elif tool_name == "roo":
        p = base / ".roomodes"
        if p.is_file():
            content = _read_file_safe(p)
            if content:
                a = A.add_asset(
                    name="roo-modes",
                    asset_type="rule",
                    content=content,
                    tags=["imported", "roo"],
                    path=assets_path,
                    trust_level=1,
                )
                created.append(a)

    elif tool_name == "cursor":
        p = base / ".cursorrules"
        if p.is_file():
            content = _read_file_safe(p)
            if content:
                a = A.add_asset(
                    name="cursor-rules",
                    asset_type="rule",
                    content=content,
                    tags=["imported", "cursor", "rules"],
                    path=assets_path,
                    trust_level=1,
                )
                created.append(a)

    elif tool_name == "codex":
        for fname in ("codex.json", ".codex/config.json"):
            p = base / fname
            if p.is_file():
                content = _read_file_safe(p)
                if content:
                    a = A.add_asset(
                        name="codex-config",
                        asset_type="skill",
                        content=f"# Codex 配置\n```json\n{content}\n```",
                        tags=["imported", "codex"],
                        path=assets_path,
                        trust_level=1,
                    )
                    created.append(a)
                    break

    elif tool_name == "continue":
        for fname in (".continuerc.json", ".continue/config.json"):
            p = base / fname
            if p.is_file():
                content = _read_file_safe(p)
                if content:
                    a = A.add_asset(
                        name="continue-config",
                        asset_type="skill",
                        content=f"# Continue.dev 配置\n```json\n{content}\n```",
                        tags=["imported", "continue"],
                        path=assets_path,
                        trust_level=1,
                    )
                    created.append(a)
                    break

    elif tool_name in ("openclaw", "hermes"):
        fnames = {
            "openclaw": ["openclaw.json", ".openclaw/config.json"],
            "hermes": ["hermes.config.json", ".hermes/config.json"],
        }[tool_name]
        for fname in fnames:
            p = base / fname
            if p.is_file():
                content = _read_file_safe(p)
                if content:
                    a = A.add_asset(
                        name=f"{tool_name}-config",
                        asset_type="skill",
                        content=f"# {tool_name} 配置\n```json\n{content}\n```",
                        tags=["imported", tool_name],
                        path=assets_path,
                        trust_level=2,
                    )
                    created.append(a)
                    break

    return created


def migrate_all(
    source_dir: Optional[pathlib.Path] = None,
    assets_path: Optional[pathlib.Path] = None,
) -> dict[str, list[dict]]:
    """
    Run migration for all detected tools in source_dir.
    Returns {tool_name: [created_assets]}.
    """
    detected = detect(source_dir)
    results: dict[str, list[dict]] = {}
    for tool_name, info in detected.items():
        if info["found"]:
            assets = migrate_tool(tool_name, source_dir, assets_path)
            if assets:
                results[tool_name] = assets
    return results
