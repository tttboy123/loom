"""A2A AgentCard schema (loader module).

This is the canonical kind constant for A2A AgentCard documents.

AGENT_CARD_KIND == "AgentCard" is exported so other modules can import
the constant rather than hardcoding the string.
"""
from __future__ import annotations

import json
import pathlib

SCHEMA_PATH = pathlib.Path(__file__).resolve().parent / "a2a_agent_card.schema.json"
SCHEMA = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))

AGENT_CARD_KIND = "AgentCard"