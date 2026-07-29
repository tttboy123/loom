# P2A-W2 Credential Transaction and Shutdown Closure RED

Date: 2026-07-30

Command:

```text
go test ./internal/credentials ./cmd/loomd -run 'Test(KeychainHelper|ProcessKeychainStore|ProductCredentialVerify)' -count=1
```

Result: expected RED (`exit 1`).

The new tests fail to compile only because the frozen production boundaries do
not exist yet:

```text
cmd/loomd/product_daemon_test.go:804:13: undefined: productCredentialMutator
internal/credentials/keychain_helper_darwin_test.go:96:3: undefined: keychainHelperRequest
internal/credentials/keychain_helper_darwin_test.go:97:15: undefined: keychainHelperPut
internal/credentials/keychain_helper_darwin_test.go:102:27: undefined: keychainHelperOK
```

The RED introduces no Keychain access, Provider request, daemon activation,
native-app launch or network access. It fixes the required behavior at the
process-helper protocol, two-second store budget and lost-response recovery
boundaries before production code changes.
