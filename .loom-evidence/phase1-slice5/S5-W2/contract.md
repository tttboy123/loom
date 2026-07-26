# S5-W2 Contract — WorkPackage and Phase 1 Engineering Demo Integration

Status: FROZEN CANDIDATE — independent Contract Review required before RED.

- WorkItem: `S5-W2`
- Risk: `HIGH`
- Baseline: `93bdb1f`
- Date: `2026-07-26`
- Depends on: accepted Slices 1–4, accepted S5-W1, and the frozen Slice 5
  Exit Contract
- Capability: immutable bounded Coding/knowledge WorkPackages plus one
  controlled local Phase 1 engineering-demo suite and a non-executing final
  live-gate manifest

S5-W2 is the second and final Slice 5 product WorkItem. WorkPackage value,
codec, built-ins, parity integration, approval/recovery/reconnect/restart
fixtures, fail-closed cases, private-mode checks, and final live-gate manifest
are internal parts of this one Candidate. No WorkPackage-codec-only,
Demo-fixture-only, S5-W3, or other thin WorkItem may follow.

## Exact ownership

New product/tests:

- `internal/work/work_package.go`
- `internal/work/work_package_test.go`

New integration test only:

- `internal/app/phase1_engineering_demo_test.go`
  - external package `app_test`;
  - consumes accepted production APIs without reopening an authority;
  - may contain private deterministic clocks, in-process adapters, temporary
    SQLite/Evidence fixtures, and test-only crash/restart/effect counters;
  - never imports an installed Runtime or Provider.

Governance and final-gate artifacts:

- `.loom-evidence/phase1-slice5/S5-W2/**`
- the Controller-owned S5-W2 hunk in `docs/CURRENT.md`

No existing product or test file is reopened. No migration, dependency,
writer, Projection, API, CLI, daemon, Runtime adapter, Supervisor, Team,
Rule, verification, or Evidence implementation file is owned. Any need to
reopen one requires a reviewed amendment before editing.

`AGENTS.md`, `PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and
`.loom-evidence/phase1-slice3/POST-S3-W5-QUEUED-CONTRACT-INPUTS.md` remain
excluded and unstaged.

## Dependency direction and reused authorities

Production direction is only:

```text
internal/work/work_package.go
  -> Go standard library
```

The external demo test may import:

```text
internal/app
internal/api
internal/authorization
internal/evidence
internal/journal
internal/mode
internal/projection
internal/rules
internal/runtime
internal/supervisor
internal/teams
internal/verification
internal/work
protocol/bridge/v1
```

It invokes the accepted Journal, Work Authority, Grant Authority,
TeamCoordinator, Supervisor, Evidence Store, Rule/approval authority,
Projection, and TeamExecutionStream directly. It does not wrap them in a new
writer, scheduler, coordinator, acceptance authority, or retry loop.

WorkPackage is descriptive input only. It never creates, mutates, authorizes,
dispatches, approves, retries, verifies, completes, or activates any object.
The demo binds the package ID into the existing Rule `ActionContext` and uses
package fields only to select fixture AgentDefinition labels, tool-category
expectations, verifier key, Evidence vocabulary, and Rule-template labels.
The accepted authority sequence remains identical for both domains.

## Exact WorkPackage API

`internal/work` adds only:

```go
var (
    ErrInvalidWorkPackage        = errors.New("invalid WorkPackage")
    ErrWorkPackageDigestMismatch = errors.New("WorkPackage digest mismatch")
)

type WorkPackageDomain string

const (
    WorkPackageCoding       WorkPackageDomain = "coding"
    WorkPackageKnowledge    WorkPackageDomain = "knowledge_work"
    WorkPackageSchemaVersion                  = 1
    MaxWorkPackageEncodedBytes                = 4096
    MaxWorkPackageAgentDefinitions            = 3
    MaxWorkPackageToolCategories              = 16
    MaxWorkPackageEvidenceTypes               = 16
    MaxWorkPackageRuleTemplates               = 16
)

type WorkPackageInput struct {
    ID                             string
    Version                        int
    DomainKind                     WorkPackageDomain
    RecommendedAgentDefinitionIDs  []string
    ToolCategories                 []string
    DefaultVerifierKey             string
    EvidenceTypes                  []string
    DefaultCustomerRuleTemplateIDs []string
}

func NewWorkPackage(WorkPackageInput) (WorkPackage, error)
func LoadWorkPackage(WorkPackageInput, string) (WorkPackage, error)
func CodingWorkPackage() (WorkPackage, error)
func KnowledgeWorkPackage() (WorkPackage, error)

func (value WorkPackage) SchemaVersion() int
func (value WorkPackage) ID() string
func (value WorkPackage) Version() int
func (value WorkPackage) Digest() string
func (value WorkPackage) DomainKind() WorkPackageDomain
func (value WorkPackage) RecommendedAgentDefinitionIDs() []string
func (value WorkPackage) ToolCategories() []string
func (value WorkPackage) DefaultVerifierKey() string
func (value WorkPackage) EvidenceTypes() []string
func (value WorkPackage) DefaultCustomerRuleTemplateIDs() []string
func (value WorkPackage) MarshalJSON() ([]byte, error)
```

`LoadWorkPackage` returns `ErrInvalidWorkPackage` for invalid fields and wraps
`ErrWorkPackageDigestMismatch` when the supplied digest is not the exact
canonical digest. Zero values marshal to `ErrInvalidWorkPackage`; no partial
value is returned on error.

## WorkPackage validation and bounds

- ID, AgentDefinition IDs, verifier key, and Rule-template IDs are 1–128
  UTF-8 bytes.
- Tool categories and Evidence types are 1–64 UTF-8 bytes.
- Every text is already trimmed, contains no Unicode control character, and
  consists only of lower-case ASCII letters, digits, `.`, `_`, and `-`; the
  first byte is a lower-case letter.
- Version is 1–1,000,000.
- Domain is exactly `coding` or `knowledge_work`.
- Recommended AgentDefinition IDs contain 1–3 values.
- Tool categories, Evidence types, and default customer Rule-template IDs
  each contain 1–16 values.
- Every input collection is copied, duplicate-checked, and sorted ascending.
  Caller order does not change identity. Empty/nil collections are invalid.
- Accessors and JSON encoding return copies and never expose caller-owned
  storage.
- Canonical encoded WorkPackage is at most 4096 bytes.

No field may contain a prompt, credential, artifact path, script, command,
raw Rule, Grant, Evidence body, hidden reasoning, or executable content.

## Canonical identity and JSON

Digest input is compact `encoding/json` output of this private struct in exact
field order:

```text
schema_version
id
version
domain_kind
recommended_agent_definition_ids
tool_categories
default_verifier_key
evidence_types
default_customer_rule_template_ids
```

`schema_version` is `1`. All collections are the copied sorted values. Digest
is lower-case SHA-256 of those exact JSON bytes.

Public `MarshalJSON` emits compact JSON in exact field order:

```text
schema_version
id
version
digest
domain_kind
recommended_agent_definition_ids
tool_categories
default_verifier_key
evidence_types
default_customer_rule_template_ids
```

The digest field is the computed lower-case SHA-256. There is no map,
whitespace, HTML-escaping customization, `omitempty`, null collection, or
unknown/mutable payload.

## Exact built-in WorkPackages

`CodingWorkPackage` is:

```text
id: work-package.coding
version: 1
domain_kind: coding
recommended_agent_definition_ids:
  agent.coding.main
  agent.coding.reviewer
  agent.coding.test
tool_categories:
  artifact.read
  repository.read
  workspace.edit
  workspace.test
default_verifier_key: verifier.code-review.v1
evidence_types:
  code_patch
  test_report
  verification_report
default_customer_rule_template_ids:
  rule.template.coding-review
  rule.template.protected-write
digest: 4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f
```

`KnowledgeWorkPackage` is:

```text
id: work-package.knowledge
version: 1
domain_kind: knowledge_work
recommended_agent_definition_ids:
  agent.knowledge.main
  agent.knowledge.research
  agent.knowledge.verifier
tool_categories:
  artifact.read
  document.read
  research.local
  workspace.edit
default_verifier_key: verifier.source-review.v1
evidence_types:
  analysis_report
  source_index
  verification_report
default_customer_rule_template_ids:
  rule.template.human-publication
  rule.template.source-attribution
digest: 5a834baad4a0e4c557d96883f242860f4d136de208dd6721b85d8c2b8c5a0393
```

The built-ins differ only in the fields allowed by `TECH-PLAN.md` section 4.2.
They have distinct digests and are rebuilt afresh on each call.

## Controlled engineering-demo suite

The new external integration file freezes these test families:

```text
TestPhase1EngineeringDemoWorkPackageParity
TestPhase1EngineeringDemoApprovalRestartReconnectAndRecovery
TestPhase1EngineeringDemoFailClosedAndPrivateModes
TestPhase1LiveGateManifestIsBoundedAndNonExecuting
```

Together they are the S5-W2 engineering canary. They use at most:

- 2 WorkPackage scenarios;
- 1 Main plus 2 SubAgents;
- 3 attempts per logical node;
- 2 independent in-process scheduler/coordinator callers;
- 32 recorded ordered demo steps per scenario;
- the accepted 96-stream/128-page/32-KiB cursor bounds;
- one 8-KiB non-executing live-gate manifest.

The suite must prove:

1. explicit Agent routing and an accepted saved-Team/TeamInstance binding are
   fixture prerequisites; ordinary conversation creates no execution object;
2. both built-ins enter the same `teams.ExecutionPlan`,
   `work.Authority`, `TeamCoordinator`, Supervisor, Grant, Evidence,
   verification, recovery, and timeline APIs;
3. normalized authoritative Event-kind/state-transition traces are identical
   between domains; only package fields and fixture Evidence vocabulary differ;
4. one Main plus two independent SubAgents obey the same dependency order and
   capacity ceiling;
5. an existing customer Rule decision pauses a WorkItem for approval, survives
   a new Authority/Projection instance, and resumes only after the accepted
   decision;
6. transient/invalid output consumes only bounded explicit attempts, with a
   distinct WorkItem/Run/generation/Grant/Evidence lineage and no hidden retry;
7. a high-risk node uses a distinct verifier Runtime/Run/Grant/Evidence and
   verifier tentative output is never delivered to the source subscriber;
8. authorized source tentative output is observed only after private capture
   and before terminal authoritative verification/Done is read from Journal;
9. disconnect after a bounded cursor and reconstruction with new
   Projection/TeamExecutionStream instances yields the remaining facts exactly
   once;
10. restart/recovery does not duplicate a Run, Grant, Evidence, Done, or the
    test-only external-effect marker;
11. malformed/stale cursor, generation, Frame, Projection, and WorkPackage
    fail closed, while a failed Projection rebuild retains the prior
    `GlobalReadView`;
12. Journal/Evidence/private fixture roots and files are respectively
    `0700`/`0600`.

The canary may reuse accepted public APIs and implement private test helpers.
It may not weaken existing tests, call a live executable, use network,
Provider/model traffic, credentials, user files, a daemon, or an external
side effect.

## Mandatory WorkPackage and demo RED

Before production implementation:

```text
go test ./internal/work ./internal/app \
  -run 'WorkPackage|Phase1EngineeringDemo|Phase1LiveGate'
```

must fail on missing WorkPackage symbols/behavior and missing controlled demo
manifest. RED is behavioral; a compile failure alone is insufficient. Tests
may begin with a temporary test-local shim only long enough to expose at least:

- invalid/duplicate/oversized/mutated WorkPackage acceptance;
- digest/parity mismatch;
- different state-machine trace between packages;
- duplicate recovery/effect delivery;
- malformed/stale acceptance; or
- an executing/oversized live-gate manifest.

The exact RED output is recorded before GREEN.

## Non-executing live-gate manifest

The Candidate includes:

```text
.loom-evidence/phase1-slice5/S5-W2/live-demo-manifest.json
.loom-evidence/phase1-slice5/S5-W2/live-demo-checklist.md
```

The JSON is indented valid JSON with a trailing newline, at most 8192 bytes,
contains no secret or absolute user path, and has exactly:

```text
schema_version: 1
status: non_executing
selected_saved_team_id
selected_work_package: {id, version, digest}
required_runtime_capability:
  {adapter_type, bridge_protocol, streaming,
   session_resume_required, runtime_instance_id}
bounded_task:
  {summary, max_nodes, max_attempts_per_node,
   network_allowed, external_side_effects_allowed}
approval_expectations[]
evidence_location
stop_cancel_procedure[]
rollback_recovery_checks[]
unresolved_human_inputs[]
execution_authorized: false
```

It selects the Coding built-in and fixture saved Team
`saved-team.phase1-engineering`. Runtime capability is Pi +
`loom.bridge.v1` + streaming, with
`runtime_instance_id="HUMAN_SELECTION_REQUIRED"`. Task limits are three nodes,
three attempts, no network, and no external effects. Evidence location is the
literal `PRIVATE_USER_SELECTED_ROOT`.

The three operational arrays are exact ordered lower-snake identifiers. Every
item is 1–64 ASCII bytes, begins with a lower-case letter, contains only
lower-case letters/digits/underscore, and is unique within its array.

`approval_expectations` contains exactly:

```text
user_confirms_saved_team_and_work_package
customer_rule_may_require_start_run_approval
approval_decision_is_journal_authoritative
```

`stop_cancel_procedure` contains exactly:

```text
cancel_client_delivery_without_cancelling_run
request_authoritative_run_cancel
verify_terminal_and_grant_revocation
do_not_kill_or_restart_daemon
```

`rollback_recovery_checks` contains exactly:

```text
reconnect_from_last_cursor
rebuild_projection_from_journal
verify_evidence_digest_and_private_modes
verify_no_duplicate_run_grant_evidence_done
stop_human_required_on_indeterminate_state
```

The unresolved human inputs are exactly:

```text
installed_runtime_instance_id
private_source_root
approval_decision
live_execution_authorization
final_review_signoff
```

`execution_authorized` is always false. The checklist states exact preflight,
approval, stop/cancel, evidence, restart/reconnect, rollback, and final
signature gates, and ends `VERDICT: PASS`. Neither file executes anything or
claims the real task passed.

The complete manifest value is frozen as:

```json
{
  "schema_version": 1,
  "status": "non_executing",
  "selected_saved_team_id": "saved-team.phase1-engineering",
  "selected_work_package": {
    "id": "work-package.coding",
    "version": 1,
    "digest": "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f"
  },
  "required_runtime_capability": {
    "adapter_type": "pi",
    "bridge_protocol": "loom.bridge.v1",
    "streaming": true,
    "session_resume_required": false,
    "runtime_instance_id": "HUMAN_SELECTION_REQUIRED"
  },
  "bounded_task": {
    "summary": "bounded_local_coding_fixture",
    "max_nodes": 3,
    "max_attempts_per_node": 3,
    "network_allowed": false,
    "external_side_effects_allowed": false
  },
  "approval_expectations": [
    "user_confirms_saved_team_and_work_package",
    "customer_rule_may_require_start_run_approval",
    "approval_decision_is_journal_authoritative"
  ],
  "evidence_location": "PRIVATE_USER_SELECTED_ROOT",
  "stop_cancel_procedure": [
    "cancel_client_delivery_without_cancelling_run",
    "request_authoritative_run_cancel",
    "verify_terminal_and_grant_revocation",
    "do_not_kill_or_restart_daemon"
  ],
  "rollback_recovery_checks": [
    "reconnect_from_last_cursor",
    "rebuild_projection_from_journal",
    "verify_evidence_digest_and_private_modes",
    "verify_no_duplicate_run_grant_evidence_done",
    "stop_human_required_on_indeterminate_state"
  ],
  "unresolved_human_inputs": [
    "installed_runtime_instance_id",
    "private_source_root",
    "approval_decision",
    "live_execution_authorization",
    "final_review_signoff"
  ],
  "execution_authorized": false
}
```

## Verification matrix

Required checks:

```text
go test ./internal/work ./internal/app -count=1
go test ./internal/api ./internal/authorization ./internal/evidence \
  ./internal/journal ./internal/projection ./internal/rules \
  ./internal/runtime ./internal/supervisor ./internal/teams \
  ./internal/verification -count=1
go test -race -count=10 ./internal/work ./internal/app ./internal/api
go test ./...
go test -race ./...
go vet ./...
gofmt -d <all S5-W2 Go files>
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true ./internal/work
go mod verify
```

Additional audits:

- exact built-in JSON/digest constants and 100 input-order permutations;
- mutation/deep-copy and zero-value marshal tests;
- typed `errors.Is` matrix;
- two-domain normalized authority-trace equality;
- restart/reconnect exact-once counts;
- stale generation/cursor/Frame/Projection/WorkPackage rejection;
- no raw output, Grant, credential, prompt, hidden reasoning, absolute path,
  or tentative delta in Journal/manifest;
- state/Evidence directory and file modes;
- no change to `go.mod`, `go.sum`, `migrations/**`, accepted authority files,
  executable/daemon wiring, or excluded shared-worktree paths.

Fresh independent Implementation Review is required. Developer stops at
`ready_for_review`; only Controller may accept and create the exact local
atomic commit after all gates pass.

## Trust boundary and exclusions

S5-W2 does not authorize:

- WorkPackage persistence or activation;
- Team/Run/Grant/Evidence/Rule creation by WorkPackage;
- a new Journal, writer, Projection, scheduler, coordinator, retry,
  approval, verification, acceptance, or completion authority;
- direct SQL outside the private test fixture and accepted
  Journal/Projection APIs;
- installed Runtime execution, process launch, daemon activation, Provider or
  model traffic, network, credentials, paid/external work, publication, or
  user-file mutation;
- Web/TUI/API server/WebSocket/notification, Autopilot, session/context
  checkpoint, Provider fallback, Phase 2/3 implementation, or S5-W3;
- push, merge, rebase, reset, force, release, or final user sign-off.

The canary is hermetic engineering evidence only. Even after S5-W2 and the
whole-Slice Review pass, Phase 1 stops at
`READY_FOR_FINAL_USER_SIGNOFF`; the assistant cannot perform the live task or
sign for the user.

VERDICT: PASS
