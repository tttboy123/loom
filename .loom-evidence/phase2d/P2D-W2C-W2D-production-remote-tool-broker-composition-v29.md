# P2D-W2C/W2D Production Remote Tool Broker Composition V29

Status: `SOURCE VERIFIED / BACKEND ENROLLMENT AND INSTALLED LIVE OPEN`  
Date: 2026-08-15  
Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Acceptance boundary

The existing ATL3 Broker and crash-safe result path can now be composed by the
production Work Bundle without injecting an already-assembled remote executor.
This slice establishes ownership, exact capability publication and shutdown.
It does not enroll a Search provider or MCP Server in the installed App and it
does not perform a real network call.

## Implementation

- `toolbroker.New` accepts independently backed Search, WebFetch and MCP
  capabilities and publishes only those exact tools.
- An MCP allowlist requires a matching client. Missing backends, empty
  capability sets, duplicate/invalid allowlists and invalid timeout/result
  ceilings fail closed.
- `productRemoteToolBrokerConfig` carries bounded Search/MCP ports, an explicit
  WebFetch opt-in and non-secret limits. It contains no endpoint credential,
  Authorization header, API key, Prompt or Provider content.
- `loom-work` constructs the Broker during Bundle Start. The product builder
  passes configuration only and never calls the Broker constructor.
- The owned executor has a separate lifecycle context. Composition rollback or
  Dispose cancels active calls, closes idle system HTTP connections, revokes
  the delegate and makes retained references reject validation/execution.
- Work construction failure closes the Broker Effect before its Evidence
  store, preserving reverse ownership cleanup.
- Existing dispatch, permission, Attempt binding, encrypted result commit,
  terminal ordering and Harness delivery contracts are unchanged.

## Default behavior and privacy

- `productionDaemonBuilder` supplies no remote-tool config, so ordinary App
  startup advertises no WebSearch, WebFetch or MCPTool capability.
- No ambient environment, global proxy, arbitrary endpoint, model-supplied MCP
  server or dynamic plugin is consulted by composition.
- The hardened WebFetch client disables ambient proxy use and retains the
  existing public HTTPS, DNS, redirect, content-type and result-size controls.
- Search query, fetched body, MCP arguments/result and URL content remain in
  the existing bounded encrypted payload path; they are not added by this
  composition layer to snapshots, Journal, Evidence metadata or diagnostics.

## Verification

Passed:

```text
go test ./internal/toolbroker -count=1
go test ./internal/execution -count=1
go test ./cmd/loomd -run 'Test(ProductRemoteToolBroker|COMP2CWork|ProductAttemptLoopPersistsRemoteResultBeforeExecutionTerminal)' -count=1
go test -race ./internal/toolbroker ./internal/execution -count=1
go test -race ./cmd/loomd -run '^TestProductRemoteToolBroker' -count=20
go test ./cmd/loomd -count=1
go test -p 1 ./... -count=1
go vet ./internal/toolbroker ./internal/execution ./cmd/loomd
gofmt and git diff checks on the affected source set
```

The complete daemon suite passed in 74.175 seconds. The fresh serial repository
run passed every package, including credentials, Vault, Local IPC, Pi adapter,
Harness adapters, Tool Broker and Work authority.

## Remaining

- explicit persisted Search backend and MCP Server/Tool enrollment;
- Provider Account disclosure, retention, budget and approval policy binding
  for each enrolled backend;
- Swift governance UI and safe per-Agent capability/preflight projection;
- installed WebSearch -> WebFetch -> final and MCP continuation matrix;
- cross-Runtime restart recovery, CV6, mixed-Team ATL9 and COMP2-E.

No App, network, external MCP, Provider, real credential, user workspace or
external Runtime was accessed for this source verification.
