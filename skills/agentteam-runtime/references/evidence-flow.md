# Evidence flow

> How EvidencePackets are produced and consumed by gates. Source:
> `loom-agent-team-runtime-proposal.md` §2 Codex strengths + existing
> Loom Blueprint Phase 3.

## What an EvidencePacket looks like (Loom-native)

Loom already defines `EvidencePacket` in
`devkit/protocol_schemas/evidence_packet.schema.json`. The skill does
not redefine it; it specifies how a team run produces and consumes it.

```yaml
work_item_id: T2
source: materialized_repo        # inner_sandbox | materialized_repo | unknown
commands:
  - cmd: "pytest tests/test_T2.py"
    exit_code: 0
    stdout_ref: "evidence/T2/outputs/pytest.stdout"
evidence_paths:
  - "evidence/T2/outputs/T2_result.json"
verdict: pass                     # pass | partial | fail | unknown
confidence: medium
limitations:
  - "Covered only the happy path; edge cases not exercised"
```

## Producer-side (builder role)

When the producer finishes a task, the handoff envelope's
`verification.evidence_refs` must point at an existing
EvidencePacket under `evidence/<task_id>/`. The producer is expected
to:

1. Run all commands listed in the team's GatePolicy under
   `required_evidence.<artifact_class>`.
2. Capture stdout / stderr / produced artifacts.
3. Emit one EvidencePacket per task with full reproducibility data.

## Reviewer-side (reviewer role)

The reviewer's job is **not** to re-derive the result. The reviewer:

1. Re-runs the producer's commands in a fresh sandbox.
2. Confirms the EvidencePacket fields match the re-run output.
3. Records verdict in the gate report (`.loom/runs/<id>/gates/<gate>.md`).
4. Adds findings to the RunTrace; these later become inputs to the
   improvement candidate generator.

## Tiered evidence quality

This is the same source-classification Loom Blueprint Phase 3
defines. The skill aligns with it:

| Source | Meaning | Trust |
|---|---|---|
| `inner_sandbox` | Evidence produced inside a disposable sandbox only | lowest |
| `materialized_repo` | Evidence produced against the materialized repo after producer's write is applied | highest |
| `unknown` | Source cannot be classified | medium, must escalate |

A gate on `inner_sandbox`-only evidence **must escalate to a
human-in-loop or refuse to mark Done**. The
`loom-control-theory-extension-v2-2026-07-05.md` doc captures this
as one of the failure modes (Section 9 of that doc).

## When evidence is missing or weak

The gate refuses Done. The team emits an `improvement_candidate`
(see `self-improvement-loop.md`) with `finding` describing the
missing evidence type and `proposed_change` proposing how to add it.
This is candidate-only; it is not auto-applied.

## Cost note

Re-running producer commands doubles the compute cost of `Done`
transitions. The skill encourages:

- Cached test results where the artifact hasn't changed.
- Smaller test subsets at handoff_gate, full suite only at
  integration_gate.

A hard cap: a single team's re-runs across all gates must be ≤ 10%
of the run's total compute cost. If exceeded, the runner should be
reconfigured, not the gate.
