# P2A-W1 Mandatory RED Evidence

**Date**: 2026-07-28
**Baseline**: `3a2c3da5f1682e1acabfebefb6c55e5a57183a20`
**Result**: RED captured; implementation may begin

All commands ran from the repository root after the fresh independent Contract
Review returned `PASS`. The failures below are caused by product behavior
frozen in the W1 contract that does not yet exist. No network, resident daemon,
Provider, Runtime execution, or authoritative write was involved.

| Command | Expected missing behavior observed |
|---|---|
| `go test ./internal/projection` | `GlobalReadView` has no bounded copied `Teams`, `Runs`, `EvidenceRecords`, `RuntimeInstances`, or `TeamExecutions` enumeration and no legacy timeline anchor. |
| `go test ./internal/api` | `LocalProductReadService`, its typed snapshot/timeline requests, and safe stale-view DTOs do not exist. |
| `go test ./internal/localipc` | The framed v1 request/response protocol, private socket server/client, bounds, lifecycle, and safe error surface do not exist. |
| `go test ./internal/tui` | The Bubble Tea model/program and typed local-product API DTOs do not exist. |
| `go test ./cmd/loom` | `runDeps` has no primary app/TUI route, so no-argument invocation cannot start the product. |
| `go test ./cmd/loomd` | `--socket` is rejected as invalid input, so the optional read server is not composed with the observer. |
| `scripts/test-install-loom-local-product.sh` | It exits with `RED: local product installer is missing`. |

The first batched run used a 30-second command timeout while the new dependency
cache was cold. Any package that did not return in that bound was rerun
individually to completion. Test-only JSON/struct comparison helpers were added
before the final API RED rerun, so no accepted RED relies on a missing test
helper, syntax error, unavailable network, or weakened fixture.

The RED gate is satisfied. Production implementation remains limited to the
exact owned files and read-only authority boundary in the frozen contract.
