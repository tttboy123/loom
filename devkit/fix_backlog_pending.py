"""Sync backlog items back to pending based on decision log evidence."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


def _read_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def _read_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open("r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            value = json.loads(line)
            if isinstance(value, dict):
                rows.append(value)
    return rows


def _load_backlog_items(path: Path) -> tuple[Any, list[dict[str, Any]]]:
    payload = _read_json(path)
    if isinstance(payload, dict):
        tasks = payload.get("tasks", [])
        if not isinstance(tasks, list):
            raise ValueError("backlog.tasks must be a list")
        items = [item for item in tasks if isinstance(item, dict)]
        return payload, items
    if isinstance(payload, list):
        items = [item for item in payload if isinstance(item, dict)]
        return payload, items
    raise ValueError("backlog must be a list or an object with tasks")


def _write_backlog(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _pending_task_ids(rows: list[dict[str, Any]]) -> set[str]:
    ids: set[str] = set()
    for row in rows:
        if str(row.get("status", "")).strip().lower() != "pending":
            continue
        task_id = str(row.get("task_id", "")).strip()
        if task_id:
            ids.add(task_id)
    return ids


def sync_backlog_pending(
    *,
    decision_log_path: Path,
    backlog_path: Path,
    dry_run: bool = False,
) -> int:
    rows = _read_jsonl(decision_log_path) if decision_log_path.exists() else []
    pending_ids = _pending_task_ids(rows)
    if not pending_ids:
        print("synced=0")
        return 0

    payload, items = _load_backlog_items(backlog_path)
    synced = 0
    changed = False
    for item in items:
        item_id = str(item.get("id", item.get("task_id", ""))).strip()
        if not item_id or item_id not in pending_ids:
            continue
        synced += 1
        if item.get("status") != "pending":
            item["status"] = "pending"
            changed = True

    if synced and changed and not dry_run:
        _write_backlog(backlog_path, payload)
    print(f"synced={synced}")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="fix_backlog_pending")
    parser.add_argument("--decision-log", default="decision_log.jsonl")
    parser.add_argument("--backlog", default="backlog.json")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    try:
        return sync_backlog_pending(
            decision_log_path=Path(args.decision_log),
            backlog_path=Path(args.backlog),
            dry_run=bool(args.dry_run),
        )
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
