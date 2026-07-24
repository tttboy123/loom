# S2-W7 Frozen WorkItem Contract

- ID: `S2-W7`
- Title: Explicit Terminal Team Draft Decision
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W6 commit `b810800`
- Corresponds to: `PRODUCT-PLAN.md §4.4-§4.5`, `TECH-PLAN.md §3.3,
  §14 Slice 2, §15.3-§15.4, §15.9`, and ADR-0001
- Frozen branch/head: `codex/loom-platform-slice2` at `b810800`

## Owned files

- `internal/teams/draft_decision.go`
- `internal/teams/draft_decision_test.go`
- `.loom-evidence/phase1-slice2/S2-W7/deliverable.md`

Accepted S2-W1 through S2-W6 files remain unchanged. Any ownership amendment
requires a recorded Controller amendment and fresh contract Reviewer PASS.

## Objective

Add a pure immutable terminal decision over one exact latest S2-W6 structured
Draft revision:

1. `accepted` requires an explicit user `confirm_and_start` command bound to the
   exact eligible Draft ID, revision, catalog digest, content digest, and binding
   digest;
2. `rejected` requires an explicit user `reject` command bound to the exact
   latest Draft revision;
3. `expired` requires an explicit system `expire` command bound to the exact
   latest Draft revision;
4. every successful terminal decision creates one new immutable terminal
   revision and deterministic decision digest;
5. no terminal result can be decided again through this API; and
6. all results remain domain records only and create no execution resource.

This WorkItem establishes the terminal Draft decision boundary. It does not
perform the later atomic persistence/resource-creation transaction implied by
`accepted`.

## Frozen model

`TeamDraftDecisionKind` has exactly:

- `accepted`;
- `rejected`; and
- `expired`.

`TeamDraftDecisionActorKind` has exactly:

- `user`; and
- `system`.

`TeamDraftDecisionCommand` contains:

- non-empty decision ID;
- decision kind;
- actor kind and non-empty actor ID;
- action;
- exact Draft ID and expected revision;
- exact catalog, content, and binding digests;
- positive decision Unix-millisecond timestamp; and
- reason.

Action semantics are exact:

- `accepted` requires actor kind `user`, action `confirm_and_start`, and an
  empty reason;
- `rejected` requires actor kind `user`, action `reject`, and a non-empty
  reason;
- `expired` requires actor kind `system`, action `expire`, and a non-empty
  reason.

No boolean such as `confirmed=true`, free-form alias, ordinary conversation
text, model output, Agent output, or acceptance-eligibility Candidate is
sufficient authority.

`DecidedTeamDraft` contains private immutable state:

- a copied S2-W6 `StructuredTeamDraft` source revision;
- the terminal revision, exactly source revision plus one;
- decision kind;
- copied decision command; and
- lowercase SHA-256 decision digest.

The decision digest canonically covers the source Draft ID, source revision,
terminal revision, source state, catalog/content/binding digests, normalized
references, decision ID/kind, actor kind/ID, action, timestamp, and reason.

Copied accessors expose terminal state, source identity/digests/references and
content, command metadata, and decision digest without mutable aliases.

## Frozen command

```go
DecideStructuredTeamDraft(
    current StructuredTeamDraft,
    expectedRevision int,
    catalog TeamDraftCatalogSnapshot,
    command TeamDraftDecisionCommand,
) (DecidedTeamDraft, error)
```

The command:

1. revalidates the exact S2-W6 binding, source content, references, catalog, and
   expected revision;
2. validates every decision-command field and exact source binding;
3. for `accepted`, calls the S2-W6 structured eligibility gate and requires the
   latest gap-free `proposed` revision;
4. for `rejected` and `expired`, permits an exact latest valid `draft`,
   `awaiting_answer`, or `proposed` source revision without requiring
   eligibility;
5. creates a terminal revision exactly once without mutating the source; and
6. returns the zero `DecidedTeamDraft` on every failure.

Typed errors must distinguish:

- invalid terminal decision command;
- invalid decision actor/action semantics;
- decision/source identity mismatch;
- stale source revision;
- catalog/content/reference/binding failure propagated from S2-W6;
- ineligible acceptance; and
- decision-digest mismatch during revalidation.

An exported pure validation function must revalidate a `DecidedTeamDraft`
against the current catalog and return a zero validation Candidate on failure.
The success Candidate contains `Valid=true`, terminal state, source and terminal
revisions, exact source digests, and the decision digest. It is not resource
creation or execution authority.

## Acceptance criteria

1. Exact explicit `user` + `confirm_and_start` accepts only an exact latest
   gap-free `proposed` S2-W6 revision.
2. Draft, awaiting-answer, gap-bearing, stale, mismatched, tampered, or
   malformed acceptance fails closed with a typed error and zero output.
3. User rejection succeeds for exact latest valid draft, awaiting-answer, and
   proposed sources; wrong actor/action/reason fails closed.
4. System expiry succeeds for exact latest valid draft, awaiting-answer, and
   proposed sources; wrong actor/action/reason fails closed.
5. Every terminal revision is source revision plus one and preserves the exact
   source catalog/content/binding digest and normalized references.
6. Input/source/accessor mutation does not alter stored terminal state,
   command, content, references, or digest.
7. Decision digests are deterministic for equal semantic inputs and change for
   decision kind, actor, action, timestamp, reason, source revision, or source
   binding changes.
8. Revalidation rejects zero, tampered, catalog-mismatched,
   content/reference/binding-invalid, and decision-digest-invalid records.
9. No ordinary conversation, model output, Candidate, or caller-controlled
   boolean can substitute for the typed decision command.
10. Existing Slice 1 and S2-W1 through S2-W6 behavior remains green.
11. No TeamDefinition, TeamInstance, AgentInstance, WorkItem, Event, SQLite
    write, persistence, allocation, workspace, process, model call, Runtime
    execution, Bridge, Run, AgentGrant, credential, daemon/CLI/UI, network,
    filesystem, environment, goroutine, or activation is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/draft_decision_test.go` before production.
RED must fail on missing frozen S2-W7 symbols only.

Required groups:

1. accepted success with exact explicit user confirmation, terminal revision,
   preserved source binding, immutable command/source, and validation Candidate;
2. acceptance failures for source state/gap/stale/catalog/identity/digest,
   actor/action/reason/timestamp/ID, and all propagated S2-W6 failures;
3. rejection success from draft/awaiting/proposed plus exact user semantics and
   typed failures;
4. expiry success from draft/awaiting/proposed plus exact system semantics and
   typed failures;
5. zero output for every command and validation failure;
6. digest determinism and semantic sensitivity;
7. source and accessor deep-copy isolation; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestDecideStructuredTeamDraft|TestValidateDecidedTeamDraft' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestDecideStructuredTeamDraft|TestValidateDecidedTeamDraft' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static:
  `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/draft_decision.go
  internal/teams/draft_decision_test.go` and `git diff --check`
- Import boundary: production may use standard library and accepted
  package-local teams contracts only.

## Explicit exclusions

No changes to accepted S2-W1 through S2-W6 product/test files. No TeamDefinition
load, default Main selection, TeamInstance/AgentInstance/WorkItem creation,
transaction, Event/SQLite write, persistence, resource ID allocation, agent
process, execution, permission grant, credential, network/filesystem/environment
access, goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
