# P2D-W2C/W2D Recovery Frame And Terminal Closure V19A

Status: `SOURCE VERIFIED / PRODUCTION LIFECYCLE AND IPC OPEN`

Date: 2026-08-14

## Boundary

V19A closes the source authority gap between the V18 Loom Native continuation
and the existing Team/Run governance path. It does not expose a daemon route or
user-visible Resume action.

The recovery path now:

1. consumes the process-local recovery lease and validates its exact frozen
   restart outcome;
2. resolves exactly one unrevoked original execution grant by WorkItem, Run,
   Claim ID/generation, Runtime and Agent without recovering its bearer token;
3. constructs a recovery-specific Bridge frame authority from the consumed
   recovery grant, original grant operation set, deterministic dispatch ID and
   recovery Incident ID;
4. validates sequence, binding, correlation, ACK, operation permission,
   terminal Result and optional Team frame observation;
5. compares the AdapterResult transcript byte-for-byte with the frames already
   accepted by that authority before the Attempt Loop can terminalize;
6. resolves accounting against the current authoritative Run and its frozen
   Provider/Model Rate Card, commits the Run terminal fact, then revokes the
   exact original grant with the recovery Incident ID.

Malformed, substituted, observer-rejected or AdapterResult-mismatched streams
cannot commit Run terminal state or revoke the original grant. A valid failed
terminal frame commits the controlled `agent_attempt_recovery_failed` reason;
free-form Provider text is not promoted into Run authority.

## Security properties

- The original grant token remains unrecoverable and is never reconstructed.
- Recovery frame authority requires the private one-use recovery grant plus an
  exact non-secret original grant binding.
- The recovered stream cannot expand the original grant's operation set.
- Frame correlation is fixed to the recovery Incident ID; original execution
  correlation cannot be substituted.
- AdapterResult validation runs before Step/Turn success is written.
- Terminal/accounting and revocation use bounded contexts detached from request
  cancellation after a complete terminal stream has been validated.
- Accounting uses only bounded usage/cost facts and the frozen Rate Card. No
  Prompt, Provider body, credential or grant token enters Run accounting.
- Session close errors are returned rather than silently discarded.

## Source

- `internal/supervisor/recovery_frame_authority.go`
- `cmd/loomd/product_agent_attempt_recovery_completion.go`
- `cmd/loomd/product_agent_attempt_recovery_completion_test.go`
- `cmd/loomd/product_loom_native_attempt_recovery.go`

## Verification

The mandatory RED failed only on the absent recovery completion and frame
authority symbols. GREEN coverage proves successful accounting/terminal/grant
closure, correlation substitution rejection, AdapterResult mismatch rejection,
observer rejection, pre-dispatch original-grant conflict handling, exact real
authorization snapshot resolution, revoked-grant exclusion and authoritative
Run identity/accounting resolution.

Passed:

```text
go test -race ./cmd/loomd \
  -run 'TestProductAttemptRecoveryCompletion|TestProductAuthorizationRecoveryGrantClosure|TestProductLoomNativeAttemptRecoveryResumesExactEncryptedState' \
  -count=10

go test ./internal/supervisor -count=1
go test ./cmd/loomd -count=1
go test -p 1 ./... -count=1
go vet ./...
```

The exact touched Go file set produced no output from `gofmt -l`.

## Open gates

The production daemon does not yet construct this coordinator, recover the
mission Team frame observer/evidence collector, or expose authenticated
preview/confirm/resume IPC and Swift governance. Those are V19B/V19C.

Run terminal commit and original grant revocation are two idempotent authority
operations, not one cross-stream transaction. A crash after Run terminal commit
but before grant revocation therefore remains a deliberate, visible V19B startup
reconciliation gate. Production wiring must discover that exact state and
revoke only the uniquely matching grant; it must never redispatch the Provider
or guess among grants.

No App, network, real Provider, real credential, user workspace or external
Runtime was accessed. Installed CV6, mixed-Team ATL9, broader Runtime recovery,
authenticated recovery UI and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.
