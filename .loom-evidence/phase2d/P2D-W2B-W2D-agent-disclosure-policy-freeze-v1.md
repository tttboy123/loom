# P2D-W2B/W2D Agent Disclosure Policy Freeze v1

**Date**: 2026-08-13  
**Status**: `CURRENT / SOURCE VERIFIED / PARTIAL`  
**Goal**: unified Phase 2D only  
**Source boundary**: post-build-64; installed Loom remains v0.5.2 build 39

## Result

Each account-governed Agent Run and Team Attempt now freezes the exact Provider
Account policy version, revision, digest, trust domain, retention mode, and data
region selected at claim or dispatch. Replay resolves those values from the
immutable historical policy fact identified by revision and digest. A later
account policy revision affects only future claims.

The frozen values pass through Run projection, Team Attempt overlay, Board wire,
strict Swift decoding, and the visible Agent row. The UI does not read the
Provider Account directory's current policy to describe a historical Attempt.

## Fail-closed rules

- v1 remains replayable only with disclosure values empty and is shown as
  disclosure unspecified.
- v2 requires one closed trust domain, retention mode, and data region.
- Wire version downgrade or any unknown/substituted disclosure value is rejected.
- Policy revision/digest and disclosure values stay Attempt-local across policy
  updates and terminal Run replay.
- Board payloads contain no credential reference, endpoint fingerprint, API key,
  Authorization Header, Prompt, Capsule body, or Provider response.

## Verification

- Work tests cover v2 claim freezing, terminal preservation, and a later v2
  policy update that does not mutate an admitted Run.
- Projection tests cover exact Run reconstruction and Team Attempt overlay.
- API tests cover all non-secret Board fields.
- Swift model tests decode the exact tuple and reject version, trust, retention,
  and region substitutions.
- UI tests assert the Agent row presents policy version/revision, trust domain,
  retention mode, and region.
- Go normal and race matrices pass for `internal/work`, `internal/projection`,
  `internal/api`, `internal/app`, `internal/supervisor`, and `cmd/loomd`.
- The serialized full-repository Go suite passes every package, together with
  `go vet ./...`, module verification, Go formatting, and diff checks.
- The complete macOS suite passes 202 XCTest cases with one intentional visual
  export skip and nine Swift Testing contracts.

## Honest boundary

This closes the source-side Agent disclosure-policy identity and presentation
gap. It does not prove a Provider honored the user-selected policy, perform
installed CV6, execute the live four-Provider Team matrix, implement parallel
Aggregation, or complete the general Tool Loop.

No bundle was built, launched, or installed. No real credential, Prompt, or
Provider was accessed. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

The optional whole-file `swift-format lint` gate is not green because these
pre-existing large Swift files do not conform to the tool's default indentation
profile. This slice does not mechanically reformat unrelated source; the full
Swift compiler and test gates above are authoritative for this increment.
