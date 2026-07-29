# P2A-W1 Native App Launchability Replacement Live Gate Contract

**Date**: `2026-07-28`
**Status**: `FROZEN — FRESH INDEPENDENT CONTRACT REVIEW PASS`
**Parent**: frozen Native App Host Contract Revision and Launchability Closure
**WorkItem count**: unchanged; this remains P2A-W1
**Product files reopened**: none
**Replacement allowance before Review and activation**: `0`

## 1. Governance need

The first native-window canary installed the exact then-reviewed Candidate,
passed daemon/IPC prechecks, and consumed its only allowance. Both app launches
then aborted before SwiftUI entrypoint execution because the reviewed builder
removed `LC_UUID`. The transaction rolled back and fresh Result-Evidence
Re-review accepted the fail-closed result.

The same-W1 deterministic closure has now:

- diagnosed both immutable crash reports;
- replaced `-no_uuid` with content-derived linker `-reproducible`;
- preserved byte-identical signed clean builds;
- added pre-spawn RED and a bounded private launch smoke;
- passed the complete Swift/Go/install/race matrix;
- passed fresh independent Implementation Review with no findings.

The original no-retry result remains true. This contract does not reinterpret
it as unconsumed. It proposes one new replacement allowance solely to verify
the reviewed launchability repair and the already-frozen complete native W1
journey.

## 2. Exact Candidate lock

The replacement gate may use only:

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

LoomLocalApp
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

Info.plist
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

canonical complete bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

Immediately before live mutation, a post-Review activation audit must rebuild
from a reset Swift package and clean Go build cache as applicable, reproduce
every hash/UUID/manifest, pass the focused build/install fixtures, and remove
the Swift package cache. Any mismatch stops pre-bootstrap and leaves allowance
`1`, unconsumed.

## 3. Exact original rollback lock

Before mutation, the gate must independently reproduce:

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
mode 0600

canonical GlobalReadView digest
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

It must also prove original observer `running`, actual target-process
five-marker count `0`, SQLite integrity `ok`, Event count `1`, no Candidate
app/run/socket/launcher/native process, two preserved historical app crash
reports at their reviewed hashes, and empty staging.

The private transaction captures the exact original provenance xattr bytes for
`loomd` and `loomd-clean` without emitting them.

## 4. Contract Review and activation

This contract grants no live authority when drafted or reviewed.

After fresh independent Contract Review `PASS`, a post-Review activation audit
must pass. Exactly one replacement Candidate bootstrap is then authorized only
by a new user message containing:

```text
P2A-W1 LC_UUID Launchability Repair and one replacement native-window canary
```

No earlier activation, blanket authorization, prior Candidate phrase, or
automatic goal continuation may substitute for that exact post-Review message.
The message makes the replacement allowance `1`; successful Candidate
bootstrap consumes it. Pre-bootstrap validation failure does not.

## 5. Controlled transition

The existing reviewed native app and daemon installers are composed in one
fail-closed transaction:

1. create one private `0700` transaction root;
2. snapshot exact original files, modes, plist, xattr bytes, service/PID,
   Journal, crash reports, absent paths, and staging;
3. install the exact Candidate app and daemon/plist atomically;
4. boot out the original observer and prove label/PID quiescence;
5. bootstrap the Candidate exactly once from the credential-clean context;
6. prove exact Candidate hashes/modes, service/plist/arguments, private `0700`
   run directory, actual uid-`501` mode-`0600` non-symlink AF_UNIX socket,
   no historical socket, and target-process five-marker count `0`;
7. read the typed daemon snapshot only as a pre-UI comparison: one real
   Runtime, zero Teams/Runs/Evidence/Attention, same canonical view and
   unchanged Journal.

No Provider/model request, Runtime execution, Team creation, Journal write,
credential mutation, or authority transition is permitted.

## 6. Native-window proof

Computer Use initializes through its plugin runtime and targets only:

```text
com.earendilworks.loom.local
```

It never targets Terminal, Finder, `Loom.command`, CLI, PTY, or an archived
image.

The first `get_app_state` is the single app-launch boundary. If it returns an
error, no independently addressable window, or produces a new crash report,
the gate rolls back immediately. It is not repeated as a retry.

After every successful interaction, a fresh state is fetched before using an
Accessibility index. The actual installed window must prove:

| Screen | Required result |
|---|---|
| Home | `Runtimes 1`, `Teams 0`, `Runs 0`, `Attention 0` |
| Runtimes | `Pi 0.82.1 Resident Demo` plus bounded status/version/capacity |
| Teams | `No Teams exist in this Journal yet.` |
| Runs | `No run history is available.` |
| Evidence | `No accepted evidence is available.` |
| Compare | `Nothing to compare.` |
| Attention | `Nothing needs your attention.` |
| Team Timeline | `Select a Team from Teams to open its authoritative timeline.` |

Empty-Team activation must leave selection/timeline and Journal unchanged.
Accessibility text and a freshly inspected screenshot must contain no
credential, internal path, raw ID, terminal/bidi control, hidden reasoning, or
Provider material.

The gate then:

1. quits the native app and proves process quiescence;
2. relaunches it once through Computer Use and reproduces Home, Runtime,
   zero-Team, and Timeline-selection state;
3. quits it again;
4. performs the one contract-required Candidate daemon restart;
5. reproves socket/process/Journal/canonical-view identity;
6. relaunches the same app and reproduces the identical view.

The relaunches and required daemon restart are lifecycle proof inside the one
canary, not replacement attempts.

## 7. Crash-report and secret-negative proof

Before initial app launch, after each app process exit, and after final
relaunch, record only sorted `LoomLocalApp-*.ips` filenames and hashes.
Inventory must remain exactly the two preserved historical reports:

```text
LoomLocalApp-2026-07-28-213614.ips
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0

LoomLocalApp-2026-07-28-213628.ips
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

Any new report fails the canary, is preserved, and is recorded only by name and
hash. The transaction never deletes or rewrites DiagnosticReports.

Presence-only scans cover Candidate app/daemon process, plist, bundle,
launcher, logs, Journal, screenshot, Accessibility text, and evidence. No
process environment row, launchd environment value, credential, raw output,
prompt, Grant, or hidden reasoning is emitted.

## 8. Exact rollback

Any post-bootstrap failure uses one rollback path:

1. quit the exact native app and prove process quiescence;
2. boot out Candidate and prove label/PID/socket/lock quiescence;
3. use reviewed installers to restore original app absence and exact original
   daemon files/plist;
4. explicitly restore `loom` and `loomd` modes `0755`, wrapper `0700`, plist
   `0600`;
5. if original bootstrap is rejected with the preserved provenance metadata,
   temporarily remove only the captured provenance xattrs from exact original
   `loomd` and wrapper, bootstrap the exact original plist, prove `running`,
   then restore both xattrs byte-for-byte;
6. restore and prove exact original hashes, modes, xattr presence, service,
   clean target-process predicate, Journal, crash-report inventory, absent
   Candidate paths, Swift-cache absence, and empty staging;
7. remove only validated fixture-owned private paths.

The rollback never removes a crash report, alters Journal authority, or retries
the Candidate.

## 9. Success and terminal accounting

Full live `PASS` preserves the exact Candidate app and daemon, sets replacement
allowance to `0`, and requires a fresh independent Result-Evidence Review.
Only Result Review `PASS` permits the atomic P2A-W1 commit and unlocks P2A-W2.

Any post-bootstrap failure consumes the allowance and terminates:

```text
FAIL — ROLLED_BACK — HUMAN_REQUIRED
```

There is no second replacement, hidden retry, point Amendment, alternate host,
or evidence substitution inside this contract.
