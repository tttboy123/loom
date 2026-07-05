"""devkit/failure_codes.py — vocabulary translator across the three Loom generations.

Three failure-code vocabularies coexist today in the Loom codebase:

* **Phase B** — free-string status codes emitted by
  :func:`devkit.rdloop._resolve_gate_status` (e.g. ``"tests_failed"``,
  ``"over_budget"``, ``"blocked"``). These are the *legacy* status strings
  rdloop returns to callers from ``evaluate_final_gate``.
* **Phase D** — uppercase enum values from ``gate_verdict.schema.json``,
  produced by :class:`devkit.gatekeeper.GateVerdict.failure_codes`
  (e.g. ``TEST_REGRESSION``, ``BUDGET_EXCEEDED``, ``EVIDENCE_MISSING``,
  ``EVIDENCE_INVALID``, ``SCHEMA_VALIDATION_ERROR``).
* **Phase A** — uppercase repairer codes produced by
  :mod:`devkit.repairer` and :mod:`devkit.state_writer` (e.g.
  ``SCHEMA_VALIDATION_ERROR``, ``BUDGET_EXCEEDED``, ``NOT_ON_WHITELIST``,
  ``MISSING_WORK_ITEM_ID``, ``TASK_NOT_FOUND``). The repairer only acts on
  *Incidents*, not on gate failures, so the Phase A side is partly
  *synthesized* — there is no 1:1 between Phase D and Phase A in general.

This module is the single seam for converting between them. It exposes
plain functions and ``dict``-style reverse lookup tables, so callers that
already maintain their own dict can still re-use them. No I/O, no
side-effects — stdlib + dataclasses only.

Mapping table (canonical, mirrored in :data:`PHASE_B_TO_PHASE_D` and
:data:`PHASE_D_TO_PHASE_A`):

===========  ===================  ===================================  =====================================================
Phase B      Phase D              Phase A (single)                     Phase A (all candidates)
===========  ===================  ===================================  =====================================================
tests_failed TEST_REGRESSION      SCHEMA_VALIDATION_ERROR              SCHEMA_VALIDATION_ERROR, INVALID_TASK_SPEC
over_budget  BUDGET_EXCEEDED      BUDGET_EXCEEDED                      BUDGET_EXCEEDED
blocked      EVIDENCE_INVALID     SCHEMA_VALIDATION_ERROR              SCHEMA_VALIDATION_ERROR, INVALID_TASK_SPEC
review_requested_changes  (none)  (none)                               (none)
review_request_changes    (none)  (none)                               (none)  (legacy alias of review_requested_changes)
review_timeout            (none)  (none)                               (none)
suggested_go              (none)  (none)                               (none)
task_contract_blocked     EVIDENCE_INVALID  SCHEMA_VALIDATION_ERROR   SCHEMA_VALIDATION_ERROR, INVALID_TASK_SPEC
blocked_no_detail         (none)  (none)                               (none)  (legacy placeholder)
no_detail                 (none)  (none)                               (none)  (legacy placeholder)
===========  ===================  ===================================  =====================================================

===========  ===================  ===================================  =====================================================
Phase D                       Phase A (single)                       Phase A (all candidates)
===========  ===================  ===================================  =====================================================
TEST_REGRESSION               SCHEMA_VALIDATION_ERROR                SCHEMA_VALIDATION_ERROR, INVALID_TASK_SPEC
BUDGET_EXCEEDED               BUDGET_EXCEEDED                        BUDGET_EXCEEDED
EVIDENCE_MISSING              (none)                                 (none)
EVIDENCE_INVALID              SCHEMA_VALIDATION_ERROR                SCHEMA_VALIDATION_ERROR, INVALID_TASK_SPEC
SCHEMA_VALIDATION_ERROR       SCHEMA_VALIDATION_ERROR                SCHEMA_VALIDATION_ERROR
===========  ===================  ===================================  =====================================================

Irreversibility notes:

* Phase B → Phase D is **lossy**: ``review_requested_changes``,
  ``review_request_changes``, ``review_timeout`` and the legacy
  ``blocked_no_detail`` / ``no_detail`` placeholders have no Phase D
  equivalent because Phase D is gate-evidence-centric and has no concept
  of a review verdict.
* Phase D → Phase A is **partially lossy**: ``EVIDENCE_MISSING`` has no
  repairer equivalent (missing evidence is not an incident the repairer
  can fix). All other Phase D codes map to at least one Phase A code.
* Phase B → Phase A (via Phase D) is **lossy**: a Phase B
  ``review_requested_changes`` cannot reach Phase A because it cannot
  pass through Phase D.

Phase D → Phase B (the reverse direction) is **partial**: only
``TEST_REGRESSION``, ``BUDGET_EXCEEDED`` and ``EVIDENCE_INVALID`` have
canonical Phase B equivalents. ``EVIDENCE_MISSING`` and
``SCHEMA_VALIDATION_ERROR`` have no Phase B counterpart. Use
:func:`phase_d_to_phase_b` and fall back to the original enum on
``None``.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Iterable


# ----------------------------------------------------------------------------
# Phase D — gate_verdict schema enum (the "new" typed vocabulary)
# ----------------------------------------------------------------------------
PHASE_D_CODES: frozenset[str] = frozenset({
    "EVIDENCE_MISSING",
    "TEST_REGRESSION",
    "BUDGET_EXCEEDED",
    "EVIDENCE_INVALID",
    "SCHEMA_VALIDATION_ERROR",
})


# ----------------------------------------------------------------------------
# Phase B — rdloop._resolve_gate_status free strings (the "legacy" vocabulary)
# ----------------------------------------------------------------------------
PHASE_B_REASONS: frozenset[str] = frozenset({
    "tests_failed",
    "over_budget",
    "blocked",
    "review_timeout",
    "review_request_changes",        # returned by _resolve_gate_status
    "review_requested_changes",      # legacy alias used by callers
    "suggested_go",                  # success — no failure code
    "task_contract_blocked",         # early-return in rdloop.run_loop
    # Legacy placeholders kept for forward-compat with callers that may
    # produce them; they have no Phase D equivalent (no evidence was read).
    "blocked_no_detail",
    "no_detail",
})


# ----------------------------------------------------------------------------
# Phase A — repairer / state_writer failure codes (the "oldest" vocabulary)
# ----------------------------------------------------------------------------
PHASE_A_REPAIRER_CODES: frozenset[str] = frozenset({
    # repairer.py — _reject / dispatch / _invoke_action
    "INVALID_TASK_SPEC",
    "TASK_ID_REQUIRED",
    "INVALID_DURATION",
    "INVALID_INCIDENT",
    "SCHEMA_VALIDATION_ERROR",
    "NOT_ON_WHITELIST",
    "MISSING_WORK_ITEM_ID",
    "MISSING_LEASE_ID",
    "MISSING_TASK_PAYLOAD",
    "MISSING_CARRIER_OR_SCOPE",
    "UNKNOWN_ACTION",
    # state_writer.py — transition_task
    "TASK_NOT_FOUND",
    "TASK_ALREADY_EXISTS",
    "INVALID_STATUS",
    "TRANSITION_NOT_ALLOWED",
    "DIRECT_FINAL_STATUS_WRITE_BLOCKED",
    "STATUS_PATCH_FORBIDDEN",
    "BUDGET_EXCEEDED",
})


# ----------------------------------------------------------------------------
# Phase B → Phase D — single-best mapping (or None when no good mapping)
# ----------------------------------------------------------------------------
PHASE_B_TO_PHASE_D: dict[str, str | None] = {
    "tests_failed":              "TEST_REGRESSION",
    "over_budget":               "BUDGET_EXCEEDED",
    "blocked":                   "EVIDENCE_INVALID",
    "review_timeout":            None,    # no Phase D equivalent
    "review_request_changes":    None,    # no Phase D equivalent
    "review_requested_changes":  None,    # legacy alias; no Phase D equivalent
    "suggested_go":              None,    # success — no failure code
    "task_contract_blocked":     "EVIDENCE_INVALID",
    "blocked_no_detail":         None,    # legacy placeholder
    "no_detail":                 None,    # legacy placeholder
}


# ----------------------------------------------------------------------------
# Phase D → Phase A — single-best mapping (or None when no good mapping)
# ----------------------------------------------------------------------------
PHASE_D_TO_PHASE_A: dict[str, str | None] = {
    "TEST_REGRESSION":         "SCHEMA_VALIDATION_ERROR",
    "BUDGET_EXCEEDED":         "BUDGET_EXCEEDED",
    "EVIDENCE_MISSING":        None,    # missing evidence is not a repairer incident
    "EVIDENCE_INVALID":        "SCHEMA_VALIDATION_ERROR",
    "SCHEMA_VALIDATION_ERROR": "SCHEMA_VALIDATION_ERROR",
}


# ----------------------------------------------------------------------------
# Phase D → Phase A — *all* candidate repairer codes (some Phase D codes
# plausibly correspond to more than one repairer code; this returns them
# all so callers can pick). Empty list means no mapping.
# ----------------------------------------------------------------------------
ALL_PHASE_A_FOR_PHASE_D: dict[str, tuple[str, ...]] = {
    "TEST_REGRESSION":         ("SCHEMA_VALIDATION_ERROR", "INVALID_TASK_SPEC"),
    "BUDGET_EXCEEDED":         ("BUDGET_EXCEEDED",),
    "EVIDENCE_MISSING":        (),       # no repairer equivalent
    "EVIDENCE_INVALID":        ("SCHEMA_VALIDATION_ERROR", "INVALID_TASK_SPEC"),
    "SCHEMA_VALIDATION_ERROR": ("SCHEMA_VALIDATION_ERROR",),
}


# ----------------------------------------------------------------------------
# Phase D → Phase B — reverse mapping (the complement to PHASE_B_TO_PHASE_D).
# Only one Phase B reason is the canonical inverse of each Phase D enum;
# callers that need richer reverse coverage should fall back to the original
# enum string. Unlike PHASE_B_TO_PHASE_D, this mapping is partial:
# ``EVIDENCE_MISSING`` has no Phase B equivalent (it pre-dates the
# ``_resolve_gate_status`` vocabulary) and ``SCHEMA_VALIDATION_ERROR`` is
# not emitted by Phase B at all.
# ----------------------------------------------------------------------------
PHASE_D_TO_PHASE_B: dict[str, str] = {
    "TEST_REGRESSION":   "tests_failed",
    "BUDGET_EXCEEDED":   "over_budget",
    "EVIDENCE_INVALID":  "blocked",
}


# ----------------------------------------------------------------------------
# Translation functions — thin wrappers over the dicts above, with
# defensive validation (unknown inputs return None, no exception).
# ----------------------------------------------------------------------------
def phase_b_to_phase_d(reason: str) -> str | None:
    """Translate a Phase B free-string reason to its Phase D enum.

    Returns ``None`` if the reason has no Phase D equivalent (e.g.
    ``review_timeout`` is review-only and Phase D has no review concept)
    or if the reason is unknown to this module (forward-compat: unknown
    reasons return None instead of raising).
    """
    if not isinstance(reason, str):
        return None
    return PHASE_B_TO_PHASE_D.get(reason)


def phase_d_to_phase_a(enum: str) -> str | None:
    """Translate a Phase D enum to its single-best Phase A repairer code.

    Returns ``None`` if the Phase D code has no Phase A equivalent (e.g.
    ``EVIDENCE_MISSING`` is not a repairer-handleable incident).
    """
    if not isinstance(enum, str):
        return None
    return PHASE_D_TO_PHASE_A.get(enum)


def phase_d_to_phase_b(enum: str) -> str | None:
    """Translate a Phase D enum to its canonical Phase B free-string reason.

    Returns ``None`` when the Phase D code has no Phase B equivalent
    (``EVIDENCE_MISSING`` pre-dates ``_resolve_gate_status``;
    ``SCHEMA_VALIDATION_ERROR`` is not a status that rdloop emits). For
    Phase D codes listed in :data:`PHASE_D_TO_PHASE_B` we return the
    single canonical Phase B reason that ``_resolve_gate_status`` would
    produce for the same underlying condition.

    Tolerates unknown / non-string input by returning ``None`` — callers
    that need richer reverse coverage should fall back to the original
    enum string when this function returns ``None``.
    """
    if not isinstance(enum, str):
        return None
    return PHASE_D_TO_PHASE_B.get(enum)


def phase_b_to_phase_a(reason: str) -> str | None:
    """Compose: Phase B reason → Phase D enum → Phase A repairer code.

    Returns ``None`` if any hop in the chain returns ``None``.
    """
    d = phase_b_to_phase_d(reason)
    if d is None:
        return None
    return phase_d_to_phase_a(d)


def all_phase_a_for_phase_d(enum: str) -> list[str]:
    """Return *all* plausible Phase A codes for a given Phase D enum.

    Returns ``[]`` (empty list) when the Phase D code has no Phase A
    equivalent. Does not raise on unknown input — returns ``[]`` so the
    caller can iterate safely.
    """
    if not isinstance(enum, str):
        return []
    return list(ALL_PHASE_A_FOR_PHASE_D.get(enum, ()))


def all_phase_a_for_phase_b(reason: str) -> list[str]:
    """Return all plausible Phase A codes reachable from a Phase B reason.

    Composes through Phase D: each Phase D candidate produces a list of
    Phase A candidates. Duplicates are removed (order preserved).
    Returns ``[]`` when no chain reaches a Phase A code.
    """
    if not isinstance(reason, str):
        return []
    d = phase_b_to_phase_d(reason)
    if d is None:
        return []
    out: list[str] = []
    seen: set[str] = set()
    for code in all_phase_a_for_phase_d(d):
        if code in seen:
            continue
        seen.add(code)
        out.append(code)
    return out


# ----------------------------------------------------------------------------
# Bulk helpers — useful when emitting log lines or reports.
# ----------------------------------------------------------------------------
def translate_phase_b_to_phase_d(reasons: Iterable[str]) -> list[str | None]:
    """Vectorised form of :func:`phase_b_to_phase_d` (preserves order)."""
    return [phase_b_to_phase_d(r) for r in reasons]


def translate_phase_d_to_phase_a(enums: Iterable[str]) -> list[str | None]:
    """Vectorised form of :func:`phase_d_to_phase_a` (preserves order)."""
    return [phase_d_to_phase_a(e) for e in enums]


# ----------------------------------------------------------------------------
# Full-chain translator — useful for logging / introspection.
# ----------------------------------------------------------------------------
@dataclass(frozen=True)
class TranslationResult:
    """The full chain result for a single Phase B reason.

    Every field is exposed so callers can render the chain however they
    like (e.g. ``"blocked" → EVIDENCE_INVALID → SCHEMA_VALIDATION_ERROR``).
    """

    reason: str
    phase_d: str | None
    phase_a: str | None
    phase_a_candidates: tuple[str, ...] = field(default_factory=tuple)

    def to_dict(self) -> dict:
        return {
            "reason": self.reason,
            "phase_d": self.phase_d,
            "phase_a": self.phase_a,
            "phase_a_candidates": list(self.phase_a_candidates),
        }

    def is_actionable(self) -> bool:
        """True if the chain reached at least one Phase A repairer code."""
        return self.phase_a is not None or bool(self.phase_a_candidates)

    def render(self) -> str:
        """Human-friendly rendering (e.g. for log lines)."""
        return f"{self.reason!r} → {self.phase_d!r} → {self.phase_a!r}"


def translate_chain(reason: str) -> TranslationResult:
    """Run the full Phase B → Phase D → Phase A chain for ``reason``.

    Returns a :class:`TranslationResult` with every hop populated (or
    ``None`` where the chain breaks). Unknown reasons are tolerated —
    they yield a ``TranslationResult`` whose ``phase_d`` and ``phase_a``
    are ``None`` instead of raising.
    """
    if not isinstance(reason, str):
        return TranslationResult(reason=str(reason), phase_d=None, phase_a=None)

    d = phase_b_to_phase_d(reason)
    a: str | None = None
    candidates: tuple[str, ...] = ()
    if d is not None:
        a = phase_d_to_phase_a(d)
        candidates = tuple(all_phase_a_for_phase_d(d))
    return TranslationResult(
        reason=reason,
        phase_d=d,
        phase_a=a,
        phase_a_candidates=candidates,
    )


__all__ = [
    # constants
    "PHASE_D_CODES",
    "PHASE_B_REASONS",
    "PHASE_A_REPAIRER_CODES",
    # mapping tables
    "PHASE_B_TO_PHASE_D",
    "PHASE_D_TO_PHASE_A",
    "PHASE_D_TO_PHASE_B",
    "ALL_PHASE_A_FOR_PHASE_D",
    # single-hop translators
    "phase_b_to_phase_d",
    "phase_d_to_phase_a",
    "phase_d_to_phase_b",
    # composed / multi translators
    "phase_b_to_phase_a",
    "all_phase_a_for_phase_d",
    "all_phase_a_for_phase_b",
    # bulk helpers
    "translate_phase_b_to_phase_d",
    "translate_phase_d_to_phase_a",
    # chain translator
    "translate_chain",
    "TranslationResult",
]  