# Current State

Updated: 2026-07-24

## Product state

- `CURRENT`: product scope, Phase 1 contracts, architecture, trust boundaries,
  accepted ADRs, and the Codex bootstrap development workflow are documented.
- `CURRENT`: Slice 1 now has a Go 1.22 module, an explicit conversation versus
  Agent Mode Router, and an append-only SQLite Event Journal with versioned
  migration, transactional idempotent append, database-enforced Event
  immutability, and ordered replay. Focused, race, repository, and vet checks
  passed for these accepted WorkItems.
- `CURRENT`: the Evidence Artifact Store implements SHA-256 staging, private
  permissions, descriptor-relative I/O, atomic no-replace publication,
  digest-only identity, lifecycle leasing, canonical shard revalidation,
  non-blocking rejection of non-regular targets, and failure cleanup. Its
  human-authorized bounded repair passed Controller verification and a fresh
  strict Reviewer.
- `CURRENT`: the rebuildable in-memory projection reads only committed Journal
  rows, exposes deep-copy mode, WorkItem, and digest-only Evidence snapshots,
  and swaps state only after complete replay. Canonical replay, idempotency,
  failure preservation, cancellation, concurrency, and digest identity passed
  strict verification and a fresh Reviewer.
- `CURRENT`: the minimal CLI exposes structured `route` and read-only `status`
  commands. It uses the accepted Mode Router and projection, returns
  deterministic JSON, distinguishes invalid input from unavailable state,
  opens SQLite state read-only/query-only, and safely handles reserved
  characters in local state filenames.
- `PARTIAL`: the project-local `code_analyst` role follows the FastContext
  exploration contract, but it has only passed static configuration validation;
  no real child-Agent Canary has run. The official Microsoft FastContext runtime
  is not activated because its public repository is unavailable and the
  associated paper is withdrawn.
- `EXPERIMENTAL`: a pinned community FastContext SFT quantization may be
  evaluated through a loopback-only local runtime and Loom-owned read-only
  adapter. It is not an active dependency, execution authority, or Phase 1
  prerequisite.
- `TARGET`: the Loom daemon, CLI/TUI, SQLite state authority, Runtime discovery,
  Team Draft flow, Agent execution, task board, approval flow, and Evolution
  Sidecar.
- No running daemon, writable CLI command, real Agent runtime, or live Agent
  demo exists.
- Autonomous execution and production activation are not enabled.

Slice 1 is committed at `5861f82`. The current branch is
`codex/loom-platform-slice2`. The bounded `S2-W1` contract for pure
AgentDefinition and Runtime catalog domain contracts is frozen at
`.loom-evidence/phase1-slice2/S2-W1/contract.md` and passed a fresh independent
read-only Reviewer with no blocking findings. Its RED-first pure-domain
Candidate passed Controller focused, package, race, impact, repository-race,
vet, format, and scope checks, but fresh implementation Review 1 returned
`FAIL`: empty Definition ID could resolve across roles and the profile-switching
acceptance proof was hollow. Repair 1 added a regression RED, requires an
explicit stable ID, removes wildcard resolution, and replaces the hollow proof;
all strict Controller checks passed again, and fresh independent implementation
Review 2 returned `PASS` with no blocking findings. `S2-W1` is accepted.
Nothing is activated.

## Authoritative entry points

- Product behavior: [`../PRODUCT-PLAN.md`](../PRODUCT-PLAN.md)
- Phase 1 implementation contract and acceptance:
  [`../TECH-PLAN.md`](../TECH-PLAN.md)
- Architecture and flows: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Durable decisions: [`adr/README.md`](adr/README.md)
- Development workflow: [`DEVELOPMENT.md`](DEVELOPMENT.md)
- Code analysis integration:
  [`integrations/fastcontext.md`](integrations/fastcontext.md)

## Next development checkpoint

Create the authorized strictly scoped local S2-W1 atomic commit, then freeze one
bounded S2-W2 contract from the remaining `TECH-PLAN.md §14` Slice 2 boundary
and obtain a fresh contract Reviewer PASS before RED or implementation. Bridge,
real Runtime Adapter, AgentGrant, claim generation, and WorkItem dispatch
remain Slice 3 boundaries even when research drafts propose them earlier. Push,
merge, release, runtime activation, credential changes, and FastContext
installation remain unauthorized. The optional local FastContext Spike remains
separate and was not installed or activated.
