"""devkit/scheduler.py — Phase D Loom Scheduler (cross-project task driver).

The Scheduler replaces the ad-hoc "first pending task in backlog.json"
logic that ``devkit.iterate`` and ``devkit.autoloop.pick_next`` used to
own. It is a *selection* module only — it does not run the task, does
not write to ``backlog.json``, and does not replace ``iterate.py``'s
run-loop. The runner still owns the loop; the Scheduler just decides
"given the current backlog and the current leases, what should we run
next?".

Responsibilities
----------------
1. Filter the backlog to ``status == "pending"`` items whose deps are all
   ``done`` / ``skipped`` / (optionally) ``failed``.
2. Reject items that exceed the per-task budget cap (delegates to
   :mod:`devkit.budget`).
3. Reject items that are already leased by another run.
4. Sort by priority (``high`` > ``medium`` > ``low``), then by stable id.
5. Persist / release / inspect a single global lease file (atomic JSON).

This module is **stdlib + jsonschema only** (same constraint as
``devkit.repairer`` and ``devkit.gatekeeper``). It does not import
``iterate`` or ``autoloop`` — it sits above them.

Public API
----------
* :class:`ScheduleDecision` — dataclass describing one selection (or one
  blocked task).
* :func:`select_next_pending` — return the next runnable task, or None.
* :func:`list_blocked` — return every pending task that is blocked,
  together with what blocks it.
* :func:`claim_lease` — atomic lease claim with 5-min freshness window.
* :func:`release_lease` — best-effort lease release.
* :func:`is_lease_stale` — pure check for "is this lease older than X?".

Why a Scheduler (and not just ``iterate.pick_next``)?
-----------------------------------------------------
The legacy ``iterate`` helper is a *reflection prompt generator*; it
assumes a single LLM in the loop and runs the reflection dialogue. The
Scheduler is the typed seam where a non-LLM runner (an A2A / MCP
controller, or any future external system) can ask "what's next?"
without dragging the reflection prompt along with it. Phase D only ships
the typed seam; the integration into ``devkit auto`` is opt-in (see
``devkit/__main__.py`` notes in HANDOFF).
"""
from __future__ import annotations

import json
import logging
import os
import pathlib
import re
import tempfile
import threading
import time
import uuid
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import Any, Iterable, Optional

from devkit import budget as _budget

# ----------------------------------------------------------------------------
# Paths and defaults
# ----------------------------------------------------------------------------
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
DEFAULT_BACKLOG_PATH = REPO_ROOT / "devkit" / "backlog.json"
DEFAULT_LEASE_PATH = REPO_ROOT / "devkit" / "scheduler.lease.json"

PROTOCOL_VERSION = "loom.dev/v1"

VALID_PRIORITIES: tuple[str, ...] = ("high", "medium", "low")
PRIORITY_RANK: dict[str, int] = {"high": 0, "medium": 1, "low": 2}

# statuses that count as "deps already satisfied"
DEP_SATISFIED_STATUSES: frozenset[str] = frozenset({"done", "skipped"})

DEFAULT_LEASE_MAX_AGE_S: int = 300  # 5 minutes

DEFAULT_BUDGET_CAP_USD: float = _budget.DEFAULT_COST_LIMIT_USD


# ----------------------------------------------------------------------------
# Logger
# ----------------------------------------------------------------------------
logger = logging.getLogger("devkit.scheduler")
if not logger.handlers:
    handler = logging.NullHandler()
    logger.addHandler(handler)
logger.setLevel(logging.INFO)


# ----------------------------------------------------------------------------
# Internal helpers
# ----------------------------------------------------------------------------
def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _utc_now_epoch() -> float:
    return datetime.now(timezone.utc).timestamp()


def _ensure_str(value: Any, field_name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{field_name} must be a non-empty string")
    return value.strip()


def _ensure_path(value: Any, field_name: str) -> pathlib.Path:
    if value is None:
        raise ValueError(f"{field_name} is required (path-like)")
    return pathlib.Path(value)


def _coerce_float(value: Any, default: float = 0.0) -> float:
    if value is None or value == "":
        return default
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def _coerce_int(value: Any, default: int = 0) -> int:
    if value is None or value == "":
        return default
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _coerce_str_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    out: list[str] = []
    for item in value:
        text = str(item or "").strip()
        if text and text not in out:
            out.append(text)
    return out


# ----------------------------------------------------------------------------
# Backlog loading — tolerate both legacy {"tasks": [...]} and bare [...]
# ----------------------------------------------------------------------------
def _load_backlog_items(path: pathlib.Path) -> list[dict]:
    """Read a backlog.json and return the list of task dicts.

    Accepts both the legacy ``{"tasks": [...]}`` envelope and a bare list
    (Phase B fix-backlog-pending style). Returns ``[]`` for missing /
    malformed files so the Scheduler degrades to "no work to do".
    """
    if not path.exists():
        return []
    try:
        raw = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        logger.info("scheduler: backlog %s unreadable: %s", path, exc)
        return []
    if isinstance(raw, list):
        items = raw
    elif isinstance(raw, dict):
        tasks = raw.get("tasks")
        items = tasks if isinstance(tasks, list) else []
    else:
        items = []
    out: list[dict] = []
    for item in items:
        if isinstance(item, dict):
            out.append(item)
    return out


def _index_by_id(items: Iterable[dict]) -> dict[str, dict]:
    return {
        str(item.get("id", "")).strip(): item
        for item in items
        if str(item.get("id", "")).strip()
    }


# ----------------------------------------------------------------------------
# Per-task budget probe
# ----------------------------------------------------------------------------
def _estimate_cost_usd(item: dict) -> float:
    """Best-effort USD estimate for a single task.

    Priority order (first hit wins):

    1. ``item.budget_usd`` or ``item.budget`` (legacy: 5 for medium, 10 for high).
    2. ``item.estimated_cost_usd`` (an explicit estimate from a Planner).
    3. Heuristic from the carrier's ``max_tokens`` × default rate ($3/MTok).
    4. Default ``$0.50`` for unknown / legacy entries.

    Returns a *non-negative* number. Callers compare against the cap with
    :func:`devkit.budget.check` for actual enforcement.
    """
    explicit = item.get("budget_usd")
    if explicit is None:
        explicit = item.get("budget")
    if explicit is not None:
        cost = _coerce_float(explicit, -1.0)
        if cost >= 0:
            return cost

    est = item.get("estimated_cost_usd")
    if est is not None:
        cost = _coerce_float(est, -1.0)
        if cost >= 0:
            return cost

    carrier = item.get("carrier") if isinstance(item.get("carrier"), dict) else {}
    if carrier:
        max_tokens = 0
        for role_max in carrier.values():
            if isinstance(role_max, (int, float)):
                max_tokens = max(max_tokens, int(role_max))
        if max_tokens:
            # $3 per million tokens is a reasonable default for devkit tasks.
            return round((max_tokens / 1_000_000.0) * 3.0, 4)
    return 0.50


def _within_budget(item: dict, cap_usd: float) -> bool:
    """Return True iff ``item`` fits inside the budget cap."""
    cost = _estimate_cost_usd(item)
    if cap_usd <= 0:
        # cap of 0 means "no work" — refuse everything
        return False
    if cost > cap_usd:
        return False
    return True


# ----------------------------------------------------------------------------
# ScheduleDecision — typed seam returned to runners
# ----------------------------------------------------------------------------
@dataclass
class ScheduleDecision:
    """What the Scheduler tells the runner to do next.

    Fields:

    * ``work_item_id`` — empty when ``reason`` indicates no work.
    * ``reason`` — one of:

      * ``"ready"`` — picked, deps satisfied, budget ok, no lease conflict.
      * ``"no_pending_tasks"`` — backlog has no pending rows at all.
      * ``"blocked"`` — task exists but is blocked (see ``blocked_by``).
      * ``"leased"`` — task is currently leased by another run.
      * ``"over_budget"`` — task's estimated cost exceeds the cap.
      * ``"backlog_missing"`` — backlog.json missing or unreadable.
    * ``blocked_by`` — dep ids that are not yet ``done``/``skipped``.
    * ``estimated_cost_usd`` — best-effort USD estimate for the run.
    """

    work_item_id: str = ""
    reason: str = "no_pending_tasks"
    blocked_by: list[str] = field(default_factory=list)
    estimated_cost_usd: float = 0.0
    priority: str = "medium"

    # ---- helpers ----
    def is_actionable(self) -> bool:
        return bool(self.work_item_id) and self.reason == "ready"

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict) -> "ScheduleDecision":
        if not isinstance(payload, dict):
            raise ValueError(
                f"decision payload must be a dict, got {type(payload).__name__}"
            )
        return cls(
            work_item_id=str(payload.get("work_item_id", "") or ""),
            reason=str(payload.get("reason", "no_pending_tasks") or "no_pending_tasks"),
            blocked_by=_coerce_str_list(payload.get("blocked_by", [])),
            estimated_cost_usd=_coerce_float(payload.get("estimated_cost_usd", 0.0)),
            priority=str(payload.get("priority", "medium") or "medium"),
        )

    def __post_init__(self) -> None:
        if not isinstance(self.blocked_by, list):
            self.blocked_by = list(self.blocked_by or [])
        if self.priority not in VALID_PRIORITIES:
            self.priority = "medium"


# ----------------------------------------------------------------------------
# Lease helpers — used by select / claim / release / stale
# ----------------------------------------------------------------------------
_LEASE_LOCK = threading.Lock()


def _read_lease_dict(lease_path: pathlib.Path) -> dict | None:
    if not lease_path.exists():
        return None
    try:
        raw = json.loads(lease_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        logger.info("scheduler: lease %s unreadable: %s", lease_path, exc)
        return None
    return raw if isinstance(raw, dict) else None


def _atomic_write_json(path: pathlib.Path, payload: dict) -> pathlib.Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp_name = tempfile.mkstemp(
        prefix=f".{path.name}.", suffix=".tmp", dir=str(path.parent)
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as fh:
            fh.write(json.dumps(payload, ensure_ascii=False, indent=2))
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(tmp_name, path)
    except Exception:
        try:
            os.unlink(tmp_name)
        except OSError:
            pass
        raise
    return path


def _parse_iso(value: Any) -> datetime | None:
    text = str(value or "").strip()
    if not text:
        return None
    try:
        dt = datetime.fromisoformat(text)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def is_lease_stale(
    lease_path: pathlib.Path | str,
    max_age_s: int = DEFAULT_LEASE_MAX_AGE_S,
    *,
    now_epoch: float | None = None,
) -> bool:
    """Return True if ``lease_path`` is missing, malformed, or older than ``max_age_s``.

    ``now_epoch`` is injectable for deterministic tests.
    """
    path = pathlib.Path(lease_path)
    data = _read_lease_dict(path)
    if not data:
        return True
    now = float(now_epoch) if now_epoch is not None else _utc_now_epoch()

    # Prefer explicit ``claimed_at`` epoch; fall back to ISO ``claimed_at_iso``.
    claimed_at = data.get("claimed_at")
    parsed = None
    if isinstance(claimed_at, (int, float)):
        parsed = float(claimed_at)
    elif isinstance(claimed_at, str) and claimed_at.strip().isdigit():
        try:
            parsed = float(claimed_at)
        except ValueError:
            parsed = None
    if parsed is None:
        iso = _parse_iso(data.get("claimed_at_iso") or data.get("claimed_at"))
        if iso is not None:
            parsed = iso.timestamp()

    if parsed is None:
        # also tolerate a heartbeat timestamp
        iso = _parse_iso(data.get("heartbeat_at"))
        if iso is not None:
            parsed = iso.timestamp()

    if parsed is None:
        # malformed lease → treat as stale so callers can reclaim
        return True
    age = now - parsed
    return age > max(0, int(max_age_s))


def release_lease(lease_path: pathlib.Path | str) -> None:
    """Delete the lease file (best-effort — missing file is not an error)."""
    path = pathlib.Path(lease_path)
    try:
        path.unlink()
    except FileNotFoundError:
        return
    except OSError as exc:
        logger.info("scheduler: could not release lease %s: %s", path, exc)


def claim_lease(
    work_item_id: str,
    run_id: str,
    lease_path: pathlib.Path | str,
    *,
    max_age_s: int = DEFAULT_LEASE_MAX_AGE_S,
) -> bool:
    """Atomically claim a lease for ``(work_item_id, run_id)``.

    Returns True when the lease was claimed (newly written or stolen
    from a stale prior holder). Returns False when a non-stale lease
    already exists for *some* work item.

    The file format is:

    .. code-block:: json

        {
          "protocol_version": "loom.dev/v1",
          "work_item_id": "...",
          "run_id": "...",
          "claimed_at": 1700000000.123,
          "claimed_at_iso": "2026-01-01T00:00:00+00:00"
        }
    """
    wid = _ensure_str(work_item_id, "work_item_id")
    rid = _ensure_str(run_id, "run_id")
    path = pathlib.Path(lease_path)

    with _LEASE_LOCK:
        existing = _read_lease_dict(path)
        if existing and not is_lease_stale(path, max_age_s=max_age_s):
            existing_wid = str(existing.get("work_item_id", "")).strip()
            existing_rid = str(existing.get("run_id", "")).strip()
            if existing_wid == wid and existing_rid == rid:
                # idempotent re-claim by the same owner: refresh timestamp
                payload = _build_lease_payload(wid, rid)
                _atomic_write_json(path, payload)
                return True
            logger.info(
                "scheduler: claim refused; lease held by wid=%s run=%s",
                existing_wid,
                existing_rid,
            )
            return False

        payload = _build_lease_payload(wid, rid)
        try:
            _atomic_write_json(path, payload)
        except OSError as exc:
            logger.warning("scheduler: claim failed to write %s: %s", path, exc)
            return False
        return True


def _build_lease_payload(work_item_id: str, run_id: str) -> dict:
    now = _utc_now_epoch()
    return {
        "protocol_version": PROTOCOL_VERSION,
        "work_item_id": work_item_id,
        "run_id": run_id,
        "claimed_at": now,
        "claimed_at_iso": datetime.fromtimestamp(now, tz=timezone.utc).isoformat(),
        "lease_id": f"lease-{uuid.uuid4().hex[:12]}",
    }


# ----------------------------------------------------------------------------
# Core selection logic
# ----------------------------------------------------------------------------
def _priority_of(item: dict) -> int:
    raw = str(item.get("priority", "")).strip().lower()
    return PRIORITY_RANK.get(raw, PRIORITY_RANK["medium"])


def _status_of(item: dict) -> str:
    return str(item.get("status", "")).strip().lower()


def _deps_of(item: dict) -> list[str]:
    deps = item.get("deps", [])
    if isinstance(deps, list):
        return [str(d).strip() for d in deps if str(d).strip()]
    if isinstance(deps, str):
        return [d.strip() for d in re.split(r"[,\s]+", deps) if d.strip()]
    return []


def _blocked_by(item: dict, by_id: dict[str, dict]) -> list[str]:
    out: list[str] = []
    for dep in _deps_of(item):
        other = by_id.get(dep)
        if not other:
            # unknown dep is treated as not-yet-done so we don't silently pass it
            out.append(dep)
            continue
        if _status_of(other) not in DEP_SATISFIED_STATUSES:
            out.append(dep)
    return out


def _pending_items(items: Iterable[dict]) -> list[dict]:
    return [item for item in items if _status_of(item) == "pending"]


def _lease_held_work_item(lease_path: pathlib.Path | None) -> str:
    if lease_path is None:
        return ""
    data = _read_lease_dict(pathlib.Path(lease_path))
    if not data:
        return ""
    if is_lease_stale(lease_path):
        return ""
    return str(data.get("work_item_id", "")).strip()


def _sort_key(item: dict) -> tuple[int, str]:
    return (_priority_of(item), str(item.get("id", "")).strip())


def select_next_pending(
    backlog_path: pathlib.Path | str,
    lease_path: pathlib.Path | str | None = None,
    *,
    budget_cap_usd: float | None = None,
    now_epoch: float | None = None,
) -> ScheduleDecision | None:
    """Return the next runnable task as a :class:`ScheduleDecision`.

    Algorithm (kept simple so it's audit-friendly):

    1. Load backlog from ``backlog_path`` (tolerates missing / malformed).
    2. Filter ``status == "pending"``.
    3. Reject any task whose deps are not in ``DEP_SATISFIED_STATUSES``.
    4. Reject any task whose estimated cost > budget cap (default
       :data:`DEFAULT_BUDGET_CAP_USD`).
    5. Reject any task whose ``id`` equals the leased work item in
       ``lease_path`` (when provided).
    6. Sort by priority (``high`` > ``medium`` > ``low``), then by id,
       and return the first one as a ``ready`` decision.

    Returns ``None`` when no row passes all filters.
    """
    bp = _ensure_path(backlog_path, "backlog_path")
    items = _load_backlog_items(bp)
    if not items:
        return None

    cap = (
        float(budget_cap_usd)
        if budget_cap_usd is not None
        else DEFAULT_BUDGET_CAP_USD
    )

    by_id = _index_by_id(items)
    leased_wid = _lease_held_work_item(lease_path) if lease_path is not None else ""

    pending = _pending_items(items)
    ready: list[dict] = []
    for item in pending:
        wid = str(item.get("id", "")).strip()
        if not wid:
            continue
        blocked = _blocked_by(item, by_id)
        if blocked:
            continue
        if leased_wid and leased_wid == wid:
            continue
        if not _within_budget(item, cap):
            continue
        ready.append(item)

    if not ready:
        return None

    ready.sort(key=_sort_key)
    picked = ready[0]
    wid = str(picked.get("id", "")).strip()
    return ScheduleDecision(
        work_item_id=wid,
        reason="ready",
        blocked_by=[],
        estimated_cost_usd=_estimate_cost_usd(picked),
        priority=str(picked.get("priority", "medium")).strip().lower() or "medium",
    )


def list_blocked(
    backlog_path: pathlib.Path | str,
    lease_path: pathlib.Path | str | None = None,
    *,
    budget_cap_usd: float | None = None,
) -> list[ScheduleDecision]:
    """Return one :class:`ScheduleDecision` per blocked pending task.

    Reasons emitted:

    * ``"blocked"`` — task has deps that are not yet ``done``/``skipped``.
    * ``"leased"`` — task is currently held by another run.
    * ``"over_budget"`` — task's estimated cost exceeds the cap.

    Tasks that pass every filter are NOT returned here — they belong in
    :func:`select_next_pending`.
    """
    bp = _ensure_path(backlog_path, "backlog_path")
    items = _load_backlog_items(bp)
    if not items:
        return []

    cap = (
        float(budget_cap_usd)
        if budget_cap_usd is not None
        else DEFAULT_BUDGET_CAP_USD
    )

    by_id = _index_by_id(items)
    leased_wid = _lease_held_work_item(lease_path) if lease_path is not None else ""

    out: list[ScheduleDecision] = []
    for item in _pending_items(items):
        wid = str(item.get("id", "")).strip()
        if not wid:
            continue
        blocked = _blocked_by(item, by_id)
        if blocked:
            out.append(
                ScheduleDecision(
                    work_item_id=wid,
                    reason="blocked",
                    blocked_by=blocked,
                    estimated_cost_usd=_estimate_cost_usd(item),
                    priority=str(item.get("priority", "medium")).strip().lower() or "medium",
                )
            )
            continue
        if leased_wid and leased_wid == wid:
            out.append(
                ScheduleDecision(
                    work_item_id=wid,
                    reason="leased",
                    blocked_by=[f"lease:{leased_wid}"],
                    estimated_cost_usd=_estimate_cost_usd(item),
                    priority=str(item.get("priority", "medium")).strip().lower() or "medium",
                )
            )
            continue
        if not _within_budget(item, cap):
            out.append(
                ScheduleDecision(
                    work_item_id=wid,
                    reason="over_budget",
                    blocked_by=[f"budget_cap_usd:{cap:.4f}"],
                    estimated_cost_usd=_estimate_cost_usd(item),
                    priority=str(item.get("priority", "medium")).strip().lower() or "medium",
                )
            )
            continue
    # sort for stable output: priority then id
    out.sort(key=lambda d: (PRIORITY_RANK.get(d.priority, 1), d.work_item_id))
    return out


# ----------------------------------------------------------------------------
# Convenience: a single-call "decide" wrapper that mirrors what the
# future devkit auto integration will need.
# ----------------------------------------------------------------------------
def decide(
    backlog_path: pathlib.Path | str | None = None,
    lease_path: pathlib.Path | str | None = None,
    *,
    run_id: str = "",
    budget_cap_usd: float | None = None,
    claim: bool = False,
) -> dict:
    """Decide what to do next. Returns a dict with ``decision`` + ``blocked``.

    Convenience wrapper for callers that want a single function call.

    When ``claim=True`` and a ready decision is found, also atomically
    claims the lease for ``run_id``. Returns ``{"decision": <Decision>,
    "blocked": [<Decision>, ...], "claimed": bool}``.
    """
    bp = pathlib.Path(backlog_path) if backlog_path is not None else DEFAULT_BACKLOG_PATH
    lp = pathlib.Path(lease_path) if lease_path is not None else DEFAULT_LEASE_PATH

    decision = select_next_pending(bp, lp, budget_cap_usd=budget_cap_usd)
    blocked = list_blocked(bp, lp, budget_cap_usd=budget_cap_usd)

    claimed = False
    if claim and decision is not None and decision.is_actionable():
        rid = (run_id or f"sched-{uuid.uuid4().hex[:8]}").strip()
        claimed = claim_lease(decision.work_item_id, rid, lp)

    return {
        "decision": decision,
        "blocked": blocked,
        "claimed": claimed,
    }


__all__ = [
    # dataclass
    "ScheduleDecision",
    # constants
    "PROTOCOL_VERSION",
    "VALID_PRIORITIES",
    "PRIORITY_RANK",
    "DEP_SATISFIED_STATUSES",
    "DEFAULT_LEASE_MAX_AGE_S",
    "DEFAULT_BUDGET_CAP_USD",
    "DEFAULT_BACKLOG_PATH",
    "DEFAULT_LEASE_PATH",
    # API
    "select_next_pending",
    "list_blocked",
    "claim_lease",
    "release_lease",
    "is_lease_stale",
    "decide",
]