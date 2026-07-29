# P2A-W1 Contract Review

**Date**: 2026-07-28
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: PASS

## Candidate

The Reviewer examined:

- `.loom-evidence/phase2a/P2A-W1/contract.md`
- the latest P2A-W1 hunk in `docs/CURRENT.md`
- the frozen Phase 2A Exit Contract and ADR-0011;
- accepted ADR-0002, ADR-0004, ADR-0008, and ADR-0009;
- `TECH-PLAN.md`, `PRODUCT-PLAN.md`, current Go module metadata, and the
  existing CLI, daemon, API, Projection, and timeline code boundaries.

Pre-existing shared-worktree dirt was excluded. No file was modified by the
Reviewer.

## Blocking findings

None.

## Confirmed boundaries

- Exact ownership is sufficient and excludes Journal migrations, StateWriter,
  Scheduler, Supervisor, Grant/Evidence authority, Credential Broker, Runtime
  adapters, and installed service state before the live gate.
- The legacy execution-only anchor is terminal, read-only, exact-lineage, and
  cannot synthesize a Team fact or authorize future execution.
- UDS path, lock, stale cleanup, TOCTOU, peer UID, framing, deadlines,
  concurrency, lifecycle, and safe error requirements are implementable.
- `--socket` is optional, so the existing observer-only daemon remains backward
  compatible.
- Online CLI and TUI share the daemon API; explicit offline recovery remains
  separate and read-only.
- W1 does not absorb Team creation, Provider credentials, WorkPackage creation,
  dispatch, approval, retry, Runtime execution, or Provider/model requests.
- RED, deterministic E2E, race, repository, portability, dependency, security,
  authority, and live rollback gates are sufficient for the frozen boundary.

## Advisory findings

### 1. Independently repeat dependency verification

The Reviewer environment could not reach `proxy.golang.org`. The implementation
must reverify the frozen source, module checksums, transitive dependency diff,
and `go mod verify`. The Controller already resolved the exact tag from direct
GitHub VCS and obtained the signed public checksum record through the alternate
Google checksum endpoint; implementation evidence must preserve those values.

### 2. Use only the existing copied stream-head accessor

The legacy anchor needs the already-existing `GlobalReadView.Head` method.
Implementation must not expose the complete head map, raw Events, or a mutation
surface.

### 3. Windows is a compile-only portability gate

Unsupported-platform IPC remains fail-closed. Windows compilation must not be
misreported as Windows runtime socket support, and tests must not weaken peer
authorization to satisfy the compile gate.

## Decision

The P2A-W1 contract may become `FROZEN`. Mandatory behavioral RED may begin
inside the exact owned test files. No product implementation or live action is
authorized before RED is captured.

VERDICT: PASS
