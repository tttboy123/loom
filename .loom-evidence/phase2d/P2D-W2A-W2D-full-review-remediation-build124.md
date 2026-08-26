# Phase 2D full review remediation and installed Build 124

Status: `SOURCE + INSTALLED CORE PATH VERIFIED / PHASE 2D PARTIAL`

Date: 2026-08-24

Incident: `ae03d34b-6b09-42c5-a2c0-4fa21edbe442`

## Bundle identity

- Product version: `0.5.3`
- Bundle build: `124`
- App executable SHA-256:
  `56ed2599546a0ce955c3c06339069a365af628ae1a0339917fd1c8eeae3dcee5`
- Bundled daemon SHA-256:
  `24d39b7cfd8f287892537e86c3092006e5b5d1cc323a2bcec9407413f8aaa182`
- Strict deep signature verification: passed
- Reproducible two-build fixture and installer replacement fixture: passed

## Review remediation

- Setup import candidates always carry the strict digest, endpoint fingerprint
  and review-policy fields; one invalid candidate is isolated instead of
  hiding the Provider and Runtime catalogs.
- Runtime projection decoding rejects invalid identifiers, display text,
  Harness adapters, statuses, capacity, models, capabilities and source probes.
- The App may construct its real IPC client before the run directory exists,
  but every exchange and health probe revalidates the private Socket's owner,
  mode, type and resolved parent.
- Socket connect/poll health work runs outside the main actor. Swift never
  removes the daemon Socket or lock; daemon lock authority owns cleanup.
- Native and account-scoped OpenCode profiles always route through the
  dedicated OpenCode responder. The exact Provider Account, credential
  revision and model are frozen without a first-available credential scan.
- OpenCode Conversation runs deny tools and sharing. Agent runs expose only
  explicitly scoped Loom context MCP capability.
- Conversation deletion serializes with sends, persists plaintext deletion,
  deletes encrypted documents before Capsules, rolls back on failure and clears
  stale Swift disclosure and Mission links.
- Mission new-attempt dispatch identities are salted. Mission pagination uses
  the Team-row cursor domain and advances through saved-only and filtered rows.
- Tool search soft fallback applies only to bounded tool failure or empty
  results; denial, invalid input, configuration, size and unknown failures
  remain fail closed.
- Legacy owner-owned, single-link private registry files at `0644` migrate to
  `0600`; unsafe ownership, mode, type, symlink and hard-link cases fail closed.
- Installed-live evidence is bound to the exact gate, bundle build and App plus
  daemon hashes. Export rejects symlink/public roots and existing destinations,
  commits through a private temporary file, and contains no user-local path.
- Historical G3 evidence is correctly classified as a two-Provider partial
  observation; it does not close the four-Provider completion gate.

## Source verification

- Swift: 329 passed, 2 visual-export tests skipped by design, 0 failed.
- Swift strict model tests: 20 passed.
- Go repository excluding the process-contract package: passed.
- Go `internal/localipc` process-contract suite: passed in 840.811 seconds.
- Focused Go race tests for chat deletion, Mission pagination and OpenCode
  routing: passed.
- `go vet ./...`, Go formatting, shell syntax and `git diff --check`: passed.
- Independent final Codex review: no unresolved P0-P3 findings.

## Installed core path

- A controlled legacy `chat-sessions.json` mode was changed to `0644`; the
  installed App migrated it to owner-only `0600` on launch.
- Product readiness recovered automatically after the daemon startup window;
  no manual daemon start or terminal action was required.
- Setup projected 25 Providers, 7 online Runtimes, 6 Conversation Profiles and
  5 privacy-safe CC Switch import candidates.
- DeepSeek and MiniMax Vault accounts remained verified. No Keychain credential
  helper process was present during the conversation calls.
- Exact live Conversation binding:
  `opencode + deepseek.primary + revision 2 + deepseek/deepseek-chat`.
- The Provider returned the exact 11-byte `Loom-NET-OK` reply and the strict
  Conversation tool stream contained no tool event.
- Terminating only the bundled daemon preserved the App process. The App
  launched a new child daemon with canonical state/isolation/Socket and managed
  parent arguments; Setup and the same real reply passed again.
- Registry, Vault key and Vault database remained owner-only `0600`; the
  conversation sentinel did not appear in the registry.

## Remaining Phase 2D gates

This evidence closes the Build 124 review-remediation and installed core
conversation checkpoint. It does not close a real approved custom-endpoint
import, the four-distinct-Provider Team, real account fault isolation, the
full-capacity Context Capsule matrix or approved cross-trust route transitions.
Phase 2D remains `ACTIVE / PARTIAL`.
