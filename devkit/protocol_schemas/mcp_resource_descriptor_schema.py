"""MCP ResourceDescriptor schema (loader module)."""
from __future__ import annotations

import json
import pathlib

SCHEMA_PATH = pathlib.Path(__file__).resolve().parent / "mcp_resource_descriptor.schema.json"
SCHEMA = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))

RESOURCE_DESCRIPTOR_KIND = "ResourceDescriptor"