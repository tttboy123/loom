# SF-W2 Owned-File Amendment 1 (bounded, frozen)

Date: `2026-08-05`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the only and existing `SF-W2` (Ephemeral Worker Pools +
Lease/Reconciler + routing, frozen in `SF-WORKITEMS.md` and governed by the
accepted Gate 1 set). This Amendment creates no SF-W4, no thin WorkItem, no
new authority, no second database, no new schema beyond the frozen
`SF-EXIT-CONTRACT.md` §5 events, and no external action.

## 1. Reason

The accepted `SF-WORKITEMS.md` SF-W2 owned-file allowlist cannot deliver the
accepted SF-W2 journey and wire contract as frozen. Four delivery-critical
wiring surfaces are absent from the allowlist:

1. `cmd/loomd/product_daemon.go` (+ `product_daemon_test.go`) — the daemon's
   single IPC dispatch must build and wire the workers service and route
   `workers_snapshot` / `workers_command`; the composition signature gains
   one additive `*api.LocalWorkersAPI` parameter.
2. `cmd/loomd/sf1_queue_wire_test.go` — the accepted SF-W1 socket wire test
   calls `localProductHandlerWithComposition`; the additive signature change
   requires passing one extra `nil` argument (no behavior change).
3. `internal/tui/model.go` (+ `model_test.go`) — the Bubble Tea model must
   add a `Workers` screen (Tab navigation, refresh, View dispatch) so the
   real PTY TUI can observe worker/attempt state; `internal/tui/workers.go`
   is already in the allowlist.
4. `apps/macos/Sources/LoomLocalAppContractProbe/main.swift` and
   `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift` — the
   alternative-verification method drives GUI-side actions through the
   production Swift client; the SF-W2 journey requires new
   `workers_snapshot`/`workers_command` probe modes and the `callJourney`
   method-allowlist widening (additive).

## 2. Supersession (bounded)

This Amendment reopens only the four paths above for additive
delivery/evidence wiring. Existing accepted behavior, authority boundaries,
schemas, RED, journeys, and the SF-W1 lock are unchanged. No new schema,
authority, database, or writer is introduced; the SF-W2 schema surface is
exactly the frozen `SF-EXIT-CONTRACT.md` §5 `Attempt` record and event names.

## 3. Not claimed

No product/UI behavior change beyond additive wiring; no dependency,
migration, staging, push, merge, network, or live action is authorized by
Review PASS.

## 4. Independent Review acceptance

The Amendment passes only if a fresh read-only Reviewer proves: bounded
supersession; additive-only wiring; the SF-W2 allowlist plus these four paths
are the complete delivery boundary; no SF-W4/thin WorkItem; no schema or
authority change.

VERDICT: `FROZEN — PENDING REVIEW`
