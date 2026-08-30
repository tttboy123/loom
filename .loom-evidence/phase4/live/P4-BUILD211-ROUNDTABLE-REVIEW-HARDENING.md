# Phase 4 Build 211 RoundTable review hardening

Status: `ACCEPTED`

Date: 2026-08-27

## Installed identity

- App: `/Users/lune/Applications/Loom.app`
- Version/build: `0.5.3 (211)`
- App executable SHA-256:
  `89627eaf5ca4cd8a78be0a867b9c6684a6c05db6a2ae61fae96d9b533837ae04`
- Bundled daemon SHA-256:
  `9795d053beb888ac02c4b4db8c2b43e54182ef8c2ef67286d9b5a0a298ffc23e`
- Managed launch: App PID `17167`, daemon PID `17197`; the daemon canonical
  argv retained state, isolation, socket, runtime paths and managed parent
  identity.

## Independent review closure

Build 211 closes every actionable finding from the final independent review.

- The daemon projects `pending`, `available` or `unavailable` from the exact
  active Attempt. Swift no longer infers live Steer from `harness_adapter`.
- RoundTable Steer uses the same 4,096-byte boundary in App and daemon and
  clears rejected guidance bytes.
- Import requires top-level Session, Moderator and conclusion time to match the
  digest-bound AlignmentSummary even when the packet digest is recomputed.
- The macOS file reader reads at most 1 MiB plus one detection byte; it does not
  load an oversized local file before rejecting it.
- A concluded discussion takes precedence over stale intervention history, and
  session statistics distinguish `2 Agents + Moderator` from participant-seat
  capacity.

## Installed UI gate

The signed installed App launched its bundled daemon without a separate user
step. The retained Mission-linked concluded session opened through
RoundTable -> Missions and showed:

- `Discussion concluded`, with no contradictory `Input needed` status;
- `2 Agents + Moderator` while the seat editor correctly retained the two-to-six
  Agent capacity;
- both exact Loom Native / MiniMax account/model routes, the accepted candidate
  and the Alignment Summary after restart.

Build 208 remains the retained real Provider gate for Loom Native Steer,
encrypted delivery, seat-local failure and restart restoration. Build 211
changes capability projection and boundary validation without replacing that
content-negative live evidence.

## Verification

- `go test ./... -p 1 -count=1`
- `go vet ./...`
- focused `-race` coverage for active Attempt capability, live Steer admission,
  wrapper forwarding, terminal race, 4,096-byte rejection and import identity
  mismatch
- complete macOS suite: 396 XCTest cases, two conditional skips, zero failures
- Swift Testing: 20 contract tests, zero failures
- strict deep bundle signature verification and transactional installation
