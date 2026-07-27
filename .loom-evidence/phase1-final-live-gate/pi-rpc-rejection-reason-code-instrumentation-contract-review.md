# Final Live Gate Pi RPC Rejection Reason-Code Instrumentation Contract Review

- Date: `2026-07-27`
- Reviewer role: fresh independent read-only Contract Reviewer
- Baseline: `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`
- Reviewed frozen-contract SHA-256:
  `b5b2203d78e405e2d53ee2c7832f40b7e80fb6bf5bfd04922bee14ac9e90e731`
- Contract:
  `pi-rpc-rejection-reason-code-instrumentation-amendment.md`

## Findings

### Critical

None.

### Important

None.

### Minor

Existing worktree changes are outside this amendment's ownership. The contract
correctly declares prior evidence, `source-lock.json`, user-owned dirty files,
and all private attempts read-only. Its scope gates require staging and
committing only owned files.

## Review basis

The Reviewer independently inspected the frozen contract, committed Pi RPC
adapter and tests, opt-in live harness, progressive canary evidence, locked
installed Pi `0.82.1` source behavior, and accepted authority boundaries.

The Reviewer confirmed:

- the contract is diagnostic-only and freezes acceptance equivalence;
- it cannot permit `responseModel` or any parser/behavior widening;
- phase, event, and reason vocabularies are closed and cannot contain
  transcript-derived text;
- non-disclosure excludes raw JSON, output, prompts, IDs, model/provider
  strings, usage values, paths, credentials, and arbitrary Runtime text;
- RED and GREEN gates are specific and testable;
- the live boundary permits exactly one isolated post-commit diagnostic
  canary, with no retry or fallback; and
- all possible canary outcomes remain `HUMAN_REQUIRED`.

The locked Pi source supports the diagnostic need: Pi may populate
`responseId` and conditionally `responseModel` on Provider chunks, while the
current Loom debug state cannot identify which predicate rejected. The prior
progressive canary evidence proves the bounded post-assistant-start failure and
consumed authorization.

No edit, stage, commit, live Runtime/model, network action, credential access,
or private-state mutation was performed by the Reviewer.

VERDICT: PASS
