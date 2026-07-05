"""A2A AgentMessage schema (loader module)."""
from __future__ import annotations

import json
import pathlib

SCHEMA_PATH = pathlib.Path(__file__).resolve().parent / "a2a_message.schema.json"
SCHEMA = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))

AGENT_MESSAGE_KIND = "AgentMessage"