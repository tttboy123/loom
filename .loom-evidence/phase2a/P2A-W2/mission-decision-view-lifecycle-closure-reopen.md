# P2A-W2 Final Mission Decision View Lifecycle Closure Reopen

**Date**: 2026-08-01

**Status**: FROZEN — Contract Review PASS; RED unlocked, live locked

**Authority**: Product Owner standing authorization and active Phase 2A Extra Goal

**Parent**: `mission-orchestration-workbench-exit-contract.md`

**Reopens**: the complete remaining P2A-W2 decision lifecycle exit

**Repository baseline**: `595c3132eb348153bf44640973fa0ce7e152af47`

**Reviewed product Candidate**: `700086d`
**Risk**: STRICT — authoritative snapshot coherence and prepared-command fencing

## 1. Historical truth and precedence

This is one complete P2A-W2 lifecycle reopen. It is not a point Amendment,
W2 subdivision, wrapper WorkItem or P2A-W4. It preserves the consumed lineage
unchanged:

```text
p2a-w2-mission-decision-live-20260730-003
FAIL — PREPARED_DECISION_VIEW_STALE_AFTER_DISCOVERY
```

The fresh Result-Evidence Review accepted only the accuracy of that failed
classification. It did not accept P2A-W2 or authorize reuse of Attempt 003.

The exact root cause is frozen:

1. the controlled fixture creates real prepared Authorization, Review and
   Recovery commands against a valid GlobalReadView;
2. the observer's first Runtime discovery commits a later Journal fact;
3. a fresh product snapshot rebuilds and publishes the later GlobalReadView;
4. the prepared command registry still publishes the earlier view version;
5. submission correctly refreshes the current view and rejects the stale
   command before authority writes.

Stale-command rejection is correct and must remain. The missing capability is
an atomic read-side lifecycle transaction that binds every still-prepared
command to the exact successful snapshot view that publishes it.

## 2. Exact owned product boundary

Product writes are limited to:

- `internal/app/local_product_decision.go`;
- `internal/api/local_product_read.go`.

Test writes are limited to:

- `internal/app/local_product_decision_test.go`;
- `internal/api/local_product_read_test.go`;
- `cmd/loomd/product_daemon_test.go`.

Governance and evidence writes are limited to this contract, its reviews and
RED/GREEN/live records, the P2A-W2 source lock/manifest, and
`docs/CURRENT.md`.

This reopen does not own or modify:

- Event, Journal, StateWriter, Projection, Runtime discovery, Rules, Work,
  Grant or Evidence schemas or authorities;
- `cmd/loomd/product_daemon.go` or local IPC protocol/server methods;
- Swift production code, strict decoders or Mission UI;
- Provider, credential, Keychain, Codex or Pi installation/authentication;
- `PRODUCT-PLAN.md`, `TECH-PLAN.md`, Phase 1 evidence, `.codex/`,
  `.loom-drafts/` or unrelated dirty paths.

## 3. Frozen lifecycle semantics

The read facade may request prepared commands for one exact view using an
explicit query carrying:

- the expected canonical GlobalReadView version; and
- either `refresh_current` or `preserve_stale` mode.

The modes have the following semantics.

### 3.1 Successful projection — `refresh_current`

After Projection rebuild succeeds, `ReadLocalProductSnapshot` passes that
exact view version to the prepared-command source. The source must:

1. refresh every unconsumed entry using its existing Journal-backed refresher;
2. require every refresh to equal the expected snapshot view;
3. reject an in-flight entry, malformed digest, refresh error or mismatch;
4. mutate no entry unless all entries pass; and
5. then rebind every unconsumed sheet to that one expected version and return
   deep-copied commands.

The resulting invariant is:

```text
snapshot.view_version == every prepared_decision.view_version
```

This is read-side optimistic-concurrency metadata only. It writes no Journal
fact, makes no authority decision and does not consume a command.

### 3.2 Projection failure — `preserve_stale`

If Projection refresh fails after a prior good snapshot, the service must
retain the last immutable view and its previously bound prepared commands.
It must not call a refresher, advance a command version or mix current commands
with the stale snapshot. If the cached commands no longer match the cached
view, the request fails closed as state unavailable.

### 3.3 Submission and fencing

Submission remains independently guarded. It must:

- exact-match the prepared sheet tuple and prepared action;
- refresh the current view immediately before authority dispatch;
- reject an old pre-rebind command, stale generation, replay, identity drift,
  concurrent loser or view advance after snapshot;
- perform no hidden retry; and
- keep the existing single-winner and post-success remaining-command update.

An old command must remain rejected even if a later snapshot has rebound the
registry. A caller must use the exact newly published command.

## 4. Mandatory RED

Before production behavior changes, causal tests must fail on the current
Candidate and prove:

1. a Journal Runtime discovery advances the authoritative view after fixture
   preparation;
2. a later successful product snapshot cannot currently publish prepared
   commands at that exact new version;
3. the old command remains rejected with zero decision-side-effect Events;
4. the new command can complete its exact prepared authority path once;
5. projection failure preserves the previous view and prepared-command bytes
   without invoking refresh; and
6. a refresh mismatch/error/in-flight entry fails atomically without partial
   rebind.

At least one vertical test must use the real
`LocalProductReadService -> Go IPC server` composition. Existing strict Swift
client fixture coverage must be rerun against the final Go server; no Swift
decoder relaxation is permitted.

## 5. Deterministic verification and reviews

Before live action, the exact Candidate must pass:

1. focused app lifecycle tests;
2. focused API successful/stale snapshot tests;
3. production daemon Go IPC composition tests;
4. real Go IPC server to strict Swift client fixture;
5. focused stale-view, stale-generation, replay and concurrent-winner tests;
6. `go test -count=1 ./...`;
7. `go test -count=1 -race ./...`;
8. `go vet ./...`;
9. Swift debug tests with attempt-local scratch;
10. Swift Thread Sanitizer tests with attempt-local scratch;
11. Swift Release build with attempt-local scratch;
12. source-lock verification, forbidden-authority scan and
    `git diff --check`.

Repository `apps/macos/.build` must remain absent. Existing quarantine and all
prior live attempt roots are immutable.

A fresh independent Implementation Reviewer must return `PASS` with no P0/P1
before live. Review must explicitly verify all-or-nothing rebind, stale-cache
preservation, old-command rejection, exact authority dispatch and owned-file
scope.

## 6. One replacement controlled lineage

Only after Contract Review and Implementation Review pass may the Controller
create one fresh isolated lineage:

```text
attempt_id = p2a-w2-mission-decision-live-20260801-004
```

The attempt root must be private `0700`; database, manifest and source-lock
copy must be regular private `0600`. It starts from a fresh controlled semantic
fixture and contains no state, socket, lock, artifact or isolation residue
from Attempt 003.

Exactly one daemon start and one signed native app invocation may prove:

1. first Runtime discovery advances the GlobalReadView;
2. the next native snapshot and every prepared Decision sheet expose that
   exact current view;
3. `Not now` remains zero-authority;
4. one native prepared Deny or Allow action succeeds exactly once;
5. the pre-discovery command, stale generation and replay are rejected;
6. Journal/Event deltas match only the selected authority transition;
7. Board, Mission Room, Team Pulse, Provider Manage, missing-Evidence Review
   Gate and TUI parity remain intact;
8. no credential, raw Grant or hidden reasoning is exposed or persisted; and
9. app, TUI and daemon exit normally with absent process/socket/lock/handle,
   empty isolation, SQLite integrity and source immutability.

Read-only IPC/SQLite diagnostics may corroborate but never substitute for the
required native decision action.

## 7. No retry and stop rules

This reopen allows one fresh lineage only. It authorizes no restart, alternate
binary, hot patch, direct IPC mutation, direct SQLite mutation, manual socket
cleanup, hidden retry, new point Amendment or P2A-W4.

Stop `HUMAN_REQUIRED` if Contract or Implementation Review fails, a product
file outside the exact boundary is required, the source cannot preserve
all-or-nothing/stale-cache semantics, isolation/preflight fails, or any live
step fails after daemon start.

## 8. Exit

P2A-W2 passes only when RED is preserved, the complete deterministic matrix
and independent Implementation Review pass, the one replacement native
lineage and fresh Result-Evidence Review pass, prior failures stay immutable,
unrelated dirt remains excluded, and the accepted Candidate/evidence are
atomically committed.

Only then may P2A-W2 become `ACCEPTED` and P2A-W3 become eligible. No P2A-W4
exists.
