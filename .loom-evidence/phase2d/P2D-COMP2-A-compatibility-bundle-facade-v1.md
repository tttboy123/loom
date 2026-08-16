# P2D-COMP2-A compatibility Bundle facade v1

Date: 2026-08-14

Status: `SOURCE VERIFIED / COMP2-B ROUTE MIGRATION OPEN`

This evidence belongs to the sole Phase 2D Goal. It does not create a new Goal,
complete P2D-COMP2, or authorize installed/live work.

## Outcome

The production daemon now activates one desktop Composition Snapshot after the
existing product services and legacy handler are assembled but before
`localipc.NewServer` can admit a socket. Desktop, headless, and test profiles
compile the same nine versioned compatibility Bundles and one explicit legacy
dispatch route.

The facade delegates every admitted request to the exact existing wrapped
handler. It does not change method routing, response schemas, state paths,
Provider clients, Vault access, per-Agent binding, Context Capsule behavior, or
the manual service construction root.

## Admission and shutdown

- `loom-local-ipc` owns the compatibility handler capability.
- Bundle marker ports encode the initial Core, assets, observability,
  governance, Vault, Conversation, work, Agent Runtime, and local IPC dependency
  order without exposing the underlying service objects.
- The handler checks atomic composition readiness on every call.
- Close takes an exclusive admission lock, waits for already admitted handlers,
  closes the snapshot lifecycle, and rejects every later call through old
  handler references with `state_unavailable / daemon_admission`.
- Product shutdown remains IPC, composition facade, then the unchanged legacy
  resource order. Construction failure closes composition before legacy
  resources. Nil compatibility state remains valid only for older isolated
  runner fixtures.

## Diagnostics

Composition compile, validate, route, Bundle register/start/ready/stop/dispose,
and scope open/close records use the existing owner-only, bounded, rotating
operational JSONL store. Records include only Incident/Profile/Snapshot,
Bundle/scope identity, stage, elapsed time, result, controlled code, and
retryability.

Every required startup record is persisted before production server creation.
If persistence fails, facade activation fails closed and the legacy handler is
never called. Existing credential/chat diagnostic tests now parse the JSONL
stream rather than assuming it contains exactly one record.

## Verification

Passed:

```text
go test -race ./cmd/loomd -run '^TestCOMP2A' -count=10
go test ./...
go vet ./...
gofmt -d ...
git diff --check
```

Focused tests cover all three Profile manifests and deterministic snapshot
shape, exact response delegation, invalid admission, concurrent quiescence,
old-reference revocation, all ten diagnostic stages, diagnostic write failure,
and real production-builder activation before `Run`. The full daemon suite
proves existing setup, Credential Vault, Conversation, execution, lifecycle,
socket, and restart behavior remains green.

Canonical snapshots and diagnostics do not contain request/response content,
Prompt, transcript, tool result, Provider response, credential, VMK,
Authorization header, state path, or service object.

## Open gates

- COMP2-B complete method-level RouteDescriptor and typed registry migration;
- COMP2-C/D service construction extraction and operational scope ownership;
- COMP2-E legacy composition removal after parity/restart/privacy gates;
- installed CV6 and mixed-Team ATL9 acceptance.

No App bundle was built, signed, launched, or installed. No credential,
Provider, network, user workspace, or external tool was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.
