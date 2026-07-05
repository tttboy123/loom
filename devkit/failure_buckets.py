"""Failure bucketing for active control-plane queue items."""
from __future__ import annotations

from collections import Counter, defaultdict


def classify_failed_tasks(tasks: list[dict]) -> dict:
    buckets: dict[str, list[dict]] = defaultdict(list)
    status_counts = Counter()
    for task in tasks or []:
        if not isinstance(task, dict):
            continue
        status = str(task.get("status", "")).strip().lower()
        if status not in {"failed", "blocked"}:
            continue
        bucket = classify_task(task)
        status_counts[status] += 1
        buckets[bucket].append(
            {
                "id": str(task.get("id", "")).strip(),
                "status": status,
                "latest_status_code": str(task.get("latest_status_code", "")).strip() or None,
                "task": str(task.get("task", "")).strip(),
            }
        )
    return {
        "totals": {
            "failed_or_blocked": sum(len(items) for items in buckets.values()),
            "status_counts": dict(status_counts),
            "bucket_counts": {bucket: len(items) for bucket, items in sorted(buckets.items())},
        },
        "buckets": {bucket: items for bucket, items in sorted(buckets.items())},
    }


def classify_task(task: dict) -> str:
    status_code = str(task.get("latest_status_code", "")).strip().lower()
    text = " ".join(
        [
            str(task.get("id", "")),
            str(task.get("task", "")),
            str(task.get("source_failure_code", "")),
            str(task.get("source_status_code", "")),
        ]
    ).lower()
    if status_code == "review_request_changes":
        return "review.request_changes"
    if status_code == "blocked":
        if any(token in text for token in ("api key", "auth", "401", "model", "provider", "quota")):
            return "provider.auth_or_model"
        return "blocked.other"
    if status_code == "tests_failed":
        if any(token in text for token in ("applylock", "allowlist", "lockdir")):
            return "tests_failed.applylock"
        if any(token in text for token in ("materialize", "materializer", "fence", "ast", "codeblock", "artifact")):
            return "tests_failed.materialize"
        if any(token in text for token in ("pytest", "import", "sys.path", "conftest", "collect")):
            return "tests_failed.pytest_import_path"
        return "tests_failed.other"
    return f"{status_code or 'unknown'}.other"
