# P2A-W1 Amendment 2 Independent Review

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer in a new review turn
**Verdict**: `PASS`
**Blocking findings**: none

## Findings

1. Live Gate 1 failed before daemon, socket, or TUI startup, performed no retry,
   and restored exact file, service, socket-absence, SQLite hash, integrity, and
   Event-count state.
2. Amendment 2 changes only service-transition readiness: after clean
   `bootout`, wait at most ten seconds in 100ms intervals for both exact service
   absence and exact old-PID absence before one bootstrap.
3. It preserves one default socket, no alternate daemon, no Provider/model/
   Runtime execution, no credential disclosure or global launchd environment
   mutation, exact rollback, and `HUMAN_REQUIRED` on repeated failure.
4. The post-load zero-Provider-key requirement correctly prevents preserving a
   service whose loaded metadata contains the three stale legacy marker keys.
5. The restored installed state was independently confirmed:
   original `loom`, `loomd`, plist, and SQLite hashes; SQLite
   `integrity_check=ok`; one Event; observer-only PID `33235`; absent default
   run directory/socket and absent launcher.
6. The Reviewer independently rebuilt the current Candidate with `-trimpath`.
   The hashes exactly match Live Gate 1:
   - `loom`:
     `cfcf711da265a68ce924c9dc78e76103de66193378dc430fe4f79e6ce6a10a8a`
   - `loomd`:
     `1d76c641ad66142f8dd9706c05880a6953f6368815ef4068d3a25dfb5d25e4f2`

No file edit, staging, service mutation, bootout/bootstrap, Provider/Runtime/
model/network action, or live canary was performed by the Reviewer.

## Gate result

Amendment 2 may become `FROZEN`. Exactly one replacement controlled live canary
may execute its bounded quiescence sequence. No further retry is authorized.
