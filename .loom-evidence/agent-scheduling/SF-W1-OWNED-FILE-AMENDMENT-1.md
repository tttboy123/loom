# SF-W1 Owned-File Amendment 1 (bounded, frozen)

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the only and existing `SF-W1` (Queue State/Projection +
Admission/Eligibility/Conflict Arbiter, frozen in `SF-WORKITEMS.md` and
accepted by `GATE1-CONTRACT-REVIEW-2.md`). This Amendment creates no SF-W4,
no thin WorkItem, no new authority, no second database, no new schema, and
no external action.

## 1. Reason

The accepted `SF-WORKITEMS.md` SF-W1 owned-file allowlist cannot deliver the
accepted SF-W1 journey as frozen. Three delivery-critical wiring files are
P3A-owned and absent from the allowlist:

1. `cmd/loomd/product_daemon.go` — the daemon's single IPC dispatch
   (`localProductHandlerWithComposition` switch) must route the new queue
   methods (`queue_snapshot`, `queue_command`) and build/wire the queue API;
   there is no plugin or alternate dispatch path.
2. `internal/tui/model.go` — the Bubble Tea model owns the screen list, Tab
   navigation, refresh keys, and `View` dispatch; a new queue screen cannot
   be shown in the real PTY TUI without additive wiring there.
3. `apps/macos/Sources/LoomLocalAppContractProbe/main.swift` — the accepted
   alternative-verification method drives GUI-side mutations and reads
   through the production Swift client via `LoomLocalAppContractProbe`; the
   SF-W1 journey requires new queue action modes in the probe.

Without these additive wiring changes the frozen SF-W1 deliverables
(queue/board IPC surface; GUI+TUI journey with conflict serialization and
DAG-cycle rejection visible in both clients) are unimplementable.

## 2. Supersession (bounded)

This Amendment amends only the `SF-WORKITEMS.md` SF-W1 owned-file allowlist
by adding the four files below with additive-only carve-outs. It changes no
other WorkItem allowlist, no contract schema, no authority boundary, no
RED, no journey definition, no verification matrix, and no acceptance item.
`SF-EXIT-CONTRACT.md` and ADR-0014 remain in full force.

## 3. Allowlist additions (additive-only)

1. `cmd/loomd/product_daemon.go` — additive: build and wire the queue API
   service and add `queue_snapshot` / `queue_command` routing to the existing
   switch; existing methods, params, and semantics unchanged.
2. `cmd/loomd/product_daemon_test.go` — additive: queue routing and
   service-wiring regression tests.
3. `internal/tui/model.go` — additive: a new `ScreenQueue` constant, its
   entry in the `screens` list, Tab/refresh wiring, and `View` dispatch;
   existing screens, keys, and behaviors unchanged.
   `internal/tui/model_test.go` — additive counterpart: the screen-navigation
   test is updated to include `ScreenQueue` in the expected Tab sequence
   (a production IPC refresh command is expected on entry to `ScreenQueue`,
   as on `ScreenAssets`).
4. `apps/macos/Sources/LoomLocalAppContractProbe/main.swift` — additive: new
   queue action modes (`queue_action` create/read variants) for the SF-W1
   journey; existing modes and their outputs unchanged.
   `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift` — additive
   visibility widening only: `call`/`callJourney` change from `private` to
   module-internal so `LocalQueueModels.swift` can reuse the production
   framing for the queue methods; no behavior or wire change.

Each file remains governed by the same additive rule used for
`internal/localipc/protocol.go`: new additions only; existing behavior and
wire format unchanged; accepted P3A semantics are never reopened.

## 4. Not claimed

This Amendment does not authorize any product-behavior change beyond the
additive wiring above, any authority/schema/credential expansion, any second
writer/database, any push/merge, any network or paid action, any migration,
or any modification of accepted P2A/P2B/P3A contracts or journey evidence.

## 5. Independent review acceptance

The Amendment passes only if a fresh read-only Reviewer proves: bounded
supersession; the four additions are strictly additive wiring required by
the frozen SF-W1 journey; no authority/schema/RED/journey/acceptance change;
no SF-W4 or thin WorkItem created.

VERDICT: `FROZEN — PENDING REVIEW`
