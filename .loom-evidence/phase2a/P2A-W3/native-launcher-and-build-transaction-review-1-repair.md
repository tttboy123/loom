# P2A-W3 Native Launcher and Build Transaction Review 1 Repair

**Date**: 2026-08-02  
**Superseded source-lock SHA-256**:
`c28eacb6a97ba11fce493c808c5e141df96174a467ccd2cef532e6f20571e6e7`  
**Scope**: `cmd/loomd/product_daemon_test.go` only  
**Authority/schema expansion**: none

Implementation Review 1 supplied the causal evidence gate: the reviewed lock
did not contain the contract-required setup-only MiniMax real-product proof.
Repair added
`TestProductDaemonSetupOnlyMiniMaxTestOverRealIPCDoesNotInitializeExecution`.

The test uses a migrated temporary SQLite and the real product runner, setup
service, local IPC server, framed client, Credential Broker and StateWriter. It
passes `Execution: nil` and no local-model catalog, executable or model path.
It seeds exactly one configured MiniMax metadata fact, performs one strict
`credential_verify` with an operation ID, and uses a deterministic denied
credential-store read so no Provider or network call occurs. The authoritative
result is exactly one next-revision `ProviderCredentialVerified` terminal with
`rejected/unavailable`.

It proves before construction, after construction, after the Test and after
shutdown that:

- the execution bundle is nil;
- the execution root does not exist;
- no WorkItem/Run/Grant/Frame/Evidence or other execution fact exists;
- the private socket is published and then removed cleanly; and
- no hidden retry or second terminal occurs.

The new test passed once and at `-count=20`. During the complete race matrix an
existing production decision test exhausted its one-second client budget under
sanitizer load. That failure was reproduced as resource sensitivity, not a
product response defect: only that test client budget was raised to five
seconds, then its exact race case passed at `-count=20` and the complete
serialized race matrix passed. No production behavior changed in this repair.
