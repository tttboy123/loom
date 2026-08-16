# P2D-W2C Codex and Claude Context MCP Transport v1

**Date**: 2026-08-13  
**Status**: `CURRENT / SOURCE VERIFIED / POST-BUILD-64`  
**Goal**: unified Phase 2D only  
**WorkItem**: existing P2D-W2C with P2D-W2D diagnostics and accounting

## Result

Codex and Claude Code Harness Attempts can now invoke the existing scoped
`loom_read_context` broker through one private Attempt-scoped MCP service. The
service is created only after the frozen execution binding, Capsule Authority,
dispatch Capsule digest, disclosure receipt, Agent, Provider Account, Model,
and auth mode agree. Capability and retriever mismatches fail before a
credential lease or Harness process.

The service listens only on an ephemeral IPv4 loopback port and uses one random
256-bit bearer capability delivered through a minimal child environment. It is
not placed in argv, the MCP URL, Prompt, Journal, Evidence, or diagnostics.
The service exposes exactly one read-only tool, permits one successful call,
reuses the scoped Context broker, revalidates item/content digest, artifact
scope, trust, source class, and returned content digest, and zeroizes the
mutable retrieved body when the call ends.

Codex starts with user configuration ignored, a single private MCP server, an
exact enabled tool, and per-tool approval. Its real CLI flow performs deferred
tool search, the namespaced MCP call, the function result round, and final
assistant output. Claude starts with strict MCP configuration and the exact
allowed MCP tool. The transport accepts the bounded standard client metadata
used by both CLIs but ignores it for authorization; unknown fields, secret-like
metadata, controls, excess depth, duplicate keys, batch calls, and protocol
drift fail closed.

## Exact executable conformance

Runtime discovery does not infer Context retrieval from a CLI name or version.
It publishes `context_retrieval` only for executable SHA-256 identities that
passed the real component contract:

- Codex 0.144.1: `sha256:29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a`
- Claude Code 2.1.196: `sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a`

Any executable-byte drift retains ordinary Harness capabilities but receives no
Context retrieval capability. New Profiles require `context_retrieval` only
when the observed Runtime publishes it; the resulting Attempt freezes that
capability in its execution-binding digest. Exact historical Runtime records
with only the prior capability set are eligible for one append-only capability
upgrade. Unknown capability sets or any other identity drift remain rejected.

Every Context-enabled Attempt re-hashes the exact executable and rechecks the
attestation after binding/dispatch/Authority validation but before opening the
MCP listener, acquiring a credential, or starting the Harness. Replacing the
CLI after Runtime discovery therefore fails closed rather than inheriting the
old projection's capability.

The attestation is closed and source-controlled. It cannot be extended through
environment variables or user configuration. A new CLI release therefore
requires a new real component gate and explicit attestation update.

## Claude Attempt Gateway compatibility

Real Claude Code 2.1.196 uses `/v1/messages?beta=true` and performs one
unauthenticated `HEAD /` loopback health probe. The Attempt Gateway now accepts
only that exact Anthropic query and forwards it to the exact Provider endpoint.
All query drift remains rejected. The health probe returns an empty local 204,
does not access the credential, does not call the Provider, does not consume the
inference request budget, and is not available on the OpenAI Gateway.

## Verification

- MCP unit tests cover both protocol versions, exact client fields, list/call
  metadata, one-use reads, authorization, close, zeroization, metadata secret
  rejection, malformed fields, duplicate keys, and unknown methods.
- Gateway tests cover exact Anthropic beta forwarding, query drift, local
  health probe isolation, Provider credential substitution, and rejection of
  the same health probe on OpenAI.
- Runtime tests cover exact executable attestation, hash drift, cross-adapter
  substitution, observed capability publication, Profile capability freezing,
  exact historical eligibility, and identity/capability drift rejection.
- The real Codex and Claude binaries use isolated temporary HOME/workspace
  roots, loopback fake Providers, controlled fake keys, and no external
  network. Each completes its real MCP lifecycle and final result. The combined
  component matrix passed 20 consecutive race-enabled runs.
- The directly affected daemon Runtime/Profile matrix passed 20 race-enabled
  runs. `go test ./...`, `go vet ./...`, `go mod verify`, and
  `git diff --check` pass.

The broader `go test -race ./internal/runtime/harnessadapter ./internal/runtime
./cmd/loomd -count=3` gate did not pass: an unrelated existing real-Pi timeout
test failed because the subsequent setup/Codex IPC read became unavailable.
An isolated 10-run reproduction failed twice with `invalid local IPC protocol`,
confirming a pre-existing timeout/IPC race outside this Harness Context path.
This evidence does not rewrite that failure as a pass.

## Open boundary

This closes the bounded Codex and Claude Code Attempt-scoped Context MCP source
transport and exact-binary capability-freeze slice. It does not provide a
general multi-tool loop, persisted tool-result delivery or crash resume,
parallel sibling/Aggregation Attempts, encrypted export, or installed CV6
acceptance.

Build 64 predates this source. No bundle was built, launched, installed, or
connected to a real Provider. Installed Loom remains v0.5.2 build 39 and was not
modified. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
