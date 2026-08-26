# P2D-W3A Governed Handoff / Roundtable V32

Status: `SOURCE VERIFIED / STRICT IPC + CROSS-CLIENT JOURNEY + RELEASE BUNDLE`

Date: 2026-08-16

## Acceptance boundary

V32 adds the Loom Governed Handoff / Roundtable ledger: a moderator hosts
multiple independent seats and relays bounded, digest-bound messages with
per-hop confirmation, reusing the existing Journal CAS and Evidence Store. It
does not create a second Journal, Artifact, ContextPacket, decision or
projection authority. External A2A/Claude/Codex adapters, cross-account,
multi-user and auto-relay are explicitly out of scope.

## Implemented source

- `internal/roundtable` (new): journal-backed authority with
  `CreateSession/AddSeat/RetireSeat/OpenRound/ProposeMessage/RelayMessage/
  AcknowledgeMessage/InsertMessage/DropMessage/ConcludeSession/ReadView`.
  Message lifecycle `pending -> relayed -> acknowledged -> inserted|dropped`.
  Bounds: 8 KiB body, 16 artifact refs (digest-only), 16 seats, 256 rounds,
  1024 messages/round. Every fact is validated against the replayed stream and
  appended through `journal.AppendBatchIfStreamHeads` (CAS, single-winner).
  Conclude publishes a SHA-256 digest-bound `AlignmentSummary` Artifact to the
  Evidence Store and records the digest in the Journal; post-conclude writes
  are rejected.
- Canonical view digest is a pure function of the normalized view (seats and
  messages ordered by ID), so command results, replay and idempotent re-propose
  always agree. Empty collections serialize as `[]`, never `null`.
- `cmd/loomd` route controller (`product_roundtable.go`): strict per-method IPC
  params, server-authoritative timestamps, typed error codes
  (`not_moderator`, `seat_unavailable`, `concluded`, `invalid_body`,
  `invalid_digest`, `too_many_messages`, `too_many_seats`, `conflict`,
  `not_found`, `invalid_request`, `state_unavailable`).
- `internal/localipc` `validMethod` + route manifest + admission accept the 11
  `roundtable_*` methods; nil service returns `state_unavailable`.
- Swift: `LocalRoundtableModels.swift` (strict decode, subset seat invariant,
  digest/status validation), `LocalRoundtableClientProtocol`, 11
  `LocalIPCClient` methods, `LocalProductStore` helpers with typed error
  surfacing, and a self-guiding `RoundtableWorkbench` UI reachable from the new
  "Roundtable" sidebar item (create -> seats -> open round -> propose -> relay
  -> acknowledge -> insert -> conclude, with a "Run full journey" path).

## Verification

- `go test ./... -count=1 -p 1`: full suite green (real-process tests that
  flake under parallel load pass serially/in isolation).
- `go test -race ./internal/roundtable/ ./cmd/loomd/ -run Roundtable`: green.
- `swift test`: 240 tests, 0 failures, 1 skipped (pre-existing visual export
  skip).
- Go E2E over an authenticated Local IPC Unix socket: full lifecycle + typed
  error codes + nil-service availability (`TestProductRoundtableRoute*`).
- Cross-client journey: the Swift `LocalIPCClient` runs
  create/add-seat×2/open-round/propose/relay/ack/insert/conclude/snapshot
  against a real Go server over a real Unix socket and decodes every view
  (`TestStrictSwiftClientRunsRoundtableJourneyOverRealGoServer`).
- Release bundle `scripts/build-loom-local-app.sh --output
  .loom-build/Loom.app`: clean build; bundled daemon contains the
  `roundtable_*` methods.

## Deliberate exclusions

- No external A2A/Claude/Codex adapter, cross-account, multi-user or
  auto-relay path.
- Default production composition does not publish any remote tool capability.
- No credential, Prompt or Provider body enters Journal, diagnostics or
  evidence.
