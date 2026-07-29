# P2A-W1 Native App Host Review Repair 1 GREEN

**Date**: 2026-07-28
**Status**: `GREEN`
**Current gate**: fresh independent Implementation Re-review

## Closed findings

1. Request IDs now validate exact UTF-8 bytes against ASCII
   `[A-Za-z0-9._:-]`. Non-ASCII alphanumerics and slash are rejected before
   connect.
2. Safe text now normalizes newline and tab to bounded single spaces and
   removes carriage return and other C0/C1 controls. Visible/accessibility
   labels remain single-line.
3. The builder now performs its own non-emitting forbidden API, bridge,
   workspace, environment-read, credential-token, and private-key static scan
   before it accepts package inputs. A shadow-source fixture proves a
   `Process()` surface fails with the closed message
   `forbidden native app source surface`.

## Same-contract strengthenings

- nonrecoverable daemon errors now enter an explicit `fatal` UI state;
  recoverable transport errors remain offline and cursor/gap errors remain
  stale/recoverable;
- the app installer validates every bundle descendant's owner and exact
  directory/file mode;
- install and rollback transactions restore controlled current/previous state
  from the private transaction root on `HUP`, `INT`, `TERM`, command failure,
  or injected post-swap failure;
- a deterministic `TERM`-after-swap fixture proves both bundle digests are
  unchanged.

## Verification

```text
cd apps/macos && swift test
cd apps/macos && swift build -c release
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
git diff --check
git diff --cached --check
```

All commands passed. Swift now reports `17` tests with zero failures. All `23`
Go packages and the complete race matrix passed, including the real Go
server/Swift probe. Module verification returned `all modules verified`.
Staging is empty. `swift package reset` removed all generated `.build` state.

No native app or Candidate daemon was installed or launched. No resident
daemon, Journal, Provider, Runtime, credential, staging, Reopen 4 accounting,
or P2A-W2 state changed.
