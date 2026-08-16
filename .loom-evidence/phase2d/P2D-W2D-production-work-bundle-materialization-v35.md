# P2D-W2D Production Work Bundle Materialization V35

Status: `SOURCE VERIFIED / RUNTIME MATERIALIZATION WIRED / INSTALLED LIVE OPEN`

Date: 2026-08-16

## Acceptance boundary

V35 closes the last open "runtime client materialization" source gap: the
default production daemon composition now consumes persisted Remote Tool Backend
Enrollments through the trusted Work Bundle materialization boundary instead of
relying only on an injected test seam. It does not add a real network Search or
MCP transport, install an App, use a Provider or credential, or publish any
remote capability. Default production (no persisted Enrollment and no injected
ports) still exposes no remote tool capability.

## Implemented source

- `projection.GlobalReadView.RemoteToolBackendEnrollmentCatalog()` returns every
  valid persisted Enrollment across all Provider Accounts, sorted
  deterministically by Provider / Account / Enrollment ID.
- `cmd/loomd/product_remote_tool_enrollment_materializer.go`:
  - `productRemoteToolEnrollmentDeps` maps the daemon's injected typed ports
    (Search backend / MCP clients) into the materialization deps.
  - `newProductRemoteToolExecutorsFromEnrollments` iterates the catalog and
    materializes only active + policy-current Enrollments whose Adapter ID is
    in the trusted built-in catalog and whose typed ports are available.
    Revoked, policy-drifted, unsupported-adapter and port-less Enrollments are
    skipped per-Enrollment (their bound Agents are already blocked by V32
    preflight) and never produce a capability. It returns nil when nothing
    materializes.
  - `productCompositeRemoteToolExecutor` fans proposals out over the explicit
    materialized set, unions `AllowedRemoteTools`, validates against any member,
    executes through the matching member, zeroes content if the shared lifecycle
    closes mid-call, and its `Close` is idempotent.
- `cmd/loomd/product_composition_work.go` wires the materialized executor into
  the execution adapter alongside the injected broker: default (none) keeps the
  adapter remote-tool unavailable; a single source uses it directly; both
  sources combine under one composite whose close joins the member effects.

## Verification

Passed:

- `go test ./cmd/loomd -run 'TestProductRemoteToolExecutorsFromEnrollmentsMaterializesOnlyTrustedActive|TestProductCompositeRemoteToolExecutorCloseIsIdempotent'`
- `go test -race ./cmd/loomd -run 'TestProductRemoteToolExecutorsFromEnrollmentsMaterializesOnlyTrustedActive|TestProductCompositeRemoteToolExecutorCloseIsIdempotent|TestProductRemoteToolBroker'`
- `go test ./cmd/loomd` (complete daemon suite with the new wiring)
- `go test ./internal/projection` (complete; catalog assertions added to the
  existing enrollment projection test)
- `go vet ./cmd/loomd ./internal/projection`, `gofmt -l` clean,
  `git diff --check` clean, `go build ./...`
- Full `go test ./...` re-run: no new failures.

The materializer test proves: with an injected Search port, exactly the trusted
active Enrollment materializes (unsupported adapter and revoked records are
skipped) and executes a bounded WebSearch; without a port nothing materializes;
after revocation the same record stops materializing. The composite close is
idempotent and a closed composite exposes no tools.

## Privacy and live status

No App was installed or launched, no network request, Provider, real credential
or user workspace was used, and no remote tool capability was published. The
installed-live G5 gate remains operator-executable via the V33 acceptance
runbook; this slice makes that execution consume persisted Enrollments through
the same trusted boundary. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
