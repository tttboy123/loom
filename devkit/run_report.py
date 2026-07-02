# devkit/run_report.py
"""HTML summary report generator for a single Loom run. Standard library only."""
from __future__ import annotations

import os
import re

_GATE_RE = re.compile(r"Gate[:：]\s*(.+)", re.IGNORECASE)


def load_run(run_dir: str) -> dict:
    """Read all .md files in run_dir; return {run_id, stages, gate}.

    Returns {run_id:'', stages:[], gate:''} when run_dir does not exist.
    """
    if not os.path.isdir(run_dir):
        return {"run_id": "", "stages": [], "gate": ""}

    run_id = os.path.basename(run_dir.rstrip("/"))
    gate = ""
    stages: list[dict] = []

    try:
        entries = sorted(os.listdir(run_dir))
    except OSError:
        return {"run_id": run_id, "stages": [], "gate": ""}

    for name in entries:
        if not name.endswith(".md"):
            continue
        path = os.path.join(run_dir, name)
        try:
            content = open(path, encoding="utf-8").read()
        except OSError:
            content = ""

        if name == "run-log.md":
            m = _GATE_RE.search(content)
            if m:
                gate = m.group(1).strip()
        else:
            stages.append({"name": name[:-3], "content": content})

    return {"run_id": run_id, "stages": stages, "gate": gate}


def to_html(run_data: dict) -> str:
    """Convert run_data dict to an HTML string starting with <!DOCTYPE html>."""
    run_id = run_data.get("run_id", "") or "Unknown Run"
    gate = run_data.get("gate", "") or ""
    stages = run_data.get("stages", []) or []

    stage_html = ""
    for s in stages:
        name = s.get("name", "")
        content = s.get("content", "")
        stage_html += (
            f"<section><h2>{name}</h2>"
            f"<pre>{_escape(content)}</pre></section>\n"
        )

    return (
        f"<!DOCTYPE html>\n<html lang='en'><head><meta charset='UTF-8'>"
        f"<title>Run: {_escape(run_id)}</title></head><body>\n"
        f"<h1>Run: {_escape(run_id)}</h1>\n"
        f"<p>Gate: <strong>{_escape(gate)}</strong></p>\n"
        f"{stage_html}"
        f"</body></html>"
    )


def _escape(s: str) -> str:
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
