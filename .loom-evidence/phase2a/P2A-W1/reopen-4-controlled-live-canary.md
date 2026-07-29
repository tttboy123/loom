# P2A-W1 Reopen 4 Replacement Controlled Live Canary

**Date**: `2026-07-28`
**Status**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`
**Replacement allowance remaining**: `0`
**Product capability claimed**: no
**Candidate bootstrap performed**: yes, exactly once before the UI gate
**TUI screens accepted**: no
**Explicit daemon restart performed**: no

## Authority and pre-bootstrap guard

The user supplied the exact Reopen 4 post-Implementation-Review activation:

```text
P2A-W1 Vertical Live Closure Reopen 4 and one replacement controlled canary
```

Before installation, the controlled gate revalidated:

- the exact independently reviewed Candidate hashes:

  ```text
  loom   7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd
  loomd  f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
  ```

- the exact original `loom`, `loomd`, wrapper, and plist hashes and modes;
- a valid original plist with 21 arguments and a private Candidate plist with
  exactly two appended arguments:
  `--socket` and the authoritative default socket;
- SQLite byte hash, mode `0600`, `integrity_check=ok`, one Event, and canonical
  GlobalReadView head digest;
- a running original observer whose exact target-process environment predicate
  was `clean`;
- absent product run directory, default socket, historical socket, and
  launcher;
- empty Git staging.

Two harness-only pre-install errors occurred before any installation, bootout,
or bootstrap:

1. the first temporary plist construction used an invalid `plutil` option
   form for the leading-hyphen string;
2. the first temporary shell harness attempt contained a malformed multiline
   test expression.

Both stopped with the installed files, service, Journal, and paths still exact.
They did not consume the invocation allowance. The corrected temporary inputs
were syntax-checked and the complete pre-bootstrap guard passed before the
controlled transition.

## Controlled transition

The gate:

1. created the authoritative product run directory as `0700`;
2. atomically installed the exact reviewed binaries, launcher, and
   two-argument plist;
3. cleanly booted out the original service;
4. proved exact service and old-PID quiescence within the frozen bound;
5. bootstrapped the Candidate once from the reviewed credential-clean command
   context;
6. observed the private default Unix socket as `0600`, uid `501`.

The Candidate bootstrap succeeded, so the single Reopen 4 replacement
allowance was consumed.

Before opening the user interface, all non-UI gates passed:

- installed Candidate hashes and `0700` modes were exact;
- wrapper hash and `0700` mode were unchanged;
- plist was the exact reviewed two-argument addition, mode `0600`;
- no historical resident socket existed;
- the durable exact-target process predicate returned `clean`;
- loaded service marker names equaled the reviewed five-name ambient manager
  set;
- non-emitting per-key comparisons proved the manager values unchanged;
- plist, launcher, process, logs, and resident Journal remained
  secret-negative;
- SQLite bytes, integrity, one-Event count, and canonical head digest remained
  unchanged;
- the typed local snapshot returned the same canonical view, exactly one real
  `Pi 0.82.1 Resident Demo` Runtime, and zero Teams, Runs, Evidence, and
  Attention records.

No Provider/model request, Runtime execution, Team creation, Journal write, or
authority transition occurred.

## Blocking Computer Use result

Computer Use successfully:

1. opened Finder;
2. navigated to the installed no-argument `Loom.command`;
3. selected that installed launcher;
4. invoked Finder's exposed `Open` action.

macOS Terminal opened because `Loom.command` is the Phase 2A W1 TUI launcher.
At that point the Computer Use safety layer rejected both:

- reading Terminal accessibility state; and
- sending even a navigation key to Terminal.

The returned reason was that Computer Use is not allowed to use
`com.apple.Terminal` for safety reasons. No independently addressable Loom app
window existed; the running-app inventory exposed only Terminal for the
launched TUI.

The frozen contract requires Computer Use to inspect the actual Pi Runtime,
truthful no-Team state, direct Team Timeline selection instruction, and the
Runs, Evidence, Compare, and Attention empty states. It also requires the empty
Teams Enter action, quit/relaunch, and one explicit daemon restart.

Those requirements were not silently replaced with CLI output, an alternate
PTY, an archived Journal, a synthetic fixture, or an unreviewed app wrapper.
Therefore the live UI gate is unproven and the canary cannot pass. The explicit
daemon restart was not attempted after the UI gate failed.

This is a `test_defect` / controlled-tool capability boundary, not evidence of
a Loom process, state, credential, or read-model defect. It still consumes the
canary and fails the product live gate under the frozen no-retry contract.

## Exact rollback

The controlled trap:

1. booted out the Candidate service;
2. waited for exact Candidate service/PID quiescence;
3. atomically restored the original `loom`, `loomd`, and plist;
4. retained the original wrapper unchanged;
5. removed the Candidate launcher, socket, socket lock, and product run
   directory;
6. bootstrapped the original observer from the credential-clean context.

Fresh post-recovery verification proves:

- original `loom`:
  `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698`,
  mode `0755`;
- original `loomd`:
  `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`,
  mode `0755`;
- original wrapper:
  `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4`,
  mode `0700`;
- original plist:
  `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4`,
  mode `0600`, plist-valid;
- resident SQLite:
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`,
  mode `0600`, `integrity_check=ok`, one Event;
- canonical GlobalReadView head digest:
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`;
- restored observer loaded and running with exact-target process predicate
  `clean`;
- restored service marker-name set equal to the reviewed ambient set;
- product run directory, default socket, historical socket, and launcher
  absent;
- Git staging empty.

The Terminal application was not running before the gate. The one Terminal
application instance opened by Finder for this canary was terminated after
rollback, restoring that external pre-state.

## Invocation accounting and terminal result

The Candidate bootstrap occurred, so Reopen 4's only replacement allowance is
consumed. The frozen Vertical Live Closure forbids another point Amendment,
second bootstrap, hidden retry, alternate terminal emulator, unreviewed wrapper,
or evidence substitution.

P2A-W1 therefore remains unaccepted, uninstalled, uncommitted, and
`HUMAN_REQUIRED`. P2A-W2 remains locked. No P2A-W1 product capability is
claimed from this failed canary.
