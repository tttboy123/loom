# P2D-W2C/W2D Agent Attempt Recovery Runtime Attachment V16

**Date**: 2026-08-14
**Goal**: Phase 2D only
**Status**: SOURCE VERIFIED / PRODUCTION REATTACHER AND USER ACTION OPEN
**Contracts**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

V16 composes the one-use V15 recovery grant with a trusted Runtime reattachment
port and the existing revocable active-Attempt registry. It does not add a
Provider dispatch method, replay an uncertain Provider request, or wire a
production Runtime reattacher.

The recovered route identity is no longer incomplete:

- `pre_model_resume` extracts one exact Segment ID from the consumed encrypted
  Agent Inbox bindings selected by the recovered input IDs;
- missing or cross-Segment input metadata produces only the affected Attempt's
  non-retryable `agent_input_recovery_conflict` projection;
- the Segment ID is frozen in the v2 recovery-candidate digest and the
  content-free authorization metadata;
- the consumed grant freezes the recovery operation Incident ID and exposes a
  defensive validation boundary to trusted product composition.

Runtime composition follows one order:

1. take the process-local lease once;
2. validate the complete grant;
3. ask the trusted port to reattach a session;
4. require exact Runtime instance and session-binding digests;
5. register the recovered Attempt from the validated grant;
6. on any post-attachment failure, close the session;
7. on normal close, revoke the active registration before closing the session.

The session interface exposes only identity and close. It cannot issue a model
request or call a Provider. Concurrent close is idempotent. A registry conflict
preserves the prior registration and closes the newly attached session.

## Verification

- focused `internal/work` Segment/candidate/grant tests pass;
- focused `cmd/loomd` attachment, identity-substitution, registry-conflict and
  close-order tests pass;
- ten-run race tests pass for both affected packages;
- complete affected-package tests, serial `go test -p 1 ./... -count=1`,
  `go vet ./...` and `git diff --check` pass.

No Loom App, network, Provider, credential, user workspace or external Runtime
was accessed.

## Open gates

Production still provides no restart-safe capability resolver or reattacher.
Codex and Claude continuations remain same-process only, and no encrypted native
session handle is persisted. Authenticated recovery IPC, Swift approval/action,
post-attachment Runtime continuation, installed CV6, mixed-Team ATL9,
accounting completion and COMP2-E remain open under the sole Phase 2D Goal.
