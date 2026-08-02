# P2A-W3 Mandatory RED

**Date**: 2026-08-01  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Contract**: Contract Repair 1 Re-review `PASS`  
**Status**: `RED CONFIRMED`

## Test-only changes

- added `internal/app/local_product_execution_test.go`;
- added the closed `mission_execution` method proof to
  `internal/localipc/protocol_test.go`;
- changed no production file before RED.

## Focused RED evidence

```text
go test -count=1 ./internal/app ./internal/localipc

internal/app/local_product_execution_test.go:11:12:
undefined: MissionExecutionPreflight
internal/app/local_product_execution_test.go:12:12:
undefined: MissionExecutionResult
internal/app/local_product_execution_test.go:14:14:
undefined: MissionExecutionCommand
FAIL loom-pi-rebuild/internal/app [build failed]
```

The compile failure is limited to the frozen missing W3 application symbols.

```text
go test -count=1 ./internal/localipc \
  -run '^TestMissionExecutionIsTheOnlyAcceptedExecutionMethod$'

mission_execution decode error = invalid local IPC request: unknown_method
FAIL loom-pi-rebuild/internal/localipc
```

The protocol failure is limited to the frozen missing method. No daemon,
Runtime, Provider, credential, native app or live process ran.

## Bounded Repair RED 1: real-client correlation independence

The initial green test reused the preflight correlation ID for Start, while the
real Swift/TUI clients correctly generate a fresh request correlation. Changing
the Start test to a fresh correlation produced a typed conflict and zero runner
calls. The canonical preflight digest incorrectly included the transport/request
correlation ID.

The repair blanks only `CorrelationID` while calculating the canonical
preflight digest. Team/WorkPackage/view/plan/Runtime/model/auth/permissions and
all authority-affecting fields remain bound. The repaired real-client shape and
concurrent-start tests pass.

## Bounded Repair RED 2: Timeline Evidence identity

The new production vertical test reached source and Verifier terminal closure,
but failed because both `evidence_available` Timeline records had an empty
`evidence_digest` even though the authoritative Snapshot exposed both digests.
The accepted stream deliberately strips raw Event payloads; the local product
mapping therefore had an incomplete user-visible Evidence identity.

The repair enriches only a Journal-authority `evidence_available` record whose
exact `evidence/<id>` stream has a matching current GlobalReadView Evidence
record with a valid digest. Missing/mismatched records remain empty and no raw
payload is exposed. The vertical test then passes ten consecutive runs.

## Implementation Review 1 Repair RED

Fresh independent Implementation Review 1 returned `FAIL` with three P1
findings and one P2 finding. Live gates remained locked. The bounded Repair 1
tests first failed on the exact missing behavior:

```text
unknown field Decisions in AuthoritativeMissionExecutionConfig
preflight.ExpiresAt undefined
```

The replacement registry expectation also rejected completed terminal flights
remaining resident, and the production Swift probe test was extended from
ping-only behavior to require a real `mission_execution` preflight/start round
trip through the Go UDS server.

During the concrete prepared-control proof, a second focused RED found that the
new router emitted `Operation:"decide"` while the accepted Decision service
requires `Operation:"submit"`. The exact failing command retained the prepared
Decision ID/digest, node, attempt, generation and fresh product correlation;
only the operation token was wrong. The minimal one-token repair is now covered
by `TestPreparedMissionExecutionDecisionRouterMapsExactRecoveryCommand`.

Repair 1 changes no accepted authority or Event schema. No Provider, credential,
daemon live process or canary ran during RED or repair.

## Implementation Re-review 2 Repair RED

Fresh independent Re-review 2 returned `FAIL` on two P1 identity findings.
Repair 2 first added product-level expectations that:

- every execution command uses the rebuildable identity
  `mission/<team_instance_id>`;
- a control carrying the right Team/execution lineage but another MissionID
  cannot reach the decision router;
- the native client and TUI derive the same Mission identity without user input.

The focused RED reproduced both defects:

```text
mission identity drift error/calls = <nil>, 1
invalid commands reached backend: MissionID:"mission/team-other"
XCTAssertEqual: random mission-<uuid> != mission/team-1
```

The minimal Repair 2 makes canonical Mission identity part of the strict Go
command validator, derives it from the selected Team in Swift and TUI, uses it
in the production Swift contract probe, and reconstructs the same identity on
daemon restart. The real-SQLite vertical test additionally requires the Start
result MissionID to equal the terminal Snapshot MissionID.

No Event field, accepted authority, second store or new WorkItem was required.
No live action ran.
