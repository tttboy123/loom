# G5 — Installed Web/MCP diagnostics (installed-live)

Status: `PASS` (installed-live; port-less fail-closed + revocation isolation)

Date: 2026-08-16

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

The default production daemon ships no Search/MCP port, so
`newProductRemoteToolExecutorsFromEnrollments` materializes nothing and
exposes no unbound remote capability; a bound Agent whose Enrollment is valid
+ policy-current is dispatchable, and revocation fails closed per-Agent.

## Fixes that unblocked this gate (V34)

- `productRuntimeProfileFromRecord` and `resolveMissionExecutionProfile` now
  carry `RemoteToolEnrollmentID`/`Digest`, and `MaterializeConfirmedTeam`
  threads the materialization profiles through the saved-Team binding, so the
  Enrollment pair survives confirm → materialization → preflight.
- `internal/localipc` `validMethod` + Swift allowlist accept the enrollment
  configure/revoke routes.

## Safety

No remote tool capability is published by default; real Search/MCP execution
requires an operator-injected typed port. No credential, Prompt, Provider
body, or user workspace entered source, logs, or evidence. Evidence files are
owner-only (`0600`).
