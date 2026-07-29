# P2A-W1 Native App Host Controlled Live Canary

**Date**: `2026-07-28`
**Status**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`
**Native live allowance remaining**: `0`
**Candidate bootstrap performed**: yes, exactly once
**Native window accepted**: no
**Explicit Candidate daemon restart performed**: no
**Product capability claimed**: no

## Authority

The user supplied the exact post-Implementation-Review activation:

```text
P2A-W1 Native App Host and one controlled native-window canary
```

The frozen controlled-live runbook was therefore activated once. This was not
a reuse of any earlier activation or Reopen 4 allowance.

## Controlled installation and daemon precheck

The transaction atomically installed the exact reviewed Candidate:

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

LoomLocalApp
6e9f03c064a1af09e27446a194bcbf5f5e14c3cd7e69d4a959af73229f2ebea8

canonical complete bundle manifest
221e9878db56b451dd5b724ec0b2e3ceacf349269da9fea3cf571690bc33f67a
```

It installed `/Users/lune/Applications/Loom.app`, installed the Candidate
daemon and two-argument socket plist, bootstrapped the Candidate exactly once,
and observed:

- one running Candidate daemon;
- an actual non-symlink AF_UNIX socket owned by uid `501`, mode `0600`;
- the socket parent directory owned by uid `501`, mode `0700`;
- none of the five frozen Provider marker names in the Candidate target
  process;
- a typed `daemon_api` snapshot with a versioned view, one Runtime, and zero
  Teams, Runs, Evidence, and Attention records;
- unchanged resident SQLite bytes, integrity, Event count, and canonical view.

That successful Candidate bootstrap consumed the sole native live allowance.
No Provider/model request, Runtime execution, Team creation, Journal write, or
authority transition occurred.

## Blocking native-window result

Computer Use was initialized through its plugin-owned runtime and targeted
only the frozen native bundle identifier:

```text
com.earendilworks.loom.local
```

It did not target Terminal, Finder, `Loom.command`, a CLI, or a PTY.

Two fresh `get_app_state` calls against that bundle identifier each returned:

```text
Computer Use server error -10005: timeoutReached
```

No independently addressable native window or Accessibility state was
obtained. The second call was a read of the same launched app state, not a
second install, bootstrap, generation, or live canary.

Section 6 of the frozen runbook explicitly requires failure and rollback when
the bundle identifier cannot expose an independently addressable native
window. Therefore none of the eight required screens, empty-Team activation,
quit/relaunch proof, screenshot proof, or explicit Candidate daemon restart
was attempted or claimed. CLI snapshot evidence was not substituted for UI
evidence.

The result is a native-host/tool-compatibility live failure. It is not evidence
of a Journal, Projection, socket, credential, or daemon-read defect, but it
still fails P2A-W1 under the frozen no-retry contract.

## Exact rollback

The still-active fail-closed transaction received exactly `rollback`. It:

1. terminated the Candidate native app;
2. booted out the Candidate service;
3. restored the original `loom`, `loomd`, wrapper, and plist;
4. removed the validated Candidate app, product run directory, socket, and
   compatibility launcher;
5. bootstrapped the original observer;
6. deleted the private live transaction root.

The first post-rollback health check found exact file and Journal restoration
but the original LaunchAgent at `spawn scheduled`, with no active target
process and an `OS_REASON_CODESIGNING` exit reason. Read-only diagnosis proved
the restored binary's embedded ad-hoc signature verified while macOS execution
assessment rejected the original provenance-bearing binary and wrapper.

To complete the same rollback rather than leave a degraded pre-state, the
recovery transaction:

1. captured the exact `com.apple.provenance` xattr bytes from the original
   `loomd` and `loomd-clean`;
2. temporarily removed only those two xattrs;
3. bootstrapped the exact original plist;
4. proved the original observer reached `running`;
5. restored both provenance xattrs byte-for-byte;
6. reconfirmed the exact original content hashes.

This recovery did not install Candidate code, invoke the native app, mutate the
Journal, or consume another canary. It was part of restoring the frozen
original state.

Final post-recovery verification proves:

```text
original loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
mode 0755

original loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
mode 0755

original wrapper
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
mode 0700

original plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
mode 0600

resident SQLite
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
```

Additional final predicates:

- original `loom` and `loomd` modes: `0755`;
- original wrapper mode: `0700`;
- original plist mode: `0600`;
- original observer state: `running`;
- target-process Provider marker count: `0`;
- SQLite mode: `0600`;
- SQLite `integrity_check`: `ok`;
- SQLite Event count: `1`;
- canonical GlobalReadView digest remains
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`
  because the authoritative SQLite bytes are identical;
- native app absent;
- Candidate product run directory and compatibility launcher absent;
- native app process absent;
- Git staging empty.

## Terminal result

The only Native App Host live allowance is consumed. The frozen contract
forbids a hidden retry, another Candidate bootstrap, UI substitution, or
automatic scope expansion.

P2A-W1 remains unaccepted, uninstalled, uncommitted, and
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`. P2A-W2 remains locked. A fresh
independent Result-Evidence Review may validate this fail-closed result, but
cannot turn it into product acceptance or authorize another canary.
