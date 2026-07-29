# P2A-W1 Native App Host Live Pre-Bootstrap Repair 1 GREEN

Date: 2026-07-28  
Status: `GREEN — PENDING FRESH INDEPENDENT IMPLEMENTATION REVIEW`  
Live allowance: `1`, unconsumed

## Repair

The existing local-product installer now treats each current and previous
installation generation as one of three exact shapes:

```text
empty
loom + loomd
loom + loomd + Loom.command
```

It still rejects a lone binary, a launcher without both binaries, symlinks,
non-user-owned paths, incomplete generations, and unsafe roots.

Installing over the accepted two-binary resident state:

1. preserves both exact prior binaries as `.previous`;
2. creates the reviewed current compatibility launcher;
3. does not fabricate a launcher for the prior generation.

Rollback now swaps the optional launcher independently from the mandatory
binary pair. A Candidate triple rolls back to the exact launcher-absent legacy
pair while retaining the Candidate triple as the new previous generation; a
second rollback returns to the Candidate triple and restores the legacy pair
without a fabricated previous launcher.

Implementation Review 1 found that generic signal traps cleaned scratch paths
but did not restore and terminate an active install transaction. Review Repair
adds separate `EXIT` cleanup and `HUP`/`INT`/`TERM` handlers to both install and
rollback paths. Once live mutation begins, either a signal or any unexpected
exit restores the complete snapshot. Signal handlers ignore reentrant signals,
clean transaction scratch, and terminate with fixed status `98`; they never
resume the mutation body.

The change is limited to the Native App Host Revision's already-owned files:

```text
scripts/install-loom-local-product.sh
scripts/test-install-loom-local-product.sh
```

No daemon, app, IPC, Journal, Projection, StateWriter, Runtime, Provider,
authorization, or scheduler behavior changed.

## Deterministic proof

Focused repair proof:

- pre-repair legacy fixture: `RED`, `incomplete current installation`;
- post-repair legacy dry-run: byte-for-byte no mutation;
- legacy pair to Candidate triple install: `PASS`;
- exact pair/triple optional-launcher rollback swap in both directions:
  `PASS`;
- deterministic `TERM` after the first install swap: exact digest restoration
  and exit status `98`;
- deterministic `TERM` after the first rollback swap: exact digest restoration
  and exit status `98`;
- existing clean install, update, injected mid-transaction failure, rollback,
  TUI launch fixture, symlink attack, and secret-negative checks: `PASS`;
- exact signed-off live resident dry-run: `PASS`, byte-for-byte no mutation;
- shell syntax: `PASS`.

Complete Native App Host verification:

- Swift tests: `17 passed`;
- Swift release build: `PASS`;
- native bundle build fixture: `PASS`;
- native app installer fixture: `PASS`;
- local-product installer fixture: `PASS`;
- `go test -count=1 ./...`: `PASS`;
- `go test -race -count=1 ./...`: `PASS`;
- `go vet ./...`: `PASS`;
- `go mod verify`: `all modules verified`;
- `git diff --check`: `PASS`;
- `git diff --cached --check`: `PASS`;
- staging: empty.

## Live-state proof

The failed live attempt stopped at dry-run before mutation and did not consume
the allowance. After the repair and exact live dry-run:

- original binary, wrapper, plist, and SQLite hashes remain exact;
- original observer remains loaded and running;
- Journal integrity is `ok`, Event count is `1`, and canonical digest is
  unchanged;
- product run directory, sockets, compatibility launcher, and native app
  remain absent;
- no Provider value was emitted or changed;
- no app/daemon install, bootout, bootstrap, Computer Use, Journal write,
  Runtime execution, Team creation, or authority transition occurred.

Fresh independent Implementation Review is mandatory before any new live
activation. This GREEN does not reuse the pre-repair activation.
