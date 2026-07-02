"""Structured failure classification for external-runner result packets.

The functions here are pure mapping helpers. They do not decide whether a run
is acceptable, choose a carrier, inspect a repo, or mutate any packet emitter.
"""
from __future__ import annotations

FAILURE_KINDS = (
    "candidate_topic_drift",
    "missing_contracted_outputs",
    "verification_failed_authoritative_surface",
    "review_rejected",
    "empty_or_non_actionable_model_output",
)

NEXT_ACTION_HINTS = (
    "retry_immediate",
    "retry_different_carrier",
    "skip_candidate_reopen_task",
    "cool_down",
)

_DEFAULT_HINT_BY_KIND = {
    "candidate_topic_drift": "skip_candidate_reopen_task",
    "missing_contracted_outputs": "retry_different_carrier",
    "verification_failed_authoritative_surface": "cool_down",
    "review_rejected": "skip_candidate_reopen_task",
    "empty_or_non_actionable_model_output": "retry_immediate",
}

_OUTCOME_TAG_TO_KIND = {
    "candidate_topic_drift": "candidate_topic_drift",
    "topic_drift": "candidate_topic_drift",
    "wrong_candidate": "candidate_topic_drift",
    "wrong_patch_surface": "candidate_topic_drift",
    "missing_contracted_outputs": "missing_contracted_outputs",
    "missing_required_outputs": "missing_contracted_outputs",
    "apply_not_applied": "missing_contracted_outputs",
    "not_applied": "missing_contracted_outputs",
    "apply_partial": "missing_contracted_outputs",
    "verification_failed_authoritative_surface": "verification_failed_authoritative_surface",
    "verify_failed": "verification_failed_authoritative_surface",
    "verification_failed": "verification_failed_authoritative_surface",
    "authoritative_verify_failed": "verification_failed_authoritative_surface",
    "review_rejected": "review_rejected",
    "request_changes": "review_rejected",
    "no_go_review": "review_rejected",
    "empty_or_non_actionable_model_output": "empty_or_non_actionable_model_output",
    "empty_output": "empty_or_non_actionable_model_output",
    "non_actionable_output": "empty_or_non_actionable_model_output",
    "model_empty": "empty_or_non_actionable_model_output",
}

_FALLBACK_KIND = "empty_or_non_actionable_model_output"
_FALLBACK_HINT = "retry_different_carrier"


def _norm(value: str | None) -> str:
    return str(value or "").strip().lower().replace("-", "_").replace(" ", "_")


def normalize_failure_kind(kind: str) -> str:
    normalized = _norm(kind)
    if normalized not in FAILURE_KINDS:
        raise ValueError(f"unknown failure_kind: {kind}")
    return normalized


def normalize_next_action_hint(hint: str) -> str:
    normalized = _norm(hint)
    if normalized not in NEXT_ACTION_HINTS:
        raise ValueError(f"unknown next_action_hint: {hint}")
    return normalized


def default_hint_for_failure_kind(kind: str) -> str:
    return _DEFAULT_HINT_BY_KIND[normalize_failure_kind(kind)]


def classify_outcome_tag(outcome_tag: str) -> dict:
    """Map a non-GO outcome tag to an advisory failure classification."""
    source = _norm(outcome_tag)
    failure_kind = _OUTCOME_TAG_TO_KIND.get(source)
    fallback = failure_kind is None
    if fallback:
        failure_kind = _FALLBACK_KIND
        hint = _FALLBACK_HINT
    else:
        hint = default_hint_for_failure_kind(failure_kind)
    return {
        "source_outcome_tag": source,
        "failure_kind": failure_kind,
        "next_action_hint": hint,
        "advisory": True,
        "fallback": fallback,
    }
