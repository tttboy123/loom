# P2A-W2 Mission Orchestration Live Closure Reopen Contract

**Date**: 2026-07-30
**Status**: FROZEN — fresh independent Contract Review PASS
**Authority**: Product Owner authorization for subsequent in-scope actions
without another confirmation
**Parent**:
`mission-orchestration-workbench-exit-contract.md`
**Reopens**: only the complete failed P2A-W2 live exit
**Baseline**: `e77a117ee990ac9d4095376383d22c66ec9755df`
**Candidate**: `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`
**Risk**: STRICT — one replacement daemon/native-window/TUI lineage with real
Journal-backed prepared authority commands

## 1. Authorization, precedence and historical truth

The Product Owner authorized all subsequent in-scope actions without another
confirmation. This contract uses that authority to reopen the complete P2A-W2
live closure once. It does not create a point Amendment, W2a/W2b or P2A-W4.

The first Mission Workbench lineage
`p2a-w2-mission-workbench-live-20260730-001` remains immutably:

```text
FAIL — daemon construction returned exit 3 / daemon unavailable
```

Its reviewed result, 72-Event fixture state and all post-stop evidence are not
reclassified, overwritten, reused as fresh state or deleted.

This reopen:

- changes no committed product source;
- changes no Event, Grant, Evidence, StateWriter, Projection, Scheduler,
  Runtime, Provider, credential or local IPC authority;
- does not relax the native-auth regular-file executable boundary;
- does not teach the product to follow symlinks;
- corrects the controlled preflight input by resolving and freezing the
  canonical regular Codex executable before any daemon start;
- carries the complete original 15-step vertical native-window canary;
- permits exactly one replacement lineage after every gate below passes.

If committed product bytes must change, an accepted authority must expand, the
canonical executable cannot be frozen safely, or any gate fails, this contract
stops `HUMAN_REQUIRED`.

## 2. Exact owned boundary

Repository writes are limited to:

- this contract and its independent review;
- one deterministic/preflight verification record;
- one preflight implementation review;
- one replacement live result and one independent Result-Evidence Review;
- `mission-orchestration-workbench-candidate-manifest.json`;
- `docs/CURRENT.md`.

External controlled writes are limited to one fresh attempt root:

```text
/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-live-20260730-002
```

No file under `cmd/`, `internal/`, `apps/`, `TECH-PLAN.md`, `AGENTS.md`,
`PROGRESS.md`, `README.md`, Phase 1 evidence, `.codex/`, `.loom-drafts/` or
`apps/macos/.build/` is owned. Existing dirty paths remain user-owned and
unstaged.

## 3. Frozen Candidate identity

The replacement lineage must use the unchanged committed Candidate:

- Candidate commit:
  `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`;
- 31-file source lock:
  `de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`;
- Implementation Review 6: `PASS`;
- Visual Review 3: `PASS`;
- prior deterministic verification matrix: `PASS`.

Before live execution the Controller must reproduce:

- every source-lock entry and the combined digest;
- no diff in committed product source;
- exact daemon, TUI and signed native executable hashes built from the
  Candidate commit;
- the complete deterministic Go, repository race, vet, Swift debug, Swift
  Thread Sanitizer and Swift Release matrix;
- the focused real Go IPC to strict Swift fixture;
- the focused prepared Authorization/Review/Recovery and no-hidden-retry
  tests.

No earlier PASS is transferred if any locked byte or verification command
differs.

## 4. Canonical Codex executable transaction

Before the replacement daemon starts, the Controller must:

1. begin from the configured user-level Codex entry
   `/Users/lune/Documents/Codex/devtools/npm/bin/codex`;
2. resolve it exactly once with a read-only canonicalization operation;
3. require the resolved value to be absolute, canonical, user-owned, a regular
   file and executable;
4. require `Lstat` on the resolved value to report a regular file, not a
   symlink;
5. freeze device, inode, owner, mode, size and SHA-256;
6. perform one bounded status-only `login status` observation before daemon
   start;
7. require exactly one of stdout or stderr to contain one accepted complete
   status line, with the other stream empty;
8. recheck the same file identity immediately before daemon start;
9. pass only this canonical regular path through `--codex-executable`.

The status-only observation may report logged in or not logged in. It may not
start login, mutate OAuth, read tokens into evidence or weaken the observer.
Unknown, dual-stream, multiline, identity-drift or execution failure stops
before canary consumption.

The configured symlink itself remains unchanged. No install, update, copy,
wrapper or alternate Codex binary is allowed.

## 5. Fresh replacement lineage

The Controller must create exactly:

```text
attempt_id = p2a-w2-mission-workbench-live-20260730-002
```

The attempt root and all owned directories are user-owned `0700`. The new
SQLite file and manifest are regular, user-owned `0600`. The state starts
fresh; the failed lineage database and artifacts are read-only historical
evidence and are not copied into the replacement.

The controlled manifest must:

- use schema version `1`;
- use purpose `p2a-w2-mission-workbench-controlled-live`;
- bind the exact attempt ID, state path and fresh artifact root;
- bind Candidate commit `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`;
- use a fixed UTC authoritative time not in the future;
- use a new fixture ID;
- be private, canonical and symlink-free.

Before start, the default product socket and product socket lock must be
absent, the separate resident Runtime observer must be identified and left
untouched, the isolation directory must be empty, and the failed lineage must
remain unchanged.

## 6. One complete replacement canary

After fresh Contract Review `PASS`, deterministic/preflight verification
`PASS` and fresh read-only Preflight Implementation Review `PASS`, exactly one
daemon start is allowed. It must complete the original vertical proof:

1. start one exact controlled product daemon;
2. open the real signed native app through native computer interaction;
3. open the Workspace Orchestration Board;
4. view the real Journal-backed Mission fixture;
5. inspect Team Pulse and Topology;
6. open Mission Room, change its local draft and Inspector state, return to
   Board and prove restoration;
7. reach Provider Manage and perform the bounded Provider check through the
   product UI;
8. prove `Not now` and `Deny` produce no execution fact;
9. prove one legal prepared authorization produces its existing authoritative
   confirmation;
10. prove missing Evidence cannot complete;
11. prove prepared Recovery creates a fresh Attempt and generation;
12. prove native GUI and real TUI show the same Mission/Team/Node/Attempt
    state;
13. prove no secret, raw Grant or hidden reasoning is visible or persisted;
14. close the TUI, native app and daemon normally;
15. prove product socket/lock cleanup, no orphan process, empty isolation,
    SQLite integrity, source immutability and the exact bounded Event delta.

GUI actions use the native computer-interaction surface. TUI interaction uses
the real built TUI in a terminal window. Terminal commands, direct IPC calls or
SQLite edits may gather read-only evidence but may not substitute for a
required product interaction.

## 7. No retry and stop rules

This contract permits one replacement daemon start only. There is no:

- second replacement;
- daemon restart;
- alternate executable after start;
- reuse or manual repair of partial state;
- direct SQLite mutation;
- manual product socket/lock deletion;
- terminal-as-GUI bypass;
- hidden retry;
- new single-point Amendment.

A pre-consumption check may stop without consuming the lineage only if no
daemon process was started and no controlled fixture side effect occurred.
Once the daemon process is invoked, any failure consumes the replacement and
stops `HUMAN_REQUIRED`.

## 8. Exit

This reopen and the Phase 2A Extra Goal pass only when:

- every GoalSpec acceptance item remains backed by the committed Candidate and
  deterministic evidence;
- the complete replacement canary passes all 15 steps;
- fresh independent Result-Evidence Review returns `PASS`;
- the failed first lineage remains immutable;
- excluded dirt remains unmodified and unstaged;
- result evidence and `docs/CURRENT.md` are atomically committed.

Only then may P2A-W2 be marked `ACCEPTED` and the active Extra Goal be marked
complete. P2A-W3 remains locked until that point; P2A-W4 does not exist.
