# P2D-W2A/W2B/W2C Product Parallel RouteSet Authoring v1

Status: `SOURCE VERIFIED / INSTALLED LIVE OPEN`

Date: 2026-08-13

## Scope

The Agent Team Builder now authors a versioned parallel RouteSet for one stable
Agent. The persisted authority is not a Team-level Provider choice. It contains:

- the Agent's existing primary Execution Profile;
- one or two additional exact execution routes;
- one explicit Synthesis route;
- exact Harness, Provider Account, model, auth mode, credential revision,
  endpoint fingerprint, reasoning effort, timeout, budget, and capabilities for
  every route.

Parallel RouteSet and fallback are deliberately mutually exclusive in this
slice. A fallback remains an approved transition after failure. A parallel
RouteSet means all sibling Attempts are intentionally evaluated and then
synthesized. Combining them without per-sibling fallback authority fails closed.

## Authority and restart

`TeamDefinitionSaved` stores `parallel_route_set` beside the owning role. State
normalization and Projection reject unknown versions, missing Synthesis,
duplicate/substituted profile identities, credential-revision drift, more than
two additional routes, or fallback/parallel ambiguity. Projection returns deep
copies of every nested Profile.

Builder preview, confirmation, saved-Team reopen, Mission preflight, and Mission
reconstruction preserve the same RouteSet. A logical Agent expands to
`route_sibling` nodes sharing one Agent identity and one versioned route group;
the original logical node becomes `aggregation` and depends on the exact sibling
set. Downstream Agents depend only on that aggregation node.

The four-Agent fixture proves six physical nodes: Codex/OpenAI, Claude/Anthropic,
MiniMax, Kimi/Kimi sibling, Kimi/DeepSeek sibling, and Kimi Synthesis. Restart
reconstruction reproduces the same plan digest, 12 bounded Attempt executions,
and six semantics records.

## Product UI

Each Agent editor has a compact Parallel routes menu. It displays the selected
Provider Account and model, supports add/remove, caps the set at three total
routes, and shows the explicit Synthesis model. Fallback controls are disabled
while parallel mode is active. Swift decoders use a strict closed shape and
reject cross-Provider account IDs, non-positive brokered credential revisions,
unknown fields, invalid topology, and fallback/parallel overlap.

## Isolation and privacy

Credential revocation remains route/account-local at binding resolution. In the
mixed fixture, revoking Kimi blocks the Kimi sibling while the DeepSeek sibling
remains ready. Synthesis remains held until its exact dependency contract is
satisfied; no Provider is silently selected as fallback.

Journal and Builder wire contain references and revisions only. No API key,
Authorization header, Prompt, Capsule body, Provider response, or hidden
reasoning is added by this slice.

## Verification

- `go test ./...`
- `go test -race ./internal/state ./internal/projection ./internal/app ./internal/teams ./internal/work`
- `swift test` in `apps/macos`: 202 XCTest cases passed, one intentional visual
  export skip; 10 Swift Testing contracts passed
- `swift test --filter LoomGraphiteViewTests/testWorkspaceShellConsumesAgentTeamBuilderSession`:
  the product Shell source contract retains Parallel route add/remove controls
  and the explicit Synthesis label
- `git diff --check`

## Remaining gate

This closes product Team Builder RouteSet authoring and source restart authority.
Ordinary Conversation parallel UX, real Harness Synthesis output, installed App
CV6, and the live mixed-Provider Team matrix remain open. No bundle was built,
launched, or installed and no real credential or Provider was accessed. Phase 2D
remains the sole `ACTIVE / PARTIAL` Goal.
