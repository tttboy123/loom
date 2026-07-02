# Request 005: External Candidate Drift Gate And Structured Failure Classification

status: pending_scan  
owner: Buddys  
target: Loom upstream  
type: generalized capability request, not direct patch

## 1. Problem

When Loom is used as an external execution engine for a long-running autonomy loop, a run can fail in very different ways, but the caller may only receive a generic non-GO outcome.

Observed generic failure classes:

1. the implement candidate is on the wrong topic entirely
2. the candidate never materializes the contracted outputs
3. verification fails on the real repo surface
4. review rejects the candidate even though build/test passed
5. the model returns empty or non-actionable content

Without structured failure classes, external callers cannot safely decide whether to:

- retry immediately
- switch carrier
- skip verify/review
- cool down the task
- reopen another task instead

## 2. Why this should be upstream

This is not specific to Buddys:

- any external queue runner can hit topic-drift or wrong-candidate outputs
- generic `request_changes` is too weak for automated retry policy
- long-running loops need to distinguish "bad candidate" from "real product bug"

## 3. Requested capability surface

### A. Candidate drift gate

Loom should expose a generic way to mark a run as off-topic before full downstream verification when the candidate clearly misses the contracted target, for example:

- wrong file targets
- wrong topic tokens
- wrong patch surface
- unrelated deliverable shape

### B. Structured failure kinds

Loom should emit machine-readable failure kinds such as:

- `candidate_topic_drift`
- `missing_contracted_outputs`
- `verification_failed_authoritative_surface`
- `review_rejected`
- `empty_or_non_actionable_model_output`

### C. Retry / cooldown hints

Result packets should help external callers decide whether the next action should be:

- immediate retry
- retry with different carrier
- skip current candidate and reopen task
- cool down as a genuine product failure

## 4. Non-goals

This request does not ask Loom to:

- hardcode Buddys docs, tasks, or repo layout
- hardcode Buddys model choices
- replace downstream acceptance logic

## 5. Acceptance criteria for Loom intake

Loom should only take this request if it can aim for:

1. machine-readable candidate drift detection hooks or output classes
2. structured failure classes beyond a single generic non-GO result
3. retry/cooldown signals useful to generic external queue runners

## 6. Buddys evidence behind this request

This generalized request is grounded in repeated external-project observations:

- implement candidates could drift to unrelated topics while still producing syntactically valid stage output
- verify/review could then burn a full loop on the wrong candidate
- the resident queue would appear stalled because generic cooldown treated drift like a product bug

These are evidence inputs only. Loom should solve the general external-runner classification problem, not copy Buddys-specific wrappers.
