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
