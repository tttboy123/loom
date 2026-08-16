# P2D-W2B/W2D Multi-Agent Builder Authoring Evidence

Date: 2026-08-12

Status: `CURRENT / SOURCE AND UNINSTALLED CANDIDATE`

## Boundary

This increment closes the product-authoring gap between Loom's existing
multi-role Team domain and the macOS Team Builder. It does not close Phase 2D,
installed mixed-Team dispatch, or healthy-sibling execution when one required
Agent is initially blocked.

## Accepted behavior

- `BuilderEditCommand.role_agent_definition_id` scopes every sub-Agent edit to
  one stable AgentDefinition identity.
- Missing, stale, duplicate, or ambiguous targets fail before Draft revision or
  binding-digest mutation.
- Add Agent rejects duplicates and cardinality overflow. Remove Agent preserves
  at least one sub-Agent.
- Harness, Provider Account, Model, reasoning, timeout, budget, and fallback
  remain independent per Agent.
- Successive edits continue from the session-local immutable Runtime Profile.
- Fallback RouteSet state is per Agent rather than global to all sub-Agents.
- Coordinator, Bounded Worker, Reviewer, Researcher, and Verifier identities
  are available across compatible verified routes.
- Swift sends the exact non-secret target identity and never receives a
  credential reference or secret.

## Verification

- focused Go service RED/GREEN for add, remove, ambiguous-target rejection,
  exact account/revision selection, successive timeout edit, and peer isolation;
- focused product catalog tests for Codex/OpenAI, Claude Code/Anthropic,
  Loom/DeepSeek, Loom/Kimi, and Loom/MiniMax routes;
- strict real Go/Swift contract probes pass after source freeze;
- complete Swift suite passes 191 XCTest cases with one intentional
  visual-preview skip and eight Swift Testing contracts;
- affected Go race suites pass;
- `go vet ./...` passes;
- `git diff --check` and Info.plist validation pass;
- final serialized `go test -p 1 ./... -count=1 -timeout=15m` passes every
  package. One earlier concurrent run had a transient strict-loopback
  `unavailable`; the exact case passed five isolated reruns before the clean
  serialized repository result.

## Candidate

The source is frozen as unlaunched, uninstalled v0.5.2 build 51 at:

`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build51-candidate-2026-08-12/BUILD-MANIFEST.md`

Static release, arm64, deep-signature, owner-only/no-symlink, contract-string,
and ZIP byte-equivalence gates pass. Candidate App and daemon were not launched.
Installed Loom remains v0.5.2 build 39.

## Remaining execution gap

Mission preflight projects a blocked role independently, but Start currently
fails the whole command because the authoritative execution plan has no
initial-blocked node state. P2D-W2D must add that state and dependency behavior
before healthy sibling Agents may continue while only the affected Agent is
blocked.
