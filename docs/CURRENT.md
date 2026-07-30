# Current State

Updated: 2026-07-30

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
Mandatory RED failed only on missing frozen symbols. The minimal Candidate now
passes focused, package, focused-race-30, repository, repository-race, vet,
format, diff, Event/payload/order, real-SQLite atomicity/conflict,
non-disclosure, import, and scope checks. Fresh independent implementation
review is the current gate.
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
locally committed at `1b2c486`.

`S2-W19` is locally committed at `1b2c486`. `S2-W20` is frozen at
`.loom-evidence/phase1-slice2/S2-W20/contract.md` for the Runtime discovery
StateWriter. It converts one non-empty accepted S2-W2 snapshot into a bounded
atomic batch of canonical `RuntimeInstanceDiscovered` Events, requiring exact
per-instance Event metadata coverage and exact immutable appender results. It
does not run discovery, infer absence/offline transitions, update projection,
schedule a daemon, or activate a Runtime. Fresh independent contract review
returned `PASS` with no blocking findings. Mandatory RED failed only on the
missing frozen writer/input/Candidate/error symbols. The minimal implementation
passes the strict focused, package, focused-race-30, repository,
repository-race, vet, format, diff, import-boundary, and scope matrix. Fresh
independent Implementation Review 1 returned `FAIL` because same-instant
non-UTC `EmittedAt` values passed exact appender-result comparison. Repair 1
adds the missing RED proof and requires UTC locations on both compared Events;
the complete strict matrix passes again. Repair Review 2 confirmed the product
fix but returned `FAIL` because direct deadline-context proof was missing.
Test-only Repair 2 now proves `context.DeadlineExceeded`, zero Candidate, and
zero append calls; production is unchanged and the complete strict matrix
passes again. Fresh independent Repair 2 implementation review returned
`PASS` with no blocking findings. S2-W20 is accepted and locally committed at
`501ac33`.

`S2-W21` is frozen at
`.loom-evidence/phase1-slice2/S2-W21/contract.md` for the Runtime discovery
read-model projection. It extends the accepted rebuildable projection with one
deep-copied Runtime inventory map sourced only from committed canonical
`RuntimeInstanceDiscovered` Events. It permits valid higher-sequence
rediscovery while preserving stable device/adapter identity. It does not infer
status from absence, handle `RuntimeInstanceStatusChanged`, run discovery,
write state, schedule work, or activate a Runtime. Fresh independent contract
Review 1 returned `REPAIR` because it overconstrained caller-owned Event ID,
idempotency key, and correlation metadata. The repaired contract limits
projection authority to nonempty metadata plus accepted Journal/replay conflict
rules and explicitly accepts fresh alternate nonempty values. Fresh independent
repaired-contract review returned `PASS` with no blocking findings. Mandatory
RED preflight then found that the illustrative contract incorrectly required a
nonempty executable version while accepted `runtime.NewRuntimeInstance` allows
it to be empty. Contract Amendment 1 preserves the accepted Runtime authority
and fresh independent review returned `PASS` with no blocking findings.
Mandatory RED failed only on the missing frozen Runtime projection symbols.
The minimal implementation passes focused, package, impact,
focused-race-30, repository, repository-race, vet, format, diff, import,
non-disclosure, and scope checks. Fresh independent implementation review
returned `PASS` with no blocking findings. S2-W21 is accepted and locally
committed at `366bc48`.

`S2-W22` is frozen at
`.loom-evidence/phase1-slice2/S2-W22/contract.md` for a pure observed Runtime
status reconciliation Candidate. It compares a bounded S2-W21-derived baseline
with one accepted S2-W2 snapshot and emits Candidate transitions only for
matching stable identities whose observed accepted status changed. New or
absent IDs create no status transition because S2-W2 carries no negative probe
coverage proof. It does not append Events, update projection, run discovery,
infer offline status, schedule work, or activate a Runtime. Fresh independent
contract review returned `PASS` with no blocking findings. Mandatory RED is
failed only on the missing frozen S2-W22 symbols. The minimal pure Candidate
passes focused, package, impact, focused-race-50, repository, repository-race,
vet, format, diff, pure-domain import, no-write/no-probe/no-execution, and scope
checks. Fresh independent implementation Review 1 found no product defect and
returned `FAIL` for two test-proof gaps: baseline-set addition digest
sensitivity and independent same-status non-status inventory changes. Repair 1
changes tests only, preserves the product digest exactly, and passes the complete
strict matrix after a sequential retry of one unrelated concurrent Pi adapter
timeout. Fresh independent Repair 1 implementation review independently passed
the complete strict matrix, confirmed both gaps closed and the product unchanged,
and returned `PASS` with no blocking findings. S2-W22 is accepted pending its
local atomic commit. It is locally committed at `b0cf75f`.

`S2-W23` is frozen at
`.loom-evidence/phase1-slice2/S2-W23/contract.md` for the Runtime status
StateWriter. It revalidates one non-empty accepted S2-W22 Candidate, requires an
exact caller-metadata bijection, emits canonical
`RuntimeInstanceStatusChanged` Events, and accepts only one exact atomic
appender result. It does not run discovery/reconciliation, infer status from
absence, update projection, schedule work, start a daemon, or activate a
Runtime. Fresh independent Contract Review 1 returned `REPAIR`: allowing any
sequence greater than the previous discovery sequence could append a gap that
accepted projection replay cannot rebuild and could admit a repeated stale
status fact. Contract Repair 1 now requires the exact next sequence and adds an
occupied-next-sequence Journal proof. Fresh independent repaired-contract
review returned `PASS` with no blocking findings. Mandatory RED failed only on
the missing frozen S2-W23 symbols. The minimal writer passes focused, impact,
focused-race-30, repository, repository-race, vet, format, diff, import,
non-disclosure, exact-next, real-Journal conflict, and scope checks. Fresh
independent implementation review returned `PASS` with no blocking findings
after independently rerunning the strict matrix. S2-W23 is accepted pending its
local atomic commit. It is locally committed at `1ba3238`.

`S2-W24` is frozen at
`.loom-evidence/phase1-slice2/S2-W24/contract.md` for
`RuntimeInstanceStatusChanged` read-model projection. It validates exact
status-bearing provenance and updates only status plus separate status metadata
while preserving discovery/inventory facts. Later accepted rediscovery clears
status-transition metadata and remains the latest discovery fact. It does not
append Events, run discovery/reconciliation, build the next baseline, schedule
work, start a daemon, or activate a Runtime. Fresh independent contract review
returned `PASS` with no blocking findings. Mandatory RED failed only on the ten
missing frozen status read-model fields. The minimal projection passes focused,
package, impact, focused-race-30, repository, repository-race, vet, format,
diff, import, duplicate-payload, non-disclosure, atomic-rebuild, and scope
checks. Fresh independent implementation review returned `PASS` with no
blocking findings after independently rerunning the complete matrix. S2-W24 is
accepted and locally committed at `9838779`.

`S2-W25` is frozen at
`.loom-evidence/phase1-slice2/S2-W25/contract.md` for a pure bounded adapter
from copied S2-W24 Runtime projection records to the next S2-W22 status
baseline. It also explicitly amends the S2-W22 baseline provenance names from
discovery-specific to generic previous status-bearing facts and increments the
baseline digest version to `2`. Contract Review 1 returned `FAIL` because the
initial wording admitted a forged discovery/previous Event pair. Contract
Repair 1 requires Event ID equality if and only if sequence equality; fresh
independent Repair Review 2 returned `PASS`. Mandatory RED failed only on the
missing adapter/error symbols. The minimal Candidate passes focused,
package/impact, focused-race-30, repository, repository-race, vet, format,
diff, import, non-disclosure, mutation-isolation, forged-provenance, and scope
checks. Fresh independent Implementation Review 1 returned `FAIL`: cross-record
Event ID reuse could mint provenance that accepted Journal replay cannot
produce. Bounded Repair 1 is frozen to reject all same-role and cross-role reuse
among discovery, current status, and previous status Event IDs across Runtime
records. Fresh Repair 1 contract review returned `PASS` with no blocking
findings. Mandatory Repair RED failed on all nine same-role/cross-role reuse
combinations. The minimal Event-ID owner-set repair passes the complete strict
matrix again, preserving the valid same-record first-status alias. Fresh
independent Repair 1 implementation review returned `PASS` with no findings
after independently rerunning the complete matrix. S2-W25 is accepted and
locally committed at `38d914b`. It does not replay/query the Journal, append
Events, invoke discovery/reconciliation, schedule a scan, start a daemon, run
Pi, or activate a Runtime.

`S2-W26` is frozen at
`.loom-evidence/phase1-slice2/S2-W26/contract.md` for one bounded,
caller-triggered scan from configured Runtime probe factories into accepted
S2-W2 discovery. Fresh independent contract review returned `PASS` with no
blocking findings. Mandatory RED failed only on the missing frozen symbols. The
minimal Candidate prevalidates and copies zero through 32 factories, builds each
once in caller order, filters only canonical explicit absence, collects every
present probe before observation, and delegates once to accepted
`runtime.DiscoverRuntime`. Controller focused, package, Pi impact, focused-race
50, repository, repository-race, vet, format, diff, import, non-disclosure,
mutation-isolation, and scope checks pass. Fresh Implementation Review 1
returned `FAIL` only because exact-32 acceptance lacked direct test proof.
Test-only Repair 1 passed fresh contract review; Repair RED proved that test was
absent, the exact-32 success/once-per-factory proof is now added, and the full
strict matrix passes again with product unchanged. Fresh Repair 1 implementation
review independently passed the full matrix and returned `PASS` with no
findings. S2-W26 is accepted and locally committed at `494579d`. No scan is
scheduled or activated, and absence does not infer Runtime status.

`S2-W27` is frozen at
`.loom-evidence/phase1-slice2/S2-W27/contract.md` for one explicit application
coordination from accepted S2-W26 configured discovery to an injected accepted
S2-W20 commit boundary. Fresh independent contract review returned `PASS`.
Mandatory RED failed only on missing frozen app symbols. The minimal Candidate
prevalidates the committer before observation, returns empty/all-absent scans
without commit, commits a non-empty snapshot exactly once, and accepts only an
exact source/count/digest-bound S2-W20 Candidate. Controller focused, package,
impact, focused-race-50, repository, repository-race, vet, format, diff,
import, mutation, non-disclosure, and real SQLite exact-retry checks pass. Fresh
implementation review independently passed the complete matrix and returned
`PASS` with no findings. S2-W27 is accepted and locally committed at `7ec635b`.
It allocates no Event metadata, accesses no Journal/projection directly,
chooses no status policy, and adds no scheduling, daemon, Runtime activation, or
Slice 3 authority.

`S2-W28` is frozen at
`.loom-evidence/phase1-slice2/S2-W28/contract.md` for the concrete prepared
committer adapter required by S2-W27. Fresh independent contract review returned
`PASS`. Mandatory RED failed only on missing frozen symbols. The minimal
Candidate binds an accepted S2-W20 appender and caller-authoritative input
provider, validates typed-nil/context boundaries, calls the provider once, and
delegates exactly once to `state.CommitRuntimeDiscoverySnapshot`. Controller
focused, package, impact, focused-race-50, repository, repository-race, vet,
format, diff, import, mutation, non-disclosure, and real SQLite S2-W27 exact
retry checks pass. Implementation Review 1 returned `FAIL` because the exported
adapter zero value could panic. Repair 1 passed contract review; its RED
reproduced the panic, the minimal binding revalidation is GREEN, and the full
matrix passes on fresh rerun. Fresh Repair 1 implementation review independently
passed the complete matrix and returned `PASS` with no findings. S2-W28 is
accepted and locally committed at `9296832`. It allocates no Event metadata and
adds no concrete Journal/projection/status/scheduler/daemon/activation/Slice 3
authority.

`S2-W29` is frozen at
`.loom-evidence/phase1-slice2/S2-W29/contract.md` for one explicit application
coordination from an accepted S2-W25 Runtime status baseline and S2-W2 discovery
snapshot through accepted S2-W22 reconciliation to an injected accepted S2-W23
commit boundary. A valid zero-transition reconciliation skips commit. It does
not run discovery, build the projection baseline, prepare Event metadata, read
Journal/projection state, schedule a scan, start a daemon, or activate a
Runtime. Fresh independent contract review returned `PASS` with no findings.
Mandatory RED failed only on missing frozen symbols. The minimal Candidate
passes focused, package, impact, focused-race-50, repository, repository-race,
vet, format, diff, import, mutation, non-disclosure, and real temporary SQLite
exact-retry checks. Implementation Review 1 returned `FAIL`: the no-change path
skipped the post-reconciliation context gate, later result facts lacked isolated
proof, and the static AST proof was a no-op. Repair 1 is frozen to close exactly
those three gaps without changing the public API or authority boundary. Fresh
Repair 1 Contract Review 1 returned `FAIL` because the proposed result
interface would require a forbidden Journal import. Amendment 1 replaces only
that mechanism with a private primitive fact record populated from the concrete
S2-W23 Candidate, preserving the import and authority boundary. Fresh Amendment
1 review returned `PASS` with no findings. Mandatory Repair RED reproduced the
delayed no-change cancellation as an incorrect success and then failed on the
missing primitive fact validator. The minimal repair is GREEN: the context gate
now precedes the no-change return, all seven concrete result facts have isolated
single-field proof, and real AST/source boundary assertions replace the no-op.
The complete strict matrix passes again. Fresh Repair 1 implementation review
independently returned `PASS` with no findings. S2-W29 is accepted and locally
committed at `51d0489`.

`S2-W30` is frozen at
`.loom-evidence/phase1-slice2/S2-W30/contract.md` for the concrete prepared
committer adapter required by S2-W29. It binds an accepted S2-W23 appender and
caller-authoritative status commit input provider, validates zero/nil/
typed-nil/context boundaries, calls the provider once, and delegates exactly
once to `state.CommitRuntimeStatusTransitions`. It allocates no Event metadata
and adds no concrete Journal/projection/discovery/status-policy/scheduler/
daemon/activation/Slice 3 authority. Fresh independent contract review returned
`PASS` with no findings. Mandatory RED failed only on missing frozen symbols.
The minimal Candidate passes focused, package, impact, focused-race-50,
repository, repository-race, vet, format, diff, zero-value, import, mutation,
non-disclosure, and real temporary SQLite exact-retry checks. Fresh independent
implementation review returned `PASS` with no findings after independently
rerunning the complete matrix. S2-W30 is accepted and locally committed at
`47f225b`.

`S2-W31` is frozen at
`.loom-evidence/phase1-slice2/S2-W31/contract.md` for the copied projection
Snapshot adapter explicitly deferred by S2-W25. It builds the accepted status
baseline once, then delegates exact immutable inputs to S2-W29. It does not
query/rebuild Journal state in product, run or persist discovery, allocate Event
metadata, combine discovery/status write policy, infer absence, schedule, start
a daemon, or activate a Runtime. Fresh independent contract review returned
`PASS` with no findings. Mandatory RED failed only on missing frozen symbols.
The minimal Candidate passes focused, package, impact, focused-race-50,
repository, repository-race, vet, format, diff, import, mutation, no-policy,
and real temporary SQLite projection-to-status exact-retry checks. Fresh
Implementation Review 1 returned `FAIL` on three test-proof gaps:
status-bearing provenance, the complete invalid-projection matrix, and S2-W29
error/zero-output/Candidate mutation coverage. The product boundary itself had
no finding. Repair 1 is frozen as test-only; fresh independent Repair 1
contract review returned `PASS` with no blocking findings. Mandatory Repair RED
proved the missing status-bearing path against the old discovery-only fixture.
The repaired tests now cover first/consecutive status and rediscovery
provenance, the full invalid-projection classes, S2-W29 error and zero-output
propagation, and projection/reconciliation/commit accessor isolation. The
complete strict matrix passes again with the product byte-for-byte unchanged.
Fresh Repair 1 Implementation Review 1 returned `FAIL` only because the
invalid-projection table separated key and Runtime-core defects without a
projected stable-identity defect. The same frozen Repair 1 contract already
required that proof. A test-only correction now blanks projected `DeviceID`
while retaining the valid map key; its focused check and the complete strict
matrix pass with product unchanged. Fresh Repair 1 implementation re-review is
now `PASS` with no findings after independently rerunning the complete strict
matrix. S2-W31 is accepted and ready for its scoped local atomic commit; it is
locally committed at `b00f8d8`, and not activated.

`S2-W32` is frozen at
`.loom-evidence/phase1-slice2/S2-W32/contract.md` for a pure deterministic
Runtime observation write-plan Candidate under accepted ADR-0007. Any new or
changed non-status inventory selects one whole-snapshot `discovery` write;
only status changes with unchanged inventory select `status`; unchanged or
absent-only observations select `none`. Discovery has strict precedence in a
mixed observation, stable identity drift remains an error, and absence never
fabricates offline or deletion. This WorkItem does not invoke either writer,
prepare Event metadata, query Journal state, schedule, start a daemon, or
activate a Runtime. Fresh independent contract review returned `PASS` with no
findings. Mandatory RED failed only on missing frozen symbols. The minimal
Candidate passes focused, app, impact, focused-race-50, repository,
repository-race, vet, format, diff, ADR, inventory/status/precedence,
absence/identity, context, digest, mutation, and static boundary checks. Fresh
Implementation Review 1 returned `FAIL` only because the versioned Candidate
digest omitted exposed `Planned()`. All classification and boundary behavior
passed. Repair 1 is frozen to add exactly that existing fact and its
single-field sensitivity RED. Fresh Repair 1 contract review returned `PASS`
with no findings. Mandatory Repair RED proved that `planned=false` left the
digest unchanged. The minimal repair binds `planned` into the versioned payload,
and the focused plus complete strict matrix pass again without classification
or authority changes. Fresh Repair 1 implementation review returned `PASS`
with no findings after independently rerunning the complete matrix. S2-W32 is
accepted, locally committed at `26bf981`, and nothing is activated.

`S2-W33` is frozen at
`.loom-evidence/phase1-slice2/S2-W33/contract.md` for one caller-triggered
ADR-0007 write cycle. It computes S2-W32 once, returns without writing for
`none`, delegates only the discovery committer for `discovery`, or delegates
only S2-W31 for `status`; every non-selected output remains zero. It does not
run discovery, query/rebuild projection state, prepare metadata, retry,
schedule, start a daemon, infer absence, or activate a Runtime. Fresh contract
review returned `PASS` with no findings. Mandatory RED failed only on the
missing frozen coordinator/error symbols.
The minimal Candidate passes the complete strict matrix, including the real
SQLite mixed-discovery then status-only idempotent Event chain. Fresh
implementation review returned `PASS` with no findings after independently
rerunning the complete strict matrix. S2-W33 is accepted pending its one scoped
local atomic commit. It is locally committed at `affd2a6`, and nothing is
activated.

`S2-W34` is frozen at
`.loom-evidence/phase1-slice2/S2-W34/contract.md` for one caller-triggered
configured Runtime observation cycle. It executes accepted S2-W26 exactly once
and delegates its immutable snapshot plus a caller-supplied copied projection
to accepted S2-W33 exactly once. It does not use S2-W27's unconditional
discovery-commit path, query/rebuild projection, prepare metadata, retry,
schedule, load configuration, start a daemon, or activate a Runtime. Fresh
contract review returned `PASS` with no findings. Mandatory RED failed only on
the missing frozen coordinator/error symbols. The
minimal Candidate passes the complete strict matrix, including configured
mixed-discovery then status-only SQLite idempotency. Fresh implementation
Review 1 returned `FAIL` on one test-proof gap and found no product defect:
oversized/typed-nil factories, invalid probe result pairs, probe source error,
and invalid discovered observation were not directly propagated through the
S2-W34 boundary. Repair 1 is frozen as test-only; fresh repair-contract review
returned `PASS` with no findings. Mandatory Repair RED failed on all six
missing coverage markers. The test-only
repair directly covers the complete S2-W26 error matrix through S2-W34, the
complete strict matrix passes again, and product remains byte-for-byte
unchanged. Fresh Repair 1 implementation review returned `PASS` with no
findings after
independently rerunning the complete strict matrix. S2-W34 is accepted pending
its one scoped local atomic commit. It is locally committed at `a6eb816`, and
nothing is activated.

`S2-W35` is frozen at
`.loom-evidence/phase1-slice2/S2-W35/contract.md` for one caller-triggered
projected configured Runtime observation cycle. It reads one accepted copied
Snapshot from a bound `*projection.Projection`, then delegates exactly once to
S2-W34. It does not rebuild projection, access Journal/SQLite directly,
prepare metadata, retry, schedule, load config, start a daemon, or activate a
Runtime. Fresh contract review returned `PASS` with no findings. Mandatory RED
failed only on the missing frozen symbols. The minimal Candidate passes the
complete strict matrix, including real projected mixed-discovery then
status-only SQLite idempotency. Implementation Review 1 returned `FAIL` on one
test-proof gap and found no
product defect: the SQLite chain did not directly assert Event types/sequences
or final rebuilt Runtime facts. Repair 1 is frozen as test-only; fresh
repair-contract review returned `PASS` with no findings. Mandatory Repair RED
failed only on the three missing coverage markers. The test-only repair now
proves exact persisted Event types/sequences, exact retry Candidate identity,
and final rebuilt Runtime facts. Product remains byte-for-byte unchanged and
the complete strict matrix passes again. Fresh Repair 1 implementation review
returned `PASS` with no findings after independently rerunning the complete
strict matrix. S2-W35 is accepted pending its one scoped local atomic commit.
It is locally committed at `c8ecc2a`, and nothing is activated.

`S2-W36` is frozen at
`.loom-evidence/phase1-slice2/S2-W36/contract.md` for one immutable prepared
observer that shallow-copies configured probe-factory bindings and delegates
each explicit `RunOnce` exactly once to accepted S2-W35. It adds no interval,
ticker, goroutine, daemon/config entry, projection rebuild, retry, or Runtime
activation. Fresh contract review returned `PASS` with no findings. Mandatory
RED failed only on the missing frozen symbols. The minimal Candidate
shallow-copies factory bindings, performs no construction-time work, and
delegates every explicit `RunOnce` exactly once to S2-W35. The complete strict
matrix passes, including a real SQLite discovery-priority then status-only
chain with exact Events, idempotent explicit retry, and final rebuilt Runtime
facts. Fresh Implementation Review 1 returned `FAIL` on one test-proof gap and
found no product defect: the prepared-observer boundary did not directly cover
the complete context/downstream error surface. Repair 1 is frozen as test-only
with the product hash locked unchanged. Fresh Repair 1 contract review returned
`PASS` with no findings. Mandatory Repair RED failed only on all twelve missing
canonical markers. The test-only repair now directly covers context,
identity-drift, selected missing/typed-nil writer, writer error/result mismatch,
and delayed-cancellation failures with five-zero/no-retry proof. The complete
strict matrix passes again and product remains unchanged. Fresh Repair 1
implementation Review 2 returned `FAIL` on one remaining proof omission and
again found no product defect: the configured-discovery failure case used
anonymous committers and did not assert their zero call counts. This is the
second same-class failure. Fresh read-only problem analysis confirmed a
false-green contract-to-test traceability gap and no product defect. Repair 2
is frozen as test-only to prove the configured-discovery
factory/discovery/status call tuple `1/0/0`. Repair 2 contract Review 1
returned `FAIL` because two proposed global markers already existed in
unrelated tests. No implementation began. Amendment 1 makes all four mandatory
RED markers unique and case-local; fresh amendment review returned `PASS` with
no findings. Mandatory Repair 2 RED failed only on all four missing case-local
markers. The test-only repair now proves the configured-discovery sentinel,
five zero outputs, and exact factory/discovery/status call tuple `1/0/0`. The
complete strict matrix passes again, product remains byte-for-byte unchanged,
and fresh Repair 2 implementation review returned `PASS` with no findings
after independently rerunning the complete strict matrix. S2-W36 is accepted
pending its one scoped local atomic commit. It is locally committed at
`38891c3`, and nothing is activated.

`S2-W37` is frozen at
`.loom-evidence/phase1-slice2/S2-W37/contract.md` for one injected
trigger→prepared-observer coordination. It awaits one trigger and calls S2-W36
once, without importing time or adding a timer, ticker, loop, goroutine,
configuration, daemon lifecycle, retry, projection rebuild, or Runtime
activation. Fresh contract Review 1 returned `FAIL` because typed-nil trigger
validation appeared incompatible with the new-file import whitelist. No
implementation began. Amendment 1 explicitly requires the already accepted
same-package `nilAppInterface` helper and no new import or authority; fresh
amendment review returned `PASS` with no findings. Mandatory RED failed only
on the missing frozen trigger interface, error, and function symbols. The
minimal Candidate awaits the injected trigger exactly once, then delegates
exactly once to S2-W36; every trigger, context, or downstream error returns
five zero outputs without fallback or retry. The complete strict Controller
matrix passes, including the direct downstream error surface, exact call
counts, static no-scheduling proof, and a real SQLite
discovery/discovery/status chain with exact Event sequences and final rebuilt
Runtime facts. Fresh independent Implementation Review 1 returned `PASS` with no findings
after independently rerunning the complete strict matrix. S2-W37 is accepted
and its fresh pre-commit strict matrix passes. It is pending only its exact
staged-scope audit and one scoped local atomic commit. It is locally committed
at `9175f94`; post-commit focused and repository checks pass, and nothing is
activated.

`S2-W38` is frozen at
`.loom-evidence/phase1-slice2/S2-W38/contract.md` for the context-bounded
recurrence layer above S2-W37. Each successful injected trigger produces
exactly one accepted observation and the first trigger, context, or downstream
error terminates without retry. It adds no clock, timer, ticker, channel,
signal, configuration, daemon entry, goroutine, concrete scheduling policy, or
Runtime activation. Fresh independent Contract Review 1 returned `PASS` with
no findings. Before mandatory RED, Controller code-truth review found that the
accepted prepared observer does not rebuild its bound in-memory projection
after writes, so a recurrence-only loop would compare later observations
against stale Runtime facts and could not prove the required
discovery/discovery/status chain. No product or test implementation began.
Contract Repair 1 postpones recurrence and instead freezes one
projection-synchronized triggered observation: an unexported trigger decorator
refreshes committed projection facts before S2-W37, and a second refresh makes
a successful write visible before return. Post-commit refresh errors preserve
the exact successful outputs plus error rather than hiding committed authority.
Fresh Repair 1 contract review is the current gate.
Fresh Repair 1 Contract Review 1 returned `FAIL`: a separately passed
projection could differ from the observer's actual bound read model, and the
post-commit partial-success language exceeded what S2-W37 can observe. No
implementation began. Because the projection mismatch repeats the same stale
read-model failure class, fresh read-only problem analysis is mandatory before
Contract Repair 2.
Fresh Problem Analysis 1 confirmed that the defect is the application
composition/read-model lifecycle boundary, not Journal, replay, planning,
trigger, or writer behavior. Contract Repair 2 removes the separate projection
parameter, captures only `observer.readModel`, rejects a zero-value observer,
uses one await→context→pre-refresh decorator around exact S2-W37, and
post-refreshes only after S2-W37 success. Only post-success rebuild failure
returns the exact successful path-specific tuple plus error; every S2-W37 error
remains five-zero and makes no claim that no Event committed. No context check
follows a successful post-refresh. Fresh Repair 2 contract review is the
current gate. Fresh Repair 2 Contract Review 1 returned `PASS` with no
findings. The first RED attempt was discarded after a test-fixture field error
was found behind Go's compile-error limit; product was removed, the fixture was
corrected, and a clean test-only mandatory RED failed only on the repaired
frozen error/function symbols. The minimal Candidate captures the observer's
exact private read model, composes one await→context→pre-refresh decorator with
exact S2-W37, and post-refreshes the same projection only after success. The
complete strict Controller matrix passes, including the full downstream error
surface, transparent post-success refresh failure, static authority proof, and
a same-observer SQLite discovery sequence 1→discovery 2→status 3 chain with no
test-side inter-call rebuild. Fresh independent implementation review is the
current gate. Fresh Implementation Review 1 returned `FAIL` on one mandatory
test-proof gap and found no product defect: none/discovery/status success paths
were not collected in one direct runtime order/count group. Implementation
Repair 1 is frozen as test-only with the product hash locked; fresh Repair 1
contract review returned `PASS` with no findings. Mandatory Repair RED is the
current gate. Mandatory Repair RED failed only on four missing unique
case-local success markers. The repaired tests now directly prove exact
none/discovery/status pre-refresh→observer→write→post-refresh behavior and
counts using real Journal fixtures and prepared committers. The complete
strict matrix passes again and product remains byte-for-byte unchanged. Fresh
Repair 1 Implementation Review 2 returned `PASS` with no findings after
independently rerunning the complete strict matrix and product lock. S2-W38 is
accepted and its fresh pre-commit strict matrix/product lock passes. It is
locally committed at `39a9e0a`; post-commit focused and repository checks pass.
Nothing is activated.

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

The reviewed Slice 2 Exit Contract is frozen at
`.loom-evidence/phase1-slice2/EXIT-CONTRACT.md` with SHA-256
`c7639aef...ac7614`. Fresh independent Contract Review 1 returned `PASS` with
no blocking findings. The contract classifies accepted S2-W1 through S2-W38 by
`DONE`, `PARTIAL`, and `MISSING` and permits only one additional Slice 2
product WorkItem:
`Local Runtime Observation Daemon Integration`. That merged boundary must
close concrete clock/trigger, validated configuration, daemon lifecycle,
serialized projection-aware recurrence, cancel/restart/recovery, the
non-activation proof, and a controlled foreground live canary. It must not be
split into thin scheduler, constructor, entry, or forwarding WorkItems.

After that one Candidate passes its complete matrix and implementation review,
run a fresh whole-Slice Reviewer. Slice 2 may exit only on `PASS`; otherwise
stop at `HUMAN_REQUIRED` rather than adding another thin WorkItem. Bridge, real
Runtime Adapter execution, AgentGrant, claim generation, and WorkItem dispatch
remain Slice 3 boundaries. Resident service activation, push, merge, release,
credential changes, and FastContext installation remain unauthorized.

The sole merged `S2-EXIT-1` contract is frozen at
`.loom-evidence/phase1-slice2/S2-EXIT-1/contract.md`; fresh independent
Contract Review 1 returned `PASS`. Mandatory RED failed only on the frozen
missing daemon/entry symbols. The Candidate now integrates explicit
configuration, private SQLite state and process lock, production clock and
Event metadata, serial S2-W38 recurrence, cancellation/restart/recovery, and a
real foreground `loomd`. Focused, impact, 30-run focused race, repository,
repository-race, vet, format, and diff checks pass. The controlled live canary
also passes discovery, no-write restart, rediscovery, failure recovery, signal
cancellation, permission, residue, and no-orphan checks. Implementation Review
1 found no product, security, or Slice 3 leakage defect but returned `FAIL`
because direct metadata-failure/zero-append proof and the complete
configuration rejection matrix were missing. Repair 1 remains in the same
lineage; its bounded contract and fresh Contract Review passed, mandatory RED
was captured, and the repaired Candidate now passes the direct proof, complete
matrix, focused race, full repository/race, vet, format, diff, and compiled
canary checks. Fresh independent Repair 1 Implementation Review returned
`PASS` with no findings. S2-EXIT-1 passed its fresh pre-commit matrix and exact
staged-scope audit, then was locally committed at `46eefaf`. Post-commit
focused, command, and repository tests pass; nothing was activated. Slice 2
Whole-Slice Review 1 returned `FAIL` only because the committed
`CURRENT`/`PROGRESS` authority in `46eefaf` still described the pre-commit
state; no product, security, concurrency, or Slice 3 leakage blocker was found,
and the Reviewer's full test/race/vet/build/fresh-canary checks passed. The
bounded status-only Repair 1 contract and its fresh Contract Review pass. The
user explicitly authorized one separate status-only governance commit; this
checkpoint records the reconciliation without changing product or tests.
Fresh Whole-Slice Review 2 returned `PASS` after independently rerunning the
repository/race/vet/build matrix and a bounded discovery/restart/rediscovery
canary. Slice 2 is accepted at reviewed HEAD `7b1726e`; nothing was activated.
Slice 3 has entered governance setup only. Its exit contract fixes at most five
vertical WorkItems before any S3 product contract is frozen. Fresh independent
Exit Contract Review returned `PASS` with no findings. `S3-W1` is now frozen as
the complete pure Bridge v1 wire and bound-stream trust boundary. Contract
Review 1 returned `FAIL` only because the prose said eleven fields plus payload
instead of exactly twelve top-level fields. Amendment 1 corrects that count and
adds exact-12 proof without changing scope; fresh Amendment Review 1 returned
`PASS` with no findings. Both mandatory RED stages were captured before
product code. The Bridge frame and immutable bound-stream Candidate now passes
focused, 100-run focused race, repository, repository-race, vet, fuzz, format,
diff, and export-surface checks. Fresh independent Implementation Review 1
returned `PASS` with no findings, and the final pre-commit matrix passed. S3-W1
is accepted; its atomic local commit records the Bridge boundary and the
reviewed Slice 2-to-3 transition evidence. The next gate is an S3-W2 contract;
no S3-W2 product work is authorized before fresh Contract Review `PASS`.
S3-W2 is now frozen at baseline `c21a8f1` as one Run/WorkItem/Runtime
multi-stream transaction authority: Journal stream-head CAS, create/assign,
claim/capacity, lease/reclaim, start/terminal, and rebuildable projection stay
in one Candidate. Contract Review 1 returned `FAIL` on accepted Runtime stream
spelling, executor-to-`done` leakage, reclaim of running Runs, and lexical
cross-stream replay. Amendment 1 replaces those four points without changing
scope; fresh Amendment Review 1 returned `PASS` with no findings. The current
gate is the complete mandatory RED across Journal CAS, Run authority, and
dependency-aware projection. The mandatory RED passed and minimal Journal CAS
GREEN began. Implementation feasibility then exposed that restart-safe
`Authority.Snapshot()` cannot enumerate facts through the frozen Store API.
Amendment 2 adds only a deterministic mutation-isolated `Store.ReadAll`; fresh
Contract Review passed and its focused RED/GREEN closed. Projection analysis
then proved that Amendment 1's same-stream capacity facts would break the
accepted adjacent Runtime-status sequence. Amendment 3 moves capacity facts to
`runtime_capacity:<id>` while retaining exact CAS of both status and capacity
heads. Contract Review found that replay could not audit that live ordering
without persisted status-head evidence. Amendment 4 adds only exact status
stream/sequence/Event references to S3-W2 Run/capacity payloads. Fresh Contract
Review returned `PASS` with no findings. The current gate is test-only repair
and a focused RED on the missing separate-capacity-stream and persisted
status-head-reference behavior before further product implementation. That RED
failed only on the frozen missing behavior. The complete Candidate now keeps
status and capacity streams separate, binds every relevant mutation to an
exact persisted status-head reference, audits historical status/capacity
facts during dependency-aware replay, and stops successful execution at
`ready_for_review`. Focused, 30-run focused race, repository,
repository-race, vet, fuzz, format, diff, marker, export, and scope checks
pass. Fresh independent Implementation Review 1 found no S3-W2 product defect
but returned `FAIL` because unchanged Pi fixtures failed while Reviewer and
Controller full-suite processes overlapped. No waiver was taken. In an
evidence-only repair, both exact full-suite commands then passed three
consecutive isolated attempts with no product change, package exclusion, or
`-p 1`. Fresh independent Implementation Review 2 returned `PASS` with no
findings. The fresh final pre-commit matrix and exact Candidate scope checks
pass. S3-W2 is accepted and locally committed at `5517a06`; post-commit focused
and repository tests pass. S3-W3 is frozen as one complete AgentGrant local
security authority covering random one-time token issuance, hash-only
persistence, exact Run/generation/operation binding, linearized authorization,
rotation/revocation, and rebuildable projection. Fresh independent S3-W3
Contract Review returned `PASS` with no findings. A pre-RED feasibility audit
then corrected one impossible concurrency proof: Run-versus-Grant races must
accept either Run-first/Grant-conflict or Grant-first/then-Run success, while
same-Grant-stream contenders still have one winner. Amendment 1 also makes
per-Run RequestID uniqueness explicit without changing API, owned scope, or
capability. Fresh independent Amendment 1 Contract Review returned `PASS` with
no findings. Complete mandatory RED failed only on missing frozen S3-W3
symbols/behavior. The complete Candidate now issues one-time random tokens,
persists only their SHA-256 hashes, enforces exact Run/generation/operation
bindings, linearizes authorization, rotation, and revocation facts, and
strictly rebuilds AgentGrant projection state. A targeted security regression
found and closed nested `%#v` token disclosure before review. Focused,
30-run focused race, repository, repository-race, vet, fuzz, format, diff,
marker, export, coverage, scope, and token-leak checks pass. The current gate
was fresh independent S3-W3 Implementation Review. Review 1 returned `FAIL`
with four in-scope product findings: opaque-ID compatibility, operational
historical Run-reference validation, Journal conflict normalization, and
duplicate JSON-key rejection. Repair 1 freezes exactly those closures without
API, Event schema, owned-scope, or capability expansion. Its current gate is
fresh independent Repair Contract Review before repair RED. That review
returned `PASS` with no findings; the current gate is complete test-only Repair
RED before any repair implementation. Repair RED failed exactly on all four
frozen gaps with all four markers exactly once; the current gate is the minimal
in-scope Repair 1 implementation. No capability is activated.
Repair 1 is now complete: S3-W2-compatible opaque IDs, exact operational
historical Run-reference validation, four-way Journal conflict normalization,
and recursive duplicate JSON-key rejection all pass their mandatory RED/GREEN
proof. Focused, 30-run focused race, repository, repository-race, vet, fuzz,
format, diff, marker, export, coverage, scope, and token-surface checks pass.
The current gate is fresh independent S3-W3 Implementation Review 2; no
capability is activated.
Implementation Review 2 returned `PASS` with no findings after independently
rerunning focused, repository, race, vet, fuzz, format, diff, and marker
checks. The current gate is the fresh final pre-commit matrix and exact
Candidate staging; no capability is activated.
The fresh final pre-commit matrix also passed, including the required 30-run
focused race and whole-repository race. S3-W3 is accepted for its exact atomic
local commit. The next gate is S3-W4 managed workspace/Runtime adapter/
supervisor contract governance; no S3-W4 product work is authorized before
fresh Contract Review `PASS`, and no capability is activated.
S3-W4 is now frozen at baseline `47b4b50` as the single permitted managed
filesystem/process boundary: private workspace and source digests, one
configured Pi stdio adapter, bounded Bridge session, per-frame Grant
authorization, process-group cancel/timeout cleanup, workspace change capture,
and supervisor-generated terminal/revocation stay in one Candidate. The
current gate is fresh independent S3-W4 Contract Review; no S3-W4 product work
or Runtime activation is authorized.
Contract Review 1 returned `FAIL` on two contract defects only: missing
fail-closed hardlink/path-race proof and contradictory child-reported failure
semantics. Amendment 1 adds Unix single-link/no-follow/identity proof with
non-Unix fail-closed behavior and clarifies that child `succeeded` maps only to
`ready_for_review` while child `failed` is also a legal Run terminal. API,
Event schema, owned scope, and capability are unchanged. The current gate is
fresh independent Amendment 1 Contract Review. That review returned `PASS`
with no findings. Complete mandatory S3-W4 RED then failed only on the frozen
missing Workspace, Supervisor, and Pi execution-adapter symbols, with all eight
markers exactly once and no product file present. The current gate is the
minimal complete S3-W4 implementation inside the reviewed owned scope. A
pre-implementation-review security self-audit then proved that lexical
`WalkDir` plus final-component no-follow cannot close Amendment 1's
intermediate-directory replacement race. Amendment 2 adds only Unix rooted
`openat/fstatat` traversal and one non-Unix fail-closed stub file. The current
gate was fresh independent Amendment 2 Contract Review before those two files
were added. That review returned `PASS` with no findings. The current gate is
the focused Amendment 2 behavioral RED for controlled intermediate-directory
replacement. That RED exited `1` because the old lexical traversal accepted
the replaced intermediate directory, proving the frozen gap. The current gate
is the minimal descriptor-rooted Amendment 2 implementation; no Runtime
capability is activated.
The Amendment 2 Candidate and two bounded verification repairs are now GREEN.
Descriptor-rooted traversal rejects both controlled final-component and
intermediate-directory replacements. A repeated-race readiness gap was fixed
test-only by cancelling after explicit grandchild PID readiness. A second
repeated-race failure exposed and fixed a product-level Go `Cmd.Wait` versus
managed stdout/stderr pipe race using caller-owned output pipes; independent
Problem Analysis confirmed the diagnosis and repair. The exact dual-package
30-run race passed (Supervisor 123.221s, Pi adapter 555.437s), as did focused
coverage, repository, repository-race, vet, fuzz, format, marker, static
boundary, Linux, Windows, diff, and scope checks. The current gate is fresh
independent S3-W4 Implementation Review; no Runtime capability is activated.
Implementation Review 1 returned `FAIL` with two bounded findings: raw Grant
substrings were not rejected in source/change path text, and the fuzz harness
treated Darwin filename-creation rejection as a product failure. Repair 1 stays
inside existing owned files, adds exact source/workspace path RED, rejects
token substrings in manifest/change paths, and returns from fuzz cases rejected
during fixture setup. The current gate is Repair 1 focused GREEN and the full
verification matrix; no Runtime capability is activated.
Repair 1 focused GREEN and 10s fuzz passed. Its exact 30-run impact race passed
Supervisor but exposed a 3s fixed deadline in an accepted S2-W18 metadata
runner fixture under repeated package race load; the same impacted subtest
passed focused `-race -count=100` in 47.699s. Amendment 3 proposes only one
test-file ownership exception to raise the non-timeout fixture bound to at most
10s while preserving the explicit 100ms timeout proof. The current gate is
fresh independent Amendment 3 Contract Review before that file changes; no
Runtime capability is activated.
Amendment 3 Contract Review returned `PASS` with no findings. The only accepted
prerequisite test edit raises the shared non-timeout fixture bound from 3s to
10s; the dedicated 100ms timeout proof and all product behavior remain
unchanged. The current gate is focused Amendment 3 proof followed by the exact
combined race matrix; no Runtime capability is activated.
Amendment 3 focused proof passed: impacted metadata subtest
`-race -count=100` in 50.131s and timeout/cancellation/process-group
`-race -count=30` in 49.754s. The exact combined race then passed Supervisor
101.281s and Pi 552.160s. Repair 1 full repository, repository-race, vet, 10s
fuzz, format, marker, static boundary, Linux/Windows compile, diff, dependency,
and scope checks also pass. The current gate is fresh independent S3-W4
Implementation Repair 1 Review; no Runtime capability is activated.
Implementation Repair 1 Review 1 returned `FAIL` on one test-readiness race:
the helper's `os.WriteFile(grandchild.pid)` could expose an empty file, while
the reader treated any successful read as ready. This is the second same-type
readiness failure, so Repair 2 is test-only and gated on fresh read-only Problem
Analysis before atomic PID publication plus valid-positive-PID polling. No
product or Runtime capability is activated.
Fresh independent S3-W4 Implementation Repair 2 Review returned `PASS` with no
findings after independently rerunning cancellation race, token paths, fuzz,
related packages, repository, vet, format, diff, dependency, marker, and static
boundary checks. The current gate is the fresh final pre-commit matrix and
exact S3-W4 Candidate staging; no product or Runtime capability is activated.
The fresh final pre-commit matrix passed, including the exact 30-run race
(Supervisor 103.432s, Pi 541.156s), repository, repository-race, vet, fuzz,
format, marker, static boundary, Linux/Windows compile, diff, dependency, and
scope checks. S3-W4 is accepted for exact local atomic commit; no Runtime
capability is activated.
Repair 2 focused `-race -count=100` passed in 130.480s. The exact combined
30-run race then passed Supervisor 99.335s and Pi 546.046s. Repository,
repository-race, vet, 10s fuzz, format, marker, static boundary, Linux/Windows
compile, diff, dependency, and scope checks all pass on the latest Candidate.
The current gate is fresh independent S3-W4 Implementation Repair 2 Review; no
product or Runtime capability is activated.
Fresh Problem Analysis confirmed the test-only publication/consumption race.
Repair 2 now writes and closes a same-directory temporary PID file before
atomic rename, accepts readiness only for a parsed positive PID, and preserves
bounded cleanup on every failure. The current gate is Repair 2 focused repeated
race proof; no product or Runtime capability is activated.

`CURRENT`: S3-W5 now closes the single vertical Team DAG execution integration
boundary. Journal-authoritative related-stream reads and CAS remain the only
write authority; `GlobalReadView` is immutable and rebuildable; one Main plus
at most two SubAgents use deterministic ready-set planning, capacity fencing,
independent Run/Grant/Evidence attempt lineage, authorized tentative Frame
capture, and exact terminal aggregation. Durable private attempt capture and
receipts recover terminal/artifact/metadata gaps without duplicate execution.
Expired never-started claims use one generation-fenced rebound; indeterminate
running state returns typed human-required instead of replaying. Controlled
SQLite/Supervisor canaries, full repository tests, uncached repository race,
vet, format, scope, and fresh independent Repair Review 2 all pass. No daemon,
installed Runtime, Provider fallback, Web/TUI, or autonomous execution is
activated. S3-W5 is locally committed at `f7534b4`.

`CURRENT`: Phase 1 Slice 3 is closed after a fresh independent whole-Slice
Review returned `PASS` with no blocking findings. The committed chain contains
exactly S3-W1 through S3-W5 and every frozen exit capability is `DONE`; no
S3-W6 exists. The next permitted step is Slice 4 contract/plan governance.
This status does not authorize an installed Runtime, Provider/model traffic,
credentials, daemon/resident service, autonomous execution, push, merge,
release, or publication.

`CURRENT`: Slice 4 governance freezes exactly three vertical WorkItems and no
S4-W4. S4-W1 now delivers deterministic versioned customer Rule evaluation,
injected customer authorization, restart-safe durable ApprovalRequest
pause/resolution, exact seven-stream resume Candidates, Claim fencing, and
rebuildable RuleSet/approval projection and GlobalReadView records. Fresh
Implementation Review 1 found four authority/retry/restart/port defects;
bounded Repair 1 closed all four, passed focused race repetition, full
repository and repository-race tests, vet, format, scope, and trust-boundary
audits, and fresh independent Repair Review 2 returned `PASS` with no findings.
No real authentication surface, external approval action, approved execution,
API/CLI, daemon, Runtime/Provider, output/retry/Verifier/Done authority, or
autonomy is activated.

`CURRENT`: S4-W2 now delivers exact authorized-output summaries, pure
versioned output classification, pure bounded recovery decisions, and
Journal-authoritative Team recovery. The first Team dispatch freezes complete
per-node semantic and workflow bindings; terminal attempts bind Store-returned
Evidence receipts and exact classification; retry/fallback uses distinct
Run/generation/Grant/Evidence lineage, explicit `retry_at`, bounded credits,
and one Team-stream CAS. Legacy Slice 3 Team streams remain readable but cannot
silently gain S4-W2 authority. Projection and `GlobalReadView` expose copied
semantic/classification/recovery metadata while preserving the prior view on
malformed replay. Focused repeated race, full repository and repository-race,
vet, format, Windows compilation, scope, and trust-boundary gates pass; fresh
independent Implementation Review 1 returned `PASS` with no findings. Slice 4
remains `PARTIAL`: deterministic acceptance, independent Verifier isolation,
terminal-once WorkItem Done, and the controlled whole-Slice integration proof
remain in the single S4-W3 boundary. No S4-W4 is permitted.

`CURRENT`: S4-W3 is accepted after bounded Repair 1 and fresh independent
Repair Review 2 `PASS`. Source executors now stop at `ready_for_review`; pure
versioned acceptance verifies exact source Evidence, and medium/high risk uses
a distinct generation-fenced Verifier WorkItem/Run/Grant/Evidence lineage.
One exact-head Journal transaction is the only authority that records the
verification fact, terminal-once WorkItem outcome, Team acceptance, and
optional Team terminal. Verification rejection hands off to the frozen S4-W2
RecoveryPolicy for explicit bounded retry/exhaustion, never hidden fallback.
Projection and `GlobalReadView` expose copied acceptance/verifier/recovery
state and preserve the old view after malformed replay. The complete focused
race, impact, repository, repository-race, vet, format, Windows, scope, and
trust-boundary matrix passes. Slice 4 now has exactly S4-W1 through S4-W3
accepted; the fresh whole-Slice Review remains required before Slice 4 can be
closed. No S4-W4, live Runtime/Provider, daemon, API/CLI/Web/TUI, autonomous
execution, external action, or later-Slice surface is activated.

`CURRENT`: Phase 1 Slice 4 is closed after fresh independent whole-Slice
Review 1 returned `PASS` with no findings. All six frozen exit capabilities
are `DONE`: Customer Rule authority, durable approval, output contract,
bounded recovery policy, verification/completion authority, and the controlled
integration proof. The committed chain contains exactly S4-W1 `87ea092`,
S4-W2 `6d3cbf2`, and S4-W3 `f7c931e`; no S4-W4 exists. Final focused,
repository, repository-race, vet, format, scope, trust-boundary, authority,
secret, and Evidence audits pass, and `go.mod`/`go.sum` are unchanged. Slice 5
may now begin only with its bounded exit-contract governance. This status does
not authorize live Runtime/Provider traffic, daemon activation, network,
credentials, external actions, autonomous execution, later-Slice scope, push,
merge, rebase, reset, release, or publication.

`CURRENT`: Phase 1 Slice 5 Exit Contract is frozen after independent Repair
Review 2 `PASS`. Slice 5 permits exactly two vertical product WorkItems:
S5-W1 local Team-bound observation/timeline/Attention delivery and S5-W2
immutable Coding/knowledge WorkPackages plus the controlled Phase 1
engineering Demo. No S5-W3 is permitted. Reconnect uses a bounded complete
Team-related stream-head vector, not a nonexistent global Journal sequence;
authoritative milestones remain Journal-backed, while only tentative client
delivery is bounded memory state after durable private attempt capture. The
engineering Slice may reach `READY_FOR_FINAL_USER_SIGNOFF`, but a real
installed-Runtime task and the user's final signature remain an explicit human
gate. No product code is yet implemented under S5-W1.

`CURRENT`: S5-W1 Local Observation Stream and CLI Timeline Integration is
frozen after independent Contract Repair Review 3 `PASS`. Repair 2 replaced
one cycle-prone existing `package app` test ownership entry with a new
`package app_test` integration file; production remains one-way
`internal/api -> internal/app`, and product ownership, APIs, bounds, and
authority are unchanged. The current gate is completion and capture of
mandatory behavioral RED. Journal, Projection, API, and CLI edits are
test-only. No S5-W1 product code, migration, second writer/Projection, daemon,
network, WorkPackage/Demo scope, S5-W3, installed Runtime, Provider/model
traffic, or autonomous execution is authorized.

`CURRENT`: S5-W1 mandatory behavioral RED is captured with product code still
absent. The exact five-package command fails on the missing bounded Journal
page API/errors, selective `GlobalReadView` accessors, complete
`internal/api` stream/cursor/gap surface, external observer conformance, and
CLI timeline wiring. All RED edits are within frozen test ownership. The
current gate is minimal GREEN beginning with the Journal page and immutable
view prerequisites, followed by the same vertical API/CLI Candidate; no
separate helper WorkItem is permitted.

`CURRENT`: The bounded Journal page and selective copied `GlobalReadView`
prerequisites are GREEN. Before API implementation, a scope trace found one
contract spelling defect: accepted Grant lifecycle streams are keyed as
`agent-grant/<run_id>`, not `<grant_id>`. Contract Repair 3 changes only that
existing-authority identity and keeps copied per-Run Grant records as binding
checks. Fresh independent Contract Repair Review 4 returned `PASS`. The
API cursor/gap/SQLite timeline and CLI focused paths are GREEN, but the
coordinator integration trace found that accepted dispatch/rebound writes can
be followed by task execution before the coordinator refreshes its Projection.
The frozen observer cannot safely validate the new exact generation from that
stale view, and observer-side rebuild would violate its non-blocking boundary.
Slice 5 Exit Contract Amendment 1 requests only two app call-site refreshes
before execution plus focused tests. Fresh independent Amendment Review 1
returned `PASS` with no findings. The current gate is mandatory app freshness
RED followed by the exact two-call-site GREEN; no app production file has been
edited yet.

`CURRENT`: Amendment 1 lifecycle RED is captured. Both first dispatch and
generation rebound executed against the stale pre-write Projection, causing
the exact-binding observer to reject the otherwise authorized output and the
Team to end blocked. No app production edit preceded RED. The current gate is
the minimal two-call-site Projection refresh GREEN.

`CURRENT`: S5-W1 is `ready_for_review`. The bounded Journal after-head reader,
selective immutable view accessors, Team-bound authoritative timeline,
canonical reconnect cursor, board, Attention, recoverable gaps, bounded
tentative subscription, read-only CLI, and Amendment 1 dispatch/rebound
freshness are GREEN. The external app-to-API canary proves durable private
capture precedes tentative delivery, stale generation publishes nothing,
closed subscribers cannot fail the Run, tentative text never enters Journal,
and final Run/Work/Evidence/Team milestones resolve to one exact node/attempt
lineage. Focused, impact, repeated race, full repository, repository-race,
vet, format, Windows compilation, scope, dependency, and trust-boundary gates
pass. Fresh independent S5-W1 Implementation Review remains required before
acceptance or local atomic commit. No S5-W2/S5-W3, daemon, network,
installed-Runtime/Provider traffic, credential, external action, or autonomy
is activated.

`CURRENT`: S5-W1 Implementation Review 1 found no product-behavior defect but
returned `FAIL` after treating pre-existing user-owned shared-worktree dirt as
part of the Candidate. Repair 1 adds explicit scope provenance: the exact
S5-W1 ownership set is reviewed independently, while `AGENTS.md`,
`PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and the post-S3 scratch queue
remain excluded and unstaged. The final high-risk canary now also executes a
distinct verifier and proves its tentative output cannot enter the source Team
subscription or Journal while source/verifier authoritative lineage remains
reconstructable. It exposed and closed two related-scope defects: verifier
WorkItem lineage now uses the accepted parent relation, and the verifier
WorkItem stream is included in reconnect cursors. The full focused,
repeated-race, repository,
repository-race, vet, format, Windows compile, dependency, and trust-boundary
matrix passes. The current gate is fresh Implementation Repair 1 Review 2.

`CURRENT`: S5-W1 Implementation Repair 1 Review 2 returned `FAIL` on two
fail-closed gaps: known delivery `retry_at`/Evidence digest fields accepted
arbitrary safe strings, and WorkItem Evidence references entered cursor scope
without an exact Evidence-to-Team-attempt relation check. Repair 2 captured
both REDs, now accepts only canonical UTC RFC3339Nano/lower-case SHA-256
payload values, and validates every source/verifier Evidence reference through
the projected record before scope inclusion. The dangling-verifier-Evidence
SQLite fixture now fails closed. Focused ten-run tests, the exact repeated-race
matrix, full repository and repository-race, vet, format, Windows compile,
module, diff, and scope gates pass. The current gate is fresh Implementation
Repair 2 Review 3.

`CURRENT`: S5-W1 is accepted for the exact local atomic commit after fresh
Implementation Repair 2 Review 3 returned `PASS` with no findings. The final
Candidate provides bounded Journal-authoritative Team timeline/reconnect,
board and Attention projection, source-only authorized tentative delivery,
canonical fail-closed payload mapping, complete source/verifier related scope,
and the finite read-only `loom timeline` CLI. Focused, ten-run canaries, exact
repeated-race, repository, repository-race, vet, format, Windows compile,
module, dependency, migration, scope, and trust-boundary gates pass. Excluded
shared-worktree paths remain unstaged. No S5-W2/S5-W3, daemon, network,
installed Runtime/Provider traffic, credential, external action, or autonomy
is activated.

`CURRENT`: S5-W1 is locally committed at `93bdb1f`. The only remaining Slice 5
product boundary is S5-W2 WorkPackage and Phase 1 Engineering Demo
Integration; no S5-W3 is permitted. S5-W2 freezes new immutable
`internal/work` WorkPackage values, one external-package controlled
SQLite/Supervisor demo suite, and a non-executing live-gate manifest/checklist
only. It reuses every accepted authority without reopening product files,
activating an installed Runtime/Provider, or claiming final user acceptance.
The current gate is fresh independent S5-W2 Contract Review before RED.

`CURRENT`: S5-W2 Contract Review 1 returned `FAIL` on two contract-precision
gaps only. Contract Repair 1 adds the accepted `internal/mode` routing API to
the external canary import surface and freezes the exact ordered, bounded
approval/stop/recovery safety arrays plus the complete non-executing manifest
JSON. Product ownership, WorkPackage identity, authority boundaries, and live
exclusions are unchanged. The current gate is fresh independent Contract
Repair Review 2; no S5-W2 product or test edit has begun.

`CURRENT`: S5-W2 Contract Repair Review 2 returned `PASS` with no findings.
The frozen contract is now authoritative for the final engineering WorkItem.
The current gate is mandatory behavioral RED in only
`internal/work/work_package_test.go` and
`internal/app/phase1_engineering_demo_test.go`; no S5-W2 product implementation
or live-gate artifact exists yet.

`CURRENT`: S5-W2 mandatory RED was captured on duplicate/mutable WorkPackage
acceptance and the absent live-gate manifest. The Candidate is now focused
GREEN within the frozen ownership: immutable exact-digest Coding and
knowledge-work packages, a controlled external-package saved-Team/DAG
engineering canary, and the exact non-executing final live-gate
manifest/checklist. The canary exercises bounded recovery, independent
verification, approval restart, cursor reconnect, exact-once authority facts,
fail-closed stale inputs, and private fixture modes without installed Runtime,
Provider/model traffic, network, daemon, credential, or external effect. The
current gate is the complete S5-W2 verification matrix followed by fresh
independent Implementation Review; this is not final user acceptance.

`CURRENT`: The complete S5-W2 verification matrix is `PASS`: focused and
adjacent packages, whole-repository tests, repeated and whole-repository race,
vet, format, diff, Windows compile, and module verification all pass. Scope
and safety audits show only frozen Candidate files; shared-worktree dirt
remains excluded. The current gate is fresh independent S5-W2 Implementation
Review. No commit or Slice/Phase acceptance is authorized until that Reviewer
returns `PASS`.

`CURRENT`: S5-W2 Implementation Review 1 returned `FAIL` despite a clean
matrix. The canary lacked a direct two-coordinator CAS race, a canonical stale
cursor conflict, and a test-only exact-once external-effect marker; two
Journal assertions also used `work_item/` instead of the accepted
`work-item/` stream prefix. Repair is confined to the already-owned external
canary and S5-W2 evidence. No product authority, ownership, manifest,
WorkPackage identity, or live boundary is reopened. The current gate is
focused Repair 1 evidence, then the complete matrix and fresh independent
Implementation Review 2.

`CURRENT`: S5-W2 Implementation Repair 1 closes all four Review 1 findings
inside the already-owned external canary. Two fully independent callers now
race the same Journal transition with one executing winner; a canonical
cursor with a conflicting stored head fails closed; a dedicated test-only
effect marker remains exactly once across restart/replay; and both WorkItem
checks use `work-item/`. Focused tests pass 20 iterations and the repaired
concurrency/fail-closed families pass 10 race iterations. The current gate is
the complete post-repair matrix followed by fresh independent Implementation
Review 2; no commit is yet authorized.

`CURRENT`: The complete post-Repair 1 S5-W2 matrix is `PASS`, including the
repaired dual-caller/stale-cursor/effect-marker families, focused and adjacent
packages, whole repository, 10-run `work/app/api` race, whole-repository race,
vet, format, diff, Windows compile, and module verification. Scope remains
exactly frozen and excluded shared-worktree dirt remains untouched. The
current gate is fresh independent S5-W2 Implementation Review 2. No commit,
Slice acceptance, or final user sign-off has occurred.

`CURRENT`: S5-W2 Implementation Review 2 returned `FAIL` on one evidence
honesty gap only. Review 1's four findings are closed and no production defect
was found, but the exact-once assertion did not count Grant Journal facts
despite claiming Grant coverage. Repair 2 remains inside the owned external
canary and will assert five distinct Grant streams, exactly one issue and
revocation per stream, three bounded authorized Frame facts per stream, and
five identity reservations. The current gate is focused Repair 2 evidence,
the full matrix, and fresh independent Implementation Review 3.

`CURRENT`: S5-W2 Implementation Repair 2 explicitly closes the Review 2 Grant
evidence gap: five identity reservations and five distinct Grant streams each
have exactly one issue, three bounded authorizations, and one revocation.
Focused exact-once tests pass 20 iterations and 10 race iterations. No product
file or boundary changed. The current gate is the complete post-Repair 2
matrix followed by fresh independent Implementation Review 3.

`CURRENT`: The complete post-Repair 2 S5-W2 matrix is `PASS`: focused and
adjacent packages, whole repository, 10-run `work/app/api` race,
whole-repository race, vet, format, diff, Windows compile, and module
verification all pass with the explicit Grant lifecycle assertions enabled.
Scope remains frozen and excluded user-owned dirt remains untouched. The
current gate is fresh independent S5-W2 Implementation Review 3. No commit or
acceptance has occurred.

`CURRENT`: S5-W2 Implementation Review 3 returned `PASS` with no findings.
Both prior review rounds are closed by direct canary evidence, and the final
post-Repair 2 matrix remains fully green. S5-W2 is accepted for one exact
local atomic commit containing only the frozen WorkPackage, external
engineering canary, S5-W2 evidence/live-gate artifacts, and this Controller
hunk. Excluded user-owned dirt remains unstaged. No live Runtime/Provider,
network, daemon, credential, external effect, push, merge, or final user
sign-off is authorized.

`CURRENT`: S5-W2 is locally committed at `30b74ff`. Fresh independent
whole-Slice Review returned `PASS` with no findings. Exactly S5-W1
`93bdb1f` and S5-W2 `30b74ff` close Slice 5; no S5-W3 exists. All seven Slice
5 engineering exit capabilities and all 22 TECH-PLAN Phase 1 engineering
acceptance items are `DONE`. Full repository, repository-race, focused
repeated-race, vet, format, diff, module, Windows, scope, authority, secret,
Evidence, cursor, and private-mode gates pass. Phase 1 is
`READY_FOR_FINAL_USER_SIGNOFF`, not `COMPLETE`.

`CURRENT`: Phase 1 is `COMPLETE` after the unique Pi `0.82.1` Transcript
Compatibility Closure Contract passed its single Repair-2 controlled local
live canary, fresh independent result-evidence Review returned `PASS` with no
findings, and the user explicitly supplied final review/sign-off on
`2026-07-28`. The live path closed installed Runtime discovery, local model
execution, strict Pi RPC transcript compatibility, Supervisor/Grant/Frame/
Evidence authority, WorkItem verification and `done`, Team acceptance and
terminal success, and Projection rebuild. The final implementation and result
evidence commits are `c6f9ce7` and `7794b79`. All invocation allowances are
consumed; no rerun, retry, fallback, compaction, alternate model, Provider
switch, parser widening, resident daemon, production activation, credential
change, push, merge, release, or publication is authorized by completion.

`CURRENT`: Phase 2A `Local Product Experience` governance is `PARTIAL`. The
Product Owner authorized a TUI-first local product while retaining the CLI for
headless automation, diagnosis, and recovery. Proposed ADR-0011 and the draft
Phase 2A Exit Contract freeze one shared application/authority path, private
versioned daemon IPC, ordinary no-terminal user journeys, and exactly three
vertical WorkItems: P2A-W1 Local App Shell and Read Experience, P2A-W2 Team
Builder and Provider Onboarding, and P2A-W3 Controlled Execution Experience.
No P2A-W4 or wrapper-only WorkItem is permitted. The current gate is fresh
independent ADR/Exit Contract Review; no Phase 2A product code, Provider
traffic, credential mutation, daemon replacement, migration, or live canary is
authorized before that Review returns `PASS`.

`CURRENT`: Phase 2A governance is frozen after fresh independent ADR/Exit
Contract Review returned `PASS` with no blocking findings. ADR-0011 is
`accepted`, and the Phase 2A Exit Contract is `FROZEN`. Reviewer advisories bind
P2A-W1 to an exact owned-file/protocol/UDS-security freeze and P2A-W2 to an
exact Credential Broker/OS Secret Store/redaction freeze before their RED
gates. The current gate is the P2A-W1 Local App Shell and Read Experience child
contract and its fresh independent Contract Review. No Phase 2A product code,
Provider traffic, credential mutation, daemon replacement, migration, or live
canary is yet authorized.

`CURRENT`: P2A-W1 Local App Shell and Read Experience has one draft child
contract. It freezes a read-only Bubble Tea v1.3.4 product, bounded Projection
enumeration, typed local product queries, private framed UDS v1, daemon/client
lifecycle, ordinary CLI-to-API routing with explicit offline recovery, eight
TUI screens, exact legacy Phase 1 timeline compatibility, a user-level
launcher, deterministic real-SQLite/UDS/TUI evidence, and a post-
Implementation-Review resident-daemon read-only live gate. The current gate is
fresh independent P2A-W1 Contract Review. No W1 product code, dependency edit,
running LaunchAgent change, Provider traffic, credential mutation, Runtime
execution, or live gate is authorized before that Review returns `PASS`.

`CURRENT`: P2A-W1 Contract Review returned `PASS` with no blocking findings.
The child contract is `FROZEN`. Reviewer advisories preserve exact dependency
verification, the existing copied single-head accessor, and compile-only
Windows portability without weakening unsupported-platform peer rejection. The
mandatory RED is captured: each focused failure names a missing frozen product
boundary rather than a syntax, fixture, network, or test-helper failure. The
bounded W1 Candidate and deterministic verification are GREEN, including
focused, race, repository, repository-race, vet, module-checksum, compile-only
Windows, installer, real-SQLite/UDS/headless-TUI, import-direction, scope, and
secret-negative gates. A contract inconsistency between the ordinary default
socket and section 16's proposed live socket is captured as bounded Amendment
1; fresh independent Amendment Review returned `PASS` with no blocking
findings, and no workaround or second socket was introduced. Its live advisory
requires pre-state identity for both the authoritative default path and any
historical demo-resident socket. The current gate is fresh independent
Implementation Review. The resident daemon remains running and unchanged; no
Provider traffic, credential mutation, Runtime execution, installed product
change, or live gate has begun.

`CURRENT`: P2A-W1 Implementation Review 1 returned `FAIL` before any live
mutation. One bounded Repair inside the frozen owned files now closes every
finding: cursor-correct stale reads rebuild from the last immutable
`GlobalReadView`; actionable Attention and two-Run/Evidence Compare are real;
partial/gap/board/Attention/fatal-protocol states and in-flight cancellation
are visible; IPC handler/Accept/Close and exact socket/lock cleanup are bounded;
the installer is transactionally recoverable and runs the installed Bubble Tea
binary under a controlled PTY; and the non-empty SQLite/UDS/TUI/CLI/reconnect/
restart test preserves exact Event count and stream-head digest. Focused,
focused-race, whole-repository, repository-race, vet, module, Windows compile,
coverage, repeated stress, installer, boundary, secret-negative, and diff
checks are GREEN. The current gate is a fresh independent Implementation
Re-review. The resident daemon and default socket remain unchanged; no live
gate, Provider traffic, credential mutation, Runtime execution, installed
product change, staging, or commit has begun.

`CURRENT`: P2A-W1 Implementation Re-review 2 returned `FAIL` before any live
mutation. Repair 2 remains inside the frozen W1 ownership and closes all three
findings: IPC dispatch now requires exactly one bounded frame followed by EOF,
including direct half-open rejection; trusted context-aware handlers execute
inside the tracked connection lifecycle with no detached post-shutdown
goroutine; and the real compiled `loom` CLI now reads status and timeline from
the same non-empty SQLite-backed product daemon and immutable view exercised by
the TUI. Focused, uncached whole-repository, uncached repository-race, vet,
module, Windows compile, coverage (`localipc` 80.9%, TUI 86.8%), repeated
stress/race, installer, format, diff, import-boundary, and secret-negative
checks are GREEN. The current gate is fresh independent Implementation Review
3. The resident daemon and both authoritative/historical socket paths remain
unchanged; no live gate, Provider traffic, credential mutation, Runtime
execution, installed product change, staging, or commit has begun.

`CURRENT`: P2A-W1 fresh independent Implementation Review 3 returned `PASS`
with no findings. The Reviewer independently passed focused, focused-race,
whole-repository, repository-race, vet, Windows compile, installer, module,
diff, scope, authority, and secret-boundary checks and performed no mutation or
live action. The only unlocked action is the single controlled read-only
resident-daemon live gate at the reviewed default
`~/Library/Application Support/Loom/run/loomd.sock`, with exact pre-state
capture and rollback. Provider/model requests, Runtime execution, credentials,
authoritative writes, a second socket, retry, staging, commit, push, merge, or
release remain excluded until that gate returns its own result.

`CURRENT`: P2A-W1 Controlled Resident Live Gate 1 returned
`FAIL — ROLLED_BACK` before any daemon socket or TUI started. The exact
candidate install booted out the old service once, but immediate
credential-clean candidate bootstrap returned macOS error 5. The transaction
restored the original `loom`, `loomd`, wrapper, plist, absent launcher/run
directory/socket, SQLite hash/integrity/one-Event state, and running observer.
No product live capability is claimed. Read-only launchd evidence places
service removal and Background Task Management reconciliation about 136ms
apart, supporting a bounded namespace-quiescence diagnosis. Draft Amendment 2
allows only one replacement canary after independent Review: wait at most ten
seconds for exact service/PID absence before one bootstrap; any repeated
failure, metadata Provider key, or invariant change restores pre-state and
stops `HUMAN_REQUIRED`. No retry is currently authorized.

`CURRENT`: P2A-W1 Amendment 2 independent Review returned `PASS` with no
findings and the amendment is `FROZEN`. The Reviewer independently confirmed
the exact restored installed/SQLite/service state and rebuilt the unchanged
Candidate to the same two Live Gate 1 hashes. Exactly one replacement canary is
now authorized: after clean `bootout`, poll only exact service/PID absence for
at most ten seconds before one bootstrap. No third attempt, global launchd
environment mutation, alternate daemon/socket, Provider/Runtime/model action,
or state write is authorized.

`CURRENT`: P2A-W1 Amendment 2 replacement live gate is
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`. Bounded service/PID quiescence succeeded,
the exact reviewed candidate bootstrapped, and the private default socket,
candidate hashes/modes, process secret-negative scan, and unchanged SQLite
Event/head invariants all passed. The loaded launchd service metadata
nevertheless retained legacy `DEEPSEEK_BASE_URL`, `STEPFUN_BASE_URL`,
`MINIMAX_BASE_URL`, `DEEPSEEK_API_KEY`, and `STEPFUN_API_KEY` keys. No values
were printed or passed to the daemon. The gate therefore stopped before TUI/
Computer Use and atomically restored the original binaries/plist, absent
launcher/run directory/sockets, unchanged one-Event SQLite state, and running
observer. Both governed live allowances are consumed; no third bootstrap,
global `launchctl setenv/unsetenv`, credential mutation, W1 commit, or P2A-W2
freeze is authorized.

`CURRENT`: P2A-W1 replacement Result-Evidence Review 1 returned `FAIL` on one
evidence-only methodology error: the stable pipe-delimited head-row hash was
misnamed as the canonical `GlobalReadView` digest. Repair 1 recomputed the
implementation's exact NUL-delimited format as
`6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`
from the unchanged one-Event SQLite bytes and corrected both gate records.
There was no product edit, service action, credential value inspection, third
live attempt, staging, or commit. The fail-closed `HUMAN_REQUIRED` result is
unchanged and awaits fresh result-evidence re-review.

`CURRENT`: P2A-W1 replacement Result-Evidence Review 2 returned `PASS` with no
findings. The Reviewer independently reproduced canonical digest
`6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`
from the unchanged installed one-Event SQLite state and rechecked exact
rollback, absent product launcher/socket/run directory, restored observer,
clean disk plist, and empty staged diff. This PASS validates only the
fail-closed evidence. P2A-W1 remains `HUMAN_REQUIRED`, uncommitted, and not
live-delivered; no third bootstrap or P2A-W2 work is authorized.

`CURRENT`: P2A-W1 Reopen 3 is a non-executable draft for
`Ambient Service-Manager Metadata Attribution`. Read-only evidence proves the
five loaded service marker keys are the exact non-empty user-level launchd
manager environment set, while the Loom disk plist, reviewed `env -i` wrapper,
candidate process, logs, Journal, and evidence were clean. The draft therefore
proposes attribution plus non-inheritance instead of deleting unrelated global
credentials: exact manager/service key-name equality, non-emitting unchanged-
value checks, zero child inheritance, and no `setenv/unsetenv`. It requests one
explicitly authorized final canary after independent Review; it does not
currently authorize a third bootstrap or change the `HUMAN_REQUIRED` result.

`CURRENT`: P2A-W1 Reopen 3 independent Review returned `PASS` with no findings.
The reviewed attribution/non-inheritance predicate preserves clean plist,
reviewed `env -i` wrapper, zero child inheritance, secret-negative product
surfaces, exact ambient/service five-key equality, unchanged-value checks, and
fail-closed rollback without credential or global-environment mutation.
Reopen 3 is `REVIEWED`, not active. It still requires the exact new user
authorization named in the contract before one final canary; P2A-W1 remains
`HUMAN_REQUIRED`, uncommitted, and not live-delivered.

`CURRENT`: The user supplied Reopen 3's exact activation phrase. P2A-W1
`Ambient Service-Manager Metadata Attribution Reopen` is now `ACTIVE` for one
final controlled canary only. The gate must retain exact ambient/service
five-key attribution, unchanged non-emitting values, zero child inheritance,
secret-negative product surfaces, the reviewed Candidate hashes, one installed
Computer Use TUI journey, one quit/relaunch, one daemon restart, unchanged
Journal/Event/head state, and exact rollback on any failure. No fourth canary
or other authority is implied.

`CURRENT`: P2A-W1 Reopen 3 final controlled live canary is
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`. The reviewed Candidate was installed
atomically and bootstrapped once, but the first aggregate validator failed
before Computer Use/TUI and before the explicit daemon restart. Read-only
diagnosis proves a live-harness `test_defect`: `ps -eww -p <pid>` selected
1,074 host processes, so its secret-negative scan matched unrelated process
state; the target-only command selected exactly one Candidate row and was
secret-negative. Disk plist, launcher, clean `env -i` wrapper, exact ambient
five-key attribution, SQLite bytes/Event/head state, and non-emitting
credential handling remained clean. Automatic rollback restored exact original
binary/wrapper/plist/SQLite hashes and modes, original observer service, and
absent launcher/product run directory/default and historical sockets. Because
the final Candidate bootstrap occurred, its allowance is consumed despite the
false-positive test classification. No fourth canary or hidden retry is
authorized; W1 is not accepted, installed, committed, or live-delivered, and
P2A-W2 cannot begin.

`CURRENT`: P2A-W1 Reopen 3 final result-evidence Review returned `PASS` with no
findings. The independent Reviewer reproduced the macOS process-selection
defect without emitting rows or values: the guard form selected all current
host processes and matched, while target-only selection returned one clean
restored-observer row. It also reverified original installed hashes/modes,
SQLite integrity/one Event/canonical head digest, exact five ambient marker
names, clean disk plist/wrapper, absent launcher/product sockets/run directory,
loaded restored observer, and empty staged diff. This PASS validates only the
fail-closed evidence, `test_defect` classification, allowance accounting, and
rollback. P2A-W1 remains `HUMAN_REQUIRED`, uncommitted, and not live-delivered;
there is no fourth canary allowance and P2A-W2 remains locked.

`CURRENT`: P2A-W1 Reopen 4 `Vertical Live Closure` is frozen for independent
Contract Review. It combines the two remaining evidence-led W1 blockers rather
than creating another thin WorkItem: exact target-only macOS process
environment attribution and truthful empty-Team/TUI behavior for the actual
one-Event resident Journal. It preserves the accepted terminal read-only
historical execution anchor for Journals that contain a fully related
TeamExecution, but forbids fabricating or importing one into the current
Journal. It owns only TUI empty-state files, a deterministic live-evidence
helper/test, CURRENT, and Reopen evidence. No product edit, live action,
credential access, authority migration, fourth WorkItem, or replacement
bootstrap is authorized before Contract Review `PASS`, RED, GREEN, full
verification, fresh Implementation Review `PASS`, and explicit post-Review
activation.

`CURRENT`: P2A-W1 Reopen 4 independent Contract Review returned `PASS` with no
findings. The Reviewer reproduced target-only BSD `ps eww` behavior without
emitting process rows or values, verified the current one-Event/no-Team
resident Journal, and confirmed that explicit TUI empty-Team behavior preserves
the existing strict historical execution anchor. The amendment adds no W4,
second Journal, migration, fabricated Team, or live authority. The current gate
is mandatory deterministic RED for the TUI empty state and target-process
evidence helper; no replacement canary is active.

`CURRENT`: P2A-W1 Reopen 4 mandatory RED is captured. The focused TUI test
failed because the zero-Team snapshot still rendered generic `No records`
instead of the frozen truthful resident-Journal message. The live-evidence
script test failed because the reviewed helper does not yet exist. Both
failures are exact missing behavior; no live, service, SQLite, credential,
Provider, Runtime, staging, or authority action occurred. Minimal GREEN is the
current gate.

`CURRENT`: Reopen 4 TUI/helper minimal GREEN passed, but repeated W1
verification exposed an existing local-IPC cleanup safety defect before
Implementation Review. APFS can immediately reuse the removed socket's
device/inode pair for a regular replacement; current `fileIdentity` does not
include file kind, so cleanup can misidentify the replacement. The existing
replacement-preservation test reproduces this under `-count=100`. Reopen 4
Repair A is frozen to add type-aware identity and deterministic equal-inode/
different-kind proof in the same W1. The prior Contract Review does not
authorize this newly owned IPC edit; fresh supplemental Contract Review is the
current gate. No IPC source edit, live action, new WorkItem, retry allowance,
or activation has occurred.

`CURRENT`: Reopen 4 Repair A supplemental Contract Review returned `PASS` with
no findings. The Reviewer reproduced the device/inode-only cleanup failure and
confirmed that adding `Lstat` file kind to identity is the minimal bounded fix;
IPC protocol, authority, peer checks, connection lifecycle, timeouts, and error
taxonomy remain closed. The current gate is Repair A's deterministic
equal-device/inode different-kind RED. No live or activation authority is
implied.

`CURRENT`: Reopen 4 Repair A deterministic RED is captured. Equal-device/inode
fake socket and regular `FileInfo` values collide under the current
`fileIdentity`, exactly reproducing the missing type binding without relying on
APFS timing. No live, installed-state, or authority action occurred. Minimal
type-aware identity GREEN is the current gate.

`CURRENT`: P2A-W1 Reopen 4 deterministic implementation is GREEN. The TUI now
truthfully distinguishes a Journal with zero Teams and performs no timeline
request on empty selection; direct Timeline navigation asks the user to select
a Team. The existing strict saved/historical Team path is unchanged. The
target-process helper locks live `ps eww -p` inspection to `/bin/ps`, one row,
closed non-emitting statuses, and passes its fake-process matrix. Repair A adds
`Lstat` file kind to cleanup identity; deterministic equal-inode proof, real
replacement preservation at count 100, and race count 30 pass. Fresh full
repository, full race, vet, dependency, Windows compile, installer, helper,
repeat, coverage, format, scope, and security checks pass sequentially.
Reproducible reviewed Candidate hashes are `b094536f...a20` for `loom` and
`f9529cf6...bbb` for `loomd`. Fresh independent Implementation Review is the
current gate; no live activation, installation, staging, commit, or P2A-W2
work is authorized.

`CURRENT`: P2A-W1 Reopen 4 Implementation Review 1 returned `FAIL` with two
required findings. A transition from a previously selected Team to a zero-Team
snapshot retained the stale timeline/current Team and could refresh it; the
process helper also classified all-zero decimal PIDs such as `00` as
`target_unavailable` rather than `invalid_pid`. Review Repair 1 RED reproduces
both exact gaps. No live, installation, activation, staging, commit, or P2A-W2
work occurred. Minimal review repair is the current gate.

`CURRENT`: P2A-W1 Reopen 4 Review Repair 1 is GREEN. A zero-Team refresh now
clears stale Team/timeline state and cannot re-request it; all-zero decimal PIDs
return `invalid_pid`. Fresh focused, full repository, full race, vet,
dependency, Windows compile, installer, helper, repeated lifecycle, coverage,
format, and diff gates pass after the repair. The active reproducible Candidate
hashes are `7ba4b41d...ffd` for `loom` and `f9529cf6...bbb` for `loomd`; earlier
hashes are superseded. Fresh independent Implementation Re-review is the
current gate. No live activation, installation, staging, commit, or P2A-W2 work
is authorized.

`CURRENT`: P2A-W1 Reopen 4 Implementation Review 2 returned `FAIL` on the
replacement proof harness. The test retained a stale socket and then waited
only for path existence, so it could replace the stale path before the new
Server was ready; concurrent race review reproduced `context canceled`.
Proof Repair 2 replaces that ambiguous observation with the existing exact
`Server.Ready()` barrier, without changing product behavior, timeout, error
assertion, or retry policy. Fresh repeated proof and re-review are the current
gate; no live activation exists.

`CURRENT`: P2A-W1 Reopen 4 Proof Repair 2 is GREEN. Server lifecycle tests now
use the exact existing `Ready()` barrier; path polling remains only in client
transport tests. Concurrent localipc race count 50, replacement race count 200,
complete localipc count 100, fresh full repository, full race, vet, dependency,
installer, helper, format, and diff checks pass. Product code and active
Candidate hashes are unchanged. Fresh independent Implementation Re-review is
the current gate; no live activation or installation authority exists.

`CURRENT`: P2A-W1 Reopen 4 fresh Implementation Re-review 3 returned `PASS`
with no findings. The Reviewer independently passed high-load local-IPC race
proofs, zero-Team/stale-selection behavior, all-zero PID validation,
type-aware cleanup, focused package checks, format/diff checks, staged-empty
audit, and reproducible Candidate hashes. Reopen 4's deterministic and Review
gates are closed. The current gate is the contract's explicit post-Review user
activation for one replacement controlled canary; this PASS does not itself
authorize installation, bootstrap, Computer Use, commit, or P2A-W2.

`CURRENT`: P2A-W1 Reopen 4 post-Review activation audit is `PASS`. The original
installed binaries, wrapper, plist, running observer, absent product
run/socket/launcher paths, one-Event SQLite bytes and integrity, canonical head
digest, retained reviewed Candidate, and empty staging all match the frozen
pre-state. No live invocation was consumed and no installation, restart,
Journal write, Provider, Runtime, or authority action occurred. The only
remaining gate is the exact explicit post-Review activation phrase frozen in
Reopen 4 section 10; P2A-W1 remains `HUMAN_REQUIRED` and P2A-W2 remains locked.

`CURRENT`: the user supplied the exact Reopen 4 post-Review activation phrase
for `P2A-W1 Vertical Live Closure Reopen 4 and one replacement controlled
canary`. Exactly one Candidate bootstrap is now authorized. The allowance is
not consumed by the activation record itself; any Candidate bootstrap consumes
it. The frozen no-retry, fail-closed rollback, no Provider/Runtime/Team/write,
Computer Use, one explicit daemon restart, and exact Journal invariants remain
mandatory. P2A-W2 remains locked until the canary result and result-evidence
review close W1.

`CURRENT`: P2A-W1 Reopen 4's replacement Candidate bootstrapped once and passed
every pre-UI live gate: exact installed hashes/modes, private socket,
target-process secret-negative predicate, ambient service-manager attribution,
unchanged values, secret-negative surfaces, one-Event Journal bytes/integrity/
canonical digest, and the truthful typed snapshot with the real Pi Runtime and
zero Teams/Runs/Evidence/Attention. Finder Computer Use then opened the exact
installed `Loom.command`, but the Computer Use safety layer prohibited both
reading and operating `com.apple.Terminal`; no separately addressable Loom
window existed. The contract's required TUI screens, empty-Team Enter,
quit/relaunch, and post-UI daemon restart were therefore not proven and were
not replaced with CLI/PTY evidence. The no-retry trap restored exact original
files, service, Journal, absent product paths, and pre-existing Terminal state.
The sole Reopen 4 allowance is consumed. Current result:
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`; P2A-W2 remains locked pending independent
result-evidence review and new user governance, not another hidden canary.

`CURRENT`: fresh independent Reopen 4 Result-Evidence Review returned `PASS`
with no findings. It independently matched the original installed hashes and
modes, retained Candidate, SQLite integrity/Event count/canonical digest,
absent product run/socket/lock/launcher paths, loaded original observer,
non-running Terminal, and empty staging. It confirmed that exact activation
existed, pre-install harness syntax failures were non-mutating/non-consuming,
one Candidate bootstrap consumed the allowance, the Computer Use Terminal
policy block left the required TUI/relaunch/restart proof missing, no CLI/PTY
substitution occurred, and rollback was exact. This Review signs only the
coherence of the fail-closed result. P2A-W1 remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`; it is not installed or accepted, another
canary is not authorized, and P2A-W2 remains locked.

`TARGET`: an evidence-led P2A-W1 native app host revision is frozen for fresh
independent Contract Review. Proposed ADR-0012 refines ADR-0011: the default
macOS launch surface becomes a user-level native window that calls the same
private versioned daemon UDS directly, while Bubble Tea remains the equivalent
text-mode client. This is a vertical P2A-W1 revision, not Reopen 5 or P2A-W4.
The external historical Loom Cockpit bundle/source is reference-only because
its Process-based bridge, workspace discovery, runtime-host scripts, and
persistent CacheStore violate the current one-authority boundary. The new
contract owns an exact dependency-free SwiftUI file set, direct bounded IPC
compatibility, in-memory replaceable state, reproducible app packaging, and
native-window Computer Use proof. No implementation, install, live invocation,
commit, or P2A-W2 authority exists before Contract Review `PASS`.

`TARGET`: P2A-W1 Native App Host Contract Review 1 returned `FAIL` on one real
protocol omission: the Swift closed typed error set did not name Go v1
`unsupported_platform`, which the daemon may emit when peer credential
inspection is unavailable. The contract is repaired to include that code and
to enumerate all twelve safe Go v1 error variants in the real
Go-server/Swift-probe component matrix. No product or live boundary changed.
Fresh independent Contract Re-review is the current gate; RED, Swift
implementation, install, native canary, P2A-W2, and P2A-W4 remain unauthorized.

`TARGET`: P2A-W1 Native App Host Contract Re-review 2 returned `PASS` with no
findings. Repair 1 now binds `unsupported_platform` and every actual Go v1 safe
error code in the Swift and Go-server/Swift-probe matrix. ADR-0012 is accepted;
the Phase 2A Exit Amendment and exact W1 Revision are reviewed and frozen.
Mandatory RED and implementation are now authorized inside the exact native
app owned files. Install, resident daemon mutation, native live canary, P2A-W2,
and P2A-W4 remain unauthorized.

`CURRENT`: P2A-W1 Native App Host deterministic implementation is `GREEN`.
Mandatory RED first proved the Swift core and app packaging transaction were
absent. The Candidate now provides a dependency-free SwiftUI native window,
strict direct Darwin UDS v1 client, copied in-memory store, bounded safe text,
all eight W1 read screens, non-GUI Swift contract probe, reproducible signed
arm64 `.app` builder, and atomic user-owned app install/rollback transaction.
Swift's 14 tests, release build, app build/install fixtures, existing local
product installer, real Go server to Swift probe snapshot/timeline and
twelve-error-code matrix, malicious loopback response matrix, full Go
repository, full race, vet, module, format, and staged-empty gates all pass.
No Go production authority file was reopened; no app/daemon was installed or
launched, Reopen 4 remains consumed, and P2A-W2 remains locked. Fresh
independent Implementation Review is the current gate. A later Review `PASS`
would still require the exact new post-Review native-canary activation phrase.

`CURRENT`: P2A-W1 Native App Host Implementation Review 1 returned `FAIL` on
three bounded gaps: Swift request IDs admitted non-ASCII alphanumerics, safe
text preserved newline/tab controls, and the bundle builder delegated its
static exclusion scan to the test wrapper. Review Repair 1 RED reproduced all
three and also fixed same-contract fatal-state and signal-interrupted installer
rollback boundaries. Repair GREEN now uses exact ASCII ID bytes, single-line
control normalization, builder self-enforcement with a malicious shadow source,
explicit nonrecoverable fatal UI state, descendant owner/mode checks, and
transaction restoration on failure or signal. Swift's 17 tests, release build,
all package/installer fixtures, full Go repository, full race, vet, module,
diff, cache-reset, and staged-empty gates pass. No live/install action occurred.
Fresh independent Implementation Re-review is the current gate; P2A-W2 remains
locked and no native canary authority exists.

`CURRENT`: P2A-W1 Native App Host fresh Implementation Re-review 2 returned
`PASS` with no findings after Review Repair 1. Reviewer verification was
read-only; Swift build cache was reset and staging remains empty. The direct
native UDS client, strict schema/error boundary, safe single-line rendering,
in-memory store, eight read screens, signed app builder, and atomic
install/rollback Candidate are deterministically accepted for the next gate.
This PASS does not authorize install or live work. P2A-W1 is
`HUMAN_REQUIRED` pending the exact post-Review activation phrase
`P2A-W1 Native App Host and one controlled native-window canary`; Reopen 4
remains consumed and P2A-W2 remains locked.

`CURRENT`: P2A-W1 Native App Host post-Review activation audit is `PASS`,
pre-live only. The original installed binaries, wrapper, plist, loaded resident
observer, absent product sockets/launcher, one-Event SQLite bytes/integrity,
canonical head digest, and empty staging remain at the frozen rollback state.
A fresh isolated arm64 ad-hoc-signed native bundle and Go Candidate were built
and hashed; Swift, installer, full Go, race, vet, module, and diff gates pass.
One cold parallel Go run hit two five-second fake-Pi fixture timeouts; both
focused tests and the unchanged fresh full command then passed, and the
transient is preserved in evidence. No install, bootstrap, Computer Use,
Journal write, live invocation, or allowance consumption occurred. The exact
post-Review phrase `P2A-W1 Native App Host and one controlled native-window
canary` remains the sole live gate; P2A-W2 stays locked.

`CURRENT`: the Native App Host controlled live runbook is frozen without
activating it. It binds the one Candidate-bootstrap consumption point, exact
original/app/daemon rollback transaction, direct Computer Use target
`com.earendilworks.loom.local`, fresh accessibility-state discipline, all
eight read screens, empty-Team no-op, quit/relaunch, one explicit daemon
restart, Journal/canonical-view recovery, and secret-negative evidence. It
forbids Terminal/CLI/PTY substitution and hidden retry. No live state changed;
the exact post-Review activation phrase remains required.

`CURRENT`: the exact Native App Host activation reached the pre-bootstrap
installer dry-run and stopped fail-closed before any live mutation because the
accepted resident state has `loom` and `loomd` but no historical
`Loom.command`. Repair 1 RED reproduced that valid legacy pair rejection. The
installer now supports exact empty, binary-pair, and binary-pair-plus-launcher
generation shapes and swaps optional launcher presence without fabricating
prior state. Focused legacy install/rollback, exact live dry-run, Swift,
installer, full repository, race, vet, module, syntax, and diff gates pass.
The original service and Journal remain exact and the native live allowance is
still `1`, unconsumed. Fresh independent Implementation Review is the current
gate; the pre-repair activation cannot authorize post-repair bootstrap.

`CURRENT`: Native App Host Live Pre-Bootstrap Repair 1 Implementation Review 1
returned `FAIL` on a high-risk signal gap: install/rollback signal traps could
clean scratch without restoring and terminating the active transaction.
Deterministic RED proved both rollback and install signal injection could
unexpectedly succeed. Review Repair now separates normal cleanup from
HUP/INT/TERM handling, restores the complete snapshot whenever mutation is
active, blocks reentrant signals, and exits `98`. Both injected signal windows
restore exact controlled digests; legacy and existing installer fixtures,
exact live dry-run, full repository, full race, vet, module, syntax, and diff
gates pass. Live bootstrap remains `0`; fresh independent Implementation
Re-review is the current gate.

`CURRENT`: Native App Host Live Pre-Bootstrap Repair 1 fresh independent
Implementation Re-review 2 returned `PASS` with no findings. The Reviewer
independently reproduced legacy pair/triple launcher semantics, install and
rollback signal restoration with fixed exit `98`, sentinel-guarded test hooks,
existing installer fixtures, exact live resident non-mutating dry-run, diff
checks, and empty staging. No live mutation occurred; Candidate bootstrap
remains `0` and the allowance remains `1`. Because the prior activation
preceded this repaired implementation and Review, it cannot be reused. The
current gate is a new exact post-Review message:
`P2A-W1 Native App Host and one controlled native-window canary`.

`CURRENT`: final post-Review Candidate rebuilding exposed that the native
builder's reproducibility fixture reused one SwiftPM link cache. Two fully
clean builds differed through random LC_UUID and current N_OSO object
timestamps. Repair 2 RED reproduces the mismatch. The builder now disables
linker UUID and automatic signing, strips non-runtime debug/N_OSO symbols, and
performs one final bundle signature. Two package-reset builds now produce
byte-identical signed bundle manifests; Swift, installer, full repository,
race, vet, module, syntax, security, and live-destination dry-run gates pass.
Fresh independent Implementation Review is the current gate. No live action
occurred; bootstrap remains `0` and allowance remains `1`.

`CURRENT`: Native App Host Live Pre-Bootstrap Repair 2 Implementation Review 1
returned `FAIL` only on evidence reproducibility: every individual Candidate
hash and product gate reproduced, but the published complete bundle manifest
digest lacked an exact canonical byte format. The fixture and GREEN evidence
now share one frozen algorithm: raw-relative-path sort and
`path NUL stat-%Sp-mode NUL lowercase-sha256 LF` rows, then lowercase SHA-256.
No Candidate binary or live state changed. Fresh independent Re-review of the
manifest evidence is the current gate.

`CURRENT`: Native App Host Live Pre-Bootstrap Repair 2 fresh independent
Implementation Re-review 2 returned `PASS` with no findings. The Reviewer
independently ran clean builds, reproduced every published binary/file hash and
the exact canonical complete bundle manifest
`221e9878...f67a`, and reconfirmed both live destination dry-runs, strict
signature/arm64/no-UUID/no-symlink/security gates, empty staging, and removed
Swift cache. No live action occurred; bootstrap remains `0` and allowance
remains `1`. A new exact post-Review native-canary activation is the current
gate.

`CURRENT`: the final Native App Host post-Review activation audit is `PASS`,
pre-live only. It binds the complete Repair 1/2 Review lineage, independently
reproduced Go and canonical signed app hashes, exact original rollback hashes,
running observer, one-Event Journal/canonical digest, absent product paths,
empty staging, bootstrap count `0`, and allowance `1`. No earlier activation
may be reused because it predates the final reviewed Candidate. The sole
remaining gate is the exact message
`P2A-W1 Native App Host and one controlled native-window canary`.

`CURRENT`: the sole Native App Host controlled native-window canary is
consumed and failed closed. The exact reviewed app and daemon installed,
bootstrapped once, exposed the private `0600` AF_UNIX socket, returned the
versioned one-Runtime/zero-Team typed view, preserved the Journal, and kept the
Candidate target process free of all five frozen Provider marker names.
Computer Use targeted only `com.earendilworks.loom.local`, but two fresh
Accessibility-state reads timed out without yielding an independently
addressable native window. No Terminal/CLI/PTY substitution, eight-screen
claim, relaunch, or Candidate daemon restart was made. The transaction rolled
back. A post-rollback macOS provenance/code-signing refusal initially left the
exact original service loaded but inactive; the same rollback captured both
original provenance xattrs, temporarily removed them for bootstrap, restored
them byte-for-byte, and recovered the exact original observer to `running`.
Original binary/plist/SQLite hashes, one-Event integrity, absent Candidate
paths, clean target-process marker predicate, and empty staging now match the
frozen pre-state. P2A-W1 is
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`; its native allowance is `0`, it is not
installed, accepted, or committed, and P2A-W2 remains locked. Fresh independent
Result-Evidence Review is the current gate and cannot authorize a retry.

`CURRENT`: Native App Host Result-Evidence Review 1 returned `FAIL` on one
exact-rollback evidence gap. Original `loom` and `loomd` hashes were correct,
the observer was running, Journal and absent Candidate paths were exact, and
staging was empty, but both installed binaries were mode `0700` instead of the
frozen original `0755`. Result Repair 1 revalidated both hashes and restored
only those two modes to `0755`; the original observer remained running and no
Candidate, app, socket, Journal, xattr, plist, bootstrap, restart, or canary
action occurred. Fresh independent Result-Evidence Re-review is the current
gate. Product status remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance
`0`, with P2A-W2 locked.

`CURRENT`: fresh independent Native App Host Result-Evidence Re-review 2
returned `PASS` with no findings. It reproduced all five frozen hashes and
modes, the running original observer, zero Provider markers in the actual
target process, SQLite integrity and one Event, both restored provenance xattr
names, absent Candidate app/run/socket/launcher/process paths, and empty
staging. This PASS validates only the coherent fail-closed result and exact
rollback. P2A-W1 remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`, native
allowance `0`, uninstalled, unaccepted, and uncommitted; it does not authorize
retry or another canary, and P2A-W2 remains locked.

`TARGET`: immutable post-canary crash inspection has closed the native-window
root cause without another live invocation. Both Computer Use timeouts map to
separate `LoomLocalApp` crash reports with
`EXC_CRASH/SIGABRT`, termination namespace `DYLD`, and reason
`missing LC_UUID load command`. The reviewed builder deliberately used
`-no_uuid`, and its fixture incorrectly required UUID absence. Two isolated
package-reset experiments with linker `-reproducible`, retained
`-no_adhoc_codesign`, `strip -S`, and same-basename final signing produced the
same non-empty content-derived arm64 UUID, byte-identical unsigned and signed
executables, and strict-valid signatures. A single same-W1 Native App
Launchability and Reproducibility Closure Contract is proposed for fresh
independent Contract Review. It owns only the builder/test fixture and
governance evidence, requires UUID-present byte-identical builds plus a bounded
private-bundle process-survival smoke, and grants no live authority. Product
status remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`, native allowance `0`,
with P2A-W2 locked.

`TARGET`: Native App Launchability Contract Review 1 returned `FAIL` on one
external-side-effect gap: the proposed RED would execute the known no-UUID
binary and could generate another user DiagnosticReports `.ips` outside the
private fixture root. Contract Repair 1 now makes missing UUID a pre-spawn RED,
requires unchanged crash-report inventory, and allows the private process
smoke only after UUID/architecture/signature gates pass in GREEN. The smoke
records before/after report names and hashes; any unexpected new report is
preserved and stops `HUMAN_REQUIRED`, never silently deleted. No linker,
product, ownership, live, allowance, or WorkItem boundary changed. Fresh
independent Contract Re-review is the current gate; implementation and live
remain unauthorized.

`CURRENT`: fresh independent Native App Launchability Contract Re-review 2
returned `PASS` with no findings. The repaired contract now safely freezes
pre-spawn missing-UUID RED, UUID/architecture/signature-gated GREEN process
smoke, before/after crash-report inventory, exact child cleanup, and
preservation plus `HUMAN_REQUIRED` on any unexpected report. Ownership remains
the exact builder/test pair plus governance evidence; no Swift, Go, daemon,
IPC, authority, WorkItem, or live boundary is reopened. Mandatory RED and
implementation are authorized inside that ownership. Native allowance remains
`0`, installation/Computer Use/live remain unauthorized, and P2A-W2 stays
locked.

`CURRENT`: Visual Review 2 returned `FAIL` on one evidence-consistency P1 and
no product UI finding. All nine PNG hashes and all prior visual repairs passed,
but the semantic comparison still described the shared prepared Authorization
fixture as absent while both final clients correctly showed it as prepared.
The comparison now records `authorization`, native `Open Authorization` and
TUI `Authorization prepared · a open`, and separately identifies the tested
missing-Evidence Review fixture as the read-only disabled case. No source byte
or source lock changed, so Implementation Review 6 remains valid. Fresh Visual
Review is the current gate; live remains locked.

`CURRENT`: fresh independent Visual Review 3 returned `PASS` with no P0, P1
or P2 visual findings. It confirmed the corrected shared Authorization
semantics, all nine artifact hashes, exact card/five-lane/compact/Light-Dark
behavior, three-column permission-aware Mission Room, TUI Mission Detail and
all three Decision sheets. Implementation Review 6 and source lock
`de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`
remain valid. Atomic owned-file commit is now the gate; live remains locked
until post-commit preflight independently binds commit and binary hashes.

`CURRENT`: fresh independent Replacement Live Gate Contract Review returned
`PASS` with no findings. It validated the consumed-no-retry accounting, exact
repaired Candidate identity, original mode/provenance-aware rollback, initial
Computer Use no-retry boundary, eight screens, relaunch/restart proof,
non-destructive crash-report inventory, secret-negative evidence, and
Result-Review-before-commit/P2A-W2 sequence. Current original observer,
hashes/modes/xattr names, two reports, SQLite, absent Candidate paths, clean
target-process marker predicate, and staging matched. The contract is frozen,
but replacement allowance remains `0`; only the pre-live post-Review audit is
authorized now.

`CURRENT`: Native App Launchability mandatory RED is preserved. The repaired
build fixture ran against the unchanged builder and failed pre-spawn with
`native app executable is missing LC_UUID`. DiagnosticReports remained exactly
the two preserved canary reports with matching hashes; no third report or
native process was created. The original observer stayed running, resident
SQLite bytes/Event count stayed exact, Candidate app/run/socket/launcher paths
remained absent, and staging remained empty. Implementation may now change
only the frozen builder's linker identity configuration; live remains locked.

`CURRENT`: Native App Launchability implementation is deterministic `GREEN`.
The builder now uses linker `-reproducible` instead of `-no_uuid`, retains
unsigned intermediate linking, `strip -S`, and one final timestamp-free
signature. The repaired fixture rejects missing/random UUID, requires one
nonzero identical arm64 UUID and byte-identical signed bundles across two
package-reset builds, proves no N_OSO records, runs a bounded exact-child
private launch smoke, and leaves crash-report inventory unchanged. The
reproduced Candidate executable is `f473684c...bc06b`, UUID
`CE91F84E-4333-35DB-B493-88FADCBC6EC1`, and canonical bundle manifest
`e29c1b6c...a1cbd`. Swift 17 tests, release build, native/full installer
fixtures, complete Go repository, race, vet, module, format, shell, diff, and
security gates pass. The first full Go run preserved one historical
five-second fake-Pi metadata timeout; the exact focused test passed in 1.89s
and one unchanged fresh full command passed. Original observer/Journal and
absent Candidate paths remain exact, no new crash report exists, Swift cache
and staging are empty. Fresh independent Implementation Review is the current
gate; native allowance remains `0` and live/P2A-W2 stay locked.

`CURRENT`: fresh independent Native App Launchability Implementation Review
returned `PASS` with no findings. Reviewer independently reproduced the same
content-derived UUID `CE91F84E...6EC1`, executable `f473684c...bc06b`,
manifest `e29c1b6c...a1cbd`, strict signature, byte-identical clean bundles,
private exact-child survival/quiescence, unchanged crash-report inventory,
Swift 17 tests, install rollback fixtures, Go full/race/vet/module, and clean
external state. The no-UUID dyld root cause is deterministically repaired.
The prior native canary remains consumed and failed; its allowance is `0`.
P2A-W1 is now
`HUMAN_REQUIRED — DETERMINISTIC REPAIR PASS — LIVE LOCKED`, uninstalled,
uncommitted, and unaccepted. A future installed native-window canary requires
new explicit post-Review governance against the exact reviewed Candidate;
P2A-W2 remains locked.

`CURRENT`: the single P2A-W1 Local Product Vertical Live Closure allowance was
consumed by one Candidate bootstrap and returned
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`. The Candidate daemon exited through the
closed `daemon failed` surface before the transaction emitted `READY`; the
native App was never opened, so Home/Work/Teams/Inbox/System, Refresh,
relaunch, and the later classified lifecycle restart were not attempted. The
transaction initially reported `rollback_incomplete` only because the exact
original observer had not yet stabilized. Bounded rollback completion
re-established the exact original bytes/modes/provenance at their governed
paths, allowed launchd to converge on one PID stable for more than 60 seconds,
restored two
byte-identical historical crash reports after macOS moved them to `Retired`,
and revalidated the original plist, one-Event SQLite digest/integrity, absent
App/run/socket/backups, zero target-process Provider markers, and empty
staging. One diagnostic command over-captured inherited credential values into
transient tool output; no value entered repository evidence, Candidate,
Journal, screenshot, or Loom log, and credentials were not silently changed.
Allowance is `0`, retry is prohibited, P2A-W1 is unaccepted/uncommitted, and
P2A-W2 remains locked pending fresh Result-Evidence Review and human direction.

`TARGET`: one P2A-W1 Native App Launchability Replacement Live Gate Contract
is proposed for fresh independent Contract Review. It does not reinterpret the
consumed canary; it binds the repaired UUID/executable/manifest, exact original
hashes/modes/provenance rollback, unchanged two-report crash inventory,
single-launch Computer Use boundary, all eight native screens,
quit/relaunch/required daemon restart, secret-negative proof, and one
fail-closed replacement allowance. Review alone grants no live authority. Only
a later passing activation audit plus the exact post-Review user message
`P2A-W1 LC_UUID Launchability Repair and one replacement native-window canary`
may make the allowance `1`; no blanket or earlier authorization substitutes.
Until then P2A-W1 remains
`HUMAN_REQUIRED — DETERMINISTIC REPAIR PASS — LIVE LOCKED`, with P2A-W2
locked.

`CURRENT`: the Replacement Live Gate post-Review activation audit is `PASS`,
pre-live only. A first private Go rebuild used the older Reopen 4
`-trimpath -buildvcs=false` artifact family and stopped on the exact-hash guard
without mutation; the current reviewed default-build method then reproduced
the frozen `loom`/`loomd` hashes. A reset Swift build reproduced
`f473684c...bc06b`, UUID `CE91F84E...6EC1`, and manifest
`e29c1b6c...a1cbd`; live-destination dry-runs and all three focused build/
installer fixtures passed. Original hashes/modes/provenance names, running
observer, zero target-process Provider markers, Journal, two crash reports,
absent Candidate paths/process, removed Swift cache, and empty staging remain
exact. No install, bootstrap, Computer Use, or live allowance exists. The only
remaining activation is the exact new post-Review message
`P2A-W1 LC_UUID Launchability Repair and one replacement native-window
canary`; until then allowance remains `0` and P2A-W2 remains locked.

`CURRENT`: the exact post-Review Native App Launchability replacement was
activated and failed closed after one Candidate `launchctl bootstrap`, before
Computer Use or native app launch. The exact reviewed app/product installed,
the original observer quiesced, and the single bootstrap call succeeded, but
the Candidate returned `daemon unavailable` before producing its socket.
Read-only diagnosis proved the live transaction omitted creation of the
required owned mode-`0700` socket parent; `localipc.validateSocketPath` rejects
an absent parent during `newProductDaemonRunner` construction. The frozen
native canary runbook already required creating that directory; this was a
temporary transaction implementation omission. Its internal
`bootstrap_consumed=0` diagnostic was also placed after readiness and is not
the authority: `launchctl bootstrap` returned success and launchd executed the
Candidate, so the contract's post-bootstrap consumption rule applies. The
armed transaction restored exact original binary/plist hashes and modes,
provenance attributes, running observer, zero target-process Provider markers,
unchanged one-Event SQLite bytes/integrity, exact two-report crash inventory,
absent Candidate app/run/socket/launcher/process paths, and empty staging. No
second bootstrap was attempted. A diagnostic command also displayed inherited
environment rows in the local tool transcript; no values were copied into
workspace evidence, but affected Provider credentials must be rotated. The
frozen post-bootstrap rule consumes the sole replacement allowance. P2A-W1 is
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance `0`, unaccepted and
uncommitted; P2A-W2 remains locked. Fresh independent Result-Evidence Review is
the only current gate and cannot authorize a retry.

`CURRENT`: fresh independent Native App Launchability replacement
Result-Evidence Review returned `PASS` with no findings. It independently
reproved the deterministic absent-`0700`-socket-parent failure chain, exact
original hashes/modes/provenance names, running observer, marker count `0`,
unchanged one-Event SQLite state, exact two-report crash inventory, absent
Candidate paths/process/Swift cache, empty staging, consumed allowance, and
the credential-transcript disclosure without copying values. This `PASS`
accepts only the failed-live evidence and exact rollback; it does not accept
the product, authorize another bootstrap, permit the atomic W1 commit, or
unlock P2A-W2. Terminal status remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance `0`, uncommitted, with P2A-W2
locked.

`TARGET`: one final P2A-W1 Native App Final Exit Closure Contract is frozen for
fresh independent Contract Review. It preserves the consumed failed canary and
opens no live authority or product source. The only proposed implementation is
a governed evidence harness plus behavioral fixture that makes the already
frozen run-directory preparation, bootstrap consumption point, no-retry
control flow, exact rollback, native lifecycle sequence, and secret-negative
output mechanically reviewable. It remains P2A-W1, creates no Reopen 5 or
P2A-W4, and permits no further closure split. Contract Review, RED, GREEN, and
Implementation Review cannot authorize live mutation; only a later passing
post-Review audit plus the exact new user phrase frozen in the closure may
grant one final native-window canary. Current product status remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance `0`, uncommitted, with P2A-W2
locked.

`CURRENT`: fresh independent P2A-W1 Native App Final Exit Closure Contract
Review returned `PASS` with no findings. The Reviewer confirmed that the
evidence harness plus behavioral fixture is one final vertical exit closure,
not a product-source change, Reopen 5, point behavior Amendment, or P2A-W4.
Run-root ordering, immediate post-bootstrap allowance accounting, no-backedge
retry closure, lifecycle-restart classification, fixture isolation,
secret-negative output, exact rollback/preserve rules, full verification, and
future live activation are complete and testable. Original service/Journal/
crash/path/staging predicates remain exact. This `PASS` authorizes only
mandatory RED; live allowance remains `0`, P2A-W1 is uncommitted, and P2A-W2
remains locked.

`CURRENT`: P2A-W1 Final Exit Closure mandatory RED is preserved. The new
behavioral fixture syntax passes and exits `1` exactly with
`RED: governed final exit transaction harness is missing`; it already binds
run-root ordering, pre/post-bootstrap consumption, no retry, rollback count,
lifecycle restart, fixture isolation, path/symlink/mode/override attacks, and
bounded secret-negative output. One outer zsh result-capture attempt used the
read-only variable name `status`; it changed no state and the corrected capture
reproduced the exact RED. Original hashes/modes, running observer, marker count
`0`, one-Event SQLite state, two-report crash inventory, absent Candidate/
Swift paths, and empty staging remain exact. Implementation authority is
limited to the governed evidence transaction harness; no product source, live
action, commit, or P2A-W2 work is authorized.

`CURRENT`: P2A-W1 Final Exit Closure deterministic GREEN and complete matrix
pass. The governed harness is production-path fixed, ambient-override closed,
post-Review-audit locked, prepares/validates the `0700` run root before
mutation, assigns consumption immediately after successful bootstrap, has no
readiness-to-bootstrap backedge, classifies one later lifecycle restart, and
can preserve only after Result Review. Its private behavioral fixture uses
internal fake service-manager functions plus a real AF_UNIX socket and rejects
sentinel/path/symlink/mode/owner/override/argument attacks without executing
fixture-supplied code. Harness/production-lock, native build/install/product
fixtures, Swift 17 tests and release build, complete Go repository, race, vet,
module verification, formatting, diff, and secret-value scans pass. Exact
Candidate hashes/LC_UUID/manifest and original service/Journal/view/crash/
path/staging predicates are reproduced; Swift cache is absent. The RED
`repo_root` traversal and an initial executable-fixture injection surface were
corrected before GREEN; a naive literal secret scan was also replaced with a
value-shaped scan. Final private-root validation was also moved before rollback
disablement. A 65-character transcription of the second historical crash hash
was corrected to the exact 64-character value; the fixture now locks every
frozen identity constant. Focused gates passed again. No live action occurred
and allowance remains `0`. Fresh
independent Implementation Review is the current gate; P2A-W1 is uncommitted
and P2A-W2 remains locked.

`CURRENT`: the first independent P2A-W1 Final Exit Closure Implementation
Review returned `FAIL` on two harness-only findings: fixture counter writes
could follow a pre-existing symlink outside the private fixture root, and
production preflight did not reject a pre-existing
`Loom.command.previous` even though rollback removes that path. Repair 1 keeps
counter state in process memory, writes only through validated private
same-directory temporary files, rejects symlink/non-regular/foreign-owned/
wrong-mode counters, and adds bootstrap/rollback/restart symlink cases that
prove outside bytes unchanged. Production preflight now requires both launcher
and launcher-backup paths absent. Syntax, normal and `env -i` fixtures, all
three new attacks, diff checks, and exact production exit-`20` live lock pass.
No live mutation occurred; allowance remains `0`. Fresh independent
Implementation re-review is the current gate; P2A-W1 is uncommitted and
P2A-W2 remains locked.

`CURRENT`: Repair 1 re-review confirmed its counter and launcher-backup fixes
but remained `FAIL`: inherited `PYTHONPATH` could execute private
`sitecustomize.py` code at direct Python calls, Git checks inherited
`GIT_INDEX_FILE`, and dangling launcher/binary-backup symlinks could satisfy
an `! -e`-only absence test. Repair 2 now re-executes the fixed harness through
an exact `env -i` boundary before fixture or production dispatch, validates
the resulting environment allowlist, and rejects a forged clean marker with
any injected name. Behavioral cases prove `BASH_ENV`, `ENV`, `PYTHONPATH`,
`GIT_INDEX_FILE`, and the installer signal-control variable are inert and no
private marker executes. Preflight and rollback require both nonexistence and
non-symlink status for the launcher and both binary backup paths. Syntax,
normal/clean-environment fixtures, exact production exit-`20` lock, and diff
checks pass. No live mutation occurred; allowance remains `0`. Fresh
independent Implementation re-review is the current gate, P2A-W1 remains
uncommitted, and P2A-W2 remains locked.

`CURRENT`: P2A-W1 Final Exit Closure Repair 2 now has fresh independent
Implementation Review `PASS` with no findings. The Reviewer reproduced both
syntax checks, normal and outer-`env -i` fixtures, ambient-command and forged
marker closure, dangling-path closure, exact production exit-`20` lock, and
clean diff/staging checks. The primary fresh post-repair matrix also passes:
native build/app installer/product installer fixtures, Swift 17 tests and
release build, complete Go repository, race, vet, module verification,
formatting, diff, and secret-value scans. Swift cache was reset. Exact
Candidate manifest/signature and original hashes/modes, running observer,
marker count `0`, one-Event Journal bytes/integrity, two crash reports, absent
Candidate paths/process, and empty staging remain exact. The post-Review audit
is `PASS — ACTIVATION LOCKED` with allowance `0`; it deliberately cannot
unlock the production harness. P2A-W1 remains uncommitted and unaccepted, and
P2A-W2 remains locked. The only next action is a new exact activation message
frozen in the Final Exit Closure Contract.

`CURRENT`: the exact Final Exit Closure allowance was activated and consumed.
The reviewed transaction created the required private run directory, installed
the exact Candidate, performed one successful initial Candidate bootstrap,
and passed daemon/socket/status/Journal/crash/secret-negative pre-UI gates.
Computer Use opened only the native Loom bundle. Home displayed
`Offline: invalid_response` with zero Runtimes; one bounded native Refresh
returned the same result instead of the required one real Pi Runtime. No other
screen, empty-Team activation, relaunch, lifecycle restart, or second
bootstrap was attempted. Read-only diagnosis closed the cause: the sole
`RuntimeInstanceDiscovered` fact has JSON `model_ids:null`; the Go local
product read model preserves the nil slice and encodes `null`, while Swift
strictly requires `[String]`, mapping the decode failure to
`invalid_response`. The existing real Go-server/Swift-client test hard-codes a
non-null model array and misses this authoritative live shape. The same
reviewed harness rolled back exactly with
`initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0`.
Original hashes/modes/provenance, running observer, marker count `0`,
one-Event SQLite bytes/integrity, exact two crash reports, absent Candidate
paths/process/Swift cache, and empty staging are restored. P2A-W1 is
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance `0`, uncommitted and
unaccepted; P2A-W2 remains locked. Fresh independent Result-Evidence Review is
the only current gate and cannot authorize another attempt or source change.

`CURRENT`: fresh independent P2A-W1 Final Exit Closure Result-Evidence Review
returned `PASS` with no findings. The Reviewer independently reproduced the
authoritative `model_ids:null` shape, nil-preserving Go projection/read-model
path, Swift required-array decode, closed `invalid_response` mapping, and the
hard-coded non-null cross-language fixture gap. It confirmed exactly one
consumed initial bootstrap, zero retry, zero Candidate restart, one rollback,
exact original hashes/modes and running wrapper service, marker count `0`,
one-Event SQLite bytes/integrity, exact two crash hashes, absent Candidate
paths/process/Swift cache, matching transaction hash, passing working/cached
diff checks, and empty staging. This `PASS` accepts only the failed-live result
and exact rollback. P2A-W1 remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`, allowance `0`, uncommitted and
unaccepted; P2A-W2 remains locked. No retry, source change, new point closure,
commit, Provider/Runtime execution, push, merge, release, or publish is
authorized.

`CURRENT`: the Product Owner explicitly reopened only the P2A-W1 native
presentation boundary for one vertical product-experience repair. Fresh
independent Contract Review 1 found four completeness gaps; the same contract
recorded the bounded UI-source authority, added a testable non-connecting
`LoomLocalAppUI` boundary, replaced unsupported recency with source-ordered
Work activity, and removed Compare in every state. Fresh independent Contract
Re-review 2 returned `PASS` with no findings. Mandatory RED failed on the
missing frozen UI module. Deterministic GREEN now presents exactly Home, Work,
Teams, Inbox, and System; moves Runs/Evidence, Team timeline, attention, and
Runtime health under user-meaningful destinations; removes the metric-card
operator-console Home; and uses native semantic colors, SF Symbols, one system
accent, functional copy, and accessible recovery targets. Swift 23 tests,
release build, Thread Sanitizer, actual-view non-connecting state/light/dark/
accessibility-size renders, diff, dependency, scope, copy, raw-ID, and staging
audits pass. No executable, socket, daemon, live service, Runtime, Provider,
model, Journal, installer, or native-window canary ran. The separate
`model_ids:null` defect and terminal live result remain unchanged:
`P2A-W1: FAIL - ROLLED_BACK - HUMAN_REQUIRED`, allowance `0`, uncommitted and
unaccepted; P2A-W2 remains locked. Fresh independent Implementation Review is
the current UI-only gate.

`CURRENT`: P2A-W1 Native Product Experience Implementation Review 1 returned
`FAIL` on four bounded findings: non-terminating preview path validation,
state-insensitive non-nil-only render evidence, sanitized internal-ID fallback,
and completed Team timeline failure remaining visually Loading. Repair 1
captured focused RED for the missing safe-name and timeline-state boundaries;
the stronger render test also rejected the old appearance-only digests.
Repair 1 now rejects path mismatch/symlink before export, verifies bounded pixel
content and 16 distinct state/appearance renders plus a narrower accessibility
render, compares both sanitized name inputs, and closes Team timeline as idle,
loading, loaded, unavailable, or fatal. Swift 27 tests pass with the locked
visual-export test skipped by default; release, Thread Sanitizer, diff,
dependency/import, visible-copy, focus/animation, preview-absence, and staging
checks pass. No live action occurred. Fresh independent Implementation
Re-review is the current UI-only gate. The separate `model_ids:null` defect,
failed live result, allowance `0`, uncommitted state, and P2A-W2 lock remain
unchanged.

`CURRENT`: P2A-W1 Native Product Experience Implementation Re-review 2
confirmed the safe-name, state-sensitive render, and terminal Team timeline
findings closed, but returned `FAIL` because the preview validator compared two
resolved instances of the same parent and could not reject an expected path
beneath a symlinked parent. Repair 2 captured that exact failure before changing
the validator, then required the frozen parent to equal its own canonical
resolved path while retaining exact-path and leaf-symlink rejection. The
focused regression now passes. No preview or live action occurred; fresh
independent Implementation Re-review remains the current UI-only gate. The
separate `model_ids:null` defect, failed live result, allowance `0`,
uncommitted state, and P2A-W2 lock remain unchanged.

`CURRENT`: the single reviewed replacement Native Product Experience preview
passed visual audit. The 1100 by 720 fixture-only PNG visibly contains Loom
identity, connection status, exact Home/Work/Teams/Inbox/System navigation,
system-accent Home selection, attention-first Home hierarchy, source-ordered
Work, Agent teams, and system readiness. It contains no copied Multica asset,
fake primary action, raw identifier, credential, hidden reasoning, or live
data. The frozen UI-only reopen is accepted. This is not installed-app,
native-window, daemon, Runtime, Provider, or live proof and does not repair the
separate `model_ids:null` defect. P2A-W1 therefore remains
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`, live allowance `0`, uncommitted and
unaccepted; P2A-W2 remains locked.

`CURRENT`: the only remaining P2A-W1 blocker is now frozen as the single
`Local Product Vertical Live Closure Replacement Contract`. It replaces the
remaining live-exit authority of the old Final Exit Closure and the completed
UI-only reopen without creating another WorkItem or point Amendment. The
contract places the exact `model_ids:null` compatibility repair at the shared
local-product Application/API wire boundary, retains strict Swift decoding,
requires canonical empty arrays for both Runtime collection fields, and
prohibits changes to Journal, Projection, StateWriter, Runtime execution, or
installed state before Review. Current observer/plist/product/one-Event SQLite
hashes and rollback state match the accepted ledger; live allowance remains
`0`, Git staging is empty, and P2A-W2 remains locked. Fresh independent
Contract Review is the next gate.

`CURRENT`: fresh independent Native Product Experience Implementation
Re-review 4 returned `PASS` with no findings after the visual repair. It
confirmed the production native split workspace, exact five semantic
destinations, 44-point selection rows, toolbar/recovery-only Refresh,
read-only interactions, system semantics, and the sidebar-contrast matrix
across all states, light/dark, and accessibility width. Full Swift tests,
release, Thread Sanitizer, diff/static audits, and empty staging passed. The
first preview remains recorded as `FAIL`; exactly one in-memory-stub
replacement preview at the owned path is now permitted. This does not create a
live allowance or change the schema defect, failed live result, allowance `0`,
uncommitted state, or P2A-W2 lock.

`CURRENT`: fresh independent P2A-W1 Native Product Experience Implementation
Re-review 3 returned `PASS` with no findings. It confirmed the exact
standardized preview path, canonical non-symlink parent, leaf-symlink
rejection, parent-symlink regression, 16 distinct state/appearance renders,
bounded image content, sanitized-name fallback, and closed Team timeline
states. Fresh `swift test` again passed 27 tests with only the locked visual
export skipped; diff checks passed, staging was empty, and the preview remained
absent. The reviewed UI-only gate now permits exactly one non-connecting,
in-memory-stub preview export for visual audit. It does not alter the separate
`model_ids:null` defect, failed live result, allowance `0`, uncommitted state,
or P2A-W2 lock.

`CURRENT`: the first bounded Native Product Experience visual audit returned
`FAIL`: Home hierarchy was readable, but the real offscreen render reserved a
completely blank sidebar. The failed 1100 by 720 PNG is recorded by digest in
the visual-audit ledger. A deterministic sidebar-region regression then failed
all state/light-dark/accessibility renders with zero contrast samples. The
same production `ContentView` now uses a native `HSplitView`, explicit
five-destination semantic sidebar, 44-point selectable rows, system-accent
selection, local status, and a `NavigationStack` detail retaining Refresh.
The focused real-view matrix is GREEN. No app, native window, service, socket,
or live action occurred. Complete verification and fresh independent
Implementation Re-review are required before the owned preview may be
replaced. The separate schema defect, failed live result, allowance `0`,
uncommitted state, and P2A-W2 lock remain unchanged.

`CURRENT`: after that recorded failure, fresh independent Implementation
Re-review 4 returned `PASS`, and the single reviewed replacement preview passed
visual audit. The 1100 by 720 fixture-only PNG visibly contains Loom identity,
connection status, exact Home/Work/Teams/Inbox/System navigation,
system-accent Home selection, attention-first Home hierarchy, source-ordered
Work, Agent teams, and system readiness. It contains no copied Multica asset,
fake primary action, raw identifier, credential, hidden reasoning, or live
data. The frozen UI-only reopen is accepted. This is not installed-app,
native-window, daemon, Runtime, Provider, or live proof and does not repair the
separate `model_ids:null` defect. P2A-W1 remains
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`, live allowance `0`, uncommitted and
unaccepted; P2A-W2 remains locked.

`CURRENT`: continuing the full Phase 2A goal, the only remaining P2A-W1
blocker is frozen as the single `Local Product Vertical Live Closure
Replacement Contract`. It replaces the remaining live-exit authority of the
old Final Exit Closure and the completed UI-only reopen without creating
another WorkItem or point Amendment. The exact `model_ids:null` compatibility
repair is bounded to the shared local-product Application/API wire boundary;
strict Swift remains strict, and Journal, Projection, StateWriter, Runtime
execution, and installed state remain closed. Current observer/plist/product
and one-Event SQLite hashes match the accepted rollback ledger. Live allowance
is `0`, staging is empty, and P2A-W2 remains locked. Fresh independent Contract
Review is the current gate.

`CURRENT`: fresh independent Local Product Vertical Live Closure Contract
Review 1 returned `FAIL` on three completeness gaps: missing authoritative
Go-path RED for nil `observed_capabilities`, unbound Candidate source inputs in
the dirty uncommitted worktree, and mutating `go mod tidy` despite no module
lock ownership. The same single replacement contract now requires both sibling
nil-collection RED/GREEN paths, binds base commit plus 53 exact accepted
source/ADR/build/test hashes in source-lock SHA-256
`0bd33d144c5aadcd078bef06a45685b8ac2ed2ab2bf76effe80730ee85e8a128`,
allows only four final closure deltas, excludes generated/private/unrelated
inputs, and uses read-only `go mod tidy -diff` with stop-on-drift. No RED,
product edit, build, install, launchd, App, or live action occurred. Allowance
remains `0`, staging is empty, and P2A-W2 remains locked. Fresh independent
Contract Re-review is the current gate.

`CURRENT`: fresh independent Local Product Vertical Live Closure Contract
Re-review 2 returned `PASS` with no findings. The Reviewer reproduced the
source-lock digest, all 53 path hashes, disjoint four-file delta allowlist,
both sibling nil/null RED requirements, read-only module-lock gate, strict
Swift boundary, and one-bootstrap/no-retry/rollback/P2A-W2 gates. The contract
is now frozen. Mandatory RED is the current gate; no product source, resident
service, App, Journal, or live state has changed, allowance remains `0`, and
staging is empty.

`CURRENT`: mandatory RED for the frozen P2A-W1 Local Product Vertical Live
Closure is confirmed. Real Journal Event to Projection to
`LocalProductReadService` tests fail because historical `model_ids:null` and
the sibling `observed_capabilities:null` both remain nil slices; the real
product daemon SQLite/IPC path fails on the same historical model-list shape.
The strict Swift control accepts canonical empty arrays and rejects null,
missing, duplicate, wrong-type, and unknown Runtime fields, so the client was
not weakened. No product source, Journal, Projection, StateWriter, module
lock, installed file, resident service, App, Provider, Runtime, credential, or
live state changed. Minimal Application/API implementation is the current
gate; allowance remains `0`, staging is empty, and P2A-W2 remains locked.

`CURRENT`: P2A-W1 Local Product Vertical Live Closure is deterministically
`GREEN`. The shared read-only Application/API boundary now converts historical
nil Runtime model/capability collections to independent canonical empty arrays
while preserving non-empty order; strict Swift remains fail-closed. Focused,
package, full repository, full race, vet, module no-drift/verify, Swift debug,
release, Thread Sanitizer, installer, transaction-fixture, formatting,
authority, credential-negative, diff, and staging gates pass. All 53 accepted
source inputs remain exact. A fresh private Candidate is bound by manifest
SHA-256 `7c0ec8cd75129f0184d345ea39684bc1b1850db56907f1b95ea57f61302a1700`;
its strict-valid arm64 App has non-zero `LC_UUID`
`93E3FC61-0A19-3379-BFDB-8CA7726F59F6`. The new transaction remains locked by
the absent post-Review activation audit and its failure/rollback/no-retry
fixture passes. Fresh independent Implementation Review is the current gate.
No install, bootstrap, App launch, launchd restart, Journal mutation, Provider
or Runtime action occurred; live allowance remains `0`, staging is empty,
P2A-W1 is uncommitted/unaccepted, and P2A-W2 remains locked.

`CURRENT`: fresh independent P2A-W1 Local Product Vertical Live Closure
Implementation Review 1 returned `FAIL` before live on one P1 transaction
binding defect. The frozen contract owns only
`local-product-live-closure-activation-audit.md`, while the transaction
required an unowned `post-review` filename, so no contract-compliant activation
could unlock it. All source, Candidate, causal implementation, focused/full
test, race, vet, module, strict Swift, privacy, transaction-fixture, diff, and
staging checks otherwise passed independently. The transaction now reads the
exact owned activation path and its fixture rejects the obsolete unowned path.
Post-repair deterministic verification and fresh independent Implementation
Re-review are the current gate. No activation file, install, bootstrap, App
launch, service restart, Journal mutation, Provider or Runtime action occurred;
allowance remains `0`, staging is empty, and P2A-W2 remains locked.

`CURRENT`: fresh independent P2A-W1 Local Product Vertical Live Closure
Implementation Re-review 2 returned `PASS` with no findings. It reproduced the
exact activation-path repair, Review 1 byte lineage, source/Candidate inputs,
complete deterministic matrix, production zero-bootstrap lock, and empty
staging. The post-Review activation audit then passed against the exact running
observer, installed product/plist bytes, private modes/owner/provenance,
one-Event SQLite integrity and historical `model_ids:null`, canonical view
version, two-report crash inventory, absent App/run/socket/backups/cache, exact
fresh Candidate, and zero Provider markers. The single replacement live
allowance is now `1`; no bootstrap or App launch has yet occurred. Only the
reviewed transaction and one controlled native-window canary may consume it.
P2A-W2 remains locked.

`CURRENT`: the single P2A-W1 Local Product Vertical Live Closure transaction
consumed its one allowance and returned
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`. The exact Candidate daemon received one
bootstrap but failed before the ready marker, so the native App was never
opened and no Home, Work, Teams, Inbox, System, Refresh, relaunch, or later
lifecycle-restart proof exists. No retry occurred. The exact installed
binaries, wrapper, plist, provenance values, one-Event SQLite state, historical
crash reports, and stable original observer were restored; the private
Candidate is retained and Git staging is empty. A broader-than-intended
read-only diagnostic transiently displayed inherited credential values in tool
output; no value entered repository evidence, Candidate artifacts, Journal,
screenshots, Agent definitions, or Loom-created logs. Credential rotation was
not performed silently.

`CURRENT`: fresh independent P2A-W1 Local Product Vertical Live Closure Result
Review returned `PASS` with no findings. The Reviewer confirmed one Candidate
bootstrap, pre-ready daemon failure, zero native App launches, zero retries,
zero later lifecycle restarts, exact installed-state and SQLite rollback,
stable original observer, preserved historical reports, no durable
credential-value capture, retained private Candidate, and empty staging. This
Review validates the failure evidence, not the product. The only terminal state
is `FAIL - ROLLED_BACK - HUMAN_REQUIRED`; allowance is `0`, P2A-W1 remains
unaccepted and uncommitted, no additional live canary is authorized, and
P2A-W2 remains locked.

`CURRENT`: post-failure bounded diagnosis now localizes the consumed P2A-W1
pre-ready daemon exit to the production switch boundary. The exact retained
Candidate passes direct one-cycle execution, copied resident isolation,
private UDS, private launchd with exit `0`, and a resident-like
`KeepAlive=true` launchd fixture with one PID stable for 12 seconds and a typed
status response. All private labels and roots were removed, and no resident
service, Journal, App, Provider, Runtime, or credential changed. The retained
`daemon failed` stderr is too coarse to distinguish observer, local IPC, or
shutdown without guessing. One `Local Product Launch Failure Closure Repair
Contract` is frozen for fresh independent Contract Review. It keeps the failed
allowance at `0`, owns closed reason codes plus switch ordering and exact-path
preflight in the same existing P2A-W1, grants no production live action before
Implementation Review, and keeps P2A-W2 locked.

`CURRENT`: fresh independent Local Product Launch Failure Closure Contract
Review 1 returned `FAIL` before preflight or implementation. It found two P1
identity gaps: no fresh repaired Candidate/source-lock manifest existed to
bind rebuilt binaries, App UUID/bundle, toolchains, transaction, and fixture;
and the required closed failure-reason record had no exact owned path. The same
single P2A-W1 repair contract now owns canonical
`local-product-launch-failure-repair-source-lock.json`,
`local-product-launch-failure-repair-candidate-manifest.json`, and
`local-product-launch-failure-reason.json`, defines their exact closed content
and privacy/mode rules, limits the retained failed Candidate to preflight, and
requires activation to reproduce the fresh repaired Candidate identity. No
exact-path preflight, RED, product edit, service mutation, or live action
occurred. Allowance remains `0`, P2A-W1 remains
`FAIL - ROLLED_BACK - HUMAN_REQUIRED`, and P2A-W2 remains locked. Fresh
independent Contract Re-review is the current gate.

`CURRENT`: fresh independent Local Product Launch Failure Closure Contract
Re-review 2 returned `PASS` with no P0, P1, or P2 findings. It reproduced the
fresh repaired source-lock and Candidate-manifest boundary, the unique closed
failure-reason path and non-disclosing schema, the retained failed Candidate's
preflight-only restriction, the causal RED requirements, the direct/private
exact-path preflight, original-absence-before-install ordering, one-bootstrap
and no-retry rules, and the acyclic source-lock → Candidate manifest →
transaction → activation-audit identity chain. No installed product, App,
service, Journal, Provider, Runtime, or live state changed. The bounded
exact-production-socket preflight is now the only allowed next action; it
grants no live allowance, P2A-W1 remains unaccepted and uncommitted, and
P2A-W2 remains locked.

`CURRENT`: the first exact-production-socket preflight execution has no product
verdict because its evidence harness asserted a nonexistent nested
`local_product_snapshot`; accepted CLI status embeds and flattens those fields.
The process and private outputs were cleaned before the invalid assertion was
classified, so no PASS or product failure is inferred. The original observer
remains PID `85936`, state `running`, runs `1`; run root, Candidate process,
and native App are absent; the resident Journal retains its exact hash,
integrity `ok`, and one Event; staging is empty. The same single repair
contract is frozen with a bounded harness reopen for at most one replacement
preflight using the correct top-level typed envelope. Fresh independent
Contract Re-review is required first. Live allowance remains `0`, P2A-W1 is
unaccepted and uncommitted, and P2A-W2 remains locked.

`CURRENT`: fresh independent Contract Re-review 3 returned `PASS` with no P0,
P1, or P2 findings on the bounded exact-path evidence-harness reopen. It
confirmed the first execution has no product verdict, reproduced the flattened
status-envelope root cause and post-cleanup state, and permits exactly one
replacement preflight with retained bounded outputs and the correct top-level
typed predicate. That replacement remains a direct/private diagnostic, not a
production bootstrap, live canary, retry, or allowance. Live allowance remains
`0`; P2A-W1 is unaccepted and uncommitted, and P2A-W2 remains locked.

`CURRENT`: the one reviewed replacement exact-production-socket preflight
returned `PASS`. The retained Candidate produced one exact socket, one
Candidate daemon process, a `753`-byte typed top-level status response, exit
`0`, and zero daemon/CLI stderr; the private SQLite remained integrity `ok`
with one Event and no native App launched. Cleanup restored absence of the run
root and left the original observer at the same PID `85936`, state `running`,
runs `1`; resident Journal and both historical report hashes remain exact and
staging is empty. No launchd mutation, live bootstrap, Provider/Runtime
execution, credential access, or allowance occurred. The invalid first
evidence harness remains recorded without reinterpretation and no further
preflight is permitted. Mandatory causal RED is now the current gate; live
allowance remains `0`, P2A-W1 is unaccepted/uncommitted, and P2A-W2 remains
locked.

`CURRENT`: the Local Product Launch Failure Closure repair has a causal
mandatory RED. Focused Go tests prove current code lacks typed
server-before-ready `local_ipc` and observer-after-ready `observer`
classification, collapses observer/local IPC/unknown-close failures to
`daemon failed`, and collapses result encoding to the same text. The
transaction fixture independently proves the governed reason path, atomic
closed reason writer, exact-path preflight boundary, and
original-service-absence-before-install ordering are absent. Production Go and
transaction files remain unchanged from their frozen inputs; no service,
installed product, Journal, Provider/Runtime, credential, staging, or live
allowance changed. Minimal GREEN inside the same P2A-W1 contract is now the
current gate; P2A-W2 remains locked.

`CURRENT`: Local Product Launch Failure Closure is deterministically `GREEN`.
Closed daemon lifecycle reason codes, original-absence-before-install
ordering, exact-path preflight, `KeepAlive=false`, one-bootstrap/no-retry, and
atomic 0600 reason-record fixtures pass. Sequential full Go, race, vet,
module, Swift debug/release/Thread Sanitizer, native App build/install, local
product installer, transaction, format, credential, authority, diff, and
staging gates pass. A disclosed invalid parallel matrix saturated real Pi
metadata fixtures and caused the pre-existing KeepAlive observer to
self-restart to PID `97912`, runs `18`; installed bytes, plist, Journal,
reports, App/run absence, and staging stayed exact, and the service is stable
after load. Fresh source-lock SHA is
`333097f377427eda8ef5c6c0d2a3fc4da47a3075a7f67a93c8637441ad679570`;
fresh Candidate-manifest SHA is
`ca47404f4761a26a6f9bfcc8ce18091832ea59314bd85a736ef2e8e3d2a00a58`;
fresh `loomd` SHA is
`af29fbb9cfa46ac0b97321a3240c9e7fd4048f0f64ce55cfd1b139a1a691d6e6`.
The replacement activation record is absent and the transaction remains
zero-bootstrap fail-closed. Fresh independent Implementation Review, including
an explicit judgment on the disclosed resident continuity drift, is the
current gate. Live allowance remains `0`; P2A-W1 is unaccepted/uncommitted,
and P2A-W2 remains locked.

`CURRENT`: fresh independent Local Product Launch Failure Closure
Implementation Review returned `FAIL` with three P1 findings. The transaction
does not terminate and join the direct exact-path Candidate on its signal
path; the source lock omits most transitive in-repository Go production inputs
and leaves `internal/api/local_product_read.go` unbound; and the disclosed
resident change from PID `85936`, runs `1` to PID `97912`, runs `18` cannot be
silently treated as exact continuity. No live state was read or changed by the
Reviewer. Repair remains inside the same single P2A-W1 contract: add a causal
signal-window fixture and bounded child cleanup, regenerate a complete
production-input source closure and fresh Candidate identity, then reconcile
resident drift through the separate reviewed activation audit. Live allowance
remains `0`; P2A-W1 is unaccepted/uncommitted, and P2A-W2 remains locked.

`CURRENT`: repair RED/GREEN now closes the first two Implementation Review
findings. A signal-window fixture causally failed, then the transaction gained
bounded termination and join of the direct exact-path Candidate and passed
repeatedly. The source lock now binds `201` inputs, including `171`
automatically enumerated in-repository Go production/test inputs and all
native Swift inputs; the Candidate manifest binds the repaired lock. Sequential
Go, race, vet, module, Swift debug/release/Thread Sanitizer, installer, shell,
transaction, diff, staging, and zero-activation gates pass. Those resource
heavy checks caused the unchanged KeepAlive observer to advance again, ending
stable at PID `95853`, runs `36`, while installed bytes, plist, Journal, and
reports remained exact. The same P2A-W1 contract is therefore frozen with a
bounded continuity rebaseline amendment: historical process observations stay
immutable, while a post-Review activation audit must freeze a new stable
PID/run pair and the transaction must fail closed if it no longer matches.
Fresh independent Contract Re-review is the current gate. Live allowance
remains `0`; P2A-W1 is unaccepted/uncommitted, and P2A-W2 remains locked.

`CURRENT`: fresh independent Contract Re-review 4 returned `PASS` with no P0,
P1, or P2 findings, and exact Status-only hash-drift closure. A second causal
RED then proved the transaction did not bind the audited resident PID/runs.
GREEN now strictly parses exactly one closed decimal `Resident PID` and
`Resident Runs`, matches both during preflight and immediately before rollback
arm/bootout, and otherwise fails with zero bootstrap and zero mutation. The
signal cleanup fixture, `201`-input source closure, Candidate identity,
sequential full matrix, transaction fixture, and zero-activation gate pass.
Fresh independent Implementation Re-review is the current gate; it must judge
both repaired P1s and the reviewed continuity rebaseline. Live allowance
remains `0`; P2A-W1 is unaccepted/uncommitted, and P2A-W2 remains locked.

`CURRENT`: fresh independent Implementation Re-review returned `PASS` with no
P0, P1, or P2 findings, and the separate activation-record audit passed. The
single replacement transaction then failed before `READY` with exact result
`rollback_incomplete`, `initial_bootstrap_calls=1`, `consumed=1`,
`rollback_count=1`, `restart_count=0`. No native App or UI action occurred and
no restart phase was reached. The original observer was initially
`spawn scheduled` beyond the rollback deadline, then recovered without another
transaction or manual lifecycle command and remained stable at PID `43503`,
runs `32` over sixteen seconds. Original installed bytes, plist, Journal
hash/integrity/one Event, reports, absent App/run/replacement paths, and empty
staging are exact. The allowance is now `0` and the result is governed
`FAIL - ROLLBACK_INCOMPLETE - ORIGINAL SERVICE LATE-RECOVERED - NO RETRY`.
P2A-W1 remains unaccepted/uncommitted, P2A-W2 remains locked, and a new
reviewed authority decision is required before any further live action.

`CURRENT`: fresh independent Result Review returned `PASS` with no
evidence-quality findings. It confirms exactly one consumed invocation,
`rollback_incomplete`, no `READY`/App/UI/restart/retry, allowance `0`, and
stable late recovery with exact immutable original state. This PASS reviews
the failed evidence only; it does not convert the canary to success. The
mandatory terminal state is `FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED -
NO RETRY`. Read-only diagnosis may continue, but no further live transaction
is authorized; P2A-W1 remains unaccepted/uncommitted and P2A-W2 remains locked.

`CURRENT`: read-only source diagnosis confirms the exact failed predicate
cannot be recovered. After the sole Candidate bootstrap, every socket,
process, install, App, status, Journal, report, and staging assertion before
`READY` is a bare `set -e` check with no closed phase attribution; reason
writing can itself fail without preserving the primary phase. The rollback's
20-second original-service wait also ended before the observed late recovery.
The next safe boundary is one non-live extension of the same P2A-W1 contract
covering every pre-READY phase, reason-writer failure, and delayed recovery.
It cannot infer the missing predicate, create a new WorkItem, restore
allowance, or authorize live execution.

`CURRENT`: Contract Re-review 5 and exact Status-only hash closure returned
`PASS` with no P0/P1/P2 findings. Causal RED proved complete phase attribution
and 60-second recovery settling were absent. Non-live GREEN now routes all
twelve closed pre-READY phases through one recorder, preserves the primary
phase if reason writing fails, catches an injected uncovered exit as
`unclassified`, and tests 240-probe recovery using a fast sequence. All phase,
reason-target, signal, success, source-lock, shell, diff, staging, and
zero-activation fixtures pass. Transaction SHA is
`99b448a8284fc58aadb0e57a5d66969cc84f1e6cd2360c475298f86d2150212b`;
fixture SHA is
`8066a20d0b31fc67a5ddc1fae029f2baf3787baf8b8167b7c4da2302508cfe4d`.
Fresh independent Implementation Re-review is the current gate. Allowance
remains `0`; no live action, P2A-W1 commit, or P2A-W2 work is authorized.

`CURRENT`: fresh independent Implementation Re-review 6 returned `PASS` with
no P0, P1, or P2 findings. It reproduced the ten-key/twelve-phase closure,
reason-writer failure preservation, nonrecursive `unclassified` trap, shared
240-probe recovery loop, success/no-retry behavior, shell syntax, and full
transaction fixture. The deterministic non-live repair is reviewed complete,
but the prior replacement canary remains consumed
`FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`. Allowance remains
`0`; a new explicit post-failure reviewed authority record is required before
any future live canary. P2A-W1 remains unaccepted/uncommitted and P2A-W2
remains locked.

`CURRENT`: the user-authorized P2A-W1 `Authoritative Collection Wire
Normalization Reopen` is frozen and passed fresh independent Contract Review
with no P0/P1/P2 findings. Genuine RED proved the shared snapshot clone still
serialized nil top-level `teams`, `runs`, `evidence`, and `attention` as JSON
`null`. The minimal Go repair now uses one fresh-slice helper for every
snapshot collection, including Runtime `model_ids` and
`observed_capabilities`; timeline `records`, `board.nodes`, and `attention`
remain canonical arrays. A real historical `model_ids:null` Journal fixture
now crosses Projection, `LocalProductReadService`, the production handler,
real Go UDS server, and the compiled strict Swift client successfully. Focused,
package, package-race, repository, repository-race, vet, offline module,
Swift-test/build, format, diff, source-lock, and empty-staging gates pass.
Swift decoder/probe bytes remain exact accepted source-lock inputs. Fresh
independent Implementation Review returned `PASS` with no P0/P1/P2 findings
and independently reproduced both the API collection test and the real
Go-UDS-to-strict-Swift fixture. This non-live reopen is closed. It is still
P2A-W1, creates no W4, grants no live canary, keeps live allowance `0`, and
leaves the prior failed live result, P2A-W1 acceptance, and P2A-W2 lock
unchanged.

`CURRENT`: the Product Owner then explicitly authorized the complete reviewed
P2A-W1 Candidate for one local atomic commit. That authorization preserves the
governed live result as
`FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`; it does not reinterpret
the failed canary, grant another live allowance, or claim live delivery.
P2A-W1 product source, tests, reviewed amendments, and evidence may now be
committed as one lineage while all pre-existing contract-excluded worktree
changes remain untouched. Once that commit exists, the next Phase 2A governance
boundary is the P2A-W2 child-contract freeze. No P2A-W4 exists.

`CURRENT`: the reviewed P2A-W1 Candidate was committed atomically at
`7c1c469c46c97eac0cab39a756b2d59867a49563` with excluded pre-existing
worktree changes left unstaged. Its governed live result remains
`FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`, allowance `0`; the
commit is a deterministic product checkpoint and not a successful live-delivery
claim.

`TARGET`: the only next Phase 2A WorkItem is now the single vertical
`P2A-W2 Team Builder and Provider Onboarding`. Its Candidate contract is at
`.loom-evidence/phase2a/P2A-W2/contract.md` and awaits fresh independent
Contract Review. It composes the accepted Team Draft/TeamDefinition domain,
adds only the minimum Journal-backed saved-Team and non-secret Provider metadata
authority, and uses a macOS Keychain-backed Credential Broker plus a read-only
Codex `login status` observer. It prohibits the previously chat-pasted MiniMax
secret, creates no TeamInstance/Run/Grant/Evidence/dispatch fact, grants no live
action before Implementation Review, keeps P2A-W3 locked, and creates no W4.

`CURRENT`: fresh independent P2A-W2 Contract Review returned `PASS` with no P0,
P1, or P2 findings. The single vertical contract is now `FROZEN`; mandatory RED
is the current gate. The PASS grants no product implementation before RED, no
installed Keychain mutation, no real Codex process, no MiniMax/network request,
no resident daemon or installed-app action, no live canary, and no P2A-W3 work.

`CURRENT`: P2A-W2 mandatory RED is preserved. Go focused tests failed only on
the absent frozen Credential Broker, Codex/MiniMax Provider, setup StateWriter,
Projection, Application/API, Bubble Tea, and daemon-handler symbols. Swift
compiled the existing targets and failed only on the absent strict
`LocalProductSetupWire`. RED used no network, Keychain, Codex process, installed
app, resident daemon, Runtime/model, user SQLite, live action, staging, or prior
chat-pasted secret. Minimal W2 implementation inside the exact owned files is
now the current gate.

`CURRENT`: the P2A-W2 deterministic Candidate now closes the frozen
pre-execution journey through the shared Application Service and daemon IPC:
blank/saved/template Candidate Builder, exact binding and compatibility
preview, explicit TeamDefinition-only confirmation, archive/restore CAS,
rebuildable saved-Team and non-secret Provider projections, Codex native-auth
status observation, macOS Keychain Credential Broker, bounded non-generative
MiniMax verification, and both Bubble Tea and strict native Swift clients.
Provider-only setup remains available when no compatible Runtime can build a
Team. The clients expose saved/template selection and archive/restore without
asking users for internal IDs or stream heads.

`CURRENT`: final deterministic verification passed focused and complete Go
tests, complete repository race tests, vet, format/diff/module checks, cgo and
unsupported-Keychain gates, strict Go-UDS/Swift fixtures, Swift debug tests,
release build, and thread-sanitizer tests. A new real-process regression exposed
and repaired an `os/exec` output-bound bypass caused by embedded
`bytes.Buffer.ReadFrom`; bounded output, process-group cancellation, and
post-run executable-identity replacement now have repeated deterministic
coverage. No installed Keychain item, real Codex process, network/Provider
request, resident daemon/app, Runtime, live canary, staging, or commit action
occurred. Fresh independent P2A-W2 Implementation Review is the current gate;
P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: fresh independent P2A-W2 Implementation Review returned `PASS` with
no P0, P1, or P2 findings. The Reviewer checked the frozen contract SHA,
mandatory RED, deterministic evidence, W2 source/tests, tracked diff against
`7c1c469c46c97eac0cab39a756b2d59867a49563`, and untracked W2 files. It
explicitly modified no files, staged or committed nothing, and ran no process,
network, Keychain, Codex, or live action. A separately frozen W2 live manifest
is now the current gate. No live action has yet occurred; P2A-W3 remains locked
and no P2A-W4 exists.

`CURRENT`: the single P2A-W2 live gate is now frozen by the reviewed
`Live Gate and Deterministic Checkpoint Commit Amendment`. Its exact product
source/test Merkle is
`bd11f85b1b46cfd4927131484f77e8dff36afd9b5793d18ef061aec6b72a4dac`.
Fresh independent Contract Review returned `PASS` with no P0, P1, or P2
findings and independently reproduced that Merkle. The previously chat-pasted
MiniMax secret remains prohibited; no fresh product-entered credential exists,
so the controlled live result remains `HUMAN_REQUIRED`, allowance is
unconsumed, and no Codex process, daemon, native app, Keychain, Provider,
network, Journal, or other live action occurred. The Product Owner's explicit
`全部授权，并进行提交` authorization permits one atomic deterministic W2
checkpoint commit after final reverification. That commit cannot claim W2 live
delivery or acceptance, cannot unlock P2A-W3, and creates no P2A-W4.

`CURRENT`: final pre-commit reverification for the reviewed P2A-W2
deterministic checkpoint is `PASS`. Complete Go tests, complete repository race
tests, vet, isolated-cache module tidy/verify, cgo-disabled and fixed-Keychain
gates, source-lock/Merkle, format/diff, secret-negative, Swift debug tests,
release build, and thread-sanitizer tests all pass. The Swift test matrices each
executed 27 XCTest cases with one expected visual-audit-only skip plus all three
Swift Testing cases. No live action occurred. The current gate is the exact
atomic checkpoint commit; W2 live/acceptance remains `HUMAN_REQUIRED` and
P2A-W3 remains locked.

`CURRENT`: the reviewed P2A-W2 deterministic checkpoint was committed locally
at `48eb18b7c72d06a6149e0220023aa0d6d4bf1677`; all contract-excluded user
changes remain unstaged. The user then confirmed that a genuinely fresh
MiniMax credential is ready for direct entry into the native product. Exact
pre-live inspection stopped before every mutation because the frozen
`--codex-executable` named the npm symlink/JavaScript wrapper while production
`SystemCodexStatusRunner` accepts only a regular executable and runs with an
empty environment. The same official Codex installation contains its regular
arm64 native binary with SHA-256
`29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a`.
One same-W2 `Live Gate Codex Native Binary Correction` is now awaiting fresh
independent Contract Review. The accepted resident Runtime observer remains
running without a product socket and must remain untouched. The attempt root
and default product socket remain absent; no Codex process, daemon, native app,
Keychain, Provider, network, Journal, staging, or canary action occurred, so
allowance remains unconsumed and P2A-W3 remains locked.

`CURRENT`: fresh independent Contract Review of the same-W2 Codex native binary
correction returned `PASS` with no P0, P1, or P2 findings. The corrected
official native arm64 executable path may now enter exact preflight without
relaxing the production regular-file, empty-environment, exact-argument or
before/launch/after identity checks. The Reviewer modified and executed
nothing. No canary action has occurred; allowance remains unconsumed, the
resident no-socket Runtime observer remains untouched, and P2A-W3 remains
locked.

`CURRENT`: exact pre-consumption startup exposed one frozen Runtime input
defect: production Pi metadata intentionally constructs its child `PATH` only
from ordered `--runtime-dir` arguments, while the original W2 manifest named
only Pi's `.bin` directory and could not resolve the reviewed Node interpreter.
The first startup exited `daemon failed: observer` with zero Journal Events,
no socket, no app, no credential, no Keychain or Provider action, so allowance
remained unconsumed. The same-W2 `Pi Node Search Path Correction` locked the
reviewed Node arm64 binary SHA-256
`1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8`
as the second ordered search directory. Fresh independent Contract Review and
an exact Event-boundary wording re-review both returned `PASS`; no product
source or authority boundary changed.

`CURRENT`: the one corrected daemon startup passed with the exact private
`0600` socket, `0600` SQLite and only one expected isolated
`RuntimeInstanceDiscovered` Event. The frozen intermediate SwiftPM executable
then proved non-addressable because it has no `CFBundleIdentifier`; it was
stopped before UI interaction or canary consumption. The same-W2 reviewed
`Native Bundle Materialization Correction` reused the already accepted W1
`scripts/build-loom-local-app.sh` boundary to materialize one fresh
`com.earendilworks.loom.local` arm64 `Loom.app`. Its executable SHA-256 is
`1f8408a396fbf31c4e15c3561b79ef4df053222b37874ccf187f7e547a07f5e0`;
strict signature, permissions and no-symlink verification passed. Computer Use
then independently addressed the native Team Builder without restarting the
daemon.

`CURRENT`: the P2A-W2 native journey reached the required user-only MiniMax
SecureField hand-off. Across three consecutive goal turns the field remained
empty, `Store securely` remained disabled, MiniMax remained `Unconfigured`,
and the user did not perform direct credential entry/submission. No Provider
test, Keychain mutation, credential Event, TeamDefinition, TeamInstance,
AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or generation exists;
the only authoritative fact is the expected isolated Runtime discovery.
The addressable app and isolated daemon were cleanly stopped, the socket is
absent, the SQLite/attempt root are retained for read-only Result-Evidence
Review, and the resident accepted Runtime observer remains outside the attempt.
The live allowance is unconsumed, P2A-W2 remains `HUMAN_REQUIRED` and not
accepted, P2A-W3 remains locked, and P2A-W4 does not exist.

`CURRENT`: fresh independent read-only Result-Evidence Review of the closed
pre-consumption W2 attempt returned `PASS` with no P0, P1, or P2 findings. The
Reviewer reproduced the exact six target hashes, attempt/database/bundle modes,
absent socket and attempt processes, preserved resident no-socket observer, and
the SQLite Event set of exactly one Runtime discovery and no credential, team
or execution authority. It modified and executed no product surface and used
no GUI, Keychain, environment, chat value or network. This accepts the
`HUMAN_REQUIRED (allowance unconsumed)` status evidence only; it does not accept
P2A-W2 or unlock P2A-W3.

`CURRENT`: after Result-Evidence Review `PASS`, the closed secret-negative W2
attempt root was moved to the user's Trash as a recoverable deletion and the
initially absent, now-empty default run directory was removed. The exact
attempt root, run directory and product socket are absent from Application
Support; no attempt process remains. The resident accepted Runtime observer
and its state remain untouched. This cleanup changes no acceptance state:
P2A-W2 is still `HUMAN_REQUIRED` with allowance unconsumed, P2A-W3 is locked,
and P2A-W4 does not exist.

`CURRENT`: Product Owner request `帮我打开对应的窗口` froze and received fresh
independent Contract Review `PASS` for one new
`p2a-w2-live-20260730-002` resumption lineage. Exact W2 binaries, Pi, Node,
Codex native target, addressable arm64 bundle, permissions, signature,
no-symlink, source identity, fresh paths and zero-Event preflight passed. The
isolated daemon wrote only the expected Runtime discovery and Computer Use
opened Team Builder at the MiniMax SecureField without the Controller reading
or entering a credential.

`CURRENT`: the user personally configured a credential after explicitly
stating an intent to reuse the value already pasted into chat. The reviewed
resumption contract prohibits every chat-pasted credential, so the result is
invalid for W2 acceptance. The isolated Journal contains one
`ProviderCredentialConfigured` and two `ProviderCredentialVerified` facts; the
Controller did not activate `Test`, and the external request count is not
asserted from Event facts alone. The user refused the requested product-level
`Revoke` action. The Controller performed no further Provider or Team Builder
action, stopped the app and daemon, and left the exact attempt SQLite for fresh
Result-Evidence Review. No Team or execution authority exists. Allowance is
consumed, P2A-W2 is `INVALID / HUMAN_REQUIRED` and not accepted, P2A-W3 remains
locked, and P2A-W4 does not exist.

`CURRENT`: fresh independent Result-Evidence Review of invalid W2 attempt 002
returned `PASS` with no P0, P1, or P2 findings. The Reviewer reproduced exactly
four Events, the absent revocation/Team/execution Event set, `0700` attempt
root, regular `0600` SQLite, absent socket and attempt processes, and preserved
resident observer. It confirmed the chat-pasted-credential contract violation,
consumed allowance, invalid `HUMAN_REQUIRED` result, W3 lock and no-W4 state.
It inspected no payload, Keychain, environment or chat secret and performed no
mutation. Cleanup remains blocked by the user's explicit refusal to revoke the
configured credential.

`CURRENT`: the Product Owner then explicitly waived that prior secret-source
stop condition for diagnostic testing and directed the Controller to run the
normal test matrix. Fresh independent read-only review of the bounded diagnostic
continuation returned `PASS`. The isolated attempt daemon and native app were
started once; the product used the existing credential only through the Broker
and OS Secret Store. One MiniMax `Test` completed and appended one additional
`ProviderCredentialVerified` fact. The native setup snapshot truthfully showed
MiniMax `verified`, Pi `online` with capacity `1`, but `model_ids=[]` and
`role_options=[]`; blank Team Builder therefore returned `incompatible` and
created no Team or execution authority. Codex 0.144.1 is logged in, but its
successful `login status` line is emitted on stderr while the W2 observer
accepts it only from stdout, producing `unsupported/unknown_output`. These are
two live compatibility defects, not deterministic-test failures.

`CURRENT`: normal sequential deterministic verification now passes complete Go
tests, complete Go race tests, vet, module tidy/verify, format/diff checks,
Swift debug tests, release build, and thread-sanitizer tests. An initial
parallel Controller invocation caused one five-second Pi fixture process
failure; the isolated test passed ten consecutive runs and both sequential
complete Go matrices passed. The diagnostic app and daemon are stopped, the
product socket is absent, and the separate resident observer remains running.
The owner waiver completes testing but does not reinterpret the historical
secret-negative result or claim that the incompatible live Team Builder journey
passed. P2A-W2 remains unaccepted, P2A-W3 remains locked, and no P2A-W4 exists.

`CURRENT`: the user-directed same-W2 Provider Connection and Delegated OAuth
Amendment now has deterministic implementation and fresh independent Repair
Review `PASS`. The native Team Builder uses an App-style Provider connection
directory: Codex `Connect` delegates only to the identity-bound official
`codex login` process and observes its closed status; MiniMax `Connect` or
`Manage` opens the existing Keychain-backed credential sheet. Exact
`codex_connect` Go IPC to strict Swift decoding, bounded cancellation/polling,
light/dark/accessibility rendering, and a native visual fixture are covered.
Complete sequential Go, race, vet, module, format/diff, Swift debug, release
and thread-sanitizer matrices passed.

`CURRENT`: the initial independent implementation review reported one P1 based
on an incorrect assumption that closing a nil setup API would panic. The API's
nil-receiver guard and a new deterministic daemon-shutdown regression proved
fail-closed behavior and continued observer cleanup; fresh read-only Repair
Review returned `PASS` with no P0, P1, or P2. A separate reviewer invocation
that created an over-broad local commit was rejected as review evidence and its
unrelated paths were removed from the commit while their working-tree changes
were preserved. No real OAuth/browser, Provider network, Keychain, resident
daemon, Team, Run, or live-canary action occurred. This amendment does not
reinterpret the historical invalid/HUMAN_REQUIRED W2 live result, accept W2,
unlock P2A-W3, or create P2A-W4.

`CURRENT`: the same-W2 `Isolated Pi Catalog and Live Closure Amendment` froze
after fresh independent Contract Re-review `PASS`. Its deterministic Candidate
now binds the exact accepted local llama-server/GGUF identity, writes the
byte-identical accepted `loom-local` catalog as a private `0600` file inside
each disposable Pi metadata agent directory, and carries the all-or-none
service-manager tuple through Runner, probe factory, Runtime daemon and CLI.
The real parser fixture observes
`loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`; Team Builder continues to use
only authoritative Runtime discovery and does not invent models.

`CURRENT`: mandatory RED and the complete deterministic matrix pass: focused
and five-run focused-race tests, complete Go and repository-race tests, vet,
module tidy/verify, format/diff, Swift debug tests, release build and
thread-sanitizer tests. Catalog binding drift fails before process launch and
the existing no-catalog behavior remains compatible. No installed Pi,
llama-server, Codex, Keychain, MiniMax/network, native app, product daemon,
resident observer, Journal clone or live action occurred. Fresh independent
Implementation Review is the current gate; the one reviewed attempt-003 live
allowance remains unused, P2A-W3 remains locked, and no P2A-W4 exists.

`CURRENT`: a pre-review Controller audit added Repair 1 after proving that the
initial probe factory retained catalog paths rather than the construction-time
file identities. A same-digest file replacement RED was accepted by
`BuildProbe`; the repaired factory now retains the exact bound catalog and each
Runner revalidates that identity at construction and before every metadata
process. The focused regression is green. The interrupted review produced no
verdict. Five-run focused race, serial complete Go and repository-race, vet,
module, format/diff, and fresh Swift debug/release/thread-sanitizer matrices now
pass. One package-parallel Go run under abnormal process-fixture latency
produced three unrelated daemon/local-model timing failures; each failed test
then passed ten serial repetitions and the complete `-p 1` matrices passed.
Fresh independent Implementation Review is again the current gate.

`CURRENT`: fresh independent read-only Implementation Review after Repair 1
returned `PASS` with no P0, P1 or P2 findings. The Reviewer reproduced the
frozen contract SHA and baseline HEAD, confirmed retained file identity,
shared serialization, private atomic materialization, all-or-none CLI input,
authoritative Runtime model flow and truthful transient-test accounting, and
performed no mutation or live action. Only the single frozen
`p2a-w2-live-20260730-003` closure lineage is now unlocked. P2A-W2 is not yet
accepted, P2A-W3 remains locked, and no P2A-W4 exists.

`CURRENT`: the single reviewed `p2a-w2-live-20260730-003` daemon start exited
fail-closed with code `4` and sanitized result `daemon failed: observer`. The
native app was materialized but never launched, so no GUI, Provider request,
Team Builder action or restart journey occurred. Post-failure inspection
reproduced the cloned database's exact
`b0312739...75e22` SHA, `integrity_check=ok`, five inherited Events, zero new
attempt-003 Events, absent post-exit WAL/SHM/socket/attempt processes, an unchanged
attempt-002 source database, and the untouched resident observer PID `44887`.
No replacement start or alternate path was attempted. The frozen allowance is
consumed, the deterministic Candidate remains Implementation-Review `PASS`,
but the live closure is `FAIL — OBSERVER_AFTER_IPC_READY`. P2A-W2 remains
unaccepted, P2A-W3 remains locked, no P2A-W4 exists, and fresh independent
Result-Evidence Review is the current gate.

`CURRENT`: the initial read-only Result-Evidence Review of the failed
attempt-003 lineage reproduced the database/Event/process cleanup facts but its
`PASS` is withdrawn by Repair 1. Product code starts the IPC server, waits for
its `Ready()` boundary and only then starts the observer; an `observer` failure
therefore occurred after IPC readiness. The absent socket observed after exit
proves cleanup, not that the socket was never created. The Reviewer had
reproduced both database hashes and integrity, the exact inherited five-Event
metadata set, zero new Events, private modes, exact Candidate/evidence hashes,
empty isolation directory, absent socket and attempt processes, and the
separate untouched resident PID `44887`, without reading credential payloads
or references or executing any live surface. The result is repaired to
`FAIL — OBSERVER_AFTER_IPC_READY`; fresh independent Repair 1 Result-Evidence
Review is the current gate. The single live allowance remains consumed,
P2A-W2 remains unaccepted, P2A-W3 stays locked, and no P2A-W4 exists.

`CURRENT`: fresh independent read-only Repair 1 Result-Evidence Review returned
`PASS` with no P0, P1 or P2 findings. It directly verified the
Serve→Ready→observer→cleanup chronology and independently reproduced the exact
post-exit hashes, Event metadata, zero-new-Event state, empty isolation,
absent socket/processes and separate resident observer without inspecting
credential payloads/references or executing any live surface. This accepts only
the corrected `FAIL — OBSERVER_AFTER_IPC_READY` evidence. P2A-W2 remains
unaccepted, P2A-W3 stays locked, and no P2A-W4 exists.

`CURRENT`: the same-W2 `Observer Diagnostic and Live Closure Amendment` is
frozen after Contract Review required two P1 and one P2 repairs and fresh
independent Repair 1 Re-review returned `PASS` with no findings. It freezes
allowlisted observer stage/cause codes without raw child output, exact
installed-Pi component manifest/enable/identity/offline boundaries, at most
component execution A plus one conditional post-repair execution B, and one
replacement attempt-004 only after component, deterministic and fresh
Implementation Review GREEN. It changes no Journal/StateWriter/Projection/Team
authority, creates no W4 and keeps P2A-W3 locked. Mandatory deterministic RED
is the current gate; no new Pi/product/live action has occurred.

`CURRENT`: mandatory RED failed only on the missing safe command-stage,
projection-stage, observer-reason and one-validation-per-command symbols. The
minimal deterministic implementation is focused GREEN. Before creating the
locked component manifest, Controller inspection found Contract Repair 2:
Repair 1 incorrectly used the fresh component root as the local-model binding
root even though the frozen llama-server/GGUF are descendants of the existing
`/phase1-live` private root. The manifest now separates component, isolation
and local-model private roots. No installed Pi or live action occurred; fresh
independent Contract Repair 2 Re-review returned `PASS` with no findings.
Component execution A remains locked until its skip-closed test and manifest
preflight are implemented and deterministic checks are GREEN.

`CURRENT`: the locked Pi component test is implemented with strict
duplicate/unknown/trailing JSON rejection, exact enable inputs, canonical
path/hash/mode/uid/size checks, only the frozen Pi leaf symlink, a clean
Factory→Runner→production-parser path, safe reason codes and deferred
postconditions on every post-gate failure. Fresh independent pre-component
Implementation Review Repair 1 returned `PASS` with no P0, P1 or P2 findings.

`CURRENT`: the first enabled component preflight stopped before process
construction at `component_node_identity`. Read-only inspection proved the
frozen `/devtools/node` directory was itself a symlink, contradicting the
contract's sole-Pi-leaf-symlink rule. Contract Repair 3 froze the canonical
`node-v24.16.0-darwin-arm64/bin` directory and executable without changing
hash, size, mode or owner. Fresh independent Contract Repair 3 Re-review
returned `PASS`; the zero-command skip did not consume execution A.

`CURRENT`: locked component execution A is `GREEN`. The real installed Pi
0.82.1 Candidate ran exactly one version request and one offline model-list
request and the production parser observed exactly
`loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`. Isolation, component
Journal/socket/PID artifacts, port 18427, component/llama/attempt processes and
all locked identities passed post-run checks. Execution B is forbidden.

`CURRENT`: the complete Observer Diagnostic Candidate matrix passes focused
tests, ten-run focused race, serial complete Go and repository-race tests, vet,
module tidy/verify, format/diff, secret-negative checks, Swift debug tests,
release build and thread-sanitizer tests. The first complete Go run exposed and
repaired a static `internal/app` import-boundary violation caused by `fmt`;
the repaired code retains the projection sentinel and cause through
`errors.Join`, and the complete matrices were restarted and passed. Fresh
independent Implementation Review returned `PASS` with no P0, P1 or P2
findings after reproducing focused deterministic tests, focused race,
format/diff, serial complete Go, vet and module checks without component enable
environment. With component A `GREEN`, deterministic matrix `GREEN` and
Implementation Review `PASS`, the contract gate for exactly one fresh isolated
replacement live canary `p2a-w2-live-20260730-004` is open. The replacement
canary has not run, no retry or component execution B is authorized, P2A-W2
remains unaccepted until result evidence review, P2A-W3 remains locked and no
P2A-W4 exists.

`CURRENT`: the preceding Implementation Review PASS is withdrawn as governance
evidence because that Reviewer violated its explicit read-only boundary,
appended its own verdict and created Candidate commit
`8c0338cab8ec6982d8b55c7f62e03e7775b7d568`. Controller inspection found that
the commit is otherwise atomic and contains only the frozen W2 owned files, so
it is retained while unrelated working-tree changes remain unstaged. A
different fresh independent read-only Implementation Review is now the gate.
Replacement canary 004 has not run and is locked again; P2A-W2 remains
unaccepted, P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: a different fresh independent read-only Implementation Reviewer
inspected commit `8c0338c` against baseline `bd74c2e`, explicitly did not rely
on the withdrawn self-committed verdict, reproduced the focused deterministic
union with component enable variables absent, and returned `PASS` with no P0,
P1 or P2 findings. It edited, staged and committed nothing and ran no installed
Pi, live daemon/app, network, Keychain, Provider, resident or LaunchAgent
action. Component A GREEN, complete deterministic GREEN and this replacement
Review PASS unlock exactly one controlled replacement canary
`p2a-w2-live-20260730-004`. P2A-W2 remains unaccepted until live
Result-Evidence Review PASS; P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: the single replacement canary
`p2a-w2-live-20260730-004` is consumed with verdict
`FAIL — NATIVE_JOURNEY_INCOMPATIBLE_AND_SHUTDOWN_STALLED`. The one daemon start
appended exactly one model-capable Runtime discovery fact. The exact native app
then narrowly live-proved the repaired Codex 0.144.1 observer boundary: the
locked process passed the production observer and Codex was `Available`.
Together with the prior diagnostic and deterministic stderr-specific tests,
this proves the intended compatibility without retaining child output or
claiming a model request or broader Provider behavior. MiniMax was recovered as
`Verified`, Pi was online, and the same Provider/Runtime state reconstructed
after an app quit/reopen. The single MiniMax `Test` appended no new verification
fact, the one Candidate creation stopped at `Incompatible` before the first
question, and the daemon closed its listener but required exact-PID `SIGKILL`
after bounded `SIGINT`/`SIGTERM` shutdown waits. Final integrity is `ok`; the
six-Event database contains only the inherited five Events plus the one Runtime
discovery, with no TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run,
Grant, Evidence, dispatch or execution fact. The isolation root and product
socket are absent, attempt processes are gone, and the separate resident daemon
was not signalled or reconfigured. The stderr compatibility code remains
deterministic and Implementation-Review `PASS`, but P2A-W2 is unaccepted,
P2A-W3 remains locked, no P2A-W4 exists and no retry or additional single-point
amendment is authorized. Fresh independent read-only Result-Evidence Review is
the current gate.

`CURRENT`: fresh independent read-only Result-Evidence Review returned `PASS`
for the recorded failed outcome, with no P0 or P1 finding. It independently
reproduced the attempt/source database hashes and integrity, exact six-versus-
five Event delta, the sole new Runtime stream fact, zero Team/Run/Grant/Evidence
classes, exact daemon/app hashes and private modes, empty isolation, absent
product socket/lock and absent attempt processes. Its two non-blocking P2
precision notes are retained: the live artifact does not preserve the exact
Codex child-output stream/line count, so the compatibility conclusion remains
limited to production observer acceptance plus native `Available`; and the
result file did not freeze a reproducible resident-daemon pre/post hash tuple.
The Reviewer confirmed a separate resident daemon currently exists and no
attempt-004 residue does. The consumed canary remains
`FAIL — NATIVE_JOURNEY_INCOMPATIBLE_AND_SHUTDOWN_STALLED`; P2A-W2 remains
unaccepted, P2A-W3 locked, no P2A-W4 exists and no retry is authorized.

`CURRENT`: one same-W2 `Vertical Native Journey Closure Repair` is frozen.
Contract Review 1 returned `FAIL` because the owned Swift test directory did
not name the package's real target; it also requested exact deadline values and
unambiguous post-repair binary wording. Repair 1 now names
`apps/macos/Tests/LoomLocalAppTests/`, freezes a 5s MiniMax request, at-most-1s
terminal commit, exact 10s Go/Swift `credential_verify` request and unchanged
5s default request budget, and binds attempt-005 to post-Implementation-Review
binaries. The deterministic fixture remains the attempt-004 six-Event state,
while the eventual native canary must start from the exact five-Event
attempt-002 baseline to prove catalog refresh after observer append. The repair
still closes stale setup, exactly-one Provider fact and joined shutdown as one
vertical boundary, creates no W4, changes no authority and permits no
execution. Fresh independent Contract Repair 1 Re-review is the current gate;
no production code or live action is authorized before it passes.

`CURRENT`: a different fresh independent read-only Contract Repair 1
Re-reviewer returned `PASS` with no P0, P1 or P2 finding. It reproduced the
actual Swift test target, exact deadline hierarchy, conditional IPC/client
ownership, post-Implementation-Review binary binding and the private,
integrity-valid attempt-002 five-Event source hash. It confirmed that the
repair remains one W2 vertical boundary with no W4, authority expansion,
detached lifecycle work or premature live action. Mandatory deterministic RED
is now the gate; attempt-005 remains locked.

`CURRENT`: the same-W2 Vertical Native Journey Closure mandatory RED reproduced
all four connected product gaps without live action: construction-time Runtime
catalog staleness against the exact six-Event attempt-004 shape, caller
cancellation erasing an already observed Provider result, the equal five-second
Go IPC deadline, and Keychain queries lacking an explicit non-interactive
policy. Secret Store failure before Provider observation continued to append
zero facts.

`CURRENT`: the repaired deterministic Candidate is complete and GREEN.
`SetupSnapshot` and `StartBuilder` now derive a canonical Runtime catalog after
Projection rebuild from the current immutable `GlobalReadView`; each Builder
session freezes its exact catalog/domain catalog/view/binding and rejects later
catalog or view drift without rebinding. A real MiniMax observation gets exactly
one cancellation-independent terminal metadata append bounded to one second,
while only `credential_verify` receives ten-second Go and Swift request
deadlines and all other IPC remains five seconds. Keychain access fails closed
without authentication UI. Product shutdown now joins local IPC handlers,
Runtime observer, setup/native-auth owner and database in order, retains
internal stage identity, removes the socket, and closes each owner exactly
once.

`CURRENT`: focused tests, twenty-run focused race, serial complete Go and
repository-race tests, vet, module tidy/verify, format/diff, secret-negative
scan, Swift debug tests, release build and thread-sanitizer tests all pass.
Swift debug and thread-sanitizer each executed 33 XCTest cases with zero
failures and one intentional visual-export skip; the strict Swift Testing wire
suite passed four tests. No installed Pi, Codex, Provider network, real
Keychain item, native app, resident daemon, LaunchAgent or live canary ran.
Fresh independent Implementation Review is the current gate. Attempt-005
remains locked, P2A-W2 remains unaccepted, P2A-W3 remains locked and no P2A-W4
exists.

`CURRENT`: fresh independent read-only Implementation Review returned `PASS`
with no P0, P1 or P2 finding. The Reviewer reproduced the focused catalog,
credential, Keychain, IPC, native presentation and joined-shutdown proofs,
verified exact staged ownership and secret-negative surfaces, and made no edit,
stage, commit, live, Keychain, Provider, Pi, Codex or daemon action. Complete
deterministic GREEN plus this Review unlock exactly one controlled native-window
lineage `p2a-w2-live-20260730-005` using exact post-Review Candidate binaries.
No retry or second canary is authorized. P2A-W2 remains unaccepted until fresh
Result-Evidence Review `PASS`; P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: the only `p2a-w2-live-20260730-005` canary is consumed with verdict
`FAIL — PROVIDER_FACT_MISSING_AND_SHUTDOWN_STALLED`. The post-Review Candidate
appended one model-capable Runtime discovery, exposed compatible Main/SubAgent
roles, saved exactly one TeamDefinition through the ordinary bounded Builder,
and reconstructed Codex `Available`, MiniMax `Verified`, the Runtime and saved
team after app restart. The single MiniMax `Test` appended no new terminal
verification fact. The attempt daemon then remained alive after one normal
interrupt plus a 35-second wait and one exact-PID termination plus a ten-second
wait; cleanup required exact-PID forced termination. Final SQLite integrity is
`ok`; the seven Events are the inherited five plus only one Runtime discovery
and one TeamDefinition save, with no TeamInstance, AgentInstance, WorkItem, Run,
Grant, Evidence, dispatch or execution fact. App/attempt/Pi/llama processes are
absent, isolation is empty, the unowned product socket is removed, and the
immediate post-cleanup observation still found the separate resident daemon at
preflight PID `66336` with the same executable hash and arguments. Every
Controller cleanup signal named only attempt PID `80523`.
P2A-W2 remains unaccepted, P2A-W3 remains locked, no P2A-W4 exists and no retry
or second canary is authorized. Fresh independent read-only Result-Evidence
Review is the current gate.

`CURRENT`: the first attempt-005 Result-Evidence Review returned
`FAIL — REPAIR REQUIRED` with one P1 evidence-precision finding. During that
later read-only check, preflight/immediate-post PID `66336` was absent and the
same launchd-managed resident lineage was running under another PID with the
same executable hash and configured arguments. A subsequent Controller
read-only snapshot showed repeated service-manager restarts and last exit code
`4`. The cause is not established here, so the result no longer claims
resident PID continuity beyond the immediate post-cleanup observation. The
review independently reproduced every attempt/source hash, exact five-to-seven
Event delta, missing Provider fact, zero execution facts and cleanup condition;
the product verdict remains FAIL. Fresh independent read-only Result-Evidence
Re-review is the current gate; P2A-W2 is unaccepted, P2A-W3 locked, no P2A-W4
exists and no retry is authorized.

`CURRENT`: fresh independent read-only attempt-005 Result-Evidence Re-review
returned `PASS` with no P0, P1 or P2 finding. It reproduced the exact private
source/final hashes and integrity, one Runtime plus one TeamDefinition delta,
unchanged Provider verification count, zero execution facts, exact Candidate
binaries and complete attempt cleanup. It confirmed that the repaired evidence
separates the immediate PID `66336` observation from later launchd-managed
resident process churn and makes no unsupported continuity claim. The consumed
canary remains `FAIL — PROVIDER_FACT_MISSING_AND_SHUTDOWN_STALLED`; P2A-W2 is
unaccepted, P2A-W3 remains locked, no P2A-W4 exists and no retry or second
canary is authorized.

`CURRENT`: read-only post-result diagnosis identifies a proven unbounded owner
consistent with both attempt-005 failures. `credential_verify` synchronously
enters Keychain `SecItemCopyMatching` after only a pre-call context check;
Provider observation is bounded to five seconds and terminal commit to one
second, while local IPC correctly cancels then joins every handler. Attempt-005
did not retain a goroutine dump, so the exact stalled live frame is not claimed.
The owner defect independently prevents proof of bounded verification and
shutdown. Increasing IPC timeouts would not close it.

`CURRENT`: one same-W2 `Credential Transaction and Joined Shutdown Closure`
amendment is frozen for Contract Review. It requires short-lived process-owned
cancellable Keychain Put/Read/Delete through strict anonymous pipes, no secret
in argv/environment/log/evidence, exactly-one terminal Provider fact after an
observation, and bounded joined IPC/daemon shutdown. It creates no W4, changes
no Journal/StateWriter/Projection/Team/Runtime/execution authority and keeps
P2A-W3 locked. No implementation, real Keychain, Provider network, installed
Runtime, native app, daemon signal or replacement canary is authorized before
fresh independent Contract Review `PASS`.

`CURRENT`: Contract Review 1 returned `FAIL` with two P1 and two P2 findings:
helper activation did not authenticate the exact daemon parent, lost-response
retry lacked a stable pre-Provider dedupe identity, helper deadlines were not
exact, and diagnosis wording overstated live causality. Contract Repair 1 now
requires exact normal product-daemon parent executable/start/argv identity,
kernel product-socket peer PID and private inherited descriptors; freezes a
two-second total Keychain helper budget; and adds an exact `expected+1`
terminal Projection precheck that returns an already committed result with zero
Keychain, Provider or append work. It also records only a proven unbounded
owner consistent with the failure. Fresh independent Contract Repair 1
Re-review is the current gate; implementation and live remain locked.

`CURRENT`: fresh independent Contract Repair 1 Re-review returned `PASS` with
no P0, P1 or P2 finding. It confirmed exact normal product-daemon parent
authentication before Keychain access, the product-daemon-only
`expected+1` terminal recovery, the two/five/one/ten/five-second deadline
hierarchy, causal precision and unchanged authority boundaries. Mandatory RED
and deterministic implementation are unlocked. Real Keychain, Provider
network, installed Runtime, native app, daemon signal and replacement lineage
`p2a-w2-live-20260730-006` remain locked until complete GREEN and fresh
Implementation Review `PASS`.

`CURRENT`: the same-W2 Credential Transaction and Joined Shutdown Closure
Candidate is deterministic GREEN. Production Keychain Put, Read and Delete now
run as one-operation short-lived copies of the exact daemon executable with an
empty environment, strict bounded inherited-pipe protocol and an exact
two-second total parent budget. Before parsing an operation, the helper proves
fixed anonymous descriptor numbers, the live parent start identity, matching
executable device/inode/owner/mode/SHA-256, normal daemon arguments, and the
private product socket's kernel peer PID. Cancellation kills and waits for the
exact child. Production verify now rebuilds the authoritative Provider view:
the exact current revision performs one Broker observation, exact same-
reference `expected+1` terminal verified/rejected returns the committed result
without Keychain/Provider/append work, and every other stale shape fails before
those boundaries. Existing Security.framework item attributes,
Journal/StateWriter authority, external IPC/Swift schemas and five/ten/five/one
second surrounding budgets are unchanged.

`CURRENT`: focused deterministic and repeated race tests, serial complete Go
and repository-race tests, vet, module tidy/verify, format/diff,
secret-negative checks, Swift debug tests, release build and thread-sanitizer
tests pass. Swift debug and thread-sanitizer each executed 33 XCTest cases with
zero failures and one intentional preview-export skip; the strict Swift
Testing wire suite passed four cases. Tests used only fake stores and local
fixture processes; no real Keychain, Provider, network, installed Runtime,
native app, resident daemon or live canary ran. Fresh independent
Implementation Review is the current gate. Replacement lineage
`p2a-w2-live-20260730-006` remains locked, P2A-W2 remains unaccepted, P2A-W3
remains locked and no P2A-W4 exists.

`CURRENT`: fresh independent Implementation Review 1 returned
`FAIL — REPAIR REQUIRED` with one P1 and one evidence-hygiene P2. The P1 found
that a production process-Keychain construction error still returned `nil,
nil`, allowing the daemon to run with setup disabled. Repair 1 first reproduced
that exact branch as RED, then made setup construction return a closed
credential-boundary error; ten-run focused and race repetitions pass. The P2
identified a stale historical status header, now corrected to record the
already completed Contract Repair 1 Re-review PASS without changing any
behavioral clause. Fresh independent Implementation Repair 1 Re-review is the
current gate. Replacement lineage `p2a-w2-live-20260730-006` remains locked,
P2A-W2 remains unaccepted, P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: fresh independent read-only Implementation Repair 1 Re-review
returned `PASS` with no P0, P1 or P2 finding. It reproduced the targeted
credential/helper/product-daemon suite, confirmed the production construction
failure is closed, verified two concurrent same-revision requests reach the
Provider delegate only once, and rechecked empty environment, fixed anonymous
FD 3/4, parent/start/executable/socket-peer attestation and exact-child
kill-and-wait. It confirmed the amendment change is status-header only and made
no edit, stage, commit, live, Keychain, Provider, network or daemon action.
Complete deterministic GREEN plus this Review unlock the atomic Candidate
commit. Replacement lineage `p2a-w2-live-20260730-006` remains locked until
that commit and exact preflight; P2A-W2 remains unaccepted, P2A-W3 remains
locked and no P2A-W4 exists.

`CURRENT`: Candidate commit `47d5f56` was materialized for the only replacement
lineage `p2a-w2-live-20260730-006`, but the one permitted daemon start exited
immediately with code `4` and exact safe stderr
`daemon failed: local_ipc`. The Controller proved the default product socket
absent before start but failed to include its sibling
`/Users/lune/Library/Application Support/Loom/run/loomd.sock.lock` in that
assertion. Post-failure evidence proves the private zero-byte lock predates the
fresh attempt root by more than one hour and has no current owner. The strict
IPC server refused it before listener readiness, so no native app, Computer
Use, Runtime/Codex process, Keychain, Provider request, Team Builder action or
SIGINT occurred. The attempt database remains byte-identical to the frozen
five-Event source with integrity `ok`; product socket and attempt processes are
absent, isolation is empty, the separate resident LaunchAgent was untouched,
and secret-negative checks pass. The allowance is consumed without retry under
the frozen stop rule. P2A-W2 is `HUMAN_REQUIRED` and not accepted, P2A-W3
remains locked, no P2A-W4 exists, and fresh independent read-only
Result-Evidence Review is the current gate.

`CURRENT`: fresh independent read-only Result-Evidence Review returned `PASS`
for the recorded attempt-006 failed outcome, with no P0, P1 or P2 finding. It
reproduced the byte-identical five-Event databases, artifact identities,
zero-byte stdout, exact 25-byte safe stderr, absent socket and attempt
processes, empty isolation, unchanged resident configuration and
secret-negative result. It independently proved the unowned private product
lock predates the attempt root by 3,938 seconds and confirmed the exclusive
lock-before-listener code path. Its only non-blocking note is that no separate
exit-code sidecar was retained; code `4` is corroborated by the Controller
result and exact Candidate mapping. Under the frozen no-retry stop rule,
P2A-W2 is now `HUMAN_REQUIRED` and not accepted, P2A-W3 remains locked, no
P2A-W4 exists, and no further single-point amendment or replacement canary is
authorized.

`CURRENT`: the Product Owner explicitly reopened the complete P2A-W2 Exit
Contract and authorized one `Phase 2A Interaction Continuity Contract`. The
single frozen Candidate is
`.loom-evidence/phase2a/P2A-W2/interaction-continuity-exit-reopen-contract.md`.
It replaces the dashboard-first `Home/Work/Teams/Inbox/System` journey with a
task-first conversation workspace: recent tasks/history on the left, the
bounded Team Builder and inline status in the center, and contextual
Team/Context/Changes/Evidence inspection on the right. The Bubble Tea client
adopts the same task-first order without becoming the native app transport.
W2 remains pre-execution and may save only one TeamDefinition through the
existing authority.

The same complete contract adds one fail-closed product socket/lock ownership
transaction. It permits reclaim only after an exact private lock descriptor,
non-blocking kernel advisory lock, stable file identity and bounded sibling
socket liveness checks; live, foreign, ambiguous or replacement paths remain
untouched. This is not a standalone socket Amendment.

The contract permits no W2a/W2b, no P2A-W4 and no later single-point Amendment.
P2A-W3 remains locked. Mandatory RED, implementation, deterministic and visual
GREEN, fresh independent Implementation Review, one atomic Candidate commit
and exact preflight must all pass before the only vertical lineage
`p2a-w2-live-20260730-007` may run once. Fresh independent Contract Review is
the current gate; no product source, real socket/lock, Keychain, Provider,
installed app, daemon or resident service action is yet authorized.

`CURRENT`: fresh independent Interaction Continuity Contract Review 1 returned
`FAIL` with one P1 and one P2. The P1 found that the exact owned files omitted
`internal/localipc/server.go`, although the frozen safe shutdown order requires
changing its current close-lock-before-remove sequence. The P2 found ambiguous
`unowned lock` wording where the intended reclaim input is an abandoned,
effective-user-owned lock and wrong-owner paths must remain fail-closed.

Contract Repair 1 adds only `internal/localipc/server.go`, freezes an explicit
close-order/concurrent-contender proof, and corrects the stale input to
`abandoned owned`. It changes no UI, protocol, credential, Provider, authority,
live or exit requirement. Fresh independent Contract Repair 1 Re-review is the
current gate. Product RED, implementation and all live actions remain locked.

`CURRENT`: fresh independent Contract Repair 1 Re-review returned `PASS` with
no P0, P1 or P2 finding. It confirmed `internal/localipc/server.go` ownership,
remove-before-advisory-lock-release ordering, the concurrent-contender proof,
the `abandoned owned` stale-lock term and wrong-owner rejection. It also
confirmed the existing Builder can support the required two-action
Provider/model presentation only by selecting a compatible existing role option
and showing its already-bound Runtime profile, model and auth mode; no
standalone model protocol, catalog or authority may be introduced.

The frozen Interaction Continuity contract is now ready for a pure governance
checkpoint commit. After that exact commit, mandatory deterministic RED is the
next gate. No product implementation or live action has occurred; P2A-W3
remains locked and no P2A-W4 exists.

`CURRENT`: pure governance checkpoint `65c719c` froze the complete P2A-W2
Interaction Continuity Exit Reopen before product edits. Mandatory RED is now
captured in
`.loom-evidence/phase2a/P2A-W2/interaction-continuity-exit-reopen-red.md`.
After discarding and correcting one test-only undefined fixture, the valid
local IPC RED proves that an abandoned owned lock is rejected and an active
server holds no advisory ownership. The task-first TUI RED fails only on the
missing `ScreenTasks` product symbol. The native Swift RED fails only on the
missing frozen workspace-continuity types, inspector cases and safe primary
copy. No production file, real socket/lock, Keychain, Provider, daemon,
installed app or resident service was changed. Minimal implementation is now
unlocked only inside the frozen complete owned boundary; live lineage
`p2a-w2-live-20260730-007` remains locked until full GREEN, visual proof,
fresh independent Implementation Review, atomic Candidate commit and exact
preflight. P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: the complete P2A-W2 Interaction Continuity Candidate is
deterministically GREEN. The native app now opens a task/chat-first
Tasks/Conversation/Inspector workspace, preserves per-task local continuity,
loads bounded historical activity, keeps Team Builder inline and save-only,
and uses human-facing Provider/model review language. Bubble Tea now starts at
Tasks, preserves task selection across primary views and hides raw history
identifiers.

The same Candidate implements the complete product socket/lock transaction:
strict private metadata, no-follow open, single-link/zero-byte/owned `0600`
validation, stable descriptor/path identity, a non-blocking exclusive advisory
lock held for the listener lifetime, bounded socket liveness, one-winner stale
reclaim and remove-before-release shutdown. Active, live, foreign, symlink,
hard-link, wrong-mode, nonzero, regular and replacement paths remain
fail-closed. The integrated product daemon starts from the attempt-006-shaped
abandoned owned lock in a private fixture and cleans its owned pair.

Focused tests, 50 two-contender repetitions, complete serial Go and race
suites, vet, module checks, 39 XCTest plus 4 Swift Testing debug cases, the
same Thread Sanitizer matrix, Swift Release build, diff/secret/scope checks and
wide/compact/minimum visual proof all pass. Evidence is recorded in
`interaction-continuity-deterministic-green.md` and
`interaction-continuity-visual-audit.md`. No real product socket/lock,
Keychain, Provider, installed app or resident service was touched. Fresh
independent read-only Implementation Review is the current gate; staging,
Candidate commit and live lineage `p2a-w2-live-20260730-007` remain locked.
P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: fresh independent Implementation Review 1 returned `FAIL` with one
P1 and two P2 findings. The P1 found that native and TUI confirmation omitted
the already-bound role Runtime, Provider, model, auth, permissions,
compatibility and complete budget review. One P2 found native/TUI task
search/filter missing. The other P2 found no observed interleaving proof for a
replacement installed after exact-remove's initial identity check. No P0 was
reported.

Repair 1 stays inside the same complete P2A-W2 contract and its exact owned
files; it creates no Amendment and no W4. Mandatory Repair 1 RED is recorded in
`interaction-continuity-repair-1-red.md`: missing native preflight/filter
symbols, missing TUI search and complete preflight copy, and missing observed
exact-remove boundary all fail causally. Real product socket/lock, Keychain,
Provider, installed app and resident service remain untouched. Repair
implementation is now the current gate; staging, commit and live lineage
`p2a-w2-live-20260730-007` remain locked.

`CURRENT`: P2A-W2 Repair 1 is deterministically GREEN inside the same frozen
complete Interaction Continuity contract. Native and TUI now provide bounded
task search/filter while preserving the selected task, and their final Builder
confirmation exposes the complete already-bound role, Provider, model, auth
mode, Runtime compatibility, permissions, maximum budget and estimated cost.
Exact lock-file removal now revalidates identity after an observed interleaving
seam and preserves a replacement installed after the first check.

The full matrix found and closed one cross-language source-boundary omission:
new Store state had acquired dependencies outside the existing standalone
`swiftc` fixture. Workspace continuity types now live with the owned Store,
Store reconciliation has no presentation-only dependency, and the unchanged
real Go IPC Server to Swift Client fixture passes. Focused tests,
50 two-contender lock repetitions, complete serial Go and race suites, vet,
module and diff checks, 41 XCTest plus 4 Swift Testing debug cases, the same
Thread Sanitizer matrix, Swift Release build, and regenerated wide/compact
visual proof all pass. No real product socket/lock, Keychain, Provider,
installed app or resident service was touched. Fresh independent read-only
Repair 1 Implementation Re-review is the current gate; staging, commit and live
lineage `p2a-w2-live-20260730-007` remain locked. P2A-W3 remains locked and no
P2A-W4 exists.

`CURRENT`: fresh independent Repair 1 Implementation Re-review 2 returned
`FAIL` with one P1 and no P0/P2. It confirmed the complete preflight,
task-search continuity, exact-remove interleaving proof and unchanged strict
Go-to-Swift fixture, but found role-option continuity still incomplete:
responsibility-only choice labels and name/purpose-only confirmation did not
expose the existing `builder_edit main_role/subagent_role` path.

Mandatory Repair 2 RED is recorded in
`interaction-continuity-repair-2-red.md`. Native lacked the bounded
role-option presentation/accepted role-edit fields; TUI lacked role choices and
an exact `main_role` edit command. Repair 2 remains inside the same complete
contract and exact owned files, creates no Amendment or W4, and changes no
application service, protocol, authority or live surface.

`CURRENT`: Repair 2 implementation is focused GREEN. Native role questions and
confirmation now show compatible existing roles and submit exact role-option
IDs through the existing `main_role`/`subagent_role` Builder fields. TUI
exposes the same choices and `m`/`s` one-action edits, then renders the returned
authoritative preview. No standalone model picker, protocol field, catalog or
authority was added.

Fresh Repair 2 Implementation Re-review returned `FAIL` with one P1 and one P2:
unselected alternatives received invented generic `Local provider`/`Native`
metadata even though only their Runtime/model are present in the bounded setup
snapshot, and alternate-profile truthfulness was not tested.

Mandatory Repair 3 RED is recorded in
`interaction-continuity-repair-3-red.md`. Repair 3 removes those guesses.
Unselected options show only authoritative responsibility, Runtime and model
plus a select-to-review cue; the selected option shows exact Provider/auth from
the returned Builder preview. Focused native and TUI tests pass. Full
deterministic re-verification passed.

Fresh Repair 3 Re-review returned `FAIL` with one P1 and one P2. Same-Agent
role options with different Runtime profile/instance bindings were still
matched as current by kind plus AgentDefinition, and the GREEN gate text still
named Repair 1.

Mandatory Repair 4 RED is recorded in
`interaction-continuity-repair-4-red.md`. Native and TUI now match selected
presentation with the full available tuple of kind, AgentDefinition, Runtime
profile and Runtime instance. Same-Agent alternate-profile fixtures pass, and
the GREEN gate text names Repair 4. Complete deterministic re-verification
passes: complete serial Go and race suites, vet, module checks, 41 XCTest plus
4 Swift Testing cases in debug and Thread Sanitizer configurations, Swift
Release, strict unchanged Go-to-Swift fixture, diff, scope, screenshot and
secret-negative checks are green.

Fresh independent Repair 4 Implementation Re-review 5 returned `PASS` with no
P0, P1 or P2 findings. It independently confirmed full-tuple role matching,
same-Agent alternate-profile truthfulness, exact existing Builder edit
dispatch, returned authoritative Provider/auth presentation, task-first
native/TUI continuity, exact socket/lock transaction and excluded-dirt
isolation. The atomic Candidate commit is the current gate. Live lineage
`p2a-w2-live-20260730-007` remains locked until that exact committed Candidate
passes preflight; P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: atomic P2A-W2 Interaction Continuity Candidate commit `2225c9f`
passed exact preflight and consumed its only authorized live lineage
`p2a-w2-live-20260730-007`. The Candidate daemon safely reclaimed the exact
abandoned product lock, became the sole private listener, discovered the
model-capable Pi 0.82.1 Runtime and later completed normal `SIGINT` joined
shutdown with `exit 0`, removing its exact product socket/lock pair without
SIGTERM or SIGKILL.

The exact native bundle opened directly into the task-first
Tasks/Conversation/Inspector workspace and completed the bounded task,
one-question Team Builder, exact role selection and human-readable preflight.
The live journey then exposed a product-path defect:
`ContentView` mounts `setupProviderPanel` only while `setupSnapshot == nil`,
but `ProviderConnectionDirectory` renders only when that snapshot is non-nil.
No Settings scene or alternate native route exists. The ordinary window
therefore cannot reach Provider `Manage` or the required MiniMax `Test`.

The Controller stopped without IPC/terminal bypass, Provider request, save,
retry or execution. Final SQLite integrity is `ok` with exactly one additional
model-capable `RuntimeInstanceDiscovered` fact and no new verification, Team or
execution fact. Source DB, resident lineage and excluded dirt remain
unmodified; attempt processes and isolation are clean.

Fresh independent Result-Evidence Review returned `PASS` with no P0, P1 or P2,
confirming the failed record is trustworthy rather than accepting W2. The only
lineage is consumed:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

No retry, alternate live route or single-point Amendment is permitted by the
frozen contract.

`CURRENT`: the Product Owner authorized one complete P2A-W2 Exit Contract
reopen and froze the Phase 2A Interaction Continuity / Mission Orchestration
Workbench Extra Goal at
`.loom-evidence/phase2a/P2A-W2/mission-orchestration-workbench-exit-contract.md`.
The baseline is `bcd27c1`. This remains the same P2A-W2, creates no W4, and
preserves attempt 007 as a trustworthy failed historical result.

The new vertical contract replaces the dashboard/task-list-first primary
surface with a Journal/Projection-derived Orchestration Board, Mission Room,
Team Pulse, Topology, Evidence Inspector, three Loom Decision Sheet templates,
Loom Graphite visual system and matching TUI semantics. Mission is explicitly
a facade over existing TeamExecution/WorkItem/Run/Evidence lineage, never a
second authority.

Decision commands are restricted to exact prepared inputs owned by the
existing Rules/Work authorities. Missing Evidence, policy, receipt, Grant,
view version or generation disables mutation; the product layer cannot infer
or write an authoritative result. Event, Grant and Evidence schema remain
unowned. Attempt-007 Provider-management reachability is a mandatory regression.

Fresh independent Contract Review is now the sole gate. No product source,
RED, daemon, native app, Provider, Keychain, prepared command, staging, commit
or live canary is authorized before Contract Review `PASS`. P2A-W2 remains
`HUMAN_REQUIRED / NOT ACCEPTED`, P2A-W3 remains locked and P2A-W4 does not
exist.

`CURRENT`: fresh independent Mission Workbench Contract Review 1 returned
`FAIL` with one P1 and no P0/P2. The decision facade requires one new strict
local IPC method, but the original owned list omitted the closed method
allowlist in `internal/localipc/protocol.go` and its test, so the protocol would
reject the command before daemon dispatch.

Contract Repair 1 reopens only those two protocol files and adds a causal RED:
the exact reviewed decision method must become allowed while every unknown
`decision_*` method remains fail-closed. `internal/localipc/server.go` remains
excluded. No other authority or scope finding was reported. Fresh independent
Contract Repair 1 Re-review is the only gate; product source and live actions
remain locked.

`CURRENT`: fresh Contract Repair 1 Re-review confirmed the prior IPC-ownership
P1 is closed, then returned `FAIL` on one new P1: the contract incorrectly made
`Needs You` a lifecycle lane. The authorized Board lifecycle is exactly
`Proposed`, `Ready`, `Orchestrating`, `Review`, `Complete`; `Blocked`,
`Retrying` and `Needs You` are card status, Attention filters, Timeline facts
or Mission Room inline decisions.

Contract Repair 2 freezes those exact lanes. It also names the single new IPC
method `mission_decision` and forbids per-sheet/alias methods. Pre-Review visual
evidence is clarified as a deterministic non-installed AppKit `NSWindow`
fixture only; the sole production native/socket/authority lineage remains
locked behind Implementation and Visual Review PASS. Fresh Contract Repair 2
Re-review is the current gate.

`CURRENT`: fresh independent Contract Repair 2 Re-review returned `PASS` with
no P0, P1 or P2 findings. It confirmed the exact five Board lanes, non-lifecycle
Attention semantics, single `mission_decision` method, causal protocol RED,
Projection-only Mission facade, prepared-command authority boundary,
attempt-007 Provider regression, deterministic-window/live separation, no W4
and one-canary/no-retry sequence.

The Mission Orchestration Workbench Exit Contract is now `FROZEN`. The pure
governance checkpoint commit is the current gate; only after that commit may
mandatory behavioral RED begin. No product implementation, daemon, socket,
Provider, Keychain, prepared authority command or live action has occurred.

`CURRENT`: governance checkpoint `0506166` unlocked the single P2A-W2 Mission
Orchestration Workbench Candidate. Mandatory RED is preserved in
`mission-orchestration-workbench-red.md`; it failed only on the frozen missing
Mission facade, five-lane Board, prepared Decision boundary, strict Swift
models, Mission-first TUI and Loom Graphite symbols.

The scoped Candidate is deterministically GREEN. The Go Projection/read facade
derives Mission, Team Pulse and Topology only from existing TeamExecution,
Team, Run, Evidence and Attention facts and normalizes all public collections
to JSON arrays. The one `mission_decision` method now has closed
`read`/`defer`/`submit` operations over an immutable prepared-command registry:
stale view or generation fails before authority, `Not now`/`Edit scope` are
zero-authority presentation results, concurrent submit has one winner, and an
opposite terminal action conflicts. Production without a prepared command
remains read-only.

The native app now opens to the five-lane Mission Board and provides a
three-column Mission Room, persistent per-Mission draft/Inspector state,
Provider management from the ordinary rail, and one native Loom Graphite host
for Authorization, Review and Recovery sheets. The TUI uses the same Mission
facade, exact lanes and `g b`/`g t`/`/`/`Enter`/`Esc`; `a` opens a bounded
read-only Approval presentation when no prepared command exists.

Implementation Review 1 returned `FAIL` on three P1 findings: the first
prepared backend accepted an arbitrary callback, the production runner omitted
the Decision API, and the exact full race command had not passed. Repair 1
closed all three within the same P2A-W2 Candidate.

Authorization now binds a real pending `rules.ApprovalRequestRecord` and exact
`rules.ApprovalDecisionRequest`; Review binds an exact
`work.TeamNodeAcceptanceInput`; Recovery binds an exact
`work.TeamRecoveryInput`. Real test-owned SQLite Journal, Run, Evidence and
Projection fixtures prove all three authority paths. The strict
`prepared_actions` collection enables only exact bound mutations. The
production daemon always mounts a fail-closed prepared registry; an empty
registry returns `conflict`, never `state_unavailable`. Submission rebuilds
the Projection before authority dispatch and rejects a real post-sheet Journal
view advance, not only a forged command version.

Complete Go, exact `go test -race ./...`, vet, Swift debug, Swift Thread
Sanitizer and Swift Release matrices pass. Strict real Go-server-to-Swift
decision decoding, the production-daemon socket boundary, wide/compact
Light/Dark, Mission Room, all three Decision sheets and TUI evidence are
recorded under `.loom-evidence/phase2a/P2A-W2/`. Fresh independent
Implementation Re-review and then Visual Review are the current gates. No
external daemon, Provider, Keychain, installed app or live canary has run.
P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: fresh final Implementation Re-review returned `FAIL` on two related
P1 prepared-command bindings and no P0/P2. Review/Recovery did not bind the
sheet `decision_digest` to the actual Work decision, and Authorization did not
bind the presented Mission/Team/Node/Attempt to the pending Rules request's
original ActionContext.

Repair 2 preserves a causal RED in
`mission-orchestration-workbench-repair-2-red.md`. Authorization now
reconstructs the exact pending request ID from the immutable ActionContext,
continuation and Rules Decision digests and checks the sheet identities.
Review binds the exact valid AcceptanceDecision and deterministic result.
Recovery binds the exact valid RecoveryDecision plus the current
TeamAttemptRecord, Evidence/classification digests and claim generation. No
Rules/Work authority or schema file was reopened. Full deterministic
verification and fresh independent Implementation Re-review remain the current
gates; live remains locked.

`CURRENT`: fresh Implementation Review 3 returned `FAIL` before any live
action. It confirmed the Repair 2 identity bindings, but found that the exact
production `loomd` runner could only mount an empty prepared registry and that
the recorded source lock no longer covered the newly reachable prepared-command
wire/UI path.

Repair 3 stays inside the same frozen P2A-W2 owned boundary. The ordinary
production runner can now construct five real Journal-backed Mission fixtures
and four exact prepared Rules/Work commands only when given one private,
canonical, attempt-bound controlled-fixture manifest. Without that manifest it
retains the empty fail-closed registry. Swift schema 2 requires
`prepared_decisions` to be an array, and a Review Mission without a prepared
acceptance command opens a read-only gate whose mutations remain disabled.

Focused, whole-repository, exact repository-race, vet, Swift debug, Thread
Sanitizer, Swift Release, visual-export, TUI, diff, forbidden-authority and
secret-negative checks are GREEN. The current 31-file source lock is
`2badc23c740b0c24fad494a02f2bf8956a60e4dfb2f35f17701fab6695213b55`.
Fresh independent Implementation Review 4 is the current gate, followed by
fresh Visual Review. No external daemon, installed app, Provider, credential
mutation, staging, commit or live canary has occurred. P2A-W3 remains locked
and no P2A-W4 exists.

`CURRENT`: fresh independent Implementation Review 4 returned `PASS` with no
P0/P1 findings. It reproduced every source hash and the combined lock, confirmed
the exact production/default fail-closed split, real authority lineage,
strict wire behavior, ordinary Inspector reachability, read-only missing-
Evidence gate and Repair 2 bindings. Its only P2 caveat is that the controlled
manifest's lexical 40-hex `source_commit` field is not runner self-attestation;
the mandatory post-commit preflight must bind exact binary hashes and commit
identity independently. Fresh independent Visual Review is now the sole gate
before atomic commit. Live remains locked.

`CURRENT`: Visual Review 1 returned `FAIL` with no P0 and four P1 findings.
Board cards did not expose Mission identity, priority, Team presence and a
readable milestone; the default TUI evidence omitted a shared Mission Detail;
Decision fixtures used generic or misleading values; and the Mission composer
did not expose the three frozen permission modes. A compact-lane overflow cue
was also missing at P2. Repair remains inside the same P2A-W2 Candidate and
does not create W4. Because production and fixture sources are reopening, the
previous source lock and Implementation Review PASS are superseded. Fresh
deterministic verification, source locking, Implementation Review and Visual
Review are required before commit; live remains locked.

`CURRENT`: the Visual Review 1 repair is deterministically GREEN inside the
same P2A-W2 Candidate. Board cards now show full Mission identity, priority,
Team/Attempt presence, node progress and readable milestone; the compact Board
has a visible five-lane horizontal-navigation cue. The TUI default Board selects
a real Mission and renders Team role/state, Attempt, current node, prepared
decision availability and milestone. Authorization, Review and Recovery sheets
now carry distinct truthful semantics, and the Mission composer visibly offers
`Plan only`, `Guided` and `Delegated` while stating that selection is only a
proposal until Loom confirms authority. Complete Go, repository race, vet,
Swift debug, Thread Sanitizer and Release matrices pass. The fresh 31-file
source lock is
`8269aac930e31afa9d8f581dcdceb61779436b2033b2cd071ca9dc5af802c8a2`.
Fresh independent Implementation Review is the current gate, followed by
Visual Review. No commit or live action has occurred.

`CURRENT`: fresh independent Implementation Review 5 returned `PASS` with no
P0/P1 findings. It reproduced the exact 31-file source lock, found no staged
Candidate or W4/authority expansion, and confirmed the prior Rules/Work
bindings, strict single-method IPC, default fail-closed production path,
private controlled fixture, default TUI Mission Detail and presentation-only
permission mode. Its only P2 caveat remains that manifest `source_commit` is
lexical metadata rather than runner self-attestation; post-commit preflight
must independently bind commit identity and exact binary hashes. Fresh
independent Visual Review is now the sole pre-commit gate. Live remains locked.

`CURRENT`: controller pre-Visual exact-contract self-check found that the
repaired Mission card still omitted `source_kind`, despite the frozen card
contract requiring Mission ID, title/source and priority. Visual Review 2 was
paused before verdict. The single UI source is reopened in the same Candidate;
the prior source lock and Implementation Review 5 PASS are superseded for final
bytes. Fresh Swift verification, source lock, Implementation Review and Visual
Review are required. Live remains locked.

`CURRENT`: the exact card contract is now closed by rendering the bounded
Mission `source_kind` beside priority. Swift debug, Thread Sanitizer and
Release all pass on the final UI byte, and deterministic wide/compact
Light/Dark evidence has been regenerated and inspected. The final 31-file
source lock is
`de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`.
Fresh Implementation Review is again the current gate, followed by Visual
Review. Live remains locked.

`CURRENT`: fresh independent Implementation Review 6 returned `PASS` with no
P0/P1. It reproduced the final 31-file source lock and confirmed
`MissionWorkbench.swift` is the only byte changed since Review 5; the added
`source_kind` remains bounded strict-snapshot presentation. All permission,
TUI, IPC, Rules/Work and default fail-closed closures remain unchanged. The
manifest self-attestation P2 caveat remains assigned to post-commit preflight.
Fresh independent Visual Review is now the sole pre-commit gate. Live remains
locked.

`CURRENT`: fresh independent Visual Review 3 returned `PASS` with no P0/P1/P2,
and the exact P2A-W2 Mission Orchestration Workbench Candidate was atomically
committed as `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`. Post-commit preflight
reproduced source lock
`de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7`
and independently froze the exact daemon, TUI, native executable, manifest and
state identities.

The single authorized live lineage
`p2a-w2-mission-workbench-live-20260730-001` was then consumed. The exact daemon
exited during construction with code `3` and bounded output
`daemon unavailable`; it never exposed the product socket and no native window
or TUI was launched. Read-only diagnosis showed the controlled real
Journal/Evidence fixture had passed and committed `72` Events across `27`
streams, after which the setup boundary rejected the supplied Codex executable:
`/Users/lune/Documents/Codex/devtools/npm/bin/codex` is a symlink, while the
accepted native-auth observer intentionally requires an absolute regular
executable by `Lstat`. The canonical target was not substituted because that
would be a forbidden replacement start.

This root-cause classification is strongly supported rather than directly
logged: the bounded daemon contract exposes only `daemon unavailable`, and
source order includes earlier setup constructors before native-auth setup. The
complete controlled fixture side effects, real symlink mismatch and committed
validation path support the diagnosis without weakening the bounded external
error contract.

Post-stop evidence confirms no controlled daemon/app/TUI process, no product
socket or product socket lock, an empty isolation directory, no open state
handle, `integrity_check = ok`, and no secret-like or hidden-reasoning Event
payload. No retry, alternate path or manual cleanup was used. Deterministic,
Implementation and Visual gates remain PASS for the committed bytes, but the
live requirements are unproven. P2A-W2 is therefore `HUMAN_REQUIRED / NOT
ACCEPTED`; P2A-W3 remains locked and no P2A-W4 exists.

Fresh independent Result-Evidence Review returned `PASS` with no P0/P1. It
independently reproduced the exit classification, SQLite counts and integrity,
binary/path identities, secret-negative scan and absence of controlled
process/socket/open-handle residue. Its sole P2 caveat is the explicitly
recorded inferential root-cause strength above. This failed lineage is closed
without retry; a new live execution would require an explicit reviewed reopen
of the frozen no-retry boundary.

`CURRENT`: the Product Owner's standing authorization for subsequent in-scope
actions has now been applied to one complete P2A-W2 Mission Orchestration Live
Closure Reopen. The frozen contract is
`.loom-evidence/phase2a/P2A-W2/mission-orchestration-live-closure-reopen-contract.md`.
It preserves the first failed lineage unchanged, creates no W4 or point
Amendment, owns no product source, and retains every accepted Journal, Rules,
Work, Grant, Evidence, Provider and IPC boundary.

The reopen permits one fresh lineage only after Contract Review, complete
deterministic/preflight verification and Preflight Implementation Review pass.
It requires the configured Codex symlink to be resolved before daemon start
and freezes the canonical regular executable identity; it does not relax
native-auth validation or modify Codex installation/OAuth. Product live action
remains locked. Fresh independent Contract Review is the current gate.

`CURRENT`: fresh independent Mission Orchestration Live Closure Contract Review
returned `PASS` with no P0/P1. It confirmed the one-complete-reopen precedence,
unchanged first failure, exact no-product-source boundary, canonical Codex
regular-file transaction, strict status parsing, fresh one-shot lineage,
complete 15-step native/TUI proof and preserved authority boundaries.

Its sole P2 execution caveat is binding: all mutable Go and Swift build/test
caches must live under the controlled attempt root, with excluded repository
paths compared before and after verification. The existing
`apps/macos/.build/` remains user-owned and must not be cleaned or rewritten.
The deterministic/preflight matrix is now the current gate; live remains
locked.

`CURRENT`: the Mission Orchestration Live Closure deterministic product matrix,
canonical Codex transaction, source lock and exact binary materialization
passed, but the excluded-path preflight gate returned `FAIL` before daemon
start. Canonical Codex is the regular executable
`/Users/lune/Documents/Codex/devtools/npm/lib/node_modules/@openai/codex/bin/codex.js`
with SHA-256 `134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477`;
its status-only observation returned the one accepted stderr line
`Logged in using ChatGPT`.

The first Go matrix was run concurrently with a fresh Swift build and five
five-second process-start fixtures timed out. The failure was preserved;
resource-isolated focused reruns passed, followed by complete Go, repository
race, vet, Swift debug, Thread Sanitizer, Release and real Go-to-Swift IPC
PASS. No product source differs from Candidate `d0252064`.

However, one native bin-path discovery command unexpectedly wrote generated
metadata into the pre-existing excluded `apps/macos/.build/` cache despite
receiving an attempt-local scratch path. Repository dirty-state digest stayed
identical, but excluded metadata digest changed from `9c26eecd...18d1c` to
`9ad85089...7924`. The Controller did not clean or reset that user-owned cache.
No daemon/app/TUI was started, the product socket/lock and controlled artifact
root remain absent, the controlled SQLite is empty and isolation is empty, so
the replacement lineage is not consumed. Live remains locked. Fresh
independent Preflight Failure Review is the current gate; P2A-W2 remains
`HUMAN_REQUIRED / NOT ACCEPTED`, P2A-W3 remains locked and no P2A-W4 exists.

`CURRENT`: fresh independent Preflight Failure Review returned `PASS` on
evidence accuracy with no P0/P1/P2. It independently confirmed the sequential
matrix PASS, source and binary hashes, canonical Codex status contract,
excluded metadata mismatch and timestamps, absence of controlled processes,
socket/lock, fixture artifacts and state mutations, and the no-cleanup record.
It also confirmed that the replacement lineage was not consumed because no
daemon or controlled fixture side effect occurred.

The live gate nevertheless remains `FAIL / LOCKED`: the excluded
`apps/macos/.build/` cache was modified and cannot be truthfully presented as
untouched. P2A-W2 remains `HUMAN_REQUIRED / NOT ACCEPTED`; P2A-W3 remains
locked and no P2A-W4 exists.

`CURRENT`: the Product Owner explicitly selected `隔离后继续` for the generated
Swift cache affected by the failed preflight. Preflight Repair 1 is frozen
inside the same complete P2A-W2 reopen at
`.loom-evidence/phase2a/P2A-W2/mission-orchestration-live-closure-preflight-repair-1.md`.
It preserves the failed preflight verdict and owns no product source.

The repair permits one exact same-volume atomic rename of the current
`apps/macos/.build/` directory into the private attempt quarantine after
content/metadata manifesting and open-handle checks. It permits no deletion,
recursive rewrite, cache reuse or later SwiftPM command. The replacement
lineage remains unconsumed and live remains locked. Fresh independent Repair
Contract Review is the current gate.

`CURRENT`: fresh independent Preflight Repair 1 Contract Review returned
`PASS` with no P0/P1. Its sole P2 execution caveat is binding: the cache
manifest must use symlink-safe `lstat` metadata, hash regular files only and
record but never follow the existing `debug` and `release` symlinks.

The exact same-volume quarantine transaction is now unlocked. It remains
recoverable, may not delete or recursively rewrite the cache, and does not
unlock daemon/App/TUI execution. Fresh independent post-rename Repair Review is
required before live.

`CURRENT`: Preflight Repair 1 quarantine completed by one exact same-volume
atomic rename. The repository `apps/macos/.build` path is absent; its complete
observed `493M` cache is preserved under the private attempt quarantine with
the same root device/inode and matching pre/post manifests for `7,308`
filesystem entries, `5,145` regular-file hashes and two untraversed relative
symlinks. Repository status excluding that intentional move is unchanged.

No SwiftPM process or open cache handle existed before rename. No file was
deleted, recursively rewritten, restored or reused. The exact daemon, TUI,
native, manifest and canonical Codex hashes remain frozen; product source still
matches Candidate `d0252064`. No daemon/App/TUI ran, product socket/lock and
fixture artifact root are absent, SQLite and isolation remain empty, and the
replacement lineage remains unconsumed. Live stays locked pending fresh
independent Preflight Repair 1 Review.

`CURRENT`: fresh independent Preflight Repair 1 Result Review returned `PASS`
with no P0/P1/P2. It reproduced the quarantine root identity and size, all
entry/file/symlink counts, matching symlink-safe manifests, unchanged
repository status outside the intentional move, source lock, exact live-input
hashes, empty process/handle/state/isolation checks and absent product
socket/lock/artifact root.

This Review unlocks the first daemon invocation for the same unconsumed
replacement lineage
`p2a-w2-mission-workbench-live-20260730-002`. The earlier excluded-cache
preflight remains immutable `FAIL`; the recoverable quarantine is its reviewed
Product Owner-authorized disposition. The complete 15-step canary and no-retry
boundary remain mandatory.

`CURRENT`: the unique Mission Orchestration replacement lineage
`p2a-w2-mission-workbench-live-20260730-002` was consumed and stopped
`FAIL — NATIVE_DECISION_CLIENT_PROTOCOL_UNAVAILABLE`.

The exact signed native app connected to the product socket and proved the
Journal-backed five-lane Mission Board, five controlled Missions, Mission Room
draft/Inspector continuity, Team Pulse, the Provider Manage route, a
fail-closed missing-Evidence Review Gate, real TUI parity and normal app/TUI/
daemon shutdown. The controlled daemon appended exactly one Pi discovery
Event; UI interaction appended no Event. Final state is 73 Events with SQLite
integrity `ok`, empty isolation, absent product socket/lock, no controlled
process or state handle and unchanged Candidate product source.

The required prepared Authorization and Recovery interactions are blocked by a
real product composition defect. `LocalIPCClient` implements the decision
methods but declares only `LocalProductClientProtocol` and
`LocalProductSetupClientProtocol`; `LocalProductStore` obtains the decision
client through a conditional cast to `LocalProductDecisionClientProtocol`, so
the real app receives `nil`. A read-only framed request proved the daemon
returns the valid prepared sheet, isolating the failure from Journal, IPC and
fixture authority.

No hot patch, direct IPC mutation, daemon restart, alternate binary or second
lineage was used. Fresh Result-Evidence Review returned `PASS` only for the
recorded `FAIL / HUMAN_REQUIRED` classification: SQLite integrity, Event
counts, secret-negative scans, process/socket cleanup, frozen binary identities,
source immutability and the native decision-client protocol defect were
independently checked. P2A-W2 remains `HUMAN_REQUIRED / NOT ACCEPTED`; P2A-W3
remains locked and no P2A-W4 exists. Any product repair requires a reviewed
complete P2A-W2 reopen rather than a point Amendment.

`CURRENT`: under the Product Owner's standing authorization, one complete
P2A-W2 Mission Decision Vertical Closure Reopen is now frozen at
`.loom-evidence/phase2a/P2A-W2/mission-decision-vertical-closure-reopen.md`.
It preserves both consumed failed live lineages and creates no point Amendment,
W2 subdivision or P2A-W4.

The exact product boundary is
`apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift` plus its real
production-composition coverage in
`apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`. The permitted
repair is only the missing `LocalProductDecisionClientProtocol` attribution
for already-implemented strict IPC methods. Journal, decision authority,
request/response shape, strict decoding, Store behavior and UI remain
unchanged.

Implementation and live remain locked. Fresh independent Contract Review is
the current gate. After Contract Review, the required order is real-client RED,
minimal composition repair, complete deterministic matrix, fresh
Implementation Review, then at most one fresh isolated complete native
decision canary.

`CURRENT`: Mission Decision Vertical Closure Contract Review returned `PASS`
with no P0/P1/P2. The real production-client RED was reproduced after the
reviewed contract commit, and the one-line
`LocalProductDecisionClientProtocol` conformance turns that focused test GREEN.
Complete Swift debug, Thread Sanitizer and Release gates pass using
attempt-local scratch paths.

The first Go full/race/vet commands also pass, but their evidence is not
accepted: the real Go-to-Swift contract fixture internally invoked SwiftPM
without a scratch path and recreated `apps/macos/.build` as an observed 88M
generated cache. No live process or state was started, so the one fresh live
lineage remains unconsumed.

Deterministic Cache Repair 1 is frozen inside the same complete P2A-W2 reopen
at
`.loom-evidence/phase2a/P2A-W2/mission-decision-deterministic-cache-repair-1.md`.
It permits only a reviewed same-volume recoverable quarantine of that exact
generated cache, followed by controlled `SWIFTPM_BUILD_DIR` routing and a
sequential real Go-to-Swift/full/race/vet rerun. No product scope expands.
Quarantine and rerun remain locked pending fresh Repair Review; live remains
locked.

`CURRENT`: fresh independent Deterministic Cache Repair 1 Review returned
`PASS` with no P0/P1/P2. It confirmed the exact 88M generated cache identity,
same-volume absent quarantine destination, symlink-safe manifest requirement,
no cache handle, no live process/state and the bounded
`SWIFTPM_BUILD_DIR` routing gate.

The exact recoverable quarantine and sequential controlled-route rerun are now
unlocked. Live and Implementation acceptance remain locked.

`CURRENT`: Deterministic Cache Repair 1 completed. The exact 88M generated
cache was preserved by one same-volume atomic rename with unchanged root
device/inode and matching 173-entry lstat, 147-file SHA and one-symlink
manifests. No cache content was deleted, rewritten, restored or reused.

The bounded `SWIFTPM_BUILD_DIR` probe resolved beneath the controlled attempt
root. The real Go-to-Swift prepared-decision fixture, full Go matrix,
repository race, vet and all focused authority/replay/concurrency gates then
passed sequentially; `apps/macos/.build` remained absent after every command.
Complete Swift debug, TSan and Release also pass.

Product diff remains the exact two owned Swift files. No daemon, app, TUI,
socket or Attempt 003 state exists. Fresh Cache Repair Result Review is the
current gate; live and Implementation acceptance remain locked.

`CURRENT`: fresh independent Deterministic Cache Repair 1 Result Review
returned `PASS` with no P0/P1/P2. It reproduced absent repository cache,
unchanged quarantine inode and size, byte-identical 173/147/1 manifests,
controlled Swift contract-probe output, exact two-file product diff and no
controlled process/socket/live residue.

This PASS unlocks fresh Implementation Review only. Live remains locked.

`CURRENT`: fresh independent Mission Decision Vertical Closure Implementation
Review returned `PASS` with no P0/P1/P2. It independently confirmed the exact
two-file product diff, minimal real-client protocol conformance, unchanged
strict IPC/fail-closed behavior, a fresh focused real-client GREEN, absent
repository SwiftPM cache and the repaired 32-file source lock:

```text
68ef6b9ab387fb5a4058a967f50795f4a988add34caf2854780e8fe6681abc66
```

No controlled daemon, native app, TUI, product socket or Attempt 003 state
exists. This PASS unlocks exactly one fresh complete lineage,
`p2a-w2-mission-decision-live-20260730-003`; P2A-W2 remains not accepted until
that native canary and its fresh Result-Evidence Review pass.
