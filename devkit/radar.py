# devkit/radar.py — Community Radar: discover and import MCPs/skills/rules into assets
from __future__ import annotations

import json
import pathlib
import urllib.request
from typing import Any, Optional

# Curated seed catalog — quality-checked MCP servers and prompt skills
CATALOG: list[dict[str, Any]] = [
    {
        "name": "mcp-filesystem",
        "category": "mcp",
        "description": "官方 MCP 文件系统服务器（读/写/搜索本地文件）",
        "source": "modelcontextprotocol/servers",
        "tags": ["filesystem", "official"],
        "install_hint": "npx -y @modelcontextprotocol/server-filesystem <path>",
        "trust_level": 3,
    },
    {
        "name": "mcp-brave-search",
        "category": "mcp",
        "description": "Brave Search MCP — 网络搜索（需 BRAVE_API_KEY）",
        "source": "modelcontextprotocol/servers",
        "tags": ["search", "web", "official"],
        "install_hint": "npx -y @modelcontextprotocol/server-brave-search",
        "trust_level": 3,
    },
    {
        "name": "mcp-github",
        "category": "mcp",
        "description": "官方 GitHub MCP — 读写 PR/Issue/仓库",
        "source": "modelcontextprotocol/servers",
        "tags": ["github", "git", "official"],
        "install_hint": "npx -y @modelcontextprotocol/server-github",
        "trust_level": 3,
    },
    {
        "name": "mcp-sqlite",
        "category": "mcp",
        "description": "SQLite MCP — 本地数据库读写",
        "source": "modelcontextprotocol/servers",
        "tags": ["database", "sql", "official"],
        "install_hint": "npx -y @modelcontextprotocol/server-sqlite <db-path>",
        "trust_level": 3,
    },
    {
        "name": "mcp-memory",
        "category": "mcp",
        "description": "官方 Memory MCP — 持久化键值知识图谱",
        "source": "modelcontextprotocol/servers",
        "tags": ["memory", "knowledge", "official"],
        "install_hint": "npx -y @modelcontextprotocol/server-memory",
        "trust_level": 3,
    },
    {
        "name": "rule-no-overengineering",
        "category": "rule",
        "description": "反过度设计规则：最小 diff、零新依赖、无多余抽象",
        "source": "loom-builtin",
        "tags": ["quality", "ponytail", "builtin"],
        "content": (
            "你是一个严格的代码审查员。\n"
            "规则：\n"
            "1. 最小 diff — 只改任务要求的行，不做顺手重构\n"
            "2. 零新依赖 — 不引入 requirements 中没有的包\n"
            "3. 无多余抽象 — 三处相似才提取函数；不提前设计接口\n"
            "4. 无占位注释 — 不写 'TODO: implement later'\n"
            "违反任意一条 → REQUEST-CHANGES，说明违反了哪条。"
        ),
        "trust_level": 6,
    },
    {
        "name": "rule-tdd-first",
        "category": "rule",
        "description": "TDD 优先规则：先写失败测试，再写实现",
        "source": "loom-builtin",
        "tags": ["tdd", "testing", "builtin"],
        "content": (
            "执行顺序：\n"
            "1. 先写会失败的测试（pytest / unittest）\n"
            "2. 写最少代码让测试通过\n"
            "3. 重构（保持测试绿）\n"
            "不允许：先写实现再补测试。"
        ),
        "trust_level": 6,
    },
    {
        "name": "rule-security-baseline",
        "category": "rule",
        "description": "安全基线规则：禁止硬编码密钥、shell 注入、eval",
        "source": "loom-builtin",
        "tags": ["security", "baseline", "builtin"],
        "content": (
            "安全基线（违反即 NO-GO）：\n"
            "- 禁止硬编码 API key / password / token（用环境变量）\n"
            "- 禁止 subprocess with shell=True\n"
            "- 禁止 os.system()\n"
            "- 禁止 eval() 处理外部输入\n"
            "- SQL 查询必须用参数化，禁止字符串拼接"
        ),
        "trust_level": 6,
    },
    {
        "name": "skill-python-stdlib-only",
        "category": "skill",
        "description": "约束：只用 Python 标准库，不引入第三方包",
        "source": "loom-builtin",
        "tags": ["python", "stdlib", "constraint", "builtin"],
        "content": "约束：本次实现只允许使用 Python 标准库。不得 import 任何第三方包（不在 stdlib 的）。",
        "trust_level": 6,
    },
    {
        "name": "skill-output-one-file",
        "category": "skill",
        "description": "约束：只输出一个文件，不修改已有文件",
        "source": "loom-builtin",
        "tags": ["constraint", "builtin"],
        "content": "约束：只交付一个新文件。不得修改任何已有文件。",
        "trust_level": 6,
    },
]

_SCAN_PATTERNS = [
    # (glob_pattern, asset_name_prefix, category, description_template)
    ("CLAUDE.md", "claudemd", "rule", "Claude Code 项目规则（CLAUDE.md）"),
    (".claude/CLAUDE.md", "claudemd", "rule", "Claude Code 项目规则（.claude/CLAUDE.md）"),
    (".clinerules", "clinerules", "rule", "Cline 规则文件"),
    (".roomodes", "roomodes", "rule", "Roo Code 模式配置"),
    (".aider.conf.yml", "aider-conf", "skill", "Aider 配置约束"),
    (".cursorrules", "cursorrules", "rule", "Cursor 规则文件"),
    ("system-prompt.md", "system-prompt", "rule", "系统 Prompt"),
    ("system_prompt.md", "system-prompt", "rule", "系统 Prompt"),
]


def list_catalog(category: Optional[str] = None) -> list[dict]:
    """Return the curated catalog, optionally filtered by category."""
    if category:
        return [e for e in CATALOG if e.get("category") == category]
    return list(CATALOG)


def get_catalog_entry(name: str) -> dict | None:
    """Fetch one catalog entry by name."""
    for e in CATALOG:
        if e["name"] == name:
            return e
    return None


def scan_dir(path: pathlib.Path | str | None = None) -> list[dict]:
    """
    Scan a directory for known tool config files (CLAUDE.md, .clinerules, etc.)
    Returns a list of scan results ready to be imported as assets.
    """
    base = pathlib.Path(path) if path else pathlib.Path.cwd()
    results = []
    seen_names: set[str] = set()
    for glob_pat, name_prefix, category, desc_tmpl in _SCAN_PATTERNS:
        candidate = base / glob_pat
        if not candidate.is_file():
            continue
        try:
            content = candidate.read_text(encoding="utf-8").strip()
        except Exception:
            continue
        if not content:
            continue
        # Deduplicate by name_prefix
        name = name_prefix
        suffix = 1
        while name in seen_names:
            name = f"{name_prefix}-{suffix}"
            suffix += 1
        seen_names.add(name)
        results.append({
            "name": name,
            "category": category,
            "description": desc_tmpl,
            "source": str(candidate),
            "tags": ["imported", category],
            "content": content,
            "trust_level": 0,  # external — review before trusting
        })
    return results


def fetch_remote(url: str, timeout: int = 10) -> str | None:
    """Fetch raw text from a URL. Returns None on failure."""
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "loom-radar/1.0"})
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.read().decode("utf-8", errors="replace")[:32000]
    except Exception:
        return None


def import_to_assets(
    name: str,
    assets_path: Optional[pathlib.Path] = None,
    catalog_entry: Optional[dict] = None,
    scan_result: Optional[dict] = None,
) -> dict | None:
    """
    Import a catalog entry or scan result into the asset store.
    Returns the created/updated asset dict, or None if the entry has no content.
    """
    from devkit import asset as A

    entry = catalog_entry or scan_result or get_catalog_entry(name)
    if entry is None:
        return None

    content = entry.get("content", "")
    if not content:
        # For MCP entries without inline content, build a stub with install instructions
        hint = entry.get("install_hint", "")
        desc = entry.get("description", "")
        content = f"{desc}\n\n安装方式：\n{hint}" if hint else desc
    if not content:
        return None

    tags = list(entry.get("tags", []))
    trust = int(entry.get("trust_level", 0))
    a = A.add_asset(
        name=entry["name"],
        asset_type=entry.get("category", "rule"),
        content=content,
        tags=tags,
        path=assets_path,
        trust_level=trust,
    )
    return a
