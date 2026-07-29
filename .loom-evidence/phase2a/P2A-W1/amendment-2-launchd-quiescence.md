# P2A-W1 Amendment 2: Launchd Namespace Quiescence

**Date**: 2026-07-28
**Status**: FROZEN — independent Amendment Review `PASS`
**Parent**: frozen P2A-W1 Local App Shell and Read Experience Contract
**Evidence**: `live-gate-1.md`

## Problem

The first controlled gate performed the frozen atomic install and one clean
bootout/bootstrap sequence. Candidate bootstrap returned macOS error 5 before
the daemon or socket started. Automatic rollback restored every controlled
file, SQLite invariant, and the prior running observer.

The sequence attempted bootstrap immediately after `bootout` returned.
Read-only launchd evidence shows the service namespace was still completing
removal and Background Task Management reconciliation during the following
136ms. The frozen live method did not require proof of namespace quiescence
between those two operations.

## Amendment

This amendment authorizes exactly one replacement controlled live canary with
one change to the service transition:

1. After clean-environment `bootout` succeeds, poll only the exact service
   target and old PID for at most ten seconds.
2. Quiescence requires both:
   - `launchctl print gui/501/com.earendilworks.loom.runtime-observer` reports
     the service absent; and
   - the exact pre-transition PID no longer exists.
3. Polling is bounded to 100ms intervals, prints no service metadata, and does
   not mutate global launchd environment.
4. Only after quiescence may the candidate plist be bootstrapped once.
5. If quiescence is not reached, bootstrap fails, the default socket is not
   private/owned, or loaded service metadata still contains any Provider key,
   atomically restore the exact pre-state and stop `HUMAN_REQUIRED`.
6. No third canary, hidden retry, sleep-based unbounded loop, `launchctl
   setenv/unsetenv`, global credential mutation, alternate socket, or manual
   daemon fallback is permitted.

The replacement canary reuses the exact reviewed Candidate hashes from Live
Gate 1. If repository bytes change before execution, the replacement authority
expires and a fresh Implementation Review is required.

## Preserved boundaries

- one daemon, one default private UDS, one read-only product;
- no Provider/model request or Runtime execution;
- no credential read/write/value disclosure;
- no Journal/StateWriter/Scheduler/Grant/Evidence mutation;
- exact pre/post Event count, stream-head digest, modes, owner, process,
  argument, metadata, log, screenshot, restart, and rollback gates;
- installed TUI must still be operated through Computer Use without entering a
  Team ID, cursor, SQLite path, or service-manager command;
- one post-TUI daemon restart remains required;
- no P2A-W1 split or P2A-W4.

## Exit

Independent Amendment Review `PASS` is required before replacement execution.
Another error 5 or any other live failure consumes the replacement allowance,
restores the prior resident service, and stops P2A-W1 `HUMAN_REQUIRED`.
