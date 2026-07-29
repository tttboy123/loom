# P2A-W1 Native App Host Post-Review Activation Audit

Date: 2026-07-28  
Status: `PASS — PRE-LIVE ONLY — ACTIVATION NOT RECEIVED`  
Live invocation allowance: `1`, unconsumed  
Candidate bootstrap/install count: `0`

## Boundary

This audit follows Native App Host Implementation Re-review 2 `PASS`. It is a
read-only pre-live audit and isolated Candidate rebuild. It does not activate
section 10 of the frozen Native App Host Contract Revision.

The required post-Review activation message remains exactly:

```text
P2A-W1 Native App Host and one controlled native-window canary
```

The historical Reopen 4 allowance remains consumed at `0`; this native-window
allowance is separate and has not been consumed.

## Original installed state

The original resident installation still matches the frozen rollback state:

| Surface | SHA-256 | Mode |
|---|---|---|
| `demo-resident/bin/loom` | `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698` | `0755` |
| `demo-resident/bin/loomd` | `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a` | `0755` |
| `demo-resident/bin/loomd-clean` | `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4` | `0700` |
| `com.earendilworks.loom.runtime-observer.plist` | `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4` | `0600` |
| `demo-resident/loom.sqlite` | `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4` | `0600` |

The original `com.earendilworks.loom.runtime-observer` service is loaded and
running from `demo-resident/bin/loomd-clean`. Its PID changed once during the
long verification interval while launchd kept the same service, program,
plist, binary bytes, and Journal. No command in this audit bootstrapped,
booted out, killed, restarted, or rewrote that service.

The default product run directory and socket, historical resident socket, and
old `Loom.command` launcher are absent.

## Journal

The exact existing SQLite path was opened with `mode=ro` and
`PRAGMA query_only=ON`:

```text
integrity_check  ok
Event count      1
canonical GlobalReadView head digest
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The canonical digest was reproduced from the implementation-defined sorted
`stream_id NUL sequence NUL event_id LF` representation. No database, WAL,
stream head, Event, projection, or authority state changed.

## Provider metadata

The current resident target process contains none of the five frozen Provider
markers:

```text
DEEPSEEK_BASE_URL
STEPFUN_BASE_URL
MINIMAX_BASE_URL
DEEPSEEK_API_KEY
STEPFUN_API_KEY
```

The launchd user-domain ambient metadata still has all five names present.
This audit checked presence only and did not emit or persist any value. The
reviewed native entrypoint unsets exactly those names before constructing the
app, store, view, or IPC client. No credential or service-manager environment
mutation occurred.

## Isolated Candidate rebuild

The Candidate was rebuilt under a new `0700` `/private/tmp` root without
installing it:

| Artifact | SHA-256 | Mode |
|---|---|---|
| `loom` | `3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f` | `0755` |
| `loomd` | `ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357` | `0755` |
| `Loom.app/Contents/MacOS/LoomLocalApp` | `e25c6b764779268c248052650844da33accfde487f983839e5e97b76a3da8d37` | `0700` |
| `Loom.app/Contents/Info.plist` | `554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5` | `0600` |
| `Loom.app/Contents/_CodeSignature/CodeResources` | `6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b` | `0600` |

The canonical bundle-file manifest digest is:

```text
773f0733e359a91874f97c85468ca26c18ba47c246a66b47e54b95d14580ca51
```

The bundle identifier is `com.earendilworks.loom.local`; the executable is
`arm64`; strict code-signature verification passes; and the bundle contains
no symbolic links. The temporary Candidate root is not an installation or
durable authority surface and is removed after evidence capture.

## Deterministic verification

The post-Review verification result is:

- Swift tests: `17 passed`;
- Swift release build: `PASS`;
- native app build fixture: `PASS`;
- native app installer fixture: `PASS`;
- combined local-product installer fixture: `PASS`;
- focused real-Go-server/Swift-probe component checks: included in
  `internal/localipc` and `PASS`;
- `go test -count=1 ./...`: `PASS`;
- `go test -race -count=1 ./...`: `PASS`;
- `go vet ./...`: `PASS`;
- `go mod verify`: `all modules verified`;
- `git diff --check`: `PASS`;
- `git diff --cached --check`: `PASS`;
- staging: empty.

The first cold parallel full-Go invocation recorded two `internal/app`
fixture-process timeouts at the exact five-second limit while other packages
were under load. Both named tests then passed in an isolated focused run
(`0.73s` and `0.21s`), and the unchanged exact full command passed on a fresh
run; the race suite also passed. This transient is retained here rather than
being rewritten as an initial pass. It did not touch live state.

## Decision

All non-live Native App Host gates remain closed and the current original state
is recoverable. The exact post-Review activation phrase has not been supplied,
so no app or daemon installation, service bootstrap, Computer Use, native
window launch, daemon restart, Journal write, commit, or P2A-W2 work is
authorized by this audit.
