# P2D-COMP1 governed composition kernel v1

Date: 2026-08-14

Status: `SOURCE VERIFIED / PRODUCT ACTIVATION OPEN`

This evidence belongs to the sole Phase 2D Goal. It does not create a new Goal,
activate product Bundles, or close P2D-COMP2.

## Outcome

Loom now has a deterministic in-process composition kernel that compiles one
closed Launch Profile and exact built-in Bundle descriptors into an immutable,
content-addressed Composition Snapshot. It validates the complete dependency
and route graph before starting resources, publishes Ready atomically, stops in
forward dependency order, and disposes registration/start Effects in reverse
order.

Desktop, headless, and test Profile manifests freeze the same nine Loom-owned
Bundle identities with exact versions while retaining their distinct native UI,
diagnostic, and injected-dependency policies. Product service descriptors and
their full route manifests remain COMP2-A/B work; no production path uses this
kernel yet.

## Authority boundary

- `BundleContext` is a read-only lifecycle view containing only capabilities
  declared by the frozen Bundle descriptor.
- Protected Core capabilities can be provided only by `loom-core`; they are not
  visible to non-Core Bundles and are automatically masked at Root-to-Product.
- Operational scopes are exactly Root, Product, Conversation, Team, Agent,
  Attempt, and Turn. Child scopes can inherit, mask, or narrow admitted ports,
  but cannot add a missing capability or bind protected authority.
- Attempt scope freezes both Composition Snapshot digest and independent Frozen
  Execution Binding digest. A later snapshot does not rewrite an active Attempt.
- Capability Context, Go `context.Context`, Attempt Context, and Context Capsule
  remain separate. The package imports only the Go standard library and has no
  Journal, Vault, Provider, Prompt, Evidence, or execution-authority dependency.

## Compiler and lifecycle gates

The compiler rejects unknown Profile/Bundle/capability versions, non-catalog
Bundle identities, missing or cyclic dependencies, duplicate providers/routes,
hidden Route dependencies, protected overrides, compatibility mismatch, and
factory registration that differs from the descriptor.

Validation starts zero resources. A failed hook's returned partial Effect is
also retained for cleanup. Close is concurrent and idempotent; scope-owned
Effects expire with their scope. Lifecycle and scope diagnostics use controlled
codes and metadata-only identities, and recorder failure is observational.

## Verification

Passed:

```text
go test ./internal/composition -count=1 -coverprofile=...
  83.0% statement coverage
go test -race ./internal/composition -count=20
go test ./...
go vet ./...
git diff --check -- internal/composition
```

Focused tests cover deterministic input reordering and concurrent compile,
desktop/headless/test manifests, graph and Route failures, protected authority
isolation, factory consumption validation, partial-start rollback, Ready/Stop/
Dispose ordering, scoped narrowing and concurrent close, frozen Attempt digests,
snapshot clone isolation, safe diagnostics, and schema/privacy-negative cases.

The privacy scan found sensitive words only in validator deny markers and
negative tests. Canonical snapshots and diagnostic records contain no secret,
Prompt, transcript, tool arguments/results, Provider body, or internal service
state.

During COMP2-C Agent Runtime migration, startup reconciliation proved that the
original lifecycle wrapper discarded underlying `errors.Is` identity. The
repaired `lifecycleFailure` retains a controlled metadata-only Error string and
unwraps both the Composition category and original cause. Focused privacy tests
prove private source text remains absent; exact mission conflict identity and
the full repository/race matrix now pass.

## Open gates

- remaining COMP2-C service construction after source-verified compatibility,
  declarative routes, Assets route, and Agent Runtime slices;
- deeper COMP2-D scoped lifecycle migration after source-verified Product scope;
- COMP2-E legacy-path removal after parity/race/restart/privacy/shutdown gates;
- sequential ToolCall and later ATL expansion after COMP2-C/D;
- installed Credential Vault CV6 and mixed-Team ATL9 acceptance.

No App bundle was built, signed, launched, or installed. No credential,
Provider, network, user workspace, or external tool was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39.
