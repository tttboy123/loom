# P2A-W2 Mission Orchestration Workbench Live Canary

Date: 2026-07-30

Status: FAIL

Result classification: `preflight_input_identity_defect`

Attempt:
`p2a-w2-mission-workbench-live-20260730-001`

Candidate commit:
`d0252064e43dc2c7d9e047aaa36942ef7b92d97b`

## Verdict

The one authorized controlled live canary was consumed and stopped at daemon
construction. The exact committed daemon exited with code `3` and the single
bounded diagnostic:

```text
daemon unavailable
```

No replacement daemon start, alternate route, native-window launch, TUI launch
or manual socket cleanup was attempted. The frozen no-retry rule therefore
leaves P2A-W2 `HUMAN_REQUIRED` and not accepted.

## Locked inputs

The post-commit preflight independently established:

- repository `HEAD`:
  `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`;
- final 31-file source lock:
  `de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`;
- daemon SHA-256:
  `dc24ebc942be07e514de8921aa2acccb78bcf2e71944ac64063779f350d148f4`;
- TUI SHA-256:
  `7bcef2c1e695aefa8bdd7ee642b74c574d8ac3435cdf2802b7536ef3fba126d6`;
- native executable SHA-256:
  `03933dedc92195d9081b05c38e1253cfe97359827855b90013099825163da69b`;
- controlled manifest SHA-256:
  `465afcf2dc936fd2838f9d790abca4ce212b54521a68302d494a942337031a6a`;
- copied source-lock SHA-256:
  `5e62ac66432bc8e3036a9d7849cd3d729a1d48eb3da42bffde88be5c4706d749`;
- fresh user-owned attempt directories were mode `0700`;
- the SQLite file and manifest were regular, user-owned mode `0600`;
- the default product socket and product socket lock were absent;
- the controlled artifact root was absent;
- the attempt tree contained no symlink.

The exact attempt root was:

```text
/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-live-20260730-001
```

## Consumed start

The daemon received the private controlled-fixture manifest through
`LOOM_CONTROLLED_MISSION_FIXTURE_MANIFEST` and the frozen state, isolation,
Runtime, identity, interval, timeout, socket and Codex executable arguments.
The Codex argument was:

```text
/Users/lune/Documents/Codex/devtools/npm/bin/codex
```

That path is a symlink to:

```text
../lib/node_modules/@openai/codex/bin/codex.js
```

The daemon exited before its product socket became available.

## Root-cause assessment

The evidence strongly supports a live-preflight input identity defect; it is
not evidence of a relaxed product boundary. The external process intentionally
reported only `daemon unavailable`, so this classification is an inference
from the durable side effects and committed constructor order, not a captured
internal builder error.

The controlled fixture successfully passed its manifest, directory, state and
authority checks far enough to create the complete expected real
Journal/Evidence fixture. Committed constructor order places controlled-fixture
construction before product setup, and product setup places several bounded
setup constructors before the Codex native-auth observer.
`provider.NewCodexNativeAuthObserver` uses `os.Lstat` and accepts only an
absolute regular executable with execute bits, so it would reject the supplied
symlink with `ErrInvalidCodexNativeAuthConfig`. The available bounded output
does not directly prove that every preceding setup constructor succeeded, but
the real symlink mismatch and source order make this the supported diagnosis.
`loomd` correctly collapsed whichever internal builder error occurred to the
bounded external message `daemon unavailable`.

The canonical regular target existed, was executable, and had SHA-256:

```text
134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477
```

It was not substituted after failure because doing so would be a replacement
live start forbidden by the frozen contract.

## Bounded post-stop evidence

The failed constructor left no live process and no product listener:

- controlled daemon process: absent;
- native app process: never started;
- TUI process: never started;
- product socket: absent;
- product socket lock: absent;
- open handle on the controlled SQLite or its state lock: absent;
- isolation directory contents: empty;
- direct SQLite mutation: none;
- manual socket or lock deletion: none.

The controlled fixture committed `72` Events across `27` streams before the
setup boundary rejected the Codex path. All Events share the frozen
authoritative fixture time. Read-only SQLite verification returned
`integrity_check = ok`. The database SHA-256 after stop is:

```text
96023c1fb4680cfa264ca086ea334984c7d5c4018fe40b637f05f162ccad62fe
```

The Event payload scan found zero secret-like values and zero hidden-reasoning
markers. The fixture artifact store contains only its bounded Evidence
captures, receipts and content-addressed artifacts. No UI decision action,
Provider check, Keychain mutation, Runtime execution, approval decision,
recovery decision or Review acceptance occurred.

## Unmet live acceptance

Because the daemon never exposed the product socket, the canary did not reach:

- native Board and Mission Room;
- Team Pulse, Topology, Timeline or Capacity;
- draft/Inspector restore;
- bounded Provider management;
- Authorization, Review or Recovery decision checks;
- GUI/TUI same-Mission comparison;
- ordinary app and daemon shutdown.

Those requirements remain unproven live. Deterministic verification,
Implementation Review 6 and Visual Review 3 remain PASS for the committed
Candidate, but they do not substitute for the failed live gate.
