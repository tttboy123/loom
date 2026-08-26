# Phase 2D Build 108 Conversation, Mission and Team Governance Evidence

Status: `CURRENT / VERIFIED SLICE`

Date: 2026-08-22

## Installed product

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.3`
- Build: `108`
- Candidate: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.3-build108-2026-08-22/Loom.app`
- Strict deep bundle signature verification passed.
- The bundled daemon launched as the App child with canonical non-secret state,
  isolation, Socket, Harness executable and managed-parent arguments.
- No manual `loomd` start, credential mutation or Keychain runtime helper was
  used for this slice.

Builds 104-107 were rejected intermediate candidates used to expose cold-start,
setup recovery, Capsule omission and Team history UX defects. Build 108 is the
current installed slice.

## Installed catalog and conversation

The privacy-safe installed setup gate returned:

- Providers: `25`
- Runtimes: `7`
- Conversation Profiles: `4`
- Credential Vault: unlocked

The Runtime inspector showed Claude Code, Codex, three Loom Native routes,
OpenCode and Pi online together. The normal composer showed the selected
DeepSeek account route and model without opening Runtime & Providers first.

`TestLiveOpenCodeConversationE2E` passed against the installed daemon and
returned the exact bounded acceptance marker. The focused
`TestLiveConversationContextDisclosureModesE2E` also passed across four
DeepSeek/OpenCode transitions in one visible Conversation. Every transition
created a distinct Segment and disclosure receipt; `summary_only` and the later
`start_clean` both reported explicit omissions. Evidence records only counts,
mode and pass/fail metadata, not conversation content or Provider bodies.

## Mission and Team governance

- Mission opens as a title list and drills into one governed workflow with its
  Agent Team pulse, plan, changes and Evidence inspector.
- Timeline pagination probes streams added by the rebuilt projection before it
  can publish `has_more=false`; newly related Run, WorkItem, Evidence or
  Approval facts cannot be silently stranded behind a completed page.
- Team pickers keep the newest same-source configuration for each visible name.
  Historical Team instances remain authoritative and accessible from Mission
  history; they are not deleted or rewritten.
- The installed left rail and Team inspector no longer repeat every same-name
  historical test execution as though it were a reusable configuration.
- All seven Runtimes and the Provider directory remain visible after OpenCode
  composition and after the Team list cleanup.

## Cold-start behavior

The retry path keeps a cold start in `loading` across recoverable connection
failures and never publishes a transient `offline/unavailable` state between
attempts. The first captured build 108 installed screen was `Local service
ready`; no manual retry or settings visit was required.

## Verification

- `go test ./...`: passed.
- `go vet ./...`: passed.
- `git diff --check`: passed before evidence updates.
- macOS XCTest: 309 executed, 308 passed, one intentional visual-export skip,
  zero failures.
- Strict Swift wire contracts: 16 passed.
- Reproducible native App build fixture: passed.
- Candidate and installed bundle strict signing: passed.
- Installed OpenCode conversation gate: passed.
- Installed Context disclosure mode gate: passed.

## Remaining Phase 2D gates

Phase 2D remains `ACTIVE / PARTIAL`. This slice does not close the complete
shipped-account import/revoke/restart/custom-endpoint matrix, trust-domain and
restart Capsule matrix, encrypted Capsule completion, one installed
four-Provider Team, remaining account-local failure cells, installed MCP
mid-flight revocation, explicit approved fallback, or complete accounting and
four-Provider visual matrix.
