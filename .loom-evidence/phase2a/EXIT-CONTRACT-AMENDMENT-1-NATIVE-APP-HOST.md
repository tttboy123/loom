# Phase 2A Exit Contract Amendment 1: Native App Host

**Date**: 2026-07-28
**Status**: FROZEN — fresh independent Contract Re-review `PASS`
**Parent**: `Phase 2A Local Product Experience Exit Contract`
**Decision**: proposed ADR-0012
**WorkItem impact**: P2A-W1 only; no P2A-W4

## 1. Evidence-led need

P2A-W1 Reopen 4 bootstrapped the reviewed Candidate and passed all non-UI live
gates. Finder Computer Use opened the exact installed `Loom.command`, but the
Computer Use safety boundary prohibited reading or operating
`com.apple.Terminal`. The canary rolled back exactly and its independent
Result-Evidence Review passed.

The failure does not justify weakening Computer Use evidence, parsing CLI
output, or adding another point Reopen. It proves that the Terminal-hosted
launcher is not sufficient for the Exit Contract's ordinary-user, no-terminal
product requirement.

## 2. Exit interpretation

The following interpretations are binding if this Amendment passes Review:

1. PX-01 requires an independently addressable local product window. Opening
   Terminal is not completion.
2. PX-02 through PX-05 may be rendered by the native app or Bubble Tea client,
   but both must use the same typed daemon API and return the same canonical
   view, selection, cursor, error, and empty-state semantics.
3. PX-16 requires a reproducible user-level `.app` bundle and a resident daemon
   package. The app is a client; installing or opening it grants no execution
   authority.
4. PX-18 Computer Use evidence operates the native app window. CLI, PTY, test
   renderer, archived Journal, or screenshot-only evidence cannot substitute.
5. The three frozen vertical WorkItems remain exactly P2A-W1, P2A-W2, and
   P2A-W3. The native app host is a W1 product file set, not another WorkItem.

## 3. Preserved authority

The native app must:

- call the private versioned Unix-domain-socket API directly;
- use the same application read service, Projection, GlobalReadView, timeline,
  and Event Journal as the Go clients;
- remain read-only during W1;
- keep only replaceable in-memory view copies;
- fail closed on socket identity, version, frame, schema, timeout, cursor,
  peer, state-unavailable, and projection errors;
- send no Provider credential or environment value;
- render no raw secret, terminal escape, bidi control, hidden reasoning, raw
  Grant, or internal filesystem path.

It must not:

- invoke or parse `loom`, `loomd`, a shell, a bridge script, `launchctl`, or a
  Runtime process;
- inspect or write SQLite;
- discover a repository workspace;
- persist snapshot, timeline, draft, Run, Evidence, or authority cache;
- create a second state writer, scheduler, queue, Journal, projection, broker,
  or daemon;
- change W2/W3 mutation authority or activate a Provider/Runtime.

## 4. Historical Cockpit boundary

The external historical `Loom Cockpit.app` and source snapshot are
evidence/reference-only.

Their visual hierarchy may inform layout, but the following are forbidden:

- `Process`-based bridge execution;
- `loom-cockpit-bridge` or product-runtime-host scripts;
- workspace discovery or `LOOM_WORKSPACE`;
- historical Snapshot/Approval/Run domain types;
- filesystem `CacheStore` or other client persistence;
- historical provider, task-activation, or runtime-host commands.

No external historical file may be copied wholesale into the Candidate.
Reviewer-visible provenance and a semantic rewrite are required for any small
visual fragment reused.

## 5. Live invocation boundary

Reopen 4 remains consumed and failed. This Amendment does not retroactively
change that result and does not authorize another Terminal-hosted canary.

Only after:

1. this Exit Contract Amendment passes independent Review;
2. the exact W1 contract revision passes independent Review;
3. TDD RED and implementation close the native app boundary;
4. complete Go and Swift verification passes;
5. fresh independent Implementation Review passes; and
6. the user supplies a new explicit post-Review activation,

one native-app controlled live canary may install the reviewed bundle and
Candidate daemon, operate the native window through Computer Use, perform the
frozen read/relaunch/restart checks, and preserve only on full PASS.

No live authority exists at Contract Review, RED, implementation, or
Implementation Review.

## 6. Exit

This Amendment passes only if the Reviewer proves it:

- resolves the no-terminal contradiction vertically inside W1;
- preserves ADR-0011's one-authority and direct-IPC rules;
- does not import the historical Cockpit authority model;
- does not create W4 or a second daemon/client state authority;
- does not silently grant another live invocation.
