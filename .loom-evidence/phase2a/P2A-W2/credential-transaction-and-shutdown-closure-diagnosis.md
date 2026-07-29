# P2A-W2 Credential Transaction and Shutdown Closure Diagnosis

**Date**: 2026-07-30
**Status**: READ-ONLY DIAGNOSIS
**Source result**: `p2a-w2-live-20260730-005`

## Observed live facts

The reviewed attempt-005 result proves both of these facts in one native
transaction lineage:

1. the one MiniMax `Test` appended no `ProviderCredentialVerified` Event;
2. after the app exited, the exact attempt daemon remained alive after one
   normal interrupt plus 35 seconds and one exact-PID termination plus another
   ten seconds.

The attempt required exact-PID forced cleanup. No second Test or canary ran.

## Current code path

The production request path is:

```text
Swift Test
-> Local IPC credential_verify
-> LocalProductSetupService.VerifyCredential
-> CredentialBroker.Verify
-> KeychainStore.Read
-> MiniMaxCredentialVerifier.Verify
-> CommitCredentialMetadata
```

The bounded stages after Secret Store retrieval are:

- MiniMax request: at most five seconds;
- cancellation-independent terminal metadata commit after an observation:
  at most one second;
- Go and Swift `credential_verify` request budgets: ten seconds.

`KeychainStore.Read` checks `ctx.Err()` before calling
`SecItemCopyMatching`, but the synchronous C call itself has no cancellation or
deadline boundary. The same limitation applies to Keychain Put and Delete.

The local IPC server correctly cancels each handler context and closes every
connection during shutdown, then deliberately joins every tracked handler.
Therefore an in-flight synchronous Keychain call that does not return also
prevents `Server.Close`, `productDaemonRunner.Run` and the daemon process from
returning.

## Diagnosis

The live facts prove that the verification transaction did not reach a
terminal metadata append. Among the stages before that append:

- Provider observation is bounded to five seconds;
- terminal commit is bounded to one second;
- the request and connection are bounded to ten seconds;
- the synchronous Keychain operation is the only production stage without an
  in-flight cancellation boundary.

The current implementation therefore contains a proven unbounded owner
consistent with both live failures and cannot prove its required joined
shutdown or bounded verification semantics while a Keychain operation is in
flight. Attempt-005 did not retain a goroutine dump, so this diagnosis does not
claim that `SecItemCopyMatching` was directly observed as the exact stalled
frame. The owner defect exists independently and must be closed before another
credential/shutdown canary. Increasing Go or Swift IPC deadlines would leave
it unchanged and is not an acceptable repair.

The diagnosis does not claim the retained credential was invalid, does not
inspect the Keychain item or Journal payload, and does not expose or reproduce
the secret.
