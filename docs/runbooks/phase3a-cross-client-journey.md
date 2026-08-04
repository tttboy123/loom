# Phase 3A cross-client journey

This runbook is the final user-journey gate for the single `P3A-W1`. It does
not replace unit, component, replay, race, security, Swift, materialization or
independent review gates.

## Non-negotiable boundary

Every scenario uses:

```text
real native Loom window + production Swift model
real PTY + production Bubble Tea model
→ production IPC clients
→ one real private Unix socket
→ production daemon handler and application services
→ authoritative writer and Event Journal
→ Projection / GlobalReadView
→ reconnect and read path
```

No AppleScript, ViewModel injection, direct service call, direct Projection or
SQLite mutation, visual-only fixture, Preview, launch-only proof or
accessibility-tree-only proof may cause product behavior. Per
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md`, the native window is driven by
launching the production app with `--journey-id` and performing mutations and
reads through the production Swift client over the real Unix socket, with real
window screenshots captured at checkpoints; the TUI is driven through a real
PTY. The shell tooling only prepares a private root, feeds the real PTY and
verifies immutable evidence.

The controlled journey manifest is disabled by default. When explicitly set
through `LOOM_P3A_CONTROLLED_JOURNEY_MANIFEST`, it may only write sanitized
request/response and daemon phase logs inside the same private root. Its
one-shot fault enum is closed to `none`, `crash_before_cas`,
`crash_after_cas_before_response`, `projection_failure` and `slow_response`.
It never creates authority and is bound to the exact state path, socket path,
journey UUID, owner and 0700/0600 identities.

## Frozen scenarios

Each scenario has a new UUIDv4, 0700 root, 0600 evidence and root-local binary
copies. Failed roots are immutable; a replacement gets a new root and UUID.

| Scenario ID | Required product result |
|---|---|
| `happy-create-evaluate-activate-bind-execute-clean` | TUI creates Team and Candidate; GUI cross-observes, evaluates, activates and binds; TUI cross-observes; GUI preflights and explicitly starts; exact attempt materializes, succeeds, produces accepted Evidence and cleans; clean restart adds no duplicate fact. |
| `cancel-reject-retain` | Native Cancel and PTY Escape produce zero mutation request/fact; one Candidate is explicitly rejected and one retained; both clients cross-observe exact decisions. |
| `stale-view-digest-generation` | Stale view, wrong digest/identity and old generation are visibly rejected; Journal counts do not change for rejected operations; recovery is read-only. |
| `concurrent-single-winner` | GUI and TUI confirm conflicting operations from the same baseline; exactly one CAS wins, the loser displays a conflict/stale state, and there is one authoritative effect. |
| `crash-before-cas` | Daemon exits at the reviewed before-CAS seam; no target Event or artifact exists; both clients show unavailable then recover after restart. |
| `crash-after-cas-before-response` | Daemon exits only after the authoritative commit and before response delivery; client never synthesizes success; restart/reconnect reads the single committed result and does not redeliver a duplicate effect. |
| `projection-failure-rebuild-reconnect` | Authority commits once, the controlled post-commit refresh fails, the old immutable view remains visible, and restart rebuilds the committed state from Journal for both clients. |
| `slow-client-redelivery-clean-restart` | A bounded post-commit response delay exceeds the client deadline; the client shows timeout/unavailable without hidden retry, explicit refresh/reconnect observes one effect, and clean restart adds no duplicate Event/Evidence. |

## Prepare a scenario

Build reviewed root-local `loom`, `loomd` and native App artifacts first. Then:

```sh
scripts/run-phase3a-cross-client-journey.sh prepare \
  --scenario happy-create-evaluate-activate-bind-execute-clean \
  --root /private/tmp/loom-p3a-happy-UNIQUE \
  --loom /absolute/path/loom \
  --loomd /absolute/path/loomd \
  --app /absolute/path/Loom-P3A-UNIQUE.app \
  --source /absolute/path/reviewed-source
```

The command returns the journey ID, exact state/socket paths and the private
runtime harness manifest. Freeze the returned JSON and binary/source digests
before starting the daemon.

Start the production daemon with the returned manifest environment and the
reviewed offline Pi/local-model arguments. Network and Provider credentials
remain absent:

```sh
LOOM_P3A_CONTROLLED_JOURNEY_MANIFEST=/ROOT/manifest/journey-harness.json \
  /ROOT/bin/loomd \
  --state /ROOT/state/loom.db \
  --isolation-root /ROOT/isolation \
  --runtime-dir /REVIEWED/PI/BIN \
  --runtime-dir /REVIEWED/NODE/BIN \
  --probe-id phase3a-JOURNEY \
  --instance-id runtime.pi.earendil-works.0.82.1 \
  --device-id local-mac \
  --display-name 'Pi 0.82.1' \
  --interval 1s \
  --process-timeout 10s \
  --socket /ROOT/loomd.sock \
  --local-model-private-root /REVIEWED/LOCAL-MODEL-ROOT \
  --local-model-executable /REVIEWED/llama-server \
  --local-model-path /REVIEWED/model.gguf
```

Start the real PTY with append recording so reconnect remains in one transcript:

```sh
LOOM_JOURNEY_ID=JOURNEY \
  /usr/bin/script -aq /ROOT/tui/transcript.txt \
  /ROOT/bin/loom app --socket /ROOT/loomd.sock
```

Start the root-local signed native executable with
`--socket /ROOT/loomd.sock --journey-id JOURNEY`. The app's production
`LocalIPCClient.defaultClient()` reads `--socket` for launch reads
(`snapshot`/`setup_snapshot`, recorded with `loom-swift-<uuid>` request IDs
in the harness IPC log), and `LocalProductStore.initialJourneyID()` reads
`--journey-id` for every evolution-asset call (falling back to a fresh local
UUID when the argument is absent). Drive mutations and reads through the
production Swift client (`LocalIPCClient`/`LocalProductStore`) over the same
daemon socket with the journey UUID on every call; capture a real window
screenshot after each action with `screencapture` and append the exact action
record (including `screenshot_relative_path`) to `gui/actions.jsonl`.

## Evidence package

Every completed root contains the exact schemas frozen by
`P3A-W1-CONTRACT-REPAIR-1.md`:

```text
manifest.json
result.md
gui/actions.jsonl
gui/screenshots/*
tui/transcript.txt
tui/keystrokes.jsonl
timeline.jsonl
ipc/request-response-summary.jsonl
daemon/structured-log.jsonl
journal/event-summary.json
journal/stream-heads.json
journal/sqlite-summary.json
projection/summary.json
artifacts/digest-verification.json
processes/preflight.json
processes/postflight.json
processes/cleanup-proof.txt
state/loom.db
```

The action, keystroke and timeline logs carry the one journey ID. The daemon
logs only IDs, closed enums and digests; they never include params, asset
bytes, prompts, Runtime output, credentials, Grant tokens or hidden reasoning.
Every screenshot/transcript/log/summary is 0600 and listed by relative path,
SHA-256, size and mode in `manifest.json`.

## Shutdown and restart

Quit TUI through its real `q` key. Close the native process, then send the
daemon its normal cancellation signal. The daemon must emit a bounded result
and exit zero. The product socket and `loomd.sock.lock` must disappear without
manual deletion. A closed state-lock file is not an active lease; `lsof` must
show no handle. No root-local Pi, llama, GUI, TUI or daemon process may remain.

Restart scenarios reuse only their own state and append to their own sanitized
logs/transcript. Logger sequence numbers continue monotonically. Reconnect is
read-only unless the scenario explicitly records a new confirmed action.

## Final verification

After the postflight evidence and canonical manifest are frozen:

```sh
scripts/run-phase3a-cross-client-journey.sh verify --root /ROOT
```

The verifier fails closed on missing GUI or PTY evidence, wrong modes or
digests, invalid journey/scenario identity, non-PASS result axes, SQLite
integrity/uniqueness/stream gaps, Projection mismatch, unresolved Artifact
digests, journey drift, missing GUI/TUI IPC traffic, secret-like text,
socket/lock/process residue or an evidence-file identity mismatch.

## Orchestrator evidence production (driver contract)

The run/verify scripts freeze and verify evidence; the Codex session that
drives the scenario produces the remaining evidence files through real
interaction and read-only inspection. Exact record schemas are frozen in
`P3A-W1-CONTRACT-REPAIR-1.md` §8 (sole authority); the field lists below
mirror it:

```text
result.md                     scenario PASS/FAIL summary, journey_id, dual
                              result axes, key assertions and cleanup note
gui/actions.jsonl             one record per production Swift client action
                              sequence, monotonic_offset_micros, action,
                              control_id, input_digest,
                              expected_visible_state, observed_visible_state,
                              screenshot_relative_path
tui/keystrokes.jsonl          one record per real PTY key
                              sequence, monotonic_offset_micros, key,
                              screen_id, expected_visible_state,
                              observed_visible_state
timeline.jsonl                sequence, monotonic_offset_micros, source, kind,
                              subject_id, status, journey_id, request_id,
                              authority_event_ids[]
projection/summary.json       schema_version:1, journey_id,
                              view_version and definition/revision/candidate/
                              evaluation/binding/materialization counts read
                              from the production daemon
                              (LoomLocalAppContractProbe --assets JOURNEY)
                              with matches_journal:true after cross-checking
                              the Journal event-summary counts
artifacts/digest-verification.json
                              schema_version:1, journey_id, artifacts[]
                              artifact item = expected_digest, actual_digest,
                              size, available, match; resolved read-only from
                              the journey Evidence root
processes/preflight.json      snapshot of processes/sockets/locks/leases/temps
                              before launch (schema_version, journey_id;
                              process item = pid, executable_digest, role,
                              state; resource item = kind, path_digest,
                              owner_uid, mode, present)
processes/postflight.json     same keys; every array must be empty after
                              shutdown
processes/cleanup-proof.txt   lsof/inode evidence that socket, lock, temp
                              roots and daemon/Pi/GUI/TUI processes are gone
```

Every record/log carries the frozen journey UUID; daemon logs contain only
IDs, closed enums and digests. Screenshots are captured from the real window
with `screencapture` and stored 0600 under `gui/screenshots/`.
