# P3A-W1 Exact Lineage, Native Materialization and Cross-client Journey Amendment Review 1

Date: `2026-08-03`

Reviewer: fresh independent read-only Reviewer

Reviewed Amendment SHA-256:
`5e333ffd14c2a077a4330e5c2e095de6c7302297878d98eefc87e0936c82054d`

The Reviewer independently verified that digest and the parent Contract,
Repair 1, Repair 2 and Gate 1 Review 3 hashes referenced by the Amendment. The
Reviewer edited no file, staged nothing and ran no product, live or network
action.

## Findings

```text
P0 = 0
P1 = 0
P2 = 0
```

No repair requirement remains.

## Closure evidence

- Attempt-specific manifest/root identity is removed from immutable node
  semantics and frozen only on selected Attempt, Run and materialization facts.
- One Team dispatch authority remains the sole writer for one composite
  `AppendBatchIfStreamHeads` transaction; no second Journal, writer, Projection
  or P3A WorkItem is introduced.
- CAS-loser, pre-CAS crash, post-CAS recovery, corrupt/foreign root, cleanup and
  no-launch-before-authoritative-re-read behavior is explicit and testable.
- Artifact/template/promotion remains exact-digest-bound and Candidate/Draft
  only; raw Evidence, Grant, prompt and hidden output are not copied.
- Product owned paths do not expand, `P3A-W2` does not exist, and no dependency,
  migration, staging, push, merge or live action is authorized.
- The real native GUI plus production PTY journey matrix uses one daemon,
  socket and isolated root and requires separate Product and Operational/Trace
  results.

## Verdicts

```text
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Amendment: PASS
```

The existing P3A-W1 implementation may continue only within the reviewed
Amendment and all unchanged parent gates.

VERDICT: `PASS`
