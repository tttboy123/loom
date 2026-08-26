# G5 — Installed Web/MCP diagnostics (installed-live)

Status: `PASS / INSTALLED LIVE`

Date: 2026-08-23

## What was executed

1. Installed the App, daemon online. DeepSeek broker account verified;
   Provider Account Policy + Model Rate Card configured.
2. Configured a `web_search` Enrollment (`builtin.search.deepseek.v1`) and an
   `mcp_server` Enrollment (`builtin.mcp.stdio.v1`) under `deepseek.primary`.
   Both appear in the account's `remote_tool_backends` directory as
   `active` + `policy_current`.
3. Built a Team (DeepSeek main + one DeepSeek subagent) and bound the
   `web_search` Enrollment to the subagent through the builder
   (`subagent_remote_tool_enrollment`), frozen into the saved TeamDefinition +
   materialized TeamInstance execution profile.
4. `preflight` → the Enrollment-bound subagent resolves `ready` (valid +
   active); the unbound main is `ready`.
5. Revoked the `web_search` Enrollment → the directory shows `revoked`; the
   bound subagent's next `preflight` blocks with the exact reason
   (`Remote tool enrollment was revoked. Restore or re-select it for this
   Agent.`) while the unbound main stays `ready`.
6. Installed build 109/111 production composition materialized a private MCP
   stdio server from the persisted active Enrollment. An OpenCode Agent called
   the exact `mcp.live.probe/lookup` tool. The gate joined the exact
   `ToolExecutionProposed` and `ToolExecutionCompleted` facts by call digest and
   expected output digest; unrelated ToolCalls cannot satisfy the gate.
7. A second mixed Team bound MCP only to its OpenCode main and kept a Loom
   Native peer unbound. Mid-flight Enrollment revoke cancelled the blocked MCP
   process, prevented result commit, and failed/blocked only the OpenCode Agent;
   the healthy peer succeeded. The next preflight reported the exact revoked
   Enrollment reason only on the bound Agent.
8. Build 112 preserves `remote_tool_binding_revoked` through the execution
   adapter, cancels the exact Harness context, and uses an uncancelled authority
   context to commit failed Step/Turn terminal facts. Installed recovery then
   consumed explicit fallback approval version 1 and scheduled Attempt 2. The
   available fallback Provider returned `provider_http`, so terminal fallback
   success remains a separate open gate.
9. Build 113 removes the global fallback publication for Enrollment-backed
   remote tools. The exact scope is revalidated after backend return, before
   encrypted payload write and after the write. A second-check failure deletes
   the exact encrypted payload before acceptance; cleanup failure is
   content-free and terminal. The installed helper/revoke/fallback-dispatch
   path ran again. MiniMax Attempt 2 remained `provider_http`; an explicit
   DeepSeek fallback retry was inconclusive because OpenCode omitted the
   required ToolCall and therefore never reached the helper.
10. Build 117 makes the installed fallback injection deterministic without a
    production test hook. A pre-dispatch observer waits for the private helper's
    active-call marker, revokes the exact Enrollment revision through a separate
    authenticated UDS client, and releases the helper. OpenCode Attempt 1 fails,
    approved Loom Native Attempt 2 succeeds, the healthy peer succeeds, and the
    Team terminalizes `succeeded` with account-level usage.

## Result

```
post-revoke directory enroll-mcp-live-... status=active
post-revoke directory enroll-web-live-... status=revoked
post-revoke node main provider=deepseek status=ready
post-revoke node mission-role-... provider=deepseek status=blocked
  block="Remote tool enrollment was revoked. Restore or re-select it for this Agent."
bound subagent before="ready" after="blocked" (isolation confirmed)
--- PASS: TestLiveRemoteToolEnrollmentIsolationE2E
```

The default production daemon exposes no unbound remote capability. The
installed test explicitly supplies one persisted, typed, policy-current MCP
Enrollment under the private isolation root. Mission permission materialization
grants only the exact frozen WorkItem's `server/tool` path. Revocation is
revalidated during the in-flight call and fails closed per Agent.

Build 117 extends the recovery proof through terminal success. The exact source
WorkItem had one scoped permission binding; revocation cancelled only that
Attempt. Fallback approval version 1 was consumed by a fresh Loom Native +
DeepSeek Attempt 2, which passed independent verification and recorded 2,792
tokens. The unaffected peer recorded 3,359 tokens.

## Fixes that unblocked this gate (V34)

- `productRuntimeProfileFromRecord` and `resolveMissionExecutionProfile` now
  carry `RemoteToolEnrollmentID`/`Digest`, and `MaterializeConfirmedTeam`
  threads the materialization profiles through the saved-Team binding, so the
  Enrollment pair survives confirm → materialization → preflight.
- `internal/localipc` `validMethod` + Swift allowlist accept the enrollment
  configure/revoke routes.
- The dynamic Enrollment executor polls the exact Enrollment identity and
  policy revision during an in-flight call, cancels on revoke/drift, then
  revalidates before any result commit.
- Generic MCP tool annotations are conservative: not read-only, potentially
  destructive, non-idempotent and open-world unless a trusted adapter provides
  stronger typed metadata.

## Safety

No remote tool capability is published by default; real Search/MCP execution
requires an operator-injected typed port. No credential, Prompt, Provider
body, or user workspace entered source, logs, or evidence. Evidence files are
owner-only (`0600`).
