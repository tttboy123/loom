# P2A-W2 Mission Orchestration Workbench Deterministic GREEN

Date: 2026-07-30

Status: PASS

Source lock: `de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`

## Outcome

The frozen vertical contract is implemented without a new Journal, scheduler,
StateWriter, Projection authority, Event schema, Grant schema, Evidence schema,
WorkItem or W4.

- Mission is a read-only facade over accepted TeamExecution and related facts.
- The Board uses only Proposed, Ready, Orchestrating, Review and Complete.
- Blocked, Retrying and Needs You remain statuses or attention.
- the only new IPC method is `mission_decision`;
- its operation is exactly read, defer or submit;
- missing prepared commands remain read-only;
- each enabled mutation is listed in `prepared_actions` and is bound to one
  exact accepted Rules/Work input;
- stale command view/generation and a real post-sheet Journal view advance are
  rejected before the owning authority;
- concurrent submit has one winner and one conflict;
- Not now, Edit scope and sheet dismissal perform no authority call;
- Authorization delegates to a real pending Rules approval and exact approval
  decision, with the sheet Mission/Team/Node/Attempt identity bound to the
  original ActionContext and reconstructed pending request ID;
- Review delegates to a real ready-for-review Work/Evidence lineage and exact
  Team node acceptance, including the exact AcceptanceDecision digest;
- Recovery delegates to a real failed Attempt/Evidence lineage and exact
  deterministic Team recovery, including the RecoveryDecision digest,
  Evidence/classification identity and current claim generation;
- the production daemon always mounts the Decision API; its default empty
  registry fails closed as `conflict`, not `state_unavailable`;
- one exact private controlled-fixture manifest can construct real
  Journal-backed Rules/Work/Run/Evidence decision inputs for the single
  post-Review canary, while an absent or invalid manifest remains fail-closed;
- a Review Mission without an exact prepared acceptance command opens a
  read-only gate whose mutation actions remain disabled;
- strict Go and Swift wires preserve empty arrays and reject unknown fields,
  null collections and identity drift;
- GUI and TUI use the same Mission lane, Team Pulse, node, Attempt and attention
  semantics;
- Provider management remains reachable from the ordinary workbench.

## Exact verification

PASS:

```text
go test ./...
go test -race ./...
go vet ./...
swift test --package-path apps/macos
swift test --package-path apps/macos --sanitize=thread
swift build --package-path apps/macos -c release
git diff --check
```

Swift debug, Thread Sanitizer and Release each executed 50 XCTest cases with one
explicitly environment-gated screenshot export skip, plus four Swift Testing
cases, with zero failures. The screenshot export was separately run with its
exact authorized evidence paths and passed.

The strict real Go IPC Server to Swift Client decision probe and the real
production-daemon empty-registry socket test passed:

```text
go test ./internal/localipc \
  -run TestStrictSwiftClientReadsPreparedDecisionFromRealGoServer \
  -count=1 -v
go test ./cmd/loomd \
  -run TestProductDaemonProductionRunnerWiresFailClosedDecisionRegistry \
  -count=1 -v
```

## Race closure

Implementation Review 1 correctly rejected the serial race substitute. Repair
1 reran the exact frozen command `go test -race ./...` without other concurrent
tool load; every package passed, including the Pi metadata observer tests.

## Prepared binding closure

Implementation Review 2 correctly rejected two incomplete prepared-command
bindings. Repair 2 first preserved focused causal RED, then passed the exact
rejection and authority delegation probes:

```text
go test ./internal/app \
  -run 'TestPrepared(AuthorizationRejectsMislabeledMissionContext|AuthorizationDelegatesExactRulesAuthorityInput|ReviewRejectsUnboundDecisionDigest|ReviewDelegatesExactWorkAcceptanceInput|RecoveryRejectsUnboundDecisionAndClaim|RecoveryDelegatesExactWorkRecoveryInput|MissionDecisionRejectsStaleViewAndGenerationBeforeAuthority|MissionDecisionConcurrentSubmissionHasOneWinner|MissionDecisionRejectsReplayAfterViewAdvances)$' \
  -count=1 -v
```

Implementation Review 3 then rejected the production empty-only registry and a
stale source lock. Repair 3 added the exact controlled production fixture
boundary, strict schema-2 prepared collection, and read-only missing-Evidence
Review Gate. That Repair 3 matrix used historical source lock
`2badc23c740b0c24fad494a02f2bf8956a60e4dfb2f35f17701fab6695213b55`.

Visual Review 1 then failed four presentation-semantic P1s and one compact
overflow P2. The same Candidate now exposes Mission identity, priority, Team
presence, Attempt/node progress and readable milestone on each Board card; a
shared selected Mission Detail in TUI; kind-specific truthful Decision content;
the visible `Plan only`/`Guided`/`Delegated` composer modes; and an explicit
five-lane horizontal navigation cue. Permission mode remains per-Mission
presentation continuity and never grants authority. The complete Go, exact
repository race, vet, Swift debug, Thread Sanitizer and Release matrices were
rerun against 31-file source lock
`8269aac930e31afa9d8f581dcdceb61779436b2033b2cd071ca9dc5af802c8a2`.

The controller's final exact-contract check then added the missing bounded
`source_kind` presentation to each Mission card. Swift debug, Thread Sanitizer
and Release were rerun against that final source byte. The final 31-file source
lock is
`de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`.
The lock contains 31 files and reproduced with zero mismatches. Secret-pattern
and forbidden-authority diff scans were empty.

## No-live declaration

No external or installed product daemon, Provider request, Keychain mutation,
installed application or live canary was used. The only product socket was an
isolated test-owned socket. Prepared Rules/Work commands executed only against
test-owned SQLite and Artifact fixtures. All windows were deterministic
non-installed AppKit fixtures.
