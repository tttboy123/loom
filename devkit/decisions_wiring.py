"""devkit/decisions_wiring.py — Record decision outcomes to JSONL log.

Pure standard library, no dependencies beyond Python 3.8+.
"""

import json
import os
from datetime import UTC, datetime

def record_outcome(
    task_id: str,
    outcome: str,
    carrier: str = "",
    tokens: int = 0,
    log_path: str = "devkit/decisions.jsonl",
) -> None:
    """Append a single decision record to the JSONL log. Silent on failure."""
    try:
        record = {
            "task_id": task_id,
            "outcome": outcome,
            "carrier": carrier,
            "tokens": tokens,
            "ts": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
        }
        # Ensure parent directory exists (only for relative paths or existing dirs)
        log_dir = os.path.dirname(log_path)
        if log_dir and not os.path.exists(log_dir):
            os.makedirs(log_dir, exist_ok=True)
        with open(log_path, "a", encoding="utf-8") as f:
            f.write(json.dumps(record, ensure_ascii=False) + "\n")
    except Exception:
        pass  # Silent per spec

def batch_record(
    results: list[dict],
    log_path: str = "devkit/decisions.jsonl",
) -> int:
    """Write multiple decision records. Skips dicts missing task_id or outcome.
    Returns the number of successfully written records.
    """
    written = 0
    try:
        with open(log_path, "a", encoding="utf-8") as f:
            for item in results:
                if not isinstance(item, dict):
                    continue
                if "task_id" not in item or "outcome" not in item:
                    continue
                record = {
                    "task_id": item["task_id"],
                    "outcome": item["outcome"],
                    "carrier": item.get("carrier", ""),
                    "tokens": item.get("tokens", 0),
                    "ts": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
                }
                f.write(json.dumps(record, ensure_ascii=False) + "\n")
                written += 1
    except Exception:
        pass  # Silent on I/O errors
    return written

def read_recent(
    n: int = 20,
    log_path: str = "devkit/decisions.jsonl",
) -> list[dict]:
    """Read the most recent n records from the log. Returns [] if file missing."""
    if n <= 0:
        return []
    try:
        with open(log_path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except (FileNotFoundError, OSError):
        return []

    records = []
    for line in reversed(lines):
        line = line.strip()
        if not line:
            continue
        try:
            record = json.loads(line)
            if isinstance(record, dict):
                records.append(record)
                if len(records) >= n:
                    break
        except (json.JSONDecodeError, ValueError):
            continue
    return records
