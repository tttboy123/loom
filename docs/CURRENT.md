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

Create the S2-W35 local atomic commit only after a fresh pre-commit matrix and
exact staged-scope audit.
Scheduling and daemon entry/config remain separate later WorkItems. Bridge,
real Runtime Adapter execution, AgentGrant, claim generation, and WorkItem
dispatch remain Slice 3 boundaries even when research drafts propose them
earlier. Push, merge, release, runtime activation, credential changes, and
FastContext installation remain unauthorized. The optional local FastContext
Spike remains separate and was not installed or activated.
