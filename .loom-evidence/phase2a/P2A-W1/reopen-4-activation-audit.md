# P2A-W1 Reopen 4 Post-Review Activation Audit

**Date**: 2026-07-28
**Status**: `PASS — EXPLICIT ACTIVATION RECEIVED`
**Live invocation consumed**: no
**Installation or service mutation performed**: no

## Review gate

The fresh independent Reopen 4 Implementation Re-review 3 is `PASS` with no
findings. This audit does not replace or broaden that Review and does not
activate live execution.

The reviewed Candidate remains available at:

```text
/tmp/loom-p2a-w1-reopen4-repair1-build-a.KkCrvA
```

Its exact reviewed hashes remain:

```text
loom   7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd
loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
```

## Installed pre-state

Read-only checks prove the installed original state remains exact:

- original `loom` hash and mode `0755`;
- original `loomd` hash and mode `0755`;
- original credential-clean wrapper hash and mode `0700`;
- original LaunchAgent plist hash, mode `0600`, and valid plist syntax;
- resident observer service loaded and running;
- exact target-process environment predicate `clean`;
- no product run directory;
- no default product socket;
- no historical resident socket;
- no installed `Loom.command` launcher.

No Provider value, process row, command line, or service-manager environment was
recorded in this evidence.

## Journal pre-state

The resident SQLite Journal remains byte-for-byte equal to the frozen pre-state:

```text
sha256  91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
integrity_check  ok
Event count  1
canonical GlobalReadView head digest  6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The audit did not write, copy, merge, rebuild, or switch the Journal.

## Repository boundary

Git staging is empty. No commit, push, merge, release, Provider request,
Runtime execution, Team creation, or authoritative state transition occurred.

## Activation

Section 10 of the frozen Reopen 4 contract requires an explicit post-Review
user message authorizing:

```text
P2A-W1 Vertical Live Closure Reopen 4 and one replacement controlled canary
```

The user supplied the exact post-Review activation message on `2026-07-28`:

```text
P2A-W1 Vertical Live Closure Reopen 4 and one replacement controlled canary
```

Reopen 4 is therefore active for exactly one replacement controlled canary.
The allowance remains unconsumed until Candidate bootstrap. No second
bootstrap, hidden retry, alternate socket, Provider/model request, Runtime
execution, Team mutation, credential change, or publication is authorized.
