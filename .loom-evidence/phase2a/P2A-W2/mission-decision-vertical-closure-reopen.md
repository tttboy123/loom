# P2A-W2 Mission Decision Vertical Closure Reopen

**Date**: 2026-07-30
**Status**: FROZEN — implementation and live execution locked
**Authority**: Product Owner standing authorization recorded in
`docs/CURRENT.md`
**Parent**: `mission-orchestration-workbench-exit-contract.md`
**Reopens**: the complete remaining P2A-W2 native decision exit
**Baseline**: `bd71cb79c6242bcf8e51cbb5d9d73d7b6185219a`
**Risk**: STRICT — production client composition plus one complete
Journal-backed native decision lineage

## 1. Historical truth and precedence

This is a complete P2A-W2 reopen, not a point Amendment, W2a/W2b or P2A-W4.
It preserves the accepted deterministic Candidate and both consumed live
results unchanged:

```text
p2a-w2-mission-workbench-live-20260730-001
FAIL — daemon unavailable

p2a-w2-mission-workbench-live-20260730-002
FAIL — NATIVE_DECISION_CLIENT_PROTOCOL_UNAVAILABLE
```

Attempt 002 Result-Evidence Review passed only for the failed classification.
No prior live failure is reclassified, overwritten, retried in place or reused
as fresh state.

The blocking product composition is exact:

```text
LocalIPCClient implements readMissionDecision and decideMission
LocalIPCClient does not declare LocalProductDecisionClientProtocol
LocalProductStore obtains its decision client through a conditional cast
the real native client therefore yields nil
```

This reopen may repair that complete vertical composition and then re-prove the
remaining native Authorization, Review and Recovery exit. It may not broaden
decision authority, weaken IPC decoding or replace prepared commands.

## 2. Exact owned product boundary

Product writes are limited to:

- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`;
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`.

Governance/evidence writes are limited to:

- this contract and its fresh Contract Review;
- one RED record;
- one deterministic GREEN record;
- one fresh Implementation Review;
- one complete live result and one fresh Result-Evidence Review;
- `mission-orchestration-workbench-candidate-manifest.json`;
- an updated source lock for the accepted repaired Candidate; and
- `docs/CURRENT.md`.

No other product file is owned. In particular this reopen does not own:

- `LocalProductStore.swift`;
- Mission UI or decision-sheet UI;
- Go IPC server, Journal, Projection, StateWriter, Rules, Work, Grant,
  Evidence, Runtime, Provider or credential code;
- `TECH-PLAN.md`, `AGENTS.md`, `PROGRESS.md`, `README.md`;
- Phase 1 evidence, `.codex/`, `.loom-drafts/` or user-owned cache/quarantine
  content.

Existing unrelated dirty paths remain unmodified and unstaged.

## 3. Required RED and implementation semantics

Before production change, the Swift test must prove that a real
`LocalIPCClient`, erased to `LocalProductClientProtocol` exactly as the app
does, cannot currently be recovered as
`LocalProductDecisionClientProtocol`.

The RED must use a private owned Unix socket accepted by the real client
initializer. A stub that already declares the decision protocol is
insufficient.

The production repair is limited to declaring the existing protocol
conformance. It may not:

- change a decision request or response shape;
- alter timeout, socket, peer, frame, strict JSON or unknown-key validation;
- add fallback dispatch, direct Journal access or hidden retry;
- change `LocalProductStore` conditional-cast behavior;
- make an unprepared Review action authoritative;
- authorize `Not now`, disabled actions or stale commands; or
- expose raw credentials, Grants or hidden reasoning.

The focused test must turn GREEN and prove the real production composition.

## 4. Deterministic verification and Review gate

Before any live process, the repaired Candidate must pass:

1. focused Swift RED-to-GREEN composition test;
2. complete Swift debug tests;
3. complete Swift Thread Sanitizer tests;
4. Swift Release build;
5. real Go IPC server to strict Swift client fixture;
6. focused prepared Authorization, Review, Recovery, stale-view,
   stale-generation, replay and concurrent-winner tests;
7. `go test -count=1 ./...`;
8. `go test -count=1 -race ./...`;
9. `go vet ./...`;
10. product source-lock verification and `git diff --check`.

All SwiftPM output must use a fresh attempt-local scratch path. The repository
`apps/macos/.build` path is not created, restored, reused or queried. The
reviewed Attempt 002 quarantine is immutable historical user data.

A fresh independent Implementation Reviewer must return `PASS` with no P0/P1
before live. It must verify exact scope, real production-client coverage,
strict fail-closed behavior and full matrix evidence.

## 5. Fresh complete lineage

Only after Contract Review and Implementation Review pass may the Controller
create:

```text
attempt_id =
p2a-w2-mission-decision-live-20260730-003

attempt_root =
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-decision-live-20260730-003
```

The attempt root and directories are private `0700`. State and manifest files
are regular private `0600`. State starts fresh from the controlled semantic
fixture; no prior database, socket, lock, artifact or isolation residue is
copied.

The exact repaired commit, source lock, daemon, TUI, signed native executable,
manifest, canonical Codex executable and installed Pi identity must be frozen
before daemon start. Default product socket and lock must be absent. The
unrelated resident observer remains untouched.

## 6. One complete native decision canary

Exactly one daemon start and one native bundle invocation are allowed after all
gates pass. The complete lineage must prove:

1. the real signed app connects to the one controlled daemon;
2. Board, Mission list, Mission Room continuity, Inspector and Team Pulse
   remain intact;
3. Provider Manage remains reachable and does not expose a secret;
4. a real prepared Authorization sheet opens through
   `LocalIPCClient -> Go IPC -> prepared decision backend`;
5. `Not now` closes presentation with no Journal mutation;
6. a prepared `Deny` appends only its bounded authoritative denial facts and
   produces no execution start;
7. a separate prepared `Allow once` appends its authoritative confirmation and
   advances only the bound Mission/Attempt;
8. the missing-Evidence Review Gate remains read-only and cannot accept;
9. the prepared Review sheet with accepted Evidence can perform only its exact
   prepared terminal action;
10. prepared Recovery creates exactly one fresh Attempt with increased
    attempt number and generation;
11. stale view/generation and replay remain rejected with no hidden retry;
12. native GUI and real TUI show the same Mission/Team/Node/Attempt state;
13. no secret, raw Grant or hidden reasoning is visible or persisted;
14. TUI, app and daemon close normally; and
15. socket/lock cleanup, no orphan process, empty isolation, SQLite integrity,
    source immutability and exact Event delta pass.

Read-only IPC, CLI and SQLite diagnostics may corroborate the product journey
but may not substitute for required native actions. No decision mutation may
be invoked outside the native product UI.

## 7. No retry and stop rules

This complete reopen allows one fresh lineage only. It does not allow:

- restart or second live attempt;
- alternate binary or manifest after start;
- hot repair of a running bundle;
- direct IPC mutation;
- direct SQLite mutation;
- manual product socket/lock deletion;
- hidden retry;
- a point Amendment or P2A-W4.

If product source outside the exact owned boundary must change, a deterministic
or Review gate fails, the fresh state is not isolated, or any required native
decision step fails after daemon start, stop `HUMAN_REQUIRED`.

## 8. Exit

P2A-W2 and the active Extra Goal pass only when:

- the real production client composition is covered and accepted;
- the complete deterministic matrix passes;
- fresh Implementation Review passes;
- the one complete native decision canary passes all 15 steps;
- fresh Result-Evidence Review passes;
- both prior failed lineages remain immutable;
- unrelated dirty state remains unmodified and unstaged; and
- the repaired Candidate and its evidence are atomically committed.

Only then may P2A-W2 be marked `ACCEPTED`, P2A-W3 become the next eligible
WorkItem and the active Extra Goal be audited for completion.
