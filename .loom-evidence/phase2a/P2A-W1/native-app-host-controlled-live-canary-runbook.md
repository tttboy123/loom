# P2A-W1 Native App Host Controlled Live Canary Runbook

Date: 2026-07-28  
Status: `FROZEN — PRE-LIVE — NOT ACTIVATED`  
Live allowance: one native-window Candidate bootstrap, unconsumed  
Historical Reopen 4 allowance: consumed, unavailable

## 1. Activation guard

No live step in this runbook may execute until a new post-Implementation-Review
user message contains exactly:

```text
P2A-W1 Native App Host and one controlled native-window canary
```

The activation authorizes one Candidate daemon bootstrap and one native-window
canary. It does not authorize a Terminal-hosted retry, a second bootstrap after
failure, Provider or Runtime execution, Journal writes, Team creation, another
WorkItem, commit, push, merge, release, or publication.

The allowance is consumed only when the Candidate LaunchAgent is successfully
bootstrapped. Build, dry-run validation, and a failure before bootout/bootstrap
do not consume it. Once consumed, every subsequent failure rolls back and
terminates the canary; there is no hidden retry.

## 2. Fixed paths and identities

```text
service label
com.earendilworks.loom.runtime-observer

original/current daemon root
/Users/lune/Library/Application Support/Loom/demo-resident

resident Journal
/Users/lune/Library/Application Support/Loom/demo-resident/loom.sqlite

LaunchAgent plist
/Users/lune/Library/LaunchAgents/com.earendilworks.loom.runtime-observer.plist

Candidate run directory
/Users/lune/Library/Application Support/Loom/run

Candidate socket
/Users/lune/Library/Application Support/Loom/run/loomd.sock

native app destination
/Users/lune/Applications/Loom.app

native bundle identifier
com.earendilworks.loom.local
```

Every temporary build, manifest, plist, and rollback root must be a fresh
user-owned, non-symlink `0700` directory under `/private/tmp`. SQLite is opened
only through its exact existing path with `mode=ro` and
`PRAGMA query_only=ON`. No guessed database path is permitted.

## 3. Frozen original pre-state

Immediately before any mutation, reproduce and compare:

| Surface | Required SHA-256 | Mode |
|---|---|---|
| `demo-resident/bin/loom` | `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698` | `0755` |
| `demo-resident/bin/loomd` | `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a` | `0755` |
| `demo-resident/bin/loomd-clean` | `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4` | `0700` |
| LaunchAgent plist | `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4` | `0600` |
| resident Journal | `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4` | `0600` |

Required state:

- the original service is loaded and running from `loomd-clean`;
- target-process environment contains none of the five frozen Provider marker
  names;
- Journal integrity is `ok`, Event count is `1`, and canonical
  GlobalReadView head digest is
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`;
- product run directory, default product socket, historical resident socket,
  and the pre-Candidate `Loom.command` are absent;
- Git staging is empty;
- any existing `Loom.app` and `Loom.app.previous` are fully inventoried by
  owner, mode, bundle identifier, architecture, signature, descendant
  manifest, and digest before they are touched.

Presence-only launchd ambient-marker checks may record names and booleans. They
must never print, copy, compare, persist, or restore a Provider value.

Any mismatch is a pre-bootstrap `FAIL`; installed state remains untouched and
the live allowance remains unconsumed.

## 4. Candidate preparation

Under the fresh private root:

1. build `loom` and `loomd` from the unchanged reviewed worktree;
2. build `Loom.app` with `scripts/build-loom-local-app.sh`;
3. reproduce file hashes, owner, modes, arm64 architecture, bundle identifier,
   strict ad-hoc signature validity, and the no-symlink bundle manifest;
4. run both installers with `--dry-run` against their exact live
   destinations;
5. copy the original plist into the private root and append only:

   ```text
   --socket
   /Users/lune/Library/Application Support/Loom/run/loomd.sock
   ```

6. prove the private Candidate plist is valid, mode `0600`, retains the exact
   original arguments in order, and differs only by those two arguments;
7. recheck complete deterministic verification, `git diff --check`,
   `git diff --cached --check`, and empty staging.

No build or validation process may read a Provider value, contact a Provider,
execute Pi, write the Journal, or alter the loaded service.

## 5. One controlled installation transaction

Before the first mutation, arm one fail-closed cleanup trap containing the
exact original plist bytes, daemon installation state, app installation state,
service identity, Journal measurements, and path-absence assertions.

The controlled transition is:

1. create the exact product run directory as user-owned `0700`;
2. atomically install reviewed `loom` and `loomd` into `demo-resident` with
   `scripts/install-loom-local-product.sh`;
3. atomically install the reviewed bundle at
   `/Users/lune/Applications/Loom.app` with
   `scripts/install-loom-local-app.sh`;
4. atomically replace the LaunchAgent plist with the reviewed Candidate plist;
5. boot out the original service and wait for both the exact label and original
   PID to quiesce;
6. bootstrap the Candidate exactly once from the reviewed credential-clean
   command context.

Successful step 6 consumes the sole native-window allowance.

Before opening the app, require:

- the installed binary and bundle manifests exactly equal the reviewed
  Candidate;
- service program, plist, and arguments are exact;
- the socket is an actual non-symlink AF_UNIX socket, owner uid `501`, mode
  `0600`, under the exact `0700` run directory;
- the Candidate target process exposes none of the five frozen Provider marker
  names;
- the historical socket is absent;
- the reviewed daemon installer may materialize its existing `Loom.command`
  compatibility launcher, but Computer Use never opens or targets it;
- the typed daemon snapshot returns the same canonical view: one real
  `Pi 0.82.1 Resident Demo` Runtime and zero Teams, Runs, Evidence, and
  Attention;
- Journal bytes, integrity, Event count, heads, and canonical digest remain
  unchanged.

Any failure invokes section 10 immediately.

## 6. Native Computer Use boundary

Computer Use is initialized through its plugin-owned runtime and targets only:

```text
com.earendilworks.loom.local
```

It must never target Terminal, another terminal emulator, `Loom.command`, a
CLI, a PTY, or an archived screenshot. If the bundle identifier cannot expose
an independently addressable native window, the canary fails and rolls back.

For every interaction:

1. fetch fresh app state;
2. derive the current accessibility element index;
3. perform one bounded action;
4. fetch fresh state again before interpreting the result.

Accessibility indices are never reused across stale states. Element-index
actions are preferred; coordinates are used only when current screenshot and
accessibility evidence prove the exact target.

## 7. Required native-window proof

The live pass requires the actual installed window to show:

| Screen | Required user-visible result |
|---|---|
| Home | `Runtimes 1`, `Teams 0`, `Runs 0`, `Attention 0` |
| Runtimes | `Pi 0.82.1 Resident Demo`, bounded status/version/capacity |
| Teams | `No Teams exist in this Journal yet.` |
| Runs | `No run history is available.` |
| Evidence | `No accepted evidence is available.` |
| Compare | `Nothing to compare.` |
| Attention | `Nothing needs your attention.` |
| Team Timeline | `Select a Team from Teams to open its authoritative timeline.` |

The native window must remain connected, nonfatal, and free of database paths,
socket paths, cursors, raw internal IDs, terminal controls, bidi controls, or
Provider material.

On the empty Teams screen, perform the bounded activation/Return interaction
available to the current accessibility tree. It must leave Team selection and
timeline unchanged, produce no error, and preserve Journal bytes and canonical
view. Deterministic `LocalProductStore` proof remains the direct request-count
evidence; live evidence must not invent daemon telemetry that does not exist.

## 8. Relaunch and daemon restart

After all eight screens pass:

1. quit only `com.earendilworks.loom.local`;
2. prove the native process exited;
3. relaunch the same installed bundle through Computer Use;
4. reproduce the identical Home, Runtime, zero-Team, and Timeline-selection
   view;
5. quit the app again;
6. boot out the Candidate daemon once and wait for exact label/PID/socket
   quiescence;
7. bootstrap the same Candidate plist once as the contract's explicit restart,
   not as a retry;
8. prove the same socket owner/mode, process secret-negative predicate,
   unchanged Journal, and canonical snapshot;
9. relaunch the same native bundle and reproduce the identical view.

The explicit restart does not change the invocation count because it is a
required lifecycle check after the first successful canary bootstrap. A
failure during restart or recovery rolls back; it does not permit another
Candidate generation.

## 9. Secret-negative and evidence proof

Record only bounded, non-sensitive facts:

- native process executable identity and presence-only Provider-marker result;
- accessibility text after safe-text inspection;
- a Computer Use screenshot only after visual secret-negative inspection;
- screenshot/tool invocation digest or reference, not a copied secret-bearing
  artifact;
- presence-only, non-emitting scans of native process, daemon process, plist,
  app bundle, launcher surfaces, logs, Journal, and Markdown evidence;
- Journal hash/integrity/Event count/canonical digest before, after UI
  interaction, after relaunch, and after daemon restart;
- app/daemon hashes, modes, owner, service identity, socket metadata, and
  staging state.

Do not record a process row, complete command environment, launchd value, raw
Grant, credential, hidden reasoning, Provider response, internal prompt, or
unbounded output.

## 10. Exact failure rollback

Any post-bootstrap failure executes one cleanup path:

1. quit the native app and prove its PID quiesced;
2. boot out the Candidate service and prove its PID, label, socket, and lock
   quiesced;
3. restore the exact original `loom`, `loomd`, wrapper, and LaunchAgent plist
   bytes and modes;
4. restore the exact prior app transaction:
   - when a prior app existed, use the reviewed installer rollback;
   - when no prior app existed, remove only the validated exact
     `/Users/lune/Applications/Loom.app` Candidate and its installer-owned
     rollback target, if any;
5. remove only the exact Candidate run directory, socket, lock, and private
   transaction root after validating owner, type, and canonical path;
6. bootstrap the original observer from the credential-clean context;
7. reproduce every section 3 hash, mode, service, process, Journal, staging,
   and absent-path invariant.

Temporary harness errors before successful Candidate bootstrap are recorded and
fixed only while installed state is unchanged. After bootstrap, any failure
consumes the allowance and terminates as
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`.

## 11. Pass and handoff

Only a complete pass preserves the Candidate app and daemon installation. The
result evidence must state:

```text
PASS — NATIVE APP AND DAEMON PRESERVED
native live allowance remaining: 0
```

P2A-W1 still does not exit at that point. A fresh independent
Result-Evidence Reviewer must reproduce the result, installed state,
Journal/Projection identity, secret-negative evidence, lifecycle proof, and
invocation accounting. Only Reviewer `PASS` permits the atomic P2A-W1 commit
and unlocks P2A-W2.
