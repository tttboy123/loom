# S2-W6 Deliverable

- WorkItem: `S2-W6`
- Title: Content-Bound Team Draft Revisions
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `567967c`
- Contract SHA-256:
  `fca127c6115ab031ea7aef98aebc8eb05ad5f38f89ccf0ee39ad4fdad966ee5c`
- Product SHA-256:
  `cfa37e43689ccf4184223fb36e5c258afeb0310ce30b90188dc779ae3e93e1d5`
- Repair 1 test SHA-256:
  `019b222c626c9a3c471754c10e7907744e5fe21ded8e785cbbbab0107bf09768`

## Delivered boundary

The Candidate adds a pure immutable composition of:

- one accepted S2-W4 `TeamDraft` revision;
- one validated S2-W5 `TeamDraftContentSnapshot`; and
- one deterministic binding digest over revision, state, catalog, normalized
  references, question/answer metadata, and content digest.

Construction, presentation, answer, edit, and eligibility checks revalidate the
core revision, content, catalog, reference equality, and binding digest.
Gap-bearing content requires an unresolved question and cannot return an
acceptance-eligible Candidate. Only the latest gap-free `proposed` revision can
return `Eligible=true`.

The Candidate is not terminal Draft acceptance or user confirmation. It creates
no TeamDefinition, TeamInstance, AgentInstance, WorkItem, Event, Run, Bridge,
AgentGrant, credential, process, filesystem state, network action, or execution
authority.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W6/contract-review.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestStructuredTeamDraft|TestCheckStructuredTeamDraftAcceptable' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W6 symbols. There was no syntax,
dependency, or environment failure.

## Controller verification

After Repair 1, all commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestStructuredTeamDraft|TestCheckStructuredTeamDraftAcceptable' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestStructuredTeamDraft|TestCheckStructuredTeamDraftAcceptable' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/structured_draft.go
  internal/teams/structured_draft_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- product imports: standard library and accepted package-local contracts only
- product file remained unchanged during Repair 1
- branch/head remained `codex/loom-platform-slice2` at `567967c`

## Coverage and trust-boundary evidence

- Tests prove construction identity, catalog/content/reference binding, digest
  shape, and deep-copy isolation including slices and pointer budgets.
- Tests prove presentation preserves content, handles gap-free and gap-bearing
  content, rejects stale/catalog/state failures, and leaves prior revisions
  immutable.
- Tests prove answer/edit attach exactly one validated next content snapshot,
  preserve answer metadata, enforce the gap/question rule, propagate all
  frozen typed failures, and return zero output on failure.
- Tests prove eligibility rejects stale, catalog, state, unresolved gap,
  reference mismatch, content tamper, and binding tamper cases.
- Tests prove binding-digest determinism for semantic reorder and sensitivity
  to revision/state, question, answer, and content changes.
- Static imports exclude persistence, runtime execution, network, filesystem,
  process, environment, and goroutine surfaces.

## Implementation review

Review 1 found no product correctness/security defect but required complete
typed answer/edit failure coverage and zero-output proof.

Evidence:
`.loom-evidence/phase1-slice2/S2-W6/implementation-review-1.md`

Repair 1 adds only the missing tests. Fresh independent Repair 1 implementation
review returned `PASS` with no findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W6/implementation-review-2.md`

VERDICT: PASS
