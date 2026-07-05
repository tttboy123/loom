# Evidence-grounded execution

> No claim without provenance. Source:
> `model-self-improvement-and-boundary-awareness.md` §3.3, §8.2.

## The problem this skill fixes

Most agent demos and many production agents report progress with
unsupported claims:

- "I've finished this task." — supported by what evidence?
- "Tests pass." — which tests, on which code version?
- "This is the right approach." — compared to what alternatives?

Without grounding, the cockpit and the gate see only the producer's
self-assessment, which is correlated with the producer's biases.

## Claim classification

Every claim that reaches the cockpit, the gate, or the user gets a
status tag:

```yaml
claim:
  text: "tests/test_T2.py passes"
  status: verified                   # verified | inferred | unverified
  evidence:
    - "evidence/T2/outputs/pytest.stdout"
  reproducible:
    command: "pytest tests/test_T2.py"
    expected: "all tests pass"
  emitted_at: 2026-07-06T12:30:00+08:00
```

| Status | Meaning | When to use |
|---|---|---|
| `verified` | Supported by direct evidence from this run (tool result, file content, command output) | When the producer can produce evidence |
| `inferred` | Reasonable but not directly checked in this run | When verification would be expensive but the inference is well-grounded in evidence from related runs |
| `unverified` | Should not be presented as done | Mark this when reporting partial work, failure, or out-of-scope items |

A claim tagged `unverified` is allowed (and required) for honest
reports — "we did NOT verify X because of cost; flag for future
verification" is better than "X passes" without evidence.

## Producer-side workflow

Before reporting progress to the gate or user, the producer must:

1. Run the producer's `verification.commands` from the HandoffEnvelope.
2. Capture outputs in `evidence/<task_id>/outputs/`.
3. Tag each claim with `verified | inferred | unverified`.
4. Submit via HandoffEnvelope.

The gate reads the tags — a `verified` claim without an
`evidence_refs` is a hard rejection.

## Reviewer-side workflow

The reviewer:

1. Re-runs the producer's commands in a fresh sandbox (see
   `agentteam-runtime` skill's `evidence-flow.md`).
2. Confirms the producer's `verified` claims match the re-run.
3. Marks the re-run verdict in the gate report.
4. Adds producer's `inferred` claims to its own review queue —
   the reviewer may upgrade them to `verified` (after running) or
   downgrade to `unverified` (if it disagrees).

## No "looks done" verdicts

Allowed gate verdicts:

- `pass` — all required evidence is `verified` and matches acceptance.
- `partial` — some `verified`, some `inferred`; requires explicit
  reviewer acceptance; recorded as residual risk.
- `repair_requested` — at least one required claim is `unverified`
  or evidence insufficient.
- `fail` — producer's verification commands failed.
- `human_required` — risk ∈ {high, critical} requires human.

Not allowed: `done` (use `pass` instead) or `looks_done` (rejected
by lint).

## Cross-reference

- Same principle in Loom Blueprint Phase 3 ("Missing evidence
  becomes `request_changes` or `blocked`, not `done`").
- Companion control-theory doc §9 has a `false_done_rate` failure
  mode entry — measured as `claims without evidence_path / total_claims`.
