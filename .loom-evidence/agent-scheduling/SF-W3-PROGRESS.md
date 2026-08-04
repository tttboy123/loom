# SF-W3 Implementation Progress (RED → focused green → journey verified)

Date: `2026-08-05`

Status: `CORE IMPLEMENTED + JOURNEY VERIFIED PASS; REVIEWS + COMMIT PENDING`

Baseline: SF-W2 `36b6c4b8`.

## RED (mandatory failing tests first)

RED tests target the new integration/observability surfaces: competing
Integrators one CAS winner; stale integration rejected; canary double-run
blocked; unauthorized/stale/malformed streaming frames rejected; rollback
restores the prior version; later Run binds the exact new revision.

## Implementation

- `internal/integration` — `integrator.go` (single-writer Integrate with
  `IntegrationStarted` + `CandidateIntegrated` via CAS; stale integration
  rejected), `canary.go` (one-shot offline canary with idempotent CAS,
  duplicate blocked), `release.go` (versioned release, later-Run adoption,
  rollback without history rewrite), `model.go` (ReleaseCandidate, CanaryRun,
  NodeOutputFrame).
- `internal/observability` — `streaming.go` (authorized, generation-bound
  frames; unauthorized/stale/malformed rejected), `timeline.go` +
  `attention.go` (rebuildable projections, never an authority).
- `internal/app/local_integration.go` + `internal/api/local_integration.go`
  — `integration_snapshot` / `integration_command` wire surface.
- `internal/localipc/protocol.go` + `cmd/loomd/product_daemon.go` routing;
  `cmd/loomd/sf3_integration_wire_test.go` real-socket wire test.
- `internal/tui/attention.go` + model wiring — `Integration` screen (release/
  canary/timeline/attention) for the real PTY TUI.
- `LocalTimelineModels.swift` + `LocalIPCClient.callJourneyRaw` + probe
  `--integration-snapshot`/`--integration-command` modes + Swift model tests.

## Cross-client journey (COMPLETE — VERIFIED PASS)

`/private/tmp/sf3-journey-final` (journey
`addcee11-8224-40db-b665-fa6a40a82e22`) verified by
`scripts/verify-sf3-cross-client-journey.sh` (PASS): single Integrator
published a versioned release; competing stale integration lost (conflict);
canary ran once offline with duplicate blocked; unauthorized frame denied,
authorized frame published to Timeline/Attention; later Run adopted the
release; rollback restored the prior version; restart/reconnect checkpoint
rebuilt the identical state with no duplicate effects.

Journal event set (exact): IntegrationStarted ×1, CandidateIntegrated ×1,
CanaryStarted ×1, CanaryCompleted ×1, NodeOutputFramePublished ×1,
ReleaseAdoptedByLaterRun ×1, ReleaseRolledBack ×1 + platform init.
Deterministic matrix green (Go full/race/vet/tidy/gofmt; Swift full 96+4).

Next: implementation / dual-Result / Whole-Candidate reviews, exact staging
per `SF-W3-SOURCE-LOCK.json` (32 paths, digest `59297f53…`), single SF-W3
atomic local commit, then the whole-slice acceptance review.
