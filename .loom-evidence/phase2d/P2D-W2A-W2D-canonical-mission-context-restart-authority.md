# P2D-W2A/W2D Canonical Mission Context and Restart Authority

Date: 2026-08-12

Status: CURRENT / SOURCE VERIFIED / BUILD 49 PACKAGED / NOT INSTALLED.

## Boundary

- `MissionExecutionCommand` supports Canonical Mission Context v1 with the
  confirmed Goal, ordered confirmed constraints, and ordered accepted
  decisions.
- Preflight and start hash the exact same context through the existing
  canonical preflight digest. Invalid versions, whitespace drift, duplicates,
  cross-class duplicates, and control commands carrying context fail closed.
- Every Role Context Capsule classifies Goal, constraints, and decisions as
  separate authoritative items. The observed workspace baseline remains a
  separate observed item.
- Preflight performs no Vault write. Start persists every planned Role Context
  Capsule body and its derived dispatch payload in one authenticated encrypted
  envelope.
- Journal and projections retain only the non-secret Capsule authority record.
  Prompt text and Capsule body do not enter Journal, diagnostics, or Evidence.

## Restart contract

- The daemon lists only validated non-secret authority records for the exact
  Mission Conversation, then decrypts the uniquely matching Capsule for each
  planned execution.
- A projected Attempt must also match the complete frozen execution binding,
  including Harness, Provider Account, endpoint fingerprint, credential
  reference/revision, model, reasoning effort, timeout, budget, and
  capabilities.
- Restart restores the original source snapshot digest and dispatch payload.
  It does not recompile changed source into a new execution context and does not
  write new Capsule records while reconstructing.
- Missing body, legacy dispatch-only ciphertext, ambiguous authority,
  authentication failure, route drift, binding drift, or source drift fails
  closed before adapter or Provider dispatch.

## Compatibility

- Legacy objective-only commands remain readable when all v1 context fields are
  absent.
- Matching legacy dispatch-only Vault records may upgrade once to a complete
  envelope only when authority and validated dispatch payload are identical.
  Downgrade and divergent replacement remain forbidden.
- New Swift preflight/start commands always encode `context_version: 1` plus
  explicit constraint and decision arrays. Control commands omit all context
  fields.

## Mission Workbench confirmation

- The New Mission sheet exposes the objective, ordered confirmed constraints,
  and ordered accepted decisions as one native Mission Context review surface.
  Users can add and remove individual context rows before preflight.
- The explicit `I confirm this Mission context` checkbox is required before
  preflight. Objective, Team, work type, constraint, or decision edits revoke
  confirmation and invalidate any prepared preflight.
- A Store generation fence prevents a late async response for an invalidated
  context from restoring a ready-to-start state. Unchecking confirmation also
  invalidates the prepared result; Start remains disabled without current
  confirmation.
- Empty, whitespace-drifted, oversized, within-class duplicate, and
  cross-class duplicate authority values fail before execution IPC. The UI
  presents the same canonical boundary rather than silently trimming confirmed
  authority.
- The sheet uses one vertical scroll region with a fixed command bar so context
  growth and Dynamic Type do not push Review or Start out of the window.

## Verification

- Canonical Capsule body round-trip, tamper rejection, omitted-manifest trust
  checks, encrypted restart, plaintext-at-rest exclusion, validated authority
  listing, legacy upgrade, and downgrade rejection are covered.
- Mission tests cover source change after start, missing encrypted body,
  frozen credential/endpoint drift, preflight zero-write, start persistence,
  exact restart, and daemon composition from Journal authority plus encrypted
  body.
- Swift wire and Store tests cover explicit v1 encoding, byte-for-byte
  constraint/decision continuity from preflight to start, prepared-result
  invalidation, late-response fencing, and cross-class duplicate rejection.
- Native SwiftUI sheet tests cover light appearance plus dark appearance with
  accessibility Dynamic Type. They verify the real sheet opens at the bounded
  minimum size with a laid-out content region. No installed-App or
  screen-capture claim is made from this headless test.
- Affected packages and race, serialized `go test -p 1 ./... -count=1`,
  `go vet ./...`, and `git diff --check` pass. Swift passes 186 XCTest cases
  with one intentional visual-preview skip and eight Swift Testing contracts,
  all with zero failures.

## Open gates

- Conversation Segment transition confirmation and disclosure preview.
- Installed-App Mission Context interaction and pixel review.
- Approved installation plus restart/live Provider acceptance.
- Mixed-Team live failure isolation, accounting, fallback, and CV6.

## Candidate

The verified source is frozen as the unlaunched, uninstalled v0.5.2 build 49
Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build49-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, contract-string, and ZIP
byte-equivalence gates pass. Installed build 39 remains untouched.
