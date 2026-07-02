# devkit/watchdog.py
"""Watchdog: detect autoloop health (gateway + backlog). Standard library only."""
from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from typing import Any

# ---------- gateway ----------

def check_gateway(base_url: str = 'http://localhost:4000',
                  timeout: int = 3) -> dict:
    """GET {base_url}/health via urllib.

    Returns:
        {ok: bool, latency_ms: float, error: str}
        ok=False on connection failure / timeout.
    """
    result: dict[str, Any] = {"ok": False, "latency_ms": 0.0, "error": ""}
    url = base_url.rstrip("/") + "/health"
    start = time.perf_counter()
    try:
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            resp.read(1024)  # drain
            status = getattr(resp, "status", 200)
            elapsed_ms = (time.perf_counter() - start) * 1000.0
            result["latency_ms"] = float(elapsed_ms)
            if 200 <= status < 500:
                result["ok"] = True
            else:
                result["error"] = f"HTTP {status}"
    except (urllib.error.URLError, TimeoutError, OSError) as e:
        elapsed_ms = (time.perf_counter() - start) * 1000.0
        result["latency_ms"] = float(elapsed_ms)
        result["error"] = str(e) or e.__class__.__name__
    return result

# ---------- backlog ----------

def _iter_items(data: Any) -> list[dict]:
    """Normalize backlog data to a list of item dicts."""
    if isinstance(data, list):
        return [x for x in data if isinstance(x, dict)]
    if isinstance(data, dict):
        items = data.get("items")
        if isinstance(items, list):
            return [x for x in items if isinstance(x, dict)]
        tasks = data.get("tasks")
        if isinstance(tasks, list):
            return [x for x in tasks if isinstance(x, dict)]
    return []

def check_backlog(backlog_path: str = "devkit/backlog.json") -> dict:
    """Inspect backlog.json for stuck tasks.

    Returns:
        {ok: bool, total: int, pending: int, stuck: bool, error: str}
        stuck=True iff running>0 AND every running item has _attempts >= 2.
        ok=False when file missing / unreadable.
    """
    result: dict[str, Any] = {
        "ok": False, "total": 0, "pending": 0, "stuck": False, "error": "",
    }
    try:
        with open(backlog_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except FileNotFoundError as e:
        result["error"] = f"not found: {backlog_path}"
        return result
    except (json.JSONDecodeError, OSError, PermissionError) as e:
        result["error"] = str(e) or e.__class__.__name__
        return result

    items = _iter_items(data)
    total = 0
    pending = 0
    running: list[dict] = []

    for it in items:
        count = int(it.get("count", 1) or 1)
        total += count
        status = str(it.get("status", "pending")).lower()
        if status not in ("done", "completed", "finished"):
            pending += count
        if status == "running":
            running.append(it)

    stuck = bool(running) and all(
        int(it.get("_attempts", 0) or 0) >= 2 for it in running
    )

    result.update({"ok": True, "total": total, "pending": pending, "stuck": stuck})
    return result

# ---------- combined ----------

def health_report(base_url: str = "http://localhost:4000") -> dict:
    """Combined gateway + backlog health report.

    Returns:
        {gateway: dict, backlog: dict, overall_ok: bool}
        overall_ok = gateway.ok AND backlog.ok
    """
    gateway = check_gateway(base_url)
    backlog = check_backlog()
    return {
        "gateway": gateway,
        "backlog": backlog,
        "overall_ok": bool(gateway.get("ok")) and bool(backlog.get("ok")),
    }
