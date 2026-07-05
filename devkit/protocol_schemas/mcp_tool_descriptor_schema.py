"""MCP ToolDescriptor schema (loader module)."""
from __future__ import annotations

import json
import pathlib

SCHEMA_PATH = pathlib.Path(__file__).resolve().parent / "mcp_tool_descriptor.schema.json"
SCHEMA = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))

TOOL_DESCRIPTOR_KIND = "ToolDescriptor"