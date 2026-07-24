# S2-W4 Deliverable

- WorkItem: `S2-W4`
- Title: Versioned Team Draft Revision Core
- Risk: Strict
- Base branch/head: `codex/loom-platform-slice2` at `e196107`
- Contract SHA-256:
  `153f6efc24d724649257a1c68ec80a8e482e368b4d8adc38cb691f1c3645fcfc`
- Product SHA-256:
  `dbaf71acee08d8f12bd07f021e24d6c6f5544c332239024f544a203816fe0a95`
- Test SHA-256:
  `152338388872a880a185c26461f716e5fe92007396a6b3701961525b6a538c75`

## Delivered boundary

The Candidate adds a pure immutable Team Draft revision core:

- revision `1` construction bound to one accepted catalog digest;
- `draft` presentation to `awaiting_answer` or `proposed`;
- exact question answering with one revision increment and answer metadata;
- direct edits from `awaiting_answer` or `proposed` with metadata preservation;
- complete catalog revalidation and deterministic reference normalization on
  every revision;
- stale revision, catalog mismatch, invalid state, malformed input, wrong
  question, and invalid reference rejection with typed errors and zero outputs;
- a pure latest-proposed acceptance eligibility Candidate.

It does not accept, reject, or expire a Draft; create TeamInstance or
AgentInstance records; choose a default Main Agent; load a Team; persist or emit
Events; call a model; execute a Runtime; or authorize any action.

## Mandatory RED

Command:

```text
go test ./internal/teams -run 'TestTeamDraftRevision|TestCheckTeamDraftAcceptable' -count=1
```

Exit code: `1`.

The build failed only on missing frozen S2-W4 symbols such as `NewTeamDraft`,
`TeamDraft`, `DraftQuestion`, and `TeamDraftState`; there was no syntax,
dependency, or environment failure.

## Controller verification

All commands passed with exit code `0`:

```text
go test ./internal/teams -run 'TestTeamDraftRevision|TestCheckTeamDraftAcceptable' -count=1
go test ./internal/teams -count=1
go test -race ./internal/teams -run 'TestTeamDraftRevision|TestCheckTeamDraftAcceptable' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Additional results:

- `gofmt -d internal/teams/draft.go internal/teams/draft_test.go`: no output
- `git diff --check`: no output
- import boundary: standard library plus accepted package-local S2-W3 types only
- owned product scope: `internal/teams/draft.go`,
  `internal/teams/draft_test.go`
- branch/head remained `codex/loom-platform-slice2` at `e196107` before the
  authorized commit

## Acceptance and trust-boundary evidence

- Private Draft fields, copied slice inputs/accessors, and value-copied
  questions preserve prior revisions under caller mutation.
- One `DraftQuestion` value plus a presence bit structurally permits zero or one
  unresolved question.
- Successful presentation, answer, and edit each produce exactly one new
  revision; every failure returns a zero Draft.
- Construction and every transition call the accepted S2-W3 catalog validator.
- Reference sets are stored in deterministic order without changing semantic
  validation.
- Only an exact-revision `proposed` Draft with no question returns
  `Eligible=true`; this is Candidate data, not acceptance authority.
- No credential, environment value, filesystem path, Provider output, process,
  state-authority write, external mutation, or execution capability enters the
  Draft.

## Independent review

Fresh independent implementation review found no blocking issues and
independently reran focused, package, repository, vet, focused-race, and diff
checks successfully.

Evidence:
`.loom-evidence/phase1-slice2/S2-W4/implementation-review.md`

VERDICT: PASS
