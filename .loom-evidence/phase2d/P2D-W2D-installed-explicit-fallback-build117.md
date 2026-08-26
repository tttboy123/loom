# P2D-W2D Installed Explicit Fallback - Build 117

Status: `INSTALLED LIVE PASS`

Date: 2026-08-23

Installed product: Loom `v0.5.3 (117)`

## User-visible result

An OpenCode Agent lost its exact MCP Enrollment during an active real tool
call. Only that Agent failed its first Attempt. Loom consumed the user's
versioned fallback approval and ran a new Loom Native Attempt with the frozen
DeepSeek account and model. The healthy Team peer continued independently and
the Mission finished successfully.

## Live gate

`TestLiveMissionOpenCodeCallsMCPAndIsolatesMidflightRevoke` ran against the
installed App-owned daemon and passed in 25.41 seconds.

The gate:

1. confirmed a two-Agent Team with one exact MCP Enrollment on the OpenCode
   subagent and an explicit Loom Native fallback;
2. approved fallback version 1 before execution;
3. waited for the installed private stdio helper to start the exact governed
   MCP call;
4. revoked the exact Enrollment revision while the call was active;
5. proved the OpenCode source WorkItem had exactly one scoped permission
   binding and then terminalized as `runtime_process_failed`;
6. proved the same Agent continued only as Loom Native Attempt 2 with fallback
   consumed and approval version 1;
7. accepted the independent verifier result and completed the Team;
8. proved account-level usage and rate-card accounting for the successful
   fallback and the unaffected peer.

## Authority and accounting facts

Team: `team-instance-f5cd1f8ba8466d78fc1bea68a29f367b`

- OpenCode Attempt 1: failed with `runtime_process_failed` after the exact
  Enrollment revoke; one scoped `JobPermissionBound` fact existed.
- Loom Native fallback Attempt 2: succeeded and passed independent
  verification with `criteria_satisfied`; usage observed, 2,792 total tokens,
  `rate_card_estimate` cost source.
- Healthy Loom Native main Attempt 1: succeeded; usage observed, 3,359 total
  tokens, `rate_card_estimate` cost source.
- Team terminal state: `succeeded`.

The final Team board froze the fallback Harness, Provider Account, model,
credential revision, Context Capsule, Route Segment, policy revision and
accounting row. No silent Provider, account or model substitution occurred.

## Harness hardening

The installed test now starts its revocation observer before Mission dispatch,
waits for the helper's active-call marker, and uses a separate private-UDS
client for the exact revoke. Its review snapshot retry recognizes the daemon's
structured recoverable `state_unavailable` response. Timeout diagnostics retain
only the final non-secret board metadata and identify fallback, accounting,
peer and terminal predicates separately.

## Safety

No API key, Authorization header, Prompt, Provider response, MCP argument or
result body entered this evidence, operational diagnostics or test output.
Credential references and ciphertext were not inspected. The evidence file is
owner-only (`0600`).

## Verification

- installed fallback gate: PASS in 25.41 seconds
- post-fallback installed OpenCode Conversation: PASS with exact `E2E-OK`;
  Vault unlocked, 25 Providers, 7 Runtimes, 4 Conversation Profiles
- focused fallback source tests: PASS
- Pi cancellation/resource cleanup: PASS, five consecutive runs
- `go test -p 4 ./... -count=1`: PASS
- `go vet ./...`: PASS
- `scripts/test-phase2d-live-acceptance.sh`: PASS
- `git diff --check`: PASS
