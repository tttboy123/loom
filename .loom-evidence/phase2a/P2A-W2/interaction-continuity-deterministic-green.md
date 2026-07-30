# P2A-W2 Interaction Continuity Deterministic GREEN

**Baseline**: `65c719c`
**Status**: `GREEN AFTER REPAIR 4 — READY FOR INDEPENDENT RE-REVIEW`

## Product closure

- native product flow starts at `New task` in a three-column
  Tasks/Conversation/Inspector workspace;
- per-task composer, inspector, thread anchor and disclosure state survive
  task switches and authoritative refresh/reconnect;
- historical Team selection loads one bounded read-only activity page;
- Team Builder remains inline, one-question-at-a-time and save-only;
- compatible existing role-option labels remain the Provider/model choice
  surface; before save, native and TUI surfaces review the selected role,
  Provider, model, auth mode, Runtime compatibility, role and Team
  permissions, maximum budget and estimated cost; no new model catalog or
  authority was created;
- role questions and final confirmation show the compatible existing role
  choices with authoritative Runtime/model presentation; unselected options
  explicitly defer Provider/auth until selection, while the selected option
  shows exact Provider/model/auth from the returned preview. Native buttons and
  TUI `m`/`s` actions submit the exact existing `main_role` or `subagent_role`
  edit field and role-option ID, then render that authoritative preview;
- native and TUI task search/filter preserve the current selection even when
  it does not match the active query;
- Bubble Tea starts at `Tasks`, preserves selection across primary views and
  hides raw history identifiers;
- product daemon startup safely reclaims the exact abandoned-owned lock shape;
- active, live, symlink, hard-link, wrong-mode, nonzero, regular, replacement
  and ambiguous paths remain fail-closed;
- the server holds a non-blocking exclusive advisory lock for the listener
  lifetime and removes the exact lock path before releasing it;
- exact removal revalidates file identity after the observed interleaving seam,
  preserving a replacement installed after the first identity check.

## Repair 1 closure

Implementation Review 1 reported one P1 and two P2 findings. Repair 1 stayed
inside the same frozen complete P2A-W2 owned boundary and added no Amendment or
WorkItem:

1. the native and TUI final confirmation surfaces now expose the complete
   already-bound execution preflight;
2. both product clients now provide bounded task search/filter while retaining
   the selected task;
3. exact file removal has an observed interleaving boundary and a second
   descriptor/path identity validation before removal.

During the full matrix, the real Go-server-to-Swift-client fixture also exposed
that the new Store state had acquired source dependencies outside the fixture's
frozen standalone `swiftc` boundary. The workspace continuity types now live
with the owned Store that uses them, and Store reconciliation has no dependency
on presentation-only source. The existing strict cross-language fixture remains
unchanged and passes, so no off-contract test file is required.

## Repair 2 closure

Fresh Repair 1 Implementation Re-review confirmed the three prior findings
closed but returned one P1: role options were still responsibility-only labels
and confirmation could edit only name/purpose. Mandatory Repair 2 RED
reproduced the missing native role-choice model/allowed edit fields and missing
TUI `builder_edit main_role` action.

Repair 2 adds only client presentation and dispatch through existing accepted
Builder fields. It does not change the application service, IPC schema,
catalog, role definition, Team authority or confirmation CAS.

## Repair 3 closure

Fresh Repair 2 Re-review found that generic Provider/auth fallbacks for an
unselected alternative were not authoritative. Repair 3 tests reproduce and
remove those guesses. An unselected option now shows only its authoritative
responsibility, Runtime and model plus `Select to review provider and sign-in`.
After the edit returns, the selected role's exact Provider/auth is rendered
from the authoritative preview.

## Repair 4 closure

Fresh Repair 3 Re-review found a same-Agent alternate profile could be
misclassified as current. Repair 4 fixtures use the same AgentDefinition with
two Runtime bindings. Native and TUI now require the full available binding
tuple—kind, AgentDefinition, Runtime profile and Runtime instance—before
attaching selected state or exact Provider/auth. The alternate remains
unselected and receives only its own authoritative model plus the
select-to-review cue.

## Required verification

All commands completed with exit code `0` unless explicitly described as the
already-recorded mandatory RED:

```text
go test ./internal/localipc ./internal/tui ./cmd/loomd -run 'Test(PrepareSocket|Server|ProductDaemonReclaims|InteractionContinuity|TaskSelection)' -count=1
go test -race ./internal/localipc ./internal/tui -count=1
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
swift test --package-path apps/macos
swift test --package-path apps/macos --sanitize thread
swift build --package-path apps/macos -c release
git diff --check
```

The focused lock suite includes 50 two-contender abandoned-lock reclamation
repetitions with exactly one winner per repetition. The complete Swift debug
suite executed 41 XCTest cases plus 4 Swift Testing cases with no failure.
Thread Sanitizer executed the same 41 XCTest cases plus 4 Swift Testing cases
with no failure; only the opt-in preview export test was skipped in the TSan
run.

The strict real boundary fixture remains green:

```text
LocalProductReadService -> Go IPC Server -> Swift Client
```

It retains the strict Swift decoder and canonical empty collection wire shape.

## Static and scope checks

- `gofmt` completed on all changed Go files;
- no repository Swift format configuration exists, so no formatter command was
  invented;
- `go mod tidy -diff`, module verification and `git diff --check` are clean;
- secret-negative scan found no supplied key prefix or hard-coded
  `OPENAI_API_KEY`;
- no Event, Journal, StateWriter, Projection, Credential Broker, Provider
  verifier, Team domain, Runtime adapter, execution, Grant, Evidence,
  Scheduler, installer, LaunchAgent or release surface changed;
- real product socket/lock, Keychain, Provider, installed app and resident
  service remained untouched during deterministic implementation.

## Remaining gate

Fresh independent read-only Repair 4 Implementation Re-review returned `PASS`
with no P0, P1 or P2 findings. The atomic Candidate commit is the current gate.
The single live lineage `p2a-w2-live-20260730-007` remains locked until the
committed Candidate passes exact preflight.
