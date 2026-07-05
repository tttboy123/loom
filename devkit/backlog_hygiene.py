"""Backlog hygiene helpers for archiving stale queue state.

This keeps the active backlog small without discarding audit history:
- move explicit human-required stops to a dedicated manual queue bundle
- archive explicit superseded/stale stops
- cascade-stop descendants that only depend on moved work
- archive completed tasks that no longer have active dependents
"""
from __future__ import annotations

import json
import os
import pathlib
from collections import Counter
from datetime import datetime, timezone

ARCHIVE_STOP_REASONS = frozenset(
    {
        "legacy_archived_failure",
        "manual_stop_after_platform_fix",
        "implemented_in_control_plane_migration_2026_07_04",
        "stale_missing_workspace_paths",
        "superseded_by_family_head",
        "superseded_by_integrated_rdloop_bridge",
        "superseded_by_manual_codex_fix_2026_07_03",
        "blocked_by_archived_dependency",
    }
)
HUMAN_REQUIRED_STOP_REASONS = frozenset(
    {
        "human_required_report_only",
        "human_required_manual_queue",
    }
)
CASCADE_STOP_REASON = "blocked_by_archived_dependency"


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def timestamp_slug() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d-%H%M%S")


def load_backlog(path: pathlib.Path | str) -> tuple[list[dict], bool]:
    file_path = pathlib.Path(path)
    if not file_path.exists():
        return [], False
    data = json.loads(file_path.read_text(encoding="utf-8"))
    if isinstance(data, dict) and isinstance(data.get("tasks"), list):
        return [dict(item) for item in data["tasks"] if isinstance(item, dict)], True
    if isinstance(data, list):
        return [dict(item) for item in data if isinstance(item, dict)], False
    return [], False


def write_backlog(path: pathlib.Path | str, items: list[dict], *, wrapped: bool) -> None:
    payload = {"tasks": items} if wrapped else items
    _atomic_write_text(pathlib.Path(path), json.dumps(payload, ensure_ascii=False, indent=2) + "\n")


def load_bundle(path: pathlib.Path | str) -> dict:
    file_path = pathlib.Path(path)
    if not file_path.exists():
        return {}
    data = json.loads(file_path.read_text(encoding="utf-8"))
    return data if isinstance(data, dict) else {}


def write_bundle(path: pathlib.Path | str, payload: dict) -> None:
    _atomic_write_text(pathlib.Path(path), json.dumps(payload, ensure_ascii=False, indent=2) + "\n")


def plan_hygiene(
    backlog: list[dict],
    *,
    move_human_required: bool = True,
    archive_done_without_active_children: bool = True,
    archived_at: str | None = None,
) -> dict:
    items = [dict(item) for item in backlog if isinstance(item, dict)]
    archived_at = archived_at or now_iso()
    by_id = {
        str(item.get("id", "")).strip(): item
        for item in items
        if str(item.get("id", "")).strip()
    }

    human_queue_ids: set[str] = set()
    archive_ids: set[str] = set()
    human_queue_items: list[dict] = []
    archive_items: list[dict] = []

    def move_to_human(task_id: str) -> None:
        if not task_id or task_id in human_queue_ids or task_id in archive_ids:
            return
        row = dict(by_id[task_id])
        row["_archived_at"] = archived_at
        row["_archived_from"] = "active_backlog"
        row["_queue_kind"] = "human_required"
        human_queue_ids.add(task_id)
        human_queue_items.append(row)

    def move_to_archive(task_id: str, *, force_stop_reason: str | None = None, blocked_by: list[str] | None = None) -> None:
        if not task_id or task_id in human_queue_ids or task_id in archive_ids:
            return
        row = dict(by_id[task_id])
        if force_stop_reason:
            row["status"] = "stopped"
            row["stop_reason"] = force_stop_reason
        if blocked_by:
            row["blocked_by_archived_ids"] = list(blocked_by)
        row["_archived_at"] = archived_at
        row["_archived_from"] = "active_backlog"
        archive_ids.add(task_id)
        archive_items.append(row)

    for task_id, item in by_id.items():
        if str(item.get("status", "")).strip().lower() != "stopped":
            continue
        stop_reason = str(item.get("stop_reason", "")).strip()
        if move_human_required and stop_reason in HUMAN_REQUIRED_STOP_REASONS:
            move_to_human(task_id)
            continue
        if stop_reason in ARCHIVE_STOP_REASONS or str(item.get("superseded_by", "")).strip():
            move_to_archive(task_id)

    moved_ids = human_queue_ids | archive_ids
    changed = True
    while changed:
        changed = False
        for task_id, item in list(by_id.items()):
            if task_id in moved_ids:
                continue
            status = str(item.get("status", "")).strip().lower()
            if status == "running":
                continue
            deps = [str(dep).strip() for dep in (item.get("deps") or []) if str(dep).strip()]
            blocked_by = sorted(dep for dep in deps if dep in moved_ids)
            if not blocked_by:
                continue
            move_to_archive(task_id, force_stop_reason=CASCADE_STOP_REASON, blocked_by=blocked_by)
            moved_ids = human_queue_ids | archive_ids
            changed = True

    if archive_done_without_active_children:
        remaining_ids = {task_id for task_id in by_id if task_id not in moved_ids}
        referenced_by_remaining: set[str] = set()
        for task_id in remaining_ids:
            for dep in by_id[task_id].get("deps") or []:
                dep_id = str(dep).strip()
                if dep_id:
                    referenced_by_remaining.add(dep_id)
        for task_id in sorted(remaining_ids):
            item = by_id[task_id]
            if str(item.get("status", "")).strip().lower() != "done":
                continue
            if task_id in referenced_by_remaining:
                continue
            move_to_archive(task_id)
            moved_ids = human_queue_ids | archive_ids

    active_items = [dict(item) for item_id, item in by_id.items() if item_id not in moved_ids]
    summary = {
        "original_total": len(items),
        "live_total": len(active_items),
        "archived_total": len(archive_items),
        "human_required_total": len(human_queue_items),
        "archived_status_counts": dict(Counter(str(item.get("status", "")).strip() for item in archive_items)),
        "archived_stop_reason_counts": dict(
            Counter(str(item.get("stop_reason", "")).strip() for item in archive_items if str(item.get("stop_reason", "")).strip())
        ),
        "human_required_reason_counts": dict(
            Counter(str(item.get("human_required_reason", "")).strip() for item in human_queue_items if str(item.get("human_required_reason", "")).strip())
        ),
    }
    return {
        "active_backlog": active_items,
        "archive_items": archive_items,
        "human_required_items": human_queue_items,
        "summary": summary,
        "policy": {
            "move_human_required": move_human_required,
            "archive_done_without_active_children": archive_done_without_active_children,
            "archive_stop_reasons": sorted(ARCHIVE_STOP_REASONS),
            "human_required_stop_reasons": sorted(HUMAN_REQUIRED_STOP_REASONS),
            "cascade_stop_reason": CASCADE_STOP_REASON,
        },
    }


def apply_hygiene(
    *,
    backlog_path: pathlib.Path | str,
    archive_dir: pathlib.Path | str | None = None,
    human_queue_path: pathlib.Path | str | None = None,
    move_human_required: bool = True,
    archive_done_without_active_children: bool = True,
    archived_at: str | None = None,
) -> dict:
    backlog_path = pathlib.Path(backlog_path)
    archive_dir = pathlib.Path(archive_dir or backlog_path.parent)
    human_queue_path = pathlib.Path(human_queue_path or backlog_path.with_name("backlog.human-required.json"))
    items, wrapped = load_backlog(backlog_path)
    archived_at = archived_at or now_iso()

    plan = plan_hygiene(
        items,
        move_human_required=move_human_required,
        archive_done_without_active_children=archive_done_without_active_children,
        archived_at=archived_at,
    )
    write_backlog(backlog_path, plan["active_backlog"], wrapped=wrapped)

    archive_dir.mkdir(parents=True, exist_ok=True)
    archive_path = archive_dir / f"backlog.archive.{timestamp_slug()}.json"
    archive_payload = {
        "archived_at": archived_at,
        "source": str(backlog_path),
        "policy": dict(plan["policy"]),
        "summary": dict(plan["summary"]),
        "items": plan["archive_items"],
    }
    write_bundle(archive_path, archive_payload)

    existing_human = load_bundle(human_queue_path)
    existing_items = existing_human.get("items", []) if isinstance(existing_human.get("items"), list) else []
    merged_human_items = _merge_queue_items(existing_items, plan["human_required_items"])
    human_payload = {
        "generated_at": archived_at,
        "source": str(backlog_path),
        "summary": {
            "total": len(merged_human_items),
            "newly_added": len(plan["human_required_items"]),
            "reason_counts": dict(
                Counter(
                    str(item.get("human_required_reason", "")).strip()
                    for item in merged_human_items
                    if str(item.get("human_required_reason", "")).strip()
                )
            ),
        },
        "items": merged_human_items,
    }
    write_bundle(human_queue_path, human_payload)

    return {
        **plan,
        "archive_path": str(archive_path),
        "human_queue_path": str(human_queue_path),
    }


def _merge_queue_items(existing: list[dict], incoming: list[dict]) -> list[dict]:
    merged: dict[str, dict] = {}
    ordered_ids: list[str] = []
    for row in list(existing) + list(incoming):
        if not isinstance(row, dict):
            continue
        task_id = str(row.get("id", "")).strip()
        if not task_id:
            continue
        if task_id not in merged:
            ordered_ids.append(task_id)
        merged[task_id] = dict(row)
    return [merged[task_id] for task_id in ordered_ids]


def _atomic_write_text(path: pathlib.Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    with tmp.open("w", encoding="utf-8") as fp:
        fp.write(text)
        fp.flush()
        os.fsync(fp.fileno())
    os.replace(tmp, path)
