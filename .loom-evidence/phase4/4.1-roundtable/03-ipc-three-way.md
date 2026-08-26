# Phase 4 · 4.1 RoundTable — 03 Strict IPC Three-Way Consistency

Status: `PASS`
Date: 2026-08-17

The 11 roundtable methods are registered in exactly three places, verified by
source + tests:

1. `internal/localipc/protocol.go` `validMethod` — daemon-side gate.
2. Swift client allowlist `LocalIPCClient.call(method:params:)` — client-side
   gate (apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift).
3. `cmd/loomd/product_route_registry.go` `productRouteManifest` + admission —
   route registry.

Methods: `roundtable_session_create`, `roundtable_add_seat`,
`roundtable_retire_seat`, `roundtable_open_round`, `roundtable_propose_message`,
`roundtable_relay_message`, `roundtable_ack_message`,
`roundtable_insert_message`, `roundtable_drop_message`, `roundtable_conclude`,
`roundtable_snapshot`.

- `TestCOMP2BRouteManifestFreezesEveryLegacyMethod` freezes the exact route set
  (updated to include the roundtable methods).
- `TestCOMP2BTypedRegistryFreezesLegacyAvailability` asserts admission.
- Roundtable route tests assert strict decode (unknown field →
  `invalid_request`; bad schema version → `invalid_request`) and typed error
  codes.
- Swift `LocalRoundtableModelsTests` + `LocalConversationModelGatingTests`
  assert strict decode and selection gating.

## Verdict

Gate 3 `PASS`: the three surfaces are consistent; strict decode rejects
unknown fields/out-of-range values.
