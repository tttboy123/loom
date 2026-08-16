# P2D-W2C/W2D daemon restart Attempt reconstruction v10

Date: 2026-08-14

Status: `SOURCE VERIFIED / EXPLICIT RESUME AND INSTALLED LIVE OPEN`

## Boundary

This slice reconstructs content-free Agent Attempt recovery state when `loomd`
opens an existing Journal and encrypted Agent Inbox after daemon restart. It
does not revive a dead process, register a recovered Attempt as active, repeat a
Provider request, or authorize a retry.

## Implemented

- `AttemptLoopStarted` is the only accepted recovery anchor. Loom reconstructs
  the complete frozen `AttemptLoopBinding` from its strict payload and Incident
  ID, recomputes the stream identity, and replays every Loop event.
- Replayed state is checked against the current Run, claim ID/generation,
  Runtime, Agent, Execution Binding digest, capability digest, Capsule digest,
  budget and protected Run authority. Older claim generations remain history
  and terminal Runs produce no recovery candidate.
- The Agent Inbox authority is replayed before storage reconciliation. Journal
  or Run authority corruption fails daemon startup closed. Per-Attempt encrypted
  storage absence or status conflict becomes only that Agent's
  `recovery_blocked` outcome, so another Agent remains independently governed.
- A consumed Queue or Steer/Inject transition with no `ModelRequestAdmitted`
  reconstructs its exact Turn/Step cursor, input IDs and preceding model output
  checkpoint digest as `pre_model_resume`.
- An open Step with `ModelRequestAdmitted` is
  `provider_outcome_uncertain`. It is never automatically replayed.
- Product startup records the safe `agent_attempt_reconcile` stage with one of
  `agent_input_resume_required`, `provider_outcome_uncertain`,
  `agent_input_recovery_unavailable`, or `agent_input_recovery_conflict`.
  These diagnostics are deliberately non-retryable until a separate explicit
  resume/recovery authority exists.

The report contains only non-secret frozen identity, digests, cursor metadata,
input IDs and Provider/Account/Model identity. It has no API Key, Inbox body,
Prompt, transcript, Provider response, ciphertext or native session handle.

## Verification

- Mandatory RED failed only because `RecoverAfterRestart`, restart dispositions
  and the product diagnostic bridge did not exist.
- Focused Queue, Step, uncertain Provider, terminal and per-Agent isolation
  tests pass.
- Focused ten-run race checks pass for work authority and daemon diagnostics.
- Full `internal/work` and `cmd/loomd` package tests pass.
- Serial full repository `go test -p 1 ./... -count=1` passes.
- `go vet ./...` passes.
- `git diff --check` passes.

## Still open

- Explicit user-approved resume/recovery commands and UI actions.
- Reconstructing a persistent Harness process or native external session.
- Version-locked Codex app-server and Claude stream-json continuation.
- Official Pi installed conformance, CV6, mixed-Team ATL9 and COMP2-E removal.
- Installed App crash/restart and real Provider acceptance.

No App, network, Provider, real credential, user workspace or external Runtime
was accessed.
