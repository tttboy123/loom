# S4-W2 GREEN and Verification Evidence

Date: 2026-07-26
Baseline: `87ea092`
Candidate: uncommitted S4-W2 owned-file diff

## Focused GREEN

```text
go test ./internal/verification ./internal/rules ./internal/evidence ./internal/work ./internal/app ./internal/projection -count=1
```

Result: PASS for all six packages.

```text
go test -race ./internal/verification ./internal/rules ./internal/evidence ./internal/work ./internal/app ./internal/projection -count=10
```

Final result:

```text
ok loom-pi-rebuild/internal/verification
ok loom-pi-rebuild/internal/rules
ok loom-pi-rebuild/internal/evidence
ok loom-pi-rebuild/internal/work
ok loom-pi-rebuild/internal/app
ok loom-pi-rebuild/internal/projection
```

The final rerun includes the concurrent recovery-scheduler exact-once test.

## Impact and whole-repository gates

```text
go test ./internal/teams ./internal/authorization ./internal/supervisor ./internal/runtime/piadapter -count=1
```

Result: PASS for all four packages.

```text
go test ./... -count=1
```

Result: PASS for all packages.

```text
go test -race ./... -count=1
```

Result: PASS for all packages.

```text
go vet ./...
```

Result: PASS with no output.

```text
GOOS=windows GOARCH=amd64 go build ./internal/evidence ./internal/verification ./internal/rules
```

Result: PASS with no output.

```text
gofmt -d <all S4-W2 owned Go files>
git diff --check
```

Result: PASS with no output.

## Behavioral closure

- Evidence finalization and reopen return one immutable output summary bound to
  the exact capture identity and Evidence digest.
- Output classification distinguishes `valid_nonempty`, `valid_empty`,
  `transient_empty`, and `invalid`.
- Recovery decisions are deterministic, digest-bound, attempt/credit bounded,
  approval-gated, and preserve Agent/Runtime on retry or local workflow
  fallback.
- A concrete `rules.RecoveryDecision` is required at the Work authority
  boundary; caller-selected action/budget/fallback fields no longer exist.
- The first Team transaction freezes complete per-node output, recovery,
  credit, workflow, and approval semantics before any WorkItem/Run.
- Store-returned Receipt plus exact reclassified observation are required for
  attempt commit. Zero values, changed contracts, and forged observation
  counts write zero Events.
- Concurrent exact recovery calls produce one recovery fact and one next
  attempt lineage; CAS conflict is visible and exact post-commit retry is
  idempotent.
- The Coordinator performs at most nine passes, never sleeps, executes only
  due scheduled attempts, rejects restart policy drift, and closes allowed
  empty, transient retry, workflow fallback, and human-required canaries.
- Team Projection/View replay all semantic/classification/recovery metadata,
  deep-copy requested records, expose accepted S3 Teams as explicit
  `legacy_semantic_unbound`, and preserve the old view after malformed recovery
  replay.
- Journal recovery/terminal payloads contain bounded metadata and digests only;
  no raw Frame/output, terminal reason from the capture, Grant, credential,
  prompt, hidden reasoning, Provider/model/session, or per-token data is added.

## Scope

No dependency, Bridge v1, Supervisor/Adapter, daemon, API/CLI, Provider/model
fallback, checkpoint, Verifier, WorkItem Done, S4-W3, S4-W4, or Phase 2
implementation was added. User-owned `AGENTS.md`, `PROGRESS.md`, `.codex/**`,
`.loom-drafts/**`, and the post-S3 scratch queue remain outside the Candidate.

VERDICT: PASS
