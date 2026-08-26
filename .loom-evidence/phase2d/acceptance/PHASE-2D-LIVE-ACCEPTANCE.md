# Phase 2D Installed-Live Acceptance Matrix and Runbook

Status: `PHASE 2D ACTIVE SCOPE ACCEPTED / DEFERRED CELLS PRESERVED`

Date: 2026-08-25

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

This runbook records both active Phase 2D gates and preserved future-Phase live
scenarios. Installed-live evidence is recorded only where the exact gate ran
against a real App bundle; source tests and narrower live slices remain
explicitly partial. Rows marked `DEFERRED` are not Phase 2D completion gates.

## Current gate ledger

| Gate | Status | Current evidence and remaining boundary |
| --- | --- | --- |
| G1 Installed Vault | `CORE LIVE PASS / CUSTOM ENDPOINT DEFERRED` | DeepSeek and MiniMax import/verify/restart and normal no-helper runtime are live. Build 124 preserves 25 Providers, 7 Runtimes, 6 Conversation Profiles and 5 privacy-safe CC Switch candidates; owner-only Vault files and controlled legacy registry permission migration pass. A real approved custom credential import and executable frozen custom binding are preserved for a later Phase. |
| G2 Real conversation | `ACTIVE SCOPE PASS / BUILD 127 TRUST + CAPACITY + ROUTE ACCEPTED` | Installed 28/28 advertised model/reasoning cells pass with immutable Segment metadata; same-visible-Conversation Codex-to-DeepSeek switching and independent sessions pass. Build 112 adds one product-level real-Vault transcript + Capsule close/reopen gate with deterministic omission retrieval, forged-role denial and no plaintext in DB/sidecars. Build 113 adds an exact privacy-safe disclosure-inspector route and fixed four-Segment installed Conversation. Build 124 returns the exact real OpenCode + DeepSeek reply before and after an App-managed daemon restart. Build 127 closes trust-domain acknowledgement, exact/estimated/unavailable capacity authority and the authenticated private-UDS Route Transition matrix; installed UI and App-managed daemon restart pass without a new external Provider request. |
| G3 Mixed Team | `TWO-PROVIDER LIVE PASS / FOUR-PROVIDER DEFERRED` | Installed build 85 completed a fresh four-Agent DeepSeek/MiniMax Mission 4/4. Build 88 additionally completed a Loom Native/DeepSeek + OpenCode/DeepSeek Mission with independent Harness/Provider/model bindings, proving OpenCode is no longer modeled as a Provider. Build 97 passed both native OpenCode and OpenCode with Vault-backed DeepSeek two-node Teams; build 101 retains all seven executable-attested Runtimes and the 25-Provider directory. Codex/OpenAI + Claude/Anthropic + Loom/Kimi + Loom/MiniMax in one installed Team is preserved for a later Phase. |
| G4 Failure isolation | `CONTROLLED + CREDENTIAL/ENROLLMENT LIVE PASS / REAL ACCOUNT MATRIX DEFERRED` | Installed MiniMax credential revoke plus Web/MCP Enrollment revoke are Agent-local, and Build 117 completes approved OpenCode-to-Loom-Native recovery while its peer continues. Build 121 runs auth, rate limit, timeout, insufficient balance, corrupt Vault record and credential-revision conflict through a credential-free local Failure Lab over the real private UDS; each affected Agent gets the exact safe stage/code while its healthy peer succeeds. Real Provider Account revoke/rate-limit execution is preserved for a later Phase. |
| G5 Web/MCP diagnostics | `INSTALLED LIVE PASS + TERMINAL FALLBACK PASS` | Governed installed WebSearch/WebFetch and Enrollment isolation pass. Build 111 performs an exact real MCP stdio ToolCall from the bound OpenCode Agent. Build 113 removes unscoped remote capability and rolls back encrypted result commit on drift. Build 117 revokes the exact Enrollment during a real helper call, consumes approval version 1, succeeds as Loom Native Attempt 2, and preserves one scoped source permission fact. |
| G6 Accounting/governance UI | `SOURCE COMPLETE / INSTALLED MISSION + TEAM VISUAL + FALLBACK ACCOUNTING PASS` | Build 111 adds separate Token presentation, exact accounting coverage, errors/rate limits, cost source, policy revision and concurrency/dispatch/budget ceilings. Build 117 visually verifies Mission/Team per-Agent routes and completes approved fallback accounting: 2,792 fallback tokens and 3,359 healthy-peer tokens with independent rate-card rows. The four-Provider visual moves with G3 to a later Phase. |
| G7 Harness Gateway / Segment Session | `INSTALLED PASS / BUILD 136` | The installed App exported one privacy-safe v3 Gateway trace bound to its exact Build 136 App and daemon hashes. The machine verifier proves two Codex completions before a reviewed Loom Native/DeepSeek Segment opens, two target responses with independent Attempt Capsules in one immutable target Session, cross-Conversation overlap, exact cancellation and later reuse of the cancelled Session. The closed export excludes credentials, Prompt/transcript content and Provider bodies. |

## Scope decision — 2026-08-24

Status: `ACCEPTED / DEFERRED / NO FUTURE PHASE ACTIVATED`

The following installed-live scenarios are preserved in this runbook but no
longer block Phase 2D: the four-distinct-Provider Team, real Provider Account
revoke or rate-limit isolation, and the approved custom-endpoint import through
frozen binding and real Conversation. Existing implementations and evidence are
not rolled back. A later Phase must explicitly reactivate these gates before
using them as completion criteria.

The current accepted evidence includes
`.loom-evidence/phase2d/P2D-W2A-W2D-trust-capacity-route-build127.md`,
`.loom-evidence/phase2d/P2D-W2A-W2D-full-review-remediation-build124.md`,
`.loom-evidence/phase2d/P2D-W2A-W2D-installed-model-segment-matrix-v54.md` and
`.loom-evidence/phase2d/P2D-W2D-installed-live-deepseek-mixed-isolation-v51.md`,
plus the successful build 85 mixed-Team slice at
`.loom-evidence/phase2d/P2D-W2D-installed-mixed-team-build85.md` and the build
88 OpenCode Agent Team slice at
`.loom-evidence/phase2d/P2D-W2B-W2D-opencode-agent-team-build88.md`, plus the
build 91 Runtime/Provider Account separation slice at
`.loom-evidence/phase2d/P2D-W2B-W2D-installed-runtime-account-separation-build91.md`,
and the build 92 CC Switch import and catalog recovery slice at
`.loom-evidence/phase2d/P2D-W1-W2D-ccswitch-runtime-recovery-build92.md`, plus
the build 99 OpenCode/Mission/RoundTable slice at
`.loom-evidence/phase2d/P2D-W2B-W2D-opencode-dynamic-catalog-build99.md`, plus
the build 101 Runtime and workflow UX slice at
`.loom-evidence/phase2d/P2D-W2A-W2B-W2D-runtime-workflow-ux-build101.md`, plus
the build 108 installed conversation and governance slice at
`.loom-evidence/phase2d/P2D-W2A-W2D-runtime-governance-context-build108.md`, plus
the build 111 MCP/accounting/UX slice at
`.loom-evidence/phase2d/P2D-W2D-installed-mcp-accounting-ux-build111.md`, plus
the build 112 cancellation/fallback slice at
`.loom-evidence/phase2d/P2D-W2D-enrollment-cancel-fallback-build112.md`, plus
the build 113 disclosure/commit-hardening slice at
`.loom-evidence/phase2d/P2D-W2A-W2D-context-inspection-enrollment-hardening-build113.md`,
plus the Build 117 installed Mission/Team binding UI slice at
`.loom-evidence/phase2d/P2D-W2B-W2D-mission-team-binding-ui-build117.md`, plus
the Build 121 endpoint and controlled failure-isolation slice at
`.loom-evidence/phase2d/P2D-W1E-W1G-W2D-endpoint-failure-lab-build121.md`.
Historical evidence remains immutable and may describe an earlier open state.
Historical gate files never satisfy a newer bundle automatically. Current gate
evidence must record the exact `CFBundleVersion`, App executable SHA-256 and
bundled daemon SHA-256 produced by the acceptance harness.

## Operator prerequisites

1. **Build and install the App** (unsigned, ad-hoc signed):
   - `scripts/build-loom-local-app.sh --output /absolute/path/Loom.app`
   - `scripts/install-loom-local-app.sh --app /absolute/path/Loom.app --destination $HOME/Applications/Loom.app`
   - G7 accepts only this owner-controlled canonical user installation; the
     system `/Applications` directory is not an acceptance destination.
   - Gate: `scripts/test-build-loom-local-app.sh` and
     `scripts/test-install-loom-local-app.sh` PASS before installing.
2. **Start the App** and unlock the Loom-owned Credential Vault (explicit
   Unlock; no Keychain fallback in normal operation).
3. **Real Provider credentials** for the active gate being run. The complete
   account set below is required only when deferred G3/G4 is reactivated:
   - OpenAI/Codex native runtime, Anthropic (brokered), DeepSeek (brokered),
     Kimi (brokered), MiniMax (brokered). Configure and **Verify** each in the
     Provider Account surface; the exact credential revision is frozen per
     Agent and Conversation Profile.
4. **Runtime harnesses** available to the App: Codex CLI, Claude Code CLI, and
   the Loom Pi runtime (or their configured discovery paths).
5. **Explicit operator approval** for: installing the App, network calls to
   the selected Providers, Web Search and MCP Enrollments, and live agent
   execution. Nothing in this runbook runs without that approval; no credential
   ever enters source, scripts, logs or evidence.

## Gates and pass criteria

### G1 — Installed credential import (CV6)

The shipped-Provider Vault lifecycle is accepted for Phase 2D. The custom
endpoint portion below is `DEFERRED / FUTURE PHASE`.

- Configure an API key in the Provider Account sheet for each provider;
  verify; restart the App; Unlock the vault; confirm the account remains
  configured with the same credential reference and revision.
- Revoke one account; restart; confirm revocation persists and no helper/
  Keychain path is used in normal operation.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G1-baseline-build-launch.md`.

### G2 — Real conversation

- Open the App, pick a Conversation Profile bound to a verified account
  (e.g., DeepSeek `conversation-deepseek-deepseek-chat-account-<id>-r<rev>`),
  send a user message, receive a bounded model reply in the same thread.
- The thread shows one Attempt with the frozen Execution Binding, an Incident
  correlation, and content-free diagnostics; no Prompt, key or Provider body
  in diagnostics or the Journal.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G2-real-conversation-opencode.md`.

### G3 — Mixed Provider Team (ATL9)

Status: `DEFERRED / FUTURE PHASE` for the four-distinct-Provider live matrix.
Existing two-Provider installed evidence remains accepted Phase 2D evidence.

- Build a 4-Agent Team where each Agent binds an independent Provider Account
  and model (OpenAI/Codex, Anthropic/Claude, DeepSeek, Kimi or MiniMax) with
  exact credential revisions and reasoning effort/limits.
- Run the Team to completion; every Agent Attempt uses its own Vault lease and
  frozen binding; the Team board shows per-Agent and per-Account rows.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G3-mixed-team-atl9.md`.

### G4 — Single-Agent failure isolation matrix

Status: `CONTROLLED MATRIX ACCEPTED / REAL ACCOUNT REVOKE OR RATE LIMIT
DEFERRED`. The table remains the preserved future live procedure.

For each cell, run the mixed Team and inject the failure on **one** sub-agent's
account while peers stay healthy:

| Failure class | Expected | Source proof |
|---|---|---|
| Credential revoked | only that Agent blocks; peers succeed; Team never globally offline | `TestFourProviderTeamRevokedCredentialIsolatesOneAgent` |
| Corrupt vault record | only that Agent blocks; peers succeed | `TestFourProviderTeamCorruptVaultRecordIsolatesOneAgent` |
| Credential revision conflict | only that Agent blocks; peers succeed | `TestPhase2DTeamExecutionUsesOnlyExactApprovedFallbackBinding` family |
| Provider rate limit | only that Agent gets the retryable rate-limit diagnostic; peers unaffected | `P2D-W2D-account-local-provider-timeout-classification` focused tests + `TestPhase2DPerAgentFailureIsolationMatrix` |
| Provider timeout | only that Agent gets `timeout/provider_http/retryable`; peers unaffected | same |
| Enrollment revoked / policy drift | only the bound Agent's preflight blocks | V32 preflight tests |

- Source proof: `TestPhase2DPerAgentFailureIsolationMatrix` (V34) drives
  `provider_rate_limited` / `provider_timeout` / `provider_insufficient_balance`
  / `credential_unavailable` cells; affected Agent `blocked`/`failed` with the
  exact reason, all peers `succeeded`.
- Live pass: the affected Agent row shows a closed diagnostic, retry review
  and Incident copy; healthy peer and accounting rows inherit no actions.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G4-failure-isolation-matrix.md`.

### G5 — Installed Web/MCP diagnostics

- Configure a Web Search Enrollment and an MCP Enrollment under a verified
  account; bind one to an Agent; run an Attempt that uses the tool; verify the
  bounded result commit, tool diagnostics and Incident correlation.
- Source proof (V35): the production daemon composition materializes persisted
  active + policy-current Enrollments through the trusted Work Bundle boundary
  (`newProductRemoteToolExecutorsFromEnrollments` + composite executor);
  revoked/drift/unsupported/port-less records fail closed per-Enrollment and
  never expose a capability.
- Revoke the Enrollment mid-flight; confirm only the bound Agent is affected
  and the default production exposes no unbound remote capability.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G5-web-mcp-diagnostics.md`.

### G6 — Accounting and governance UI

Build 117 accounting/governance evidence is accepted for Phase 2D. The
four-Provider visual and real-account matrix below move with G3/G4 to a later
Phase.

- After G3/G4, confirm each Provider Account row shows attempts, exact error
  rate, rate-limited count, accounting coverage, tokens, currency-specific
  cost (`provider_reported` vs `rate_card_estimate`), policy revision,
  concurrency and budget ceiling, plus the explicit accounting-incomplete
  state where applicable.
- Evidence: `.loom-evidence/phase2d/acceptance/live/G6-accounting-ui.md`.

### G7 — Harness Gateway and Segment Session

Status: `ACCEPTED / INSTALLED PASS / BUILD 136`.

Build 136 completed the matrix below. Its App-exported diagnostic bundle and
owner-only stamped evidence pass the closed G7 verifier. See
`.loom-evidence/phase2d/P2D-HG1-build136-installed-g7.md`.

1. In one newly installed bundle, send two Codex turns in one Segment and
   Workspace. Both responses must complete under one Session and one immutable
   Session authority while their Attempt Capsule digests remain independent.
2. In the same visible Conversation, approve a switch to Loom Native/DeepSeek.
   The two source Codex responses must complete before the target Session opens.
   Two target responses must use a new Segment and one reused Session with the
   same frozen Workspace, Provider Account, credential revision, model and
   Route Transition review authority; their Attempt Capsule digests remain
   independent and the source Segment remains unchanged.
3. Start responses in two different Conversations so the second starts before
   the first terminates. Both must reach a terminal outcome.
4. Stop one active Codex response while a response in another Conversation is
   active. Only the selected Response becomes `cancelled`; the peer completes,
   and a later response completes through the cancelled Response's Session.
5. Export the privacy-safe diagnostic bundle from Loom and run:
   `scripts/export-phase2d-hg1-evidence.sh <Loom.app> <evidence-root> <app-diagnostics.json>`.
   Raw owner-only daemon operational JSONL is valid only for source-verifier
   development and cannot create installed G7 evidence.
   The App export retains at most 512 allowlisted events and the installed
   evidence reader revalidates the complete closed summary every time it checks
   the gate document.
   The verifier rejects missing lifecycle cells, same-Session authority drift,
   cross-instance composition, duplicate sequence values, incomplete Response
   lifecycles, content-bearing keys, old event schemas, stale bundle identity
   and generic G7 evidence stamps.

G7 evidence contains only bundle hashes, the non-secret Gateway Instance ID,
safe opaque IDs, Harness/backend versions, Provider Account ID, credential
revision, model/reasoning metadata, authority digests, event types/sequences and
the selected Incident ID. It never contains a credential reference, Prompt,
transcript, Workspace path or Provider body.

## Evidence rules

- Evidence exported by the installed-live harness is owner-only (`0600`) and
  privacy-safe: versions and hashes, process/socket health, non-secret
  Provider/Profile/Agent binding state, and allowlisted diagnostics only.
  Historical repository evidence follows repository access controls and never
  substitutes for current bundle-bound exported gate evidence.
- Never include API keys, Authorization headers, credential references,
  endpoint-fingerprint values, Prompts, conversation content, Provider bodies,
  MCP arguments/result bodies or raw console. A digest-only endpoint
  fingerprint may exist in the local authority UI and Journal binding, but its
  value is redacted from exported diagnostics and evidence; evidence may name
  the binding category without recording the digest.
- Each gate records Incident IDs; the same ID may correlate diagnostics with
  authoritative Journal facts but never authorizes execution.

## Harness

`scripts/phase2d-live-acceptance.sh` is a source library that:
- checks the installed App/daemon prerequisites without touching credentials;
- binds every accepted gate document to the installed bundle build plus App
  and daemon executable hashes, so stale evidence remains historical;
- exports a bounded privacy-safe evidence bundle for each gate;
- prints a structured PASS/OPEN summary.
`scripts/phase2d-hg1-trace.jq` validates the complete G7 event relationship and
`scripts/export-phase2d-hg1-evidence.sh` atomically binds its compact summary to
the installed bundle. The installed export path requires the App's complete,
closed diagnostic-bundle schema and rejects missing or nested unknown metadata;
generic G1-G6 export and raw daemon JSONL cannot create G7 evidence.
`scripts/test-phase2d-live-acceptance.sh` validates the library in source-only
mode (no side effects).

## Definition of done

Every active gate passes with evidence, the full source verification suite is
green, and `docs/CURRENT.md` records the exact accepted Phase scope. Deferred
rows are excluded only by the explicit 2026-08-24 scope decision and remain
preserved future-Phase candidates; they must never be relabeled as executed.
Default production remote-tool capability remains unpublished unless G5 is
explicitly approved and executed.

Build 127 closes the preceding G1-G6 active boundary, including G2 trust,
capacity and Route Transition. ADR-0022 subsequently added G7, so Phase 2D is
again `ACTIVE / PARTIAL` until one newer installed bundle passes that matrix.
No new external Provider request was made while adding the source verifier.
