# P2A-W2 Credential Transaction and Shutdown Closure Repair 1 GREEN

Date: 2026-07-30

Status: CANDIDATE — fresh independent Repair 1 Re-review required

## Closed finding

`buildProductSetupService` no longer converts a production
`NewProductKeychainStore` construction failure into `nil, nil`. It returns the
closed `setup credential boundary unavailable` error, so
`newProductDaemonRunner` cannot continue with setup silently disabled.

The targeted test uses a relative fixture socket to force the production
constructor rejection and proves both a non-nil error and nil setup service.
It accesses no real Keychain, Provider, network, app or daemon.

The amendment's status header now records the already completed Contract Repair
1 Re-review PASS. No behavioral clause changed.

## Verification

```text
go test ./cmd/loomd \
  -run 'TestProductSetupFailsClosedWhenProcessKeychainCannotBeConstructed|TestProductCredential|TestLoomdCredentialHelper' \
  -count=10

go test -race ./cmd/loomd \
  -run 'TestProductSetupFailsClosedWhenProcessKeychainCannotBeConstructed|TestProductCredential' \
  -count=10
```

PASS.

The final Repair 1 Candidate also passed:

```text
focused credentials/provider/localipc/app/loomd tests
five-run focused race
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
git diff --check
Swift debug tests
Swift release build
Swift thread-sanitizer tests
```

The secret-negative scan remains empty. No live action ran.
