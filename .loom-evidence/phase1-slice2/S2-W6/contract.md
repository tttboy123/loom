# S2-W6 Frozen WorkItem Contract

- ID: `S2-W6`
- Title: Content-Bound Team Draft Revisions
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W4 commit `0a98851` and S2-W5 commit `567967c`
- Corresponds to: `PRODUCT-PLAN.md §4.4-§4.6`, `TECH-PLAN.md §1.1.4,
  §3.3, §14 Slice 2, and §15.3-§15.4, §15.9`
- Frozen branch/head: `codex/loom-platform-slice2` at `567967c`

## Owned files

- `internal/teams/structured_draft.go`
- `internal/teams/structured_draft_test.go`
- `.loom-evidence/phase1-slice2/S2-W6/deliverable.md`

S2-W4 and S2-W5 files remain unchanged. Any ownership amendment requires a
recorded Controller amendment and fresh contract Reviewer PASS.

## Objective

Create a pure immutable `StructuredTeamDraft` composition that:

1. binds every S2-W4 Draft revision to exactly one validated S2-W5 content
   digest;
2. proves the core Draft references and structured-content references are
   identical and bound to the same S2-W3 catalog;
3. revalidates the core revision, structured content, catalog, and binding
   digest for every command;
4. permits presentation, answer, and edit only through content-aware wrappers;
5. requires an unresolved question whenever the attached content contains a
   capability gap;
6. returns an acceptance eligibility Candidate only for the latest gap-free
   `proposed` revision; and
7. remains Candidate data without terminal acceptance or resource creation.

The accepted S2-W4 core remains the lower-level immutable revision mechanism.
Its existing eligibility Candidate is not the later user-confirmed acceptance
command. S2-W6 provides the complete structured eligibility gate that future
acceptance must use.

## Frozen model

`StructuredTeamDraft` contains private immutable state:

- one copied S2-W4 `TeamDraft`;
- one copied S2-W5 `TeamDraftContentSnapshot`; and
- one lowercase SHA-256 binding digest.

The binding digest canonically covers:

- Draft ID, revision, state, catalog digest, normalized references, unresolved
  question, last answered question ID, and last answer text; and
- S2-W5 content digest.

Changing any revision/content semantic field changes the binding digest.
Reordering semantic sets inside valid structured content does not.

Copied accessors expose:

- Draft ID, revision, state, catalog/content/binding digests;
- normalized references;
- copied content, roles, tasks, markers, and gaps;
- optional question; and
- last-answer metadata.

No accessor exposes mutable internal slices, RuntimeProfile capability slices,
or budget pointers.

`StructuredTeamDraftAcceptanceCandidate` contains:

- `Eligible=true`;
- Draft ID and exact revision;
- catalog, content, and binding digests;
- Main AgentDefinition ID;
- role count; and
- task count.

It is not acceptance authority.

## Construction

`NewStructuredTeamDraft(id, catalog, content)`:

1. validates the S2-W5 snapshot against the supplied catalog;
2. constructs an S2-W4 revision `1` in state `draft` from the content's complete
   references;
3. verifies exact catalog/reference coherence;
4. copies the core and content;
5. computes the binding digest; and
6. returns a zero structured Draft with typed error on any failure.

Content with capability gaps may be constructed in state `draft`; it cannot be
presented as `proposed` without resolving those gaps.

## Content-aware transitions

`PresentStructuredTeamDraft(current, expectedRevision, catalog, question)`:

- revalidates the current binding and content;
- delegates the exact revision/state transition to S2-W4;
- preserves the same content digest;
- requires a valid non-empty question when current content is not
  acceptance-ready;
- produces `awaiting_answer` when a question exists or `proposed` only when
  content is gap-free; and
- increments the revision and binding digest exactly once.

`AnswerStructuredTeamDraft` receives:

- current structured Draft;
- exact expected revision and catalog;
- exact question ID and non-empty answer;
- a complete next S2-W5 content snapshot; and
- optional next question.

It revalidates the next content and delegates answer metadata/revision mechanics
to S2-W4 using the next content's references. If next content has any capability
gap, a valid next question is required. Success attaches exactly the next
content digest and creates one new revision.

`EditStructuredTeamDraft` receives:

- current structured Draft;
- exact expected revision and catalog;
- complete next S2-W5 content snapshot; and
- optional unresolved question.

It revalidates next content and delegates edit mechanics to S2-W4. Gap-bearing
next content requires a question. Prior last-answer metadata is preserved.

Every failure returns a zero next structured Draft and leaves the current core
and content unchanged.

## Structured acceptance eligibility

`CheckStructuredTeamDraftAcceptable(current, expectedRevision, catalog)`:

1. validates binding digest integrity and exact expected revision;
2. revalidates S2-W5 content against the same catalog;
3. requires `AcceptanceReady=true`;
4. verifies exact core/content reference and catalog coherence;
5. delegates S2-W4 latest-`proposed`/no-question eligibility; and
6. returns the pure structured Candidate.

Gap-bearing, `draft`, `awaiting_answer`, stale, catalog-mismatched,
content-mismatched, or binding-tampered input returns a zero Candidate with typed
error.

## Failure and typed-error semantics

Typed errors inspectable with `errors.Is` cover:

- invalid structured Draft;
- core/content catalog mismatch;
- core/content reference mismatch;
- binding digest mismatch;
- structured content not acceptance-ready;
- gap-bearing content without a question; and
- wrapped S2-W4 revision/state/question/input errors, S2-W5 validation errors,
  S2-W3 catalog errors, and S2-W1 Runtime binding errors.

No failure returns a usable next revision or eligibility Candidate.

## Acceptance boundary

1. Product imports are standard library plus accepted package-local S2-W3,
   S2-W4, and S2-W5 contracts only.
2. Construction and every command revalidate content and exact catalog binding.
3. Core and content references are semantically identical after deterministic
   normalization.
4. Every revision stores one content digest and one binding digest.
5. Presentation preserves content; answer/edit attach only the validated next
   content.
6. Any capability gap requires an unresolved question and blocks eligibility.
7. Exactly-one-question and exact-one-revision-increment semantics remain those
   of accepted S2-W4.
8. Input and accessor mutation cannot change current or prior revisions.
9. Only an exact latest gap-free `proposed` revision returns `Eligible=true`.
10. Existing Slice 1 and S2-W1 through S2-W5 behavior remains green.
11. No terminal Draft acceptance/rejection/expiry, user-confirmation record,
    default Main policy, TeamDefinition load, TeamInstance/AgentInstance,
    WorkItem/Event/SQLite write, allocation, workspace, process, model call,
    Runtime execution, Bridge, Run, AgentGrant, credential, daemon/CLI/UI,
    network, filesystem, environment, goroutine, or activation is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/structured_draft_test.go` before production.
RED must fail on missing frozen S2-W6 symbols only.

Required groups:

1. construction at revision `1`/`draft`, exact catalog/content/reference
   binding, lowercase binding digest, and deep mutation isolation;
2. construction failures for zero/invalid/tampered content and catalog
   mismatch;
3. presentation with gap-free content to `proposed`, with question to
   `awaiting_answer`, gap-bearing/no-question failure, exact increment, content
   preservation, stale/catalog/state failures, and prior immutability;
4. answer with revised content, answer metadata, optional next question, gap
   resolution, gap-without-question failure, exact increment, and all wrapped
   S2-W4 failures;
5. direct edit with next content, metadata preservation, gap/question rule,
   normalization, and typed failures;
6. structured eligibility success plus gap, stale, catalog, state, unresolved,
   reference mismatch, content tamper, and binding tamper rejection;
7. deterministic binding digest equality for semantic content reorder and
   inequality for revision, question/answer, and content digest changes;
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestStructuredTeamDraft|TestCheckStructuredTeamDraftAcceptable' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestStructuredTeamDraft|TestCheckStructuredTeamDraftAcceptable' -count=50`
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
- core/content coherence, gap/question, digest, stale command, mutation, and
  import-boundary reports;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- Structured Draft and eligibility outputs remain Candidates, not state
  authority.
- A content/binding digest proves deterministic identity, not authorship,
  approval, liveness, capacity reservation, or execution permission.
- Questions and answers are local Draft data, not commands or authorization.
- Gap-free means the structured Candidate has no declared capability gap; it
  does not prove Evidence, acceptance criteria, or customer Rules passed.
- No raw credential, token, environment value, filesystem path, Provider
  response, process output, or user identity enters the structured Draft.

## Governance and next gate

- No product write before fresh independent contract Reviewer `PASS`.
- Active continuous authorization permits mandatory RED after that PASS.
- One Developer owns the three frozen files; Controller owns checks/evidence.
- Developer ends at `ready_for_review`; fresh implementation Reviewer decides
  PASS or bounded repair.
- Three failed bounded product repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one scoped
  local S2-W6 atomic commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`, new
  dependency, external mutation, paid remote work, daemon/Runtime activation,
  and autonomous execution remain prohibited.
