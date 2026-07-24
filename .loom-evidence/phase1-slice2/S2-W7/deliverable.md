# S2-W7 Deliverable

- WorkItem: `S2-W7`
- Title: Explicit Terminal Team Draft Decision
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `b810800`
- Contract SHA-256:
  `44646dd98c830072105469f4f9e52d40711de470f2af948bc091291dbb8308f4`
- Product SHA-256:
  `9966e171a1fbfbc5fe07f1ede4149780abd2b527b51bc7d513563b53be87b77c`
- Test SHA-256:
  `b5fab077902ed7293d671800b90dccaaaf32fe1b77ed02e3439e8f45c135c8d0`

## Delivered boundary

The Candidate adds one pure immutable terminal decision record over an exact
latest S2-W6 structured Draft revision:

- `accepted` requires an exact user `confirm_and_start` command and a gap-free
  proposed source that passes the structured eligibility gate;
- `rejected` requires an exact user `reject` command;
- `expired` requires an exact system `expire` command;
- every result has source revision plus one and a deterministic digest binding
  source identity/content/references and all decision semantics; and
- every command and validation failure returns zero output.

Ordinary conversation, free-form text, model/Agent output, booleans, and
eligibility Candidates cannot replace the typed exact-bound decision command.

The result remains an in-memory domain record. It does not persist a terminal
transition or create a TeamDefinition, TeamInstance, AgentInstance, WorkItem,
Event, process, Runtime, Run, AgentGrant, credential, or execution authority.

## Contract review

Fresh independent contract review returned `PASS` with no blocking findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W7/contract-review.md`

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestDecideStructuredTeamDraft|TestValidateDecidedTeamDraft' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W7 types, constants, and functions.
There was no syntax, dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestDecideStructuredTeamDraft|TestValidateDecidedTeamDraft' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestDecideStructuredTeamDraft|TestValidateDecidedTeamDraft' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/draft_decision.go
  internal/teams/draft_decision_test.go`: no output
- `git diff --check`: no output
- `find . -type d -name '__pycache__' -print`: no output
- production imports: standard library and accepted package-local contracts only
- accepted S2-W1 through S2-W6 product/test files remain unchanged
- branch/head remained `codex/loom-platform-slice2` at `b810800`

## Coverage and trust-boundary evidence

- Tests prove exact explicit user acceptance, terminal revision, preserved
  source binding, immutable command/source, and validation Candidate.
- Tests reject draft, awaiting, gap-bearing, stale, catalog-mismatched,
  source-mismatched, content/reference/binding-tampered, and malformed
  acceptance with typed errors and zero output.
- Tests prove rejection and expiry across draft/awaiting/proposed sources plus
  exact actor/action/reason semantics.
- Tests prove terminal revalidation rejects zero, command/digest/source/
  reference/binding tamper and catalog mismatch with zero Candidate output.
- Tests prove deterministic decision digests and independent sensitivity to
  kind, actor kind/ID, action, timestamp, reason, source revision, and source
  content/binding.
- Static imports exclude persistence, runtime execution, network, filesystem,
  process, environment, and goroutine surfaces.

## Implementation review

Fresh independent implementation review returned `PASS` with no findings.

Evidence:
`.loom-evidence/phase1-slice2/S2-W7/implementation-review.md`

VERDICT: PASS
