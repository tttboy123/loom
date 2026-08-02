# P2A-W3 Post-review Live Precondition Audit

**Date**: 2026-08-02  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Implementation Review 3**: `PASS`  
**Verdict**: `HUMAN_REQUIRED / LIVE GATES LOCKED`

## Why this audit ran

Before freezing the three one-shot live manifests, the Controller had to prove
that a fresh product state could supply the confirmed executable Team required
by the frozen `mission_execution` preflight. This was a read-only/live-free
precondition check; no Provider, credential, Runtime process or canary ran.

## Repository and historical-state evidence

The accepted Team Builder confirmation writes exactly one
`TeamDefinitionSaved` fact and returns `team_instance_created:false`. It does
not create `TeamInstanceCreated` or `AgentInstanceCreated`.

Every retained P2A-W2 product database under
`/Users/lune/Library/Application Support/Loom/*/state/loom.db` was inspected
read-only. All contained zero `TeamInstanceCreated` and zero
`AgentInstanceCreated` facts. The only retained saved TeamDefinition database
also contained no Team instance. Therefore no accepted historical Team can be
copied or selected for a truthful W3 preflight.

## Repair 3 RED

Three product-level tests were added before behavior changes:

```text
go test ./cmd/loomd \
  -run '^TestProductDaemonExecutionCompositionMaterializesConfirmedTeamForPreflight$'

confirmation.TeamInstanceCreated = false
FAIL
```

```text
go test ./internal/tui \
  -run '^TestModelConfirmedExecutableTeamRefreshesSetupAndAuthoritativeSnapshot$'

post-confirm command = setupLoadedMsg, want setup + snapshot batch
FAIL
```

```text
swift test --package-path apps/macos \
  --filter 'LocalProductStoreTests/testConfirmedExecutableTeamRefreshesAuthoritativeProductView'

product snapshot requests: 1, want 2
post-confirm executable Teams: 0, want 1
FAIL
```

The tests require an explicit Builder confirmation to materialize one confirmed
TeamInstance/Main AgentInstance and refresh both client read surfaces, while
still creating zero WorkItem, Run, Grant or execution fact.

## Accepted-authority conflict

A bounded W3-owned daemon composition attempt used only the accepted
Saved-Team domain builders and `state.CommitSavedTeamInstanceRecordSet`. It
failed closed in `teams.BuildSavedTeamRuntimeBinding` with the accepted capacity
rule:

```text
Main + dormant SubAgent selections on one Pi Runtime = usage 2
installed Pi Runtime authoritative capacity          = 1
result                                                = capacity exceeded
```

The accepted product catalog requires one Main and one SubAgent. Pi metadata
correctly reports capacity 1. The accepted Saved-Team binding authority counts
every selected role against capacity even though the record-set authority keeps
the SubAgent dormant and creates only the Main AgentInstance. W3 cannot close
this mismatch from its owned files.

The attempted production composition was removed. The previously reviewed
`cmd/loomd/product_daemon.go` byte hash was restored to
`2a83556246dc1f9f3149e31686aec367f7385c276231be1f339e8e482cabe348`.
The failing RED tests remain as the bounded reproduction.

## Stop-rule application

Changing `internal/teams/saved_team_binding.go`, redefining Runtime capacity,
inventing a second Runtime instance, raising Pi capacity, writing raw Journal
events or seeding a test fixture would reopen or bypass an accepted authority.
Section 12 of the frozen W3 contract requires `HUMAN_REQUIRED` before that
change. Consequently:

- Implementation Review 3 remains valid only for its exact 30-file snapshot;
- its live authorization is superseded by this newly discovered precondition
  failure;
- Codex, MiniMax and Pi manifests were not frozen or consumed;
- no native-window/TUI live walkthrough ran;
- no W3 commit was created;
- the unrelated resident daemon was not signalled or reconfigured.

## Required governed continuation

Continuation requires one reviewed P2A-W3 amendment reopening only the accepted
Saved-Team capacity boundary and its tests. It must decide and prove whether
dormant, uncreated SubAgents consume materialization-time Runtime capacity,
without weakening dispatch-time/run-time capacity enforcement. After that
amendment passes, Repair 3 may implement the confirmed-Team product
materialization, rerun the complete deterministic matrix and obtain a fresh
Implementation Review before any live manifest is frozen.
