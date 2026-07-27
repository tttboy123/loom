# Phase 2A ADR and Exit Contract Review

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: PASS

## Candidate

The Reviewer examined only:

- `docs/adr/0011-tui-first-local-product-over-versioned-daemon-ipc.md`
- the ADR-0011 row in `docs/adr/README.md`
- `.loom-evidence/phase2a/EXIT-CONTRACT.md`
- the Phase 2A governance hunk in `docs/CURRENT.md`

Pre-existing shared-worktree modifications and untracked files were excluded.
The Reviewer did not edit product or evidence files.

## Blocking findings

None.

## Advisory findings

### 1. W1 requires an exact child-contract freeze

P2A-W1 intentionally combines the app shell, daemon IPC, typed client, read
views, timeline compatibility, reconnect, slow-consumer handling, daemon
restart recovery, install/exit behavior, and E2E proof into one vertical
WorkItem. Its child contract must name exact owned files, imports, protocol
schema, acceptance tests, rollback, and exclusions before RED.

This is advisory because the Exit Contract already mandates that child-contract
gate and forbids a wrapper-only W4.

### 2. W2 Credential Broker details are security-sensitive

The minimum Credential Broker and OS Secret Store scope is consistent with
ADR-0004. W2 must precisely freeze Provider protocol, the OS Secret Store
adapter, revoke/replace behavior, native-auth non-import, and redaction tests.
It must not reuse the credential pasted in conversation.

This is advisory because the Exit Contract already requires a reviewed W2
child contract and secret-negative evidence.

### 3. W1 must close deferred IPC precision

The governance contract correctly freezes a private UDS, `0700` parent,
protocol version, bounded messages, deadlines, fail-closed peer access, and no
public TCP. The W1 child contract must additionally state exact socket-path
rules, stale-socket cleanup, symlink/TOCTOU defenses, peer-credential behavior
where available, and structured error mapping.

This is advisory because transport precision is explicitly delegated to the W1
child contract before implementation.

## Authority assessment

The Candidate:

- preserves ADR-0002's single daemon API and Event Journal authority;
- preserves ADR-0008's immutable read-view, stream-head CAS, and single
  scheduler/writer boundaries;
- preserves ADR-0009's authorized tentative output and terminal Evidence
  authority;
- makes exactly P2A-W1, P2A-W2, and P2A-W3 governable and prohibits P2A-W4;
- blocks product code, Provider traffic, credential mutation, daemon
  replacement, migration, and live canaries until this Review passes.

## Decision

ADR-0011 may become `accepted` and the Phase 2A Exit Contract may become
`FROZEN`. Product implementation may proceed only through the P2A-W1 child
contract gate.

VERDICT: PASS
