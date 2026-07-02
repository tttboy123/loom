"""Checkpoint manager for the pipeline. Stdlib only."""
from datetime import datetime, timezone

def save(name: str, state: dict) -> dict:
    """Persist a checkpoint snapshot. Returns {name, state, saved_at}."""
    return {
        'name': name,
        'state': state,
        'saved_at': datetime.now(timezone.utc).isoformat(),
    }

def load(checkpoints: list, name: str):
    """Return the newest checkpoint matching `name`, or None."""
    matches = [cp for cp in checkpoints if cp.get('name') == name]
    if not matches:
        return None
    return max(matches, key=lambda cp: cp.get('saved_at', ''))

def list_checkpoints(checkpoints: list) -> list:
    """Unique checkpoint names, sorted ascending."""
    return sorted({cp.get('name') for cp in checkpoints})

def prune(checkpoints: list, keep: int) -> list:
    """Keep the `keep` most recent checkpoints (by saved_at desc). keep<=0 -> []."""
    if keep <= 0:
        return []
    return sorted(
        checkpoints,
        key=lambda cp: cp.get('saved_at', ''),
        reverse=True,
    )[:keep]
