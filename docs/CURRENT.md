# Current State

Updated: 2026-07-25

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
It is locally committed at `954416a`. Nothing is activated.

`S2-W2` is frozen at
`.loom-evidence/phase1-slice2/S2-W2/contract.md` for deterministic,
side-effect-free coordination of injected Runtime discovery probes and immutable
Candidate snapshots. It does not add a concrete local CLI probe, daemon
scheduling, persistence, Runtime execution, or activation. Fresh independent
contract review returned `PASS` with no blocking findings. Mandatory RED failed
only on missing frozen discovery symbols. The minimal Candidate then passed
focused, package, 50-run focused race, repository, repository-race, vet, format,
diff, import-boundary, and scope checks after a pre-review regression RED closed
snapshot immutability and one-time probe-ID capture gaps. The Candidate is
accepted after a fresh independent implementation Reviewer returned `PASS`
with no blocking findings. A fresh amendment Reviewer also confirmed that the
active contract digest `7f8dab95...6d3c9` differs from the original reviewed
digest only by removal of one empty EOF line; all earlier reviews remain
applicable. It is not activated.

`S2-W3` is frozen at
`.loom-evidence/phase1-slice2/S2-W3/contract.md` for a bounded immutable
Team Draft catalog snapshot and pure invented-ID/ceiling validation. It composes
accepted AgentDefinition and Runtime discovery Candidates but does not create a
Team Draft revision, choose a default Main Agent, load or instantiate a Team,
or authorize execution. Fresh contract Review 1 returned `FAIL` because two
catalog count units were ambiguous. Contract Repair 1 now counts selected Agent
entries, online Runtime entries, Runtime-scoped model pairs, and normalized
workspace/permission sets explicitly. Fresh independent Repair Review 2
returned `PASS` with no blocking findings. Mandatory RED failed only on missing
frozen catalog symbols. The minimal Candidate passed focused, package, 50-run
focused race, repository, repository-race, vet, format, diff, import-boundary,
and scope checks. Fresh independent implementation review returned `PASS` with
no findings. S2-W3 is accepted and not activated.

S2-W3 is locally committed at `e196107`. `S2-W4` is frozen at
`.loom-evidence/phase1-slice2/S2-W4/contract.md` for immutable versioned Team
Draft revisions, exactly one unresolved question, catalog revalidation, stale
command rejection, and acceptance eligibility Candidate checks. It does not
accept a Draft, create a Team, choose a default Main Agent, persist state, call a
model, or execute anything. Fresh independent contract review returned `PASS`
with no blocking findings. Mandatory RED failed only on missing frozen Draft
symbols. The minimal Candidate passed focused, package, 50-run focused race,
repository, repository-race, vet, format, diff, import-boundary, and scope
checks. Fresh independent implementation review returned `PASS` with no
blocking findings. S2-W4 is accepted, locally committed at `0a98851`, and not
activated.

`S2-W12` is frozen at
`.loom-evidence/phase1-slice2/S2-W12/contract.md` for a pure direct saved-Team
instantiation-plan Candidate. It revalidates explicit S2-W9 `load_team` routing
and S2-W11 Runtime binding, plans the TeamInstance plus Main seed, and keeps
unassigned SubAgents dormant. It creates no Draft, resources, tasks, state, or
execution. Fresh independent contract review returned `PASS` with no blocking
findings. Mandatory RED failed only on missing frozen S2-W12 symbols. The
minimal Candidate now passes focused, package, 50-run focused race, repository,
repository-race, vet, format, diff, import-boundary, and scope checks. Fresh
independent implementation review returned `PASS` with no blocking findings.
S2-W12 is accepted, locally committed at `e8ddc82`, and not activated.

`S2-W13` is frozen at
`.loom-evidence/phase1-slice2/S2-W13/contract.md` for a pure saved-Team
TeamInstance/Main AgentInstance record-set Candidate. It adds caller-supplied
stable resource IDs and timestamp, freezes the exact Main AgentDefinition
version/scope and Runtime binding, and retains SubAgents as dormant records. It
does not allocate IDs, write SQLite/Events, create WorkItems/Runs/grants, start
processes, or enter Slice 3. Fresh independent contract review returned `PASS`
with no blocking findings. Mandatory RED failed only on missing frozen S2-W13
symbols. The minimal Candidate now passes focused, package, 50-run focused
race, repository, repository-race, vet, format, diff, import-boundary, and scope
checks. Fresh independent implementation review returned `PASS` with no
blocking findings. S2-W13 is accepted, ready for its local atomic commit, and
not activated.

`S2-W14` is frozen at
`.loom-evidence/phase1-slice2/S2-W14/contract.md` for bounded atomic Event
Journal batch append. It provides the generic all-or-none transaction primitive
a later saved-Team StateWriter needs, while defining no Team/Agent payload,
projection, resource, process, or execution behavior. It changes no schema or
accepted single-Event API. Fresh independent contract review returned `PASS`
with no blocking findings. Mandatory RED failed only on missing frozen S2-W14
symbols. The minimal implementation passes focused, package, 50-run focused
race, repository, repository-race, vet, format, diff, migration, and scope
checks. Fresh independent implementation review returned `PASS` with no
blocking findings. S2-W14 is accepted, locally committed at `f293a9f`, and not
activated.

`S2-W15` is frozen at
`.loom-evidence/phase1-slice2/S2-W15/contract.md` for the direct saved-Team
StateWriter. It revalidates the exact S2-W13 record set and atomically appends
only `TeamInstanceCreated` plus `AgentInstanceCreated` through S2-W14. It does
not change projection, schema, WorkItems, Runs, grants, processes, or execution.
Fresh independent contract review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W15 symbols. The minimal
implementation passes focused, package, 50-run focused race, repository,
repository-race, vet, format, diff, and scope checks. Review 1 required a
bounded Repair 1 for real same-ID Team shadow and complete digest-sensitivity
evidence; that matrix also exposed and fixed exact payload-byte digest binding.
Fresh independent Repair 1 review returned `PASS` with no blocking findings.
S2-W15 is accepted, locally committed at `560834a`, and not activated.

`S2-W16` is frozen at
`.loom-evidence/phase1-slice2/S2-W16/contract.md` for a rebuildable
TeamInstance/Main AgentInstance read model over the two S2-W15 facts. It changes
no schema, CLI, WorkItem, daemon, resource, process, or execution behavior.
Contract Review 1 required only an ownership wording repair to explicitly reopen
the accepted S1-W4 projection files. Fresh independent repaired-contract review
returned `PASS` with no blocking findings. Mandatory RED failed only on missing
frozen Snapshot fields. The implementation passes focused, package, 50-run
focused race, repository, repository-race, vet, format, diff, and import/scope
checks. Implementation Review 1 required a bounded payload-presence Repair 1;
fresh independent Repair 1 review returned `PASS` with no blocking findings.
S2-W16 is accepted, locally committed, and not activated.

`S2-W16` is locally committed at `42fc661`. `S2-W17` is frozen at
`.loom-evidence/phase1-slice2/S2-W17/contract.md` for a Pi-specific metadata
probe core over the accepted S2-W2 `RuntimeProbe` port. It freezes only bounded
version/model command requests, strict output parsing, immutable observation
mapping, and a narrow injected runner port. Current upstream Pi
`--list-models` startup may run migrations, so this boundary deliberately
contains no executable runner and cannot inspect or modify user Pi state.
Fresh independent contract review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W17 symbols. The minimal
Candidate passes focused, package, focused-race-50, repository,
repository-race, vet, format, diff, import-boundary, non-disclosure, and scope
checks. Fresh independent implementation review returned `PASS` with no
blocking findings. S2-W17 is accepted and ready for its local atomic commit.
No Pi process, daemon scheduling, credential/config/environment access,
Runtime execution, model call, or activation is authorized.

`S2-W17` is locally committed at `b73cf8b`. `S2-W18` is frozen at
`.loom-evidence/phase1-slice2/S2-W18/contract.md` for a concrete but isolated
local Pi metadata process runner. It binds a caller-selected absolute
executable and private root, accepts only the two exact S2-W17 requests, uses a
fixed environment allowlist and fresh per-call state, bounds output and
timeout, cleans process groups and temporary state, and never searches PATH for
Pi. Contract Review 1 required exact combined cleanup-error semantics, honest
original-process-group scope, and explicit `/usr/bin/env` interpreter-path
residual risk. Contract Repair 1 freezes `errors.Join` inspectability,
same-group-only cleanup proof, and search-directory identity revalidation
without overclaiming interpreter-byte binding. Fresh repaired-contract review
returned `PASS` with no blocking findings. Mandatory RED is the current gate.
Mandatory RED failed only on missing frozen symbols and the initial focused
Candidate became green, but the strict matrix correctly rejected concrete
`os/exec` inside the accepted pure-domain `internal/runtime` package. Contract
Amendment 2 moves only the concrete adapter into
`internal/runtime/piadapter`, preserving the parent ports and all behavior/
trust boundaries. Fresh Amendment 2 contract review returned `PASS` with no
blocking findings. The amended child-package Candidate now passes focused,
package, focused-race-20, repository, repository-race, vet, format, diff,
pure-parent import-boundary, non-disclosure, and scope checks. It uses only
deterministic temporary fixtures and is ready for fresh independent
implementation review. Fresh independent implementation review returned
`PASS` with no blocking findings. S2-W18 is accepted and ready for its local
atomic commit. Installed Pi, user Pi state, credentials, network, Agent
sessions, prompts, model calls, daemon scheduling, and Runtime activation
remain outside this boundary.

`S2-W18` is locally committed at `8c8fb9e`. `S2-W19` is frozen at
`.loom-evidence/phase1-slice2/S2-W19/contract.md` for a configured local Pi
probe factory. It inspects only the fixed upstream `pi` name directly beneath
caller-supplied trusted search directories, treats absence separately from an
invalid shadowing candidate, and wires the accepted S2-W18 runner to the
accepted S2-W17 probe without starting a process. It does not consult ambient
PATH/HOME/cwd, schedule discovery, write Events, start a daemon, or activate a
Runtime. Fresh independent contract review returned `PASS` with no blocking
findings. Mandatory RED failed only on missing frozen S2-W19 symbols. Minimal
product implementation now passes focused, package, focused-race-20,
repository, repository-race, vet, format, diff, no-PATH/no-process,
parent-purity, non-disclosure, and scope checks. Fresh independent
implementation review returned `PASS` with no blocking findings. One Reviewer
repository non-race run failed while run concurrently with repository-race;
isolated rerun and two ten-run reproductions passed, so it is preserved as
transient evidence rather than a confirmed defect. S2-W19 is accepted and
ready for its local atomic commit.

`S2-W5` is frozen at
`.loom-evidence/phase1-slice2/S2-W5/contract.md` for a pure immutable structured
Team Draft content Candidate. It closes the content dependency that must precede
acceptance: exact Agent/RuntimeProfile/RuntimeInstance/model coverage, a bounded
first-task DAG with acceptance criteria, customer-rule summary, approval
markers, explicit capability gaps, and gap-free readiness. It does not attach
content to a revision, accept a Draft, create Team/Agent/WorkItem records,
persist state, allocate capacity, call a model, or execute anything. Fresh
independent Contract Repair 1 review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W5 symbols. The minimal Candidate
passed the strict check matrix. Fresh implementation Review 1 found no product
correctness or security defect but returned `FAIL` for two missing test proofs
and the absent pre-commit deliverable. Repair 1 adds the one-SubAgent readiness
case, dependency-edge digest sensitivity, and pending-review deliverable without
changing production behavior. All strict checks pass again; fresh Repair 1
implementation review returned `PASS` with no findings. S2-W5 is accepted, not
activated, and locally committed at `567967c`.

`S2-W6` is frozen at
`.loom-evidence/phase1-slice2/S2-W6/contract.md` for a pure immutable composition
of S2-W4 Draft revisions with S2-W5 structured content and a binding digest.
Every command revalidates core/content/catalog coherence; capability gaps require
an unresolved question and block structured eligibility. It does not perform
terminal acceptance, create resources, persist state, or execute anything.
Fresh independent contract review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W6 symbols. The minimal Candidate
passed focused, package, 50-run focused race, repository, repository-race, vet,
format, diff, import-boundary, and scope checks. Fresh implementation Review 1
found no product correctness or security defect but returned `FAIL` because the
answer/edit typed failure and zero-output proof was incomplete. Repair 1 changed
tests only, closed the full failure surface, and passed the entire strict matrix
again. Fresh independent Repair 1 implementation review returned `PASS` with no
findings. S2-W6 is accepted, locally committed in this checkpoint, and not
activated.

`S2-W7` is frozen at
`.loom-evidence/phase1-slice2/S2-W7/contract.md` for an explicit typed terminal
Draft decision bound to one exact S2-W6 revision. Acceptance requires a user
`confirm_and_start` command; rejection requires a user decision; expiry requires
a system decision. The result remains a pure immutable domain record and does
not create a TeamInstance, AgentInstance, WorkItem, Event, process, or other
resource. Fresh independent contract review returned `PASS` with no blocking
findings. Mandatory RED failed only on missing frozen S2-W7 symbols. The minimal
Candidate passed the full strict verification matrix. Fresh independent
implementation review returned `PASS` with no findings. S2-W7 is accepted,
locally committed in this checkpoint, and not activated.

`S2-W8` is frozen at
`.loom-evidence/phase1-slice2/S2-W8/contract.md` for an immutable saved
TeamDefinition core: exactly one Main, at most two SubAgents, stable
AgentDefinition/RuntimeProfile references, project-over-reusable resolution,
and a pure complete-team load Candidate. It does not select a default Main,
route Mode input, create a Draft or TeamInstance, bind a live RuntimeInstance,
persist state, or execute anything. Fresh independent contract review is the
current gate and returned `PASS` with no blocking findings. Mandatory RED is
complete: it failed only on missing frozen S2-W8 symbols. The minimal Candidate
passed the full strict verification matrix. Fresh implementation Review 1
returned `FAIL`: duplicate non-winners could block resolution and a zero-value
copied accessor could panic. Repair 1 is bounded to those two product/test
defects. The repaired Candidate passes the full strict matrix; fresh independent
Repair 1 implementation review returned `PASS` with no findings. S2-W8 is
accepted, locally committed in this checkpoint, and not activated.

`S2-W9` is frozen at
`.loom-evidence/phase1-slice2/S2-W9/contract.md` for the pure explicit
Agent-mode Team Resolver. It consumes the accepted S1 Mode Router and returns a
saved-Team load, direct selected Main, or default-Main Draft-seed Candidate.
Ordinary conversation cannot enter resolution. It does not create a Draft,
TeamInstance, AgentInstance, WorkItem, process, or persistent state. Fresh
independent contract review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W9 symbols. The minimal Candidate
passed the full strict verification matrix. Fresh independent implementation
Review 1 found no product correctness or security defect but returned `FAIL`
because direct fail-closed proof was missing for selected-definition not-found,
invalid/duplicate catalogs, and wrong-scope defaults. Repair 1 is test/evidence
only and adds those zero-Candidate proofs without changing production behavior.
The complete Repair 1 strict matrix passes. Fresh independent Repair 1 review
returned `PASS` with no findings. S2-W9 is accepted and not activated.

`S2-W10` is frozen at
`.loom-evidence/phase1-slice2/S2-W10/contract.md` for a pure accepted-Draft
instantiation-plan Candidate. It will normalize the exact accepted Main,
SubAgent, Runtime/Profile selections, bounded task DAG, customer-rule metadata,
and ceilings without allocating or creating TeamInstance, AgentInstance,
WorkItem, Run, grant, process, or persistent state. Fresh independent contract
Review 1 returned `FAIL` because the Candidate confused the accepted requested
budget/concurrency with the catalog ceilings. Contract Repair 1 now preserves
both separately and binds both into validation and digest proof. Contract
Repair 1 review returned `FAIL` because it incorrectly required a strictly
positive requested budget, while the accepted catalog contract permits zero.
Contract Repair 2 restores non-negative budget, positive concurrency, and their
separate upper ceilings. Fresh independent Repair 2 contract review is the
current gate and returned `PASS` with no blocking findings. Mandatory RED failed
only on missing frozen S2-W10 symbols. The minimal Candidate passes the full
strict verification matrix. Fresh independent implementation review is the
current gate and returned `FAIL` with no product defect: direct test proof was
incomplete for full role-selection preservation/no-widening, complete digest
sensitivity, and reference-mismatch propagation. Implementation Repair 1 is
test/evidence only. The repaired Candidate passes the complete strict matrix;
fresh independent Repair 1 review returned `PASS` with no findings. S2-W10 is
accepted and not activated.

`S2-W11` is frozen at
`.loom-evidence/phase1-slice2/S2-W11/contract.md` for a pure saved-Team Runtime
binding Candidate. It re-resolves the exact saved Team and validates explicit
per-role online RuntimeInstance selections through the accepted Runtime
binding contract, including shared-capacity bounds. It does not reserve
capacity, create instances/tasks, persist state, or execute anything. Fresh
independent contract review returned `PASS` with no blocking findings.
Mandatory RED failed only on missing frozen S2-W11 symbols. The minimal
Candidate passes the full strict verification matrix. Fresh independent
implementation Review 1 returned `FAIL`: unselected discovery observations were
not fully revalidated and source/validation failure proof was incomplete.
Repair 1 is bounded to full observation revalidation plus those tests. The
repaired Candidate passes the complete strict matrix; fresh independent Repair
1 review returned `PASS` with no findings. S2-W11 is accepted and not
activated.

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

Create the authorized strictly scoped S2-W19 local atomic commit, then freeze
only the next bounded Slice 2 contract. Daemon scheduling, Event persistence/
status transitions, and projection remain separately frozen Slice 2
boundaries. Bridge, real Runtime Adapter execution, AgentGrant, claim
generation, and WorkItem dispatch remain Slice 3 boundaries even when research
drafts propose them earlier. Push, merge, release, runtime activation,
credential changes, and FastContext installation remain unauthorized. The
optional local FastContext Spike remains separate and was not installed or
activated.
