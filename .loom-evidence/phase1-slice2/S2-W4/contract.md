# S2-W4 Frozen WorkItem Contract

- ID: `S2-W4`
- Title: Versioned Team Draft Revision Core
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W3 commit `e196107`
- Corresponds to: `TECH-PLAN.md §1.1.4, §3.3, §13.1,
  §14 Slice 2.6, and §15.3-§15.4`
- Frozen branch/head: `codex/loom-platform-slice2` at `e196107`

## Owned files

- `internal/teams/draft.go`
- `internal/teams/draft_test.go`
- `.loom-evidence/phase1-slice2/S2-W4/deliverable.md`

The Developer owns only these files. Any product-file ownership amendment
requires a recorded Controller amendment and fresh contract Reviewer PASS.

## Objective

Create a pure, immutable Team Draft revision core that:

1. binds every revision to one accepted S2-W3 catalog digest;
2. revalidates all proposed Agent, Runtime/model, Skill, member, permission,
   budget, and concurrency references against that catalog;
3. permits at most one unresolved material question;
4. increments revision exactly once for every presentation, answer, or direct
   edit;
5. rejects stale revision or catalog-digest commands; and
6. returns a Candidate stating whether the latest proposed revision is eligible
   for a later user-confirmed acceptance command.

This WorkItem does not accept/reject/expire a Draft, create TeamInstance or
AgentInstance records, choose a default Main Agent, load a defined Team, persist
state, call a model, or execute anything.

## Frozen draft model

A `TeamDraft` contains private immutable state with copied accessors for:

- non-empty stable Draft ID;
- positive revision;
- state `draft`, `awaiting_answer`, or `proposed`;
- non-empty S2-W3 catalog digest;
- complete `TeamDraftReferences`;
- optional single unresolved `DraftQuestion`;
- optional last answered question ID; and
- optional last answer text.

A `DraftQuestion` contains a non-empty stable question ID and non-empty prompt.
There is no question slice: the model structurally permits zero or one
unresolved question only.

References remain Candidate selections. A Main AgentDefinition ID in the
references is not the later default Main Agent policy and does not create an
Agent.

## Construction and presentation

`NewTeamDraft` receives a non-empty Draft ID, an accepted S2-W3 snapshot, and
complete proposed references.

- The references must pass S2-W3 membership and ceiling validation.
- The resulting immutable revision is `1`, state `draft`, bound to the snapshot
  digest, with no unresolved question or answer metadata.
- Invalid input returns a zero Draft with a typed error.

`PresentTeamDraft` receives the current `draft` revision, exact expected
revision, the same catalog snapshot/digest, and an optional next question.

- A non-empty question produces revision `+1` in `awaiting_answer`.
- No question produces revision `+1` in `proposed`.
- Presentation does not alter the validated references.
- Only state `draft` may be presented.

## Answer and direct edit

`AnswerTeamDraft` is valid only for state `awaiting_answer` and receives:

- exact expected current revision;
- the same catalog snapshot/digest;
- exact unresolved question ID;
- non-empty user answer text;
- complete revised references; and
- optional next single question.

The revised references are revalidated against the catalog. Success creates one
new immutable revision:

- revision increments exactly by one;
- the answered question ID and answer text are recorded;
- a next question produces state `awaiting_answer`;
- no next question produces state `proposed`.

The prior revision remains unchanged. Wrong question ID, empty answer, stale
revision, catalog mismatch, invalid revised references, or invalid next question
returns a zero Draft and typed error.

`EditTeamDraft` receives an `awaiting_answer` or `proposed` revision, exact
revision, same catalog, complete revised references, and an optional unresolved
question.

- References are revalidated.
- Revision increments exactly once.
- A question produces `awaiting_answer`; no question produces `proposed`.
- Edit does not fabricate answer metadata; prior last-answer metadata remains
  unchanged.
- State `draft` must use presentation rather than edit.

## Latest-revision acceptance eligibility

`CheckTeamDraftAcceptable` is pure Candidate validation. It succeeds only when:

1. expected revision equals the Draft's current positive revision;
2. supplied catalog digest and snapshot match the Draft binding;
3. state is exactly `proposed`;
4. there is no unresolved question; and
5. current references still pass S2-W3 validation.

Success returns a Candidate containing `Eligible=true`, Draft ID, revision, and
catalog digest. It does not transition the Draft to `accepted`, record user
confirmation, create a TeamInstance, write an Event, or authorize execution.

State `accepted`, `rejected`, and `expired`, plus the actual user-confirmed
accept command, are explicitly later WorkItems.

## Failure and immutability semantics

Typed errors inspectable with `errors.Is` cover:

- invalid Draft/question/answer input;
- invalid state transition;
- stale expected revision;
- catalog digest mismatch;
- question ID mismatch; and
- wrapped S2-W3 catalog-reference errors.

Every failure returns a zero next Draft or zero eligibility Candidate. The
current Draft remains unchanged.

All input slices and every accessor result are deep copied. Caller mutation of
original references, returned references, questions, or answer data cannot
change any existing revision.

## Acceptance boundary

1. `internal/teams/draft.go` imports only standard library and uses the accepted
   S2-W3 package-local catalog types/functions.
2. No Agent CLI, Provider SDK, SQLite, Journal, projection, Evidence,
   filesystem, environment, network, process, UI, adapter, Runtime execution,
   credential, or Grant package is imported.
3. Construction and every revision revalidate complete catalog references.
4. Reordering reference sets does not change semantic validation; stored set
   accessors are normalized deterministically.
5. Exactly one question can be unresolved; malformed questions fail closed.
6. Every successful presentation/answer/edit increments revision by exactly
   one; failures do not return a usable next revision.
7. Stale revision, mismatched catalog, wrong question, invalid state, and
   invalid references fail with typed errors.
8. Only the latest `proposed` revision with no question can produce an eligible
   acceptance Candidate.
9. Tests use deterministic in-memory values only and no machine/environment
   state.
10. No terminal Draft transition, user confirmation, default Main Agent,
    TeamDefinition load, TeamInstance/AgentInstance, task graph, WorkItem,
    persistence, migration, Event, daemon/CLI entrypoint, goroutine, model call,
    RuntimeProfile binding, capacity allocation, Bridge, real Runtime Adapter,
    Run, Grant, credential, or activation is introduced.
11. Existing Slice 1 and S2-W1 through S2-W3 behavior remains green.

## Mandatory RED tests

The Developer adds `internal/teams/draft_test.go` before implementation. RED
must fail on missing frozen draft symbols only.

Required groups:

1. valid construction at revision 1/state `draft`, catalog binding, input and
   accessor mutation isolation;
2. typed construction failures for empty ID, zero catalog, and every wrapped
   S2-W3 reference class;
3. presentation to `awaiting_answer` or `proposed`, exactly-one revision
   increment, and draft-only state guard;
4. answer success with revised references, answer metadata, optional next
   question, and prior-revision immutability;
5. typed answer failures for stale revision, digest mismatch, wrong question,
   empty answer, invalid next question, invalid references, and wrong state;
6. edit success/failure, metadata preservation, and normalized references;
7. acceptance eligibility for latest proposed revision only, plus stale,
   mismatched, unresolved, draft, and awaiting rejection;
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run 'TestTeamDraftRevision|TestCheckTeamDraftAcceptable' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run 'TestTeamDraftRevision|TestCheckTeamDraftAcceptable' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting, diff, and scope:
  `gofmt` changed Go files, `git diff --check`, exact owned-file audit, unchanged
  branch/head before authorized commit.

## Required evidence

- frozen contract digest;
- exact RED output and exit code;
- all deterministic check outputs;
- transition, stale-command, catalog-membership, mutation-isolation, and
  import-boundary reports;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- Team Draft revisions are model/user-edit Candidates, not state authority.
- Catalog validation proves membership and ceilings only, not role suitability,
  user approval, Runtime liveness, permission grant, or execution authority.
- Answer text is local Draft data, not a command or authorization.
- Acceptance eligibility is not acceptance; it cannot create resources or write
  authoritative state.
- No raw credential, token, executable output, environment value, filesystem
  path, or Provider response enters the Draft.

## Governance and next gate

- No product write before fresh independent contract Reviewer `PASS`.
- Active continuous authorization permits mandatory RED after that PASS.
- One Developer owns the three frozen files; Controller owns checks/evidence.
- Developer ends at `ready_for_review`; fresh implementation Reviewer decides
  PASS or repair.
- Three failed bounded product repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one scoped
  local S2-W4 atomic commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`, new
  dependency, external mutation, paid remote work, daemon/Runtime activation,
  and autonomous execution remain prohibited.
