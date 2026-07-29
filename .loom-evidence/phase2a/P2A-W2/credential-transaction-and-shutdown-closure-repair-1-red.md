# P2A-W2 Credential Transaction and Shutdown Closure Repair 1 RED

Date: 2026-07-30

Command:

```text
go test ./cmd/loomd \
  -run TestProductSetupFailsClosedWhenProcessKeychainCannotBeConstructed \
  -count=1
```

Result: expected RED (`exit 1`).

```text
--- FAIL: TestProductSetupFailsClosedWhenProcessKeychainCannotBeConstructed
buildProductSetupService() = (*api.LocalProductSetupAPI)(nil), <nil>;
want fail closed
```

The test uses an invalid local fixture socket path and does not access
Keychain, Provider, network, native app or any daemon.
