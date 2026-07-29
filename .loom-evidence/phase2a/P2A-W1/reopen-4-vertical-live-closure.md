# P2A-W1 Reopen 4: Vertical Live Closure

**Date**: 2026-07-28
**Status**: `FROZEN — REPAIR A CONTRACT RE-REVIEW PASS`
**Parent**: frozen P2A-W1 Local App Shell and Read Experience Contract
**WorkItem count**: unchanged; this remains P2A-W1

## 1. Why one combined Reopen is required

Reopen 3's final controlled canary bootstrapped the exact reviewed Candidate
and then rolled back before the TUI because its aggregate live validator
returned false. Read-only diagnosis and independent Result Review proved a
live-harness defect:

```text
ps -eww -p <pid> -o command=
```

On this macOS host, `-e` selected every process. The secret-negative scan
therefore examined more than one thousand unrelated rows. The BSD modifier
form:

```text
ps eww -p <pid> -o command=
```

selects exactly the requested process and includes its effective environment.
The restored observer produces one row and no Provider marker with that form.
No process row or environment value was printed or persisted.

The same evidence audit found a second live-methodology contradiction. The
installed resident Journal currently contains exactly one
`RuntimeInstanceDiscovered` Event and no saved Team, TeamExecution, WorkItem,
Run, Grant, or Evidence fact. Section 16 nevertheless required the installed
TUI to display the accepted Phase 1 historical Team timeline.

The W1 Candidate already implements the Exit Contract's actual compatibility
rule: a terminal, headed, non-empty, fully related TeamExecution that lacks a
saved-Team projection becomes a read-only
`historical_execution_only` timeline anchor, while malformed, nonterminal,
empty, cross-Team, or partially related data fails closed. Deterministic
real-SQLite/real-UDS tests and Implementation Review 3 passed that behavior.
The current resident Journal does not contain such an execution to anchor.

Fabricating a Team, copying an evidence database into the resident Journal,
merging two Journals, reading two product databases, or changing the active
authority would violate ADR-0011 and the Phase 2A Exit Contract. Reopen 4
therefore closes both live blockers together:

1. exact target-process environment attribution; and
2. truthful empty-Team/TUI behavior for the actual resident Journal, while
   preserving the already-reviewed historical compatibility path.

No Reopen 5, W1a/W1b, W4, screen-only WorkItem, adapter WorkItem, migration
WorkItem, or second live-harness WorkItem may be created.

## 2. Exact supersession

This Reopen supersedes only:

1. section 16 step 6's ambiguous process-environment inspection method;
2. section 16 step 7's requirement to display a historical Team timeline when
   the exact resident Journal has no Team or TeamExecution fact; and
3. Reopen 3's consumed-canary stop, for at most one explicitly activated
   replacement canary after all Reopen 4 gates pass.

Every other P2A-W1 authority, exclusion, hash, IPC, installation, rollback,
secret, Journal, Projection, Reviewer, no-retry, and sequencing rule remains
binding.

## 3. Owned boundary

Reopen 4 may modify only the already-owned W1 files:

```text
internal/tui/model.go
internal/tui/model_test.go
docs/CURRENT.md
```

It may add only:

```text
scripts/p2a-w1-live-evidence.sh
scripts/test-p2a-w1-live-evidence.sh
.loom-evidence/phase2a/P2A-W1/reopen-4-*.md
```

No Journal, Projection, API schema, IPC schema, daemon, installer, StateWriter,
Team, WorkPackage, Scheduler, Supervisor, Grant, Evidence, credential,
Runtime, or Provider file is reopened.

### Repair A owned-boundary amendment

Post-GREEN repeated verification reproduced a pre-existing socket-cleanup
identity defect. Reopen 4 additionally owns only:

```text
internal/localipc/socket.go
internal/localipc/socket_test.go
internal/localipc/server_test.go
```

`internal/localipc/server.go`, the IPC protocol/API, socket path policy, peer
identity, connection lifecycle, and all other files remain closed.

## 4. Target-process evidence contract

The durable live-evidence helper must:

1. accept exactly one decimal PID;
2. reject zero, missing, signed, whitespace, or nondecimal input;
3. execute only:

   ```text
   /bin/ps eww -p <pid> -o command=
   ```

4. require exactly one non-empty row;
5. perform a non-emitting, case-insensitive scan for the frozen Provider and
   credential marker classes;
6. return success only when the exact target row is secret-negative;
7. never print the row, environment, matched text, secret value, or process
   arguments;
8. expose only a closed status such as `clean`, `invalid_pid`,
   `target_unavailable`, or `forbidden_marker`;
9. have a deterministic test with a fake `ps` executable proving:
   - the argument vector is exactly `eww -p PID -o command=`;
   - a single clean row passes;
   - zero or multiple rows fail;
   - a forbidden marker fails without echoing its value;
   - the former `-eww` form is absent.

The controlled canary must call the helper with `/bin/ps` locked by exact
path. Tests may inject a private fake executable only through the helper's
test-only entry point; live execution cannot override `/bin/ps`.

Service-manager metadata attribution remains exactly Reopen 3:

- clean disk plist;
- reviewed `env -i` wrapper;
- exact five ambient/service marker names;
- non-emitting unchanged-value comparisons;
- zero Candidate child inheritance;
- no `setenv`, `unsetenv`, credential mutation, or value persistence.

### Type-aware socket cleanup

Repeated verification of
`TestServerReclaimsOnlyStaleSocketAndPreservesReplacement` exposed an APFS
inode-reuse window. The test removes the live socket path and creates a regular
replacement. APFS may immediately reuse the socket's device/inode pair for the
regular file. The current `fileIdentity` contains only device and inode, so
cleanup can misidentify the replacement as the original socket.

Repair A must:

1. bind file identity to device, inode, and `Lstat` file kind;
2. distinguish socket, regular file, symlink, directory, and other file kinds
   even when device/inode are equal;
3. preserve the existing exact-identity behavior for a still-identical socket
   or lock;
4. return the closed `ErrInvalidSocketPath` classification and preserve any
   type-changed replacement;
5. add a deterministic fake-`FileInfo` RED proving equal device/inode with
   socket versus regular kind are unequal;
6. keep the existing real replacement test and pass it repeatedly:

   ```text
   go test ./internal/localipc \
     -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
     -count=100
   go test -race ./internal/localipc \
     -run '^TestServerReclaimsOnlyStaleSocketAndPreservesReplacement$' \
     -count=30
   ```

No retry loop, inode polling, replacement deletion, recursive cleanup, relaxed
error assertion, or test timeout increase is permitted.

## 5. Truthful empty-Team product behavior

When the snapshot contains no Team summaries:

1. the Teams screen must state that the current Journal has no Teams yet;
2. pressing Enter performs no timeline request and causes no mutation;
3. opening the Team Timeline tab directly must instruct the user to select a
   Team from the Teams screen rather than displaying a fabricated empty
   authoritative timeline;
4. Home must remain truthful about zero Teams;
5. Runtime, Runs, Evidence, Compare, and Attention remain independently
   navigable;
6. no internal ID, SQLite path, cursor, socket, or service command is requested
   from the user.

When an exact saved-Team or accepted
`historical_execution_only` summary exists, the existing typed selection and
timeline request behavior remains unchanged.

This behavior does not mark the missing Team as partial, synthesize a Team,
infer a Team ID from unrelated Runs, or weaken `deriveRelatedScope`.

## 6. RED and GREEN

Mandatory RED:

```text
go test ./internal/tui \
  -run 'TestModelTruthfullyHandlesJournalWithNoTeams' -count=1
scripts/test-p2a-w1-live-evidence.sh
go test ./internal/localipc \
  -run '^TestFileIdentityIncludesFileKind$' -count=1
```

RED must fail only because the current TUI uses generic empty/timeline text and
the reviewed helper does not yet exist, plus the existing file identity treats
equal device/inode socket and regular fixtures as identical.

Minimal GREEN:

```text
gofmt -w internal/tui/model.go internal/tui/model_test.go
go test ./internal/tui \
  -run 'TestModelTruthfullyHandlesJournalWithNoTeams' -count=1
scripts/test-p2a-w1-live-evidence.sh
go test ./internal/localipc \
  -run '^TestFileIdentityIncludesFileKind$' -count=1
```

The full W1 verification matrix and fresh independent Implementation Review
must then pass. No live action is part of RED, GREEN, or Review.

## 7. Replacement controlled canary

Only after:

1. independent Reopen 4 Contract Review `PASS`;
2. captured RED;
3. GREEN and complete W1 verification;
4. fresh independent Implementation Review `PASS`; and
5. explicit post-Review user activation,

one replacement canary may repeat the reviewed atomic install/rollback
transition.

The replacement must:

1. revalidate exact original and Candidate hashes, modes, SQLite integrity,
   Event count, and canonical heads;
2. use the durable target-process helper and Reopen 3 ambient attribution;
3. open the installed no-argument TUI only after all non-UI gates pass;
4. use Computer Use to inspect:
   - the actual Pi Runtime;
   - the truthful no-Team Journal state;
   - the direct Team Timeline selection instruction;
   - Runs, Evidence, Compare, and Attention empty states;
5. press Enter on the empty Teams screen and prove no timeline IPC request,
   Journal write, or state change;
6. quit and relaunch, then perform the one explicit daemon restart and recover
   the same view version and empty-Team state;
7. preserve the Candidate only if every gate passes; otherwise restore exact
   pre-state.

The canary must not open an archived evidence database, copy or merge Journal
facts, switch the resident state path, create a Team, type a Team ID, run a
Provider/model/Runtime, or claim the deterministic historical fixture exists
in the resident Journal.

## 8. Invocation accounting

Reopen 3's final bootstrap remains consumed and its failed evidence is
preserved. Reopen 4 may authorize exactly one replacement bootstrap only after
the activation boundary below. A pre-install guard syntax failure does not
consume the allowance; any Candidate bootstrap does.

There is no hidden retry. A failed Reopen 4 replacement stops
`HUMAN_REQUIRED`.

## 9. Repair A review boundary

The original Reopen 4 Contract Review remains evidence for the TUI/helper/live
methodology, but it predates Repair A and cannot authorize the IPC edit.
Repair A requires a fresh independent supplemental Contract Review before its
RED or implementation. That Review does not activate live execution.

## 10. Activation boundary

Contract Review, RED, implementation, deterministic verification, and
Implementation Review do not activate the replacement canary.

After Implementation Review `PASS`, live execution requires an explicit user
message authorizing:

```text
P2A-W1 Vertical Live Closure Reopen 4 and one replacement controlled canary
```

Until that post-Review activation exists, the installed original observer and
state remain unchanged, P2A-W1 remains `HUMAN_REQUIRED`, and P2A-W2 remains
locked.
