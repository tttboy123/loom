# Current State

Updated: 2026-08-17

## Phase 4 · 联网工具链安装版 Mission 全链路（V33，2026-08-17）

- 安装版 live 验证：Mission 子 Agent（DeepSeek + web_search Enrollment）模型主动调用
  `loom_web_search` → Attempt-Loop 准入 → 执行适配器授权 → 真实执行；
  Journal 记录 `ToolCallAdmitted` / `ToolExecutionProposed` / `ToolExecutionAllowed` 与
  WebSearch 工具事实。
- 修复：Enrollment binding digest 往返（journal 不再砖）、supervisor profile 保留 enrollment、
  DeepSeek 模型名格式校验、tool_calls `index` 字段、内容+工具调用并存、并行工具调用循环、
  Context 拒绝可恢复、Web 工具在 Mission 暴露（动态 Enrollment 物化 + App 派生环境合并）、
  远程工具免 worktree、gateway JobID=WorkItemID。
- V33b 追加：DDG 空结果返回受控 `search_unavailable` 消息；WebFetch 纳入只读放行；
  工具结果 content-type 修复；多工具 sequence 绑定；空最终答案回填；evidence 提交
  snapshot 补齐 policy/rate-card/capacity 流；verifier 错误传播 + 真实派发。
- 已知剩余：独立 verifier 重试报 `binding_changed`（enrollment-bound profile 的
  重试绑定契约），需单独切片。见
  `.loom-evidence/phase4/network-capability/P2D-W4-mission-web-tool-loop-live-v33.md`。

## Phase 4 · 4.1 RoundTable（受治理交接）上线与稳定化 (2026-08-17)

`CURRENT / 4.1 INSTALLED-LIVE PASS`: RoundTable is committed (01ff06c8 +
this slice), the app is rebuilt + reinstalled at `/Users/lune/Applications/Loom.app`
(v0.5.3; old preserved at `Loom.app.previous`), and the full dual-seat journey
runs over the real installed socket: create → add writer/target seats → open
round → propose → relay → ack → insert → conclude. The `AlignmentSummary`
artifact exists in the Evidence Store, its SHA-256 equals the Journal's
`summary_digest`, and a re-read after conclude returns the identical view
digest (restart consistency; post-conclude writes rejected).

`CURRENT / TUI`: new **Roundtable** screen (tab-cycled) drives the same
Journal-authoritative journey with `n` next / `r` replay / `e` session; a
loading guard prevents repeat-`n` double-continue; live frames end in
`Session concluded · digest <64hex>`. macOS `RoundtableWorkbench` (sidebar
Roundtable) remains the graphical dual-seat journey.

`CURRENT / FIXES`:
- `AlignmentSummary` now records a real `concluded_at` and serializes empty
  artifact refs as `[]` (was zero timestamp / `null`).
- The Journal `RoundtableMessageProposed` fact payload now stores
  `artifact_refs: []` (was `null`), consistent with the replayed view.
- TUI wire request types exported so the CLI/live driver can drive the journey.

`VERIFICATION`: full Go suite green (serial, `-p 1`); `swift test` 244/0;
race on roundtable+tui+loomd green; gofmt/vet/`git diff --check` clean;
installed live IPC journey + TUI journey logs and per-gate evidence in
`.loom-evidence/phase4/4.1-roundtable/`. Independent review performed
(BLOCK raised on TUI restart/entry hazards, resolved → APPROVE);
sign-off pending operator (08-review-signoff.md). Frozen 4.2–4.5 slice
prompts are ready in `09-next-slices-plan.md` and execute one-at-a-time after
4.1 sign-off.

## Phase 2D Conversation Model Routing + Credential Gating FIX — Installed 0.5.3 (2026-08-17)

`CURRENT / INSTALLED 0.5.3`: rebuilt and installed `/Users/lune/Applications/Loom.app`
(v0.5.3 build 65; previous build preserved at `Loom.app.previous`). Live IPC
`setup_snapshot` on the running daemon confirms the OpenCode conversation
profile default is now `opencode/deepseek-v4-flash-free` (OpenCode's hosted
free tier), not `deepseek-chat`; DeepSeek stays a separate verified-only
brokered profile.
- Effective model is strictly bound to the selected Provider's catalog: a
  stale cross-Provider selection (e.g. `deepseek-chat` left on a MiniMax or
  OpenCode conversation) can no longer display or dispatch.
- Selecting an unavailable model (for example `zai/glm-*` or a MiniMax model
  before MiniMax is verified) now surfaces an actionable in-composer notice
  ("Requires a verified <provider> Provider credential...") instead of a
  silent no-op; the notice clears on a successful selection or profile switch.

## Phase 2D Conversation Model Routing + Credential Gating FIX (2026-08-17)

`CURRENT / FIXES`:
- The OpenCode profile no longer silently defaults to a DeepSeek/MiniMax
  model. `openCodeConversationDefaultModel` now always returns OpenCode's own
  hosted free-tier model (`opencode/deepseek-v4-flash-free`,
  `OpenCodeConversationDefaultModel`), so opening the App never routes a
  "DeepSeek/MiniMax" conversation through the OpenCode harness by surprise;
  those Providers have their own verified-only conversation profiles.
- MiniMax/DeepSeek/zai models are gated at selection time. The client derives
  each model's owning Provider (qualified `provider/model` prefix, otherwise
  the selected Provider) and disables the model unless that Provider has a
  verified Loom account; `selectConversationModel` refuses unavailable models
  and `effectiveConversationModelID` falls back to a usable native model when
  the selected model's account is revoked. The picker shows an actionable
  "Requires a verified <provider> Provider credential" hint.
- `OpenCodeCredentialEnv("opencode")` is native (`ok=false`): OpenCode's own
  free-tier models run without injecting a key, so they no longer fail with
  "requires a verified opencode Provider credential".

`VERIFICATION`: full Go suite green (`go test ./... -count=1 -p 1`); `swift
test` 242 tests, 0 failures (1 pre-existing visual-export skip); new Swift
gating tests cover verified/unverified model availability, refused selection,
and effective-model fallback after revocation.

## Phase 2D V32 Governed Handoff / Roundtable — Strict IPC + Cross-Client Journey PASS (2026-08-17)

`CURRENT / ROUNDTABLE (GOVERNED HANDOFF)`: new `internal/roundtable` journal
authority — a moderator hosts multiple seats and relays bounded (8 KiB),
digest-bound messages with per-hop confirmation
`pending -> relayed -> acknowledged -> inserted|dropped`, reusing the existing
Journal CAS and Evidence Store (no second authority). Conclude publishes a
SHA-256 `AlignmentSummary` Artifact and rejects post-conclude writes. Strict
Local IPC methods (`roundtable_session_create` … `roundtable_snapshot`) with
server-authoritative timestamps and typed error codes
(`not_moderator`, `seat_unavailable`, `concluded`, `invalid_body`,
`too_many_messages`, `too_many_seats`, `conflict`, `not_found`,
`invalid_request`, `state_unavailable`). Swift client models + 11 client
methods + `LocalProductStore` helpers with typed error surfacing, and a
self-guiding "Roundtable" workbench in the app (create -> seats -> open round
-> propose -> relay -> acknowledge -> insert -> conclude, or "Run full
journey"). Canonical view digest is a pure function of the normalized view;
empty collections serialize as `[]`, never `null`.

`CURRENT / FIXES`: two real wire defects fixed while wiring the Swift client:
- Empty `artifact_refs` / empty round `messages` marshaled as `null` from nil
  slices, which strict Swift decoders reject; the authority now normalizes to
  non-nil empty slices (`normalizedArtifactRefs`, `cloneView`).
- The Swift view decoder initially required seats to equal referenced seats,
  which rejected valid early views; it now checks referenced seats are a
  subset of registered seats.

`VERIFICATION`: full Go suite green (`go test ./... -count=1 -p 1`; the
real-process tests that flake under parallel load pass serially/in
isolation); `go test -race` on roundtable packages green; `swift test` 240
tests, 0 failures (1 pre-existing visual-export skip); Go E2E over an
authenticated Local IPC Unix socket; cross-client journey — the Swift
`LocalIPCClient` runs the full lifecycle against a real Go server over a real
Unix socket and decodes every view; release bundle builds clean with the
`roundtable_*` methods in the bundled daemon.

## Phase 2D V34 G4/G5 — Failure Isolation + Web/MCP Diagnostics Installed-Live PASS (2026-08-16)

`CURRENT / G4 INSTALLED-LIVE PASS`: revoking the MiniMax broker credential on
the installed App blocks exactly the two MiniMax-bound Agents at preflight with
the credential reason while the two DeepSeek-bound peers stay `ready` and
dispatch real paid calls; the accounting board isolates the failure to the
MiniMax account (`TestLiveSingleAgentFailureIsolationE2E`).

`CURRENT / G5 INSTALLED-LIVE PASS`: a `web_search` + `mcp_server` Enrollment
configured under `deepseek.primary` appears active + policy-current in the
account directory; binding `web_search` to one subagent freezes the Enrollment
pair into the saved TeamDefinition + materialized TeamInstance; revoking it
blocks only the bound Agent at preflight
(`Remote tool enrollment was revoked...`) while the unbound main stays ready;
default production (no Search/MCP port) materializes no remote capability and
stays fail-closed (`TestLiveRemoteToolEnrollmentIsolationE2E`).

`CURRENT / FIXES`: three real defects fixed for G4/G5:
- `productRuntimeProfileFromRecord` dropped the Enrollment pair during
  materialization; `MaterializeConfirmedTeam` now threads the materialization
  profiles through the saved-Team binding.
- `resolveMissionExecutionProfile` dropped the Enrollment pair when resolving
  the frozen execution binding, so a revoked Enrollment never blocked its
  Agent at preflight.
- `internal/localipc` `validMethod` + Swift allowlist accept the enrollment
  configure/revoke routes.

`VERIFICATION`: full Go suite green; `swift test` all suites pass;
`git diff --check` clean; all four live gates (G3/G6, G4, G5) pass on the
production App bundle.

## Phase 2D V34 G6 Accounting + Governance Board — Installed-Live PASS (2026-08-16)

`CURRENT / G6 INSTALLED-LIVE PASS`: with Provider Account Policies + Model
Rate Cards configured, the installed App daemon's `timeline_page` board now
projects real per-account accounting for the mixed-provider Team: attempt
counts, failed/rate-limited counts, error-rate basis points, budget, policy
revision + digest, concurrency/dispatch/budget ceilings, accounting coverage,
token usage (input/output/cache), and per-attempt cost rows
(`provider_reported` vs `rate_card_estimate`), plus the explicit
accounting-incomplete state for attempts that failed before usage.

`CURRENT / FIXES`: two real defects blocked G5/G6 configuration and mission
completion:
- `internal/localipc` `validMethod` was missing
  `provider_model_rate_card_configure` and
  `remote_tool_backend_enrollment_configure`/`_revoke` (the daemon rejected
  them with `unknown_method`). Added them and the Swift client allowlist.
- `internal/api` `maxTentativeDelta` was 2 KiB; the loom-native adapter
  publishes the full bounded model response as one `MessageEvent` delta, so
  real responses were rejected (`invalid node output`) and attempts stayed
  `running` forever. Raised to 256 KiB. A completed MiniMax attempt now
  projects exact tokens (2898) + a `rate_card_estimate` USD cost.

`VERIFICATION`: full Go suite green (known harnessadapter flake passes in
isolation); `swift test` all suites pass; `git diff --check` clean.

## Phase 2D V33 Brokered-Only Mission Execution — G3 Installed-Live PASS (2026-08-16)

`CURRENT / G3 INSTALLED-LIVE PASS`: the installed App daemon now confirms and
starts a real 4-Agent mixed-provider Team end to end (DeepSeek main + MiniMax +
DeepSeek reviewer + MiniMax researcher). `builder_confirm` materializes the
TeamInstance, preflight resolves `4/4 ready` nodes across two independent
Provider accounts, and `mission_execution start` returns `status=running` with
real paid Provider calls under per-Agent Vault leases and frozen bindings.
Verified with `LOOM_LIVE_TEAM_E2E=1` on the production bundle at
`/Users/lune/Applications/Loom.app` (daemon launched without
`--local-model-*`; no local GGUF model present).

`CURRENT / BROKERED-ONLY MISSION EXECUTION`: removed the hard
`LocalModelCatalog` prerequisite for Mission execution. The installed Pi
runtime ships the Pi CLI but no local model, so the previous daemon built no
Execution config and preflight returned `state_unavailable`. Now:
`missionExecutionConfigFromDaemonBuild` emits a brokered-only Execution config;
`newProductMissionExecutor`/`buildProductMissionExecutionAPI` accept a nil
catalog and only append the `pi-cli` deferred adapter when a local model
exists; legacy `pi-cli` single-role bindings still fail closed without a model.

`CURRENT / PER-INSTANCE RUNTIME DISCOVERY DIGEST`: the materializer recorded
the full-catalog composite discovery digest as the main Agent's
`RuntimeDiscoveryDigest`, which never matched the projected runtime instance's
per-instance `DiscoveryDigest`, so configured mixed-provider bindings always
conflicted at preflight. The binding candidate now carries the main runtime's
per-instance digest (`WithMainRuntimeDiscoveryDigest`) and the writer persists
it; validation compares canonical fields only.

`CURRENT / GOVERNED CONTEXT RETRIEVAL PAIRING`: `productMissionExecutor` no
longer injects a bare `ContextRetriever` (the supervisor rejects a retriever
without its paired `DeliveryBroker`). The governed attempt-loop adapter now
materializes the scoped retriever and its delivery coordinator together
downstream, preserving attempt-loop accounting.

`CURRENT / NATIVE PROMPT BOUND`: raised the OpenAI-compatible nativeadapter
prompt limit from 4 KiB to 64 KiB (the loom-native Context Capsule cap is
32 KiB; conversation adapters use 64 KiB). Real mission prompts (objective +
workspace context) previously exceeded 4 KiB and were rejected as a bridge
protocol failure.

`CURRENT / LIVE GATE HARNESS`: `TestLiveMixedProviderTeamE2E` now uses valid
UUID correlation IDs, the exact `work-package.coding` digest, snapshot limit 64
(so it reads real TeamInstance IDs instead of silently falling back), selects
the newly confirmed TeamInstance, archives leftover `team-live-mixed-*` teams,
retries view-version conflicts, asserts `4/4 ready`, and decodes the start
envelope status.

`VERIFICATION`: `go test ./...` green except the known pre-existing
`internal/runtime/harnessadapter` child-process flake (passes 3/3 in
isolation); `swift test` all suites pass; focused race checks pass;
`git diff --check` clean.

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

`CURRENT`: the one permitted
`p2a-w2-mission-decision-live-20260730-003` lineage was consumed and stopped
`FAIL — PREPARED_DECISION_VIEW_STALE_AFTER_DISCOVERY`.

The signed native app proved the repaired real-client composition by opening a
prepared Authorization sheet. Mission Board, Mission Room continuity, Team
Pulse and Provider Manage also remained intact, and `Not now` produced no
Journal mutation. The first and only `Deny` submission then failed closed with
the sheet preserved and Events unchanged at 73.

Read-only diagnostics bound the failure exactly: the current authoritative
view version was `18ca4c66...32db5`, while every prepared decision retained
`9d3f8a9f...f9579` from before the daemon's first Pi discovery Event advanced
the GlobalReadView. Stale-view fencing therefore correctly rejected the
command, but the controlled prepared-decision lifecycle is not live-submittable
after first discovery.

No second click, direct mutation, hidden retry, alternate binary or restart
occurred. The native app and one controlled daemon exited normally; socket and
lock are absent, isolation is empty, SQLite integrity is `ok`, all 73 baseline
Events remain unchanged and product source is still Candidate `700086d`.

P2A-W2 remains `HUMAN_REQUIRED / NOT ACCEPTED`; P2A-W3 remains locked and
P2A-W4 does not exist. Fresh independent Result-Evidence Review is the only
remaining permitted action for this consumed lineage.

`CURRENT`: fresh independent Attempt 003 Result-Evidence Review returned
`PASS` with no P0/P1/P2 for the failed-outcome evidence accuracy. It reproduced
the private attempt identities, SQLite `ok`/73-Event baseline and hashes, zero
decision-side-effect Events, absent controlled processes/socket/lock/handles,
empty isolation, absent repository SwiftPM cache, exact 32-file source lock
and product-source identity at Candidate `700086d`.

Code inspection independently confirmed the lifecycle cause: the fixture
freezes prepared sheet view versions before the observer's first discovery
write, while execution refreshes the current view and rejects the resulting
stale command.

The authoritative stop state is now synchronized:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

The Result Review PASS validates only that failed classification. The consumed
complete reopen authorizes no restart, retry, direct mutation, new attempt or
single-point Amendment.

`CURRENT`: under the Product Owner's standing authorization and the active
Phase 2A Extra Goal, one complete P2A-W2 Final Mission Decision View Lifecycle
Closure Reopen is drafted at
`.loom-evidence/phase2a/P2A-W2/mission-decision-view-lifecycle-closure-reopen.md`.
It preserves consumed Attempt 003 unchanged, creates no W2 subdivision or
P2A-W4, and owns only the prepared-command/read-snapshot lifecycle plus exact
tests.

The frozen intended invariant is that a successful authoritative snapshot and
all still-prepared Decision commands carry one exact GlobalReadView version.
The registry must refresh and rebind atomically only after every unconsumed
entry proves that version; a projection failure preserves the prior immutable
view and command bytes without refresh. Submission still independently fences
stale view/generation, replay, identity drift and concurrent losers before the
existing Rules/Work authority.

Fresh independent Contract Review is the current gate. Production source,
RED, daemon, native app, Provider, credential, staging, commit and replacement
live lineage remain locked. P2A-W2 is `HUMAN_REQUIRED / NOT ACCEPTED`, P2A-W3
is locked and P2A-W4 does not exist.

`CURRENT`: fresh independent Final Mission Decision View Lifecycle Contract
Review returned `PASS` with no P0/P1/P2 findings. It confirmed the complete
reopen precedence, exact owned files, successful-view atomic rebind,
projection-failure cache preservation, old-command and generation fencing,
unchanged authority boundaries, real Go-server/strict-Swift fixture, one fresh
lineage/no retry and no-W4 constraints.

The contract is now `FROZEN`. A pure governance checkpoint is the current
gate. Only after that checkpoint may mandatory RED modify the exact test
boundary. Product implementation, daemon/native live action and replacement
lineage remain locked.

`CURRENT`: governance checkpoint `a649679` froze the complete Final Mission
Decision View Lifecycle Closure contract and independent Review PASS.
Mandatory RED is preserved at
`.loom-evidence/phase2a/P2A-W2/mission-decision-view-lifecycle-closure-red.md`.

The real Journal-backed app test proves a later authoritative fact changes the
GlobalReadView while the prepared registry still publishes the old view. The
API test proves a Projection-failure stale snapshot currently re-reads and
mixes in a different command version instead of preserving the previous
coherent pair. Both tests fail only on the frozen missing lifecycle behavior;
no production source or live process changed.

The exact two production files are now eligible for minimal implementation.
Live, staging and Candidate acceptance remain locked pending deterministic
GREEN and fresh independent Implementation Review.

`CURRENT`: the Final Mission Decision View Lifecycle Candidate is
deterministically GREEN inside the exact frozen five-file boundary. Successful
snapshot reads now atomically rebind every unconsumed prepared command only
after all Journal-backed refreshers prove the snapshot's exact GlobalReadView.
Projection failure verifies and preserves the prior immutable view/command
pair; a mismatch fails state-unavailable. Old view/generation, replay,
identity drift, in-flight listing and concurrent losers remain fail-closed
before the existing Rules/Work authority.

Focused app/API/daemon tests, real Journal -> read service -> Go IPC coverage,
full Go, repository race, vet, real Go-server-to-strict-Swift fixture, Swift
debug, Thread Sanitizer and Release all pass from attempt-local build paths.
The initial package-parallel Go run encountered only a shared SwiftPM
`build.db` lock and is not counted; serialized package execution passed the
complete matrix. Repository `apps/macos/.build` remains absent.

The exact five-file source lock is
`10ae688306cab607d3de8b266ce358bbb5d0f972fb69eb62882030e59b8d1dea`.
Fresh independent Implementation Review is now the sole gate before an atomic
Candidate commit and preflight. No daemon/native live action or replacement
lineage has run; P2A-W2 remains not accepted, P2A-W3 locked and no P2A-W4
exists.

`CURRENT`: fresh independent Final Mission Decision View Lifecycle
Implementation Review 1 returned `FAIL` with one P1 and one P2. Production
semantics and authority boundaries passed review, but the new vertical test
stopped at direct handler invocation and therefore did not prove the exact
snapshot -> Runtime discovery -> rebound Decision lifecycle through a real Go
IPC server/socket/client. The source-lock combined digest also reproduced only
in file order while its method text incorrectly said sorted.

Repair 1 is frozen inside the same complete P2A-W2 contract and exact owned
boundary. It reopens only `cmd/loomd/product_daemon_test.go` to route the
existing lifecycle through `localipc.NewServer` and `NewClient`, plus evidence
and source-lock wording. Both production files remain locked byte-for-byte.
Fresh deterministic verification and independent Implementation Re-review are
required; Candidate commit and live remain locked.

`CURRENT`: Repair 1 is deterministically `PASS`. The exact lifecycle test now
uses one private real Go IPC server/client for snapshot before discovery,
Runtime discovery commit, snapshot after discovery, old-command conflict and
current-command read, with unchanged Event count. Focused and full Go/race,
vet, strict Swift real-server fixture, and Swift debug/TSan/Release all pass.

Implementation Review 1's independent Swift fixture had recreated an 81 MiB
repository `.build` cache without the controlled environment. With no process
or handle using it, the exact directory was preserved by same-volume atomic
move to Loom's private quarantine; device/inode `16777229/77062929`, size
`83264 KiB` and `172` entries are unchanged. Nothing was deleted or reused,
and repository `apps/macos/.build` is absent again.

The corrected five-file source lock is
`c5c103e3ba540f694685673430f464ba0c4be523ae810a6acc38345fe9068724`,
computed in files-array order. Both production files are byte-identical to
Review 1. Fresh independent Implementation Re-review is the only gate;
Candidate commit and live remain locked.

`CURRENT`: fresh independent Implementation Re-review returned `PASS` with no
P0/P1/P2. It reproduced all five hashes and combined source lock, confirmed
the real Unix socket/framing/client lifecycle closes Review 1 P1, and accepted
all-or-nothing refresh, stale-pair preservation, independent submission
fencing, concurrency behavior and unchanged authority boundaries. Focused
Go/race, vet and diff checks passed; no Swift command ran and repository
`.build` stayed absent.

The atomic Candidate commit is now the sole gate before exact post-commit
preflight. The one replacement live lineage remains locked until that commit
and preflight succeed. P2A-W2 remains not accepted, P2A-W3 remains locked and
no P2A-W4 exists.

`CURRENT`: atomic Candidate commit `c94f30b` contains the complete Final Mission
Decision View Lifecycle repair. Exact post-commit preflight then passed for the
single fresh lineage `p2a-w2-mission-decision-live-20260801-004`.

The controlled daemon and signed native app each started exactly once. The
daemon's first Pi discovery advanced the GlobalReadView; the next real product
snapshot returned all four unconsumed prepared commands rebound to the exact
current view. In the real native Mission Room, `Not now` left Journal Events at
73. One `Deny` then succeeded exactly once and appended only:

```text
ApprovalDecided              +1
WorkItemApprovalResolved     +1
```

The resulting authoritative snapshot advanced again, retained exactly three
prepared commands on the new view and removed the consumed deny command. A
replay of the consumed command failed closed with `conflict`. The same native
lineage also proved Mission Board, Mission Room, Team Pulse, Provider Manage,
missing-Evidence Review Gate and disabled unauthorized composer behavior. The
attempt-local TUI read the same five Mission/lane identities from the same
socket and exited without a Journal write.

Native app, TUI and daemon exited normally. Product socket and
`loomd.sock.lock` cleaned up without manual removal; SQLite integrity is `ok`
with exactly 75 Events; isolation is empty; product source and executable
hashes remain locked to Candidate `c94f30b`; repository `apps/macos/.build`
remains absent; and the unrelated resident Runtime observer was not signalled
or reconfigured.

Fresh independent Attempt 004 Result-Evidence Review returned `PASS` with no
P0/P1. It independently reproduced Candidate/source/manifest/native identities,
the exact 73 -> 75 two-fact authority delta, SQLite integrity, process/socket
cleanup, empty isolation and absence of persisted sensitive data. Its P2 notes
correctly limit offline reproducibility of Controller-only IPC/UI/TUI
transcripts and distinguish inactive attempt-local SQLite/SwiftPM lock files
from the cleaned product IPC lock.

The Phase 2A Extra Goal's Mission Orchestration Workbench boundary is accepted
inside P2A-W2. It creates no second authority and does not activate autonomous
execution. The next Phase 2A work may only begin by freezing the separate
P2A-W3 Controlled Execution Experience contract.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ELIGIBLE FOR CONTRACT FREEZE / NOT YET STARTED
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the unique `P2A-W3 Controlled Execution Experience` is now frozen
at `.loom-evidence/phase2a/P2A-W3/contract.md` against baseline `848f068`.
Its required DONE/PARTIAL/MISSING reconciliation is frozen at
`.loom-evidence/phase2a/P2A-W3/exit-gap-matrix.md`. Contract Review 1 returned
`FAIL` on three owned-boundary defects: an unsupported authoritative
WorkPackage/restart claim, an unowned real Swift contract probe, and unowned
PX-02 Go/Swift health surfaces. Contract Repair 1 keeps WorkPackage as a typed
pre-start proposal, begins durable authority at the accepted TeamExecution
facts, and adds the exact Swift probe and product-read/model files to ownership.
Fresh independent Repair 1 Re-review returned `PASS` with no P0/P1.

No Event schema, Journal, Projection, Rules, Work, Grant, Evidence, Supervisor,
Runtime adapter or bridge authority is reopened. Codex and MiniMax W3 canaries
remain bounded authentication/provider product-path claims; only Pi may claim
controlled execution. Mandatory behavioral RED and two bounded Repair REDs are
recorded. The Candidate now composes the strict `mission_execution` API through
the accepted TeamCoordinator, Work/Grant authorities, Supervisor, Pi adapter,
Evidence Store, GlobalReadView and TeamExecutionStream. Native and TUI product
flows provide visible WorkPackage/confirmed-Team selection, exact preflight,
explicit Start, tentative output, prepared decision routing, exact cancel and
same-lineage reconnect without typed internal IDs.

A real-SQLite deterministic vertical loopback closes source and independent
Verifier Run/Grant/Frame/Evidence lineages plus one canonical terminal, and
proves reconnect does not redispatch. Restart reconstructs only exact
Journal-visible plan/semantic/workflow/recipe state and fails closed on unknown
recipes. Focused, repeated, whole-repository, race, vet, dependency, Swift,
Thread Sanitizer and native Release-build gates pass. The Candidate is now
source-lock ready for fresh independent Implementation Review. Review 1 returned
`FAIL` with three P1 findings: the concrete backend implemented only cancel,
the production Swift probe did not exercise `mission_execution`, and terminal
flights were never reaped; it also reported an unbounded preflight lifetime as
P2. Bounded Repair 1 now routes every closed non-cancel action through exact
refresh-current prepared Decision commands, adds a five-minute server-owned
preflight lease and strict Swift expiry wire, exercises production Swift
preflight/start plus malformed-wire rejection through the real Go UDS server,
and reaps completed authoritative terminal flights. A focused RED additionally
caught and corrected `decide` versus the accepted `submit` operation token.

All focused, full, race, vet, dependency, Swift, TSan, release-build, format and
diff gates pass again after Repair 1. The repaired source lock is ready for a
fresh independent Implementation Re-review. Re-review 2 returned `FAIL` on two
P1 identity gaps: native/TUI used an ephemeral MissionID that the Journal-backed
read model could not reconstruct, and control did not reject MissionID drift.
Repair 2 now derives the single rebuildable `mission/<team_instance_id>` identity
in native, TUI, contract-probe and restart paths, rejects Mission/Team mismatch
before any backend call, and proves the Start result identity equals the
terminal Snapshot identity. The full deterministic matrix passes again.

The Repair 2 source lock is ready for a new fresh independent Implementation
Re-review. P2A-W3 is not yet accepted and no live process, credential action or
canary has occurred.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 2 COMPLETE / IMPLEMENTATION RE-REVIEW PENDING / NO LIVE CANARY
P2A-W4 = DOES NOT EXIST
```

`HUMAN_REQUIRED`: fresh Implementation Re-review 3 returned `PASS` for the
exact Repair 2 30-file source lock. Before freezing any live manifest, the
required post-review precondition audit proved a vertical product gap that the
review had not exercised: accepted Builder confirmation persists a
TeamDefinition but creates no TeamInstance/Main AgentInstance, and every
retained P2A-W2 product database has zero such facts. Therefore a fresh user
cannot select the executable confirmed Team required by W3 preflight.

Repair 3 RED now proves the missing Go daemon, TUI refresh and native refresh
behavior. A W3-owned composition through the accepted Saved-Team builders
failed closed because the mandatory Main plus dormant SubAgent selections count
as Runtime usage 2 while installed Pi truthfully reports capacity 1. Correcting
that rule requires reopening the accepted
`internal/teams/saved_team_binding.go` authority boundary, which Section 12 of
the frozen contract forbids without a reviewed amendment. The attempted
production composition was removed and the reviewed daemon source hash was
restored. The exact evidence is in
`.loom-evidence/phase2a/P2A-W3/live-precondition-audit.md`.

No Codex, MiniMax or Pi manifest was frozen or consumed; no Provider,
credential, Runtime execution, native walkthrough or W3 commit occurred. The
unrelated resident daemon remains untouched.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / SAVED-TEAM CAPACITY AMENDMENT REQUIRED / NO LIVE CANARY
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the Product Owner authorized the bounded `P2A-W3 Saved-Team Dormant
Capacity Amendment`. It is frozen at
`.loom-evidence/phase2a/P2A-W3/saved-team-dormant-capacity-amendment.md` and
reopens only `internal/teams/saved_team_binding.go` plus its test. A fresh
independent read-only Contract Review returned `PASS` with no P0/P1/P2
findings. The reviewed decision counts only the single active-on-materialization
Main against saved-Team materialization capacity while retaining and fully
validating every dormant SubAgent binding. Runtime truth, dispatch/run-time
capacity, future activation, CAS/generation fencing, Journal and all other
accepted authorities remain unchanged.

Repair 3 is now unlocked RED-first. No production change under the Amendment,
Provider/credential access, Runtime execution, live manifest or canary has yet
occurred. Live remains locked until the complete deterministic matrix and a
fresh independent Implementation Review pass.

```text
P2A-W2 = ACCEPTED
P2A-W3 = DORMANT CAPACITY AMENDMENT CONTRACT PASS / REPAIR 3 RED NEXT / NO LIVE CANARY
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: P2A-W3 Repair 3 has implemented the reviewed Saved-Team Dormant
Capacity Amendment. Only the active-on-materialization Main consumes saved-Team
materialization capacity; every dormant SubAgent binding is still retained and
fully validated, and no active execution capacity or future activation rule is
changed. Builder confirmation now composes the accepted Saved-Team authorities
to create exactly one TeamInstance and one Main AgentInstance, with no WorkItem,
Run, Grant or execution fact, then refreshes the TUI/native authoritative view.

The Amendment RED, three product-level Repair 3 REDs, focused and complete Go/
Swift suites, whole-repository non-race/race tests, vet, dependency, Thread
Sanitizer, Release-build, format, diff, scope and secret gates all pass. A fresh
independent Implementation Review over the exact Repair 3 source lock is next.
No live manifest, Provider/credential access, Runtime execution or canary has
occurred; all live gates remain locked pending that Review.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 3 DETERMINISTIC PASS / IMPLEMENTATION REVIEW PENDING / NO LIVE CANARY
P2A-W4 = DOES NOT EXIST
```

`HUMAN_REQUIRED`: P2A-W3 Repair 3 passed the complete deterministic matrix and
fresh independent Implementation Review 4 over the exact 42-entry source lock
`03d213d9862ce1f7be95d4547bfaae67893e58a8c98018d6a6cd651bd251c3ce`.
The reviewed Saved-Team Dormant Capacity Amendment also passed its live bounded
claim: one online capacity-1 Pi Runtime materialized exactly one Main
AgentInstance while retaining the selected SubAgent as dormant, with zero
WorkItem, Run, Grant or execution fact.

The three separate source-locked, one-shot live gates did not pass:

- Codex was visible as `Available`, but the daemon closed at
  `observer_models_timeout` before product preflight;
- MiniMax retained `Verified`, but one explicit product `Test` action committed
  no new terminal `ProviderCredentialVerified` fact; and
- Pi saved an executable Team, but exact read-only Mission preflight failed
  closed because live metadata exposes
  `loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m` while the source-locked mission
  binding accepts only the unnamespaced identity.

No Mission Start occurred. All controlled processes exited, product socket and
lock cleaned up, isolation is empty, retained SQLite databases pass integrity,
and no credential material was retained. Fresh independent Result-Evidence
Reviews returned `PASS` for evidence trustworthiness with no P0/P1; they confirm
the product failures rather than accepting them. The no-terminal walkthrough
is ineligible because the live prerequisites failed.

All live lineages are consumed with no retry. The Candidate is not accepted and
must not be committed. Any correction requires a reviewed complete P2A-W3
reopen of the affected product boundaries; it must not be split into another
single-point Amendment or P2A-W4.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / LIVE GATES FAILED / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`HUMAN_REQUIRED`: the unique `P2A-W3 Complete Live Compatibility` reopen closed
its complete deterministic matrix after three independent repair Reviews. The
final source lock
`904b1ceec6b1cd11d27498382b221c734180e0817eff8b31360d98d0b3160ccd`
passed fresh independent Implementation Review 4 with no P0/P1/P2. The repair
keeps only exact typed Pi metadata timeouts containable, publishes bounded
Runtime degradation as snapshot `partial` plus exact reason while daemon,
Journal and Projection remain available/current, preserves strict Swift schema
v2 decoding through a real Go UDS fixture, closes exact qualified Pi identity,
causal MiniMax operation identity and saved-Team human display.

Three fresh source-locked replacement live lineages were then consumed once:

- Codex `PASS`: one deterministic Pi `--list-models` timeout was contained with
  exactly one invocation; the same product daemon remained available, native
  `Codex Available` and `Partial authoritative view` were visible, and the saved
  Team returned exact product `Preflight is ready` without Start or state write.
- MiniMax `FAIL / product_defect`: one native `Test` visibly traversed
  `Verified -> Testing -> Unavailable`, but a bounded Journal observation still
  found only the three inherited `ProviderCredentialVerified` facts. No new
  attributable terminal revision/fact was committed and no retry occurred.
- Pi `FAIL / observer_unknown`: the single daemon start failed closed with
  `daemon failed: observer_unknown` before product socket publication. No app,
  preflight, Start, local model, Run, Grant, Frame or Evidence path was entered,
  and no retry occurred.

Fresh independent Result Review over result lock
`42f71716993d279fe96255812bfd309045cf099e5440d9ea1487500513d5e267`
returned `PASS FOR EVIDENCE / PRODUCT FAIL / HUMAN_REQUIRED`. All manifest,
result, source, authority and retained SQLite hashes matched; integrity passed;
no raw secret, hidden retry, allowance violation, attempt process, socket/lock
or isolation residue exists. The unrelated pre-existing `demo-resident` daemon
remains visible and untouched.

The final no-terminal walkthrough is ineligible. All replacement allowances are
consumed; no additional canary, commit or P2A-W4 is authorized. Any correction
requires fresh Product Owner authorization and a new reviewed governed P2A-W3
boundary; it must not be a retry-only, wrapper-only or split single-point
Amendment.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / CODEX PASS / MINIMAX PRODUCT DEFECT / PI OBSERVER UNKNOWN / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the Product Owner's fresh instruction to reprocess the two failed
replacement outcomes has been applied only as governance authorization. A
read-only causal diagnosis is recorded at
`.loom-evidence/phase2a/P2A-W3/complete-live-failure-diagnosis.md`: the MiniMax
terminal gap is confirmed at the pre-Provider Secret Store read boundary, while
the retained Pi `observer_unknown` output is too lossy to recover its causal
leaf. No live process, credential action, Runtime invocation, state mutation,
staging or commit occurred during diagnosis.

One complete vertical repair is frozen at
`.loom-evidence/phase2a/P2A-W3/authoritative-terminal-and-observer-closure-contract.md`.
It jointly requires an idempotent authoritative unavailable fact for an
explicit MiniMax Test that fails before Provider observation and a complete
typed, non-disclosing observer error taxonomy with copied-state Pi component
proof. It reopens no Event schema, Keychain implementation, Provider verifier,
Projection, StateWriter, Supervisor, Grant, Evidence, Rules, Scheduler or Pi
RPC authority. Independent Contract Review is pending. The contract itself
authorizes no live attempt.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / UNIFIED REPAIR CONTRACT REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the unique P2A-W3 Authoritative Terminal and Observer Closure
Contract passed independent Contract Review with no P0/P1/P2. Causal REDs then
proved both live defects before behavior changed. The implementation now records
one idempotent authoritative unavailable verification fact when Secret Store
access fails before Provider observation, and publishes a complete safe typed
observer reason taxonomy while preserving exact timeout-only containment.

Focused, repeated, real Swift-to-Go UDS, copied-state Pi 0.82.1, complete
serialized Go, complete serialized Go race, vet, module, format, diff, secret,
Swift normal, Swift ThreadSanitizer and arm64 Release gates all pass. Locked
Event/StateWriter/Projection/Supervisor/Bridge authority hashes remain exact.
The immutable implementation source lock and a fresh independent Implementation
Review are next. No Provider, Keychain, Pi, native, live-canary, staging or
commit action has occurred under this repair.

```text
P2A-W2 = ACCEPTED
P2A-W3 = UNIFIED REPAIR DETERMINISTIC PASS / IMPLEMENTATION REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: independent Implementation Review 1 correctly returned `FAIL` with
two P1 findings: four observer reason families did not fail closed for every
known-plus-unknown tree, and the copied-state Pi proof used a synthetic
one-event database rather than the retained six-fact live SQLite. That source
lock is superseded and authorized no live action.

Repair 1 added causal RED for all four ambiguity exceptions, removed those
exceptions, and now copies the exact retained Pi database with SHA-256
`677624b6...e5461a2`. The fixture proves all six facts, saved-Team payloads,
Pi 0.82.1 identity, qualified local model, no-write observation, complete public
failure attribution and final database bytes remain unchanged. Focused,
retained-state, complete serialized Go/race, Swift normal/ThreadSanitizer,
Release, vet, module, format, diff and secret gates pass. A new immutable source
lock and a fresh independent Implementation Re-review are next; live remains
locked.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 1 DETERMINISTIC PASS / IMPLEMENTATION RE-REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: independent Implementation Re-review 2 returned `FAIL` with two
remaining P1 findings: same-command incompatible Pi metadata failure leaves
were not treated as ambiguous, and the retained six-fact SQLite proof was
conditional on an environment variable. The Repair 1 source lock is superseded
and authorized no live action.

Repair 2 added a causal same-command conflict RED, now requires uniqueness of
both metadata command and failure category, and removed the synthetic fallback.
Every ordinary test decodes the reviewed compressed six-fact SQLite evidence,
requires exact restored SHA-256 `677624b6...e5461a2`, and proves Runtime,
saved-Team and complete Journal immutability across success and the full public
failure matrix. Focused/full/race/Swift/TSan/Release/static/security gates pass.
A fresh immutable Repair 2 lock and independent Re-review are next; live remains
locked.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 2 DETERMINISTIC PASS / IMPLEMENTATION RE-REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the third fresh independent Implementation Re-review verified exact
Repair 2 source lock `9ddf62b8...419430a8`, all locked authorities and the
self-contained retained six-fact SQLite, then returned `PASS` with no P0/P1/P2.
The authoritative MiniMax unavailable terminal, complete fail-closed observer
taxonomy and retained Pi/saved-Team immutability are deterministically closed.

No live action has yet occurred under the passing lock. The next permitted
actions are to freeze, preflight and consume at most one new MiniMax explicit
Test lineage and one new Pi saved-Team controlled-execution lineage. They remain
separate, fresh, no-retry attempts and require independent Result Review before
walkthrough or commit.

```text
P2A-W2 = ACCEPTED
P2A-W3 = IMPLEMENTATION REVIEW PASS / LIVE MANIFESTS PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`HUMAN_REQUIRED`: both newly frozen Authoritative Terminal/Observer live
lineages were consumed and failed before product socket publication. MiniMax
attempt `phase2a-w3-live-20260802-minimax-003` and Pi attempt
`phase2a-w3-live-20260802-pi-003` each returned exit code `3` with the sole
public line `daemon unavailable`. No native Test, Provider request, Pi RPC,
local-model invocation, saved-Team preflight, Start, Run, Grant, Frame or
Evidence path was entered. Both SQLite files are byte-identical to preflight,
pass integrity, and have no Journal delta. No retry is permitted.

Read-only diagnosis at
`.loom-evidence/phase2a/P2A-W3/authoritative-live-construction-failure-diagnosis.md`
identifies the common construction incompatibility: the frozen live manifests
supplied the normal user-level npm Codex path, but that path is a symlink and
the strict Codex native-auth constructor accepts only an executable regular
file observed through `Lstat`. The failure occurs during setup construction,
before execution composition and IPC publication, then `run()` masks every
builder error as generic `daemon unavailable`.

The required correction is one governed P2A-W3 vertical repair covering
canonical executable identity binding, safe build-stage reason attribution,
and a real live-shaped product construction test with local-model execution
enabled. It must not weaken identity checks, expose raw paths/errors, split a
new WorkItem, or reuse either consumed lineage. Result-evidence review is the
current gate; no walkthrough, staging, commit, or P2A-W4 is permitted.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / BOTH LIVE CONSTRUCTION FAILED / RESULT REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Result Review over immutable result lock
`64e5c53d...67ed94` returned `PASS WITH P2 LIMITATIONS` for evidence and
`FAIL / HUMAN_REQUIRED` for the product. It independently reproduced the npm
Codex symlink identity incompatibility and verified both unchanged SQLite
databases, exact fact counts, integrity, source/authority hashes, process/socket
cleanup, resident exclusion, and non-disclosure. The P2 limitation is that the
retained attempt roots do not contain an independent raw stderr/start-counter
transcript; it does not authorize reinterpretation or retry.

The allowed next gate is a new reviewed P2A-W3 vertical repair covering
canonical Codex launcher identity, safe build-stage attribution, and the full
live-shaped construction path. Both old lineages remain consumed. No final
walkthrough, live action, staging, commit or P2A-W4 is permitted.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / RESULT REVIEW PASS / VERTICAL REPAIR CONTRACT NEXT / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the unique P2A-W3 Native Launcher and Build Transaction Closure
Contract is frozen at
`.loom-evidence/phase2a/P2A-W3/native-launcher-and-build-transaction-closure-contract.md`.
It closes official npm Codex launcher resolution to a canonical native binary,
complete safe build-stage attribution, and real setup-only plus
execution-enabled product construction through the Go IPC server in one
vertical boundary. It creates no W4, reopens no Journal/Projection/Execution
authority, and authorizes no live action. Fresh independent Contract Review is
the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = NATIVE LAUNCHER AND BUILD TRANSACTION CONTRACT REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Contract Review verified exact contract hash
`4493858a...d30b18` and returned `PASS` with no P0/P1/P2. Mandatory causal RED
is now the gate. No live action, staging, commit or W4 is authorized.

```text
P2A-W2 = ACCEPTED
P2A-W3 = NATIVE LAUNCHER AND BUILD TRANSACTION CONTRACT PASS / CAUSAL RED NEXT / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`HUMAN_REQUIRED`: causal RED closed the npm launcher and safe build-attribution
unit defects, but the mandatory execution-enabled product construction fixture
proved a locked-boundary conflict. A non-nil `LocalModelCatalog` is validated
inside the Pi adapter against the production-frozen 1.1GB model SHA before
setup/execution/IPC, so a small deterministic fixture fails as
`build_observer / invalid local runtime observation daemon`.

The reviewed contract explicitly locks Pi adapter files and requires a stop if
RED proves they are needed. The recommended repair is a reviewed, test-only
expected-digest/binding injection seam that production cannot access; the
alternative is a non-hermetic mandatory dependency on the installed 1.1GB
model. No scope expansion, live action, staging or commit has occurred.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / CAUSAL RED PROVED LOCKED PI TEST-SEAM CONFLICT / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the blocker is resolved without reopening Pi authority by freezing
the P2A-W3 Native Launcher Build Methodology Amendment. The ordinary matrix
remains hermetic; one separate mandatory component gate opts into the exact
already installed private model/server and validates their frozen hashes before
crossing the real execution-enabled production builder. It is read/hash only,
starts no model/Pi/Provider/UI process, creates no W4 and authorizes no live
action. Independent Amendment Review is the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = METHODOLOGY AMENDMENT REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent review verified Native Launcher Build Methodology
Amendment SHA-256 `78a57fda...be22bc` and returned `PASS` with no P0/P1/P2.
The mandatory read/hash-only component gate then validated the exact installed
model and server, crossed the production execution-enabled builder and real Go
IPC path, and preserved the retained six-fact SQLite byte-for-byte without
starting Pi, llama-server, Provider, UI or any live canary. Focused and complete
Go normal/race plus static and Swift-normal gates pass. Swift ThreadSanitizer,
arm64 Release, final scope/authority reconciliation, immutable source lock and
fresh independent Implementation Review remain before any new live manifest.

```text
P2A-W2 = ACCEPTED
P2A-W3 = NATIVE LAUNCHER REPAIR VERIFICATION IN PROGRESS / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: final pre-lock audit added causal RED and repaired three same-contract
fail-closed build-attribution gaps: nested different reasons now close to
`build_unknown`, typed setup-native-auth attribution is preserved, and real IPC
server construction failure is `build_ipc`. The focused/repeated/component,
complete serialized Go normal/race, vet/module/format/diff/security and Swift
normal/ThreadSanitizer/arm64 Release gates all pass after the final source
change. Immutable source lock
`.loom-evidence/phase2a/P2A-W3/native-launcher-and-build-transaction-source-lock.json`
has SHA-256 `c28eacb6...71e6e7`; all enumerated files and locked authorities match.
Fresh independent Implementation Review against that exact lock is the current
gate. It authorizes no live action by itself.

```text
P2A-W2 = ACCEPTED
P2A-W3 = NATIVE LAUNCHER REPAIR SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: independent Implementation Review 1 verified source lock
`c28eacb6...71e6e7` and found no P0/P1, but returned `FAIL` for one P2 contract
evidence gap: setup-only MiniMax Test was not proven through real product
construction and Go IPC with execution initialization absent. That lock is
superseded and authorized no live action.

Repair 1 added the exact real-IPC credential Test proof with `Execution: nil`
and no local-model inputs. It produces one next-revision Journal terminal via a
deterministic missing credential-store read, makes zero Provider requests,
keeps the execution bundle nil, never creates the execution directory or any
execution fact, and cleans the socket. The new proof passes at `-count=20`.
Focused/component, complete serialized Go normal/race, static/security and
Swift normal/ThreadSanitizer/arm64 Release gates pass after the final test-only
change. New immutable source lock SHA-256 is
`f6bb68656cd70b2c9d75861bc5a77325ea488bd78dcfa49fc0c1cc41dd64d772`.
Fresh independent Implementation Re-review is the current gate; live remains
locked.

```text
P2A-W2 = ACCEPTED
P2A-W3 = NATIVE LAUNCHER REPAIR 1 SOURCE LOCKED / IMPLEMENTATION RE-REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Implementation Re-review 2 verified exact source
lock `f6bb6865...64d772` and returned `PASS` with no P0/P1/P2. The next permitted
step is to freeze and preflight exactly one wholly new setup-only MiniMax
lineage and one wholly new Pi saved-Team controlled-execution lineage. They
must be sequential, each one start with no retry, and neither consumed `-003`
lineage may be reused. No live process has started under the passing lock yet.

```text
P2A-W2 = ACCEPTED
P2A-W3 = IMPLEMENTATION RE-REVIEW PASS / NEW LIVE MANIFESTS PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: two wholly new `-004` live roots and manifests are materialized and
preflighted under the passing implementation lock. MiniMax manifest SHA-256 is
`889350d9...8c75d4`; Pi manifest SHA-256 is `0b76124d...0feaa`. Both roots,
SQLite states, binaries, signed native bundles, source-lock/review copies,
external identities, default socket/lock absence and resident exclusion pass.
No `-004` process has started. MiniMax must run first with zero local-model
flags; Pi remains locked until MiniMax is fully stopped.

```text
P2A-W2 = ACCEPTED
P2A-W3 = LIVE PREFLIGHT PASS / MINIMAX-004 UNCONSUMED / PI-004 LOCKED / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: MiniMax-004 consumed its one daemon start and one native `Test`.
The exact product surface displayed `Unavailable` and the Event Journal
authoritatively appended exactly one next-revision
`ProviderCredentialVerified` fact: revision `5`, status `rejected`, reason
`unavailable`. SQLite integrity, one-shot limits, no-execution boundary,
process/socket cleanup and non-disclosure pass.

Pi-004 was then consumed before product exercise by a Controller invocation
error: an extra non-manifest `daemon` positional argument caused exact loomd to
exit `2` with public stderr `invalid input`. SQLite remained byte-identical; no
daemon, socket, Pi, local model, preflight, Start or Provider action began.
Fresh independent Result Review reproduced result-lock SHA-256
`0a66ec71...f1fbdd`, returned `PASS` for evidence integrity, classified
MiniMax `PASS` and Pi `controller_invocation_error`, and correctly kept
walkthrough/commit/additional-live gates closed under the parent contract.

The bounded P2A-W3 Native Launcher Live Invocation Transaction Amendment is
now frozen at SHA-256 `f1ab059b...deb6a42`. It changes no product source or
authority and creates no W4. It proposes one wholly new Pi replacement only
after independent Amendment Review PASS, with a canonical exact executable +
32-element argv digest, no positional subcommand, one binary invocation and no
retry. Amendment Review is the current gate; no replacement root or process
has started.

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / MINIMAX PASS / PI CONTROLLER ERROR / INVOCATION AMENDMENT REVIEW PENDING / NO LIVE / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: MiniMax-004 remains a live product PASS. Pi-005 proved both source
and independent Verifier Run/Grant/Evidence lineages but stopped before
`WorkItemVerificationCommitted`, `TeamNodeAcceptanceCommitted` and
`TeamExecutionTerminal`. Independent Result Review classified the retained
evidence as trustworthy and the product `HUMAN_REQUIRED`.

The single P2A-W3 Authoritative Acceptance and Recovery Time contract, Repairs
1-3 and final independent Contract Review now close the complete time boundary
without a W4. Causal RED reproduced the Pi-005 stop under a successive-call UTC
clock. The implementation makes Work Authority own final acceptance and
recovery decision times, preserves exact replay before clock reads, binds UI
commands to stable time-independent intents, and keeps final timestamp-bearing
digests in Journal facts. Accepted and rejected advancing-clock paths pass.

Complete serialized Go normal/race, vet, Swift tests and Swift Release build
pass. Immutable source lock
`.loom-evidence/phase2a/P2A-W3/authoritative-acceptance-recovery-source-lock.json`
is frozen; fresh independent Implementation Review is the current gate. No
final live lineage, walkthrough, staging or commit is authorized yet.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ACCEPTANCE/RECOVERY SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the first Acceptance/Recovery Implementation Review returned one
P1: recovery replay validated a genuine decision but not the complete recovery
transaction. Repair 4 remained inside the same P2A-W3 and nine-file boundary.
Its causal RED proved that missing `TeamNodeAttemptScheduled` and missing
`TeamExecutionTerminal` facts were incorrectly accepted as exact replay.

New writes and replay now share one transaction constructor and bind every
recovery/downstream Event field, ordering edge and canonical payload. The two
REDs plus a mismatched-causation fixture pass; existing retry, terminal,
concurrency and idempotency tests remain green. Complete serialized Go
normal/race, vet, Swift tests and Swift Release build pass.

Repair 4 source lock SHA-256 is
`70e7b380f7eb40d1b978dee393c6f4ed21efe63e3df2afb091c7ee322f8c9954`.
Fresh independent Implementation Review is the only current gate. No live
lineage, product process, walkthrough, staging or commit is authorized yet.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 4 SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 4's first re-review found that exact recovery replay still
accepted an exact transaction followed by a duplicate scheduled-attempt or
Team-terminal fact. Both histories were captured as causal REDs. Team replay
now rejects duplicate logical node/attempt scheduling and every repeated Team
terminal, while preserving legitimate progress after a single retry schedule.

The full serialized Go normal/race matrix, vet, Swift tests and Swift Release
build pass again. Final Repair 4 source lock SHA-256 is
`3148271012989d1ebdd00d190587f730fb691444bfae9bd4f8f29fe861c981ea`.
A fresh independent final Implementation Re-review is the only current gate.
No live lineage, product process, walkthrough, staging or commit is authorized.

```text
P2A-W2 = ACCEPTED
P2A-W3 = REPAIR 4 FINAL SOURCE LOCKED / FINAL IMPLEMENTATION RE-REVIEW PENDING / NO LIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: final Repair 4 Implementation Re-review passed with no P0/P1/P2.
The one authorized Pi-006 lineage then consumed exactly one daemon invocation,
one native preflight and one Start through the ordinary New Mission surface.

Attempt 1 completed source and independent Verifier Evidence, committed a real
rejected acceptance, and entered one explicit Rules/Work-Authority bounded
recovery. Attempt 2 used wholly new source/verifier WorkItem, Run, Grant and
Evidence identities, committed an accepted outcome and exactly one
`TeamExecutionTerminal(succeeded)`. The native surface rendered
`Complete · Succeeded`, `1 node(s), 1 complete` and accepted Evidence
availability.

Final SQLite has `103` Events, integrity `ok`, one recovery, two acceptance
transactions, four distinct terminal Run/Grant/Evidence lineages and one Team
terminal. Read-only refresh appended zero facts. Socket/IPC lock/process/state
holder cleanup and bounded non-disclosure scans pass. Result-lock SHA-256 is
`d115c37db635b99879847f10169a59bbb872e80741b3c9b67329d533eb3ffeb8`.
Fresh independent Result Review is the current gate; walkthrough, staging and
commit remain locked until PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = PI-006 LIVE PASS / RESULT REVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the Pi-006 Result Review returned `PASS` with no blocking P0/P1/P2.
The first authorized no-terminal walkthrough then exposed a product defect
before acceptance: visible `Teams`, `Needs You`, and `Library` rail controls
were empty Swift closures, so the frozen History/Compare/Attention walkthrough
could not be completed. The diagnostic copy was stopped; it added only one
Runtime discovery metadata fact to its copied SQLite and no execution or
terminal fact. Pi-006 remains unchanged and is not retried.

The repair stays inside the existing five-file Swift W3 boundary. It adds real
Teams, Attention, and Library routes and renders Teams, authoritative Run
History, accepted Evidence counts, and two-Run Compare from the existing
snapshot only. Causal RED, focused rendering tests, complete Go normal/race,
vet, Swift tests/Release build, and diff checks pass. History resolves Runtime
display names and does not expose internal instance IDs. Source lock SHA-256 is
`f1cde4c072fa47bd0c526d79851c91a94fd64a6fc40eccdc50770b3cd8bb36ba`.
Fresh independent Implementation Review is the current gate. No replacement
live canary, walkthrough, staging, commit, or P2A-W4 is permitted before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = WALKTHROUGH NAVIGATION REPAIR SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Navigation Repair Implementation Review 1 returned `FAIL` with one
P1: partial, stale and offline-preserved snapshots could display a definite
empty Attention state and authoritative History/Compare wording. No
walkthrough or live action followed that review.

Repair 2 adds a closed presentation state derived from connection status plus
snapshot presence. Only an online snapshot can state `Nothing needs you`;
partial/preserved views explicitly warn that items may be missing, missing
snapshots are unavailable, and preserved Team records do not claim execution
availability. Causal degraded-state RED, 63 XCTest tests plus 4 Swift Testing
tests, Release build and diff checks pass. Existing complete Go normal/race/vet
results remain valid because no Go source changed. Final navigation source lock
SHA-256 is
`52409c9f398d1653519fa9026acd7ae00a7a6324881f6a508e09e57235199f76`.
Fresh independent Implementation Re-review is the current gate; walkthrough,
staging, commit and P2A-W4 remain locked.

```text
P2A-W2 = ACCEPTED
P2A-W3 = WALKTHROUGH NAVIGATION REPAIR 2 SOURCE LOCKED / IMPLEMENTATION RE-REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Navigation Repair 2 Implementation Re-review
reproduced source digest
`52409c9f398d1653519fa9026acd7ae00a7a6324881f6a508e09e57235199f76`
and returned `PASS` with no P0/P1/P2. Real rail routing, online-only definite
Attention emptiness, partial/preserved/unavailable wording, stale-Team
non-executability, neutral History/Compare headings, Runtime display-name
privacy, Mission continuity and snapshot-only reads all pass.

Exactly one wholly fresh no-terminal native/TUI walkthrough may now be
materialized. It must not enter objective text, request preflight, Start,
Provider Test, approval mutation, recovery mutation or any terminal command.
Staging and commit remain locked until the walkthrough, source/evidence lock
and final independent whole-Candidate review pass.

```text
P2A-W2 = ACCEPTED
P2A-W3 = WALKTHROUGH AUTHORIZED / NO TERMINAL ACTION / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the authorized walkthrough 002 completed every native read-only
surface, but the TUI then exposed raw Mission/Team IDs on its Board and retained
direct Run/Runtime/WorkItem/Evidence identifiers in Compare/Evidence rendering.
The root was stopped and is diagnostic only. Its copied SQLite remains
integrity `ok` with exactly `103` Events; no preflight, Start, Provider,
approval, recovery, cancel, terminal, or other execution action occurred.

A presentation-only safe-name repair is now in progress inside the existing
P2A-W3 Swift UI and Go TUI owned files. Causal RED reproduced the raw Mission
ID and missing reachable Compare surface; Swift RED proved the display-name
resolver was absent. Authority IDs remain internal command bindings and are
not rewritten. No new walkthrough, staging, commit, or P2A-W4 is authorized
until full verification, source locking, and independent Implementation Review
PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = WALKTHROUGH 002 DIAGNOSTIC FAIL / SAFE-NAME REPAIR IN PROGRESS / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the presentation-only safe-name repair passes focused Go TUI and
Swift UI tests, complete serialized Go normal/race matrices, Go vet, all 63
XCTest tests with one visual-export-only skip, all 4 Swift Testing tests, Swift
Release build, and diff checks. TUI Compare is now reachable; primary Mission,
Node, Team, Runtime, Evidence, Compare, Home, stale-view, and decision copy uses
safe display names or non-identifying fallbacks while exact authority IDs stay
inside command and fencing bindings.

The four-file source lock combined SHA-256 is
`b5d66e77a0e81f1b219afb4fb24dd3b801f109cb0582a878d86bb35c5446a730`.
Fresh independent Implementation Review is the only current gate. No root003,
staging, commit, or P2A-W4 is authorized before Review PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = SAFE-NAME SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Safe-Name Implementation Review 1 reproduced the lock but returned
`FAIL` with one P1 and one P2. Team Builder still displayed setup Runtime names
without comparing them to the Runtime instance ID, and the TUI fallback test
did not use the exact diagnostic Mission-title-equals-ID input. No walkthrough
followed that review.

Repair 2 adds both exact regressions and a fail-closed setup Runtime display
resolver. Focused TUI, complete serialized Go normal/race, vet, all Swift tests,
Swift Release build and diff checks pass again. The four-file Repair 2 source
lock combined SHA-256 is
`5e34e5f9aca0cc4e314fb8b2180c574c197d98a7719b12721894d55a178d73fb`.
Fresh independent Implementation Re-review is the only current gate; root003,
staging, commit and P2A-W4 remain locked.

```text
P2A-W2 = ACCEPTED
P2A-W3 = SAFE-NAME REPAIR 2 SOURCE LOCKED / IMPLEMENTATION RE-REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Safe-Name Repair 2 Implementation Re-review
reproduced source digest
`5e34e5f9aca0cc4e314fb8b2180c574c197d98a7719b12721894d55a178d73fb`
and returned `PASS` with no P0/P1/P2. Setup and snapshot Runtime fallbacks,
exact Mission-title-equals-ID regression, primary safe-name surfaces, reachable
read-only Compare, and unchanged internal command/fencing bindings all pass.

Exactly one wholly fresh no-terminal root003 native/TUI walkthrough is now
authorized. It must not enter objective text, request preflight, Start,
Provider Test, approval/recovery/cancel mutation, or terminal action. Staging
and commit remain locked until the walkthrough, final evidence lock, and fresh
whole-Candidate Review PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT003 WALKTHROUGH AUTHORIZED / NO TERMINAL ACTION / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: root003 was materialized fresh from the accepted 103-Event Pi-006
SQLite and opened by Computer Use. The first native Board immediately exposed
a third identifier shape: the read model populated Mission title with internal
`team_definition_id`, which is neither `mission_id` nor `team_instance_id`.
No click or product action followed. The app and daemon stopped; final SQLite
is byte-identical, integrity `ok`, and still 103 Events. root003 is diagnostic
only.

Repair 3 reopens only the rebuildable LocalProduct Mission read-model title and
its API tests in the same P2A-W3. Causal RED proves both matched-definition
display name and missing-definition non-identifying fallback. Journal facts,
authority IDs, schema, command/CAS/fencing behavior and client strict decoders
remain unchanged. Focused API/TUI tests pass; the full matrix and fresh review
are now required before any replacement walkthrough.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT003 DIAGNOSTIC FAIL / SAFE-NAME REPAIR 3 IN PROGRESS / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Safe-Name Repair 3 moves the final classification to the rebuildable
Mission read model. It resolves a matched TeamDefinition to the validated Team
name, falls back to `Saved team` when the definition cannot be validated, and
uses `Historical mission` when no Team exists. The real SQLite projection →
LocalProductReadService → Go IPC server → TUI test asserts the safe rendered
copy and explicitly rejects the internal TeamDefinition ID.

Focused API/TUI/real-IPC checks, complete serialized Go normal/race matrices,
vet, all Swift tests, Swift Release build, and diff checks pass. The eight-file
Repair 3 source lock combined SHA-256 is
`2a4f54fa1fb8758c76edded6e1f5ff1dbc9a23af4538182f3bdfb3261736b3df`.
Fresh independent Implementation Review is the current gate. No replacement
walkthrough, staging, commit, or P2A-W4 is authorized before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = SAFE-NAME REPAIR 3 SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Safe-Name Repair 3 Implementation Review
reproduced source digest
`2a4f54fa1fb8758c76edded6e1f5ff1dbc9a23af4538182f3bdfb3261736b3df`
and returned `PASS` with no P0/P1/P2. Validated Team-name derivation,
non-identifying fallbacks, real private-Go-IPC-to-TUI proof, client defenses,
and unchanged authority/fencing boundaries all pass.

Exactly one wholly fresh replacement no-terminal root004 native/TUI walkthrough
is now authorized. It must not enter objective text, request preflight, Start,
Provider Test, approval/recovery/cancel mutation, or terminal action. Staging
and commit remain locked until walkthrough PASS, final evidence lock and fresh
whole-Candidate Review PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT004 WALKTHROUGH AUTHORIZED / NO TERMINAL ACTION / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: root004 passed native safe-name surfaces and TUI Board, New Mission,
Team Builder, Runs / History, Compare, and Attention. Opening the selected
completed Mission then failed read-only with `state_unavailable`; navigation
stopped. The native app, TUI, and daemon were stopped. The final SQLite remains
byte-identical to Pi-006, integrity `ok`, exactly 103 Events, with socket/lock,
isolated processes, and open handles absent. root004 is consumed diagnostic
evidence only.

The cause was a Timeline reader contradiction: accepted `WorkItemRejected`
Events carry an attempt number on an already-authoritative work-item stream but
do not redundantly carry a logical-node ID. The reader incorrectly required
both payload fields when either appeared. Repair 4 now independently
corroborates every present field against stream + GlobalReadView lineage and
continues to reject missing binding, node/attempt mismatch, zero, and malformed
attempts.

Focused regression, current-source read of the unchanged root004 SQLite,
complete serialized Go normal/race matrices, vet, all Swift tests, Swift
Release build, and diff checks pass. The two-file Repair 4 source lock combined
SHA-256 is
`a41f59f9ab90911c95c34cbf1d2496cc91575af94a6953398191a51ad77137dd`.
Fresh independent Implementation Review is the current gate. No root005,
staging, commit, or P2A-W4 is authorized before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT004 DIAGNOSTIC FAIL / TIMELINE REPAIR 4 SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Timeline Repair 4 Implementation Review reproduced source lock
`a41f59f9ab90911c95c34cbf1d2496cc91575af94a6953398191a51ad77137dd`
but returned `FAIL` with one P1 and one P2. An explicitly present empty
`logical_node_id` could be treated as omitted, and the permanent regression
did not traverse the full safe-field → authoritative lineage → delivery mapper
path. root005 was not authorized.

Repair 5 remains inside the same P2A-W3 boundary. It must distinguish field
presence from empty value, reject every malformed present field, add a durable
authoritative mapping fixture for the real attempt-only rejection shape, and
close the root004 Inspector tabs that selected Plan/Changes/Evidence without
changing the rendered Team content. No schema, authority, mutation, live
action, commit, or P2A-W4 is permitted before RED, complete verification,
source lock, and fresh independent Re-review PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = TIMELINE/INSPECTOR REPAIR 5 IN PROGRESS / ROOT005 NOT AUTHORIZED / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Timeline/Inspector Repair 5 closes both independent Review findings
and the native placeholder defect without widening authority. Payload lineage
is still only corroborating metadata: an omitted field is allowed, while every
present node or attempt field must be non-empty, valid and exactly match the
lineage derived from the authoritative stream plus versioned GlobalReadView.
The permanent regression traverses the complete safe-field → lineage resolver
→ authoritative mapper path for the accepted attempt-only rejection and all
malformed-presence cases.

The native Team, Plan, Changes and Evidence Inspector tabs now render four
distinct safe read-only sections. Changes and Evidence are limited to exact
Team journal records, raw IDs and digests are not rendered, and unavailable
Timeline data is explicit rather than invented. Focused checks, current-source
read of the unchanged root004 SQLite, complete serialized Go normal/race
matrices, vet, all Swift tests, Swift Release build and diff checks pass.

The four-file Repair 5 combined source lock is
`d616d7ac1e5b98db887a65fcdffd02a14222f185767cd63cd56ef448ce12c405`.
Fresh independent Implementation Re-review is the only current gate. root005,
staging, commit and P2A-W4 remain locked before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = TIMELINE/INSPECTOR REPAIR 5 SOURCE LOCKED / IMPLEMENTATION RE-REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 5 independent Re-review reproduced source lock
`d616d7ac1e5b98db887a65fcdffd02a14222f185767cd63cd56ef448ce12c405`
but returned `FAIL` with one P1. Changes and Evidence could treat a Timeline
with an explicit stream gap or `has_more=true` as complete, presenting unknown
or unread later history as an authoritative empty result. All lineage fixes,
the real mapper fixture, read-only seam, safe complete-history sections and
unchanged authority boundaries passed. root005 was not authorized.

Repair 6 remains in the same P2A-W3 boundary. Strict negative fixtures now
cover gap and incomplete-page states for both Changes and Evidence. Those
sections render records or a truthful empty result only for the exact Team with
`gap=nil` and `has_more=false`; every other state is explicitly unavailable
with zero rows. No pagination, cursor mutation, schema, writer, command, CAS,
Grant, generation, Evidence, Projection authority or P2A-W4 was introduced.

Focused RED/GREEN, complete serialized Go normal/race matrices, vet, all Swift
tests, Swift Release build and diff checks pass. The four-file Repair 6 source
lock is
`473b4b454fa02b6f63c0b4d33c356862b60d8c91b819b935000ce80b6a3ab1e2`.
Fresh independent Implementation Re-review is the sole current gate. root005,
staging and commit remain locked before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = INCOMPLETE TIMELINE REPAIR 6 SOURCE LOCKED / IMPLEMENTATION RE-REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Repair 6 Implementation Re-review reproduced
source lock
`473b4b454fa02b6f63c0b4d33c356862b60d8c91b819b935000ce80b6a3ab1e2`
and returned `PASS` with no P0/P1/P2. Exact-Team complete history, stream-gap
and incomplete-page failure states, complete positive rows, Repair 5 lineage
guards and the read-only authority boundary all pass.

Exactly one fresh isolated root005 replacement native/TUI walkthrough is now
authorized. It is read-only and no-terminal: no objective entry, preflight,
Start, Provider Test, approval/recovery/cancel mutation or terminal action.
root004 stays consumed. Staging and commit remain locked until root005 passes,
final evidence is locked and a fresh whole-Candidate Review passes.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT005 WALKTHROUGH AUTHORIZED / NO TERMINAL ACTION / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh root005 replacement native/TUI walkthrough returned `PASS`.
The native Board, New Mission, Teams, Needs You, Library, Runtime & Providers,
completed Mission and all four Inspector tabs rendered distinct safe product
copy. The real Timeline remained incomplete on its first bounded page, so
Changes and Evidence truthfully rendered complete activity unavailable rather
than false empty/complete history. The TUI opened the same Mission without the
root004 `state_unavailable` failure and traversed Runs, Compare, Attention and
both bounded Timeline reads with human-safe labels.

No objective, preflight, Start, Provider Test, credential, approval, recovery,
cancel, terminal or other authority mutation occurred. The daemon completed
one no-write observation cycle. Final SQLite is byte-identical to Pi-006,
integrity `ok`, exactly 103 Events; socket/lock, root005 processes, open state
handles and isolation residue are absent. `demo-resident` remains untouched.
root005 is consumed and never reusable.

The final walkthrough result and result lock are now evidence-complete. A fresh
whole-Candidate source/evidence lock and independent Review are the only
remaining gates before staging and one atomic P2A-W3 commit.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ROOT005 WALKTHROUGH PASS / FINAL WHOLE-CANDIDATE REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent whole-Candidate Review 1 returned `FAIL` with two
P1 blockers and one P2 evidence gap. Native Timeline loads only the first 64
records, so the accepted 103-Event fixture cannot present complete Changes and
Evidence. The proposed final inventory also includes contract-excluded user
files and non-W3 evidence. root005 has native screenshots but no durable TUI
transcript.

Repair 7 remains inside the unique P2A-W3 and reopens only the already-owned
strict Swift read path, its tests and the existing Go-to-Swift contract test. It
freezes bounded eight-page/512-record aggregation, exact view/Team/Board/
Attention identity, cursor and delivery de-duplication, gap/conflict closure,
and cancellation/selection fencing. It introduces no writer, authority, schema,
daemon mutation, durable cursor, second read model or P2A-W4. Contract Review is
the current gate; no implementation, replacement walkthrough, staging or commit
is authorized before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 CONTRACT REVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: independent Repair 7 Contract Review returned `FAIL` with two P1
scope/identity blockers and one P2 proof ambiguity. The accepted Go cursor is a
canonical unpadded base64url stream-head envelope up to 32 KiB, while the Swift
client incorrectly applies the ordinary 256-byte identifier ceiling. The
contract also did not bind first-page nested Board/Attention identities, and a
static fixture handler could have satisfied its real-Go wording.

Contract Repair 1 precisely reopens `LocalIPCClient.swift` and its tests plus
the parent-owned daemon test. It freezes a cursor-specific 32 KiB grammar,
first-page and cross-page nested identity checks, and an exact SQLite Journal ->
LocalProductReadService/TeamExecutionStream -> localProductHandler -> Go IPC ->
Swift Client -> Store proof using an authoritative cursor above 256 bytes. No
production Go authority or protocol path is reopened. Repaired Contract Review
is the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 CONTRACT REPAIR 1 REVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 Contract Repair 1 Review accepted its owned scope, nested
identity rules and authoritative vertical proof, but returned `FAIL` with one
P1 canonical-cursor ambiguity. Alphabet/length/no-padding checks alone do not
reject invalid modulus or non-zero unused trailing bits that Go rejects by
decode/re-encode equality.

Contract Repair 2 freezes exact Go-equivalent transport validation: Swift may
temporarily RawURL-decode and canonical RawURL-re-encode only to require
byte-for-byte equality, while parsing no cursor contents and sending the
original bytes unchanged. Mandatory RED now includes invalid modulus and
re-encode mismatch. Repaired Contract Review is the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 CONTRACT REPAIR 2 REVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Contract Repair 2 Review returned `PASS` with no
P0/P1/P2. Go-equivalent canonical RawURL validation, original-byte transport,
ordinary-ID separation, nested page identity, bounded aggregation, fencing and
the exact authoritative vertical test path are implementable inside the
reviewed W3 boundary. Mandatory RED is now the current gate; no walkthrough,
staging or commit is authorized.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 MANDATORY RED / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 Mandatory RED is causal. The current Store reproduced 24
expected failures: one-page publication, gap and nested-identity acceptance,
unseen duplicate/cursor replay, missing eight-page bound and late selection /
cancellation publication. The cursor test separately fails only because the
frozen canonical 32 KiB validator is absent. Existing focused Store tests
remained green. Minimal implementation is now the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 IMPLEMENTATION / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 focused GREEN closes the first-page defect. Swift Store and
cursor tests pass 33/33; strict Go IPC fixture aggregation passes; and the real
SQLite Journal -> LocalProductReadService/TeamExecutionStream ->
localProductHandler -> Go IPC -> Swift Client -> Store path passes with a
greater-than-256-byte authoritative cursor returned byte-for-byte on page two.
The complete aggregate contains two Evidence records and one succeeded terminal
with no gap or remaining page. Full deterministic verification is the current
gate; no walkthrough, staging or commit is authorized.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 FULL VERIFICATION / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 complete deterministic verification returned `PASS`. Go
normal and race full-repository matrices, vet, module verification, Go format,
Swift 72-test package suite (one visual-only skip), Swift Release build, diff
and non-disclosure checks all pass. The exact eight-file source lock combined
SHA-256 is
`8520c8ab66cb4b85a4a06710db684a5452d037582d21231699c47123d0a0d040`.

Fresh independent Repair 7 Implementation Review is the current gate. No
replacement walkthrough, final inventory, staging or commit is authorized
before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 independent Implementation Review 1 returned `FAIL` with
P0 `0`, P1 `2`, P2 `0`. Production behavior and the exact locked source were
accepted, but the mandatory test matrix omitted several 512-record, nested
identity and generation-fencing branches. The authoritative Go-to-Swift proof
also did not compare Journal Events/heads before and after the Store probe or
assert the complete ordered unique delivery IDs. Test-only pagination padding
was explicitly found non-polluting.

Test Proof Repair 1 is frozen inside the reviewed Repair 7 boundary and reopens
only the Swift Store test, the Go product-daemon test, W3 evidence and this
status file. Production source remains locked. Focused and full verification,
an updated exact source lock, and independent Implementation Re-review are
required before any replacement walkthrough.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 TEST PROOF REPAIR 1 / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Test Proof Repair 1 reproduced a real original-contract defect:
Task cancellation fenced a late successful page but a late client error could
reach the generic catch and publish `offline/unavailable`. The original Repair
7 Store boundary is reopened only for the minimal `Task.isCancelled` fail-closed
guard in every error branch. The strict wire decoder remains unchanged; Store
schema defense is tested through an in-memory model. Focused verification is
the current gate.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 CANCELLATION ERROR REPAIR / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: Repair 7 Test Proof Repair 1 focused and complete verification now
passes. The 512/513 boundary, nested identities, cancellation late error,
Team/Mission/same-selection generations, exact ordered unique deliveries and
Journal Events/heads no-mutation proof are all executable. Full Go normal/race,
vet/module/format/diff, Swift 75-XCTest plus four Swift-Testing, and Swift
Release matrices pass. The refreshed exact eight-file combined SHA-256 is
`db6cc1a2a2e89496960de661edcd01f5faa8477e553d646ba24d9eca0c47eb47`.

Fresh independent Implementation Re-review is the only current gate. No
replacement walkthrough, staging or commit is authorized before PASS.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 REPAIRED SOURCE LOCKED / IMPLEMENTATION REREVIEW PENDING / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent Repair 7 Implementation Re-review returned
`PASS` with P0/P1/P2 all zero. It independently reproduced the exact lock and
closed every 512-bound, nested identity, generation, cancellation-error,
Journal no-mutation and ordered-delivery proof. Exactly one wholly fresh
isolated root006 read-only replacement native/TUI walkthrough is now eligible.

The walkthrough must use a private 0600 copy of accepted Pi-006 state, current
signed binaries and the real private product socket. It may perform one
no-write observation cycle and read-only navigation only; objective entry,
preflight, Start, Provider Test, approval, recovery, cancel and terminal writes
remain forbidden. It must preserve a durable TUI transcript, native Changes /
Evidence screenshots, byte-identical 103-Event SQLite, cleanup and resident
exclusion. Staging and commit remain locked pending Result Review and final
whole-Candidate Review.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL NATIVE TIMELINE REPAIR 7 IMPLEMENTATION ACCEPTED / ROOT006 WALKTHROUGH ELIGIBLE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: root006 manifest and preflight are frozen and pass. The root was
wholly absent, is private 0700 with no symlinks, and contains current locked
loom/loomd/native artifacts plus a 0600 byte-identical Pi-006 SQLite copy.
Integrity is `ok`, Event count `103`, product socket/lock/process/state handles
are absent, isolation is empty and `demo-resident` remains excluded. The single
read-only native/TUI walkthrough may now start.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL ROOT006 PREFLIGHT PASS / ONE READ-ONLY WALKTHROUGH ACTIVE / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: root006 single read-only walkthrough completed its product claim.
Native shows the complete succeeded Mission, complete Changes across both
Attempts and four Evidence rows; TUI durably records Board, Mission, Team
Builder, four Runs, Compare, Attention, Timeline and next-page traversal. One
daemon cycle was no-write. Final SQLite is byte-identical, integrity `ok`, 103
Events, and root006 socket/process/handle/isolation cleanup passes.

The unrelated demo-resident state was not targeted and remains old-mtime,
integrity `ok`, one Event, but its preflight PID was independently absent at
final cleanup. It was not restarted. Fresh independent root006 Result Review is
the current gate and must judge this disclosed liveness note. root006 is
consumed; no reuse, staging or commit is authorized.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL ROOT006 PRODUCT PASS / RESULT REVIEW PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: fresh independent root006 Result Review returned Evidence `PASS`
and Product `PASS`, with P0/P1/P2 all zero. The Reviewer reproduced every
locked hash, the native and TUI product surfaces, byte-identical 103-Event
SQLite, one no-write daemon result, private permissions, cleanup and
non-disclosure. The disclosed demo-resident liveness loss is not attributable
to root006 and is not a Candidate boundary violation. root006 is consumed and
must never be reused.

No further live action is authorized or required. The only remaining gates are
a corrected narrow final source/evidence inventory, fresh independent
whole-Candidate Review, exact staging inspection and one atomic local commit.

```text
P2A-W2 = ACCEPTED
P2A-W3 = FINAL ROOT006 RESULT REVIEW PASS / FINAL INVENTORY PENDING / NO COMMIT
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the corrected narrow final inventory locks exactly 48 P2A-W3
source/test files and 177 prior P2A-W3 evidence files. It excludes every
pre-existing Phase 1, plan, draft, Codex-config, root-document, generated-build
and external-live-root path. Fresh independent whole-Candidate Review 2
reproduced every source/evidence hash and returned `PASS` with P0/P1/P2 all
zero. Review 1's native pagination, contaminated inventory and missing durable
TUI-proof findings are all closed.

The Reviewer independently reran serialized whole-repository Go tests, the
full Swift package, the exact product/test diff check and the locked-source
non-disclosure check. The bound race, vet, module and Release matrices remain
applicable to the exact final source digest. Historical governance Markdown
retains its locked CommonMark hard-line-break bytes.
PX-01 through PX-18 are closed. P2A-W3 now satisfies its controlled execution
product and governance Exit Contract; no further live action is justified.

Final whole-Candidate Review 2 SHA-256 is
`08693a2594536b7212d0b4d6d5ad225e2e0477a6fd738ac400eb0b9dde21667d`.
Exact-path staging inspection passed with 228 Candidate paths, zero excluded
paths and no index/hash drift. The unique P2A-W3 is accepted and locally
committed as one atomic Candidate; no live action or P2A-W4 follows from this
acceptance.

```text
P2A-W2 = ACCEPTED
P2A-W3 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE
P2A-W4 = DOES NOT EXIST
```

`CURRENT`: the post-W3 Whole-Phase Controller reconciliation preserves every
historical failed canary and independently maps the current accepted product
to PX-01 through PX-18. PX-01 through PX-17 are Controller-classified `DONE`;
PX-18 has a complete root006 no-terminal journey but remains
`REVIEW_PENDING` until fresh independent Whole-Phase Review and explicit
Product Owner sign-off.

Fresh verification passed full serialized Go normal/race, vet, module/tidy,
projection/GlobalReadView/strict IPC, Swift normal/Release, installer,
reproducible native bundle and launch-smoke matrices. The first fresh Swift
TSAN run found a race in a W3-only suspended Timeline test fixture. It was not
waived: both test clients are now actor-isolated, focused/normal/full-TSAN
reruns pass with zero sanitizer warnings. No product authority, protocol,
Journal, Projection, Provider, Runtime or live allowance changed.

The four repository-wide gofmt findings belong only to the explicitly excluded
Cloud MCP commits `6473f80c..848f068c`; all 120 Phase 2A source/test/script
paths are clean and locked separately. No live canary was rerun. The next gate
is one fresh independent read-only Whole-Phase Review; Phase 2B remains locked.

```text
P2A-W1 = ACCEPTED CURRENT PRODUCT / HISTORICAL FAILED LIVE EVIDENCE PRESERVED
P2A-W2 = ACCEPTED
P2A-W3 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE
PHASE2A = WHOLE-PHASE CONTROLLER PASS / INDEPENDENT REVIEW PENDING
P2A-W4 = DOES NOT EXIST
PHASE2B = LOCKED
```

`CURRENT`: fresh independent Whole-Phase Review returned `PASS` with
P0/P1/P2 all zero. The Reviewer reproduced the 120-source and 463-evidence
digests, current shared module hashes, full Go and Swift matrices, zero-warning
Swift TSAN repair, historical failure preservation and the distinct PX-17
Codex/MiniMax/Pi product roles. PX-01 through PX-17 are independently `DONE`.
PX-18's root006 journey and review are `PASS`; only explicit Product Owner
sign-off remains.

No live action, staging, commit or Phase 2B contract freeze is permitted before
that sign-off. P2A-W4 does not exist.

```text
PHASE2A = INDEPENDENT WHOLE-PHASE REVIEW PASS / PRODUCT OWNER SIGN-OFF REQUIRED
P2A-W4 = DOES NOT EXIST
PHASE2B = LOCKED UNTIL SIGN-OFF
```

`CURRENT`: on 2026-08-03 the Product Owner explicitly approved the complete
Phase 2A sign-off after the fresh independent Whole-Phase Review PASS. PX-01
through PX-18 are now `DONE`; Phase 2A is closed and no P2A-W4 exists.

The Product Owner also authorized entry into Phase 2B contract governance.
Only the single vertical P2B-W1 `Side-task Handoff and Parent Decision` may be
frozen. Product implementation, Event/authority/schema changes and any
controlled canary remain locked until the roadmap amendment and P2B-W1
contract pass independent review and Mandatory RED is recorded.

```text
PHASE2A = ACCEPTED / PRODUCT OWNER SIGN-OFF COMPLETE
P2A-W4 = DOES NOT EXIST
PHASE2B = CONTRACT GOVERNANCE AUTHORIZED / PRODUCT WRITES LOCKED
P2B-W2 = DOES NOT EXIST
```

`CURRENT`: the Phase 2B Side-task Handoff Roadmap Amendment Repair 1 received
a fresh independent Plan Re-review `PASS` with P0/P1/P2 all zero. The accepted
planning boundary adds the single P2B-W1 to the `v0.2.0` release train without
making it a Phase 3A prerequisite or changing Phase 3A assets, Phase 3B
sandbox, v0.3 routing or v0.4 scheduling.

`PRODUCT-PLAN.md` and `TECH-PLAN.md` now record this target only. No product,
Event/schema, authority, Runtime/Provider, daemon or canary change has been
authorized by the plan edit. The next gate is a single exact P2B-W1 Contract
Review; implementation remains locked until that Review passes and Mandatory
RED is recorded.

```text
PHASE2B = ROADMAP AMENDMENT REVIEW PASS / P2B-W1 CONTRACT PENDING
P2B-W2 = DOES NOT EXIST
PRODUCT WRITES = LOCKED
```

`CURRENT`: P2B-W1 Contract Review 1 returned `FAIL` with no P0 and four P1
implementability gaps: reviewed-roadmap hash drift, synthetic child-Team
binding/reconstruction ambiguity, non-restart-closed parent continuation/
cancellation, and incomplete exact Event/IPC/deadline schemas. One P2
platform-ownership gap also affected Artifact read support.

Contract Repair 1 restores the exact reviewed roadmap bytes, freezes a
parent-binding/child-execution adapter and deterministic reconstruction,
defines a Journal-authorized parent-effect reconciler with explicit
ContextPacket consumption and recovered-flight cancellation, makes the wire
and Event schemas exact, and owns the Windows plus Mission snapshot tests.
Product code remains untouched. Fresh independent Contract Repair 1 Re-review
is the current gate; RED and implementation remain locked.

```text
PHASE2B = P2B-W1 CONTRACT REPAIR 1 RE-REVIEW PENDING
P2B-W2 = DOES NOT EXIST
PRODUCT WRITES = LOCKED
```

`CURRENT`: Contract Repair 1 Re-review found one remaining P1: its fail-closed
policy-reference behavior reduced the previously reviewed roadmap's claimed
standing-policy success path. Plan Repair 2 resolved the mismatch from current
code truth: Phase 2B v1 is explicit-confirmation-only, every policy reference
returns zero-write `capability_gap`, and a success path requires a separately
reviewed Rules capability with revocation, expiry and budget semantics.

Fresh independent Plan Repair 2 Review and Contract Repair 2 Re-review both
returned `PASS` with P0/P1/P2 all zero. The immutable P2B-W1 contract SHA-256 is
`2bc98c99c301a014d8dabe7491bd53287148c10803537b6de9d420492c22a866`.
The next gate is behavior-level Mandatory RED inside its exact owned files; no
live canary is authorized before Implementation Review PASS.

```text
PHASE2B = P2B-W1 CONTRACT REVIEW PASS / MANDATORY RED NEXT
P2B-W2 = DOES NOT EXIST
PRODUCT WRITES = TESTS-FIRST ONLY
```

`CURRENT`: P2B-W1 completed Mandatory RED, its single vertical implementation,
focused and whole-repository verification, and fresh independent Implementation
Re-review with P0/P1/P2 all zero. The one source-locked deterministic offline
authority canary passed and is consumed: it produced 296 Events with zero
duplicate Event IDs or idempotency keys, exactly one ContextPacket, parent
continuation, parent-effect completion and typed decision, plus 18 verified
content-addressed Artifacts. The real Go IPC-to-Swift read and TUI typed-decision
journey passed without Provider, credential or network use.

After manual Mac unlock, read-only native inspection exposed a same-boundary
presentation completeness defect. Mandatory RED, full verification and a
separate independent presentation Implementation Re-review closed it without
changing Journal, IPC, authority or schema. A supplemental source lock binds
the two Swift files while preserving the consumed-canary source lock. A
replacement visual-only Computer Use inspection then showed purpose, status,
summary, finding, Evidence/Artifact references, risk, usage, explicit empty
uncertainty and scope, next action and decision availability. It did not rerun
authority, dispatch work, open the product writer, use a Provider or access the
network.

Fresh independent Result Review returned `PASS` with P0/P1/P2 all zero and
reproduced the retained database/manifest/source hashes, SQLite invariants,
0600 screenshot/accessibility evidence and complete process cleanup. No further
live action is authorized or required. The only remaining gates are fresh
independent whole-P2B Candidate Review, exact-path staging inspection and one
atomic local commit.

```text
PHASE2A = ACCEPTED / PRODUCT OWNER SIGN-OFF COMPLETE
P2A-W4 = DOES NOT EXIST
P2B-W1 = RESULT REVIEW PASS / WHOLE-CANDIDATE REVIEW PENDING / NO COMMIT
P2B-W2 = DOES NOT EXIST
LIVE ACTION = COMPLETE / NO RERUN
```

`CURRENT`: fresh independent whole-P2B Candidate Review returned `PASS` with
P0/P1/P2 all zero. It reproduced the exact owned boundary, original and
supplemental locks, current Swift bytes, retained authority database and
manifest, Event uniqueness/cardinality, visual evidence and all explicit
exclusions. Focused Go, race, vet, strict IPC, restart, projection and TUI
checks passed. One older Pi metadata timeout seen only in a broader concurrent
package run passed in isolated reproduction and is not a P2B Candidate defect.

No further implementation, review repair or live execution is justified. The
only remaining actions are exact-path staging inspection and one atomic local
commit. `internal/projection/team_execution_test.go`, Phase 1 evidence, root
documents, Codex configuration, drafts and generated builds remain excluded.

```text
PHASE2A = ACCEPTED / PRODUCT OWNER SIGN-OFF COMPLETE
P2A-W4 = DOES NOT EXIST
P2B-W1 = WHOLE-CANDIDATE REVIEW PASS / EXACT STAGING NEXT
P2B-W2 = DOES NOT EXIST
LIVE ACTION = COMPLETE / NO RERUN
```

`CURRENT`: exact-path staging inspection passed with 72 Candidate paths, zero
excluded paths, a clean Go/Swift/JSON source check and no secret sentinel
matches. The final self-describing Candidate lock binds the other 71 sorted
path hashes, both canary-era source locks, independent Result and whole-
Candidate Reviews, retained authority hashes and native visual evidence.

The unique P2B-W1 is accepted by the atomic local commit containing this
record. No P2B-W2, further canary, Provider action, push or merge follows from
this acceptance. Unrelated dirty and untracked user files remain outside the
commit.

```text
PHASE2A = ACCEPTED / PRODUCT OWNER SIGN-OFF COMPLETE
P2A-W4 = DOES NOT EXIST
P2B-W1 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE
P2B-W2 = DOES NOT EXIST
LIVE ACTION = COMPLETE / NO RERUN
```

`CURRENT`: Phase 3A Goal is active at accepted baseline `6d380233`. Gate 0
repository identity and dirty-boundary checks pass, but the Entry Audit found
three critical prerequisites that are not yet closed: saved-Team exact Skill
revision data does not enter authoritative Run/Attempt lineage; current Pi
execution explicitly disables Skills and exposes no governed materialization
capability; and there is no correlation-complete real native-window plus PTY
shared-root journey substrate for the newly mandatory Cross-client Exit Gate.

Journal/CAS, immutable Artifact/Evidence, accepted terminal authority,
GlobalReadView and Credential Broker foundations are present and must be reused.
No product code was changed. A single bounded Entry Amendment is proposed to
reopen only exact asset lineage, private Runtime materialization and
correlation-only journey metadata/harness inside the one P3A-W1. It creates no
P3A-W2.

Product implementation, ADR/Exit Contract freeze and client journeys remain
locked until explicit Product Owner authorization and independent Amendment
Review PASS.

Read-only contract discovery has since narrowed the proposed reopening to the
saved-Team-to-Run exact asset lineage, Runtime catalog/Pi private
materialization, P3A product/API clients and correlation-only journey metadata.
The existing Journal Store, Evidence Store, Credential Broker, root policy and
accepted P2B-W1 remain closed. This discovery does not freeze the Amendment or
unlock product code.

The Product Owner has now authorized the single bounded Entry Amendment. The
normative `ENTRY-AMENDMENT.md` is frozen for fresh independent Contract Review.
It permits the later Gate 1 contracts to select exact owned files only from its
closed allowlist, maps strict P3A `journey_id` correlation to the existing
Journal `Event.CorrelationID`, and defines private atomic Pi Skill
materialization without removing `--no-skills` until compatibility is proven.
This freeze still grants no product edit, RED, migration, daemon launch,
materialization, journey, live canary, staging or commit authority.

Fresh independent Entry Amendment Contract Review 1 verified the exact frozen
SHA-256 `e0346dc41dc69629b3fa3598e8a5cec85b244cadfb9a7538a8698bad7bea545f`
and returned `P0=0`, `P1=0`, `P2=0`, Product/Authority `PASS` and
Operational/Trace Governance `PASS`. No file/schema/authority contradiction was
found. Gate 1 may now freeze the ADR, Phase 3A Exit Contract and exact single
P3A-W1 contract. Product code remains locked until that combined Contract
Review passes.

Gate 1 governance is now frozen as one Candidate: proposed ADR-0013, the Phase
3A Exit Contract and exact `P3A-W1 Versioned Evolution Asset Lifecycle and
Runtime Materialization` contract. The P3A-W1 contract selects only paths from
the reviewed Entry Amendment, explicitly excludes the pre-existing modified
`internal/projection/team_execution_test.go`, freezes Event/IPC/journey schema,
CAS/replay/migration/materialization rules, Mandatory RED, deterministic
verification, real shared-root GUI+PTY journeys, rollback and one atomic local
commit. Fresh independent combined Contract Review is required before RED or
any product edit.

Gate 1 Combined Contract Review 1 returned `P0=0`, `P1=4`, `P2=0`,
Product/Authority `FAIL` and Operational/Trace Governance `FAIL`. Product code
remains locked. The four contract repairs are: remove governance files from W1
product ownership; make authoritative time service/authority-derived only;
freeze authoritative Agent/Team/WorkPackage asset-binding facts and commands;
and replace Event/IPC/materialization/journey schema prose with exact canonical
field/stream/idempotency definitions before RED. Repair remains inside the
single P3A-W1 governance Candidate; no P3A-W2 is created.

P3A-W1 Contract Repair 1 is now frozen. It removes governance files from W1
product ownership, makes authoritative time internal-only, adds exact
Agent/Team/WorkPackage binding resolver/Event/action/projection/CAS semantics,
and freezes canonical JSON, stream/Event/idempotency formulas, every Event and
IPC action field, materialization manifest and journey evidence manifest. It
uses only the reviewed Amendment's existing/new source paths. Product code
remains locked pending fresh independent Re-review.

Contract Repair 1 Re-review returned `P0=0`, `P1=2`, `P2=0` and dual `FAIL`:
binding stream identity omitted scope identity even though Agent/Team IDs may be
reused across scopes, and the Event-code prose contradicted two literal mapping
rows. Repair 2 is now frozen with a full canonical subject identity digest and
binding stream, plus the mapping table as sole Event-code authority and golden
Template/RunPromotion Event IDs. No source ownership or authority expanded.

Fresh independent Repair 2 Re-review returned `P0=0`, `P1=0`, `P2=0`,
Product/Authority `PASS` and Operational/Trace Governance `PASS`. It independently
recomputed both golden Event identities and found no remaining repair. ADR-0013
is accepted. Gate 1 is closed successfully and the unique P3A-W1 may now capture
Mandatory RED. No implementation behavior, daemon, materialization, client
journey, staging or commit has occurred yet.

P3A-W1 Mandatory RED is now partially captured. Compile-clean failures prove
the missing asset domain/authority, strict journey wire, exact
ExecutionPlan/dispatch/Run lineage, GlobalReadView accessors, Pi conformance,
private materializer, application/API, production TUI and Swift surfaces. One
test-only missing import was excluded and rerun correctly. Lifecycle/CAS/
promotion/replay/materialization/security/template and client-parity behavioral
RED remains mandatory, so implementation is still locked.

```text
PHASE3A = GATE 1 PASS / MANDATORY RED PARTIAL
P3A-W1 = FROZEN / RED TESTS ONLY
P3A-W2 = DOES NOT EXIST
PRODUCT CODE = RED TESTS ONLY
CROSS-CLIENT JOURNEY = NOT AUTHORIZED
```

`CURRENT`: P3A-W1 implementation is complete on the same accepted baseline
`6d380233` (no new commits since). Mandatory RED is fully captured and the
unique P3A-W1 Candidate passed the focused/race/vet/tidy/format gates, the
deterministic matrix (Go full/race/vet/tidy/gofmt; Swift full/TSAN/Release),
an independent Implementation Review (P0=P1=P2=0) and an owned-path repair
review chain. Three post-final-review product changes were completed and
regression-tested: the content-addressed evaluation fixture unification
(`app.CanonicalEvolutionEvaluationFixture`, service/TUI/Swift shared digest
formula), the journey isolation-root recovery
(`recoverProductJourneyIsolationRoot`, fail-closed on foreign/symlink/
non-owned Pi metadata roots), and the Swift contract probe extension
(`--asset-action` with `stale_view`/`wrong_digest` variants plus public
`sha256Text`/`canonicalEvolutionAssetDigests`).

The mandatory Cross-client Exit Gate was amended by the frozen
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` (Product Owner instruction
2026-08-04, independent Review PASS): Computer-Use-driven real-window
automation is replaced by the production Swift client over the real daemon
socket, the real native window launched with `--socket --journey-id` and
captured with `screencapture`, and the real PTY TUI. The GUI evidence
surface is defined precisely by the frozen
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` (same PO instruction; the
production window is launched and connected per scenario with launch reads
in the IPC log, screenshots are real checkpoint captures, and
journey-specific GUI behavior is proven by the production Swift client
records). All eight scenarios are
frozen and verified PASS
(`.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md`):
happy lifecycle, cancel-reject-retain, stale-view-digest-generation,
concurrent-single-winner, crash-before-cas, crash-after-cas-before-response,
projection-failure-rebuild-reconnect and slow-client-redelivery-clean-restart.
Independent dual Result reviews returned Product Result PASS and Operational
and Trace Behavior PASS with P0=P1=0 and only documentation-precision P2
findings that are now closed
(`.loom-evidence/phase3a/P3A-W1-DUAL-RESULT-REVIEWS.md`, including the
final-generation closure addendum recording the eight final `-r2` roots, the
happy-root 498-event/492-correlated counts and the read-only re-verify PASS
on all eight roots). The Whole-Candidate Review returned PASS (P0=0 P1=0
P2=2) with its two documentation closures applied.

The unique P3A-W1 is accepted by the atomic local commit `7d5f0b01`
containing this record. The final root set, binary digests and evidence
bundle are recorded in
`.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md`. No P3A-W2,
further journey, push or merge follows from this acceptance. Unrelated dirty
and untracked user files remain outside the commit.

```text
PHASE3A = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE
P3A-W1 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE
P3A-W2 = DOES NOT EXIST
PRODUCT CODE = IMPLEMENTED / RED + DETERMINISTIC MATRIX PASS
CROSS-CLIENT JOURNEY = ALTERNATIVE VERIFICATION / 8/8 PASS
```

`CURRENT`: the v0.4.0 Agent Scheduling Framework / concurrent development
pipeline goal is active on the accepted P3A-W1 baseline `7d5f0b01`. Gate 0
passed (`.loom-evidence/agent-scheduling/GATE0-AUDIT.md`): physical cwd,
Git top-level, branch `codex/loom-platform-slice2` and HEAD agree with
`docs/CURRENT.md`; the dirty/untracked boundary is an exact exclusion list
(root docs, `internal/projection/team_execution_test.go`,
phase1-final-live-gate, earlier-slice evidence, `.codex/**`, `.loom-drafts/**`,
`apps/macos/.build/**`); the nine prerequisite capabilities are DONE
(Journal authority + AppendBatchIfStreamHeads, rebuildable Projection/
GlobalReadView, dispatch CAS, lease/generation fencing, Grant/Evidence
lineage, independent worktree/path isolation, failure-classification +
human_required lanes, real GUI/TUI cross-client substrate, Runtime
capability matrix).

Gate 1 is frozen and accepted as one combined Candidate:
`docs/adr/0014-agent-scheduling-framework.md`
(single-authority architecture, explicitly forbidding a second
Scheduler/Journal/StateWriter/Projection/queue DB; Timeline/Attention/
streaming are projections, never an authority),
`.loom-evidence/agent-scheduling/SF-EXIT-CONTRACT.md` (lanes + seven-class
failure taxonomy, at-least-once dispatch + idempotent CAS +
lease/generation fencing, fairness with repair aging, frozen schemas,
replay/restart semantics, authority boundaries, Decomposition Compiler
rules, bounded-recovery ownership in SF-W2, streaming/Timeline/Attention
ownership in SF-W3, mandatory RED, verification matrix, controlled self-host
canary, cross-client Exit Gate, exact staging + one atomic commit per
WorkItem, self-evolution gap-to-successor reconciliation, minimum
acceptance, stop conditions), and
`.loom-evidence/agent-scheduling/SF-WORKITEMS.md` (exactly three vertical
WorkItems SF-W1 Queue State/Projection + Admission/Eligibility/Conflict
Arbiter, SF-W2 Ephemeral Worker Pools + Lease/Reconciler + routing, SF-W3
Single-writer Integration + controlled canary + Timeline/Attention; no
SF-W4, no thin WorkItem). Cross-client journeys continue the accepted
alternative-verification method (production native window + real PTY TUI +
production Swift client over the real socket) per the Product Owner
instruction 2026-08-04; Computer-Use-driven window automation remains
skipped. Independent Contract Review 1 returned `FAIL`
(`GATE1-CONTRACT-REVIEW-1.md`, P0=0 P1=7 P2=10: lane/status state machine,
per-event payload schemas, GapProposal record, crash seam, dual-Scheduler
single-winner, SF-W1/SF-W2 TUI ownership, per-WorkItem restart journeys,
plus ten P2 precision items). `SF-CONTRACT-REPAIR-1.md` closed all seven P1
and all ten P2 findings against the amended text, and a fresh independent
Contract Re-review 2 returned `PASS` with P0=P1=P2=0
(`GATE1-CONTRACT-REVIEW-2.md`), including the previously missing
dual-Scheduler CAS single-winner RED (§10 #21) and minimum-acceptance
(§16 #13) items. The docs-only Gate 1 governance commit (ADR-0014 + Exit
Contract + WorkItems + Gate 0 audit + Review 1 + Repair 1 + Review 2 +
`docs/CURRENT.md`/`docs/adr/README.md` records) is made as one atomic local
commit. Product code remains locked until SF-W1 RED is established.

```text
V0.4.0 = GATE 0 PASS / GATE 1 CONTRACT RE-REVIEW 2 PASS / GOVERNANCE COMMIT
SF-W1/W2/W3 = ACCEPTED / NO PRODUCT CODE
SF-W4 = DOES NOT EXIST
PRODUCT CODE = LOCKED UNTIL SF-W1 RED
```

`CURRENT`: the v0.4.0 Agent Scheduling Framework slice is complete and
accepted:

- **Gate 1** (ADR-0014 + Exit Contract + SF-W1/W2/W3 freeze) passed combined
  Contract Review 2 after Contract Repair 1; governance committed
  `737c58c8`.
- **SF-W1** `1abf033c` — Journal-authoritative queue projection, Decomposition
  Compiler admission (eligibility, DAG cycle, duplicate active work,
  protected-authority fail-closed, vertical capability), owned-path/mutex/
  resource Conflict Arbiter, digest-bound Gap Proposal convergence.
- **SF-W2** `36b6c4b8` — ephemeral worker pools (one claim per worker, no
  oversell), attempt leases with generation fencing, Journal-rebuilt
  Reconciler (crash reclamation with before/after-CAS seam), bounded
  retry/backoff, seven-class failure routing with repair aging/fairness,
  Reviewer read-only.
- **SF-W3** `8e888565` + repair `ad9726a5` — single Integrator CAS,
  versioned release lifecycle (publish / later-Run adoption / rollback),
  one-shot offline canary with idempotent CAS, Timeline/Attention projections
  with authorized generation-bound streaming (old view preserved during
  controlled projection failure).
- Each WorkItem completed a real GUI+TUI cross-client journey (alternative
  verification: production Swift client over the real daemon socket, real
  PTY TUI, real native window, restart/reconnect checkpoint) with Product
  Result and Operational/Trace review records PASS (P0=P1=P2=0). Whole-slice
  acceptance 12/12 PASS
  (`.loom-evidence/agent-scheduling/WHOLE-SLICE-ACCEPTANCE-REVIEW.md`).
- Deterministic matrix green per WorkItem (Go full/race/vet/tidy/gofmt;
  Swift full/TSAN/Release); exact staging per each source-lock;
  `internal/projection/team_execution_test.go` and documented exclusions
  untouched; no push/merge/network/paid/user-config action.

```text
V0.4.0 = ACCEPTED / WHOLE-SLICE REVIEW PASS
SF-W1/W2/W3 = ACCEPTED / ATOMIC LOCAL COMMITS COMPLETE
SF-W4 = DOES NOT EXIST
PRODUCT CODE = IMPLEMENTED / DETERMINISTIC MATRIX + JOURNEYS PASS
```

`CURRENT`: the v0.4.1 product-line scheduling closure is active on the
accepted v0.4.0 baseline `7e24ec29` with the frozen
`.loom-evidence/agent-scheduling/V0.4.1-CONTRACT.md` (three delivery
boundaries W1 product-line closure, W2 two real parallel Candidates, W3
repeatable self-host canary; exclusions unchanged; no SF-W4). W1 is
complete and accepted by its atomic local commit: the real PTY TUI Queue /
Workers / Integration screens now carry genuine product operations
(`n` create Job via `queue_command create_job`, `c` claim,
`t` deterministic test success, `v` read-only Reviewer PASS,
`i` integrate / `a` adopt / `b` rollback), the production Swift client
reads the same projection over the real socket, and the integration
snapshot normalizes empty collections so Swift decodes the exact closed
shape (no null-array failure). The W1 cross-client journey
(`105723c8-0595-4f6d-9890-0b411f16cb01`) passed the frozen evidence schema
and `scripts/verify-v041-w1-cross-client-journey.sh` PASS with the exact
Journal event set, dual-client IPC, real PTY transcript, native-window
screenshot, restart/reconnect rebuild checkpoint and empty postflight.
Deterministic matrix green (Go full/race/vet/tidy/gofmt; Swift full
96+4). `internal/projection/team_execution_test.go` is owned by this
candidate as a required test-compilation repair: its two baseline call
sites fail to compile against the accepted `buildGlobalReadView` signature,
so they switch to the existing error-asserting `mustBuildGlobalReadView`
helper (test-only, no product behavior). Implementation / dual-Result /
Whole-Candidate reviews PASS with P0=P1=P2=0
(`.loom-evidence/agent-scheduling/V0.4.1-W1-*`). W2 (two genuinely
parallel development Candidates driven by the Loom scheduling flow) and W3
(repeatable controlled self-host canary) remain the current gates; no
push/merge/network/paid/user-config action.

```text
V0.4.1 = W1 ACCEPTED / ATOMIC LOCAL COMMIT COMPLETE
W2 = PENDING / TWO REAL PARALLEL CANDIDATES
W3 = PENDING / REPEATABLE SELF-HOST CANARY
PRODUCT CODE = W1 IMPLEMENTED / MATRIX + JOURNEY PASS
```

`CURRENT`: W2 of the v0.4.1 contract is complete and accepted by its atomic
local commit. Using the Loom scheduling flow itself, the framework-driven
journey (`63d4ea90-588d-4359-bb0c-942a2c767020`) drove two genuinely
parallel development Candidates: two Jobs admitted with distinct owned
paths; worker A claimed via the real PTY TUI while worker B claimed via the
production Swift client with A still open (two `AttemptClaimed` facts
before any result — the Journal proves two independent SubAgents executing
at the same time on separate Candidate branches/worktrees); worker B's
deterministic test failure (`product_defect`) was recorded before A's
development completed and the schedule Router routed it to the Repair lane,
where a fresh worker claimed the Job and succeeded; both Candidates
received read-only Reviewer verdicts PASS; the single Integrator landed
exactly one versioned release (the Swift client's competing integration was
rejected by CAS with zero events) and a later Run adopted it; the daemon
restart rebuilt queue jobs=2 / worker attempts=3 / releases=1 from the
Journal. W2 adds no product code — it exercises the shipped v0.4.0
framework plus the v0.4.1 W1 operations; `scripts/
verify-v041-w2-cross-client-journey.sh` PASS with the exact Journal fact
set and parallel/repair/single-writer assertions; Implementation /
dual-Result / Whole-Candidate reviews PASS with P0=P1=P2=0
(`.loom-evidence/agent-scheduling/V0.4.1-W2-*`). W3 (repeatable controlled
self-host canary) remains the current gate; no
push/merge/network/paid/user-config action.

```text
V0.4.1 = W1+W2 ACCEPTED / ATOMIC LOCAL COMMITS COMPLETE
W2 = ACCEPTED / TWO REAL PARALLEL CANDIDATES
W3 = PENDING / REPEATABLE SELF-HOST CANARY
PRODUCT CODE = UNCHANGED IN W2 / JOURNEY PASS
```

`CURRENT`: W3 of the v0.4.1 contract is complete and accepted by its atomic
local commit. The controlled self-host canary is frozen as a repeatable
product function (runbook `docs/runbooks/v041-w3-self-host-canary.md` +
`scripts/verify-v041-w3-self-host-canary.sh`) and demonstrated end to end
on the v0.4.1 product (`fbae6a98-3a88-44f7-8d65-67b62626d55b`): one offline
Run with locked runtime/model/skill bindings (`CanaryStarted` +
`CanaryCompleted`); a duplicate start rejected by the idempotent CAS with
zero new events (capacity 1 per canary stream, no oversell); with the
controlled projection fault active, a second canary committed to the
Journal while the snapshot preserved the old view, then rebuilt after the
fault cleared; daemon restart rebuilt the canary state from the Journal
with no duplicate facts; the real PTY TUI and the production Swift client
observed the same projection. W3 adds no product code; the verify script
PASSes with the exact binding, idempotent-rejection, fault-preserve and
rebuild assertions; Implementation / dual-Result / Whole-Candidate reviews
PASS with P0=P1=P2=0 (`.loom-evidence/agent-scheduling/V0.4.1-W3-*`).
All three v0.4.1 delivery boundaries are now closed; the whole-slice
acceptance review and final CURRENT.md record are the next gate. No
push/merge/network/paid/user-config action.

```text
V0.4.1 = W1+W2+W3 ACCEPTED / ATOMIC LOCAL COMMITS COMPLETE
W3 = ACCEPTED / REPEATABLE SELF-HOST CANARY DEMONSTRATED
WHOLE-SLICE ACCEPTANCE = PENDING
PRODUCT CODE = UNCHANGED IN W2/W3 / JOURNEYS PASS
```

`CURRENT`: the v0.4.1 whole-slice acceptance review PASSES
(`.loom-evidence/agent-scheduling/V0.4.1-WHOLE-SLICE-ACCEPTANCE-REVIEW.md`,
minimum acceptance 12/12, stop conditions no trigger, P0=P1=P2=0). All
three delivery boundaries of the frozen `V0.4.1-CONTRACT.md` are closed:
W1 `b607957f` (Queue/Workers/Integration product operations with the real
cross-client journey), W2 `27ef27b5` (two real parallel development
Candidates driven by the Loom scheduling flow), W3 `1fe5f8e8` (repeatable
controlled self-host canary with idempotent CAS, projection-failure
preserve and crash recovery). Deterministic matrix green at each product
checkpoint (Go full/race/vet/tidy/gofmt; Swift 96+4); each WorkItem has
its Implementation / dual-Result / Whole-Candidate review records PASS and
one atomic local commit; exclusions untouched; no push/merge/network/paid/
user-config action; no Computer-Use-driven window automation. The three
main-goal remaining items are closed; anything beyond the three v0.4.1
boundaries (e.g. Phase 3B-style routing or further product expansion)
requires explicit Product Owner authorization.

```text
V0.4.1 = ACCEPTED / WHOLE-SLICE REVIEW PASS
W1 = PRODUCT-LINE CLOSURE / ATOMIC COMMIT b607957f
W2 = TWO REAL PARALLEL CANDIDATES / ATOMIC COMMIT 27ef27b5
W3 = REPEATABLE SELF-HOST CANARY / ATOMIC COMMIT 1fe5f8e8
PRODUCT CODE = W1 IMPLEMENTED / W2+W3 DEMONSTRATED / NO PUSH
```

`CURRENT`: the formal product documentation set is published under
[`docs/product/`](product/README.md): capability matrix
(`product/CAPABILITY-MATRIX.md`), complete runbook index
(`product/RUNBOOKS.md`), TUI user guide (`product/TUI-GUIDE.md`) and native
app user guide (`product/NATIVE-APP-GUIDE.md`). They document the accepted
Slice 1/2, Phase 2A/2B/3A and v0.4.0/v0.4.1 capabilities with authoritative
WorkItem/commit ownership, verification evidence and client surfaces.

## Phase 3 extension completion (2026-08-06)

`CURRENT`: all Phase 3 extension WorkItems are implemented on
`codex/loom-platform-slice2` with atomic commits, deterministic matrices and
journey/canary verify PASS:

- W-RULES customer rules & standing policy — `46d522ff` (cross-client journey
  PASS, `83bfef4f…`).
- W-BRIDGE model bridge → execution adapter — `cf9360aa` (cross-runtime
  journey PASS; journey exposed and fixed a real JourneyID propagation defect;
  daemon transport needs the external Pi model binary — open item).
- Phase 3B governed sandbox — `7beab591` (RED 1-9 + loopback canary PASS;
  default off via `LOOM_SANDBOX_BACKEND`).
- W-AUTONOMY standing orders / Autopilot default-off bounded form —
  `89ec751e` Gate 1 + `975fd6a4` (RED 1-10 + journey PASS: define→human
  activate→2 bounded dispatches→budget stop→revoke block).
- Phase tree frozen at `.loom-evidence/execution-adapter/PHASE-TREE.md`
  (`cf56a3d1`); review notes and per-node open items in
  `.loom-evidence/execution-adapter/`.

`PARTIAL`: B-W1 and C-W1 production acceptance evidence is complete and
freshly re-run on the current branch (B-W1 `e515cc3f…`, C-W1 `5aa03cdf…`
cross-client journeys PASS; acceptance packages under
`.loom-evidence/execution-adapter/B-W1-ACCEPTANCE.md` and
`C-W1-ACCEPTANCE.md`), awaiting Product Owner confirmation; real activation
(C-W1 launchd/daemon activation) remains gated on explicit human approval.
Flash independent review remains an open item for every Phase 3 node
(channel unavailable; Controller cold-read + Product Owner Goal directive
used per established precedent).

`EXPERIMENTAL`: Phase 3B sandbox and W-AUTONOMY are default off; no behavior
change without explicit opt-in. No push/merge/activation/credential changes
were made.

## Phase 2C Chat-First Client Experience (2026-08-06)

`CURRENT`: Phase 2C development is active on the accepted v0.4.1 baseline
`7e24ec29`, branch `codex/loom-platform-slice2`; the Phase itself is not yet
accepted. ADR-0015
(`docs/adr/0015-chat-first-client-shell-with-governance-panels.md`) records
the three-pane shell decision: conversation as the primary lane, Agent-team
governance as an optional side panel, and no automatic Team/Mission/Run creation
from ordinary chat.

`HISTORICAL`: **P2C-W1 Workspace Shell & Entry** was recorded as accepted at
atomic local commit `4219ef62`; the Phase 2C repair round has reopened its
acceptance gates, so the results below are lineage context rather than current
acceptance evidence. The native macOS app and Bubble Tea TUI open into a
composer-first workspace rather than the Board-first Mission dashboard. The
left rail provides navigation, recents, Teams, Runtimes, Skills, and Library; the
center shows a blank composer, bounded recent work, and a prominent `Open
Folder…` affordance; the right governance panel displays Mission Board, Team,
Attention, and Runtime views. The visual token system (canvas, rail, surface,
raised, text-primary/secondary/muted, accent `#5E6AD2`, success, warning,
danger, offline) is centralized in `LoomGraphite.swift` for macOS and in
`style.go` for the TUI. Truthful connection states (`connecting`, `offline`,
`reconnecting`, `fatal`) are distinct and carry actionable recovery. No-folder
chat is a valid first-class journey. Deterministic matrix and cross-client
journey J1 (fresh launch) / J3 (offline recovery) / J10 (light/dark/compact
screenshots) pass. No Event Journal, one-writer, Projection, Scheduler, policy,
Grant, or daemon authority change.

`HISTORICAL`: **P2C-W2 Chat-First Conversation** was recorded as accepted at
atomic local commit `449ab719`; its repaired behavior remains `PARTIAL` until
the current cross-client journeys and review gates pass. The center conversation
surface at that historical checkpoint included:
- Go daemon: `internal/api/local_product_chat.go` provides in-memory thread
  isolation, a 4 096-character content bound, and role-based responses. Plain
  messages return a `loom` role with no side effects; explicit agent triggers
  (`use agent`, `agent team`, `team` + `mission`) return a `proposal` role and
  a tentative confirmation prompt. No Journal writes occur from chat.
- IPC wiring: `chat_thread` and `chat_message` methods are added to the local
  IPC handler, `LocalProductReadService`, and `LocalIPCClient`/`LocalProductStore`.
- macOS: `LoomWorkspaceShell.swift` renders a real message timeline when messages
  exist and keeps the welcome/recent cards when empty. The composer is always
  visible, supports multi-line input, and sends via the arrow button or explicit
  action. `Use Agent Team` is the explicit trigger for Agent mode.
- TUI: `ScreenHome` is the default screen with a full-width composer band,
  message history, `i` to draft, `enter` to send, `u` for Agent Team, and `g b`
  for the Board. ANSI semantic colors from `style.go` are applied across all TUI
  views.
- RED tests: `local_product_chat_test.go`,
  `LocalProductStoreTests.swift`, and `model_test.go` prove that ordinary chat
  does not create Team or Mission facts and that explicit Agent triggers require
  confirmation.

Historical verification: `go test ./...` green, `swift test` 103/0/1 green,
`go vet ./...` quiet, P2C-W2 files `gofmt` clean, Swift TSAN/Release build
green. These results do not satisfy the reopened repair gates. No
push/merge/network/paid/user-config action occurred.

`HISTORICAL`: P2C-W3 was previously recorded as the next implementation gate.
That checkpoint predates the repair round below and does not describe the
current Candidate or authorize acceptance.

### Phase 2C Repair Round (2026-08-08)

`CURRENT`: whole-product review reopened Phase 2C acceptance while preserving
the historical P2C-W1 (`4219ef62`) and P2C-W2 (`449ab719`) commits and their
evidence. The reviewed repair matrix is at
`.loom-evidence/phase2c/contracts/PHASE-2C-REPAIR-AMENDMENT.md`; independent
Review 2 reported P0=P1=P2=0 for the first repaired lock, and Repair 2 Contract
Review reports P0=P1=P2=0 for the test-fixture correction. Work remains in
W1/W2/W3, plus narrow Queue admission and provider test-fixture prerequisites
required to make the whole-repository gate deterministic without changing
Scheduler, Provider, or authority semantics.

`PARTIAL`: the repair Candidate removes the fake echo, derives stable opaque
thread IDs per task, persists bounded conversation projections across daemon
restart, opens a native folder picker, and presents one sanitized service state.
The native shell is conversation-first with a compact left rail and a
hidden/visible/pinned governance inspector. At 1080pt and above, opening the
inspector contracts the rail so conversation and governance remain operable
side by side; below 1040pt it becomes a dismissible overlay. Runtime & Providers
routes to the existing management surface. The TUI has equivalent wide split,
narrow stack, folder selection, panel continuity, and capability-gated actions.

`PARTIAL`: the daemon Candidate injects a concrete tool-disabled Pi RPC
conversation adapter when the validated local runtime tuple is configured. It
uses bounded untrusted history, strict transcript validation, cancellation and
private process roots; tool-call events fail closed. Conversation and governed
mission execution share one lazily started daemon-owned local-model process.
Ordinary configured chat returns tentative text without Supervisor frames,
Grant authority, or Event Journal facts. Missing runtime remains a truthful
recoverable state. Fresh source-lock-bound verification is pending.

`PARTIAL`: UI repair now exposes the native Agent Team builder, New Mission from
both rail and composer, distinct Topology and Timeline views, actionable
Decisions and Attention rows, and a real Evidence-to-Mission path. The TUI now
offers first-class Teams, Evidence, and Runtimes views and submits only exact
daemon-prepared Mission Decision actions, defaulting to the first prepared
action instead of the highest-authority choice. Unsupported actions and the
previous synthetic approval page are hidden; native `Edit scope` is also hidden
until a real editor exists. Focused Swift UI and TUI tests pass, but
those runs predate the new source lock and are not acceptance evidence.

`PARTIAL`: pre-lock diagnostic runs exercised configured conversation IPC,
shared model ownership, restart-safe thread identity, daemon cleanup retry,
Queue duplicate-work admission, and exact-path socket reclaim. Their results
guided the repair but are intentionally not promoted to current acceptance
claims. The complete Go/Swift/TSAN/Release matrix must be rerun after a fresh
Contract Review PASS and regenerated source lock.

`PARTIAL`: the first lock-bound serial Go normal matrix passed completely. The
full race matrix then failed once in
`TestSystemCodexStatusRunnerBoundsOutputAndCancelsProcessGroup`: the test saw a
PID file after shell redirection created it but before `printf` wrote the PID,
then reported `EOF`. Production cancellation returned `context.Canceled`; a
focused race repetition passed 50/50. Repair 2 makes only the test fixture's PID
publication atomic and adds that file to the Phase verification prerequisite.
The failed attempt remains preserved; Repair 2 Contract Review reports
P0=P1=P2=0, and a final status-only re-review plus new lock precede all reruns.

`PARTIAL`: Repair 2 completed its regenerated source lock, full normal/race/vet/
tidy/format matrix, Swift/TSAN suite, Release build, and independent contract
re-reviews. The first subsequent native-plus-real-PTY attempt then found a
product blocker: TUI Home rendered `i message` and `u Agent Team`, but its key
handler implemented neither action. Attempt 001 stopped with zero Team/Mission
facts and clean shutdown; its failure is preserved under
`.loom-evidence/phase2c/journeys/repair-2026-08-08/attempt-001/FAILURE.md`.
Repair 3 added causal RED tests and the minimal typed-client key routes; focused
tests and the full TUI package are GREEN. Fresh independent Repair 3 review
returned `P0=0`, `P1=0`, `P2=0`. A new lock and complete deterministic matrix
precede any replacement journey.

`PARTIAL`: Repair 3's new lock and complete Go/Swift/TSAN/Release matrix passed.
Replacement attempt 002 proved Home `i` operational, then showed that physical
spaces disappeared from real PTY text input because `tea.KeySpace` was ignored
by the shared entry parser. The attempt stopped before message submission with
zero Team/Mission facts and clean shutdown; its failure is preserved under
`journeys/repair-2026-08-08/attempt-002/FAILURE.md`. Repair 4 has a causal
physical-key RED and minimal bounded parser fix; focused, full TUI, and race
tests are GREEN. Fresh review, lock, and deterministic matrix precede attempt
003.

`PARTIAL`: the full J1-J10 native-plus-PTY rerun and independent
implementation/contract/dual-Result/whole-Phase reviews remain open. The frozen
journey contract requires one clean shared daemon fixture, restart comparison,
screenshots, TUI transcripts, and accessibility evidence. No automated Phase 2C
journey harness exists in the repository, and Computer-Use automation remains
skipped until explicitly reauthorized. Therefore Phase 2C and ADR-0015 are not
accepted yet.

`TARGET`: rerun J1-J10 against one clean daemon fixture across native and TUI
clients, capture the required redacted IPC/restart/visual/accessibility evidence,
complete independent reviews, then request Product Owner acceptance. Runtime
absence remains a supported recoverable state.

`PARTIAL`: Phase 2C Repair 6 Candidate boundary is recorded at
`.loom-evidence/phase2c/repair-candidate-boundary.md` on physical repository
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`, branch
`codex/loom-platform-slice2`, baseline HEAD `651f156a`. The candidate inventory
has 45 paths including Journey Manifest, Queue/provider verification
prerequisites, the Go/Swift long-operation deadline policy, the native chat
wire model and tests, and its
self-describing lock; all pre-existing Phase 1, root-document, `.codex`,
`.loom-drafts`, build-product, and other user changes remain excluded. Repair 4
Review and status-only re-review returned `PASS`, its regenerated lock and full
Go/race/Swift/TSAN/Release matrix passed, and replacement attempt 003 proved
physical spaces render correctly in a real PTY. That attempt then exposed that
`chat_message` completed in 5.270493 seconds after the five-second server/TUI
deadlines; its failure record preserves three initialization/runtime facts,
zero Team/Mission/Run facts, zero duplicates, integrity `ok`, and clean
shutdown. Repair 5 classifies exactly `credential_verify`,
`mission_execution`, and `chat_message` as ten-second operations across Go
server, Go/TUI client, and Swift client while ordinary methods remain five
seconds. Repair 5's final 43-path ordered source digest, independent byte review,
complete Go normal/race/vet/tidy/format matrix, Swift/TSAN suites, and signed
Release build passed. Replacement attempt 004 then passed real-PTY J1/J2 and
the native system folder picker, but native `chat_message` encoded `threadID`
instead of the daemon-required `thread_id`; five GUI requests failed strictly
with `invalid_request` while both TUI requests passed. The attempt preserves
three initialization/runtime facts, zero Team/Mission/execution-Run facts, zero
duplicates, integrity `ok`, no sentinel leak, and clean shutdown. Repair 6 is
limited to the missing Swift wire-key mapping and exact-shape regression test.
Fresh RED/GREEN, independent review, expanded lock, deterministic matrix, and a
new clean attempt 005 precede any Phase acceptance, staging, commit, or push.

`CURRENT`: Repair 6 subsequently passed its causal Swift wire-key test,
independent review, regenerated 44-path ordered source digest
`2afe8693ac07ed12d21fcc981485c9e8d1f2b7a61cde519c87238294cbd31971`,
complete Go normal/race/vet/tidy/format matrix, Swift/TSAN suites, and Release
build. Attempt 005 passed native and real-PTY J1-J4 plus J5 Team confirmation
and read-only Mission preflight. Its explicit `Start` then returned
`state_unavailable` after `5.054680` seconds while the daemon-owned flight
continued. The same Journey later committed eight execution facts through
`TeamReadySetDispatched`, projected the Team as `running`, and stopped without
an issued Grant, terminal Run, finalized Evidence, or Team terminal/recovery
fact. Attempt 005 is preserved at
`.loom-evidence/phase2c/journeys/repair-2026-08-08/attempt-005/FAILURE.md` with
14 unique Journal facts, integrity `ok`, no sentinel leak, exact source-lock
revalidation, and clean shutdown. No partial result is promoted.

`CURRENT`: Repair 7 reopens the Phase 2C source boundary. Its P0 contract
forbids a retry-shaped Mission start error while the same hidden flight can
later write authority facts. The implementation must project the existing
authoritative plan/dispatch lineage before slow local-model initialization;
initialization then runs inside the authorized Supervisor lifecycle so failure
or cancellation terminalizes Run, Grant, Evidence, and Team state. Raising
timeouts or treating an in-memory flight as authority is explicitly
insufficient. Repair 7 also owns the P1 stale cross-client Team Draft recovery
observed when the TUI confirmed ahead of Native, and the P2 presentation of
tool-shaped tentative model prose as untrusted non-actionable output. Causal
RED tests, bounded GREEN, independent contract review, a new lock, full matrix,
and clean attempt 006 precede any resumed J1-J10 acceptance claim.

`CURRENT`: Repair 7 implementation now defers shared local-model startup until
the Supervisor enters an already projected WorkItem/Run/capacity/capture/Grant
lineage. A blocked-starter test proves `StartMission` returns authoritative
`running` before release; controlled startup failure then exhausts two bounded
attempts with paired Run terminal, Grant revocation, Evidence, recovery, and one
Team terminal. Caller cancellation before first projection now cancels and
joins the flight, detached-refreshes any resulting authority, and removes a
zero-fact completed flight so an exact retry is possible. Native and TUI stale
draft confirmation each submit once, discard the old revision, refresh once,
and require an explicit new draft. Tool-shaped tentative chat is replaced by
the fixed non-actionable warning in the API and both client render paths.
Focused Go and Swift tests are green. Final-byte independent review, the new
51-path Repair 7 source lock, complete deterministic matrix, and clean attempt
006 remain required; Phase 2C and ADR-0015 remain `PARTIAL`.

`CURRENT`: Repair 7 independent review passed with `P0=0`, `P1=0`, `P2=0`,
and its 51-path lock was generated. The lock-bound serial Go normal matrix
passed in `414.56s`. The subsequent race matrix failed in
`TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage` after a
shared five-second detached Team outcome context expired during a later
Evidence commit. Repair 8 gives each attempt its own bounded 15-second capture
and receipt commit budget; one attempt can no longer consume another's cleanup
window. The failed race run remains evidence only and invalidates the Repair 7
lock. Focused normal/race verification, independent re-review, a new lock, and
the complete replacement matrix precede attempt 006. Phase 2C remains
`PARTIAL`.

`CURRENT`: Repair 8 independent review passed, its regenerated 51-path source
lock bound ordered digest
`e64e0ba41e84b04d0d0255497d94521383aaeebc8327845896d388435e49b745`,
and the complete Go normal/race/vet/tidy/format, Swift, Thread Sanitizer, and
signed Release matrix passed. Attempt 006 proved native and real-PTY chat-first
entry, safe tentative conversation, explicit Team confirmation, explicit
Mission start with complete governed terminalization, and exact Journal-backed
restart recovery. Its controlled J6 authoritative action failed closed with
`invalid_request`: the TUI sent operation `decide`, but the service contract
accepts prepared authoritative actions only as `submit`. The attempt also
exposed that typed `not_found` after daemon-restarted Team Drafts lacks the
existing stale-draft recovery, and that a successful TUI Mission start can be
overwritten by `preflight_expired` during its immediate snapshot refresh.
Attempt 006 remains failed history with unique Journal identities, integrity
`ok`, no sentinel leak, exact Repair 8 digest revalidation, and clean shutdown.

`CURRENT`: Repair 9 is the active Phase 2C boundary. It is limited to the
already-listed TUI and Swift Store/test paths. It must submit the exact prepared
Mission action with operation `submit`; enforce Native prepared-action and
authoritative identity-matched result boundaries; recover both `conflict` and
`not_found` Team Draft confirmation with one discard, one setup refresh,
actionable copy, and no retry; and consume a successful Mission preflight in
both clients before snapshot refresh so accepted `running` state is neither
replayed nor mislabeled as expired. The first independent review returned
P1=2/P2=1; Re-review 2 returned P1=1/P2=1 for Native pre-start expiration and
missing TUI repeated-start coverage. Both reviews are preserved, their causal
RED tests and bounded fixes are now green, and the related Go TUI and 36 Swift
Store tests pass. A passing final re-review, replacement source lock, complete
matrix, and clean Attempt 007 precede any Phase 2C or ADR-0015 acceptance claim.

`CURRENT`: Repair 9 final re-review passed with no P0/P1/P2 findings. Its
replacement 51-path lock bound ordered digest
`15696d1f7f59d3e64a80839e4021972a8661bacae5b6e115057ee581b18d5867`;
the complete Go normal/race/vet/tidy/format, Swift, Thread Sanitizer, and signed
Release matrix passed. Controlled J6 then committed real `ApprovalDecided` and
`WorkItemApprovalResolved` facts through the TUI's exact `submit` path.
Attempt 008 proved J1-J5 and J8, including one-shot `not_found` Draft recovery,
accepted `running` continuity with inert repeated Start, governed execution
terminalization, no folder-sentinel leak, and byte-identical restart
projection. Its J7 setup correctly showed that probe absence does not imply
offline and therefore wrote no status fact.

`CURRENT`: Repair 10 now provides the missing deterministic J7 authority path.
Its strict Phase 2C-only private manifest drives one existing Runtime discovery,
baseline, reconciliation, prepared committer, and Event Journal transition from
online to offline before IPC readiness. Exact post-commit daemon restart is a
validated zero-append replay; any manifest, Event, payload, projection, source,
path, permission, identity, or time drift fails closed. Omitting the fixture and
restoring ordinary Pi observation authors the offline-to-online recovery fact.
Causal focused normal/race tests and full `cmd/loomd` normal/race tests pass.
Independent Review 1 returned `P1=2/P2=1`, Re-review 2 reduced this to `P2=1`,
and Final Re-review 3 returned `PASS` with `P0=P1=P2=0`. The regenerated source
lock now authorizes a fresh complete deterministic matrix; that matrix and a
clean replacement journey remain required. Phase 2C and ADR-0015 remain
unaccepted.

`CURRENT`: Repair 10 subsequently passed its regenerated 51-path source lock,
complete Go normal/race/vet/tidy/format, Swift, Thread Sanitizer, and signed
Release matrix. Attempt 009 proved J1-J8, including exact Runtime
online-to-offline authority, ordinary offline-to-online recovery, blocked
Mission independence, byte-identical restart projection, and `408/408/408`
unique Journal identities with integrity `ok`. A real signed-Release J10 matrix
was captured in light and dark at 900/1080/1440 points. Independent final
journey/UI review nevertheless returned `FAIL`: TUI Attention `r` refreshed
permission attention while rendering stale product `snapshot.Attention`, and
native J9 lacked live accessibility/keyboard traversal. It also admitted split
TUI Journey traceability and generic native Recent labels as P2 findings.
Attempt 009 is preserved as failed history.

`CURRENT`: Repair 11 is the active Phase 2C acceptance boundary. It is limited
to the already-listed TUI model/test and Swift Store/test paths. Attention must
refresh both rendered sources in place; Recent attention items must expose safe
existing action context; the replacement harness must supply one explicit main
Journey UUID; and the signed Release must pass direct live macOS AX/keyboard
traversal plus a recaptured J10 matrix. Exact contract/boundary review, causal
RED, minimal GREEN, source re-review, a new lock, complete matrix, and clean
J1-J10 replacement precede any Phase 2C or ADR-0015 acceptance claim. Repair 11
contract re-review passed with `P0=P1=P2=0`; causal RED is the current gate.

`CURRENT`: Repair 11 causal RED reproduced both defects. TUI Attention `r`
returned only one permission-attention command instead of the required
two-source refresh. Native Store mapped both an escaped
`restore_runtime` action and an empty-action `inspect_failure` item to the same
generic title. Native Recent now maps bounded `SafeText` action context with a
deterministic kind fallback. The first independent implementation review found
a TUI P1: the initial two-command batch could expose or retain stale permission
actions before both rendered sources settled. The remediation returns one
generation-bound aggregate, keeps loading true until both reads settle, ignores
older completions, and removes permission actions on failure while preserving
the last product snapshot. Tests cover `r`, Tab, Shift-Tab, no permission
capability, both failure modes, and overlapping completions; full
`internal/tui` and focused race checks pass. No IPC, Journal, authority,
execution, persistence, or filesystem boundary changed. Independent Repair 11
source re-review is the current gate; no source lock or journey is authorized.

`CURRENT`: Repair 11 source Re-review 2 found one remaining TUI P1. Loading hid
old permission rows visually, but the in-flight model still allowed `g`, `a`,
or `x` to consume the cached actions. Attention refresh now removes permission
actions at generation start, and a focused normal/race regression proves those
keys cannot create a command until the current aggregate settles. A final
independent source re-review is required; the Repair 10 lock remains invalid
and no replacement journey is authorized.

`CURRENT`: Repair 11 final independent source Re-review 3 passed with
`P0=P1=P2=0`. Attention refresh is now one generation-bound aggregate, removes
stale permission actions before loading, ignores old completions, and closes
both failure paths without inventing authority. Native Recent uses bounded
action context with kind fallback. Focused, full TUI, adjacent CLI, race, and
Swift Store checks are green. Final status-byte review and a new source lock
must precede the complete deterministic matrix; no J1-J10 replacement journey,
ADR-0015 acceptance, or Phase 2C acceptance is authorized yet.

`CURRENT`: Repair 11 final status review returned one P2 for stale Candidate
Boundary Purpose wording. The header and all other current records were
correctly bounded. The Purpose now marks contract review, causal RED, GREEN, and
source review complete, leaving the replacement lock, deterministic matrix, and
clean journey pending. Exact-byte status re-review is the current gate.

`CURRENT`: Repair 11 status Re-review 2 and source-lock review passed with
`P0=P1=P2=0`; the 50-path lock bound digest
`b85242621bbd18719399db5108de3c1701f9edbbc67d49da416bda2b65f5df39`.
The first lock-bound Go normal matrix then failed only in the real Go-to-Swift
contract probe because its explicit compile unit includes
`LocalProductStore.swift` but not the Store's new `SafeText.swift` dependency.
The lock is invalidated. Remediation remains inside the declared Store path via
an equivalent private bounded sanitizer; focused cross-language and Swift
verification, independent source re-review, a new lock, and a complete fresh
matrix are now required. No Journey is authorized.

`CURRENT`: Repair 11 remediation subsequently passed independent review, a
replacement 50-path lock, the complete Go normal/race/vet/tidy/format, Swift,
Thread Sanitizer, and signed Release matrix. Attempt 011 proved J1-J8 with one
explicit main Journey UUID, exact prepared-decision facts, authoritative
Runtime offline/online recovery, in-place Attention refresh, and byte-identical
restart state. Live signed-Release J9 then found that Space opened the uniquely
named governance inspector but Escape did not close it. Repair 12 adds only a
tested native governance dismissal transition and `onExitCommand` routing.
Focused tests and independent source review pass with `P0=P1=P2=0`; the source
lock is invalidated. A regenerated lock, complete matrix, fresh signed Release,
and clean J1-J10 replacement remain required. Phase 2C and ADR-0015 remain
unaccepted.

`CURRENT`: Repair 12 subsequently passed its replacement source lock, complete
Go/Swift/Thread Sanitizer/signed-Release matrix, and live governance
Space/Escape verification. Attempt 012 proved J1-J8, exact Journey UUID
correlation, one explicit Mission start, prepared decision facts, authoritative
Runtime offline/online recovery, in-place Attention refresh, byte-identical
restart state, and the light/dark 900/1080/1440-point matrix. Final live J9 AX
enumeration found three enabled product actions without readable help labels:
service `Try Again`, empty-chat `Open Folder...`, and empty-chat
`Use Agent Team`. Attempt 012 is preserved as failed history. Repair 13 is
limited to those native labels and existing structural regression coverage.
Causal RED failed on all three missing-help conditions; minimal GREEN and
independent exact-byte source re-review passed with `P0=P1=P2=0`. A replacement
lock, complete matrix, fresh signed Release, and clean J1-J10 journey remain
required. Phase 2C and ADR-0015 remain unaccepted.

`CURRENT`: Repair 13 subsequently passed its replacement lock, complete
Go/Swift/Thread Sanitizer/signed-Release matrix, and direct signed-Release AX
smoke with zero unlabeled enabled product actions. Attempt 013 passed J1 and
selected the J2 folder without reading its sentinel, then exposed an equal
deadline race on the first post-restart folder chat. The daemon emitted its
bounded safe response after `10.066780s`, while the TUI client's nominal long
deadline was the same 10 seconds, so the client showed raw `timeout` before the
safe tentative unavailable message arrived and was persisted. Attempt 013 is
failed history. Repair 14 retains a 10-second handler work bound, gives server
response I/O 12 seconds, and gives Go/Swift clients a 15-second receive window.
Causal RED reproduced equal-deadline acceptance and all three Swift 10-second
values. Minimal GREEN plus a real 10-second boundary test now pass; the client
receives typed `state_unavailable` rather than a local timeout. An adjacent
real Go-to-Swift probe then exposed the Swift exchange validator's old
10-second ceiling; that validator now shares the tested public 15-second limit.
Focused cancellation, ordinary/long-method separation, Go normal/race, real
Go-to-Swift, Swift IPC, and Thread Sanitizer checks pass. Independent source
review passed with `P0=P1=P2=0`. A replacement source lock, complete lock-bound
matrix, signed Release, and clean J1-J10 replacement journey remain required.
Phase 2C and ADR-0015 remain unaccepted.

`CURRENT`: Repair 14 subsequently passed its replacement 50-path source lock,
complete Go normal/race/vet/tidy/format, Swift, Thread Sanitizer, and signed
Release matrix. Attempt 015 proved J1-J8 with one explicit main Journey UUID,
safe no-folder and post-restart folder chat, explicit Team confirmation and
Mission start, exact prepared approval facts in its named fixture,
authoritative Runtime `online -> offline -> online` transitions, coherent
Attention refresh, no folder-sentinel leak, and byte-identical restart state
with `261/261/261` main and `75/75/75` decision identities plus SQLite
integrity `ok`. Live signed-Release J9 then found six enabled Recent-row
`AXPress` buttons with empty help/title/value. Pressing the first selected its
visible `Inspect Failure` row, while scroll-bar and standard-window controls
were separately identified by AX parent/subrole. Attempt 015 is failed history.
Repair 15 is limited to dynamic Recent-row help in the existing native shell
and structural test paths. Independent contract review is the current gate;
Phase 2C and ADR-0015 remain unaccepted.

`CURRENT`: Repair 15 Contract Review 1 returned `FAIL` with two status-only P2
findings: the Repair Amendment header still named Repair 14 and Candidate
Preflight counted only Attempts 001-013. Both were corrected; Attempt 015 now
also explicitly excludes the non-canonical Deny rehearsal and binds J6 only to
the independently rebuilt canonical `Allow once` fixture. Exact-byte Contract
Re-review 2 returned `PASS` with `P0=P1=P2=0`. Causal RED is the current gate;
no replacement lock or journey is authorized.

`CURRENT`: Repair 15 causal RED added one dynamic Recent-row help assertion to
the existing native action-name test and failed exactly at that assertion. The
product helper had `.accessibilityLabel("Open recent task \(task.title)")` but
no corresponding `.help`, matching live AX's empty help/title/value. Minimal
GREEN adds the same bounded dynamic string as help on that existing button;
focused verification is the current gate.

`CURRENT`: Repair 15 minimal GREEN and the complete seven-test
`LoomGraphiteViewTests` class pass. The only product change adds dynamic help to
the existing Recent-row button; its action, selection, layout, and all authority
boundaries are unchanged. Independent exact-byte source review is the current
gate; the Repair 14 source lock remains invalid and no journey is authorized.

`CURRENT`: Repair 15 Source Review 1 returned `FAIL` with two P2 findings.
Candidate Purpose/Preflight still named RED as pending, and directly
interpolating `task.title` into AX help did not prove safety for raw saved-team
names. Remediation centralizes label construction in the native UI, applies the
existing `SafeText` sanitizer with a 48-character title bound and empty fallback,
binds the exact result to both accessibility label and help, and adds hostile
and empty-title tests. Remediation verification is the current gate.

`CURRENT`: Repair 15 bounded-label remediation passes all eight
`LoomGraphiteViewTests` and all four `SafeTextTests`. Hostile terminal escapes,
control/bidi content, overlong titles, and empty output are covered; both AX
label and help consume the one safe helper value. Source Re-review 2 is the
current gate; no replacement source lock or journey is authorized.

`CURRENT`: Repair 15 Source Re-review 2 closed the raw-title safety finding but
returned one P2 because Candidate Purpose still listed the already-passed
bounded-label remediation as pending. That phrase now marks remediation passed;
exact-byte Source Re-review 3 is the current gate. Product and test bytes are
unchanged.

`CURRENT`: Repair 15 exact-byte Source Re-review 3 returned `PASS` with
`P0=P1=P2=0`. Status Review 1 then returned two P2 findings because Candidate
Purpose/Preflight still listed completed source work as pending and this PASS
entry had been inserted into an older Phase 2A section rather than the current
tail. Candidate now records all source work complete and this entry is at the
true tail. Exact-byte Status Re-review 2 is the current gate; no source lock,
matrix, signed Release, or replacement journey is authorized.

`CURRENT`: Repair 15 exact-byte Status Re-review 2 returned `PASS` with
`P0=P1=P2=0`. All contract, source, remediation, failed-review, attempt-count,
and current-gate records agree. Replacement source lock generation and
independent lock review are the current gate; no matrix, signed Release, or
replacement journey is authorized yet.

`CURRENT`: Repair 15's independently reviewed source lock bound 50 paths with
ordered digest
`6bcca52489a38f9bc5b80955172a0d229011b484ccd17ff4f59cdfa46fe41e8e`
and lock SHA
`e2fb37fc858888f306781b3402bbde052bc474b3a08aff11e0a18cea72c9e89d`.
The first full Go normal matrix failed one controlled Runtime restart snapshot
with `local product unavailable`; the exact frozen test reproduced once in 20
runs. Repair 16 therefore invalidates that lock and owns only daemon-test
readiness synchronization: each integration wait must consume the exact
runner server's accepted `Ready()` barrier before socket verification. No
production retry, timeout, IPC, authority, or persistence change is admitted.
Independent Repair 16 contract review is the current gate.

`CURRENT`: Repair 16 independent Contract Review returned `PASS` with
`P0=P1=P2=0`. The preserved full-normal failure and one-in-20 exact
reproduction are the causal RED. Bounded test-only GREEN is the current gate;
production daemon, IPC, retry, timeout, authority, and persistence bytes remain
out of scope.

`CURRENT`: Repair 16's first GREEN attempted to pass `runner.server` directly,
but two production-builder fixtures are statically typed as `daemonRunner` and
the package correctly failed to compile. The bounded implementation now passes
each exact runner, fail-closed asserts `*productDaemonRunner` inside the test
helper, and awaits only that runner's server readiness. The controlled Runtime
restart test passes 100 consecutive runs in `5.520s`. Complete `cmd/loomd`
normal/race verification is the current gate.

`CURRENT`: Repair 16 complete `cmd/loomd` normal passed in `262.284s`; race
passed in `227.471s` with no race finding. Together with the exact count-100
GREEN, all 16 generation-bound readiness call sites are green. Independent
exact-byte source review is the current gate; no replacement lock or full
matrix is authorized yet.

`CURRENT`: Repair 16 Source Review 1 confirmed the generation-bound helper and
all 16 call sites, but returned one P2 because Candidate Preflight still named
already-completed GREEN as the current gate. It now records Contract Review,
count-100 GREEN, and daemon normal/race complete; exact-byte Source Re-review 2
is the current gate. Product and test bytes are unchanged.

`CURRENT`: Repair 16 exact-byte Source Re-review 2 returned `PASS` with
`P0=P1=P2=0`. The test-only generation readiness barrier and all 16 bindings
are accepted for lock preparation. Final status-byte review is the current
gate; no replacement lock or full matrix is yet authorized.

`CURRENT`: Repair 16 Status Review 1 returned one P2 because Candidate Purpose
still said the boundary ended at Repair 15. It now names Repair 16; exact-byte
Status Re-review 2 is the current gate. Product and test bytes are unchanged,
and no replacement lock or full matrix is authorized yet.

`CURRENT`: Repair 16 exact-byte Status Re-review 2 returned `PASS` with
`P0=P1=P2=0`. All Repair 16 contract, RED/GREEN, compile-failure, daemon
normal/race, source-review, and current-gate bytes agree. Replacement source
lock generation and independent lock review are the current gate.

`CURRENT`: Repair 16 then passed its independently reviewed 50-path source
lock, complete Go/Swift/TSAN matrix, and signed Release. Attempts 016 and 017
are failed harness history: the first omitted the original Runtime model
catalog arguments, while the second omitted the explicit Journey UUID from two
TUI permission-attention reads. Attempt 018 corrected both bindings and passed
J1-J6 plus controlled J7 offline projection/replay, but ordinary Runtime
recovery exposed `mission execution conflict` while the Mission awaited human
review. None of those attempts is promotable.

`CURRENT`: Repair 17 is active. Restart reconciliation now keeps an aggregate
`running` TeamExecution quiescent only when it has at least one node, at least
one node is `ready_for_review`, and every node is either `succeeded` or
`ready_for_review`. Causal RED failed with `mission execution conflict`;
bounded GREEN, complete `internal/app`, and supporting live recovery/restart
proof pass without a runner call, implicit rerun, or authority append. Focused
daemon restart/recovery coverage also passes.
Independent Repair 17 contract/source review is the current gate. Phase 2C and
ADR-0015 remain `PARTIAL`; a replacement lock, complete matrix, new signed
Release, and clean J1-J10 remain mandatory. Native J9/J10 traversal additionally
waits for the local macOS session to be unlocked.

`CURRENT`: Repair 17 combined Contract + Source Review 1 found no source
defect, but returned `P2=2`: the Repair Amendment header still named the Repair
16 lock gate, and Candidate Purpose still said Repair 16 lock/matrix/Release and
clean J1-J10 were pending. Both status statements now reflect the passed Repair
16 gates, failed Attempts 016-018, completed Repair 17 GREEN/focused daemon
coverage, and the current exact-byte re-review gate. Product and test bytes are
unchanged. No replacement source lock is authorized before re-review passes.

`CURRENT`: Repair 17 exact-byte Contract + Source Re-review 2 returned `PASS`
with `P0=P1=P2=0`. It closed both stale-status findings, recomputed the reviewed
production/test hashes, and accepted the narrow restart predicate without a
source finding. Final status-byte review is the current gate before replacement
source-lock generation; no matrix, signed Release, Journey, Phase, or ADR
acceptance is authorized yet.

`CURRENT`: Repair 18 bounded test-only GREEN passes. The provider exact test is
green 100/100 in `67.207s`. Both real Swift client tests plus the shared-path
assertion pass in one process in `46.770s`; complete `cmd/loomd` normal/race
pass in `50.382s`/`47.593s`, and complete `internal/provider` normal/race pass
in `2.430s`/`3.143s`, with no race finding. Production bytes remain unchanged.
Independent Repair 18 source review is the current gate; no replacement lock,
full matrix, Release, Journey, Phase, or ADR acceptance is authorized.

`CURRENT`: Repair 17 final Status Review 3 returned `PASS` with
`P0=P1=P2=0`. It verified the Review 1 and Re-review 2 records, failed Attempts
016-018, unchanged product/test hashes, current gate, `PARTIAL` Phase/ADR
status, and zero staged paths. The replacement 50-path source lock is now
generated; independent lock review is the sole gate before any complete matrix.
No signed Release or Journey is authorized.

`CURRENT`: Repair 17 source-lock review returned `PASS` with `P0=P1=P2=0`,
binding 50 paths at ordered digest
`e82334d38e561010eb32654062b9198b71cbb1b0d77a3ff15acdf0e183a03273`
and lock SHA
`3968a970cc3d8743aa3c518b28c10f713086cef5699fb3d5a33c52229421a5ea`.
The first full Go normal matrix then failed: `cmd/loomd` timed out at
`721.048s` while rebuilding the same Swift Release probe through a second
private scratch path, and the provider login-controller test read an existing
but incomplete PID file as `EOF`. The exact Swift test passed alone in
`99.926s`; provider count-100 reproduced the same `EOF` twice. Repair 18 owns
only package-process Swift probe reuse/cleanup and complete PID-fixture
readiness in the two existing test files. Independent Repair 18 contract review
is the current gate. Product bytes are unchanged; no implementation, matrix,
Release, Journey, Phase, or ADR acceptance is authorized.

`CURRENT`: Repair 18 Contract Review 1 accepted the causal diagnosis, two-file
test-only scope, process-local Swift build reuse/cleanup, and strict positive
PID readiness, but returned `P2=1` because Candidate Purpose still said the
boundary ended at Repair 17. It now says Repair 18. Exact-byte Contract
Re-review 2 is the current gate; both test files remain unchanged and
implementation is not yet authorized.

`CURRENT`: Repair 18 exact-byte Contract Re-review 2 returned `PASS` with
`P0=P1=P2=0`. It confirmed the corrected Candidate Purpose, causal RED,
two-file test-only scope, cleanup ownership, strict PID readiness, unchanged RED
source hashes, and zero staging. The bounded Repair 18 test implementation is
now authorized; no matrix, replacement lock, Release, Journey, Phase, or ADR
acceptance is authorized yet.

`CURRENT`: Repair 18 Source Review 1 found no implementation defect, but
returned `P2=2`: Candidate Purpose still left Repair 17 lock review pending, and
the Repair 18 GREEN entry had been inserted before later historical entries
instead of at the true tail. Candidate Purpose now records Repair 17 lock review
passed and Repair 18 GREEN complete. This entry is at the physical EOF.
Exact-byte Repair 18 Source Re-review 2 is the current gate; implementation
hashes are unchanged and no replacement lock or full matrix is authorized.

`CURRENT`: Repair 18 exact-byte Source Re-review 2 returned `PASS` with
`P0=P1=P2=0`. It confirmed both status P2s closed, implementation mechanics and
hashes unchanged, one physical EOF record, no leaked Swift build root, and zero
staging. Final Repair 18 status review is the current gate; no replacement
source lock, full matrix, Release, Journey, Phase, or ADR acceptance is
authorized.

`CURRENT`: Repair 18 Status Review 3 returned `FAIL` with `P0=0`, `P1=0`,
`P2=1`: Candidate Purpose still called the completed Source Re-review 2 the
current gate. Purpose now records Source Re-review 2 passed and exact-byte
Status Re-review 4 current. Implementation hashes are unchanged. No replacement
source lock or complete matrix is authorized before re-review passes.

`CURRENT`: Repair 18 exact-byte Status Re-review 4 returned `PASS` with
`P0=P1=P2=0`. It closed the final stale Purpose gate, verified the single EOF
status record, all review counts, implementation hashes, failed-matrix
provenance, source inventory, no leaked Swift root, and zero staging. The
replacement 50-path source lock is now generated; independent lock review is
the current gate. No complete matrix, Release, Journey, Phase, or ADR acceptance
is authorized.

`CURRENT`: Repair 18 replacement-lock review, complete matrix, and signed
Release passed. Attempt 022 then passed J1-J8 authority/restart boundaries and
the J10 visual matrix, but direct signed-Release J9 inspection found duplicate
VoiceOver labels for four visually distinct Recent rows. Attempt 022 is
preserved and excluded. Repair 19 changes only the existing native Recent-row
label helper and its regression test: one-based visible position plus bounded
sanitized title/subtitle now feed both accessibility label and help. The causal
RED and focused GREEN pass. Independent Repair 19 contract/source/status review
is the current gate; no replacement lock, full matrix, new signed Release,
replacement J9/J10, Phase, or ADR acceptance is authorized.

`CURRENT`: Repair 19 Contract/Source/Status Review 1 found no product defect
and returned `P2=1`: the deterministic GREEN summary named duplicate inputs at
positions three and four, while the exact test uses positions one and two. The
append-only correction now records the tested values; product and test hashes
are unchanged. Exact-byte Repair 19 re-review is the current gate. No source
lock, complete matrix, signed Release, replacement J9/J10, Phase, or ADR
acceptance is authorized.

`CURRENT`: Repair 19 exact-byte Contract/Source/Status Re-review 2 returned
`PASS` with `P0=P1=P2=0`. It verified the append-only evidence correction,
unchanged product/test hashes, bounded unique-label implementation, exact
status surfaces, Attempt 022 J9 failure provenance, zero staging, and clean
diff checks. Replacement source-lock generation and independent lock review are
now authorized. Only after lock review may the full matrix and signed Release
carry Attempt 022 J1-J8; replacement live J9/J10 remain mandatory. Phase 2C and
ADR-0015 remain unaccepted.

`CURRENT`: The Repair 19 replacement 50-path source lock is generated from the
exact Re-review 2 bytes; independent lock review is the current gate. No
complete matrix or signed Release is authorized before that PASS. Attempt 022
remains excluded, with only its J1-J8 authority/restart evidence eligible for
later carry-forward; replacement live J9/J10, Phase 2C, and ADR-0015 acceptance
remain mandatory.

`CURRENT`: Repair 19 source-lock Review 1 passed with `P0=P1=P2=0`, but its
first complete Go normal matrix failed after `753.73s`. The only failure was
`TestPhase1EngineeringDemoApprovalRestartReconnectAndRecovery`: one competing
caller returned `ErrTeamExecutionIncomplete`. Every other package passed. The
exact test then passed 100/100 in `167.398s`, confirming a low-probability test
window rather than authorization to ignore the failure. Repair 20 newly admits
only `internal/app/phase1_engineering_demo_test.go` and freezes a closed loser
classification: incomplete is acceptable only with zero executed nodes, while
exactly one `main` winner and all authority/call-count assertions remain.
Independent Repair 20 contract review is the current gate; no implementation,
replacement lock, matrix, Release, J9/J10, Phase, or ADR acceptance is
authorized.

`CURRENT`: Repair 20 Contract Review 1 returned `PASS` with `P0=P1=P2=0`.
It accepted the causal diagnosis, 52-path Candidate, test-only scope, closed
incomplete-loser condition, exactly-one-winner guard, unchanged production
hash, and zero staging. A deterministic classification RED and bounded
test-only implementation are now authorized. No replacement lock, complete
matrix, Release, J9/J10, Phase, or ADR acceptance is authorized.

`CURRENT`: Repair 20 deterministic RED failed on the intentionally missing
closed-classifier helper. Bounded GREEN then passed the classifier plus recovery
scenario, the scenario 100/100 in `192.325s`, complete `internal/app` normal in
`6.451s`, and race in `31.385s` with no race finding. Incomplete is accepted
only for a zero-execution loser; exactly one `main` winner and all downstream
authority/call-count assertions remain. Production coordinator hash is
unchanged. Independent Repair 20 source/status review is the current gate; no
replacement lock, complete matrix, Release, J9/J10, Phase, or ADR acceptance is
authorized.

`CURRENT`: Repair 20 Source/Status Review 1 returned `PASS` with
`P0=P1=P2=0`. It verified the closed classifier, exactly-one-winner and
downstream authority assertions, unchanged production coordinator hash,
52-path inventory, RED/GREEN evidence, and zero staging. The replacement source
lock is now generated; independent lock review is the current gate. No complete
matrix, signed Release, J9/J10, Phase, or ADR acceptance is authorized before
that PASS.

## Phase 2C Final Acceptance (2026-08-09)

`CURRENT`: **Phase 2C, P2C-W1 Workspace Shell & Entry, P2C-W2 Chat-First
Conversation, P2C-W3 Governance Side Panel, and ADR-0015 are accepted.** The
Product Owner explicitly confirmed acceptance on 2026-08-09 after every frozen
Phase 2C gate passed. Earlier `PARTIAL`, failed-attempt, pending-gate, and
superseded-lock entries above remain historical evidence; this section is the
current reconciliation record.

`CURRENT`: The accepted client opens chat-first in both the native macOS app
and Bubble Tea TUI. Conversation is the primary workspace; Agent Team Mission
Board, topology, timeline, decisions, Evidence, runtime health, and Attention
remain collapsible or separately opened governance surfaces. Ordinary chat
never creates a Team, Mission, Run, Grant, or authoritative fact. Agent use,
Team Draft confirmation, New Mission, decision actions, and review transitions
remain explicit daemon-authorized operations.

`CURRENT`: The final Repair 20 Candidate binds 51 source paths at ordered
digest
`6fa2f269fefdaed41e60da0c64faf8f173924c427f23c1c5b0937693fab6cfa7`
and source-lock SHA-256
`7ad412e6b0122f3f37dfbbdaa3b5127f22d8b850963a1f1949fd14bf38ec2f1b`.
Go full/race/vet/tidy/gofmt, Swift full/Thread Sanitizer, strict code-sign
verification, and the signed arm64 Release passed. Combined native and real-PTY
journeys J1-J10 passed, including restart/reconnect, authority comparison,
keyboard traversal, unique accessibility labels, compact width, and the
light/dark screenshot matrix.

`CURRENT`: Independent implementation, dual-Result, whole-WorkItem,
whole-Phase, journey/UI, source-lock, and A4 evidence-lock reviews pass with
`P0=P1=P2=0`. The Product Owner sign-off is preserved at
`.loom-evidence/phase2c/reviews/PHASE-2C-PRODUCT-OWNER-SIGNOFF.md`. The
post-signoff transition changes acceptance metadata only; product, test,
contract, Journey, Event Journal, Projection, Scheduler, policy, Grant,
Evidence, Provider, credential, and execution-authority bytes remain frozen.

`CURRENT`: A durable local delivery is available at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2c-2026-08-09/`.
Its ad-hoc signed `Loom.app` is a thin arm64 bundle with identifier
`com.earendilworks.loom.local`; executable SHA-256 is
`a9b02d9e60afa96f8c79e18a73e8e8ca15bde61b1564ae0e1d5086dc7203bc4f`.
The transport ZIP SHA-256 is
`bd6abcb0e66c5febdf024fb52537e3be68538bd2252215ad5569f6113fc54a7d`.
This is a verified local delivery, not a Developer ID notarized public release.

`CURRENT`: No path is staged and no commit, push, merge, publication,
notarization, system-directory installation, credential change, paid action,
or autonomous activation is authorized or performed by this acceptance. Phase
3A source-lock refresh and entry review are now eligible but remain separate,
unstarted governance work.

## Phase 2C v0.4.2 App Startup Patch (2026-08-09)

`CURRENT`: The Product Owner required `Loom.app` to be directly usable without
manually starting `loomd`. The prior Phase 2C local delivery bundled only the
Swift UI executable; the default
`~/Library/Application Support/Loom/run/loomd.sock` was absent, while the
unrelated legacy `com.loom.watchdog` LaunchAgent pointed at a removed script and
exited `127`. The UI therefore truthfully showed `Local service unavailable`,
but the delivery journey was incomplete.

`CURRENT`: v0.4.2 implements ADR-0016. The App bundle now contains the matching
arm64 `loomd` and a `SMAppService` LaunchAgent descriptor. Developer ID signed
builds use the macOS-managed service path. When an ad-hoc local build cannot
activate that path, the open App starts only its fixed-path bundled helper with
a bounded private environment and parent PID; no shell, `launchctl`, repository
script, external daemon path, or user-supplied process argument is used. The
helper creates and validates private state/isolation/run directories, discovers
the installed Pi Runtime, uses the standard Socket, and cancels when its App
parent exits. Swift waits for the real Socket and retries IPC before loading the
remaining views.

`CURRENT`: Focused Go and Swift RED/GREEN tests pass. Full `cmd/loomd` and Swift
test suites pass. The native build fixture passes two clean deterministic
Release builds, arm64/helper/plist/mode/deep-signature checks and launch smoke;
the native installer fixture passes install, replacement, failure recovery,
signal recovery, rollback and symlink rejection. The pre-existing Swift 6
generic `QueueCommand<Input>` Sendable warning remains outside this patch.

`CURRENT`: A real ad-hoc App launch created the standard Socket and returned a
complete IPC status with `stale=false`, `partial=false`, zero Teams and one
Runtime. The visible window showed `Local service ready` and an enabled chat
composer. Closing the App stopped the parent-bound daemon and removed the
Socket; reopening it recreated both and returned to ready without terminal or
manual service action. This is a verified local v0.4.2 Candidate; it does not
replace the accepted Phase 2C source lock or claim Developer ID notarization,
public release, push, merge, or publication.

## Phase 2C v0.4.3 Codex Conversation Patch (2026-08-09)

`CURRENT`: The v0.4.2 startup patch made the service available, but a first
ordinary message could still return `No conversation runtime is configured`.
The live Runtime projection showed Pi online with an empty `model_ids` array,
while Codex native auth was independently logged in. Construction created a
conversation responder only for a separately configured local model, so the
primary chat experience and Runtime & Providers status did not describe the
same usable path.

`CURRENT`: v0.4.3 implements ADR-0017. A configured local model remains first
choice; otherwise a supported Codex executable creates an automatic native-auth
conversation responder. Each call is ephemeral, read-only, bounded, timed out,
identity-fenced, and runs in a private empty directory with no inherited API
keys or project rules. Output remains tentative and cannot create Agent Teams
or authoritative facts. Swift now shows the pending user message and response
progress immediately, prevents duplicate sends, and restores the draft on
failure.

`CURRENT`: Focused Provider client, fixed process invocation, daemon prompt,
real IPC-to-fake-Codex, and Swift pending-response tests pass. This patch does
not silently initiate login, authorize Agent execution, replace the accepted
Phase 2C source lock, or claim a notarized public release.

`CURRENT`: Final verification passes the complete `internal/provider`,
`internal/api`, and `cmd/loomd` Go suites, Provider/daemon vet, all 129 Swift
tests with one visual-export skip, and two clean deterministic native Release
builds. The local Codex 0.144.1 parser accepts every fixed tool-disable feature.
No live remote completion was invoked; the exact text-only process path is
covered through real daemon IPC against a controlled Codex executable.

`CURRENT`: The final ad-hoc v0.4.3 App starts its bundled helper automatically,
returns complete non-stale IPC status, and presents `Codex — Available` in the
native Runtime & Providers sheet. It remains open at the chat-first workspace.
The durable local delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2c-v0.4.3-final-2026-08-09/`.
The Swift executable SHA-256 is
`d21b3ecbd3821b15a75f91dea3665a581440f57f7241a96520cac3f19b44f638`,
the bundled daemon SHA-256 is
`a94942f8205a2e83d2448219fe7a5dc731d07d2ff1abeaedee552460cc43855b`,
and the transport ZIP SHA-256 is
`5d4951a2261269f56916c6d4d098d80f91367cb0cfa8ec7ae3d0d6ea4954392c`.

`CURRENT`: The verified v0.4.3 bundle is installed at
`/Users/lune/Applications/Loom.app` through the transactional local installer;
the prior v0.2.0 bundle is retained at `Loom.app.previous` for rollback. The
installed executable/helper hashes match the durable delivery, its deep
signature passes, and opening that ordinary Applications entry starts the
installed helper as the App child and returns complete IPC health.

## Phase 2D v0.5.0 Provider Compatibility W1 (2026-08-09)

`CURRENT / UNIFIED GOAL`: Phase 2D is the sole active product Goal: an installed
Loom App must run real conversations and govern a multi-Provider, multi-model
Agent Team in which each Agent independently freezes its Harness, Provider
Account, credential revision, Model, limits, and capabilities. Observability,
failure isolation, explicit fallback, account-level accounting, and governance
UI are horizontal exit requirements. P2D-BLOCKER-1 is the current credential
path blocker under this Goal; it does not replace the Goal. Accepted Phase 2C
remains closed.

`CURRENT`: Phase 2D is inserted after accepted Phase 2C without reopening its
WorkItems or source lock. ADR-0018 defines a registry-driven Provider model and
keeps exact Provider/Model selection bound to future versioned Profiles and
individual Runs rather than a global current-Provider switch.

`CURRENT`: P2D-W1 replaces the two-row Provider management surface with separate
Agent Runtime and Model Provider sections. The macOS client renders an ordered
24-entry directory with search, category filtering, protocol/auth labels, and
truthful connection status. The legacy Codex and MiniMax snapshot fields remain
temporarily for compatibility; the primary surface uses the new `providers`
collection.

`CURRENT`: Sixteen fixed API-key Providers can be configured, verified,
replaced, and revoked through the existing Broker and macOS Keychain. Provider
verification uses registry-owned HTTPS origins, no proxy, no redirect, bounded
timeouts/responses, and non-generative model-list requests. Managed-cloud,
local Runtime, and custom endpoint entries do not enter this verifier.

`PARTIAL`: a verified credential is not yet an executable Provider Profile.
OpenAI through Codex native auth remains the automatic ordinary-conversation
path, and the pre-existing MiniMax Runtime Profile remains the governed Team
path. P2D-W2 Run-bound Profiles and P2D-W3 Runtime Client Adapters are still
required before Loom can claim broad execution compatibility.

`CURRENT`: P2D-W1 verification passes the Provider, App, API, local IPC, and
daemon Go suites and vet; all 130 Swift tests pass with one pre-existing visual
export skip; and the native fixture passes two deterministic Release builds,
bundle/helper/signature checks, and launch smoke. No live Provider completion
or credential was used. A real installed v0.5.0 App starts its bundled local
service automatically, reports `Codex - Available`, presents the searchable
Provider directory, and opens a Provider's Keychain-backed credential sheet
without exposing or persisting plaintext in the UI.

`CURRENT`: The final local delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.0-w1-final-2026-08-09/`,
and the verified bundle is installed at `/Users/lune/Applications/Loom.app`.
The installed Swift executable SHA-256 is
`2e4ef5689ccde459efa6414e1980c0f7021f684f54c6a4f9746a21fe09b90891`,
the bundled daemon SHA-256 is
`339f69471195f3cc694a3320bb69ba0b1d7fbd9e39c1c00cb3dd06c4428bb544`,
and the transport ZIP SHA-256 is
`c6a37044959df54af242e1e43300856c14ed7fa73d44709ca1e4cb7e7b9ec60c`.

## Phase 2D v0.5.1 Provider Credential Patch (2026-08-09)

`CURRENT`: A live DeepSeek connection attempt exposed a cross-layer W1
regression. The registry and setup service accepted registered Providers, but
the Credential Broker, authoritative metadata writer, and projection still
restricted durable credential metadata to `minimax`. The request therefore
failed before verification and the client collapsed the closed internal error
to `Unavailable`. The failed transaction rolled its Keychain write back and
did not append a partial credential event.

`CURRENT`: v0.5.1 replaces the legacy Provider equality checks with one strict,
bounded lowercase ASCII Provider identifier contract while the registry remains
the authority for which Providers can enter the Broker. Regression coverage
proves a DeepSeek credential can configure, verify, append non-secret metadata,
and rebuild through the projection; malformed identifiers remain rejected.
Provider, credential, state, projection, App, API, local IPC, and daemon suites
and vet pass, as does the two-build native Release fixture.

`PARTIAL / REOPENED`: The v0.5.1 bundle is installed and running at
`/Users/lune/Applications/Loom.app` with its bundled local service ready. The
durable delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.1-provider-credential-fix-2026-08-09/`.
The Swift executable SHA-256 is
`fb468214cdf0ec25307a1feb56f33ce398ae03012ca35548a2c2ebe7df63f926`,
the bundled daemon SHA-256 is
`d4569a4d8ac822362eb375ac410929c14522b6c26c7e4766612814c63b373c78`,
and the transport ZIP SHA-256 is
`028dfb8b0b636a3d623329b3555b97201be507aefa430ed0213220582c12c23e`.

`PARTIAL / REOPENED`: v0.5.1 repairs the lower-layer Provider-ID restriction,
but a fresh real DeepSeek key still leaves the installed setup snapshot at
`unconfigured`, revision zero, with no credential reference. No new Keychain
item is committed. The earlier automated matrix did not exercise a real
installed credential, so v0.5.1 must not be described as a completed live fix.
P2D-BLOCKER-1 now owns diagnosis of Swift request admission, UDS, process
Keychain helper, metadata commit, projection, and safe error presentation.

## Phase 2D v0.5.2 Conversation Profiles and Per-Agent P0 (2026-08-09)

`PARTIAL / INSTALLED CANDIDATE`: P2D-W2A adds per-conversation Profile binding
for Codex native auth and verified DeepSeek credentials. A conversation keeps
its exact Profile; changing Provider starts a new conversation. DeepSeek
requests require the current verified credential revision and use a fixed,
bounded, non-tool route. Loom v0.5.2 build 12 is installed at
`/Users/lune/Applications/Loom.app`; opening it starts the bundled `loomd`, and
the installed bundle's real process Keychain helper passes isolated
put/read/delete over UDS without using a Provider key or network request. This
distinguishes helper launch/signing/Keychain access from the earlier failure,
but it does not complete fresh-key verification or a live DeepSeek reply.

`CURRENT`: ADR-0019 and P2D-W2B establish that Team execution is never
conversation-level or Team-level Provider state. Each Team role independently
selects an Execution Profile backed by the existing `RuntimeProfileID` schema.
The P0 gap is closed in the domain binding: each role freezes Harness, Runtime
instance, Provider, Provider Account, Model, endpoint fingerprint, credential
reference/revision, timeout, budget, and capabilities. Multiple accounts for
one Provider produce distinct bindings without exposing secret bytes. Optional
reasoning effort is now part of the versioned Profile and frozen Attempt
binding, while legacy Profiles retain their original digest.

`CURRENT / PARTIAL`: P2D-W2C now persists that exact binding on each primary
Agent Attempt, validates its digest during replay, projects it into the Team
read model, dispatches with the role-local binding, and rejects ordinary retry
that silently changes Harness, Provider Account, credential revision, or Model.
Independent Verifier Agents now freeze the same binding on their authoritative
Run; replay, claim, restart, and terminal-receipt recovery reject a missing,
tampered, or Runtime-mismatched binding. The product-level mixed-Team entry path
remains to be completed before the four-pair Team matrix can be claimed.

`CURRENT / DISPATCH BINDING GATE`: Supervisor now freezes the Profile and
Runtime supplied for execution, requires its digest to equal the authoritative
Run binding, and passes that exact frozen binding into the concrete adapter.
Model, Provider Account, credential revision, Harness, or limits drift is
rejected before an adapter process starts. The Pi process adapter validates the
binding's Harness, Runtime, and native auth mode; Pi RPC additionally requires
the exact configured `loom-local` Provider and model. This closes the previous
gap where authority froze a per-Agent binding but the final adapter call saw
only the Run stream identity. It also makes the current product boundary
explicit: the dynamic daemon catalog must not advertise a brokered Provider
Profile until a concrete adapter can consume its exact Provider Account and
credential revision. Build 12 may publish the concrete DeepSeek Profile only
for an exact verified account; Kimi, MiniMax, Anthropic, OpenAI, and custom
brokered Team Profiles remain absent until their W3 adapters exist.

`PARTIAL / ACTIVE`: P2D-W2D is `Observability, Failure Isolation, Governance,
Fallback, Accounting and UI`.
Its horizontal acceptance contract and ordered execution plan are frozen at
`.loom-evidence/phase2d/contracts/P2D-W2D-observability-governance.md` without
creating a second product Goal or resetting W2A/W2B/W2C.
It must carry one privacy-safe Incident/Correlation ID from Swift, UDS,
`loomd`, Keychain helper, Provider verification, Conversation Profile, Agent
Attempt, and Team board; persist bounded `0600` installed diagnostics; expose
specific stage, recovery, retryability, and incident controls; isolate Agent
failures; make fallback explicit, versioned, approvable, and auditable; and
attribute concurrency, rate limits, budgets, token/cost usage, and error rates
to the exact Provider Account, Run, Attempt, and Agent. The Event Journal
remains separate and authoritative; operational diagnostics never grant
execution authority. The four-pair Team acceptance matrix is not yet claimed.

`CURRENT / MINIMUM OBSERVABILITY GATE`: P2D-BLOCKER-1 now reuses the strict IPC
request identity as the Incident ID from Swift through UDS and the daemon's
credential path. Safe stage values and retryability reach the Provider sheet,
which shows an actionable explanation plus Stage, View diagnostics, and Copy
incident ID controls. Swift input-admission failures and daemon terminal
credential outcomes persist to separate bounded, rotating, owner-only `0600`
JSONL stores; the bundled process console is also retained as a bounded `0600`
file instead of `/dev/null`. These operational records are separate from the
Event Journal and contain no API key, Authorization header, prompt,
conversation content, or Provider response body.

`CURRENT / INSTALLED STAGE PROPAGATION REPAIR (2026-08-10)`: build 10 proved the
installed daemon persisted an empty-secret admission failure with
`daemon_admission`, but omitted that stage from the UDS error returned to Swift.
Build 11 normalizes every credential-method failure at the daemon boundary:
existing helper, Keychain, and metadata stages are preserved, while an
otherwise unstaged admission error receives the closed
`daemon_admission` value. A real `localipc.Client -> UDS -> loomd` regression
now requires the remote error, owner-only operational diagnostic, and
unchanged setup projection to agree in one attempt.

`CURRENT / PARTIAL GOVERNANCE UI`: the Team Board API and Swift model now carry
each current Agent Attempt's safe Harness, Provider, Provider Account, Model,
reasoning effort, credential revision, and terminal reason. Team Pulse displays
this binding when available and remains compatible with legacy timeline rows.
Credential
reference, endpoint fingerprint, binding digest, prompts, and secret content do
not cross into the UI.

`CURRENT / PARTIAL ACCOUNTING`: `RunTerminalCommitted` can now freeze a bounded,
validated, idempotent accounting fact containing token classes and microunit
cost by currency. Legacy terminal Events without accounting still replay; a
retry that changes accounting conflicts. Supervisor propagates the fact; Pi RPC,
Codex, Claude Code, and Loom Native/OpenAI-compatible adapters extract only
closed numeric terminal usage/cost. Projection links it to the exact Run,
Attempt, Agent, and frozen Provider Account, and Team Board aggregates current
concurrency, failures, observed rate-limit failures, assigned budgets, tokens,
per-currency cost, and error rate separately for multiple accounts of the same
Provider. Swift decodes the closed wire shape and Team Pulse displays Agent and
account totals. API keys, credential references, Provider bodies, and prompts
do not enter accounting.

The controlled four-Provider Team canary now emits distinct numeric usage and
cost for Codex/OpenAI, Claude Code/Anthropic, Loom Native/Kimi, and Loom
Native/MiniMax Attempt 1. Rebuilt Projection preserves each fact under its exact
account even when the OpenAI Attempt fails; the three peer accounts remain
unchanged. The scheduled recovery Attempt starts without inherited accounting,
then freezes its own distinct terminal usage after execution. Ten repeated
focused runs, three race runs, and the complete `internal/app` suite pass.
Configured Provider Account ceilings and installed real-Provider accounting
remain active W2D work.

`TARGET / NEXT PROVIDER ACCOUNT CEILING SLICE`: configured ceilings will use a
separate revision-CAS policy and account-capacity stream keyed by the complete
Provider plus Provider Account identity. Dispatch must atomically reserve both
Runtime and account capacity, freeze the admitted policy revision/digest on the
Attempt, and fail before credential lease access when concurrency, bounded
dispatch rate, or assigned budget is exhausted. Retry revalidates the same
account; an approved fallback validates the target account independently. The
Board will combine these configured limits with existing observed usage/cost.
This contract is frozen as the next W2D implementation boundary; no source or
installed completion claim is made yet.

`CURRENT / PARTIAL FALLBACK GOVERNANCE`: W2D now has an authoritative
pre-approved fallback binding transition. A versioned approval freezes its
non-secret approval ID, actor reference, UTC approval time, source binding
digest, target binding digest, and approval digest in the Team plan. When
recovery selects fallback, `TeamNodeRecoveryRecorded` links that exact approval,
and dispatch permits only the approved one-shot source-to-target transition;
ordinary retry continues to require the same frozen binding. A changed binding
with only a fallback workflow name, a forged source or target, a future-dated
approval, a policy swap, or a switch back to the source fails closed. Existing
workflow-only fallback and legacy Journal replay remain compatible. The Agent
editor's fallback controls and interactive approve/reject UI, fallback health
preflight, and Board presentation of the configured target remain active W2D
work.

`CURRENT / PARTIAL AGENT PROFILE EDITOR`: the Team Builder now exposes one
independent execution-profile selector per Agent. Every row shows its safe
Harness, Provider, exact Provider Account, Model, reasoning effort, credential
revision, timeout, budget, and required capabilities; selecting another
compatible RoleOption
edits only that Agent through the existing revision-checked Builder command.
The backend regression proves that changing the primary Agent changes the Team
binding digest while the peer Agent keeps its original Profile, Provider
Account, and credential revision. Setup and Builder wire projections derive
these fields from validated Runtime Profiles and never expose credential
references, endpoint fingerprints, binding digests, secret bytes, or prompts.
`RuntimeProfile` and every new Agent Attempt binding now freeze an optional
reasoning-effort token. Explicit values require a Runtime that advertises the
`reasoning_effort` capability, use a domain-separated v2 binding digest, survive
Team and Run Journal replay, and appear in Builder, Board, and privacy-safe
diagnostic projections. Empty values retain the exact published v1 digest and
display as Provider default, preserving legacy replay. Direct versioned Profile
authoring now lets the user independently change an Agent's observed Model,
reasoning effort, timeout, and numeric budget. Each edit creates an immutable
Profile ID, changes only that role, and rejects invalid input without advancing
or partially mutating the Draft. Account health preflight and Agent-level
fallback configuration/approval remain active W2D work.

`CURRENT / VERSIONED SAVED PROFILE AUTHORITY`: confirmation now writes the
complete non-secret version-1 Execution Profile snapshot into each saved role.
The StateWriter requires it to equal the exact Runtime Profile used to validate
the Team, and Runtime admission requires the Profile to be freeze-ready before
it can become saved authority. Projection validates and deep-copies Model,
Harness, Provider Account, endpoint fingerprint, credential reference/revision,
reasoning effort, timeout, budget, and capabilities. After App/daemon restart,
the Builder and saved-Team materializer can restore a custom Profile that is no
longer in the dynamic catalog; an existing same-ID Profile with different
content fails closed. The persisted credential reference is opaque metadata;
API keys, Authorization headers, prompts, Provider bodies, and secret bytes do
not enter the Team event.

`CURRENT / PRIVACY-SAFE DIAGNOSTIC EXPORT`: a Provider Incident can now open a
native preview instead of exposing the raw diagnostics directory. The preview
shows App/daemon version and SHA-256, Socket health, daemon-console availability
and byte count, non-secret Provider/Profile/Agent binding state, recent
allowlisted operational Events, and any unsafe or malformed records skipped.
The user-selected JSON export is owner-only `0600`; diagnostics are opened with
`O_NOFOLLOW`, bounded, validated, and re-encoded from a closed field set. Raw
daemon console, API keys and credential references, Authorization headers,
environment credentials, prompts, conversations, and Provider response bodies
are excluded by default and named in the preview. A malicious extra `secret`
field and a symlinked diagnostic file cannot enter the bundle. This export is
support evidence only and remains separate from the Event Journal and execution
authority.

`CURRENT / FIRST PROVIDER-AWARE AGENT ADAPTER (2026-08-10)`: the latest source
candidate now contains the first complete W3 vertical slice for
`loom-native + DeepSeek + deepseek-chat`. The daemon registers an authoritative
local `loom-native` Runtime, publishes revision-qualified DeepSeek Team Profiles
only while the exact credential projection is verified, and routes each Agent
to a Supervisor selected by its own Harness and Runtime instance. The adapter
revalidates the frozen Provider, Provider Account, Model, endpoint fingerprint,
credential reference/revision, auth mode, timeout, and capabilities before any
credential or network access; it resolves the secret only around the bounded
Provider call, zeroes it afterwards, rejects redirects and proxy inheritance,
and returns only safe terminal reasons and numeric usage. Provider failure is
therefore local to the affected Agent and cannot silently fall back to the Pi
adapter or another Provider.

`CURRENT / KIMI AND MINIMAX LOOM-NATIVE ADAPTERS (2026-08-10)`: the strict
DeepSeek transport boundary is now a closed-descriptor OpenAI-compatible core
without exposing arbitrary endpoint or Model selection. Kimi uses the official
Moonshot `https://api.moonshot.cn/v1/chat/completions` endpoint with
`kimi-k2.6`; MiniMax uses the official
`https://api.minimaxi.com/v1/chat/completions` endpoint with `MiniMax-M3` and
`max_completion_tokens`. Each has a separate RuntimeInstance and Supervisor,
exact Provider/Account/Model/endpoint/credential-revision validation,
redirect-free and proxy-free bounded transport, secret clearing, numeric usage
accounting, safe Attempt diagnostics, and pre-Keychain cross-Provider
rejection. Verified-only runtime admission restores each Provider on cold
start or immediately after verify; rejected and unconfigured Providers do not
publish executable Profiles.

`CURRENT / MULTI-PROVIDER CONVERSATION PROFILES`: ordinary conversation now
publishes revision-frozen DeepSeek, Kimi, and MiniMax Profiles and routes each
through its exact fixed client after verified-revision admission. A stale
Profile fails before Keychain access, switching Profile still starts a new
conversation, and conversation binding is never inherited by a Team Agent.

`CURRENT / PROVIDER ACCOUNT PERSISTENCE`: broker commands now preserve an exact
Provider Account ID. Non-primary accounts use independent account-scoped
Journal streams and revisions, rebuild into an account-keyed projection, cross
the strict UDS command, and appear in the Swift setup snapshot as independent
non-secret account records. Legacy Provider credential events rebuild as the
matching `.primary` account and new `.primary` writes stay on the legacy stream,
so installed state migrates without rewriting Journal history. Agent credential
resolution now queries Provider plus Provider Account and rejects account,
reference, revision, or verification drift before Keychain access. Swift input
validation and installed operational diagnostics also preserve the non-secret
account identity.

`CURRENT / PROVIDER ACCOUNT MANAGEMENT UI`: the latest source Provider
directory shows the number of configured accounts for each Provider. Its native
credential sheet always exposes the migration `.primary` account, lists exact
additional account identities, creates a deterministic named account, and runs
connect, verify, replace, revoke, incident, and diagnostic actions against the
selected account only. Account operation state is keyed by the exact account;
the Store explicitly keeps primary operations on the legacy generic path and
routes only non-primary accounts through the account-scoped IPC contract. This
preserves installed-client compatibility while preventing a named account from
silently falling back to primary. Swift account and UI regressions cover stable
identity, display labels, primary routing, non-primary routing, and independent
projection refresh.

`CURRENT / NON-PRIMARY EXECUTION PROFILE PUBLICATION`: the dynamic Team catalog
now enumerates every exact verified DeepSeek Provider Account and publishes one
revision-qualified Coordinator Profile plus one Worker Profile for each. The
`.primary` Profile and RoleOption IDs remain migration-compatible; non-primary
IDs use a bounded stable account hash while the user-facing Builder continues
to show the exact Provider Account, Model, credential revision, timeout, and
limits. Catalog identity binds each sorted account, credential reference, and
revision. Revoking one account changes the catalog digest and removes only that
account's two RoleOptions, so every Agent can independently select another
still-verified account without a Team-level Provider switch.

Direct custom Execution Profile authoring for Model, reasoning effort, budget,
and timeout is source-complete; fallback editing and approval remain active W2D
work. Source now includes exact Harness-specific Codex + OpenAI and Claude Code
+ Anthropic Agent adapters alongside Loom Native DeepSeek, Kimi/Moonshot, and
MiniMax adapters. Custom endpoint Agent execution remains open. None of these
source contracts replaces the installed four-Provider Team or fresh-key live
gates, so this status does not change P2D-BLOCKER-1.

`CURRENT / AGENT ATTEMPT DIAGNOSTICS`: the DeepSeek Agent adapter writes a safe
terminal operational diagnostic for success, frozen-binding drift, credential
failure, Provider rejection, rate limit, transport failure, and timeout. It
preserves the request correlation ID plus non-secret Provider Account and Model
identity, stage, elapsed time, result, safe error code, and retryability in the
existing bounded owner-only store. Swift diagnostic preview/export now retains
those non-secret binding fields plus Agent reasoning effort while continuing to
exclude credential references, keys, prompts, conversation content,
Authorization headers, and raw
Provider bodies. Diagnostic persistence failure fails the adapter closed and
never becomes execution authority.

`CURRENT / INSTALLED BUILD 12`: Full Go tests, Go vet, and 148 Swift tests with
one intentional visual export skip pass. The deterministic native fixture
passes two Release builds, byte-identity checks, deep signature verification,
and launch smoke. The retained delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build12-2026-08-10/`.
Its Swift executable SHA-256 is
`a1d16ebd04600bfffb9ec01e1f6533eb4f4da34afc955380cd53c60b59e8a5fd`
and bundled daemon SHA-256 is
`0706b6ebafd44f55ddd9375b87153614a1ef7fc41995ba78eff70a6de7dc9f83`.
The installed App launches its bundled daemon as a child from the same bundle,
with an owner-only `0600` Socket. The installed helper passes
the real process Keychain put/read/delete test. A safe empty-secret UDS canary
returned `credential_unavailable`, `recoverable=true`, and
`stage=daemon_admission`; the build-12 matching `0600` operational record
carries the same Incident ID and stage, while the setup snapshot remains DeepSeek
`unconfigured`, revision zero, with no Provider Account or conversation
Profile.

`CURRENT / INSTALLED BUILD 13`: v0.5.2 build 13 adds versioned custom Agent
Execution Profile authoring and restart authority without changing the live
Provider claim. Serial full Go tests, Go vet, the real Go/Swift contract path,
149 macOS XCTest cases with one intentional visual-export skip, and four Swift
Testing contract cases pass with zero failures. The deterministic native fixture
passes two Release builds, byte-identity checks, deep signature verification,
and launch smoke. The retained delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build13-2026-08-10/Loom.app`.
Its Swift executable SHA-256 is
`85f0f1f28b3ef271272fb77a14778c7ea449e8c63b3d863dddfee58d04dac8ab`
and bundled daemon SHA-256 is
`1fe9a615e32795ca85c7cb084c28f10b6b9d9808aa389ff7b6dba1b5c6e2e1aa`.
The transactional installer preserved build 12 as the rollback bundle and the
installed hashes match the retained delivery. Opening the App starts its bundled
daemon child; the installed UDS reports daemon `serving_request`, Journal
`available`, projection `current`, and non-partial state. The installed process
Keychain helper passes isolated synthetic put/read/delete, and CoreGraphics
confirms the Loom main window is on screen. No real Provider credential or live
Provider request was used for this build acceptance.

`HISTORICAL / BUILD 14 REJECTED BY INSTALLED CHECK`: build 14 carried the first
Kimi/MiniMax adapter Candidate, but a real installed `status` read exposed that
native Runtime admission still depended on a later Mission composition branch.
It was not accepted as the retained installed Candidate. The repair moved
admission to verified Provider lifecycle boundaries and added a daemon cold
start composition regression without rewriting or deleting Journal history.

`CURRENT / INSTALLED BUILD 15`: v0.5.2 build 15 is installed from
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build15-2026-08-10/Loom.app`.
The Swift executable SHA-256 is
`802589380ff4604a769650d3bd52937daa61235875efa850a59893bfaa381013`
and the bundled daemon SHA-256 is
`3796078eaa9d46a9da4a2122abc59310ebb301062c5b7516f703455514305ef6`;
installed and retained bytes match and deep signature verification passes.
Opening the App starts that bundle's daemon child, and the installed UDS reports
daemon `serving_request`, Journal `available`, projection `current`, and
non-partial state. Socket and operational diagnostics are owner-only `0600`.
An opt-in read-only installed test confirms DeepSeek, Kimi, and MiniMax are in
the Provider directory and that any verified Provider must have its exact
revision Conversation Profile plus admitted runtime. The installed process
Keychain helper synthetic put/read/delete test also passes. No real API key or
Provider generation request was used.

`CURRENT / SOURCE VERIFICATION (2026-08-10)`: serial `go test -p 1 ./...`,
`go vet ./...`, the real Go Server/Swift contract probe, and all 149 macOS Swift
tests pass with one intentional visual-export skip and zero failures. The
mixed-Team canary now proves that an OpenAI account's
`provider_rejected` terminal leaves the Anthropic and DeepSeek Agents succeeded
with their independent frozen bindings intact; recovery retries only the failed
Agent. Provider Account accounting also passes authority, replay, projection,
API, Pi RPC, Swift strict-decoding, and UI tests. Explicit fallback governance
also passes source/target binding, approval digest, recovery replay, dispatch,
unapproved change, forged target, and future approval regressions. The source
candidate also passes independent Agent Profile selection regressions: safe
Runtime Profile fields cross Go projection and strict Swift decoding, the
Builder sends a role-local edit, and changing one Agent does not overwrite its
peer. Reasoning-effort regressions additionally lock the legacy v1 binding
digest, require an explicit Runtime capability, freeze a domain-separated v2
binding, reject post-freeze mutation, and carry the value through Team/Run
authority, Journal replay, API, strict Swift decoding, Board UI, and diagnostic
export. Supervisor and Pi/Pi RPC regressions also prove that the adapter receives
the authoritative frozen binding and that Profile or Provider/Model drift fails
before process start. DeepSeek adapter regressions additionally prove exact
binding use, verified-only catalog publication, exact projected credential
revision lookup, per-binding Supervisor routing, Agent-local failure reasons,
secret clearing, safe Attempt diagnostics, and numeric usage propagation.
Kimi and MiniMax regressions prove their exact official endpoint, Model, token
limit field, response Model, usage accounting, diagnostics, cross-Provider
pre-Keychain rejection, independent RuntimeInstance/Supervisor routing,
verified-only Profile publication, verify-time admission, and cold-start
restoration. Conversation regressions prove exact verified revision routing for
all three brokered Loom-native Providers and stale-Profile rejection before
secret access.
Provider Account regressions also prove independent same-Provider streams and
revisions, legacy `.primary` replay, account-scoped projection and adapter
resolution, strict UDS transport, Swift snapshot decoding, and safe account
diagnostics. Swift Store and UI regressions additionally prove primary versus
non-primary dispatch, named-account identity, account selection, and safe
account-local operation presentation. This verifies the current W2C/W2D and
first W3 source candidate. Daemon catalog regressions also prove that two
verified DeepSeek accounts publish independent revision-frozen Agent Profiles
and that revoking one account preserves the other's RoleOptions and changes the
catalog digest. Custom Profile regressions additionally prove independent
Model/reasoning/timeout/budget editing, immutable ID derivation, invalid-edit
rollback, exact StateWriter matching, legacy optional-snapshot replay, deep-copy
projection, restart reopening, daemon materialization, and same-ID drift
rejection. This is now an installed synthetic-path and process-Keychain
acceptance, not a live Provider acceptance. Build 15 contains the W3 DeepSeek,
Kimi, and MiniMax slices plus account publication, but no fresh real Provider
credential or Provider network reply was used; the fresh-key gate below
remains open.

`CURRENT / LIVE CREDENTIAL PATH ACCEPTED`: the installed v0.5.2 build 16 now
accepts a real DeepSeek credential. The authoritative setup snapshot reports
both `deepseek` and `deepseek.primary` verified at revision 3 and publishes
`conversation-deepseek-deepseek-chat-r3`. Configure and verify complete through
`projection_refresh`; the Keychain/bootstrap/metadata/Profile-publication part
of P2D-BLOCKER-1 is no longer the active failure.

`CURRENT / P2D-BLOCKER-1 ROOT CAUSE (2026-08-10)`: incident
`loom-swift-663d309b-d1f2-4ee6-9e53-9e861119174f` and two matching retries
prove the installed Swift request reached the bundled daemon and failed in
40-47 ms before Provider verification. The daemon's real OS argv remained
`--local-app-service --parent-pid ...`; bootstrap expansion changed only the
slice passed to `run`. The process Keychain helper therefore correctly rejected
the parent because its attestation could not find canonical `--state`,
`--isolation-root`, and `--socket` arguments. `setupCredentialResult` then
erased the original `helper_authorization` stage by returning a bare store
error, causing the UI-facing failure to degrade to `daemon_admission`. No API
key reached Keychain or DeepSeek, so this incident does not implicate the key.

`ACTIVE / BLOCKER REPAIR ORDER`: re-exec the bundled daemon with canonical
non-secret argv while preserving the App parent lifecycle; preserve staged
credential errors through Setup and IPC; prove the real Darwin bootstrap and
process-helper put/read/delete path without weakening executable identity,
Socket owner, or peer-PID attestation; then package a new installed Candidate
for the unchanged fresh-key live gate. API keys, credential bodies, prompts,
and Provider responses must never enter argv or diagnostics.

`CURRENT / INSTALLED BUILD 16 BOOTSTRAP REPAIR`: v0.5.2 build 16 is installed
from
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build16-2026-08-10/Loom.app`.
Its Swift executable SHA-256 is
`c329691c707bc85f1d9ef37d95ef1f652ef0e95f5075b8ff0057eead79aef07a`
and bundled daemon SHA-256 is
`efc16e1e29d7ddc0cb28e871ce27d92149170d7e6da6b5c3ab3ccc38a7b20092`;
installed and retained bytes match and deep signature verification passes. The
App-started daemon's real kernel argv now contains canonical `--state`,
`--isolation-root`, Runtime, and owner-only `0600` Socket arguments followed by
one validated `--managed-parent-pid`. The internal flag is removed before
`run`, while the App parent still owns daemon lifecycle. The installed daemon
reports `serving_request`, Journal `available`, projection `current`, and
non-partial state.

The revised installed regression starts that bundled daemon through the real
`--local-app-service --parent-pid` entry and completes synthetic process
Keychain helper put/read/delete. The exact same regression against build 15
fails at `credential_unavailable / daemon_admission`, proving the test no longer
bypasses bootstrap. Classified credential errors also preserve their original
safe helper/Keychain stage through Setup and IPC. Serial full Go tests, Go vet,
149 XCTest cases with one intentional visual-export skip, four Swift Testing
contracts, two deterministic Release builds, signature checks, and launch
smoke pass. Those package-time tests did not use a real Provider key or
generation request. The subsequent user credential attempt supplied the live
configure/verify evidence above; a real DeepSeek conversation reply is still
outstanding.

`ACTIVE / P2D-W2A CONVERSATION SEND BLOCKER`: the installed App can retain a
persisted Codex thread anchor while its Swift store has not yet loaded that
thread. Selecting the verified DeepSeek Profile then reused the old thread ID.
The daemon correctly rejected the same-thread Profile mismatch before user
message persistence and Provider dispatch, but Swift silently caught the error,
so the user saw a message that did not send and no recovery details.

Installed build 17 rotates the thread anchor whenever a non-empty selected
Profile changes, even before the old thread loads; its generation guard prevents
stale asynchronous Codex loads from overwriting the user's DeepSeek selection.
It preserves the draft and displays an inline safe stage, Retry, View
diagnostics, and Copy incident ID actions. App and daemon `chat_message`
diagnostics correlate by Incident ID and safe thread/Profile metadata without
recording message content. This remains the installed fail-closed compatibility
boundary and has not received a real DeepSeek reply.

`TARGET / ACCEPTED CONVERSATION ROUTE ARCHITECTURE`: P2D-W2A is extended by
`.loom-evidence/phase2d/contracts/P2D-W2A-conversation-route-segments.md`.
The target keeps one user-visible Loom Conversation while each Harness,
Provider, Provider Account, Model, or credential-revision transition creates an
immutable Conversation Segment and one or more single-binding Agent Attempts.
A stable Agent owns a versioned RouteSet of compatible Execution Profiles;
parallel routes use sibling Attempts plus an Aggregation Attempt, and fallback
uses an explicit approved Route Transition. Provider-native handles are bound to
Provider, account, Model, Segment, and credential revision and are never reused
across those boundaries.

Each transition builds a target-specific, content-addressed Context Capsule
from authoritative, observed, and explicitly untrusted sources. Classification,
redaction, ACL, target-account disclosure policy, deterministic token packing,
and an omission manifest run before a ContextAdapter dispatches the new Segment.
Hidden reasoning is never transferred, prior model output cannot become
authoritative history, and every Attempt freezes both Capsule and Execution
Binding digests. The UI offers Continue with context, Summary only, and Start
clean, and exposes only safe disclosure categories, omissions, and receipt.
W2B/W2C reuse this as per-Agent RouteSet and Role Capsule dispatch; W2D owns
disclosure/retention governance and encrypted transcript/Capsule/native-handle
storage using per-Conversation DEKs wrapped by the domain-separated Loom Vault
key hierarchy. The current
owner-only plaintext chat file is explicitly not an encryption-at-rest claim.

`CURRENT / SOURCE SEGMENT VERTICAL CANDIDATE`: the source chat authority now
uses persistence schema 2 and keeps one visible thread while appending immutable
Segments when Profile identity changes. Each routed call records a single
Attempt with Profile, Segment, Context mode, content-addressed Capsule digest,
Binding digest, and terminal status. Swift sends the explicit route mode without
rotating the thread anchor, fences stale loads, decodes Segment/Attempt state,
and labels each routed turn with Harness, Provider Account, and Model. The
compact route menu offers Continue with context, Summary only, and Start clean.
Summary mode admits recent user input and omits prior model output; Continue mode
places prior model output only inside an explicitly untrusted policy-filtered
Capsule, never as target-Provider assistant history. Profile conflict remains
fail closed when a caller omits the transition mode. Schema 1 files migrate to a
single start-clean Segment. This source Candidate passes full `internal/api` and
`internal/app`, full `internal/localipc`, focused daemon routes, API race checks,
and 153 XCTest plus four Swift Testing contracts. It is not installed or live
accepted; full structured Capsule fields, omission/disclosure receipts,
Provider-native handle isolation, and encrypted storage remain W2A/W2D work.

`CURRENT / SUPERSEDED BUILD 17 CHAT REPAIR CANDIDATE`: v0.5.2 build 17 was
installed and running from `/Users/lune/Applications/Loom.app`; the retained
delivery is
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build17-2026-08-10/Loom.app`.
The installed Swift executable SHA-256 is
`58e3e6fbf56c5b8323b3d063bf3c1f16d8b516e3ccc1074ffa1084da332c03dd`
and bundled daemon SHA-256 is
`934a59b95b3401750a6c7d83eb031c5fb2a7acfcf12a19af71f9ccd3a65883fe`;
retained and installed bytes match and deep signature verification passes.

Opening the App automatically starts the installed helper as its child with
canonical state/isolation/Runtime/owner-only Socket arguments and the validated
managed-parent PID. The Socket is `0600`; no manual `loomd` start is required.
Journal replay after installation still reports DeepSeek verified revision 3,
and the daemon restores the Loom Native `deepseek-chat` Runtime. The source and
package gates pass Go vet, full `internal/api`, `internal/localipc`, and
`cmd/loomd` packages, 153 XCTest cases with one intentional visual-export skip,
four Swift Testing contracts, two deterministic Release builds, signature
checks, and launch smoke. No paid Provider generation was issued during these
gates. P2D-BLOCKER-1 remains open only for the user-confirmed installed Profile
switch, new DeepSeek thread, correlated `conversation_dispatch`, and real reply.

`CURRENT / INSTALLED BUILD 20 VAULT CANDIDATE`: v0.5.2 build 20 is installed
transactionally from
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build20-2026-08-11/Loom.app`;
build 19 remains at `/Users/lune/Applications/Loom.app.previous`. Strict deep
signature verification passes and the installed bytes match the candidate:
`LoomLocalApp` SHA-256 is
`a2fcf42e380ff710b3dfaca59692364d2297f8dd786cd8fc95860950a544f8a6`
and bundled `loomd` SHA-256 is
`7e1348c184f365a64665c36ab2c30f7a97439f4d13cd009a29ecb1f7c92f5ef6`.
The App is parented by launchd and starts its canonical child daemon with
`--state`, `--isolation-root`, `--socket`, and `--managed-parent-pid`; the
daemon is parented by the App. The run directory and active UDS are owner-only
`0700`/`0600`. The default LocalKeyFile Vault starts automatically with
owner-only `private/vault.key` and `state/credential-vault.db` at `0600` beneath
`0700` directories.

The installed `setup_snapshot` first projected the pre-Vault DeepSeek account
as `migration_required / vault_entry_missing` and suppressed its unusable
Conversation Profile. The user then completed the explicit `Move key to Vault`
action with a real DeepSeek credential. Installed operational diagnostics show
`credential_replace` followed by two `credential_verify` operations succeeding
at `projection_refresh`; the authoritative snapshot now reports the exact
DeepSeek account `verified` at revision 6 and publishes
`conversation-deepseek-deepseek-chat-r6`.

A real App/managed-daemon stop and restart changed both PIDs while preserving
the owner-only Vault file identities. The restarted daemon automatically
unlocked LocalKeyFile, restored verified revision 6 and the r6 Profile, and no
`--credential-helper` process was present. The post-restart Conversation menu
offers DeepSeek and accepts it as the current selection without sending a
Provider request. This accepts installed Vault import, verify, Profile
publication, and restart continuity. It does not yet accept an r6 Provider
reply, multi-turn no-helper observation, or the mixed-Team live matrix.

`CURRENT / R6 LIVE ATTEMPT FAILED; FALSE-SUCCESS DIAGNOSTIC CORRECTED IN
SOURCE`: the first post-restart r6 message created immutable `segment-3` with
Profile `conversation-deepseek-deepseek-chat-r6`, `summary_only` context, and a
new frozen binding/capsule. Incident
`loom-chat-d6873ebe-e5f7-41b5-9b24-a204688ca2bb` reached the profile-bound
Conversation Attempt, but authoritative `attempt-3` ended
`failed / conversation_unavailable`; the stored Loom message is the fixed local
fallback, not a DeepSeek reply. No credential helper was active. Build 20's App
and daemon operational records incorrectly marked the IPC transaction
`succeeded` because the thread response itself was returned with `OK`, masking
the failed Attempt and leaving lease versus Provider HTTP/auth/rate-limit
indistinguishable. Therefore the r6 live gate remains failed, not passed.

The current source Candidate closes that observability gap. OpenAI-compatible
Conversation clients now classify timeout, DNS, TLS, connect, HTTP auth,
rate-limit, rejected request, server availability, and invalid response without
retaining Provider bodies. The router preserves that safe stage/code/retryable
tuple; each Conversation Attempt freezes it with the end-to-end Incident ID;
daemon and Swift diagnostics derive the terminal result from that Attempt even
when IPC returns the persisted thread. Swift keeps the authoritative failed
thread and restores the draft while showing a specific recovery banner and
Incident action. This is source evidence only until a newly installed candidate
retries r6 and exposes the actual failure class.

`CURRENT / INSTALLED BUILD 21 STAGED-FAILURE DIAGNOSTIC CANDIDATE`: v0.5.2
build 21 is transactionally installed from
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build21-2026-08-11/Loom.app`;
build 20 is the strict-signature rollback bundle. Candidate and installed bytes
match: `LoomLocalApp` SHA-256 is
`d323b8ac87c1d3cf670ce4240ce7c5b4a99e2aa139c792f6fd763d246f416790`
and bundled `loomd` SHA-256 is
`8241612d0e4fd58ca9a9a5a5fd375af9ef95f2c2b0785416d96886c0a46c2b1c`.
The App starts the managed canonical daemon, preserves the existing `0600`
Vault file identities, auto-unlocks LocalKeyFile, restores DeepSeek and
`deepseek.primary` as verified revision 6, and republishes Profile r6. No helper
process is active. The UI is open with DeepSeek selected, but no Build 21
Provider request has been sent; installed failure classification and the actual
r6 root cause remain unverified until the user retries once.

`TARGET / ACCEPTED CREDENTIAL VAULT ARCHITECTURE`: ADR-0020 and
`.loom-evidence/phase2d/contracts/P2D-W2D-credential-vault.md` add CV1-CV6 under
the existing W2D. A Loom-owned Vault replaces ProductKeychainStore beneath the
existing `SecretStore` interface. LocalKeyFile is the first automatic-unlock
mode: `loomd` loads one separate owner-only VMK at startup; random per-revision
DEKs encrypt credential bytes with AES-256-GCM and are wrapped with an
HKDF-SHA256 credential-domain KEK. Conversation and Agent hot paths use exact
account/reference/revision leases and never launch a helper or read Keychain.
Keychain remains only an optional, explicit, one-time migration source; re-entry
is the default. There is no plaintext fallback. The honest threat model protects
a copied Vault database without its key file but does not claim resistance to
the same macOS user, root, or live daemon-memory compromise.

`CURRENT / VAULT CV1 FROZEN; CV2-CV4 PRODUCTION SOURCE CANDIDATES`: focused RED
first failed on absent Vault symbols. `internal/credentials/vault` now contains
the versioned AES-256-GCM per-revision envelope, HKDF-SHA256 credential,
conversation, and export domains, canonical account/reference/revision AAD,
owner-only LocalKeyFile loading, encrypted SQLite storage with frozen file
identity, exact leases, pending mutation transactions, restart reconciliation,
and configure/verify/replace/revoke coordinators. Metadata conflict rolls back
pending ciphertext; ambiguous post-commit failures remain recoverable pending
records; replace can explicitly import a re-entered secret when authoritative
legacy metadata exists but the Vault row does not. Revoke and revision changes
invalidate only the exact old lease.

Production `run.go` now enables `UseCredentialVault`. The daemon owns one Vault
runtime, injects its gated mutator into setup and its gated exact lease access
into Conversation and Agent dispatch, and closes it before the Journal database.
Normal production setup therefore does not construct ProductKeychainStore;
the only direct SecretStore read left in daemon hot-path code is the explicitly
named legacy adapter used by tests and optional migration work. Agent and
Conversation lease failures retain `credential_lease_issue`, and operational
diagnostics plus Swift wire decoding accept the complete closed Vault stage set.
Provider operation idempotency now resolves the exact Provider Account stream,
not a Provider-only global stream.

`PARTIAL / CV5 STATUS, MIGRATION UX, AND ROTATION SOURCE CANDIDATE`: setup
compares each authoritative Provider/Account reference and revision with the
encrypted Vault row. Missing
rows project `migration_required / vault_entry_missing`, suppress the unusable
Conversation Profile, and retain the authoritative Journal fact. Vault access
failure projects `recovery_required / vault_unavailable`. The Provider sheet
names Loom Credential Vault, explains re-entry, and performs explicit import
plus verification as one user action. Setup now also publishes a strict,
non-secret top-level Vault projection with schema, active storage mode, aggregate
status, and migration/recovery account counts; Runtime & Providers renders that
state with a real privacy-safe diagnostics action. A classified LocalKeyFile
load/open failure now keeps the daemon and Setup UI available in a restricted
recovery mode: all credential mutation and lease paths fail closed with the
original Vault stage, caller secret bytes are cleared, and no Keychain fallback
is constructed. Unclassified construction failures still stop startup.
The loaded key material also retains the exact key-file device/inode identity.
Every VMK lease and Vault health/mutation boundary revalidates that identity
without rereading key bytes; live deletion or path replacement immediately
projects recovery and rejects reads, deletes, and pending transactions.
CV5 now also has a source-level LocalKeyFile rotation vertical slice. It stages
an explicit next-version key without replacement, transactionally rewraps every
active DEK and advances Vault metadata without re-encrypting credential bytes,
rejects concurrent pending
credential mutations, and preserves the old key and rows on pre-commit failure.
A fixed `vault.key.rotation-pending` startup protocol promotes a committed key
or removes an uncommitted key; any third state fails closed at `vault_rotation`.
The lease generation barrier revokes active plaintext and rejects in-flight old
generation acquisitions. The daemon UDS and Swift Runtime & Providers row expose
a real Rotate key action with extended timeout, safe operational diagnostics,
stage, and Incident ID; success refreshes the authoritative Setup Snapshot.
Interactive Lock now / Unlock is also a source Candidate. Lock closes the lease
manager and VaultStore, revokes and zeroizes active plaintext leases, releases
the in-session VMK, and projects `locked`; credential mutation and dispatch then
fail closed. Unlock revalidates rotation and key-file identity, reconstructs the
encrypted store, lease access, verifier, and coordinator, reconciles pending
mutations, and only then projects `unlocked`. Strict private UDS methods use
extended deadlines and safe Incident diagnostics; Swift refreshes the
authoritative Setup Snapshot instead of inventing local state, and displays
stage plus Incident ID on failure. Recovery reset is now a source Candidate:
it is exposed only for `recovery_required`, requires an explicit destructive UI
confirmation and an exact private UDS confirmation, validates all canonical
Vault files before crypto-erasure, fsyncs both owner-only directories, and
rebuilds the Vault runtime in place without an App or daemon restart. It never
rewrites old credential metadata into a usable state. Existing Provider
Accounts instead project `migration_required`, retain their non-secret account
identity, and require API-key re-entry. Unsafe paths, normal unlocked/locked
Vaults, repeated reset, and incorrect confirmation fail closed with
`vault_recovery` plus Incident ID. Optional one-time Keychain helper migration
and the complete installed support flow remain open.

Encrypted export is now a source Candidate rather than an inert UI promise.
The native sheet requires a matching 12-1024-byte passphrase and user-selected
`.loomvault` destination. loomd applies Argon2id with a random salt, derives
separate DEK and bundle keys beneath `loom/export-wrap/v1`, unwraps and rewraps
each credential DEK without creating an aggregate plaintext API-key archive,
then encrypts the complete manifest so Provider Account metadata is not exposed.
It never exports the VMK or `vault.key`. Export is bounded to 256 credentials
and 4 MiB, rejects pending mutations, clears the daemon passphrase buffer, and
publishes a new `0600` file with `O_EXCL` and directory fsync. Wrong passphrase,
tamper, unsafe destination, overwrite, and unknown versions fail closed at
`vault_export`; operational diagnostics retain Incident ID but exclude the
passphrase and destination. Source format tests prove exported DEKs can be
rewrapped under a new VMK with exact account/reference/revision binding. Restore
UI is not claimed by this slice.

The current source passes full relevant credentials/app/api/localipc/runtime/
daemon packages, focused Vault and daemon race checks including key identity
drift, live key-path replacement, rotation/restart, malformed-key recovery, and
explicit crypto-erasure/re-entry and encrypted export/rewrap,
Go vet, diff
checks, and 167 XCTest cases with one intentional visual-export skip plus six
Swift Testing contracts. Installed build 20 now proves automatic LocalKeyFile
Vault startup, owner-only storage, fail-closed legacy projection, real re-entry,
verify/Profile publication, and restart auto-unlock. It does not yet prove the
r6 live Conversation hot path or mixed-Team dispatch, so no completed
Keychain-removal or CV6 claim is made.

`NEXT / ORDERED PHASE 2D EXECUTION`: package and install the staged-failure
Conversation Candidate, then retry one user-approved DeepSeek r6 message. The
result must expose one safe lease/provider stage and Incident ID; fix that
specific root cause until a real reply is persisted, then repeat for multi-turn
no-helper observation. Preserve W2B/W2C multi-Agent bindings and complete the
four-pair mixed-Team isolation matrix before CV6 can close. The one-time
Keychain helper migration remains optional and outside the normal
Conversation/Agent hot path.

`CURRENT / FOUR-PROVIDER TEAM SOURCE CANARY`: the Phase 1 three-Agent ceiling
was a real blocker for the accepted Phase 2D mixed-Team matrix. TeamDefinition,
ExecutionPlan, coordinator admission, authoritative dispatch/replay, and
projection replay now share one nine-Agent maximum while retaining one main
Agent and at most three Attempts per node. Boundary regressions accept nine and
reject ten, so creation and replay cannot disagree about Team size.

The controlled source canary now executes one Team with Codex + OpenAI,
Claude Code + Anthropic, Loom Native + Kimi, and Loom Native + MiniMax. Every
Attempt freezes a distinct Harness, Provider Account, Model, credential
reference/revision, endpoint, limits, capabilities, and binding digest. A
synthetic `provider_rejected` failure on the OpenAI Agent leaves the other three
Agents succeeded and concurrently runnable; only the failed Agent follows its
versioned recovery path. Full `teams`, `work`, `projection`, and `app` tests plus
the focused race test pass. This is source-level orchestration evidence only:
real-account preflight, installed adapter dispatch, account accounting, Team
board projection, and the user-observed four-Provider live matrix remain W2C/W2D
exit gates.

`CURRENT / PRODUCT MISSION MULTI-ROLE COMPILER`: the product Mission entry no
longer collapses a confirmed saved Team to one hard-coded local Pi main Agent.
`MissionExecutionBinding` now carries one role-local binding per saved role;
the compiler emits a canonical one-to-nine-node DAG, two bounded Attempts and
one independent verifier semantic per role, and preserves each role's exact
Harness, Runtime, Provider Account, Model, credential revision, limits, and
capabilities. Legacy single-main Journal lineage retains its prior verifier
identity and remains restart-recoverable. Multi-role projected executions are
also reconstructable after daemon restart.

The projection binding source consumes the exact versioned
`TeamDefinition.Configuration.RoleBindings` snapshot when it matches the saved
Team definition digest. Main uses its authoritative AgentInstance; dormant
subagents receive deterministic execution identities without being falsely
projected as active Team instances. The stale two-subagent instantiation and
projection ceiling is removed; all saved-Team boundaries now admit the shared
nine-Agent maximum.

`CURRENT / AGENT PREFLIGHT GOVERNANCE`: Mission preflight projects one safe row
per Agent with Harness, Provider, Provider Account, Model, credential revision,
reasoning effort, timeout, budget, capabilities, status, and actionable block
reason. Credential reference and endpoint fingerprint are intentionally absent
from the wire and UI. Exact Provider Account lookup requires the configured
reference and revision to remain verified. A missing, revoked, rejected, or
revision-drifted credential marks only that Agent `blocked`; peer Agents remain
`ready`. Runtime offline, capability mismatch, and exhausted capacity use the
same role-local fail-closed path. Swift retains the blocked preflight for review
but refuses Mission Start until every Agent is ready.

Source verification covers a Codex/OpenAI main plus Loom/DeepSeek subagent,
four role-specific Attempt records, multi-node restart reconstruction, exact
saved-Team projection replay, and DeepSeek credential revocation isolation.
Full `teams`, `projection`, and `app` packages and the strict Swift suite pass.
This is not installed-live acceptance: packaged multi-Agent dispatch, actual
Provider replies, account rate-limit/timeout isolation, accounting, fallback
approval, and Team Board incident projection remain open.

`CURRENT / AGENT ATTEMPT INCIDENT BOARD CANDIDATE`: the Team projection now
retains the privacy-safe correlation ID for each current Attempt dispatch
generation beside its frozen Execution Binding. A rebound advances only that
Attempt to the new generation Incident; a later terminal operation does not
replace an already bound dispatch Incident, preserving adapter-diagnostic
correlation while remaining compatible with historical Journal operations that
used a separate terminal correlation. Team Board exposes the value only when it
matches the strict 1-64 character IPC Incident grammar. It still omits
credential reference, endpoint fingerprint, Prompt, Provider body, and
diagnostic content.

Swift strictly decodes the same grammar, shows the first eight characters next
to the affected Agent's terminal reason, and provides an icon-only Copy
incident ID action in Team Pulse. Invalid or control-bearing IDs fail closed in
Go/Swift tests. Full `projection`, `api`, and `app` packages, focused race and
vet, the real Go/Swift timeline contract, the focused daemon execution IPC
contract, 167 XCTest cases with one intentional visual-export skip, and six
Swift Testing contracts pass. This is a source Candidate and is not present in
installed build 20; packaged Agent-failure observation remains an exit gate.

`CURRENT / FALLBACK BOARD PRESENTATION CANDIDATE`: each Team Board Agent row now
projects whether fallback is configured, whether recovery approval is required,
whether a versioned approval is available, that approval's safe version, and
whether the one-shot fallback has been consumed. The Board intentionally omits
the workflow fallback key, approval ID, actor, source/target binding digests,
and approval digest. Swift rejects inconsistent combinations and renders one
bounded state beside the Agent binding: `Fallback configured`, `Fallback
approval required`, `Fallback approved vN`, or `Fallback executed`.

Focused API and race tests, full `projection`, `api`, and `app` packages, the
105-second Go/Swift IPC contract, 168 XCTest cases with one intentional visual
export skip, and six Swift Testing contracts pass. This is source-only and is
not present in installed build 21. It closes fallback status visibility, not
Agent fallback editing or interactive approve/reject. Those journeys, installed
fallback presentation, and the four-Provider live matrix remain active W2D
gates. Build 21 still has no new DeepSeek chat event; the user-triggered r6
message and real reply or exact correlated failure remain P2D-BLOCKER-1's next
live gate.

`CURRENT / AGENT ROUTESET AUTHORING CANDIDATE`: the saved Team configuration
now carries an optional version-1 fallback Route per Agent role. The route
freezes its own Runtime Profile, Runtime instance, Harness, Provider Account,
Model, endpoint fingerprint, credential reference/revision, reasoning effort,
timeout, budget, and capabilities. It must target the same AgentDefinition and
role kind as the primary binding, must use a distinct Runtime Profile, and is
accepted only with `approval_required=true`. Historical Team events remain
compatible because the route is optional. The Builder binding digest includes
the RouteSet, so a fallback edit invalidates stale confirmation instead of
silently changing an accepted Team.

Builder editing now offers an independent fallback menu on every Agent. It
lists only same-Agent, same-role compatible execution Profiles and displays the
selected Harness, Provider Account, Model, credential revision, and explicit
approval-required state. `No fallback` is a real edit, not a presentation-only
toggle. Swift accepts old responses as unconfigured but rejects contradictory
new wire states such as a configured route without required approval. Saved
Teams restore both primary and fallback Profiles after daemon restart; when the
live catalog no longer contains a frozen Profile, the Builder reconstructs a
bounded saved option from the authoritative non-secret execution snapshot.

Focused state/projection/app tests, full affected Go packages, focused app race
tests, the real Go/Swift UDS contract, and the complete Swift suite pass: 169
XCTest cases with one intentional visual-export skip plus six Swift Testing
contracts. This is source-only and is not installed in build 21. It completes
RouteSet authoring and persistence, not approval authority. Versioned
approve/reject commands, installed Board/Builder verification, and live fallback
execution remain active W2D gates. Build 21 still has no
post-`loom-chat-d6873ebe...` user-triggered
DeepSeek event, so P2D-BLOCKER-1 remains open.

`CURRENT / AGENT FALLBACK PREFLIGHT CANDIDATE`: the Mission binding source now
resolves the saved fallback Route independently for each Agent. It validates
the exact fallback Harness/Runtime, Provider Account, credential reference and
revision, Model, limits, and capabilities through the same fail-closed binding
path as the primary Profile. Credential revocation, Runtime offline state,
capability mismatch, and capacity exhaustion produce a fallback-local blocked
reason without changing a ready primary Agent or any peer Agent.

Mission preflight projects the fallback Harness, Provider Account, Model,
credential revision, capabilities, ready/blocked state, actionable reason, and
approval requirement under the owning Agent row. Swift accepts historical
preflight responses as unconfigured and rejects partial or unapproved new
fallback facts. A blocked fallback does not prevent primary Mission Start.
Conversely, the compiler does not create a changed Attempt or RecoveryPolicy
from configuration alone: both bounded Attempts retain the primary binding
until a separate authoritative `TeamFallbackApproval` exists.

The four-Provider concurrency canary also now waits for all three independent
subagents before releasing its barrier. Its previous two-arrival barrier made
the asserted three-way peak scheduler-dependent; twenty repeated runs pass
after correcting the test boundary. Full affected Go packages, focused race,
diff checks, 169 XCTest cases with one intentional visual skip, six Swift
Testing contracts, and the real Go/Swift UDS suite pass. This remains a source
Candidate outside build 21. Persistent prepared approve/reject authority, Board
projection from a live decision, and installed execution remain open.

`CURRENT / AGENT FALLBACK APPROVAL COMPILER SEAM`: the Mission compiler now
accepts an optional approval source queried by exact Team instance, Plan digest,
logical Agent node, primary binding digest, and fallback binding digest.
Configuration alone still keeps Attempt 2 on the primary binding. Only a valid,
non-future `TeamFallbackApproval` whose source and target digests exactly match
the frozen current bindings creates RecoveryPolicy v2 and materializes Attempt
2 on the fallback Harness, Provider Account, Model, credential revision, limits,
and capabilities. The dispatch frame uses that fallback Runtime instance and
the semantic binding carries the same approval fact for existing Journal and
recovery validation.

A forged target digest fails closed. If the approved fallback later becomes
unavailable, preflight blocks only its owning Agent with the fallback reason;
peer Agents remain ready and Mission Start is rejected. Preflight and Swift now
show only safe approval availability/version, and reject contradictory approval
states. Deep cloning covers both primary and fallback capability lists. Focused
Go compiler tests and strict Swift model/UI tests pass. Full affected
state/projection/app/API packages, focused app race, Go vet, 169 XCTest cases
with one intentional visual-export skip, six Swift Testing contracts, the real
Go/Swift UDS contract, and diff checks also pass. This is a source seam, not a
durable approval product: the production source that writes and resolves
versioned preflight approve/reject Journal facts, its Decision Sheet actions,
and installed fallback execution remain the next W2D slice.

`CURRENT / INSTALLED BUILD 22 OBSERVATION (2026-08-11)`: direct bundle and
process inspection now reports `/Users/lune/Applications/Loom.app` v0.5.2 build
22 with its managed canonical bundled daemon running. The newest App and daemon
operational records are still incident
`loom-chat-d6873ebe-e5f7-41b5-9b24-a204688ca2bb` from the earlier r6 attempt; no
build-22 `chat_message` event has been observed. The provenance of the external
install change from build 21 to build 22 has not been established in this task,
so no package/hash or live-fix claim is made. P2D-BLOCKER-1 still requires the
user-triggered DeepSeek message and a real reply or one newly correlated, safely
classified failure.

`CURRENT / PERSISTENT FALLBACK DECISION AUTHORITY CANDIDATE`: W2D now has an
immutable version-1 fallback decision Scope whose digest binds Team instance,
Plan digest, logical Agent node, primary binding digest, and target binding
digest. A dedicated Journal stream per Scope uses revision CAS. Approve creates
a `TeamFallbackApproval` whose version equals the decision revision; Reject
advances the same stream without carrying an approval, so a later reject removes
the earlier route's execution authority without deleting history. Payloads are
closed, non-secret facts and contain no credential reference, endpoint, Prompt,
API key, or Provider body.

Projection replay strictly reconstructs and verifies the Scope, actor, decision
time, approval digest, and source/target binding. It preserves historical nil
snapshot shape when no decision events exist, survives restart, and exposes only
the latest exact Scope. The production Mission binding source now implements
`MissionFallbackApprovalSource`; a Plan or binding drift cannot resolve a
foreign approval, and a projected Reject returns no approval. The installed
daemon compiler source wiring is present; optional side-task compilers consume
it only when their parent source supports the authority interface.

The shared Mission Decision backend and native macOS sheet now support a fourth
`fallback` kind with explicit Approve fallback, Reject fallback, and Not now.
Not now writes nothing. A prepared submit injects the current Incident ID and
authority UTC time, commits the exact frozen Scope/revision, refreshes the
projection, and only then reports an authoritative result. Full affected Go
packages, targeted race, daemon regression, 169 XCTest cases with one
intentional visual skip, six Swift Testing contracts, and the real Go/Swift UDS
contract pass.

`CURRENT / DYNAMIC FALLBACK PREFLIGHT DECISION PRODUCER CANDIDATE`: Mission
compilation now emits one non-secret decision Candidate for every configured
Agent fallback. Each Candidate contains only Mission identity, the immutable
Team/Plan/Agent/source/target Scope, Agent title, Harness, Provider, Provider
Account, Model, and credential revision; credential reference, endpoint,
Prompt, Provider body, and secret bytes are absent. Authoritative preflight
registers these Candidates in the shared prepared-decision backend without
writing the Journal. Registration atomically replaces only the same Team's
older dynamic fallback decisions, so Plan or binding drift removes stale
actions while peer-Team decisions remain intact.

The projection-backed preparer resolves the exact Scope's current decision
revision, builds deterministic Approve/Reject CAS commands, and refreshes the
authoritative Projection only after an explicit action commits. `Not now`
remains non-authoritative and writes nothing. Approve makes the existing
compiler source materialize Attempt 2 on the exact fallback binding; a later
Reject removes that authority on the next preflight. The production daemon now
injects this preparer into Mission execution, and the control router recognizes
newly registered fallback decisions without a startup-only controls table.
After preflight succeeds, the macOS Store refreshes the same authoritative
Snapshot so the newly prepared fallback Decision Sheet is immediately
reachable; a changed view still expires the preflight rather than reusing stale
authority. Focused and full `internal/app`, focused race, full `cmd/loomd`, all
60 `LocalProductStoreTests`, all 169 XCTest cases with one intentional skip,
and all six Swift Testing contracts pass.

The dynamic integration gate now approves the exact prepared action, recompiles
from the rebuilt decision Projection, and executes a controlled two-Agent Team.
The Main Agent's Codex/OpenAI Attempt fails with `provider_rejected`; the
authoritative RecoveryPolicy consumes the approval and schedules Attempt 2 on a
different Loom Native/DeepSeek Runtime, preserving the exact DeepSeek Provider
Account, credential reference, and credential revision 7. The independent
Reviewer remains on its MiniMax Account and credential revision 2, completes
normally, and never acquires fallback state.

This gate exposed and repaired a real cross-Harness defect: the ExecutionPlan's
primary Runtime instance was previously required for every Attempt, and the
recovery decision always copied the failed Runtime. The source contract now
freezes `FallbackRuntimeInstanceID` beside the exact fallback approval, keeps
retry and legacy same-binding fallback on the current Runtime, records the
target Runtime in the recovery fact, and requires the scheduled/dispatched
Attempt plus its frozen target binding to match during Work Authority and
Projection replay. Rules/work/projection/app full suites, focused race, Go vet,
and diff checks pass. This remains `PARTIAL` and source-only: installed Decision
Sheet actions and real Provider fallback dispatch remain W2D gates. Build 22 and
the DeepSeek r6 live blocker are unchanged.

`PARTIAL / UNINSTALLED BUILD 23 CANDIDATE (2026-08-11)`: the source bundle
version is now v0.5.2 build 23 and a fresh arm64 candidate exists at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build23-candidate-2026-08-11/Loom.app`.
It contains the earlier dynamic fallback decision and authoritative Board replay
source slice, but predates the subsequent cross-Runtime RecoveryPolicy repair
described above. Deep strict signing, owner-only bundle permissions, no-symlink checks,
the deterministic native build fixture, transactional installer fixture, and
transport ZIP extraction all pass. The Swift executable SHA-256 is
`375d76cbcbbd163ea5eb7364d25920dd95e87d1ef8af78c8211b9ab93c2a5611`,
the bundled daemon SHA-256 is
`072fb02b468adf1a6a42c98003ee38557e66002b9c420df2d6841b6488660c94`,
and the transport ZIP SHA-256 is
`a8dd5441db6b707111bd4cd3605f10972cf4e28fb0fb5a49070784804f3859d7`.

This candidate has not been installed or launched as a macOS App. The current
installed App and managed daemon remain external-provenance build 22 with
unchanged bytes and processes so the pending DeepSeek r6 event can still be
attributed to that exact bundle. Build 23 cannot satisfy installed fallback,
Vault CV6, or mixed-Team live acceptance until the build-22 conversation gate is
observed and the candidate is transactionally installed and exercised.

The build-23 bundled daemon has additionally passed an isolated-bundle Vault
contract using a fresh temporary HOME and synthetic secret: local-app bootstrap
created owner-only `vault.key` and `credential-vault.db`, configure committed an
opaque reference at revision 1, replace advanced it to revision 2, a full daemon
cold restart automatically unlocked LocalKeyFile and restored that exact
reference/revision, and revoke advanced only that credential to revision 3.
Focused race and the complete `cmd/loomd` suite pass, with no candidate daemon
process residue. This is binary-path Vault evidence without Provider network
traffic; it does not satisfy real-key verify, conversation reply, or installed
App gates.

`PARTIAL / UNINSTALLED BUILD 24 CANDIDATE (2026-08-11)`: a newer arm64
candidate now supersedes build 23 for source-to-binary verification at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build24-candidate-2026-08-11/Loom.app`.
It includes the cross-Runtime RecoveryPolicy repair and controlled
Codex/OpenAI-to-Loom-Native/DeepSeek recovery source described above. It was
built from the same dirty Phase 2D worktree and is not release provenance.

Deep strict signing, arm64 identity, owner-only bundle permissions, no-symlink
checks, deterministic native build, transactional installer fixture, and ZIP
extraction with byte-for-byte executable comparison pass. The Swift executable
SHA-256 is
`b7f64d64f1547959f6e4c22fdab1d60734d5458a66f39b62d291af6c51a9b8e0`,
the bundled daemon SHA-256 is
`4917838942bfd07712f4707921748faba1fd958bd18d69a7924b0f25ea299464`,
the bundle Info.plist SHA-256 is
`38b2e69dfb1af5168e506341e07a81501a0cb609c1d17576411a5cc55d562cdf`,
and the transport ZIP SHA-256 is
`c1eb757c933e83933618d488a011d9d6780d9a5b07cba186cd9a2a1914bcc935`.

The exact build-24 daemon passes the installed-bundle Vault
configure/replace/cold-restart/revoke contract in normal and race modes with a
synthetic secret and no Provider request. The complete repository Go suite,
affected-package race and vet checks, 169 XCTest cases with one intentional
visual skip, and six Swift Testing contracts pass for the source packaged into
this candidate. Build 24 has not been installed or launched as a macOS App;
installed build 22 and its running managed daemon remain unchanged for the
pending DeepSeek r6 attribution gate. Installed Decision Sheet actions, real
fallback dispatch, Vault CV6, and the four-Provider mixed-Team live matrix
remain open.

`CURRENT / PROVIDER ACCOUNT POLICY AUTHORITY AND PROJECTION CANDIDATE
(2026-08-11)`: W2D now has one strict revision-CAS policy stream for each exact
Provider Account. Its immutable version-1 value freezes maximum concurrent
Attempts, bounded dispatch starts/window, maximum assigned budget units,
configuration revision/time, and a canonical digest over the complete Provider
plus account identity. Command and event IDs remain replay metadata rather than
part of the reusable policy value, so Projection and future frozen Attempts use
the same constructor and digest.

Authority replay verifies stream sequence, causation, deterministic event ID,
idempotency, closed payload shape, canonical digest, and Provider/account
identity. The global Projection rebuilds exact account-local policies, rejects
cross-Provider lookup, keeps peer accounts independent, and leaves its previous
accepted read view unchanged when a malformed later policy fact fails rebuild.
Focused tests, five race repetitions, and complete `internal/work` plus
`internal/projection` suites pass.

`CURRENT / PROVIDER ACCOUNT CAPACITY AND ATOMIC DISPATCH CANDIDATE
(2026-08-11)`: W2D now also has a distinct non-secret capacity stream for each
exact Provider Account. Claim validates the frozen Agent execution binding
against the current account policy, then writes the Run claim, Runtime
reservation, and Provider Account reservation in one CAS append batch. It
fails closed before Adapter or credential lease access when account-local
concurrency, bounded dispatch starts, or assigned budget is exhausted.
Replacement and terminal transitions release only the exact prior generation;
concurrent claims across different Runtimes but the same account leave one
winner and no partial loser mutation.

The admitted Run freezes policy revision/digest and assigned budget. Strict
authority and Projection replay verify policy history, reservation sequence,
causation, binding digest, account identity, and matching release; policy
tightening affects future claims without rewriting an admitted Run. A
post-implementation review added three fail-closed guards: even an absent policy
head participates in claim CAS, policy revision timestamps must strictly
advance, and selectively replayed peer-Run capacity facts retain deterministic
event/idempotency validation. Clock rollback and forged orphan-capacity REDs now
pass.

The Team Board API and strict Swift models project only safe frozen policy
fields per Agent plus current policy/capacity totals per Provider Account.
Focused normal/race tests, complete affected Go packages, Go vet, strict Swift
model tests, the isolated Mission UI suite, and a 63-test ordered Swift subset
pass. Two earlier bounded full Swift runs stopped at XCTest's internal
`@MainActor` expectation before the first Mission UI assertion. The latest
unfiltered `swift test` now passes 174 XCTest cases with one intentional skip
plus seven Swift Testing contracts, and five consecutive `--skip-build` full
runs also pass. The earlier ordering symptom is not reproducible in the current
test product; no speculative product or test workaround was added.
The controlled four-Provider source matrix is now complete: Codex/OpenAI,
Claude Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax freeze
independent bindings, preserve account-local accounting, and isolate a rejected
or revoked account to the affected Agent. The installed App presentation,
Vault CV6, real Provider accounting, and four-Provider mixed-Team live matrix
remain open. Installed build 22, the DeepSeek r6 live gate, and build-24
installation status are unchanged.

`CURRENT / PROVIDER ACCOUNT POLICY CONFIGURATION AND GOVERNANCE UI CANDIDATE
(2026-08-11)`: the production source path now runs from `work.Authority`
through `LocalProductSetupService`, `LocalProductSetupAPI`, the private loomd
UDS method `provider_account_policy_configure`, strict `LocalIPCClient`,
`LocalProductStore`, and the Runtime & Providers account UI. The command is
scoped to one exact Provider plus Provider Account and carries only expected
policy revision, bounded concurrency/dispatch/budget limits, and an operation
ID. It has no secret, credential reference, policy digest, Prompt, or Provider
body. The daemon replaces any client correlation with the trusted UDS request
ID before authority admission.

The Swift client now uses that same Incident ID for input admission, UDS,
response validation, and its owner-only app operational record. The daemon
wrapper persists the matching governance terminal event with only safe
Provider/account identity, stage, elapsed time, result, error code, and
retryability. Operation ID, limits, credential data, and raw request/result
payloads are excluded from both diagnostic stores.

The service rebuilds Projection and verifies the exact policy revision/digest
before returning. The setup account directory publishes only safe policy
availability, revision, and limits. Swift strictly rejects unknown fields,
cross-Provider account identities, invalid ranges/digests, and malformed
RFC3339/RFC3339Nano timestamps; Store accepts success only after a fresh setup
snapshot contains the same exact account, revision, and limits. CAS conflict is
shown inside that account's Limits sheet with an Incident ID and does not take
global setup or peer Agents offline.

The Provider Account view now summarizes current concurrency, dispatch-window,
start-count, and assigned-budget ceilings, and its dedicated Limits sheet has
native numeric controls, loading/disabled state, inline recovery, keyboard
cancel/default actions, accessibility labels, and no credential preview.
Focused Swift model/client/Store/UI/diagnostic REDs pass, including fractional
Go timestamps, safe input-admission correlation, and account-local conflict
isolation. A 32-test Provider/Vault
XCTest subset plus two Swift Testing contracts pass; the 520-by-500 large-type
UI render passes. Complete affected Go package tests, focused race tests, Go
vet, daemon wire rejection of unknown secret fields, and both strict Swift/Go
contract probes pass. The latest unfiltered full Swift suite passes 174 XCTest
cases with one intentional skip plus seven Swift Testing contracts; five
consecutive no-rebuild full-suite runs pass as a stability check. This source
Candidate is packaged as the uninstalled build 25 Candidate below, but it has
not been installed or exercised with a real Provider Account policy. Installed
build 22 and its running daemon remain unchanged.

`PARTIAL / UNINSTALLED BUILD 25 CANDIDATE (2026-08-11)`: build 25 supersedes
the uninstalled build 24 Candidate for the current source-to-binary boundary at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build25-candidate-2026-08-11/Loom.app`.
It adds the production Provider Account policy path, atomic Runtime plus account
capacity reservation, frozen policy revision/digest, account-local Store/UI
governance, and privacy-safe app/daemon Incident diagnostics to the earlier
Vault, per-Agent binding, and explicit fallback slices.

The Candidate is v0.5.2 build 25, arm64, ad-hoc signed, owner-only, and contains
no bundle symlinks. Deep signature verification and ZIP extraction with
byte-for-byte App/daemon comparison pass. SHA-256 values are
`59129509b30f955df13012f577a2b361b8ea4ac5134cf8c50452548934cd0c50`
for `LoomLocalApp`,
`c7b274a480d0a3a8de161d6081f4e81020a74682217b1d15b1d2bad0201bd243`
for bundled `loomd`, and
`d9e9574827a777b3402c723ff722662559be7af27e5900b927f8c327c8138705`
for the transport ZIP. The artifact manifest records the focused Go/Swift,
race, vet, strict-wire, diagnostic, and large-type UI evidence plus the latest
174-XCTest/seven-Swift-Testing full-suite pass and five-run stability check.

Build 25 was not launched or installed. `/Users/lune/Applications/Loom.app`
and its managed daemon remain running build 22 with unchanged canonical argv,
so the pending DeepSeek r6 event remains attributable. Installed policy UI,
real Provider accounting, Vault CV6, and the four-Provider mixed-Team live
matrix remain open; the corresponding controlled source matrix is complete.

`CURRENT / EXACT CONVERSATION PROVIDER ACCOUNT ROUTING CANDIDATE
(2026-08-11)`: ordinary conversation routing now follows the same exact account
identity contract as Agent Attempts. Setup enumerates every verified brokered
Provider Account and publishes one immutable Conversation Profile for each.
Primary Profile IDs remain compatible with persisted threads, while
non-primary Profiles freeze the validated account suffix and credential
revision. Revoked, malformed, cross-Provider, or revision-zero records are not
published.

The production daemon replaces the former Provider-primary lookup with the
full Provider Account projection. A selected Profile must match exactly one
Provider Account record before the router acquires a Loom Vault lease scoped by
Provider, account, opaque credential reference, and revision. Ambiguity or any
identity/status drift fails closed before Provider dispatch. Swift rejects the
same malformed wire states and the conversation picker now displays Provider,
exact account, and Model.

Focused RED/GREEN tests, complete affected Go package suites, focused race,
Go vet, and the full Swift suite pass. The Swift result is 175 XCTest cases with
one intentional visual skip plus eight Swift Testing contracts. A full
repository Go run passed every package except one existing
`internal/localipc` one-second client boundary under parallel load; that exact
test then passed ten consecutive repetitions and the full package was rerun
separately. This source postdates build 25 and therefore requires build 26 for
binary attribution. Installed build 22, DeepSeek r6, Vault CV6, real Provider
accounting, and the mixed-Team live matrix remain open and unchanged.

`PARTIAL / UNINSTALLED BUILD 26 CANDIDATE (2026-08-11)`: build 26 now packages
the exact Conversation Provider Account routing source at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build26-candidate-2026-08-11/Loom.app`.
It supersedes build 25 for source-to-binary attribution and remains a dirty
worktree Candidate rather than release provenance.

The arm64 App is ad-hoc signed, owner-only, and contains no bundle symlinks.
Deep signature verification and ZIP extraction with byte-for-byte App, daemon,
and Info.plist comparison pass. SHA-256 values are
`ad29f15b08ed07dae08175b990abf508f640d72560112ed3b538122e516c61ea`
for `LoomLocalApp`,
`867dc4977ac8f9aa71dc3a7d423e90204983167e72c81ed267c5cdc68bcda49c`
for bundled `loomd`,
`6111c06b4ba52bdc1eea760d62cea3912dd25a89d6b20501c59235b25a119ee5`
for `Info.plist`, and
`986f12a5654ba58e2016f6cf62ab573dd1a254215a0c7273402b31e25ff32937`
for the transport ZIP.

Build 26 was not installed or launched. Installed build 22 and its managed
daemon retain the same process identities and canonical argv, preserving the
pending DeepSeek r6 attribution. The complete `internal/localipc` package also
passed independently after the parallel full-repository timeout described
above. Installed Provider policy UI, real accounting, Vault CV6, DeepSeek r6,
and the four-Provider mixed-Team live matrix remain open.

`CURRENT / ANTHROPIC MESSAGES CONVERSATION ROUTE CANDIDATE (2026-08-11)`:
ordinary conversation now publishes every exact verified Anthropic Provider
Account as an immutable account/revision-scoped Profile. Its Loom Native route
freezes `anthropic_messages`, `claude-sonnet-5`, Provider Account, and
credential revision, then obtains the secret only through the exact
Provider/Account/reference/revision Vault lease. No Provider-primary or global
credential lookup is available on this path.

The new Provider client uses the fixed official HTTPS Messages endpoint,
private proxy-free and redirect-free transport, bounded request/response
payloads, `x-api-key` only on the single request header, and a fixed
`anthropic-version`. It accepts only a matching assistant message containing
bounded text blocks. Model drift, tool use, unsafe content, redirect, oversize,
auth, rate-limit, timeout, and malformed responses fail closed through the same
safe conversation diagnostics used by the OpenAI-compatible routes.

Provider, setup, daemon, focused race, and vet checks pass. Strict Swift setup
decoding and Store tests prove DeepSeek to Anthropic keeps one visible Loom
Conversation, creates a third immutable Segment, and sends the account-scoped
Anthropic Profile. The complete Swift suite passes 175 XCTest cases with one
intentional visual skip and eight Swift Testing contracts. One full repository
Go run timed out two existing five-second Codex/Pi daemon conversation tests
under parallel load; the complete daemon package passed independently and both
targets then passed ten consecutive repetitions. This source postdates build 26
and requires build 27 for binary attribution. Installed Anthropic reply,
DeepSeek r6, Vault CV6, real accounting, full Capsule/disclosure, and mixed-Team
live acceptance remain open.

`CURRENT / EXTERNAL INSTALLED BUILD 26 CORRECTION (2026-08-11)`: the latest
read-only observation supersedes the earlier statement that installed build 22
was unchanged. `/Users/lune/Applications/Loom.app` is now a separately built
v0.5.2 build 26 with App PID `71211` and managed daemon PID `71252`. Its daemon
uses the canonical `--state`, `--isolation-root`, `--socket`, and
`--managed-parent-pid` argv. This task did not install, restart, or replace it.
The installed Swift executable is
`b30ff10bc805c4bf95dcf0570feb8254ea3488d443cf631dc80d206d2d5b0437`,
the daemon is
`9d2c52b9e3dd70addf5173c2764698b35c113255be28c285e8ab700c2778086a`,
and `Info.plist` is
`6111c06b4ba52bdc1eea760d62cea3912dd25a89d6b20501c59235b25a119ee5`.
These bytes do not match this task's build 26 Candidate, so no provenance is
inferred. The daemon happens to match build 27's daemon digest.

The installed owner-only setup projection reports an unlocked LocalKeyFile
Vault, `deepseek.primary` verified at revision 6, and immutable Profile
`conversation-deepseek-deepseek-chat-r6`. Persisted metadata shows one visible
Conversation with Codex, DeepSeek r3, and DeepSeek r6 Segments. No successful
DeepSeek Attempt is established: the latest correlated Attempt
`loom-chat-6135d434-ce27-4f69-bec5-1dabba5e0c8d` reached `provider_http` at
`2026-08-11T12:06:35Z` and failed `invalid_response`. This proves Vault lease,
route selection, and Provider dispatch, but not an accepted Provider reply.

`PARTIAL / UNINSTALLED BUILD 27 CANDIDATE`: build 27 packages the Anthropic
Messages route and exact account-scoped Vault lease. Its arm64 identity, deep
ad-hoc signature, owner-only permissions, no-symlink boundary, transport ZIP
extraction, and byte-for-byte App/daemon/plist comparison pass. It was not
installed or launched and is superseded by build 28 because it predates the
DeepSeek response-compatibility repair. Its manifest is retained at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build27-candidate-2026-08-11/BUILD-MANIFEST.md`.

`CURRENT / DEEPSEEK RESPONSE CLASSIFICATION AND BUILD 28 CANDIDATE`: the
OpenAI-compatible conversation client now treats a bounded Provider-resolved
model name as non-authoritative response metadata while retaining Loom's exact
requested model in the immutable Profile and Execution Binding. A malformed
successful response now preserves one safe reason code:
`response_json`, `response_model`, `response_choices`, `response_role`, or
`response_content`. It still records no Provider body, Prompt, conversation
text, credential, header, nonce, or ciphertext.

Complete `internal/provider`, `internal/app`, and `cmd/loomd` tests pass, as do
the complete repository Go suite, complete Provider race suite,
affected-package vet, and the full macOS suite of 175 XCTest cases with one
intentional visual skip plus eight Swift Testing contracts. The uninstalled
v0.5.2 build 28 Candidate passes arm64,
deep signature, owner-only permission, no-symlink, and extracted transport byte
equality checks. Its hashes and open gates are recorded at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build28-candidate-2026-08-11/BUILD-MANIFEST.md`.
Installed DeepSeek reply, Anthropic reply, Vault CV6, real accounting, and the
four-Provider mixed-Team live matrix remain open.

`CURRENT / EXTERNALLY INSTALLED BUILD 28 CORRECTION (2026-08-11)`: this latest
observation supersedes the build-26 installed-state paragraph above. During the
final read-only check, another process replaced and restarted
`/Users/lune/Applications/Loom.app` with v0.5.2 build 28. This task did not
perform that install. The installed App, daemon, and `Info.plist` hashes now
exactly match the retained build 28 Candidate:
`ae94cf38bb2e2c08e454f6572fa9feb7fd12f743aa04a535fbe90183254e7f78`,
`895454d1f8c41db1745414e7084448b952a0b9dd3dba35b5c98ed0a696acad28`,
and `0e857abeef75dd438f84f186c53401d32f1b1d6417d4a308dc708d557e41bbd9`.
Deep signature verification passes. App PID `83012` owns managed daemon PID
`83039`, whose argv retains canonical state, isolation, Socket, and parent
identity.

The exact installed build 28 automatically reopened the owner-only Vault after
restart. A read-only setup request reports LocalKeyFile `unlocked`, no migration
or recovery-required accounts, `deepseek.primary` verified at revision 6, and
the r6 Conversation Profile published. No new chat event exists after the
install, so the earlier `provider_http/invalid_response` Attempt cannot be
reinterpreted as build 28 evidence. Build startup/state recovery is accepted;
the next gate remains one user-triggered DeepSeek message producing either a
real reply or one exact safe `response_*` reason.

`CURRENT / PER-AGENT CATALOG FREEZE REPAIR AND BUILD 29 CANDIDATE
(2026-08-11)`: a Phase 2D audit found one P0 source defect in the Codex Agent
catalog. The Codex/OpenAI Profile selected `high` reasoning while requiring
only `workspace_edit`; Runtime validation requires the
`reasoning_effort` capability whenever an explicit reasoning value is frozen.
The catalog could therefore present a Codex role that failed closed during
Execution Binding freeze. RED reproduced the exact failure, and GREEN adds
both `reasoning_effort` and `workspace_edit` to the Profile.

The product catalog regression now freezes every published role option against
its exact Runtime instance and verifies that the selected model belongs to that
instance. Codex/OpenAI, Claude Code/Anthropic, Loom Native/DeepSeek, Loom
Native/Kimi, and Loom Native/MiniMax all pass. The existing controlled
four-Provider Team canary also confirms Agent-local failure isolation: a
Codex/OpenAI `provider_rejected` Attempt does not prevent the Anthropic, Kimi,
or MiniMax peers from succeeding, and each Attempt keeps independent Provider
Account, credential revision, binding digest, token usage, and cost. Projection
tests separately revoke one DeepSeek credential and preserve the OpenAI role as
`ready`; Provider Account capacity tests preserve account-local rate and budget
isolation.

The complete repository Go suite, focused daemon race, affected-package vet,
and full macOS suite pass. The Swift result remains 175 XCTest cases with one
intentional visual skip plus eight Swift Testing contracts. Uninstalled v0.5.2
build 29 packages this source at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build29-candidate-2026-08-11/BUILD-MANIFEST.md`.
Arm64, deep ad-hoc signature, owner-only permissions, no-symlink checks, and
ZIP extraction byte equality pass. The installed App remains exact build 28;
build 29 was not installed or launched. DeepSeek and Anthropic replies, Vault
CV6, real Provider accounting, and the installed four-Provider mixed-Team
matrix remain open under the same Phase 2D Goal.

`CURRENT / TEAM BOARD STRICT GOVERNANCE WIRE AND BUILD 30 CANDIDATE
(2026-08-11)`: a follow-on W2D audit found that the Swift Team Board accepted
contradictory governance facts even though Go projection emits a closed shape.
Examples included `execution_binding_available=true` with an empty Harness,
cross-Provider Account identity, a populated binding marked unavailable,
negative usage, a token total different from input plus output, accounting
facts while `accounting_available=false`, and an account error rate inconsistent
with its failed/total Attempt counts.

RED proved all of those malformed states were accepted. GREEN now validates the
exact per-Agent Harness/Provider/Account/Model/credential tuple, preserves the
native no-credential shape, applies the same usage and cost invariants as Work
Authority, and validates account-local attempt, rate, budget, usage, cost, and
currency aggregates. When the daemon intentionally marks an aggregate as
overflowed, the Team inspector now says `Accounting incomplete`; partial sums
are no longer visually indistinguishable from complete totals.

The full macOS suite passes 176 XCTest cases with one intentional visual skip
and eight Swift Testing contracts. Complete API and daemon suites pass; the
complete repository Go, focused race, and affected vet evidence remains valid
because this increment changes only Swift/UI and build metadata. Uninstalled
v0.5.2 build 30 packages the exact source at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build30-candidate-2026-08-11/BUILD-MANIFEST.md`.
Arm64, signature, owner-only permission, no-symlink, and ZIP extraction byte
equality checks pass. Installed build 28 remains unchanged; all live Provider,
Vault CV6, real accounting, and installed mixed-Team gates remain open.

`CURRENT / BUILD 31 FROZEN AGENT LIMITS AND CAPABILITIES (2026-08-11)`:
the Team Board previously stopped the per-Agent execution binding at Harness,
Provider Account, Model, reasoning effort, and credential revision. The exact
frozen timeout, optional binding budget, and capability set were available to
dispatch authority but were not visible to governance after preflight.

Build 31 projects those three non-secret fields from each Attempt's immutable
execution binding. Swift now fail-closes nonpositive timeout, negative budget,
or invalid, duplicate, or unsorted capabilities, and rejects all three fields
when no binding is available. The Team inspector shows `Timeout`, `Binding
budget`, and `Capabilities` on the exact Agent row; binding budget remains
separate from the Provider Account policy's assigned budget.

The complete repository Go suite, focused API race, affected vet, full
176-XCTest/eight-Swift-Testing suite, and artifact integrity checks pass. The
uninstalled Candidate is recorded at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build31-candidate-2026-08-11/BUILD-MANIFEST.md`.
Installed build 28 remains unchanged. DeepSeek and Anthropic replies, Vault CV6,
real Provider accounting, and the installed four-Provider mixed-Team matrix
remain open inside the single Phase 2D Goal.

`CURRENT / BUILD 32 AGENT VAULT AND PROVIDER FAILURE STAGE PRESERVATION
(2026-08-11)`: a source audit found that the Agent credential hot path erased
closed Vault failure stages twice. More seriously, it rewrote every Provider
callback failure, including authentication, rate limiting, rejection, and
availability, as `credential_unavailable`. This made account-local failure
isolation executable but misreported the reason and prevented one-attempt stage
attribution.

RED now covers AAD identity substitution, wrapped-DEK/ciphertext/version
failure, VaultStore propagation, lease revocation, per-Agent credential access,
and Provider callback preservation. GREEN reports `vault_aad_validation`,
`vault_decrypt`, `credential_lease_revoke`, `credential_lease_expire`, or
`credential_lease_issue` as applicable. Private lower-level error text is
replaced by public sentinels, while Provider typed failures reach the Adapter's
existing `provider_auth`, `provider_rate_limit`, `provider_rejected`, and
`provider_unavailable` classification.

Complete Go, affected-package race/vet, full 176-XCTest/eight-Swift-Testing,
and package-integrity gates pass. The uninstalled build 32 Candidate is recorded
at `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build32-candidate-2026-08-11/BUILD-MANIFEST.md`.
Installed build 28 remains unchanged. Installed Provider-stage observation,
DeepSeek/Anthropic replies, Vault CV6, real accounting, and the four-Provider
mixed-Team live matrix remain open in Phase 2D.

`CURRENT / BUILD 33 AGENT-LEVEL FAILURE DIAGNOSTIC PROJECTION (2026-08-11)`:
Build 32 preserved exact Vault, lease, Provider, and Harness failure stages in
operational diagnostics, but the Team Board still exposed only an authoritative
terminal reason and Incident ID. Users could not see the exact safe stage or
retryability without inspecting a diagnostic file.

Build 33 adds a bounded, read-only diagnostic source to the Team timeline. Each
summary must match the current Attempt's Incident ID, Provider, Provider
Account, and Model before it can enrich that Agent row. Unknown stages,
malformed codes, duplicate summaries, cross-account/model records, and stale
failures superseded by a successful record are omitted. Missing or unreadable
diagnostics leave the observational fields absent and never change the
authoritative Attempt, Team status, Journal, cursor, or projection version.

Swift now fail-closes contradictory diagnostic availability, unknown stage,
unsafe code, missing Incident identity, or absent execution binding. The Team
inspector displays the safe stage plus `Retry available` or `Manual recovery`
on only the affected Agent row, while preserving Copy Incident ID.

Complete repository Go, focused API/daemon/Vault/Adapter race, affected vet,
full 176-XCTest/eight-Swift-Testing, native reproducibility/smoke, deep
signature, arm64, owner-only permission, no-symlink, and ZIP byte-equality
gates pass. The uninstalled build 33 Candidate is recorded at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build33-candidate-2026-08-11/BUILD-MANIFEST.md`.
Installed build 28 remains unchanged. All installed Provider, Vault CV6, real
accounting, and four-Provider mixed-Team live gates remain open under the one
Phase 2D Goal.

`CURRENT / EXTERNAL BUILD 35 AND BUILD 36 FOUR-AGENT CARDINALITY REPAIR
(2026-08-11)`: while this WorkItem was running, another process successively
rebuilt, installed, and restarted v0.5.2 builds 34 and 35 from the shared Phase
2D worktree. This task did not perform either install. The current installed
build 35 passes deep signature verification. App PID `66019` owns managed
daemon PID `66082`; the daemon keeps canonical `--state`, `--isolation-root`,
`--socket`, and `--managed-parent-pid` argv. Installed App, daemon, and plist
hashes are respectively
`d91734b6c6942bfe819c28956f7e902c7269b30de7536f85833d0a2509ba4a1f`,
`56b7e8ae83523a3650f18d0fe52f0fe596d27ba86e41a3c778cf45f5abd1b076`,
and `4696d65db43ca97cce0cd893e44a5d4aac17edc911a3f8d8f3c7985fb6811e2a`.
The installed daemon is byte-equal to the build 36 Candidate daemon, but the
App executable and plist are not the build 36 artifacts; installed build 35
must not be relabeled as exact build 36 provenance.

A W2B/W2C source audit found that Team definitions and execution plans already
accepted `MaxTeamAgentCount = 9`, while four older boundaries still rejected a
third sub-Agent: Saved Team runtime-binding normalization, structured draft
content, accepted-draft role seeding, and accepted-plan shape validation. This
made the required Codex/OpenAI + Claude/Anthropic + Loom/Kimi + Loom/MiniMax
Team impossible to carry through product preflight even though each role could
freeze an independent Provider Account and credential revision.

RED reproduced all four failures. GREEN replaces only those historical `2`
limits with `MaxTeamAgentCount - 1`. Four-role regressions now pass through
independent frozen binding, dormant Saved Team plan, saved instance projection,
structured draft acceptance, and accepted-draft instantiation. A direct
max-plus-one case retains the nine-Agent ceiling. The change does not introduce
a Team-level Provider client, shared credential lookup, silent fallback, or
cross-Agent failure state.

The final clean `go test ./... -count=1`, Teams/Provider race, affected vet,
full 176-XCTest/eight-Swift-Testing suite, native reproducibility/smoke, arm64,
deep signature, owner-only permission, no-symlink, and ZIP byte-equality gates
pass. An earlier concurrent full Go run saw one transient Pi metadata `version`
probe failure; the exact test passed in isolation and the clean full rerun
passed. The uninstalled build 36 Candidate is recorded at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build36-candidate-2026-08-11/BUILD-MANIFEST.md`.

This closes the source cardinality blocker, not the Phase 2D Goal. Installed
post-restart DeepSeek multi-turn, Anthropic reply, real four-Provider Team
execution, account-local failure isolation, real accounting, and explicit
fallback observation remain open.

`CURRENT / CREDENTIAL VAULT PRODUCTION-DEFAULT REVALIDATION (2026-08-11)`:
ADR-0020, the P2D-W2D Credential Vault contract, and the Phase amendment remain
aligned under the single Phase 2D Goal. The normal product builder sets
`UseCredentialVault` internally and injects the resulting Vault runtime into
credential configure/verify, Conversation routing, and Agent dispatch. The
process-attested Keychain store remains only in an explicit legacy/test or
future one-time migration boundary; it is not the production Conversation or
Agent hot path.

A new product-boundary regression invokes `productionDaemonBuilder` without a
test-only Vault flag, proves that it owns `productCredentialVaultRuntime`, and
observes an `unlocked` LocalKeyFile Vault in Setup Snapshot. Focused Vault and
complete daemon tests pass with
`go test ./internal/credentials/vault ./cmd/loomd -count=1`, followed by the
exact production-default regression. This is source revalidation, not CV6
completion. Installed post-restart multi-turn Provider
replies, absence of helper processes during those live calls, encrypted Vault
restart continuity, and the real mixed-Team isolation/accounting matrix remain
open.

`CURRENT / P2D-W2B/W2C FOUR-ROLE PRODUCT-PATH REVALIDATION (2026-08-11)`:
the source acceptance chain now carries the required Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax roles from persisted
Saved Team configuration through daemon materialization, Projection Mission
binding, preflight, and compilation. Daemon materialization rebuilds all four
custom brokered Execution Profiles even when the current catalog does not
publish those Profile IDs; each reconstructed selection retains its exact
Runtime instance, and same-ID credential-revision drift fails closed.

Projection resolves four role-local Harness, Provider Account, credential
reference/revision, Model, timeout, budget, and capability tuples. Compilation
produces four plan nodes, four semantics, and eight Attempt candidates without
reducing the Team to its main Agent. Revoking `kimi.primary` marks only the
Kimi role and preflight row `blocked`; OpenAI, Anthropic, and MiniMax remain
`ready`, while start fails closed until the required role is repaired. The
separate controlled four-Provider dispatch canary continues to prove four
distinct FrozenExecutionBinding digests and peer success when one Provider
Attempt fails.

The focused tests, related race runs, affected vet, complete `internal/app`
and `cmd/loomd` packages, and clean `go test ./... -count=1` pass. This is a
source gate, not installed live acceptance. No App was built or installed and
no real Provider credential or request was used. The installed four-Provider
Team, real account-local failure, accounting, fallback, and Provider reply
matrix remain open under Phase 2D.

`CURRENT / P2D-W2C ROLE CAPSULE STATUS CORRECTION (2026-08-11)`: source audit
confirmed that Agent Attempt Execution Binding is frozen end to end, but Team
Attempts do not yet contain a real Role Context Capsule digest. The existing
`ContextCapsuleDigest` implementation belongs to Conversation Route Segments;
there is no Team Role Capsule authority record, scope/ACL admission,
deterministic packing, omission manifest, ContextAdapter output, or disclosure
receipt. Earlier amendment wording that described this as already frozen is
superseded.

The new contract
`.loom-evidence/phase2d/contracts/P2D-W2C-agent-attempt-binding-dispatch.md`
separates the `CURRENT` immutable Execution Binding chain from the `TARGET /
MISSING` Role Capsule chain. Loom must not manufacture a digest without the
corresponding admitted content. P2D-W2C remains `ACTIVE / PARTIAL`; four-role
installed dispatch and Role Capsule isolation/restart/disclosure acceptance
remain open without weakening the completed credential-reference or failure
isolation work.

`CURRENT / P2D-W2C ROLE CAPSULE MINIMUM VERTICAL SLICE (2026-08-12)`: the
2026-08-11 status correction successfully prevented a synthetic digest from
entering Team authority. Loom now has a concrete `internal/contextcapsule`
domain object generated from admitted content. It preserves authoritative,
observed, and untrusted trust classes; enforces conversation, Team, Agent,
role, Artifact, and secret-reference scopes; sorts deterministically; packs a
declared token budget; records access and budget omissions without content; and
fails closed when required context is omitted. Model-output provenance cannot
be promoted to authority, and secret-reference scope rejects any body bytes.

Each Capsule digest and disclosure receipt bind the target Agent/role,
Provider, Provider Account, Model, auth mode, Context Adapter, Disclosure
Policy, token budget, and disclosure/omission counts. Team coordinator and Work
Authority require that target to match the exact FrozenExecutionBinding. The
safe Authority record is frozen on `TeamReadySetDispatched`, survives replay
and Projection, and reaches API/Swift Team Board rows without Capsule content.
Missing or route-mismatched Capsules fail before Journal mutation. Ordinary
retry must retain the admitted digest; an approved Provider fallback receives a
new route-bound Capsule and receipt. Journal and Board non-disclosure tests
reject Capsule body leakage.

This advances P2D-W2C but does not close it. The current product source adapter
admits the confirmed Mission objective as the minimum authoritative item; full
Goal/constraint/decision/workspace/Artifact assembly, model-specific
tokenization and ContextAdapter output, encrypted Capsule body storage and
restart lookup, disclosure receipt inspection, parallel aggregation, installed
four-Provider Team execution, and live Provider acceptance remain open. No App
was built or installed and no real Provider credential or request was used in
this increment.

Verification for this minimum slice passes `go test ./... -timeout=15m`, the
affected `go test -race` set, affected `go vet`, and the complete macOS Swift
suite: 176 XCTest cases passed with one intentional visual-export skip, plus
eight Swift Testing contracts. These results prove the source authority,
replay, Projection, API, and Swift model boundary only; they do not satisfy the
installed mixed-Team or real Provider exit gates above.

`CURRENT / P2D-W2C CANONICAL CONTEXT DISPATCH VERTICAL SLICE (2026-08-12)`:
the built-in Mission compiler no longer places `command.Objective` directly in
an independent v1 prompt payload after separately freezing a Role Context
Capsule. `internal/contextcapsule` now renders one canonical v2 dispatch from
the admitted Capsule body. The rendered prompt preserves authoritative,
observed, and untrusted labels plus provenance and content-free omissions; it
does not include Provider Account identity or credential references. Opaque
secret-reference items, non-text content, bounded secret markers, unsupported
Context Adapter identities, oversized target prompts, and noncanonical payloads
fail closed.

The v2 envelope binds the Capsule digest and disclosure-receipt digest.
`TeamCoordinator` recomputes and byte-compares the payload before Work Authority
or Journal mutation, so a raw objective, changed digest, changed receipt, or
changed prompt cannot execute under the frozen Capsule record. Pi RPC, the
product Pi wrapper, Loom Native OpenAI-compatible Agents, Codex, and Claude Code
share the closed v2 decoder and retain v1 only for explicit legacy/non-Team
compatibility. Retry fixtures retain the exact rendered payload; an approved
fallback rebuilds it from the new route-bound Capsule.

Focused Context, compiler, coordinator, adapter, product-loopback, complete
affected-package, affected race, daemon race, and vet gates pass. This remains
source-only. Full Goal/decision/workspace/Artifact source assembly,
model-specific tokenization and multi-message/tool-result adaptation, encrypted
Capsule body restart, installed four-Provider Team execution, and real Provider
acceptance remain open. No App was built or installed and no credential or real
Provider request was used in this increment.

The first parallel `go test ./... -timeout=15m` run passed every package except
one existing Pi local-model `health_then_early_exit` timing case under the
concurrent daemon/Swift-contract load. The exact case then passed five
consecutive runs, and the clean serialized `go test -p=1 ./... -timeout=15m`
rerun passed. This is recorded as a load-sensitive test observation, not a
Context dispatch failure and not installed-live evidence.

`CURRENT / P2D-W2C AUTHORITATIVE ROLE CONTEXT SOURCE ASSEMBLY (2026-08-12)`:
the built-in Mission source now advances beyond the minimum objective-only
Capsule. Every role receives the confirmed Mission objective, exact WorkPackage
policy, current task and plan identity, role-local governance, and its exact
Artifact revision bindings as typed, scoped, prioritized, content-addressed
authority items. Required policy, task, and governance content fails closed;
Artifact content may be deterministically budget-omitted with a content-free
omission record. Peer role titles, Provider Account identity, credential
reference, secret, and approval actor do not enter the model prompt.

The retry boundary remains strict. Attempt number is an authoritative Attempt
fact and is not allowed to mutate Capsule content. An ordinary retry therefore
reuses the exact Capsule digest and disclosure receipt. A changed Provider,
Account, Model, or Harness remains a fallback transition and receives a new
target-bound Capsule only when the versioned approval matches both source and
target binding digests; the Capsule carries only safe approval metadata.

RED first reproduced the ordinary-retry digest drift and the resulting
`cmd/loomd` stop after `TeamNodeAttemptScheduled` but before the second
`TeamReadySetDispatched`. GREEN restores the complete two-Attempt cold-runtime
terminal path and preserves the fallback authority distinction. The Role
Capsule source tests, complete `internal/app`, three consecutive cold-runtime
regressions, three Harness process-contract runs, affected race,
`go vet ./...`, and clean serialized `go test -p 1 ./... -count=1` all pass.

P2D-W2C remains `ACTIVE / PARTIAL`. Complete Goal, confirmed-constraint,
accepted-decision, workspace/test-state, and prior-output assembly;
model-specific tokenization and richer ContextAdapter mapping; encrypted
Capsule restart lookup; installed four-Provider Team dispatch; and real
Provider acceptance remain open. No App was built or installed, and no real
credential or Provider request was used in this increment.

`CURRENT / P2D-W2B/W2C EXECUTION PROFILE GOVERNANCE PROJECTION (2026-08-12)`:
the per-Agent source audit found that the immutable Runtime Profile and Attempt
binding already froze Provider Account, credential reference/revision, Model,
auth mode, reasoning effort, timeout, budget, and capabilities, but Builder and
Mission preflight projections omitted part of that contract. In particular,
Swift could not distinguish a valid native route from a malformed brokered
route, and configured fallback rows lost auth mode, reasoning effort, timeout,
budget, and capabilities.

Builder and Mission preflight now project the complete non-secret primary and
fallback Execution Profile. Go and Swift independently reject account-bound
auth without a provider-scoped account and positive credential revision,
native auth carrying an account or revision, nonpositive timeout, negative
budget, noncanonical capabilities, contradictory ready/blocked state, and
hidden fields on an unconfigured fallback. A valid native fallback remains
representable. The Mission UI shows Harness, Provider or Provider Account,
Model, auth posture, credential revision, reasoning, timeout, budget, and
capabilities for both routes; fallback still requires explicit approval.

This is a source-only P2D-W2B/W2C repair. It does not expose credential
references, endpoint fingerprints, secrets, Prompts, Provider responses, or
hidden reasoning. No App was built or installed and no Provider credential or
network request was used. Installed mixed-Team execution, account-local live
failure isolation, accounting, fallback observation, Vault CV6, and real
Provider replies remain open under the single Phase 2D Goal.

Verification passes the complete 176-XCTest/eight-Swift-Testing suite with one
intentional visual-export skip, complete `internal/app`, focused cross-language
Swift contract probes, `go test -race ./internal/app -count=1`, `go vet ./...`,
and the clean serialized `go test -p 1 ./... -count=1 -timeout=15m` gate.

`CURRENT / P2D-W2B/W2D INDEPENDENT AGENT ROUTE EDITOR (2026-08-12)`: the
Builder previously exposed each Agent's Harness, Provider Account, and Model as
one combined Role option. The macOS editor now distinguishes stable Agent role
identity from independent Harness, Provider Account, Model, reasoning, timeout,
budget, and explicit fallback controls. Provider Account choices name the exact
non-secret account and credential revision; custom immutable Profiles retain the
correct current-route checkmark without exposing the credential reference.

The Go Builder now treats Harness and Provider Account edits as
compatibility-aware Profile composition. A Provider Account choice must come
from the same Harness/runtime and copies its exact Provider, Account, auth mode,
endpoint fingerprint, credential reference/revision, and compatible Model; the
menu previews that Model. Reasoning, timeout, budget, and capabilities remain
frozen from the current Agent. A Harness choice must already publish the current
Provider Account, credential revision, and Model, and changes only the
Adapter/runtime selection. Each accepted combination receives a new immutable
Runtime Profile and Role option, preserves AgentDefinition identity and peer
Agents, validates against the exact Runtime observation, and clears an
incompatible fallback. Cross-Agent source routes and incompatible combinations
fail before Draft revision or binding-digest mutation.

Focused account/Harness RED-GREEN tests, API and Swift Store forwarding,
complete `internal/app` and `internal/api`, affected race, `go vet ./...`, the
clean serialized `go test -p 1 ./... -count=1 -timeout=15m`, and the complete
177-XCTest/eight-Swift-Testing suite pass with one intentional visual-export
skip. This is source-only W2B/W2D progress. No App was built or installed and
no credential or Provider request was used. Installed mixed-Team execution,
real account-local failure isolation/accounting/fallback, Vault CV6, and real
Provider replies remain open under the single Phase 2D Goal.

`CURRENT / INSTALLED BUILD 39 DEEPSEEK LIVE REPLY ACCEPTANCE (2026-08-12)`:
the earlier statement that every real Provider reply remained open is now
superseded for the installed DeepSeek route only. Read-only inspection of
`/Users/lune/Applications/Loom.app` proves v0.5.2 build 39 is currently running
with its bundled child daemon. The App and daemon started at 00:34 local time,
the bundle passes strict deep signature verification, and exact SHA-256
identities were captured for the App executable, daemon, and Info.plist. No
retained build-39 delivery manifest was found, so this is exact installed-bundle
evidence and is not a claim that the bytes match a retained Candidate.

The installed projection reports `deepseek.primary` verified at credential
revision 6. The LocalKeyFile Vault key, encrypted database, and live UDS remain
owner-only `0600`; safe database metadata shows one active encrypted DeepSeek
row at the same account and revision. No credential-helper or Keychain command
was active when observed. That observation does not prove that no helper ran
during each historical Provider call, so instrumented no-helper hot-path
acceptance remains open.

Persisted schema-v2 conversation metadata proves one visible Conversation has
five immutable Segments in the order Codex, DeepSeek r3, DeepSeek r6, Codex,
DeepSeek r6. After the current build-39 App/daemon startup, final Segment
Attempts 21 through 25 all completed successfully against DeepSeek r6. Each has
a distinct Attempt ID, frozen binding digest, Context Capsule digest, and a
matching successful `conversation_dispatch` Incident in both App and daemon
operational diagnostics. Earlier account-local Provider timeouts are followed
by these successful Attempts, providing real recovery evidence without reading
conversation content or Provider bodies.

Phase 2D therefore accepts the installed DeepSeek credential import, verify,
Profile publication, real reply, repeated multi-turn dispatch, Codex-to-
DeepSeek Segment continuity, and timeout recovery portions of P2D-BLOCKER-1.
The unified Goal remains `ACTIVE / PARTIAL`: installed Anthropic reply, the
four-Provider mixed-Team matrix, account-local revocation/rate-limit/timeout
isolation, real accounting, explicit fallback, encrypted transcript/Capsule/
native-handle storage, rotation continuity, and instrumented proof that normal
Provider calls never invoke the migration helper remain open.

`CURRENT / P2D-W2C ENCRYPTED TEAM CONTEXT CAPSULE STORE (2026-08-12)`:
the Loom Vault now has a source-complete encrypted storage boundary for Team
Mission Context Capsule dispatch payloads. Each Conversation receives a random
256-bit DEK. The DEK is wrapped under the VMK-derived
`loom/conversation-wrap/v1` key, while each Capsule payload is AES-256-GCM
encrypted with its own nonce and content-addressed by the validated Capsule
digest. Canonical AAD binds the immutable Authority record, Conversation,
Capsule digest, disclosure receipt, schema, and cipher version. The database
enforces unique wrap nonces and unique per-Conversation data nonces.

The Store validates the Capsule Authority record and canonical dispatch payload
before encryption and after decryption. It supports idempotent exact writes,
restart lookup by exact Conversation and digest, tamper/substitution rejection,
and Conversation-local crypto-erasure. Vault VMK rotation rewraps both
Credential and Conversation DEKs in the same SQLite transaction. Context
payload, Prompt, credential, and Provider body never enter the plaintext
columns; only non-secret Authority metadata, wrapped keys, nonces, digests, and
ciphertext are persisted.

The built-in Team Mission compiler now writes every primary/fallback Attempt
Capsule through this Store before assembling its dispatch frame. A locked,
recovering, unavailable, or failed Store aborts compilation; there is no
plaintext fallback and no Event Journal authority is created by the Store.
Focused RED/GREEN, complete affected packages, targeted race, `go vet ./...`,
and the serialized repository Go suite pass. The wrapper used to capture the
full-suite exit marker later failed because zsh reserves `status`, after all Go
packages had already reported `ok`; this shell-marker error is not a test
failure.

P2D-W2C remains `ACTIVE / PARTIAL`. Ordinary-conversation transcript and route
Capsule encryption, Provider-native `ExternalSessionHandle`, encrypted export/
restore coverage for context data, installed migration and restart lookup, and
the four-Provider mixed-Team live matrix remain open. No App was built or
installed and no credential or Provider request was used in this source slice.

`CURRENT / P2D-W2A ENCRYPTED ORDINARY CONVERSATION STORE (2026-08-12)`:
the production-default source path no longer persists ordinary Conversation
threads as plaintext JSON. The Vault now stores one canonical encrypted
document per Loom thread under that thread's Conversation DEK. Document AAD
binds Conversation ID, document kind, monotonic revision, schema, and cipher;
same-revision exact retries are idempotent, while stale revisions, content
drift, AAD substitution, tag failure, or ciphertext corruption fail closed.

Capsule payloads and transcript documents share the Conversation DEK but reserve
data nonces through one Conversation-wide database registry. This closes the
cross-table nonce-reuse gap. VMK rotation already rewraps the Conversation DEK,
so both content classes survive the same atomic rotation without re-encrypting
their bodies. Deleting a Conversation removes its wrapped DEK, Capsule rows,
thread document, and nonce history without affecting peer Conversations.

The Chat API now loads and saves per-thread encrypted documents. Existing
`chat-threads.json` is a one-time migration source only: encrypted writes finish
first; the source is then atomically renamed to a pending identity, identity-
checked, overwritten, fsynced, removed, and parent-fsynced. Partial encrypted
writes and pending cleanup are restartable. A legacy/Vault mismatch preserves
the legacy bytes and fails closed. Unsafe owner, mode, symlink, hard-link, or
directory boundaries are rejected. The overwrite/unlink is best-effort local
cleanup and does not claim erasure of filesystem snapshots or external backups.

The default product daemon injects the Vault document store. Vault recovery
keeps the App governance surface available but makes chat non-replyable; it does
not reopen the plaintext constructor. Focused migration and daemon assembly,
complete affected packages, targeted race, `go vet ./...`, and the serialized
repository Go suite all pass with direct `exit=0` evidence.

This is a source Candidate only. The currently installed v0.5.2 build 39 still
uses its existing schema-v2 `chat-threads.json`; no App was built, installed, or
migrated in this increment. Installed migration, encrypted restart and real
reply, migration diagnostics/UI, encrypted Provider-native handles, encrypted
context backup/restore, and the mixed-Team live matrix remain open. Phase 2D
therefore remains `ACTIVE / PARTIAL`.

`CURRENT / UNINSTALLED BUILD 40 ENCRYPTED CONVERSATION CANDIDATE (2026-08-12)`:
the encrypted ordinary Conversation source increment is packaged as v0.5.2
build 40 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build40-candidate-2026-08-12/BUILD-MANIFEST.md`.
The App and bundled daemon are arm64, the owner-only bundle contains no symbolic
links, deep strict ad-hoc signature verification passes, and the ZIP round trip
preserves the App executable, daemon, and Info.plist byte-for-byte. Exact hashes
are frozen in the manifest.

This is deliberately an uninstalled and unlaunched Candidate. The build-only
script passed, but the launch-smoke script was not run because starting this
source against the current user's real Application Support directory could
migrate the installed build-39 plaintext transcript before explicit approval.
The installed App remains build 39 and its data was not modified. Installed
migration, encrypted restart lookup, a real post-migration reply, migration
diagnostics/UI, encrypted Provider-native handles, no-helper instrumentation,
Anthropic, and the four-Provider mixed-Team matrix remain open. Phase 2D stays
`ACTIVE / PARTIAL`.


`CURRENT / P2D-W2A/W2D ENCRYPTED EXTERNAL SESSION HANDLE STORE (2026-08-12)`:
the Vault now has a source-complete encrypted boundary for Provider-native
`response_id`, `conversation_id`, and `prompt_cache_id` values. Every record is
encrypted under its Conversation DEK. Canonical AAD freezes Conversation,
Segment, Provider, Provider Account, Model, auth mode, credential reference and
revision, handle kind, and monotonic handle revision. Transcript, Context
Capsule, and handle ciphertext reserve data nonces through the same
Conversation-wide registry.

Exact retries are idempotent. Stale revision, same-revision content drift,
cross-route update, account/Model/Segment/auth/credential substitution,
ciphertext or metadata tamper, tag failure, and cross-channel nonce reuse fail
closed. Brokered and provider-ephemeral records require the exact account and
positive credential revision. Native-auth records explicitly require empty
account and credential metadata, preserving the existing Codex native binding
without inventing a Provider Account. Restart, VMK rotation rewrap, and
Conversation-local crypto-erasure preserve peer isolation.

The production Vault runtime now owns a bounded callback access contract. It
clears caller-owned handle bytes after encrypted write and zeroizes decrypted
lease bytes immediately after the callback. No current DeepSeek, Anthropic,
Kimi, or MiniMax client advertises reusable Provider-native state, so those
routes remain stateless and Loom does not treat ordinary response IDs as
reusable handles.

Focused RED/GREEN, complete Vault and daemon packages, targeted race,
`go vet ./...`, and the serialized `go test -p 1 ./... -count=1 -timeout=15m`
gate all pass. This source was written after the uninstalled build 40 Candidate
and has not been packaged, launched, or installed. Installed encrypted handle
restart/isolation, a real explicitly stateful Provider adapter, transcript
migration, Anthropic reply, no-helper instrumentation, and the four-Provider
mixed-Team matrix remain open. Phase 2D remains `ACTIVE / PARTIAL`.

`CURRENT / UNINSTALLED BUILD 41 ENCRYPTED HANDLE CANDIDATE (2026-08-12)`:
the encrypted ordinary Conversation, Context Capsule, and Provider-native
ExternalSessionHandle source is packaged as v0.5.2 build 41 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build41-candidate-2026-08-12/BUILD-MANIFEST.md`.
The arm64 App and daemon, owner-only/no-symlink bundle, deep strict ad-hoc
signature, and ZIP byte-equivalence checks pass; exact hashes are frozen in
that manifest. The Candidate was neither launched nor installed because doing
so could migrate the currently installed build-39 transcript without explicit
approval. Installed Loom therefore remains build 39.

Build 41 predates the migration-availability diagnostics below. It must not be
used as evidence that a failed encrypted migration leaves governance online or
shows an Incident ID in the App. The next package containing that source must
use build 42 or later.

`CURRENT / P2D-W2A/W2D CONVERSATION MIGRATION FAILURE ISOLATION (2026-08-12)`:
the encrypted Conversation constructor now records privacy-safe
`migration_read`, `migration_commit`, and `migration_cleanup` terminal events
under one startup Incident ID. A read/validation or encrypted-commit failure
preserves the legacy source and no longer aborts the complete product daemon.
The governance and Provider setup surfaces remain available, while chat alone
is fail-closed through a non-authoritative `state_unavailable` projection that
contains only stage, retryability, and Incident ID.

Swift strictly decodes that bounded availability failure, rejects non-migration
stages, unsafe Incident IDs, unknown fields, or a contradictory replyable
thread, and projects it into the existing Conversation diagnostic banner. The
banner gives a stage-specific recovery action, View diagnostics, and Copy
incident ID. It deliberately does not offer message Retry because restarting
the unavailable in-memory chat service is required after correcting the local
storage condition. Operational diagnostics preserve the underlying event's
retryability without logging thread identity, transcript content, Prompt,
Provider body, credential, ciphertext, nonce, or wrapped key.

Focused Go migration/daemon tests and the complete 75-test Swift model/store
selection pass. This is post-build-41 source only: no App was launched,
installed, or allowed to touch the current user's transcript. Installed
migration/restart/reply, migration recovery observation, no-helper hot-path
instrumentation, Anthropic, and the four-Provider mixed-Team matrix remain open
under the single Phase 2D Goal.

`CURRENT / UNINSTALLED BUILD 42 MIGRATION GOVERNANCE CANDIDATE (2026-08-12)`:
the post-build-41 migration failure-isolation source is packaged as v0.5.2
build 42 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build42-candidate-2026-08-12/BUILD-MANIFEST.md`.
Complete serial Go, targeted race, vet, complete Swift, release build, arm64,
deep strict signature, owner-only/no-symlink, and ZIP byte-equivalence gates
pass. Exact executable, daemon, plist, and ZIP hashes are frozen in the
manifest.

Build 42 was not launched or installed. The launch-smoke test remains
deliberately unrun because it could migrate build 39's real transcript without
approval. Installed Loom remains v0.5.2 build 39. Build 42 is therefore package
evidence for the source and transport boundary only, not installed migration,
recovery, restart, reply, or Phase 2D completion.

`CURRENT / P2D-W2D VAULT-ONLY ORDINARY RUNTIME SOURCE GATE (2026-08-12)`:
the generic product daemon now selects the Loom Credential Vault when callers
do not inject an explicit credential boundary. Production already selected the
Vault explicitly; this increment removes the remaining ordinary daemon/setup
fallback that could construct `ProductKeychainStore`. A direct setup service
without a Vault, mutator, or explicitly injected legacy test store fails closed
as `explicit credential boundary unavailable`.

Non-test `cmd/loomd` source may no longer call `NewProductKeychainStore` outside
an explicitly named `credential_migration` component. An AST regression test
and the release build script enforce that rule. This preserves the accepted
one-time Keychain migration design without allowing the helper to re-enter
configure, verify, Conversation, or Agent Attempt hot paths.

Operational diagnostics bind one closed, non-secret runtime marker for the
daemon session: `credential_runtime=vault` or, for explicitly injected legacy
tests only, `explicit_legacy`. IPC, Conversation migration, and Agent Attempt
records carry that marker; Go and Swift reject unknown values such as
`keychain`. The field does not contain a credential reference, key material,
Prompt, Provider body, ciphertext, nonce, or filesystem path.

Focused RED/GREEN, complete `cmd/loomd`, serialized `go test -p 1 ./...`,
focused race, `go vet ./...`, shell syntax/source scans, diff checks, and the
complete Swift suite pass. The Swift result is 180 XCTest cases with one
intentional skip plus eight Swift Testing contracts, all with zero failures.
This is source evidence only. A diagnostic marker proves the selected backend,
not historical process non-creation; approved installed observation must still
show no credential helper during import, repeated Provider calls, Agent
Attempts, and restart. Installed Loom remains build 39 and Phase 2D remains
`ACTIVE / PARTIAL`.

`CURRENT / UNINSTALLED BUILD 43 VAULT-ONLY RUNTIME CANDIDATE (2026-08-12)`:
the no-implicit-Keychain source gate and closed credential-runtime diagnostics
are packaged as v0.5.2 build 43 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build43-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release construction, arm64 App/daemon, deep strict ad-hoc signature,
owner-only/no-symlink bundle, and ZIP byte-equivalence gates pass. Exact App,
daemon, plist, and ZIP hashes are frozen in the manifest.

Build 43 was not installed and `Loom.app` was not launched. App launch smoke
remains deliberately unrun because this source can perform the accepted
one-time encrypted Conversation migration against the current user's build-39
data. Installed Loom remains v0.5.2 build 39.

The exact bundled daemon was subsequently launched directly against an
owner-only canonical `/private/tmp` state, isolation root, and UDS. Its first
session completed ping, Vault lock/unlock, and one content-free invalid chat
admission. The second session reopened the same Vault as
`unlocked / local_key_file`, preserved the Vault key and database identities,
and completed another lock/unlock. Every recorded credential/chat operation
carried `credential_runtime=vault`; lock used `credential_lease_revoke`, unlock
used `vault_open`, and invalid chat failed at `conversation_dispatch` before a
Provider call. Both 2 ms helper-process sampling windows recorded zero
`--credential-helper` observations.

The exact bounded evidence and its limitation are recorded in
`.loom-evidence/phase2d/P2D-W2D-build43-packaged-runtime-no-helper.md`. Process
polling cannot exclude a process shorter than the sampling interval, so this
advances packaged-runtime evidence without closing installed CV6. Installed
import, real multi-turn Provider and Agent Attempt no-helper observation,
migration/restart/reply, Anthropic, mixed-Team, account-local isolation/
accounting, explicit fallback, rotation continuity, and Phase 2D completion
remain open.

`CURRENT / UNINSTALLED BUILD 44 DIRECT HELPER-ATTEMPT GATE (2026-08-12)`:
the ordinary Vault runtime now exposes a process-local monotonic count of every
Darwin `--credential-helper` process start attempt. The count increments
immediately before `command.Start()` and is snapshotted into every new daemon
operational record as the explicit, non-secret
`credential_helper_spawn_attempts` field. Go tests bind monotonicity and source
ordering; Go and Swift diagnostic contracts preserve/export the field while
remaining compatible with older records that omit it.

Focused RED/GREEN, complete serialized Go, focused race, `go vet`, and complete
Swift verification pass. The Swift result remains 180 XCTest cases with one
intentional skip plus eight Swift Testing contracts, all with zero failures.
The source is packaged as uninstalled v0.5.2 build 44 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build44-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep strict signature, owner-only/no-symlink, source gates, and
ZIP byte-equivalence checks pass. `Loom.app` was not launched and the Candidate
was not installed.

The exact bundled daemon ran twice against fresh canonical owner-only
`/private/tmp` state. The first process completed Vault lock/unlock and a
content-free invalid chat admission; the second reopened the same
`unlocked / local_key_file` Vault and repeated lock/unlock. All five operational
records carried `credential_runtime=vault` and
`credential_helper_spawn_attempts=0`. Vault key/database identity persisted
across restart, while the owner-only UDS received the expected new inode. This
directly closes the build-43 polling ambiguity for the isolated ordinary Vault
path.

The exact evidence is
`.loom-evidence/phase2d/P2D-W2D-build44-helper-spawn-counter.md`. Installed Loom
remains v0.5.2 build 39. Real installed credential import, Provider multi-turn,
Agent Attempt, restart, optional migration, and mixed-Team no-helper acceptance
remain open, together with Anthropic, account-local isolation/accounting,
explicit fallback, rotation continuity, and the rest of Phase 2D. The unified
Goal remains `ACTIVE / PARTIAL`.

`CURRENT / P2D-W2A ORDINARY CONVERSATION DISCLOSURE RECEIPT (2026-08-12)`:
ordinary Conversation Route Segments now freeze a non-secret disclosure
receipt digest and the exact numbers of Context items shared and omitted. The
same tuple is frozen on each Conversation Attempt and passed to the Provider
responder request. Execution binding schema version 2 binds the tuple beside
Segment, Profile, context mode, and Context Capsule digest.

Current persisted records recompute that v2 binding and fail closed when a
receipt or count drifts without the binding changing. Older records remain
readable only with no receipt and zero counts, preserving their existing
pre-v2 binding. Schema-1 migration generates a current receipt and v2 binding
when it creates the immutable Segment.

The macOS Conversation timeline now shows a native expandable Context summary
on the first visible turn of each Segment: shared item count, omitted item
count, and the selectable disclosure receipt. It never displays Prompt text,
transcript content, Provider response, credential, ciphertext, nonce, or
hidden reasoning.

Focused persistence and drift rejection, 20-run stability, serialized full Go,
API/daemon race, `go vet ./...`, `git diff --check`, and complete Swift gates
pass. Swift now reports 181 XCTest cases with one intentional visual-preview
skip plus eight Swift Testing contracts, all with zero failures. Exact evidence
is `.loom-evidence/phase2d/P2D-W2A-conversation-disclosure-receipt-projection.md`.

Frozen build 44 does not contain this source. It is packaged as the unlaunched,
uninstalled v0.5.2 build 45 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build45-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, wire-contract, and ZIP
byte-equivalence checks pass. Installed Loom remains v0.5.2 build 39. Full
token packing, scoped retrieval,
trust-domain disclosure approval, installed encrypted migration/restart,
DeepSeek-to-Anthropic reply, sibling Attempts/aggregation, and mixed-Team live
acceptance remain open. Phase 2D stays `ACTIVE / PARTIAL`.

`CURRENT / P2D-W2D AGENT FAILURE GOVERNANCE ACTIONS (2026-08-12)`:
the Team Pulse inspector now projects recovery controls only on the exact Agent
row with a current privacy-safe failure diagnostic. A retryable failure offers
Review retry, which navigates to the existing Attention governance surface
without dispatching. A diagnostic offers the allowlisted diagnostic preview,
and the Incident ID remains copyable.

Healthy peer Agents and Provider Account accounting rows do not inherit these
actions. A focused fixture combines one retryable Anthropic failure, one
successful OpenAI/Codex peer, and one account summary and proves this row-level
isolation. Missing, old, or out-of-range action metadata fails closed.

Complete Swift verification passes with 181 XCTest cases, one intentional
visual-preview skip, and eight Swift Testing contracts. The native rendering
matrix continues to cover wide and compact windows, light and dark appearance,
and large Dynamic Type. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-agent-failure-governance-actions.md`.

Frozen build 45 does not contain this source. It is packaged as the unlaunched,
uninstalled v0.5.2 build 46 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build46-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, action-string,
wire-contract, and ZIP byte-equivalence gates pass. The Candidate App and its
bundled daemon were not launched.

Installed Loom remains v0.5.2 build 39; real mixed-Team isolation, Provider
Account accounting, approved fallback, and the counter-backed no-Keychain live
matrix remain open. Phase 2D stays `ACTIVE / PARTIAL`.

`CURRENT / P2D-W2C OBSERVED MANAGED SOURCE BASELINE (2026-08-12)`: each
built-in Mission compile now creates one content-addressed, path-free source
observation and binds its digest to every primary and verifier execution. The
Role Context Capsule classifies it as observed/team-shared and discloses only
schema, kind, tree digest, counts, and total bytes. Top-level `.git`, file names,
paths, and contents remain outside the Capsule.

Ordinary retry keeps the same Capsule digest. If the source changes after
compile but before the managed workspace is prepared, execution fails with
`source_changed` before any Harness adapter or Provider call. Artifact revision
bindings remain separate authoritative items. Loom still does not synthesize
Goal, confirmed-constraint, accepted-decision, observed-test-state, or prior
model-output items without a valid source. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-managed-source-workspace-snapshot.md`.

`CURRENT / P2D-W2D VAULT ACCOUNT-LOCAL LEASE CONCURRENCY (2026-08-12)`: source
review found that the Vault runtime held its lifecycle mutex through the full
remote Provider callback, serializing otherwise independent Provider Accounts.
A RED test held a DeepSeek callback open and proved an Anthropic callback could
not enter. `UseCredential` now snapshots the active exact lease access under
the runtime lock and executes the callback after releasing it.

`CredentialLeaseManager` still owns callback cancellation, expiry, revoke,
rotation exclusion, and plaintext zeroization. The focused normal and race
matrix passes together with Vault lock and rotation regressions. Evidence is
`.loom-evidence/phase2d/P2D-W2D-vault-account-local-lease-concurrency.md`.

Both increments pass complete affected packages and race, repository vet,
serialized full Go, and complete Swift verification. They are frozen as the
unlaunched, uninstalled v0.5.2 build 47 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build47-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, contract-string, and ZIP
byte-equivalence gates pass. The Candidate App and bundled daemon were not
launched.

Installed Loom remains v0.5.2 build 39. CV6, real account-level limits/cost
accounting, approved fallback, and installed mixed-Team isolation remain open.
Phase 2D stays `ACTIVE / PARTIAL`.

`CURRENT / P2D-W2A/W2D CANONICAL MISSION CONTEXT AND EXACT RESTART AUTHORITY
(2026-08-12)`: Mission preflight/start now carry Canonical Mission Context v1:
the Goal, ordered confirmed constraints, and ordered accepted decisions. The
existing preflight digest freezes the exact context. Role Capsules preserve
each class as separate authoritative items while keeping the managed source
baseline observed. Legacy objective-only commands remain readable only when
all v1 context fields are absent.

Preflight is zero-write. Start stores each complete canonical Role Capsule body
and derived dispatch in one authenticated encrypted Vault envelope. Journal,
projection, diagnostics, and Evidence retain no body or Prompt; Journal keeps
only the Capsule authority record. A matching legacy dispatch-only ciphertext
may upgrade once when its authority and validated dispatch are identical, but
cannot downgrade or accept divergent content.

Restart no longer treats current-source recompilation as the frozen context.
It lists validated non-secret authority records for the exact Mission, decrypts
the unique planned Capsule for each execution, restores its source digest and
dispatch, and checks projected Attempts against their complete frozen binding.
Missing or legacy-only body, ambiguous authority, AEAD failure, Team/Agent/Role
or route drift, credential reference/revision or limit drift, and source drift
all fail before adapter dispatch. Recovery performs no Capsule write.

Affected packages and race, serialized `go test -p 1 ./... -count=1`,
`go vet ./...`, and `git diff --check` pass. Swift passes 182 XCTest cases with
one intentional visual-preview skip plus eight Swift Testing contracts, all
with zero failures. Exact scope and open gates are
`.loom-evidence/phase2d/P2D-W2A-W2D-canonical-mission-context-restart-authority.md`.
The source is frozen as the unlaunched, uninstalled v0.5.2 build 48 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build48-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, contract-string, and ZIP
byte-equivalence gates pass. Installed Loom remains build 39; Workbench
editing/confirmation UX, installed restart/live reply, mixed-Team failure
isolation, accounting, fallback, and CV6 remain open. Phase 2D remains
`ACTIVE / PARTIAL`.

`CURRENT / P2D-W2A MISSION CONTEXT CONFIRMATION UX (2026-08-12)`: New Mission
now presents the Goal, ordered confirmed constraints, and ordered accepted
decisions as one native Mission Context review surface. Users may add or remove
individual authority rows, but must explicitly check `I confirm this Mission
context` before preflight. Objective, Team, work type, constraint, or decision
edits revoke that confirmation and invalidate the prepared preflight.

The Store now fences preflight by generation. A late successful response for
an invalidated context cannot restore `ready` or make Start available.
Unchecking confirmation invalidates the current result. Empty,
whitespace-drifted, oversized, and duplicate authority items, including a value
repeated across constraint and decision classes, fail before execution IPC.
The sheet uses one scroll region with a fixed command bar so Review and Start
remain reachable as context grows.

Focused context, race, continuity, and source-contract tests pass. Complete
Swift verification passes 186 XCTest cases with one intentional visual-preview
skip plus eight Swift Testing contracts, all with zero failures. Native sheet
layout passes in light appearance and dark appearance with accessibility
Dynamic Type. Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2D-canonical-mission-context-restart-authority.md`.

This source is frozen as the unlaunched, uninstalled v0.5.2 build 49 Candidate
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build49-candidate-2026-08-12/BUILD-MANIFEST.md`.
Installed Loom remains build 39. Installed interaction/pixel review,
Conversation Segment transition confirmation, real Provider reply,
mixed-Team failure isolation, accounting, approved fallback, and CV6 remain
open. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2A CONVERSATION ROUTE TRANSITION CONFIRMATION (2026-08-12)`:
an active ordinary Conversation now reviews a Provider route change before it
mutates the selected Profile. The native sheet shows the current and proposed
Harness, exact non-secret Provider Account, Model, and credential revision;
states that Loom will create a new immutable Segment; and requires Continue
with context, Summary only, or Start clean.

The detached Composer Context mode menu is removed. Source/target Profile
records, thread anchor, and Store generation are frozen for review. Selection,
anchor, directory, or credential-revision drift fails closed and asks the user
to choose the route again. A first Profile on a blank Conversation remains a
direct selection because no prior Segment or disclosure exists. Provider-native
state and credentials are never reused across the new Segment.

Focused transition, stale-anchor, source-contract, and native light/dark
accessibility layout tests pass. Complete Swift verification passes 191 XCTest
cases with one intentional visual-preview skip plus eight Swift Testing
contracts, all with zero failures. Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-conversation-route-transition-confirmation.md`.

This source is frozen as the unlaunched, uninstalled v0.5.2 build 50 Candidate
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build50-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, route-contract, and ZIP
byte-equivalence gates pass. Installed Loom remains build 39. Installed
Codex-to-DeepSeek transition and reply, disclosure inspection, restart
continuity, mixed-Team isolation, accounting, approved fallback, and CV6
remain open. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2B/W2D MULTI-AGENT TEAM BUILDER AUTHORING (2026-08-12)`:
the App no longer treats all sub-Agents as one global editable slot. Builder
IPC now carries an exact optional `role_agent_definition_id`; every sub-Agent
role, Harness, Provider Account, Model, reasoning, timeout, budget, fallback,
and removal edit requires a unique current target. Add Agent rejects duplicate
identities and cardinality overflow. Missing or stale targets fail without
changing Draft revision or binding digest.

Fallback RouteSet state is per Agent. Successive customizations continue from
the session-local immutable Profile, so switching Reviewer to a DeepSeek
account and then changing only its timeout retains that exact account and
credential revision while Researcher remains unchanged. The built-in catalog
now publishes Coordinator, Bounded Worker, Reviewer, Researcher, and Verifier
identities across each compatible verified route. Swift exposes Add Agent and
icon-only Remove Agent controls plus role-local execution controls. The
concrete IPC client forwards the complete closed edit set rather than rejecting
non-role fields before UDS.

Focused service RED/GREEN, complete 191-XCTest/eight-Swift-Testing verification
with one intentional visual-preview skip, strict Go/Swift contract probes,
affected race, full `go vet ./...`, and `git diff --check` pass. One concurrent
full-Go run observed a transient strict-loopback `unavailable`; its exact case
passed five consecutive isolated reruns. No App was installed or launched and
no credential or Provider request was used.

The final serialized `go test -p 1 ./... -count=1 -timeout=15m` repository gate
passes every package. Exact evidence is
`.loom-evidence/phase2d/P2D-W2B-W2D-multi-agent-builder-authoring.md`.

This closes the multi-Agent App authoring gap, not the full W2D failure
isolation gate. Mission start still fails the whole preflight when one required
role begins blocked because no authoritative initial-blocked plan-node state
exists yet. Healthy-sibling start with one blocked Agent, installed four-route
authoring/dispatch, account-local accounting/fallback, and CV6 remain open.
This source is frozen as the unlaunched, uninstalled v0.5.2 build 51 Candidate;
installed Loom remains build 39 and Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D AUTHORITATIVE INITIAL AGENT FAILURE ISOLATION
(2026-08-12)`: Mission start no longer rejects the whole Team merely because
one role failed Provider Account, credential revision, Runtime compatibility,
or capacity preflight. The compiler emits a canonical direct block for the
affected role and deterministically propagates `dependency_blocked` only to
declared dependents. Independent healthy siblings remain schedulable.

`TeamExecutionPlanned` now freezes a privacy-safe initial Route summary for
every Agent and is followed by one append-only `TeamNodeInitiallyBlocked` fact
per blocked node. The summary contains Harness, Provider, non-secret Provider
Account, Model, reasoning, timeout, budget, capability set, and credential
revision. It excludes credential reference, endpoint fingerprint, binding
digest, Capsule content, Prompt, Provider response, and secret bytes. The
first Attempt must match that summary; explicit fallback remains a separate,
approved Attempt binding.

The coordinator creates no Attempt, grant, capacity reservation, or Adapter
call for an initially blocked node. Runtime capacity admission ignores nodes
already in an authoritative terminal or non-dispatchable state, so a missing
or offline blocked Runtime cannot reintroduce Team-global failure. An
all-blocked Team still receives observable blocked authority and a terminal
Team fact with zero Attempts. Source tests prove one blocked dependency plus
one healthy sibling yields blocked dependent Main, blocked affected Agent, and
succeeded healthy Agent.

Team Pulse now projects the initial Route, exact safe stage/code/retryability,
Incident ID, and actionable diagnostic or retry-review controls before an
Attempt exists. Swift wire validation accepts only the bounded correlated
shape. The UI says `Not started` instead of `Attempt 0`, and native routes say
`Native auth` instead of `Credential v0`. Known block reasons remain bounded
safe text instead of collapsing to Team-level `Unavailable` or `offline`.

Affected normal and race packages, full `go vet ./...`, `git diff --check`,
serialized `go test -p 1 ./... -count=1 -timeout=15m`, and complete Swift
verification pass. A post-package product review also found and closed the
remaining Swift Store gate: `blocked + ready` now permits Start while
`blocked only` remains fail closed. Swift reports 194 XCTest cases with one
intentional visual preview/export skip plus eight Swift Testing contracts, all
with zero failures. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-authoritative-initial-agent-failure-isolation.md`.

This closes the source authority and App presentation gap identified after
build 51. It does not close installed four-Provider dispatch, real account
revocation/rate-limit/timeout isolation, Provider Account cost accounting,
approved live fallback, or Credential Vault CV6. Keychain remains only an
optional explicit one-time migration source; normal runtime remains Loom
Credential Vault. Installed Loom remains v0.5.2 build 39. Build 52 was rejected
by static product review and never installed or launched. The corrected source
is frozen as unlaunched, uninstalled v0.5.2 build 53 and Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D ACCOUNT-LOCAL PROVIDER TIMEOUT CLASSIFICATION
(2026-08-12)`: native OpenAI-compatible Agent adapters and the bounded
Claude/Codex Harness gateway no longer collapse every network timeout into
`provider_unavailable`. A timeout before an upstream response is established
is now `timeout / provider_connect / retryable`; a timeout while reading an
upstream response is `timeout / provider_http / retryable`. An outer Agent
Attempt deadline remains the separate authoritative runtime timeout.

The diagnostic retains the exact Incident ID, Provider, non-secret Provider
Account, and Model from the frozen Attempt binding. Team Board enrichment
accepts only that exact tuple, rejects unknown stages, and leaves healthy peer
Agents without another account's failure. Provider response bodies, Prompt,
credential reference, endpoint fingerprint, and secret bytes do not enter the
diagnostic, Board, Journal, or Evidence.

This increment revalidates the accepted Credential Vault boundary rather than
creating another credential design. ADR-0020 and CV1-CV5 remain the source
contract: normal Conversation and Agent dispatch acquire exact
account/reference/revision leases from the in-process Loom Vault; Keychain is
only an optional explicit one-time migration source, with no plaintext
fallback. CV6 remains open for approved installed DeepSeek restart/multi-turn
and four-Provider mixed-Team observation.

Focused RED/GREEN, affected normal and race packages, serialized full Go,
`go vet ./...`, `git diff --check`, and complete Swift verification pass.
Swift reports 194 XCTest cases with one intentional visual preview/export skip
plus eight Swift Testing contracts, all with zero failures. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-account-local-provider-timeout-classification.md`.

Build 53 remains valid historical source/static evidence for initial-block
isolation but is superseded by the current source. Build 54 is frozen as an
unlaunched, uninstalled Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build54-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, timeout-contract, and ZIP
byte/mode-equivalence gates pass. Installed Loom remains build 39.
Installed real revocation, auth, rate-limit and timeout isolation, Provider
Account cost observation, approved fallback, and the full CV6 matrix remain
open. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D PROVIDER ACCOUNT GOVERNANCE VISIBILITY (2026-08-12)`:
Team Pulse now presents the already-authoritative per-account aggregation as a
distinct governance row instead of burying it among Agent rows. Each row names
Provider plus exact non-secret Provider Account and shows failed/total
Attempts, exact error rate, rate-limited Attempts, accounting coverage, token
usage, account policy revision, active concurrency, dispatch window, assigned
budget ceiling, costs grouped by currency, and aggregation-incomplete state.

Cost display converts integer microunits to an exact user-readable decimal
without floating-point or locale drift; error basis points use exact bounded
percentage formatting. Agent rows keep the person icon while account rows use
a separate system account icon and `Provider Account governance` accessibility
label. The same Team Pulse remains one unframed inspector list; no parallel
state authority or nested card was added.

This UI consumes Board values aggregated from each Attempt's frozen Provider
Account and terminal accounting. It does not infer account data from Provider
ID, does not expose credential reference or secret, and does not turn
operational statistics into retry/fallback authority. Cross-account isolation,
strict Swift wire validation, and backend aggregation tests remain green.

The RED fixture proved that error rate, rate-limit count, accounting coverage,
and readable cost were absent from the product presentation. GREEN focused and
complete Swift verification passes 195 XCTest cases with one intentional
visual preview/export skip plus eight Swift Testing contracts; affected API,
projection, and app Go packages and `git diff --check` pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-provider-account-governance-visibility.md`.

Build 54 remains historical unlaunched, uninstalled timeout-classification
evidence and is superseded by the current source. Build 55 is frozen as an
unlaunched, uninstalled Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build55-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, account-governance source,
and ZIP byte/mode-equivalence gates pass. Installed Loom remains build 39. Real installed account cost,
revocation/auth/rate-limit/timeout isolation, approved fallback, and CV6 remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2D PROVIDER ACCOUNT COST PROVENANCE (2026-08-12)`:
new Run terminal cost facts now freeze an explicit source. Claude Code and Pi
RPC values are `harness_reported`; a bounded Provider result may be
`provider_reported`. Historical Journal events that genuinely lack the new
field replay as `legacy_unspecified`, while new writes with missing, explicit
legacy, empty, unknown, or estimated sources fail closed.

Attempt rows and exact Provider Account rows carry the source through strict
Projection, API, and Swift wire models. Account aggregation keys cost by both
currency and source, so Provider-reported and Harness-reported USD are not
silently merged. Team Pulse renders readable labels such as `Harness reported
cost`, preserving the exact integer microunit formatting.

Loom still does not invent prices. Native OpenAI-compatible DeepSeek, Kimi, and
MiniMax execution currently reports token usage with cost unobserved.
`rate_card_estimate` remains reserved and rejected until a versioned Rate Card
identity, revision, digest, currency, and calculation basis are frozen with the
Attempt.

Focused RED/GREEN, affected normal/race packages, serialized full Go,
`go vet ./...`, `git diff --check`, and complete Swift verification pass.
Swift reports 195 XCTest cases with one intentional visual preview/export skip
plus eight Swift Testing contracts, all with zero failures. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-provider-account-cost-provenance.md`.

Build 55 remains historical unlaunched, uninstalled account-governance UI
evidence. Build 56 is superseded historical cost-provenance evidence after the
final decoder audit distinguished a missing field from explicit JSON `null`.
Build 57 is the current
unlaunched, uninstalled Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build57-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, cost-provenance source,
and ZIP byte/mode-equivalence gates pass. Installed Loom remains build 39.
Installed real Provider Account cost, Rate Card authority, account-local
revocation/auth/rate-limit/timeout, approved fallback, mixed-Team execution,
and Credential Vault CV6 remain open under the sole `ACTIVE / PARTIAL` Phase
2D Goal.

`CURRENT / P2D-W2D RATE CARD AUTHORITY AND VAULT REVALIDATION (2026-08-12)`:
Provider Account and Model Rate Cards are now append-only authority keyed by
exact Provider, Provider Account, and Model. Each revision freezes a digest,
currency, token basis, integer microunit rates, and deterministic rounding.
Run Claim contract v2 freezes the complete card per Attempt;
`rate_card_estimate` is accepted only from that frozen card and observed token
usage. Provider- and Harness-reported costs remain distinct and take
precedence. Loom ships no hardcoded official Provider prices.

Provider setup and Swift UI can configure and display each account/model Rate
Card. Board and Team Pulse expose revision, currency, basis, and estimate
source without exposing credential references or secrets. A Run replay repair
now loads the exact binding's policy, capacity, and Rate Card streams before
strictly replaying a v2 Claim, preserving CAS conflict detection and avoiding a
false `run authority conflict` during selective reads.

The accepted ADR-0020 Credential Vault remains the sole credential architecture
inside Phase 2D. Source audit confirms ordinary daemon composition defaults to
LocalKeyFile Vault and both Conversation and Agent hot paths use exact
account/reference/revision leases. Non-test daemon source does not construct
`ProductKeychainStore`; Keychain is only an optional, explicit one-time
migration source, and there is no plaintext fallback. CV1-CV5 remain source
Candidates rather than a new Goal.

Fresh verification passes full `go test ./... -count=1`, affected race tests,
`go vet ./...`, `git diff --check`, 198 XCTest cases with one intentional
visual export skip, and nine Swift Testing contracts with zero failures. Build
58 is frozen as an unlaunched, uninstalled source/static Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build58-candidate-2026-08-12/BUILD-MANIFEST.md`;
release, arm64, deep-signature, owner-only/no-symlink, no-implicit-Keychain, and
ZIP byte/mode-equivalence gates pass. Installed Loom remains v0.5.2 build 39.
Installed real account cost, counter-backed no-helper
observation, rotation/restart continuity, single-account revoke/corruption
isolation, approved fallback, and the four-Provider mixed-Team CV6 matrix remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2D FOUR-PROVIDER VAULT REVOKE ISOLATION (2026-08-12)`:
the controlled Team coordinator now composes its four independent frozen Agent
bindings with the real in-process Credential Vault lease manager. The matrix
covers Codex/OpenAI revision 3, Claude Code/Anthropic revision 7, Loom
Native/Kimi revision 11, and Loom Native/MiniMax revision 13, each with a
different Provider Account, credential reference, Model, Harness, limits, and
capabilities.

Revoking only the exact MiniMax account/reference/revision before dispatch
blocks that Agent as `credential_unavailable`; the OpenAI, Anthropic, and Kimi
Agents still execute and reach `succeeded`. The Team terminal fact remains
honestly blocked because one required Agent failed, while per-Agent authority
preserves three successes and one block instead of inventing Team-wide
`offline` or `Unavailable` state.

Team Board regression now also covers the exact privacy-safe operational
classification `credential_lease_revoke / credential_unavailable / retryable`
for the MiniMax Attempt. Incident, Provider, Provider Account, and Model must
all match before enrichment; healthy peers do not receive the failed account's
diagnostic. Operational data remains presentation-only and cannot authorize a
retry or fallback.

Focused normal and ten-run race matrices, full Go, `go vet ./...`,
`git diff --check`, and complete Swift verification pass. Swift reports 198
XCTest cases with one intentional skip plus nine Swift Testing contracts. Exact
evidence is
`.loom-evidence/phase2d/P2D-W2D-four-provider-vault-revoke-isolation.md`.

Build 58 is now historical. The later real diagnostic-chain and retry-governance
increment changes production daemon bytes; build 59 captured the first version
and build 60 below contains the final closed retry allowlist.
Installed Loom remains v0.5.2 build 39. Installed real four-Provider dispatch
and revoke/corruption observation, counter-backed no-helper operation,
restart/rotation continuity, Provider replies, accounting, approved fallback,
and CV6 remain open. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D FOUR-PROVIDER ENCRYPTED-RECORD CORRUPTION ISOLATION
(2026-08-12)`: the four-Provider Team source matrix now uses a real owner-only
LocalKeyFile and encrypted SQLite VaultStore in addition to the earlier lease
revoke case. It writes four exact Agent identities, closes the Store, verifies
that the database contains none of the four plaintext test secrets, corrupts
only the MiniMax ciphertext, and reopens with the original key file.

All four Agent Adapters then acquire short-lived credentials through one
production `CredentialLeaseManager`. MiniMax fails closed at `vault_decrypt`
and is blocked as `credential_unavailable`; Codex/OpenAI, Claude
Code/Anthropic, and Loom Native/Kimi still decrypt their independent records
and succeed. Team Journal facts contain no test secret bytes.

Board regression also accepts
`vault_decrypt / credential_unavailable / not retryable` only for an exact
Incident, Provider, Provider Account, and Model match. It does not annotate a
healthy Agent or authorize retry/fallback.

Focused ten-run normal and race matrices, full Go, `go vet ./...`,
`git diff --check`, complete Swift, and non-disclosure scans pass. Exact
evidence remains
`.loom-evidence/phase2d/P2D-W2D-four-provider-vault-revoke-isolation.md`.
This closes source composition for encrypted single-record corruption, not
installed CV6. Installed Loom remains v0.5.2 build 39, and installed real
Provider dispatch, no-helper observation, restart/rotation continuity,
revoke/corruption isolation, accounting, approved fallback, and Provider
replies remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2D FOUR-PROVIDER VAULT ROTATION AND RESTART CONTINUITY
(2026-08-12)`: the same exact four Agent credential records now pass the
production rotation transaction and a Vault restart. A pre-rotation OpenAI
lease is revoked by the lease barrier; every DEK is rewrapped, the version-2
LocalKeyFile is promoted and adopted, and no pending key remains.

After closing and reopening the VaultStore and CredentialLeaseManager, all four
Agents acquire fresh leases and succeed. Their frozen binding digests and
credential revisions remain exactly unchanged, proving rotation cannot silently
switch Provider, Provider Account, Credential Reference, Model, Harness,
limits, or capabilities.

Ten-run normal and race rotation matrices, full Go, `go vet ./...`,
`git diff --check`, and non-disclosure scans pass. This closes source-level
post-rotation restart continuity, not installed CV6. Installed Loom remains
v0.5.2 build 39; real Provider calls before and after installed
restart/rotation, the no-helper counter, installed mixed-Team isolation,
accounting, and approved fallback remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2D REAL VAULT FAILURE DIAGNOSTIC CHAIN AND RETRY GOVERNANCE
(2026-08-12)`: source acceptance now exercises a corrupted encrypted DeepSeek
credential through the complete production composition: owner-only
LocalKeyFile, encrypted VaultStore, CredentialLeaseManager, exact frozen
Provider Account/reference/revision validation, Loom Native Adapter, persistent
operational diagnostics, and exact Agent Attempt diagnostic query.

The corrupted record fails at `vault_decrypt`, the Attempt projects
`credential_unavailable`, and the Provider HTTP client is never called. The
persistent diagnostic is keyed by the same Incident ID, Provider, Provider
Account, and Model and contains no credential reference, secret, Prompt,
ciphertext, or private lower-level error. This closes the missing source proof
between the real Vault failure and Team Board enrichment.

Credential retry governance is now shared by Loom Native and Claude/Codex
Harness adapters. Recovery-required stages `vault_key_load`, `vault_open`,
`vault_decrypt`, `vault_aad_validation`, and `vault_recovery` are not presented
as directly retryable. Lease issue, expiry, and revoke retain retryable
semantics because their authoritative state can change independently. This
removes the previous contradiction where the Board called corruption not
retryable while adapters recorded it as retryable.

Focused normal and ten-run race contracts, full Go, `go vet ./...`,
`git diff --check`, the production non-disclosure scan, and complete Swift
verification pass. Swift reports 198 XCTest cases with one intentional visual
export skip plus nine Swift Testing contracts, all with zero failures.

The production change is frozen as unlaunched, uninstalled v0.5.2 build 60 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build60-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, independent byte rebuild, arm64, deep signature, owner-only/no-symlink,
no-implicit-Keychain, and ZIP byte/mode-equivalence gates pass. Builds 58 and
59 are superseded historical Candidates; build 60 adds a strict allowlist so
helper, metadata, migration, and unknown stages cannot silently become
retryable. Installed Loom remains build 39; installed Provider calls,
counter-backed no-helper operation, rotation/restart continuity,
revoke/corruption isolation, accounting, approved fallback, and CV6 remain open
under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2D VAULT-BACKED APPROVED FALLBACK AND MIXED-TEAM ISOLATION
(2026-08-12)`: explicit fallback now has real encrypted Vault composition
evidence. In the single-Agent case, the primary OpenAI record is corrupted and
fails at `vault_decrypt`; an independent Anthropic fallback record is acquired
only by the second Attempt whose target binding digest is covered by the
versioned user approval. The fallback has a distinct account, revision,
binding digest, and Role Context Capsule. Unapproved binding changes still fail
before dispatch.

The four-Provider Team matrix then combines Codex/OpenAI, Claude
Code/Anthropic, Loom Native/Kimi, and Loom Native/MiniMax. The Codex primary
account revision 3 is corrupted while an explicitly approved
`openai.main-backup` revision 5 remains healthy. Four Agents produce five
Attempts: failed main primary, successful approved main fallback, and three
successful peers. Each of the five complete Provider/Account/Reference/Revision
identities is acquired exactly once; no Provider-global lookup or silent route
change occurs, and no test secret reaches the Journal.

Accounting is frozen on those same Attempts. The failed `openai.main`
credential Attempt has no accounting fact. The successful
`openai.main-backup` Attempt owns its controlled 55-token/5,000-microunit fact,
and Anthropic, Kimi, and MiniMax retain separate 77/7,000, 121/11,000, and
143/13,000 token/microunit facts. Nothing is reassigned to the primary account.
These are deterministic source fixtures, not real Provider invoices.

Focused normal and race matrices each pass ten runs. Full Go, `go vet ./...`,
`git diff --check`, and non-disclosure scans pass. This increment changes only
Go test and Phase evidence source. A fresh production rebuild remains
byte-identical to v0.5.2 build 60 for the App executable, bundled daemon, and
`Info.plist`, so that test-only increment did not create a new build. Swift was
not rerun for that increment; build 60's unchanged production source retains
its earlier 198 XCTest plus nine Swift Testing result.

Installed Loom remains build 39. Installed approved fallback, real mixed-Team
Provider dispatch/accounting, counter-backed no-helper behavior, rotation and
restart continuity, and CV6 remain open under the sole `ACTIVE / PARTIAL`
Phase 2D Goal.

`CURRENT / P2D-W2D AGENT-SCOPED VAULT RECOVERY ACTION (2026-08-12)`:
the Team Inspector now turns durable, non-retryable Credential Vault failures
into an executable recovery path. An affected Agent row shows a key action that
opens Runtime & Providers at the Credential Vault surface while retaining its
separate diagnostics and Incident ID actions.

The capability uses a closed Vault-stage allowlist. It is not offered for
Provider auth, rate limit, timeout, or retryable lease failures, and it is not
projected onto healthy peer Agents or Provider Account summary rows. This keeps
recovery account-local and avoids turning one broken credential into a Team-wide
offline state.

Complete verification passes 199 XCTest cases with one intentional visual
export skip, nine Swift Testing contracts, full Go, ten race-detector runs of
both Vault-backed approved-fallback matrices, `go vet ./...`,
`git diff --check`, plist validation, and non-disclosure scans. The native App
fixture passes two clean reproducible release builds and its temporary smoke
gate.

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 61
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build61-candidate-2026-08-12/BUILD-MANIFEST.md`.
An independent retained rebuild and ZIP extraction are byte- and mode-identical
to the Candidate. Build 60 is historical. Installed Loom remains build 39; no
real credential, installed App, or live Provider matrix was touched. Installed
CV6 acceptance remains open under the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2A/W2D CONVERSATION VAULT FAILURE GOVERNANCE (2026-08-12)`:
ordinary Conversation dispatch now shares the exact Credential Vault retry
policy already used by Agent Attempts. Temporary lease transitions remain
retryable. Durable `vault_decrypt` and `vault_aad_validation` failures preserve
their stage through the Conversation Router, IPC `recoverable`, and failed
Attempt, and are not exposed as futile Retry loops.

The chat failure banner provides an explicit Open Credential Vault action for
only a closed allowlist of non-recoverable Vault stages. Provider auth, rate
limit, timeout, route conflict, and retryable lease failures cannot receive the
action. Privacy-safe diagnostics and Incident ID copy remain separate.

Focused normal and race tests, complete Swift, full Go, and `go vet ./...`
pass. Swift reports 200 XCTest cases with one intentional visual export skip
plus nine Swift Testing contracts, all with zero failures. Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2D-conversation-vault-failure-governance.md`.

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 62
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build62-candidate-2026-08-12/BUILD-MANIFEST.md`.
An independent retained rebuild and ZIP extraction are byte- and mode-identical
to the Candidate. Build 61 is historical. Installed Loom remains build 39; no
real credential, installed App, or Provider was touched. Installed DeepSeek
conversation recovery and the full CV6 matrix remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

Release verification continues to report the existing Swift 6
forward-compatibility warning that `QueueCommand<Input>` declares `Sendable`
while `Input` is constrained only to `Encodable`. Current-language builds pass;
the warning is recorded as non-blocking debt and was not folded into this
narrow failure-governance increment.

`CURRENT / P2D-W2A/W2D PROVIDER ACCOUNT DISCLOSURE POLICY V2 (2026-08-12)`:
Provider Account policy now authoritatively versions and digests a closed trust
domain, retention mode, and data region alongside concurrency, dispatch, and
budget limits. Historical v1 events and digests remain unchanged and replay as
disclosure policy unspecified; only complete, valid v2 settings are accepted
for new App configurations.

The exact account policy version, revision, digest, and disclosure values now
project through setup and each Conversation Profile. Swift publishes success
only after the mutation response matches a rebuilt authoritative account
snapshot. Runtime & Providers exposes compact native Pickers and explicitly
states that Loom records the selected policy rather than certifying Provider
retention or region guarantees.

Complete Swift passes 200 XCTest cases with one intentional visual export skip
plus nine Swift Testing contracts. Full Go, five focused race runs, and
`go vet ./...` pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2D-provider-account-disclosure-policy-v2.md`.

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 63
at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build63-candidate-2026-08-12/BUILD-MANIFEST.md`.
An independent retained rebuild made after `swift package reset` and an
independent ZIP extraction are byte- and mode-identical to the Candidate. Build
62 remains historical evidence for the Conversation Vault failure-governance
slice. Installed Loom remains build 39; no real credential, installed App, or
Provider was touched. Freezing policy revision/digest on Conversation Segment
and Attempt records is the next source slice; installed CV6 remains open under
the sole `ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2A/W2D CONVERSATION POLICY BINDING V3 (2026-08-12)`:
Conversation Segments and Attempts now freeze the daemon-resolved Provider,
exact Provider Account, policy version/revision/digest, trust domain, retention
mode, and data region. The frozen policy is visible in the Segment disclosure
inspector and is not replaced by the account's current settings.

Route review now carries the exact non-secret execution binding confirmed by the
user through Swift and private UDS. The daemon independently re-resolves current
authority and rejects any mismatch before persisting the user message, creating
an Attempt, acquiring a Vault lease, or calling a Provider. This closes the
confirmation-to-dispatch policy race without placing credential reference,
secret, Prompt, message content, or Provider response in the binding.

Route conflicts preserve the same visible Conversation, selected target Profile,
draft, authoritative old thread, and Incident ID. Retry returns to explicit
Segment and Context review instead of silently opening a separate Conversation.
Legacy schema-2 Conversations retain their old digest and gain a new schema-3
Segment on the next turn.

Complete Swift passes 202 XCTest cases with one intentional visual-export skip
plus nine Swift Testing contracts. Full Go, five focused race runs,
`go vet ./...`, strict IPC/model/UI contracts, and `git diff --check` pass.
Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2D-conversation-policy-binding-v3.md`.

The production increment is frozen as unlaunched, uninstalled v0.5.2 build 64 at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build64-candidate-2026-08-12/BUILD-MANIFEST.md`.
Two independent builds made across `swift package reset` and an independent ZIP
extraction are byte- and mode-identical. Both bundles pass arm64 and deep strict
ad-hoc signature verification; no Candidate process exists. Build 63 is
historical evidence for the pre-binding disclosure-policy slice.

Installed Loom remains v0.5.2 build 39; no real credential, installed App, or
Provider was touched. Installed route transitions, real Provider replies,
restart continuity, mixed-Team dispatch, accounting, approved fallback,
no-helper observation, and CV6 remain open under the sole `ACTIVE / PARTIAL`
Phase 2D Goal.

`CURRENT / P2D-W2A/W2C STRUCTURED CONVERSATION CAPSULE V1 (2026-08-12)`:
ordinary non-Agent Conversation dispatch now uses the same structured,
content-addressed Role Context Capsule and encrypted Conversation-DEK Store as
Team Agent dispatch. User turns remain authoritative; prior visible model output
is untrusted, explicitly policy-filtered for `summary_only`, and excluded by
`start_clean`. Hidden reasoning, credentials, Provider bodies, and local failure
messages do not enter the Capsule.

The daemon resolves the exact ContextAdapter, Provider Account, Model, auth mode,
and disclosure-policy identity from the immutable Conversation Profile. Capsule
encryption precedes thread metadata and Provider dispatch. Failure leaves no
message, Segment, Attempt, or Provider call; a later thread-document failure
performs an exact Authority-bound compensation delete. Real LocalKeyFile
close/reopen tests recover the Capsule and raw database scans find no plaintext
context.

Focused transaction, target-resolution, restart, non-disclosure, Pi, Codex, and
daemon tests pass; the affected race matrix passes three runs. Full Go, vet,
Swift, and diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2C-structured-conversation-capsule-v1.md`.

This source is post-build-64 and is not packaged or installed. Installed Loom
remains v0.5.2 build 39; no real credential or Provider was touched. W2A/W2C
remain `ACTIVE / PARTIAL` for tokenizer-aware packing, Provider-specific wire
mapping, scoped retrieval, complete receipt/approval UI, parallel aggregation,
encrypted export, and the installed CV6 matrix under the sole Phase 2D Goal.

`CURRENT / P2D-W2C SCOPED CONTEXT RETRIEVAL BROKER V1 (2026-08-12)`:
token-budget omissions now remain encrypted inside Context Capsule envelope v2
and can be read only by an exact daemon-frozen Attempt identity. The broker
binds Capsule Authority, WorkItem, Run, claim generation, Runtime,
execution-binding digest, Agent, Role, item/content digest, and artifact scope.
Only `budget_exceeded` omissions qualify; policy-filtered, access-denied,
credential, hidden-reasoning, and Provider-body content is never retrievable.

Retrieval emits a content-free operational diagnostic before plaintext is
released and fails closed if auditing fails. The returned mutable buffer is
zeroized on close. Real LocalKeyFile close/reopen tests recover the exact scoped
item, structured v1 records upgrade only under exact Authority and dispatch,
v2 downgrade fails, and raw Vault database scans find no omitted plaintext.

Team execution carries Capsule Authority into managed execution. Production
daemon composition creates the scoped broker only when the Vault retrieval
store and operational auditor are both present. Supervisor validation still
binds Authority to the frozen execution binding and dispatch when the optional
retriever is absent; explicit legacy/test runtimes therefore receive no
omission-read capability rather than a plaintext fallback.

Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-scoped-context-retrieval-broker-v1.md`.
The same post-build-64 source now provides a bounded Loom Native wire:
DeepSeek, Kimi, and MiniMax can propose one strict `loom_read_context` call,
consume the validated result in one second Provider round, and finish the same
Attempt. Retrieved content is not emitted to Bridge frames, Journal, Evidence,
or diagnostics. Duplicate/unknown protocol fields, identity/digest/body or
trust/scope drift, denied reads, secret markers, and a second tool request fail
closed; both rounds' token usage is attributed to the same Attempt.

This is not the complete general Tool Loop. Pi private extension delivery was
subsequently completed by the 2026-08-13 increment below. Codex/Claude Code
Attempt-scoped MCP, persisted result delivery and crash resume, parallel
aggregation, encrypted export, and CV6 remain open. This source is
post-build-64; installed Loom remains v0.5.2 build 39 and was not modified.
Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C CONTEXT RETRIEVAL CAPABILITY FREEZE (2026-08-12)`: the
bounded Loom Native wire is no longer enabled merely because daemon composition
has a Vault retrieval store. Loom Native runtime discovery now publishes the
explicit `context_retrieval` capability; newly generated DeepSeek, Kimi, and
MiniMax Execution Profiles require it, and each new Attempt freezes it in the
binding digest. The daemon checks the frozen binding before constructing a
scoped broker, and the Adapter independently rejects both a retriever without
that capability and a capable Attempt without its broker before any credential
lease or Provider request.

Installed historical runtime projections with the exact v1 identity and an
empty capability set receive one append-only, idempotent rediscovery event.
Unknown capability sets or any other Runtime identity drift remain fail-closed.
This migration makes the capability available to new Profiles only; it does not
rewrite existing Team definitions, Profiles, Runs, Attempts, or frozen bindings.
Focused migration, catalog, DeepSeek/Kimi/MiniMax tool, mismatch, and repeated
execution tests pass. The complete affected Runtime, prompting, Capsule, Vault,
Supervisor, Native Adapter, app, API, and daemon race matrix passes three runs;
full `go test ./...`, vet, format, diff, and module verification pass. No bundle
was built, launched, installed, or connected to a real Provider; installed Loom
remains v0.5.2 build 39.

`CURRENT / P2D-W2C PI CONTEXT RETRIEVAL TRANSPORT V1 (2026-08-13)`: the exact
locked Pi 0.82.1 Harness now exposes one private `loom_read_context` tool only
when the Attempt freezes `context_retrieval` and receives the matching scoped
broker. One owner-only extension and one-use UDS bind the exact child PID,
random Attempt capability, item/content digest, artifact scope, trust/source
classification, Capsule Authority, and frozen execution identity. A first Pi
turn may perform exactly one read; the second may emit only final text. The
retrieved body is not projected to Bridge, Journal, Evidence, or diagnostics,
and both model rounds share one overflow-checked Attempt accounting record.

Runtime discovery no longer infers this permission from a `0.82.1` version
string. An explicit conformance port is offered only by the runner whose
executable bytes match the locked digest and whose filesystem bindings still
validate. Ordinary/historical Pi observations therefore remain unchanged;
version drift, missing/failed conformance, typed nil, broker/capability mismatch,
peer mismatch, protocol drift, a second tool, or secret-classified content fail
closed.

The real local locked Pi component completed 20 consecutive race-enabled
two-round tests against a loopback fake endpoint. Full `go test ./...`, affected
package race tests, `go vet ./...`, `git diff --check`, and `go mod verify`
pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-pi-context-retrieval-transport-v1.md`.

This closes only the bounded Pi source transport slice. Codex/Claude Code MCP,
persistent tool-result delivery and crash resume, general multi-tool loops,
parallel aggregation, encrypted export, and installed CV6 remain open. The
source is post-build-64; installed Loom remains v0.5.2 build 39 and was not
modified. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C CODEX AND CLAUDE CONTEXT MCP TRANSPORT V1 (2026-08-13)`:
Codex and Claude Code Harness Attempts now expose the existing scoped
`loom_read_context` broker through one private, one-use loopback MCP service.
The service is created only when the frozen Attempt binding carries
`context_retrieval`, Capsule Authority and dispatch agree, and the exact scoped
retriever is present. A random 256-bit Attempt capability is supplied only in
the child environment; it does not enter argv, Prompt, Journal, Evidence, or
diagnostics. Both transports revalidate item/content digest, artifact scope,
trust and source classification, zeroize the mutable result, and reject a
second call or protocol/metadata drift before disclosure.

The real local Codex 0.144.1 and Claude Code 2.1.196 binaries each completed
their native MCP lifecycle against loopback fake Providers under isolated HOME
and workspace roots. The combined component contract passed 20 consecutive
race-enabled runs. Runtime discovery publishes `context_retrieval` only for the
exact executable SHA-256 identities that passed those gates. Hash drift keeps
ordinary Harness capabilities but receives no Context permission. New Profiles
require the observed capability and freeze it in each Attempt; exact historical
capability sets can receive one append-only upgrade, while unknown sets or
other identity drift remain fail-closed.

Every Context-enabled Attempt re-hashes the exact Harness executable after
Authority and frozen-binding validation but before opening the MCP listener,
acquiring a credential, or starting the process. Executable replacement after
discovery therefore fails closed with no disclosure or Provider access.

Claude 2.1.196 compatibility is closed to its exact `/v1/messages?beta=true`
query and local `HEAD /` health probe. The query is forwarded only for
Anthropic; the health probe returns an empty local 204 and never accesses a
credential or Provider. Query drift and the same probe on OpenAI remain
rejected.

Full `go test ./...`, directly affected daemon race tests (20 runs), real
Codex/Claude component race tests (20 runs), `go vet ./...`, `go mod verify`,
and `git diff --check` pass. A broader daemon race matrix still exposes an
unrelated pre-existing real-Pi-timeout/IPC flaky test; an isolated 10-run check
failed twice with `invalid local IPC protocol`, so this increment does not claim
the complete daemon race suite is green. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-codex-claude-context-mcp-transport-v1.md`.

This closes only the bounded Codex/Claude source transport and exact-binary
capability-freeze slice. Persisted tool-result delivery and crash resume,
general multi-tool loops, parallel aggregation, encrypted export, and installed
CV6 remain open. This source is post-build-64; installed Loom remains v0.5.2
build 39 and was not modified. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C ENCRYPTED ATTEMPT PAYLOAD STORE V1 (2026-08-13)`: the
Credential Vault now persists bounded Attempt payloads under the existing
per-Conversation DEK. Canonical AAD freezes Conversation, WorkItem, Run, claim
generation, Runtime, execution-binding digest, Capsule digest, call, sequence,
content digest, and authenticated `pending`/`delivered` state. Delivery-state
transition decrypts and authenticates the old row, then re-encrypts under a
fresh nonce in the shared Conversation nonce registry.

Focused tests prove restart recovery, idempotency, stale generation and binding
rejection, sequence conflict, AAD/tag/content/status tamper rejection, Vault
rotation continuity, plaintext-negative database scans, and Conversation-local
crypto-erasure with peer isolation. Focused tests passed ten runs, focused race
tests passed twenty runs, and the complete Vault race suite passed ten runs.
Full repository Go tests, vet, module verification, and diff checks also pass.
Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-attempt-payload-store-v1.md`.

This is only the encrypted storage prerequisite. The Context Retriever and Pi,
Codex, Claude Code, and Loom Native transports are not yet wired to a common
persist-before-delivery and acknowledge-before-delivered coordinator, and no
new Journal tool-result facts are emitted. Crash resume, general multi-tool
loops, parallel Aggregation Attempts, encrypted export, installed CV6, and the
live mixed-Team matrix therefore remain open. Source is post-build-64;
installed Loom remains v0.5.2 build 39 and was not modified. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.

## TARGET / ACCEPTED LOOM HARNESS PLATFORM CORE ABSORPTION (2026-08-13)

The Product Owner clarified the architecture after reviewing DeepSeek Harness:
Loom is the Harness Platform, while Codex, Claude Code, Pi, DeepSeek Harness,
and Loom Native are Runtimes. The target adapter seam is therefore
`RuntimeContract` / `RuntimeAdapter`, not a peer `HarnessAdapter` authority.
Loom Native implements the Runtime Contract in daemon without a fictitious
adapter; Provider adapters remain a lower model-protocol concern.

Phase 2D will selectively port DeepSeek Harness Core behavior for Agent
lifecycle, Attempt/Turn/Step loop, unified Queue/Steer/Inject inbox, capability
seams, guarded tool scheduling, parallel/exclusive barriers, cancellation,
resume, and replay. The reviewed source is pinned to
`deepseek-ai/deepseek-harness@47f943859bef60e4160492346772ded9b24f765a`
and MIT notice/provenance is required for substantial copied code.

This is not whole-core vendoring and is not implemented yet. Cordis, a second
SessionEvent authority, plaintext Prompt/tool/session logs, credentials,
runtime self-modification, and ungoverned tools remain excluded. Loom retains
the sole Journal authority, encrypted Payload/Capsule stores, Credential Vault,
Provider Account binding, approval, sandbox, recovery, and terminal decisions.
Existing v0.5.x `Harness`/`AdapterType` records stay replayable until a
deterministic Runtime terminology/schema migration is accepted.

The decision amends ADR-0019 and P2D-W2B/W2C/W2D inside the same sole
`ACTIVE / PARTIAL` Phase 2D Goal. Current source-verified RouteSet aggregation,
DeepSeek conversation work, Credential Vault, and per-Agent binding status are
unchanged.

MCP HTTP, Pi UDS, and Provider HTTP currently expose no common
application-level consumption ACK. The next delivery-coordinator slice must be
explicitly at-least-once, retain the same daemon generation/binding fence, and
must never re-execute a side-effecting tool merely to recover delivery. An
exactly-once claim requires the corresponding Journal facts and
transport-specific acknowledgement/continuation proof.

`CURRENT / P2D-W2C JOURNAL-BACKED ATTEMPT PAYLOAD DELIVERY V1 (2026-08-13)`:
Loom now encrypts one bounded Context result as `pending` before transport and
records content-free `ToolResultAccepted` / `ToolResultDelivered` facts on a
dedicated Journal stream fenced by the exact Run, claim generation, Runtime,
Agent, execution-binding digest, Capsule digest, call, sequence, content digest,
and Incident ID.

Restart recovery returns the same persisted result without calling the scoped
Retriever again. Loom Native acknowledges only after a valid Provider
continuation. Codex and Claude Code keep MCP HTTP writes pending until validated
Harness final output. Pi keeps UDS and `tool_execution_end` pending until the
complete second-turn final output, agent settled state, clean process exit, and
accounting validation. Provider-native tool-call IDs may change across retry;
Loom's semantic call identity does not.

A real LocalKeyFile Vault close/reopen plus SQLite Journal integration test
proves timeout recovery, one retrieval, one accepted fact, one delivered fact,
and plaintext-negative Vault/Journal files. Affected normal and race tests,
vet, module verification, and diff checks pass. Exact local Codex 0.144.1,
Claude Code 2.1.196, and locked Pi 0.82.1 component contracts also pass against
loopback fake Providers; Pi asserts the final delivered transition. A broad internal parallel run
retains one unrelated existing process-cleanup failure where a Harness timeout
test did not observe its child PID under load; the Harness package passes in
isolation. Aggregate runners are also terminated by existing process-group
cleanup tests, so no clean one-command full-repository result is claimed.

Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-journal-backed-attempt-payload-delivery-v1.md`.
This is at-least-once delivery of the same encrypted result without
side-effecting tool re-execution, not a general exactly-once Tool Loop. General
multi-tool execution/approval, terminal reconciliation, parallel Aggregation,
encrypted export, installed CV6, and live mixed-Team acceptance remain open.
Source is post-build-64; installed Loom remains v0.5.2 build 39 and was not
modified. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

Final review additionally makes the bounded single-read authority fail closed
before retrieval or Vault persistence when a second semantic call/sequence is
present, and returns a stable authority error for a nil Context instead of
panicking. The complete affected normal and race package matrices pass after
this repair; the installed and Goal boundaries above are unchanged.

`CURRENT / P2D-W2C/W2D TERMINAL ATTEMPT PAYLOAD RECONCILIATION (2026-08-13)`:
execution runtime startup now strictly replays dedicated Attempt Payload Journal
streams and repairs a Vault row left `pending` when strong
`ToolResultDelivered` authority committed immediately before a crash, including
after the exact Run became terminal. Replay validates Run, claim generation,
Runtime, Agent, frozen binding, Capsule, semantic call, sequence, content digest,
causation, Incident ID and consumption proof before mutation.

Conversation crypto-erasure makes a missing row an expected terminal result.
One unreadable or uncommittable Vault row becomes an Attempt-local blocked
outcome while another Agent's row continues to repair. Repaired and blocked
outcomes use privacy-safe `context_delivery_reconcile` diagnostics with exact
non-secret Provider Account/Attempt identity and no payload, Prompt, credential,
ciphertext or Provider response.

A real LocalKeyFile Vault + SQLite Journal crash-window test commits the delivery
fact, terminalizes the Run, closes/reopens the Vault, repairs the encrypted row,
and confirms plaintext-negative files. Complete affected normal and race tests
pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-W2D-attempt-payload-terminal-reconciliation-v1.md`.

General side-effecting multi-tool state, accepted-but-unproved terminal expiry,
TTL/compaction, parallel Aggregation, encrypted export, installed CV6 and live
mixed-Team acceptance remain open. Source remains post-build-64; installed Loom
remains v0.5.2 build 39 and was not modified. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2B/W2D FROZEN AGENT DISCLOSURE POLICY (2026-08-13)`:
Provider Account governance now remains exact through the complete Agent path.
At Run claim or Team dispatch Loom freezes policy version, revision, digest,
trust domain, retention mode, and data region from the immutable historical
account policy selected for that Attempt. Later policy updates affect only
future claims and do not rewrite an admitted or terminal Agent.

Authority replay, Run projection, Team Attempt overlay, Board wire, strict
Swift decoding, and the Agent governance row carry the same frozen values.
Legacy v1 policies display disclosure unspecified; v2 accepts only the closed
trust/retention/region tuple, and downgrade or value substitution fails closed.
The Board continues to exclude credential reference, endpoint fingerprint,
secret, Prompt, Capsule body, and Provider response.

Affected Go normal and race matrices pass across Work, Projection, API, App,
Supervisor, and daemon. The complete macOS suite passes 202 XCTest cases with
one intentional visual-export skip plus nine Swift Testing contracts. The
serialized full-repository Go suite, vet, module verification, Go formatting,
and diff checks also pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2B-W2D-agent-disclosure-policy-freeze-v1.md`.

No bundle was built, launched, or installed and no real credential or Provider
was accessed. Source remains post-build-64; installed Loom remains v0.5.2 build
39. Installed CV6 and live mixed-Team acceptance remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.

`CURRENT / P2D-W2A/W2C PARALLEL ROUTE AGGREGATION V1 (2026-08-13)`: Loom's
generic Team execution authority now models one stable Agent's multi-Provider
parallel evaluation as explicit sibling Attempts followed by a separate
Synthesis Attempt. ExecutionPlan canonicalization, Work replay, dispatch-wave
reconstruction, Projection, Board wire, and strict Swift decoding preserve the
node kind and versioned route group. Ordinary node canonical bytes remain
compatible; the nine-Agent limit counts unique stable Agents while physical
route nodes have a separate bounded ceiling.

Every sibling freezes a distinct Execution Binding and Provider Account. The
Synthesis Attempt is released only after the complete sibling dependency set
succeeds. A failed sibling leaves a healthy sibling intact and cannot silently
select another Provider. Each sibling and Synthesis Run retains independent
token/cost accounting.

Aggregation input comes only from exact digest-verified succeeded Attempt
Evidence. Loom extracts authorized output events, binds their Attempt,
Evidence and output-summary digests as authoritative source metadata, and
places model-produced content in the new Capsule only as untrusted prior model
output. Private runtime frames, scratchpad, Prompt, credential, Provider body,
and model output remain absent from Journal and Board state. Source Agent,
route group, digest, dependency, and plan-node substitution fail closed.

The Team inspector labels the physical rows as `Parallel provider route` and
`Synthesis` without exposing the opaque route-group ID to the user. Full Go,
the affected race matrix, `git diff --check`, and the complete macOS suite pass;
exact evidence is
`.loom-evidence/phase2d/P2D-W2A-W2C-parallel-route-aggregation-v1.md`.

This checkpoint closed only the generic source execution and governance slice.
Product Team Builder RouteSet authoring is closed by the later CURRENT entry
below; ordinary Conversation parallel UX and real Harness aggregation remain
open. No bundle was built, launched, or
installed; no credential or live Provider was accessed. Installed Loom remains
v0.5.2 build 39, installed CV6 and live mixed-Team acceptance remain open, and
Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

## TARGET / ACCEPTED P2D-W2A/W2C/W2D HARNESS CONTRACT AMENDMENT (2026-08-13)

The product owner accepted three DeepSeek Harness-informed increments inside the
existing Phase 2D Goal. They are design commitments, not completed source or
installed-App claims:

- `P2D-W2A`: authoritative Context Meter; immutable Fork/Segment continuity;
  visible, recoverable compaction disclosure; and durable Queue/Steer/Inject
  input while an Agent is running.
- `P2D-W2C`: an Attempt/Turn/Step/ToolCall state machine; explicit
  `parallel`/`exclusive` tool modes with conflict scopes; and durable,
  generation-bound Queue/Steer/Inject admission and consumption.
- `P2D-W2D`: one-shot approval bound to the exact ToolCall and argument digest;
  component-level sandbox requirements plus `full`/`partial`/`unavailable`
  enforcement evidence; tool-level Incident correlation; and explicit recovery
  for interrupted or side-effect-uncertain calls.

Loom will absorb the interaction and lifecycle ideas without adopting full
Prompt/reasoning logs, plaintext tool payloads, credentials in telemetry, silent
fallback, or model-authored authority. Content remains in authenticated
encrypted stores; Journal and operational diagnostics remain non-secret and
content-free.

Execution stays within the current Phase 2D hierarchy: first W2C authority
schemas and RED tests, then W2D approval/sandbox/Incident/recovery governance,
then W2A projections and conversation UX, followed by restart, race,
privacy-negative, installed-App, and mixed-Team acceptance. Current RouteSet
aggregation remains `SOURCE VERIFIED`; these new increments remain
`TARGET / ACCEPTED / NOT YET IMPLEMENTED`. Phase 2D is still the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2A/W2B/W2C PRODUCT PARALLEL ROUTESET AUTHORING V1 (2026-08-13)`:
the Agent Team Builder now authors, previews, confirms, persists, reopens, and
executes a versioned parallel RouteSet owned by one stable Agent. Each RouteSet
freezes the primary route, one or two additional exact Execution Profiles, and
an explicit Synthesis route. Provider Account, credential revision, model,
Harness, limits, and capabilities remain per route; no Team-level Provider is
introduced.

State and Projection reject route/profile/revision substitution, duplicate
routes, missing Synthesis, unknown versions, and fallback/parallel ambiguity.
Mission resolution expands one logical Agent into sibling route nodes plus the
original logical aggregation node. Saved-Team reopen and Mission reconstruction
preserve the exact route group and plan digest. The product editor exposes a
compact add/remove Parallel routes control, account/model rows, and Synthesis
label; fallback is disabled while parallel mode is active. Swift wire remains
closed-shape and validates account scope, credential revision, and topology.

The four-Agent source fixture produces six physical nodes and proves that a
revoked Kimi account blocks its Kimi sibling while the DeepSeek sibling remains
ready. Synthesis is held for the exact dependency set and no silent fallback is
performed. Full Go, affected race, full macOS (202 XCTest with one intentional
visual skip plus 10 Swift Testing contracts), and diff checks pass. Exact
evidence is
`.loom-evidence/phase2d/P2D-W2A-W2B-W2C-product-parallel-routeset-authoring-v1.md`.

Ordinary Conversation parallel UX, real Harness Synthesis output, installed
CV6, and live mixed-Provider acceptance remain open. No bundle was built,
launched, or installed and no real credential or Provider was accessed. Source
remains post-build-64; installed Loom remains v0.5.2 build 39. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C ATTEMPT LOOP AUTHORITY V1 (2026-08-13)`: the first
generalized Loom Harness Core authority slice is source verified. Audit found
that the bounded Attempt Payload fact stream could represent only one ToolCall
result per Attempt. Sequence 1 retains the installed-compatible legacy stream;
sequence 2 and later use call-scoped v2 streams and an atomically committed,
monotonic call index. Duplicate sequence ownership, gaps, call/digest
substitution, and concurrent races fail closed.

`AttemptLoopAuthority` now freezes exact Run/Attempt, Team/Conversation, Agent,
Runtime, claim generation, Execution Binding, Capsule, permission profile,
capability set, tool schema set, budget policy, and Incident lineage. It records
strict Turn/Step/ModelRequest/ToolCall/dispatch/Step-end/Turn-end facts, enforces
parallel versus exclusive conflict scopes and bounded budgets, and requires a
Tool result's accepted fact to cite the exact dispatch event before delivery or
Step completion. Prompt, arguments, result content, credentials, and Provider
responses remain outside the Journal and in authenticated encrypted stores.

Focused/full work tests, 10-run race, affected Runtime/daemon packages, vet,
full repository Go, strict replay, restart/terminal reconstruction, deep-copy,
privacy-negative, and diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-attempt-loop-authority-v1.md`.

This does not yet compose the new authority into the production daemon Tool
Gateway or expose Queue/Steer/Inject, native approval controls, sandbox state,
operational diagnostics, or Swift governance UI. Installed Loom remains
v0.5.2 build 39 and was not modified. CV6 and live mixed-Provider acceptance
remain open; Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C PRODUCT ATTEMPT LOOP RUNTIME CONTEXT V1 (2026-08-13)`:
the generalized Attempt Loop now enters the real product daemon Runtime path for
context-bearing Team Agent Attempts. The Supervisor passes exact non-secret
Claim and Incident identity only after `RunStarted`; a product Runtime decorator
then validates dispatch, Agent, Runtime, route, frozen Execution Binding, and
Role Context Capsule before invoking Pi, Codex, Claude Code, or Loom Native.

`loom_read_context` is the first production-governed ToolCall. Its proposal,
scoped `system.context-retrieval.v1` policy, exact capability and tool schema,
dispatch, encrypted result acceptance, and Provider/Harness delivery proof are
all bound into the Attempt/Turn/Step lineage before terminal closure. Prompt,
Context content, arguments, credentials, and Provider bodies remain outside the
Journal. Binding or account substitution fails before the Runtime delegate.
The unimplemented synthetic `TurnCancelled` terminal is rejected until W2D
adds explicit Step/Tool interruption and uncertain-side-effect recovery facts.

Focused/full daemon, Supervisor, Work, Native, Harness, and Pi tests, 10-run
race loops, full repository Go, vet, privacy-negative, and boundary checks pass.
Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-product-attempt-loop-runtime-context-v1.md`.

This closes only the Context-tool production Runtime vertical slice. Attempts
without a Context Capsule, the existing local execution Hook, general Web/MCP/
local Tool Gateway, native approval controls, sandbox evidence, Queue/Steer/Inject,
operational UI, installed CV6, and live mixed-Team acceptance remain open.
No bundle was built, signed, launched, or installed and no real credential or
Provider was accessed. Source remains post-build-64; installed Loom remains
v0.5.2 build 39. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D ONE-SHOT APPROVAL AND CONTENT-FREE JOURNAL V1 (2026-08-13)`:
local ToolCall approval now has an explicit consumed fact bound to the exact
approval digest, Job, canonical call digest, consumer execution, operation, and
Incident/correlation lineage. Same-consumer replay is idempotent; a second or
concurrent consumer fails closed, and race tests prove at most one executor
invocation.

New execution proposed/denied/failed and permission decision facts use strict
schema v2. They retain IDs, tool kind, call digest, controlled status/error/
recovery codes, timestamps, and result/evidence digests, but omit command,
path, free-form denial text, and raw executor errors. Existing schema-v1 facts
remain replayable. Permission attention projects the non-secret call digest.
When authenticated proposal details are absent, the TUI disables Allow and
keeps Reject available, so content-free facts cannot become blind approval.

Focused 10-run race/privacy tests, affected Journal/Rules/Execution/Permission/
Projection/App/TUI/daemon suites, full repository Go, and vet pass. Exact
evidence is
`.loom-evidence/phase2d/P2D-W2D-one-shot-approval-content-free-journal-v1.md`.

Authenticated encrypted proposal detail, broader multi-tool and multi-Runtime
Tool Gateway composition, interrupted-side-effect recovery, sandbox enforcement
evidence, tool-level diagnostics, Queue/Steer/Inject, Swift governance,
installed CV6, and live mixed-Team acceptance remain open. No bundle was built, signed,
launched, or installed and no real credential or Provider was accessed. Source
remains post-build-64; installed Loom remains v0.5.2 build 39. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D ENCRYPTED TOOL PROPOSAL APPROVAL INSPECTION V1 (2026-08-14)`:
pending local ToolCall detail now stays outside the Event Journal and is stored
under the Credential Vault's per-Conversation DEK. Its strict AAD freezes the
Conversation, WorkItem, Run, generation, Runtime, Agent, Execution Binding,
Capsule, call, approval, tool, operation, Incident, and content digest.

The existing Pi ask Hook writes the canonical call bytes after the exact
approval is created. Permission Attention reads the call digest from the
authoritative approval continuation, performs an exact Vault lookup, strictly
decodes the call, and recomputes its digest before displaying command/path.
Missing, tampered, or substituted detail shows unavailable and cannot be
allowed; Reject remains available. Swift exposes the encrypted/unavailable
state and sanitizes displayed detail.

Vault restart/plaintext-negative/substitution/tamper/rotation/delete/concurrent
tests, 10-run race, affected Go suites, and the full macOS contract suite (202
XCTest with one intentional skip plus 10 Swift Testing contracts) pass. Exact
evidence is
`.loom-evidence/phase2d/P2D-W2D-encrypted-tool-proposal-approval-inspection-v1.md`.

This is a safety vertical slice on the existing Pi Hook, not general Attempt
Tool Gateway completion. Broader multi-tool dispatch admission, ask-path Attempt
suspension/resume, sandbox evidence, tool diagnostics, interrupted-side-effect
recovery, native approve/reject controls, other Runtime/Web/MCP paths,
installed CV6, and live mixed-Team acceptance remain
open. No bundle was built, signed, launched, or installed and no real
credential, Provider, tool side effect, or user workspace was accessed. Source
remains post-build-64; installed Loom remains v0.5.2 build 39. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C/W2D PI ATTEMPT LOCAL TOOL GATEWAY V1 (2026-08-14)`:
the production Attempt Runtime now carries its exact binding, Turn and Step in
daemon-private context. For allowed Pi Bash/Edit calls, a per-proposal execution
gate commits `ToolCallAdmitted` and `ToolDispatchCommitted` before invoking the
executor. A dispatch authority failure records controlled
`dispatch_not_committed` state and leaves the executor at zero calls. An
approved resume passes the exact one-shot approval ID/digest into the same
authorization and dispatch lineage.

Successful digest-only execution metadata enters the Conversation-DEK Attempt
Payload Store before `ToolResultAccepted`. The Pi result frame now carries only
call digest, Tool and digest-only result, not the original command/path
Envelope. RunStream acceptance writes `run_stream_tool_result` and marks the
encrypted payload delivered. This receipt proves Loom RunStream acceptance; it
does not claim Provider continuation or that a model consumed the result.

Focused execution/work/daemon/Pi tests, exact dispatch-order and privacy tests,
the previously flaky Harness process-group test repeated three times, vet and
diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-attempt-local-tool-gateway-v1.md`.

Pi child-process result injection and actual next-model turn, ask pause/resume,
Read/Grep, Web/MCP, other Runtime adapters, sandbox enforcement report,
tool-level diagnostics/recovery UI, Queue/Steer/Inject, installed CV6 and live
mixed-Team acceptance remain open. No bundle was built, signed, launched, or
installed and no real credential, Provider, user workspace, or external tool
was accessed. Source remains post-build-64; installed Loom remains v0.5.2 build
39. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D TOOL DIAGNOSTICS AND RECOVERY-REQUIRED V1 (2026-08-14)`:
Pi Bash/Edit now emits a closed content-free stage chain for authorization,
sandbox preparation, binding validation, dispatch, result validation/commit,
encrypted payload commit, and result delivery. Product operational records
carry the exact Provider Account, Model, Attempt, Run, Agent, Execution Binding,
Capsule, call digest, operation, and Incident identity. They have no command,
path, Prompt, result body, credential, or Provider-response field. Edit
owned-path/content preflight now precedes dispatch, and raw executor failures
are reduced to controlled error codes.

Daemon startup reconciles `Allowed` executions without a terminal fact into one
fixed schema-v2 `recovery_required / side_effect_unknown` authority state and
never re-executes the side effect. Proposed-only asks remain pending. Recovery
without authorization, substituted code/action, duplicate recovery, and
unknown schema-v2 authority facts fail closed. The global Projection now admits
only the exact known execution/permission v2 facts, repairing restart without
opening a generic v2 envelope. Swift decodes and presents the result-unknown
state and Incident ID, but intentionally exposes no recovery button before a
real authority command exists.

Focused 10-run race tests, full repository Go, Go vet, full macOS tests (203
XCTest with one intentional visual skip plus 10 Swift Testing contracts), and
diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-tool-diagnostics-recovery-required-v1.md`.

This is `SOURCE VERIFIED / RECOVERY DECISION COMMAND OPEN`. Exact Attempt-bound
startup `tool_recovery` diagnostics, authoritative Resume/Retry/Skip/Cancel/New
Attempt commands and native controls, component sandbox reports, Pi Provider
continuation, ask suspension/resume, Read/Grep/Web/MCP and other Runtime paths,
Queue/Steer/Inject, installed CV6, and live mixed-Team acceptance remain open.
No bundle was built, signed, launched, or installed and no real credential,
Provider, user workspace, or external tool was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-COMP1 AND P2D-COMP2-A-B SOURCE VERIFIED (2026-08-14)`: Phase 2D
includes
`P2D-COMP1` and `P2D-COMP2` as front-loaded Harness Platform composition
slices, not as a new Goal. ADR-0021 freezes a Loom-owned typed Composition
Kernel with desktop/headless/test Launch Profiles, versioned built-in Bundles,
the `Root -> Product -> Conversation -> Team -> Agent -> Attempt -> Turn`
Capability Context hierarchy, reversible Effects, deterministic
CompositionSnapshot digest, closed RouteDescriptors, and the
`Register -> Validate -> Start -> Ready -> Stop -> Dispose` lifecycle.

Capability Context is explicitly distinct from Go `context.Context`, frozen
Attempt Context, and model-visible Context Capsule. It cannot contain Prompt,
transcript, tool-result content, Provider body, plaintext credential, Vault
VMK, Journal writer, policy authority, or terminal authority. Child scopes may
narrow capabilities but cannot expand or replace protected Core authority.
Active Attempts retain both their admitted snapshot digest and independent
Frozen Execution Binding.

P2D-COMP2 uses a compatibility-first strangler migration. It first wraps the
existing product services as built-in Bundles without behavior change, then
replaces `localProductHandlerWithComposition` and its fifteen service
dependencies with a compiled typed route/capability registry, and finally moves
construction out of `newProductDaemonRunnerWithPreparedDecisions` into
`loom-core`, `loom-vault`, `loom-conversation`, `loom-agent-runtime`,
`loom-governance`, `loom-work`, `loom-assets`, `loom-observability`, and
`loom-local-ipc` Bundles.

P2D-COMP1 is now source verified in `internal/composition`. It provides closed
desktop/headless/test Profile manifests, versioned built-in Bundle admission,
deterministic graph/route compilation and snapshot digest, descriptor-narrowed
Bundle lifecycle views, protected-Core masking below Root, exact operational
scopes, reverse Effect cleanup, atomic Ready, safe diagnostics, and independent
Attempt snapshot/execution-binding freeze. Focused coverage is 83.0%; 20
race-enabled runs, full repository Go, and vet checks pass. The product daemon
now consumes the kernel through COMP2-A/B. Exact evidence is
`.loom-evidence/phase2d/P2D-COMP1-governed-composition-kernel-v1.md`.

The source-verified ATL3 remote-result commit remains preserved. COMP2-A and
COMP2-B are now source verified. Before broadening to sequential multi-tool
calls, additional Runtime adapters, or Queue/Steer/Inject, the active execution
gate is bounded COMP2-C/D service construction and scope migration.
COMP2-E removes legacy production reachability only
after parity, race, restart, privacy, and shutdown gates pass. Existing
Credential Vault CV6, Conversation, per-Agent binding, Tool Loop, governance UI,
and installed mixed-Team gates remain unchanged. Contracts:
`.loom-evidence/phase2d/contracts/P2D-COMP1-composition-kernel.md` and
`.loom-evidence/phase2d/contracts/P2D-COMP2-product-daemon-strangler.md`.

`CURRENT / P2D-COMP2-A SOURCE VERIFIED (2026-08-14)`: the real production
builder now activates one desktop compatibility snapshot before creating the
local IPC server. Desktop/headless/test compile the same nine versioned built-in
Bundle facades. The admitted handler delegates to
the unchanged existing handler, gates calls on Ready, joins in-flight requests
on Close, and revokes old references before legacy resources close.

Composition lifecycle/scope events persist in the existing owner-only bounded
operational JSONL store before IPC admission; write failure prevents startup.
Full repository Go/vet and focused 10-run race checks pass. Existing state paths,
wire responses, Vault, Conversation, per-Agent binding, Tool Loop, startup
cleanup, and shutdown tests remain green. Legacy service constructors remain
the compatibility oracle; COMP2-B
method-level declarative routes are now active. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-A-compatibility-bundle-facade-v1.md`.

`CURRENT / P2D-COMP2-B SOURCE VERIFIED (2026-08-14)`: all 47 local product
methods are now exact, sorted RouteDescriptors in the desktop/headless/test
Composition Snapshot; the aggregate `legacy.product.dispatch` route is gone.
Each descriptor freezes its built-in Bundle owner, typed handler capability,
required capability set, unavailable error, Incident policy, and privacy class.

Product startup directly constructs `productRouteServices` and the typed route
registry. The fifteen-parameter handler survives only as a test compatibility
wrapper and an AST gate prevents production from calling it. Exact unavailable
codes, Journey IDs, Conversation and credential stages, degraded-write behavior,
and existing dispatch results remain green across focused race, daemon-package,
full-repository, vet, and diff checks. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-B-declarative-route-registry-v1.md`.

This is `SOURCE VERIFIED / COMP2-C-D OPEN`. Legacy service construction and the
large dispatch switch remain temporary behavior oracles. The next gate is
bounded Bundle construction extraction and scoped lifecycle ownership; COMP2-E,
installed CV6, mixed-Team ATL9, and all existing Phase 2D live gates remain
open. No App was built, signed, installed, or launched, and no credential,
Provider, network, user workspace, or external tool was accessed.

`CURRENT / P2D-COMP2-D PRODUCT SCOPE SOURCE VERIFIED (2026-08-14)`: the
production compatibility composition now opens the exact `loom-product` scope
after all Bundles are Ready and before local IPC admission. It freezes the
Composition Snapshot digest, carries no Execution Binding digest, masks
protected Core capabilities, and persists metadata-only open/close diagnostics.
Facade shutdown first quiesces requests, then closes Product scope before the
activation/root; concurrent Close remains idempotent.

The real production-builder test proves the Product scope and durable
`scope_open` record exist before Run. Focused 20-run race, daemon package, full
repository, vet, and diff gates pass. Conversation, Team, Agent, Attempt, and
Turn scope integration remains open. Agent Runtime construction has now moved
behind Composition, while Conversation and remaining services still predate
activation; hidden mutable scope injection remains forbidden. Further bounded
COMP2-C inversion precedes attaching deeper scopes to lifecycle boundaries.
Evidence: `.loom-evidence/phase2d/P2D-COMP2-D-product-scope-v1.md`.

`CURRENT / P2D-COMP2-C ASSETS ROUTE CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: `loom-assets` now constructs the local Assets service/API in its
Bundle Start hook and binds a one-time typed route slot. Product startup no
longer calls those constructors directly. Bundle Ready checks the binding;
failed construction prevents Product scope and IPC admission. The Start Effect
waits for in-flight Assets calls and revokes the route during composition
cleanup, so retained slot references fail closed.

Focused 20-run race, daemon package, full repository, vet, AST boundary, socket
journey, mission execution, startup cleanup, shutdown, and diff gates pass.
Agent Runtime construction subsequently moved behind `loom-agent-runtime`, and
Assets authority/Evidence ownership has now moved into `loom-assets` through a
bounded inter-Bundle materializer port. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-assets-route-construction-v1.md`.

`CURRENT / P2D-COMP2-C AGENT RUNTIME CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: `loom-agent-runtime` Start now constructs the existing Mission
Execution backend, Side Task Handoff API, and saved-Team materializer behind a
one-bind typed route slot. Product startup no longer calls the execution builder
or independently owns its closer. The Start Effect revokes routes and closes
backend, observers, handoff, and Evidence ownership after request quiescence.

Assets starts before Agent Runtime. Runtime construction failure rolls Assets
back and prevents Product scope/IPC admission. The migration also repaired
Composition lifecycle error fidelity: safe Error text remains metadata-only,
while `errors.Is` now retains both the composition category and original cause,
restoring exact startup reconciliation conflicts without leaking private text.

Focused COMP1/COMP2 20-run race plus post-review 10-run race, daemon package,
full repository, vet, startup reconciliation, mission/handoff, credential
binding, shutdown, AST boundary, and diff gates pass. Assets authority/Evidence
ownership has subsequently moved under `loom-assets`. Remaining COMP2-C work
includes observability, Vault, Conversation, governance/work, local IPC, and
protected core construction.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-agent-runtime-construction-v1.md`.

`CURRENT / P2D-COMP2-C ASSETS AUTHORITY OWNERSHIP SOURCE VERIFIED
(2026-08-14)`: `loom-assets` Start now constructs and owns the Assets Evidence
store, Asset authority, Assets API route, Pi skill materializer, and Team asset
materializer, including startup recovery. Product construction no longer keeps
`assetAuthority` or `assetEvidenceStore` owners. `loom-agent-runtime` receives
only the bounded `app.TeamAssetMaterializer` port after Assets is Ready; no
concrete authority, Evidence store, content, or protected capability enters
Capability Context.

Reverse cleanup closes Agent Runtime before Assets, revokes both bounded ports,
waits for in-flight calls, and closes the Assets store. Focused 10-run race,
daemon package, full repository, vet, ownership AST, rollback, shutdown, and
diff gates pass. COMP2-C/D, COMP2-E, CV6, ATL9, and live gates remain open.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-assets-authority-ownership-v1.md`.

`CURRENT / P2D-COMP2-C WORK QUEUE CONSTRUCTION SOURCE VERIFIED
(2026-08-14)`: `loom-work` Start now constructs and binds the existing Queue
service/API through an extensible one-bind `productWorkRoutes` slot. Production
startup no longer constructs Queue directly. Ready gates admission; failed
construction prevents Product scope/IPC; reverse cleanup waits for in-flight
Queue calls and revokes the port. Compatibility helpers preserve exact typed
nil/unavailable behavior.

Focused tests freeze Assets -> Work -> Agent Runtime startup and reverse
cleanup. Focused 10-run race, daemon package, full repository, vet, AST boundary,
Queue journey, startup/shutdown, and diff gates pass. Workers, Integration,
Execution, Production, governance/work authority, and deeper scopes remain.

`loom-observability` has a verified bootstrap dependency: Conversation
migration, Agent diagnostics, handler wrapping, and Composition lifecycle
recording all need the operational store before Bundle activation. Its bounded
solution is a minimal pre-composition recorder retained until Conversation
construction migrates, followed by explicit operational-port handoff; an
unbound proxy must not discard startup failures. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-work-queue-construction-v1.md`.

`CURRENT / P2D-COMP2-C WORK ROUTES CONSTRUCTION V2 SOURCE VERIFIED
(2026-08-14)`: `loom-work` now atomically constructs and binds Queue, Workers,
and Integration during Bundle Start. Product construction no longer creates the
Queue API, Worker execution/service API, Integration service, domain
observability service, or local Integration API. All three typed ports must be
valid before Ready; one Effect quiesces and revokes all six route methods.

Construction failures preserve exact `build_queue`, `build_workers`, and
`build_integration` identity through Composition. Focused 10-run race, full
daemon/repository, vet, AST ownership, build-stage classification, and real
socket Queue/Workers/Integration journeys pass. Execution, Production,
governance/work authority, setup/read routes, and deeper scopes remain open.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-work-routes-construction-v2.md`.

`CURRENT / P2D-COMP2-C WORK EXECUTION OWNERSHIP V3 SOURCE VERIFIED
(2026-08-14)`: `loom-work` now also constructs and owns governed Execution,
including its owner-only Evidence store, decision recorder, sandbox gate,
Adapter, pending recovery, service, and API. Recovery completes before Ready.
The typed Work slot publishes the local Execution route and one private,
bounded tool-execution port; `loom-agent-runtime` consumes only that port after
Work is Ready, never a concrete Adapter through Capability Context.

Composition preserves exact `build_execution` and `build_execution_recovery`
failure stages. Reverse cleanup closes Agent Runtime before Work, waits for
in-flight calls, revokes both Execution ports, and closes the Evidence owner
exactly once, removing the successful-start ownership leak from the product
root. Focused route/runtime tests, 10-run composition/daemon race, full daemon
and repository Go, vet, AST ownership, privacy, shutdown, and diff checks pass.
Production, governance/work authority, setup/read routes, Conversation and
observability construction, deeper scopes, COMP2-E, CV6, ATL9, and live gates
remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-work-execution-ownership-v3.md`.

`CURRENT / P2D-COMP2-C WORK PRODUCTION OWNERSHIP V4 SOURCE VERIFIED
(2026-08-14)`: `loom-work` now also constructs and owns the Production core,
local service, and API. The typed Work route set requires Production Snapshot,
Command, and the read-only Degraded gate before Ready. Product construction no
longer resolves Production paths or calls its three constructors.

The existing sandbox-root/user-Library path semantics, current executable,
Journal-replayed AdminLock, authoritative projection, and degraded-write policy
are preserved. Construction failures remain `build_production`; shutdown
quiesces and revokes Production with the other Work routes and reports degraded
after revocation. Focused Composition and real socket Production journeys,
10-run race, full daemon/repository Go, vet, AST ownership, shutdown, privacy,
and diff checks pass. Governance/work authority, setup/read routes,
Conversation/observability construction, deeper scopes, COMP2-E, CV6, ATL9,
and live gates remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-work-production-ownership-v4.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE AUTHORITY OWNERSHIP V5 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now constructs and owns the Permission approval
port and Rules authority, Permission API, Customer Rule API, and Standing Order
authority/API. Product construction no longer creates these objects. The Rules
authority remains private to the trusted built-in factory; IPC receives only
three typed route proxies, while Work receives only Request/Consume approval
methods after Governance Ready.

Governance failure prevents Work, Product scope, and IPC admission. Reverse
cleanup closes Work before Governance and revokes both route and approval ports.
Exact `build_permissions`, `build_customer_rule`, and `build_standing_order`
stages, Permission ask/decision, Customer Rule, Standing Order, Execution
approval consumption, focused 10-run race, full daemon/repository Go, vet, AST
ownership, shutdown, privacy, and diff checks pass. Mission decision,
Provider-account policy, setup/read routes, Conversation/observability
construction, deeper scopes, COMP2-E, CV6, ATL9, and live gates remain open.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-governance-authority-ownership-v5.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE DECISION OWNERSHIP V6 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now also constructs and owns the Prepared
Mission Decision backend, fallback decision StateWriter/preparer, local Decision
API, and Mission Execution decision router. The production builder no longer
creates or retains these concrete objects. Read receives only prepared-command
listing; Agent Runtime receives only execution-control and explicit fallback
interfaces; IPC receives only the Decision route.

The backend and StateWriter remain private to the trusted factory. Governance
failure prevents downstream activation, and closing the slot revokes every
decision interface. Exact `build_decision`, strict Decision IPC, prepared-view
rebind, mission preflight/control, fail-closed registry, four-Provider explicit
fallback, focused 10-run race, full daemon/repository Go, vet, AST ownership,
shutdown, privacy, and diff checks pass. Provider policy, setup/read route
construction, Conversation/observability construction, deeper scopes, COMP2-E,
CV6, ATL9, and live gates remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-governance-decision-ownership-v6.md`.

`CURRENT / P2D-COMP2-C GOVERNANCE PROVIDER ACCOUNT POLICY V7 SOURCE VERIFIED
(2026-08-14)`: `loom-governance` now also constructs and owns Provider Account
Policy and Provider Model Rate Card authority. Setup no longer constructs that
authority and receives only two exact configuration interfaces. The authority
stays inside the trusted factory; it is not placed in Capability Context and
cannot expose Journal, credential, Provider client, or terminal authority.

Account identity, revision, policy/rate-card digest, concurrency, dispatch rate,
budget, disclosure policy, token basis, and cost freezing remain exact and
account-local. Closing Governance revokes both ports; construction preserves
`build_setup_policy`. Trusted-correlation wire, projection refresh,
concurrency/budget isolation, policy/price drift, privacy-safe diagnostics,
focused 10-run race, full daemon/repository Go, vet, AST ownership, shutdown,
and diff checks pass. Setup/read route construction,
Conversation/observability construction, accounting UI, deeper scopes, COMP2-E,
CV6, ATL9, and live gates remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-governance-provider-account-policy-v7.md`.

`CURRENT / P2D-COMP2-C OBSERVABILITY BOOTSTRAP HANDOFF V8 SOURCE VERIFIED
(2026-08-14)`: the owner-only operational diagnostic store keeps only its
minimal pre-composition bootstrap role for credential-runtime selection,
encrypted Conversation migration, and Composition lifecycle failures.
`loom-observability` Start now hands bounded operational ports to a revocable
slot after which Read diagnostics, Agent Attempt diagnostics, Context retrieval
audit, governed tool diagnostics, and IPC operation recording no longer retain
the bootstrap store directly.

The slot exposes no path, file handle, Journal writer, credential, Prompt,
transcript, Provider body, or terminal authority and is not placed in
Capability Context. Start failure prevents Product/IPC admission and preserves
`build_diagnostics`; Composition close revokes all downstream ports. Focused
handoff/AST/UDS tests, ten-run race, full daemon/repository Go, vet, privacy,
shutdown, and diff checks pass. V10/V11 below subsequently move
Vault/Conversation and persistent store construction. Setup/read construction,
diagnostics UI/export, accounting projections, deeper scopes, COMP2-E, CV6,
ATL9, and live gates remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-observability-bootstrap-handoff-v8.md`.

`CURRENT / P2D-COMP2-C CONVERSATION ROUTE HANDOFF V9 SOURCE VERIFIED
(2026-08-14)`: `LocalProductReadService` now consumes the narrow
`LocalProductChatSource` port rather than concrete `LocalProductChatAPI`.
`loom-conversation` Start binds the existing persistent/encrypted chat API into
a revocable route slot; Composition close removes both thread reads and message
dispatch. Conversation failure prevents Agent Runtime and Product/IPC admission
and preserves `build_setup_provider`.

Profile, Segment, Attempt, Context Capsule, encrypted migration, Provider
Account binding, and native Provider state behavior are unchanged. The slot is
not Capability Context and exposes no content, credential, Provider response,
Journal writer, or terminal authority. Focused chat/AST/UDS tests, encrypted
migration tests, ten-run race, full daemon/repository Go, vet, shutdown, privacy,
and diff checks pass. This V9 construction boundary was subsequently advanced
by V10 below. Setup/read construction, deeper scopes, UI/accounting, COMP2-E,
CV6, ATL9, and live gates remain open. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-conversation-route-handoff-v9.md`.

`CURRENT / P2D-COMP2-C VAULT AND CONVERSATION CONSTRUCTION V10 SOURCE VERIFIED
(2026-08-14)`: `loom-vault` Start now constructs and privately owns the real
LocalKeyFile Vault or existing fail-closed recovery runtime. Only bounded
credential mutation/lease/availability, encrypted document, Context Capsule,
retrieval, Attempt payload, Tool Proposal, and external-session ports reach
trusted built-in consumers. The Vault root, VMK, Journal writer, and replacement
authority do not leave the Bundle.

`loom-conversation` Start now constructs Codex native and system Provider
clients, the account-aware Profile router, migration recorder, and persistent
encrypted Chat API after Vault Ready. Failure blocks Agent Runtime and
Product/IPC admission while preserving `build_setup_native_auth`,
`build_setup_provider`, and `build_setup_credential`. Reverse cleanup revokes
Conversation before closing Vault; recovery reset keeps the same bounded slot.
The monolithic product builder no longer calls any Vault/recovery, system
Provider, Profile router, or persistent Chat constructor. Unavailable Vault
writes clear Context Capsule, transcript, and Attempt payload buffers.

Focused ownership, lifecycle, recovery, migration, stage, and privacy tests;
ten-run race; full daemon/repository Go; vet; static constructor inspection;
and diff checks pass. The lazy Pi local-model Conversation resource, Setup/read
construction, deeper scopes, UI/accounting, COMP2-E, CV6, ATL9, and live gates
remain open. No App, network,
Provider, credential, or installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-vault-conversation-construction-v10.md`.

`CURRENT / P2D-COMP2-C OBSERVABILITY CONSTRUCTION V11 SOURCE VERIFIED
(2026-08-14)`: `loom-observability` Start now creates the owner-only persistent
diagnostic directory/store. The product builder retains only a no-I/O, bounded,
privacy-safe bootstrap sink so Composition compile/validate/register and early
start events generated before Observability Ready are not lost. Bundle Start
binds the credential-runtime identity and flushes those records in order before
publishing operational ports; overflow, store construction, or flush failure
fails activation as `build_diagnostics`.

After binding, lifecycle records continue to persist through stop/dispose while
Read, Agent Attempt, Context retrieval, Tool, and IPC access remains revocable
through the existing slot. The bootstrap carries no content, credential,
ciphertext, VMK, Journal writer, or authority and is not Capability Context.
Focused pre-start flush, filesystem failure, route revocation, and AST tests;
ten-run race; full daemon/repository Go; vet; static constructor inspection;
and diff checks pass. Lazy Pi Conversation ownership, Setup/read construction,
deeper scopes, UI/accounting, COMP2-E, CV6, ATL9, and live gates remain open. No
App, network, Provider, credential, or installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-observability-construction-v11.md`.

`CURRENT / P2D-COMP2-C SETUP CONSTRUCTION V12 SOURCE VERIFIED
(2026-08-14)`: the final built-in `loom-local-ipc` Start now constructs and
binds the existing Setup aggregate only after Observability, Assets, Vault,
Conversation, Governance, Work, and Agent Runtime are Ready. The product
handler consumes a revocable Setup slot, and the monolithic builder no longer
calls `buildProductSetupService`.

Setup continues to consume the exact bounded Vault mutation/status/availability
and Governance Provider Account Policy/Model Rate Card ports. Account identity,
credential revision, native auth, runtime admission, transactions, and
projection behavior are unchanged. Exact `build_state`, `build_setup_runtime`,
`build_setup_credential`, `build_setup_provider`, and
`build_setup_native_auth` failures survive Composition rollback. Close revokes
Setup before dependency Bundles without a runner double-close; unavailable or
nil routes clear credential input bytes. No Vault root, Provider authority,
content, VMK, or terminal authority enters Capability Context.

Focused Setup/Vault/policy/UDS/ownership tests, ten-run race, full
daemon/repository Go, vet, AST/static constructor inspection, and diff checks
pass. Read construction, lazy Pi Conversation ownership, deeper scopes,
UI/accounting, COMP2-E, CV6, ATL9, and live gates remain open. No App, network,
Provider, credential, or installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-setup-construction-v12.md`.

`CURRENT / P2D-COMP2-C READ CONSTRUCTION V13 SOURCE VERIFIED
(2026-08-14)`: the existing Read aggregate is now constructed atomically with
`loom-governance` after Governance binds and after Observability, Conversation,
and Vault are Ready. Failure closes Governance in the same Start rollback and
maps to `build_state`. The product builder no longer calls
`api.NewLocalProductReadService`.

IPC consumes only snapshot/timeline/chat from a revocable Read slot. Agent
Runtime consumes only mission observer, SideTask source injection, and observer
close ports; it no longer receives a concrete Read service. The concrete
Journal, Projection, cached views, stream subscriptions, tentative records, and
observer map remain private and do not enter Capability Context. Reverse
cleanup closes Agent Runtime, then revokes Read and residual observers before
Governance.

Focused Governance/Read/Agent Runtime/UDS/AST tests, ten-run race, full
daemon/repository Go, vet, static constructor inspection, and diff checks pass.
Lazy Pi Conversation ownership, deeper scopes, UI/accounting, COMP2-E, CV6,
ATL9, and live gates remain open. No App, network, Provider, credential, or
installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-read-construction-v13.md`.

`CURRENT / P2D-COMP2-C SHARED LOCAL MODEL OWNERSHIP V14 SOURCE VERIFIED
(2026-08-14)`: `loom-conversation` now constructs and owns the optional shared
Pi local-model runtime. Pi Conversation and Agent Runtime use the same bounded
runtime; Agent Runtime acquires it through the existing Conversation dependency.
Reverse cleanup closes Agent adapters before Conversation closes the server
exactly once, and construction rollback closes it as well. The product builder
no longer constructs the shared model, Pi Conversation responder, or retains a
separate closer.

The process owner, paths, endpoint, and server mutex remain private and do not
enter Capability Context. Focused shared identity/lazy-load/rollback/AST tests,
ten-run race, full daemon/repository Go, vet, static constructor inspection,
and diff checks pass. The required constructor ownership audit is completed by
V15 below. Deeper scopes, UI/accounting, COMP2-E, CV6, ATL9,
and live gates remain open. No App, network, Provider, credential, or
installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-shared-local-model-ownership-v14.md`.

`CURRENT / P2D-COMP2-C PROTECTED CORE OWNERSHIP V15 SOURCE VERIFIED
(2026-08-14)`: `loom-core` now constructs and owns the product SQLite
connection, Journal, Projection, replay, controlled fixture bootstrap, and
verified built-in Runtime records. Other trusted built-in Bundle factories
resolve these resources only during Start through a private revocable
construction slot; it is not Capability Context and Core closes it after all
dependent Effects. Later Bundle failure rolls back and closes the database.

The explicit legacy/test SecretStore adapter now also constructs in
`loom-vault` behind a revocable lease slot. The production builder retains only
bounded launch/test configuration, executable validation, no-I/O diagnostics
bootstrap, factory/route assembly, Composition activation, IPC server creation,
and lifecycle handoff. A constructor allowlist plus explicit AST exclusions
prevents domain construction from returning. Focused RED/rollback/error-stage,
legacy credential, route parity, ten-run race, full daemon/repository Go, vet,
and diff checks pass. COMP2-C's source construction exit is complete; deeper
COMP2-D scopes, COMP2-E, UI/accounting, CV6, ATL9, and installed live gates
remain open. No App, network, Provider, credential, or installed-live action
occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-C-protected-core-ownership-v15.md`.

`CURRENT / P2D-COMP2-D CONVERSATION ATTEMPT SCOPE V2 SOURCE VERIFIED
(2026-08-14)`: ordinary Conversation dispatch now opens the real
Product-owned Conversation -> Team -> stable Loom Agent -> Attempt -> Turn
scope hierarchy before Provider dispatch. Attempt freezes the active
Composition Snapshot digest and immutable Conversation execution binding
digest. Scope-open failure rolls back the message, Segment, Attempt, and stored
Capsule before any Provider call; every return path closes Attempt/Turn, while
higher scopes are reused until Product close.

The Chat service receives only a one-time-bound revocable manager port. Raw
thread IDs do not enter Composition diagnostics; each scope uses a
domain-separated SHA-256 opaque ID. Focused RED/rollback/reuse/production wiring
tests, Conversation parity, ten-run race, full daemon/repository Go, vet, and
diff checks pass. COMP2-D remains partial: Team mission identities and full
Frozen Execution Binding, multi-turn Tool Loop scopes, and credential/session/
tool/temporary resource Effects remain open, as do COMP2-E, CV6, ATL9, UI,
accounting, and installed live gates. No App, network, Provider, credential, or
installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-conversation-attempt-scope-v2.md`.

`CURRENT / P2D-COMP2-D TEAM AGENT ATTEMPT SCOPE V3 SOURCE VERIFIED
(2026-08-14)`: the real Team mission runner now opens an opaque internal
execution Conversation and Team scope before executor construction. Every
source and verifier call is wrapped so the exact Agent Run first freezes its
full `FrozenExecutionBinding`, then opens a stable Agent, Attempt, and Turn
carrying the active Composition Snapshot and binding digests. Different Agents
in one Team therefore retain independent immutable execution bindings and
scope admission failure stops before any Runtime or Provider delegate call.

Attempt/Turn close on every return; the underlying executor closes before the
Team execution lease revokes its Agent children. Production injects the same
one-time-bound scope slot into Agent Runtime, while compatibility tests without
Composition keep existing behavior. Scope diagnostics use domain-separated
opaque IDs and expose no raw Team, Agent, Run, or correlation identity.
Focused RED/isolation/fail-closed/production wiring tests, Conversation and Team
parity, ten-run race, full daemon/repository Go, vet, and diff checks pass.

COMP2-D remains partial: exact credential lease, Provider session, Tool Loop
channel, component process, temporary-root, and cancellable-worker Effects are
not yet scope-owned. Multi-turn Turn generation, COMP2-E, CV6, ATL9, UI,
accounting, and installed live gates remain open. No App, network, Provider,
credential, user workspace, or installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-team-agent-attempt-scope-v3.md`.

`CURRENT / P2D-COMP2-D ATTEMPT CANCELLATION OWNERSHIP V4 SOURCE VERIFIED
(2026-08-14)`: every real Team Agent Attempt now owns a cancellable Go
execution context as a Composition Effect. The mission wrapper receives that
context only after full binding freeze and scope admission, then passes it to
the delegate instead of the wider Team runner context. Normal return cancels it
during Attempt close; closing Team/Product while a delegate is active cancels
the running operation with `composition.ErrScopeClosed` before wider cleanup.

The Effect is idempotent and remains Attempt-local. Capability Context, Go
context, Attempt Context, and Context Capsule remain separate, and no content,
secret, authority, or Journal writer enters the context. RED demonstrated the
previous missing cancellation; normal-return and active-revocation tests,
ten-run race, full daemon/repository Go, vet, and diff checks pass.

COMP2-D remains partial. Credential leases, Provider sessions, Tool Loop
channels, component processes, temporary roots, multi-turn Turn ownership,
COMP2-E, CV6, ATL9, UI/accounting, and installed live gates remain open. No App,
network, Provider, credential, user workspace, or installed-live action
occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-attempt-cancellation-ownership-v4.md`.

`CURRENT / P2D-COMP2-D SEQUENTIAL TOOL TURN SCOPE V5 SOURCE VERIFIED
(2026-08-14)`: each real mission Agent Attempt now owns its current Turn and a
private controller bound into the Attempt execution context. The governed
WBridge path advances Turn N to N+1 only after a tool result is resolved and
required encrypted Attempt payload persistence succeeds. Ask polling stays in
the same Turn; duplicate, stale, cancelled, or out-of-order sequences fail
closed without corrupting the next valid transition.

Each new Turn remains under the same immutable Attempt and therefore inherits
the active Composition Snapshot and full per-Agent Frozen Execution Binding
digests. IDs remain domain-separated and opaque. The controller carries no
Prompt, result content, credential, Provider body, Journal writer, or execution
authority. RED, Turn 1→2→3, ask, duplicate, real WBridge allow, mission wiring,
tool protocol, ten-run race, full daemon/repository Go, vet, and diff checks
pass.

COMP2-D remains partial, and this is not the full ATL sequential ToolCall exit.
Tool channel/process Effects, credentials, Provider sessions, temporary roots,
other Runtime adapters, COMP2-E, CV6, ATL9, UI/accounting, and installed live
gates remain open. No App, network, Provider, credential, user workspace, or
installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-sequential-tool-turn-scope-v5.md`.

`CURRENT / P2D-COMP2-D CREDENTIAL LEASE ISOLATION V6 SOURCE VERIFIED
(2026-08-14)`: the existing Loom Vault lease hot path is now verified through
the real per-Agent Attempt scopes. A mixed Team test freezes independent
DeepSeek and MiniMax bindings, resolves each exact Provider Account,
credential reference, and revision, and holds both bounded plaintext callbacks
concurrently. Revoking DeepSeek terminates and zeroizes only that lease;
MiniMax remains active until Team Scope close, which then cancels and zeroizes
the peer lease.

This confirms the Attempt-owned execution context is the parent of the actual
CredentialLeaseManager lease and that account revoke does not become Team-wide
offline/unavailable. Scope cancellation and credential revocation remain
distinct causes. Secrets never enter Capability Context, context values,
Capsule, Journal, Evidence, diagnostics, argv, or environment.

Focused exact-resolution/isolation/zeroization tests, Vault and COMP2-D parity,
ten-run race, full daemon/repository Go, vet, and diff checks pass. COMP2-D
remains partial: Provider native sessions, tool channels/extensions, component
processes, temporary roots, other Runtime adapters, COMP2-E, CV6, ATL9,
UI/accounting, and installed live gates remain open. No App, network, real
Provider, credential, user workspace, or installed-live action occurred.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-credential-lease-isolation-v6.md`.

`CURRENT / P2D-W2D PI NATIVE TOOL CONTINUATION V1 (2026-08-14)`: the Pi
Runtime now returns governed Bash/Edit results to the actual child as a native
Pi `toolResult` and accepts the second model turn before closing the Attempt
delivery. A per-Attempt owner-only extension and UDS attest the child PID and
bind the exact Conversation, Run, Agent, claim, frozen Execution Binding,
Capsule and Incident. `ask` suspends the same request and rechecks the same
deterministic operation; approval resumes that call, rejection returns a
content-free deny, and cancellation performs no execution.

Pi can now load `loom_read_context` and `loom_tool` together. The strict
two-turn protocol locks the first selected route and rejects cross-route,
result-digest, identity and lifecycle substitution. Only a validated final
assistant output writes `harness_final_output`; the earlier
`run_stream_tool_result` proof remains distinct. The Pi wire result strips
command/path, delivery binding, approval ID/digest, Prompt and Provider body.
The product profile freezes the new `governed_tool_loop` capability only for
the exact conformance-bound Pi 0.82.1 component, and capability/Hook mismatch
fails before process start.

Full repository Go, focused 10-run Pi and daemon race suites, vet, privacy and
cleanup regressions, and diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-pi-native-tool-continuation-v1.md`.

This is `SOURCE VERIFIED / INSTALLED LIVE OPEN`. Read/Grep execution, multiple
sequential ToolCalls, Web/MCP and other Runtime adapters, component sandbox
reports, authoritative recovery commands, Queue/Steer/Inject, Swift governance
controls, installed CV6 and live mixed-Team acceptance remain open. No bundle
was built, signed, launched or installed and no real credential, Provider,
user workspace or external tool was accessed. Source remains post-build-64;
installed Loom remains v0.5.2 build 39. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2C/W2D CRASH-SAFE WEB/MCP RESULT COMMIT V1 (2026-08-14)`:
the execution authority now supports an explicitly injected Loom-owned remote
Tool Broker. WebSearch, WebFetch and MCPTool calls are strictly validated before
dispatch, and `ToolDispatchCommitted` still precedes the external call. The
bounded UTF-8 result is encrypted into the exact Conversation/Attempt Payload
and accepted by the Attempt Loop before `ToolExecutionCompleted`. A payload
commit failure returns `result_persistence_failed`, zeroizes the buffer and
cannot replay the remote call merely to reconstruct content.

The Broker publishes an exact remote tool set. The execution Adapter, daemon
Hook, Pi extension schema, dynamic system prompt and strict transcript parser
derive from that same frozen set. Undeclared or unknown tools fail before
dispatch; an empty MCP allowlist does not advertise MCPTool. Pi returns the
accepted content through its private native `toolResult`, rejects content or
digest substitution, keeps query/arguments/result outside Bridge frames and
content-free authority stores, and records delivery only after validated final
model output.

Focused tests, full repository Go, target-package race, Go vet, privacy-negative
and diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2C-W2D-crash-safe-web-mcp-result-commit-v1.md`.

This is `SOURCE VERIFIED / PRODUCTION BROKER CONFIG AND INSTALLED LIVE OPEN`.
The product daemon accepts an injected Broker but does not yet compose or
advertise a production Search backend or MCP registry; no real network/MCP call
was made. Multiple sequential ToolCalls, other Runtime adapters, component
sandbox reports, recovery commands, Queue/Steer/Inject, Swift governance,
installed CV6 and mixed-Team live acceptance remain open. No bundle was built,
signed, launched or installed and no credential, Provider, user workspace or
external service was accessed. Source remains post-build-64; installed Loom
remains v0.5.2 build 39. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-W2D PI GOVERNED READ/GREP CONTENT V1 (2026-08-14)`: the local
execution authority now implements descriptor-relative, no-symlink Read and
Grep over bounded regular UTF-8 files. It validates the relative path and Grep
pattern before `ToolDispatchCommitted`, bounds files, matches and returned
text, rejects identity/content drift, and zeroizes owned result buffers.

Read/Grep content never enters the Event Journal, Evidence metadata,
operational diagnostics, ordinary RunStream frames, or the Adapter cache. The
Hook commits the exact content to the Conversation-DEK Attempt Payload as the
closed `text/plain; charset=utf-8` type, then records `ToolResultAccepted`.
The private Pi extension decrypts only the exact pending binding, recomputes
the digest and returns the content through Pi's native `toolResult`; the strict
two-turn protocol rejects content substitution and Bridge frames remain
content/path free. Only `harness_final_output` marks delivery. Read-only replay
re-reads the target and requires the committed output digest; changed content
fails closed instead of returning stale plaintext. Result-commit failures
zeroize the executor buffer.

Focused Read/Grep product, execution, permission, Attempt authority and Pi
protocol tests, four 10-run race suites, full repository Go, Go vet, privacy
negative checks and diff checks pass. Exact evidence is
`.loom-evidence/phase2d/P2D-W2D-pi-governed-read-grep-content-v1.md`.

This is `SOURCE VERIFIED / INSTALLED LIVE OPEN`. An actual managed Pi child
Read canary now proves private content delivery and second-turn continuation;
multiple sequential ToolCalls, Web/MCP and other Runtime adapters, component
sandbox reports, authoritative recovery
commands, Queue/Steer/Inject, Swift governance controls, installed CV6 and live
mixed-Team acceptance remain open. Web/MCP stays unadvertised until remote
result bytes are durably encrypted before terminal execution state, so crash
recovery cannot repeat a remote call merely to reconstruct content. No bundle
was built, signed, launched or installed and no real credential, Provider,
user workspace, network or external tool was accessed. Source remains
post-build-64; installed Loom remains v0.5.2 build 39. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.

`CURRENT / P2D-COMP2-D CONVERSATION PROVIDER SESSION LIFECYCLE V7 SOURCE
VERIFIED (2026-08-14)`: ordinary Conversation Attempts now own a cancellable Go
execution context as a Composition Effect, and Chat passes only that context to
the Provider responder after scope admission. Product close cancels active
dispatch with `composition.ErrScopeClosed`; a missing or already-cancelled
scope context fails closed, rolls back Message/Segment/Attempt/Capsule state,
closes the scope, and cannot call the Provider.

A real encrypted ExternalSessionHandle is cross-component verified under this
Attempt context with its exact Conversation, Segment, Provider Account, model,
auth mode, credential reference, and revision binding. Product close terminates
the callback and the Vault runtime zeroizes plaintext after return. Capability
Context, Go context values, Journal, Capsule, diagnostics, argv, and environment
receive no handle or secret.

Focused RED/GREEN, ExternalSessionHandle/Vault parity, ten-run race, full daemon
and repository Go, vet, and diff checks pass. COMP2-D remains partial: this
verifies the native-session lifecycle boundary, but currently stateless
Provider adapters have not yet adopted native handle reuse. Tool channels and
extensions, component processes, temporary roots, broader Runtime adapters,
COMP2-E, CV6, ATL9, UI/accounting, and installed live gates remain open. No App,
network, real Provider, credential, user workspace, or installed-live action
occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-conversation-provider-session-lifecycle-v7.md`.

`CURRENT / P2D-COMP2-D PI TOOL PROCESS RESOURCE CLEANUP V8 SOURCE VERIFIED
(2026-08-14)`: the real Pi 0.82.1 governed Tool Adapter is now verified under an
active cancellation boundary across its child process group, private Tool UDS,
accepted connection, extension file/root, and optional short-path socket root.
The child enters a pending `ask` request and intentionally stays alive after the
native abort acknowledgement. Cancellation returns a content-free denial,
reaps the process within the configured bound, closes channel resources, and
leaves no extension or socket residue.

V4 already proves the real Team mission delegate receives the Attempt-owned
context; V8 proves the Pi resource graph cleans up when that context is
cancelled. No generic Effect registrar, environment channel, or Capability
Context authority was added. Focused Tool/process parity, ten-run race, full Pi
Adapter and repository Go, vet, and diff checks pass.

COMP2-D remains partial. This result is limited to the current Pi 0.82.1 path;
other Runtime adapters, arbitrary MCP component processes, crash/restart
residue, Provider native-handle adoption, COMP2-E, CV6, ATL9, UI/accounting, and
installed live gates remain open. No App, network, real Provider, credential,
user workspace, MCP server, or installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-pi-tool-process-resource-cleanup-v8.md`.

`CURRENT / P2D-COMP2-D HARNESS PROCESS AND PRIVATE PROMPT CLEANUP V9 SOURCE
VERIFIED (2026-08-14)`: Claude Code and Codex Harness process runners now join
owner-only system-prompt cleanup into every return path. RED proved that both
previously returned success when `.loom-private/loom-system-prompt.txt` cleanup
was blocked. Cleanup failure now returns `ErrHarnessProtocol` without replacing
an existing Provider/process error.

Real local process tests for both Harnesses start a long-lived descendant under
the system command runner. Attempt-context cancellation reaps the complete
process group within the configured bound, returns `context.Canceled`, and
removes the private prompt file and directory. No Prompt, Provider key, gateway
token, environment, or child output enters diagnostics, Journal, or Evidence.

Focused RED/GREEN, ten-run race, full Harness Adapter and repository Go, vet,
and diff checks pass. COMP2-D remains partial: live Claude/Codex CLI and
Provider dispatch, Attempt gateway/Context MCP crash cleanup, other Runtime
adapters, Provider native-handle adoption, COMP2-E, CV6, ATL9, UI/accounting,
and installed gates remain open. No App, network, real credential, Provider,
user workspace, Claude CLI, Codex CLI, or installed-live action occurred.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-harness-process-private-prompt-cleanup-v9.md`.

`CURRENT / P2D-COMP2-D ATTEMPT LOOPBACK SERVICE REVOCATION V10 SOURCE
VERIFIED (2026-08-14)`: the current credential gateway and Harness Context MCP
are now directly parented by the Attempt execution context. Cancellation closes
both loopback listeners before a deliberately blocked trusted callback or
Harness runner returns. Context MCP clears its bearer token under synchronized
lease and authorization access; ordinary completion retains graceful,
idempotent shutdown.

Focused RED/GREEN, expanded gateway/Context MCP protocol tests, ten-run race,
full Harness and repository Go, vet, and diff checks pass. This verifies
normal-process cancellation only. The credential callback must still return
before its bounded plaintext copy is cleared. Daemon crash/restart, `kill -9`
residue, arbitrary MCP component processes, other Runtime adapters, COMP2-E,
CV6, ATL9, UI/accounting, and installed gates remain open. No App, network,
real credential, Provider, user workspace, CLI, MCP server, or installed-live
action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-attempt-loopback-service-revocation-v10.md`.

`CURRENT / P2D-COMP2-D PI CONTEXT EXTENSION REVOCATION V11 SOURCE VERIFIED
(2026-08-14)`: the production Pi RPC Context extension now inherits the Agent
Attempt execution context at construction. Attempt cancellation terminates a
blocked Context delivery, closes its private UDS and accepted connection, and
removes the generated extension file, socket, optional short socket root, and
private extension root without waiting for the outer Adapter return.

Focused RED/GREEN, Pi Context protocol tests, ten-run race, full Pi Adapter and
repository Go, vet, and diff checks pass. The selected locked Pi 0.82.1 test was
skipped by its existing environment gate and is not counted as live binary
acceptance. COMP2-D remains partial: crash/restart and `kill -9` residue,
arbitrary MCP component processes, other Runtime adapters, COMP2-E, CV6, ATL9,
UI/accounting, and installed gates remain open. No App, network, real
credential, Provider, user workspace, Pi binary, MCP server, or installed-live
action occurred. Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-pi-context-extension-revocation-v11.md`.

`CURRENT / P2D-W2C/W2D PI SEQUENTIAL TOOL CONTINUATION V2 SOURCE VERIFIED
(2026-08-14)`: one real local managed Pi-compatible child now performs two
bounded sequential governed ToolCalls through separate peer-attested private
UDS connections in one RPC invocation. Each call carries a distinct sequence,
execution ID, payload/call lineage, and result digest. Final assistant output is
accepted only after both native results match; both results then receive exact
`harness_final_output` proof. Commands and payload authority do not enter
Bridge frames or the content-free transcript audit.

The first managed-child run RED because the sequential fixture used a unit-only
Prompt constant; the production parser correctly rejected the identity drift.
The fixture now uses the exact rendered child Prompt, with no production
protocol relaxation. Focused protocol, ten-run race, full Pi Adapter and
repository Go, vet, and diff checks pass. COMP2-D V5 separately proves
sequence-driven Turn 1→2→3 authority. Official Pi 0.82.1, a product Tool-hook
dual-call live canary, other Runtime transports, product Inbox ingress and
Runtime consumption, COMP2-E, CV6, ATL9, UI/accounting, and installed gates
remain open. No App, network,
real credential, Provider, user workspace, official Pi binary, MCP server, or
installed-live action occurred. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-pi-sequential-tool-continuation-v2.md`.

`CURRENT / P2D-W2C/W2D DURABLE AGENT INBOX V1 SOURCE VERIFIED
(2026-08-14)`: Loom now has a durable Queue/Steer/Inject authority core rather
than transient UI messages or the legacy developer work queue. Every input
freezes exact Conversation, Segment, Agent, claim generation, Runtime,
Execution Binding and Capsule digests, order, target Turn/Step, context scope
and encrypted content digest. Queue consumption is one CAS batch with the next
`TurnStarted`; ordered Steer/Inject consumption is one CAS batch with the exact
next `StepStarted`. Admission alone never wakes a Turn or Step, stale targets
and silent Steer-to-Queue fallback fail closed, and Loop/Inbox replay reads one
transactionally consistent stream set.

Input bytes are stored in the Loom Vault under the existing per-Conversation
DEK with canonical AES-256-GCM AAD over every frozen binding field and status.
The Journal stores only non-content facts and digests. The coordinator writes
ciphertext before authority, deletes only known-uncommitted ciphertext, keeps
unknown commit outcomes for reconciliation, and aligns pending/consumed state
after restart without inventing authority.

Focused RED/GREEN, active-Turn non-waking behavior, ordered consumption,
restart/reconciliation, Vault restart/tamper/plaintext-negative tests, ten-run
race, full related packages and daemon, full repository Go, complete related
race, vet, format and diff checks pass. This is `SOURCE VERIFIED / DAEMON
INGRESS AND INSTALLED LIVE OPEN`: authenticated IPC, Runtime model-input
assembly/zeroization, Swift governance UI, cross-Runtime conformance, CV6 and
ATL9 remain open. No App, network, Provider, real credential, user workspace or
external Runtime action occurred. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-durable-agent-inbox-v1.md`.

`CURRENT / P2D-W2C/W2D AGENT INBOX PRODUCT COMPOSITION V2 SOURCE VERIFIED
(2026-08-14)`: the real Loom Vault runtime and versioned Bundle slot now expose
the encrypted Agent Inbox store. Recovery, unbound and facade-closed states
fail closed; rejected writes clear caller-owned input bytes. The product mission
executor composes one Agent Inbox coordinator over the same Attempt Loop
authority and rejects a detached Inbox store without that authority.

Focused ten-run and race checks, full daemon and repository Go, vet, format and
diff checks pass. This closes product composition only. Active-Attempt
resolution, multi-Step Runtime consumption, model-input plaintext zeroization,
authenticated daemon IPC, Swift Queue/Steer/Inject controls, cross-Runtime
conformance, CV6 and ATL9 remain open. No App, network, Provider, real
credential, user workspace or external Runtime action occurred. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-inbox-product-composition-v2.md`.

`CURRENT / P2D-W2C/W2D ACTIVE ATTEMPT RUNTIME PROJECTION V3 SOURCE VERIFIED
(2026-08-14)`: every governed product Runtime now registers its already
validated Attempt Loop and Frozen Execution Binding in one mission-scoped,
revocable registry. Exact Conversation, Agent, WorkItem, Run and claim-
generation lookup exists only while the delegate executes; stale generation,
incomplete identity and post-return lookup fail closed. Returned mutable binding
fields are cloned and registration cleanup is token guarded.

This registry is a runtime projection, not execution authority, and no client
route exists. The current Team Capsule authority does not yet carry a Route
Segment ID, so V3 uses a deterministic Attempt-local Inbox Segment solely to
prevent cross-Attempt mixing. Authoritative Route Segment propagation,
multi-Step consumption and plaintext zeroization remain IPC blockers. Focused
20-run and race checks, full daemon and repository Go, vet, format and diff
checks pass. No App, network, Provider, credential, user workspace or external
Runtime action occurred. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-active-attempt-runtime-projection-v3.md`.

`CURRENT / P2D-W2A/W2C/W2D AUTHORITATIVE ROUTE SEGMENT BINDING V4 SOURCE
VERIFIED (2026-08-14)`: every governed Team Agent Attempt now freezes a
versioned, content-free Route Segment binding over Conversation, Team, Agent,
Role, Attempt number, Context Capsule digest and Frozen Execution Binding
digest. Team dispatch journals it, replay recomputes and validates it,
Aggregation rebuilds it with a changed Capsule, Supervisor verifies it before
Runtime execution, and the active-Attempt registry uses the supplied authority
instead of its former Attempt-local substitute.

Projection, Team board and strict Swift `LocalProductNode` now expose only
Segment availability, ID and digest. Older daemon responses may omit those
fields; new Capsule-bearing dispatch requires them. The first repository run
correctly found two strict Swift `invalid_response` failures, and both real
Go-to-Swift loopback contracts pass after the DTO update.

Focused ten-run race, multi-Provider/aggregation/fallback race, full Go
repository, full Swift package (203 passed, one existing visual skip, plus 10
Swift Testing cases), vet, format and diff checks pass. Multi-Step Runtime
consumption, input plaintext zeroization, authenticated Inbox IPC, Swift
governance controls, CV6 and ATL9 remain open. No App, network, Provider,
credential, user workspace or external Runtime action occurred. Evidence:
`.loom-evidence/phase2d/P2D-W2A-W2C-W2D-authoritative-route-segment-binding-v4.md`.

`CURRENT / P2D-W2C/W2D LOOM NATIVE AGENT INPUT CONSUMPTION V5 SOURCE VERIFIED
(2026-08-14)`: the durable encrypted Agent Inbox now reaches one real product
Runtime. Loom Native consumes ordered Steer/Inject inputs into the exact next
Step and Queue into the exact next Turn while retaining one frozen Execution
Binding, authoritative Route Segment, Provider Account, credential revision and
credential lease across all model rounds.

The production vertical test performs three DeepSeek-compatible Provider
rounds: initial input, Steer continuation, then Queue continuation. Attempt
authority records Turn 1 Step 1→2 and Turn 2 Step 1; Supervisor receives only
one final Bridge result and accounting is combined across rounds. Context tool
delivery follows a content-free current Step cursor.

Inbox payloads are released only after authority and encrypted storage status
agree. Runtime batches, message wire buffers and HTTP request payloads remain
mutable and are cleared after use; Journal scans contain no input or credential
plaintext. Focused ten-run race and full repository Go pass.

This is `SOURCE VERIFIED / LOOM NATIVE ONLY`. Authenticated daemon ingress,
Swift Queue/Steer/Inject controls, crash-mid-transition recovery, Pi/Codex/Claude
conformance, installed CV6, mixed-Team ATL9 and COMP2-E remain open. No App,
network, real Provider, real credential, user workspace or external Runtime was
accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-loom-native-agent-input-consumption-v5.md`.

`CURRENT / P2D-W2C/W2D AUTHENTICATED AGENT INPUT IPC AND SWIFT GOVERNANCE V6
SOURCE VERIFIED (2026-08-14)`: Queue, Steer, and Inject now cross the private
authenticated UDS through the typed `loom-agent-runtime` route. The request ID
is the Incident and idempotency anchor. Swift supplies only exact public active-
Agent identity, mode, scope, and bounded UTF-8 bytes; daemon resolves the shared
mission-scoped active-Attempt registry and freezes internal input/payload/order/
Turn/Step authority itself. Same-request replay is idempotent, substitution and
stale generation fail closed, and request plaintext is cleared on every path.

Mission center now provides per-active-Agent Queue/Steer/Inject controls. Each
row shows Harness, Provider Account, Model and status; drafts, progress,
receipts and failures remain Agent-local. Failures retain the draft and expose
a safe stage, recovery action and copyable Incident ID. App and daemon
diagnostics share `agent_input_admission` and persist only non-content identity,
mode, timing, result and controlled error metadata; privacy tests prove input
content does not enter the diagnostic JSONL.

Focused authenticated UDS/ingress/diagnostic race passes ten runs, full Swift
passes 205 XCTest cases with one intentional visual skip plus ten Swift Testing
contracts, `go vet ./...` passes, and a serial full repository Go run passes.
The initial parallel full-Go run exposed and then closed a missing Agent Input
model in the explicit Go-driven Swift probe; its concurrent Harness start
timeout passed ten isolated runs and the full serial run without production
relaxation.

This is `SOURCE VERIFIED / INSTALLED LIVE OPEN`. Crash-mid-transition recovery,
Pi/Codex/Claude input consumption, installed App use, CV6, mixed-Team ATL9 and
COMP2-E remain open. No App, network, real Provider, real credential, user
workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-authenticated-agent-input-ipc-swift-v6.md`.

`CURRENT / P2D-W2C/W2D AGENT INPUT PRE-MODEL CRASH RECOVERY V7 SOURCE
VERIFIED (2026-08-14)`: the authoritative-consumed/model-unseen Inbox window is
now recoverable before a ModelRequest exists. If Journal already contains the
exact Queue/Steer/Inject consumption and Turn/Step transition, the coordinator
idempotently aligns Vault status and can re-release only the same authenticated
payload while the target has no `ModelRequestAdmitted`.

The product AgentInputSource recognizes committed Step and Queue states,
requires the prior model-output checkpoint digest, recomputes the exact input
digest, restores the deterministic cursor and admits one ModelRequest before
returning mutable zeroizable content. Queue covers both Turn-only and first-
Step/no-ModelRequest windows. Wrong checkpoints and any post-ModelRequest retry
fail closed instead of replaying Provider dispatch.

RED reproduced both lost-delivery states. Focused recovery race passes twenty
runs; vet, diff and frozen-source serial full repository Go pass. This is a
bounded source recovery primitive, not general exactly-once execution. Daemon
restart checkpoint reconstruction and post-ModelRequest uncertain Provider
recovery remain open, as do Pi/Codex/Claude consumption, CV6, ATL9 and COMP2-E.
No App, network, real Provider, credential, user workspace or external Runtime
was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-input-pre-model-crash-recovery-v7.md`.

`CURRENT / P2D-W2C/W2D PI AGENT INPUT CONTINUATION V8 SOURCE VERIFIED
(2026-08-14)`: Pi now consumes Queue/Steer/Inject after an accepted assistant
checkpoint by sending another native RPC prompt to the same managed child
process. The Adapter retains one frozen Execution Binding and process identity,
one monotonic Bridge sequence, one ACK and one final terminal while combining
Harness accounting across prompts.

The shared Runtime renderer returns mutable prompt bytes. Inbox content, prompt
wire bytes, round state and accounting snapshots are cleared after their
bounded use; Bridge frames and transcript audit remain content-negative. RED,
focused ten-run race, affected Runtime packages, vet, diff and frozen-source
serial full repository Go pass.

This is managed Pi-compatible child source verification. Follow-up prompts are
currently strict text continuations; a second Context/Tool event fails closed.
Official Pi 0.82.1 multi-prompt conformance, Codex/Claude consumption, daemon-
restart reconstruction, installed CV6, mixed-Team ATL9 and COMP2-E remain open.
No App, network, real Provider, credential, user workspace or external Runtime
was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-pi-agent-input-continuation-v8.md`.

`CURRENT / P2D-W2C/W2D HARNESS MUTABLE PROMPT PREREQUISITE V9 SOURCE VERIFIED
(2026-08-14)`: Codex and Claude Code process prompts now cross Adapter, process
runner and command stdin as owned mutable bytes. Every production return path
clears the bounded prompt/stdin slice; tests clone synthetic bytes explicitly
and prove the original owner is zeroized. Prompt content remains outside argv,
environment, diagnostics, Journal, Evidence and ordinary Bridge metadata.

This does not advertise Agent Input support. Current Codex uses one-shot
`exec --ephemeral`; Claude Code uses `--print --no-session-persistence`.
Repeated process launch is not a valid continuation, so both Adapters remain
fail closed until version-locked persistent app-server/stream-json contracts
pass process identity, checkpoint, session binding, cancellation, accounting
and privacy conformance.

Harness package, focused ten-run race, vet, diff and frozen-source serial full
repository Go pass. No App, network, Provider, credential, user workspace or
external Harness was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-harness-mutable-prompt-prerequisite-v9.md`.

`CURRENT / P2D-W2C/W2D DAEMON RESTART ATTEMPT RECONSTRUCTION V10 SOURCE
VERIFIED (2026-08-14)`: daemon startup now reconstructs every current Agent
Attempt Loop from its strict first Journal event, recomputes the stream identity,
replays the complete Loop and Inbox, and revalidates current Run generation,
Runtime, Agent, Execution Binding, Capsule, capability and budget authority.
Historical generations and terminal Runs are not revived.

Consumed Queue or Steer/Inject authority with no ModelRequest reconstructs the
exact Turn/Step cursor and prior output checkpoint as `pre_model_resume`. An
open Step with `ModelRequestAdmitted` becomes `provider_outcome_uncertain` and
is never replayed automatically. Neither outcome enters the active-Attempt
registry or authorizes Runtime/Provider dispatch.

Journal/Run authority corruption remains daemon-level fail-closed. A missing or
conflicting encrypted Inbox payload is isolated to that Agent as
`recovery_blocked`; another Agent remains independently projected. Startup
persists only safe `agent_attempt_reconcile` diagnostics with controlled error
codes and frozen non-content identity.

Focused Queue/Step/uncertain/terminal/isolation tests, focused ten-run race, full
related packages, serial full repository Go, vet and diff checks pass. Explicit
resume authority and UI,
persistent Harness process recovery, version-locked Codex/Claude continuation,
official Pi, installed CV6, mixed-Team ATL9 and COMP2-E remain open. No App,
network, Provider, real credential, user workspace or external Runtime was
accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-daemon-restart-attempt-reconstruction-v10.md`.

`CURRENT / P2D-W2C/W2D CODEX APP-SERVER AGENT INPUT CONTINUATION V11 SOURCE
VERIFIED (2026-08-14)`: a version-locked Codex 0.144.1 Agent may now consume
Queue/Steer/Inject through repeated `turn/start` requests on one ephemeral
app-server thread and one managed process. Exact response, thread and Turn IDs
remain bound; only a completed assistant output becomes the next Loom
checkpoint. Per-Turn token usage is validated and combined across rounds.

The shared system session runner enforces exact executable identity, bounded
JSONL/stdout/stderr, mutable input clearing, timeout and process-group reaping.
Codex advertises Agent Input support only when the persistent runner and exact
executable digest both pass; drift fails before credential access. Ordinary
requests retain the one-shot `exec --ephemeral` path.

This is `SOURCE VERIFIED / SAME-PROCESS ONLY`. Codex native thread persistence,
daemon-restart reattachment, installed Codex/Provider live, CV6, ATL9 and
COMP2-E remain open. No App, network, Provider, credential, user workspace or
external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-codex-app-server-agent-input-v11.md`.

`CURRENT / P2D-W2C/W2D CLAUDE STREAM-JSON AGENT INPUT CONTINUATION V12 SOURCE
VERIFIED (2026-08-14)`: a version-locked Claude Code 2.1.196 Agent may now keep
one `--no-session-persistence` stream-json process and one emitted `session_id`
across Queue/Steer/Inject rounds. The parser requires exact replayed user input,
tracks typed `tool_use`/`tool_result` pairs, accepts only the audited
`compact_boundary`, and exposes no checkpoint until a successful final result.
Usage and harness-reported cost are combined across all rounds.

Claude advertises Agent Input support only when the persistent runner and exact
executable digest pass. The existing one-shot JSON path remains unchanged for
ordinary requests. Harness package/race/vet and the serial Runtime, Supervisor
and daemon matrix pass. One earlier parallel matrix hit the pre-existing
three-second cancellation fixture under load; the exact fixture then passed ten
runs and the complete affected matrix passed serially without changing its
timeout.

This is `SOURCE VERIFIED / SAME-PROCESS ONLY`. Encrypted native session-handle
persistence, daemon-restart reattachment, installed Claude/Anthropic live,
CV6, ATL9 and COMP2-E remain open. No App, network, Provider, credential, user
workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-claude-stream-json-agent-input-v12.md`.

`CURRENT / P2D-COMP2-D LOCAL IPC BUNDLE OWNERSHIP V12 SOURCE VERIFIED
(2026-08-14)`: production no longer constructs the typed product route handler
or applies IPC decorators before Composition activation. The built-in
`loom-local-ipc` Bundle now owns a stable revocable handler slot and no-I/O
factory. Bundle Start constructs Setup first, then the route handler with the
unchanged observability and controlled-journey wrapper order; Ready gates both
before request admission.

Construction failure rolls back Setup and permanently revokes the slot. Close
waits for in-flight requests, revokes old handler references, then closes Setup.
Production supplies no direct handler to the compatibility activator, while the
COMP2-A direct facade remains available only for parity tests. Missing or dual
handler sources fail closed.

The mandatory AST RED, local IPC lifecycle tests, full daemon package, focused
race, Composition race, serial full repository Go, full vet and diff checks
pass. This is source verification only. `legacy.product.dispatch`, COMP2-A
parity facade and compatibility decoders remain until COMP2-E restart,
crash-window, privacy, shutdown and installed App startup gates pass. Installed
CV6, mixed-Team ATL9, explicit recovery/UI, accounting and real Provider replies
remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network,
Provider, credential, user workspace or external Runtime was accessed.
Evidence:
`.loom-evidence/phase2d/P2D-COMP2-D-local-ipc-bundle-ownership-v12.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY PROJECTION V13 SOURCE VERIFIED
(2026-08-14)`: the Team board now accepts the closed
`agent_attempt_reconcile` stage and projects each V10 restart outcome only onto
the exact Incident/Provider Account/Model Agent row. Swift recognizes the same
stage. Mission governance renders resume approval required, uncertain Provider
outcome, unavailable encrypted input, or recovery-state conflict while
retaining View diagnostics and Copy incident ID.

This is a read-only governance boundary. V10 outcomes remain non-retryable; no
Retry or Resume command was added, recovered state does not enter the active
Attempt registry, and no Runtime/Provider dispatch or native session
reattachment occurs. Focused RED/GREEN, ten-run API race, complete Swift (207
XCTest with one existing skip plus ten Swift Testing contracts), full API and
daemon packages, full vet, and diff checks pass. Explicit recovery authority,
restart-safe native session persistence, installed CV6, mixed-Team ATL9,
accounting completion and COMP2-E remain open under the sole `ACTIVE / PARTIAL`
Phase 2D Goal. No App, network, Provider, credential, user workspace or external
Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-attempt-recovery-projection-v13.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY DECISION AUTHORITY V14 SOURCE
VERIFIED (2026-08-14)`: Loom now has a separate Journal authority for one
explicit `resume_pre_model` decision. The command freezes the exact
domain-separated V10 recovery-candidate digest and a separately domain-separated
restart-capability digest supplied by a trusted Runtime resolver. The authority
rebuilds the candidate around a consistent snapshot and atomically fences the
recovery-decision, Attempt Loop and Run stream heads. Concurrent distinct
decisions have exactly one winner.

An uncertain Provider outcome, candidate/binding/capability substitution,
Run terminalization and Loop progress fail closed before an authority fact can
be used. The fact is content-free. This is decision authority only: production
composition supplies no restart resolver, and there is no dispatch lease,
active-Attempt registration, native session reattachment, IPC/Swift action or
Runtime/Provider call. Focused RED/GREEN, ten-run race, complete
`internal/work`, serial full repository Go, full vet and diff checks pass.
Decision consumption, restart-safe Runtime composition, UI, installed CV6,
mixed-Team ATL9, accounting and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, Provider, credential, user
workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-attempt-recovery-decision-authority-v14.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY CONSUMPTION V15 SOURCE VERIFIED
(2026-08-14)`: one exact V14 recovery approval now has one durable consumer.
Before appending `AgentAttemptRecoveryConsumed`, Loom rebuilds the V10 candidate,
re-resolves the trusted restart capability, and atomically fences the recovery,
Attempt Loop and Run heads. The authority creates the lease ID internally, so a
caller cannot exploit idempotent replay to obtain duplicate success.

Only the CAS winner receives a process-local dispatch lease. Its cloned frozen
grant can be taken once; all concurrent and later consumers receive no lease.
Capability drift during confirmation leaves the decision unconsumed. Journal
facts remain content-free. Production still supplies no restart-safe resolver
or adapter and performs no active-Attempt registration, native-session
reattachment, authenticated IPC, Swift action, Runtime dispatch or Provider
replay. Focused RED/GREEN, ten-run race, complete `internal/work`, serial full
repository Go, full vet and diff checks pass. Runtime composition, UI,
installed CV6, mixed-Team ATL9, accounting and COMP2-E remain open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, Provider, credential,
user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-attempt-recovery-consumption-v15.md`.

`CURRENT / P2D-W2C/W2D AGENT ATTEMPT RECOVERY RUNTIME ATTACHMENT V16 SOURCE
VERIFIED (2026-08-14)`: safe pre-model recovery now preserves the exact Route
Segment from the consumed encrypted Inbox records selected by its frozen input
IDs. Missing or cross-Segment metadata produces only the affected Attempt's
`agent_input_recovery_conflict`. The v2 recovery-candidate digest and
content-free authorization metadata bind the Segment. The consumed grant also
freezes the recovery operation Incident ID.

Trusted product composition can take the grant once, reattach an identity-only
Runtime session, verify the exact Runtime instance and session-binding digest,
and only then insert the recovered active Attempt. Any later failure closes the
session. Normal close revokes the registry entry before session close, and is
idempotent under concurrency. The reattachment port has no Provider-dispatch
method, so an uncertain Provider outcome cannot be translated into another
request.

This is source composition only. Production supplies no restart-safe capability
resolver or reattacher, Codex/Claude native sessions are not persisted across
daemon restart, and there is no authenticated recovery IPC, Swift action,
Runtime continuation or Provider replay. Focused tests and ten-run race gates
pass together with complete affected-package tests, serial full repository Go,
full vet and diff checks. Installed CV6, mixed-Team ATL9, accounting and COMP2-E
remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network,
Provider, credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-agent-attempt-recovery-runtime-attachment-v16.md`.

`CURRENT / P2D-W2C/W2D LOOM NATIVE RESTART CAPABILITY V17 SOURCE VERIFIED
(2026-08-14)`: the built-in Loom Native adapter now explicitly declares
`loom-owned-checkpoint/v1` restart conformance. A trusted product resolver
revalidates the complete Frozen Execution Binding, exact Runtime, Provider
Account, credential revision, Model and content-free encrypted Capsule
authority, then binds the recovered Route Segment, previous output checkpoint
and input IDs into a domain-separated session digest.

V16 can attach that identity-only session and register the exact active Attempt.
Capability resolution and attachment read no Capsule content, acquire no
credential, call no Runtime `Execute`, and issue no Provider request. Unsupported
or duplicate Runtime identity, duplicate exact Capsule authority, substitution
and post-consumption Capsule drift fail closed; unrelated corrupt Capsule state
does not block another Agent.

This is source composition, not a user-visible Resume flow. The daemon does not
yet construct this recovery service or assemble and dispatch the continued Loom
Native request. Authenticated IPC, Swift governance, installed CV6, mixed-Team
ATL9, accounting and COMP2-E remain open under the sole `ACTIVE / PARTIAL`
Phase 2D Goal. Ten-run focused race, serial full repository Go, vet and diff
checks pass. No App, network, Provider, real credential, user workspace or
external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-loom-native-restart-capability-v17.md`.

`CURRENT / P2D-W2C/W2D ENCRYPTED LOOM NATIVE CONTINUATION V18 SOURCE VERIFIED
(2026-08-14)`: Loom Native now persists an authenticated previous-output
checkpoint before consuming the next Agent Input. A dedicated
Conversation-DEK store binds checkpoint content to the exact Conversation,
Segment, Attempt, Agent, WorkItem, Run/generation, Runtime, frozen Execution
Binding, Capsule, Turn and Step. It is separate from Tool Result payloads and
the Event Journal; missing, tampered, substituted or ambiguous state fails
closed and Conversation crypto-erasure removes its records.

The controlled local recovery path consumes the one-use grant, attaches and
registers the exact active Attempt, restores the frozen Capsule, checkpoint and
consumed Inbox material, admits one new ModelRequest, acquires the exact
credential lease, dispatches Loom Native, terminalizes Step/Turn and deletes
the obsolete checkpoint. The uncertain-Provider branch remains non-resumable.
Journal tests exclude checkpoint, input, reply and credential fixture content.

Focused RED/GREEN, ten-run cross-package race, serial full repository Go, full
vet, exact gofmt and diff gates pass. This is source verification with a local
simulated Provider fixture. The daemon does not yet construct an authenticated
recovery service, and there is no IPC command, Swift governance action or
user-visible Resume. A production route must also restore authorized frame
validation, commit Run terminal/accounting authority and close the prior
execution authorization; direct IPC-to-continuation wiring is forbidden because
it would leave model completion and Team board state divergent. Installed CV6,
mixed-Team ATL9, accounting, COMP2-E and restart support for Codex/Claude/Pi
remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-encrypted-agent-checkpoint-continuation-v18.md`.

`CURRENT / P2D-W2C/W2D RECOVERY FRAME AND TERMINAL CLOSURE V19A SOURCE VERIFIED
(2026-08-14)`: the controlled Loom Native recovery path now uses a dedicated
Bridge frame authority bound to the consumed one-use recovery grant, exact
unrevoked original grant identity and operation set, deterministic dispatch ACK
and recovery Incident ID. The AdapterResult transcript must exactly match every
accepted frame before Attempt Loop success can be written. Correlation or
binding substitution, observer rejection, missing/ambiguous original grant and
transcript mismatch fail before Run terminal authority.

After one valid terminal stream, recovery resolves accounting against the exact
authoritative Run and frozen Provider/Model Rate Card, commits Run terminal
state and revokes the exact original grant. The old bearer token remains
unrecoverable. Cleanup authority outlives request cancellation under a bounded
timeout, and session close errors are retained.

Focused RED/GREEN, ten-run recovery race, complete supervisor and daemon
packages, serial full repository Go, full vet and exact gofmt checks pass. This
is source verification only. Production does not yet construct the coordinator,
restore the Team frame/evidence observer or expose authenticated recovery IPC
and Swift governance. A crash after Run terminal commit and before grant revoke
is still an explicit V19B startup-reconciliation gate; it must close only the
exact grant and must never redispatch the Provider. Installed CV6, mixed-Team
ATL9, broader Runtime recovery and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-recovery-frame-terminal-closure-v19a.md`.

`CURRENT / P2D-W2C/W2D PRODUCTION RECOVERY LIFECYCLE V19B SOURCE VERIFIED
(2026-08-14)`: the production mission Bundle now owns the Loom Native recovery
decision authority and V19A completion coordinator whenever durable Agent Inbox
recovery is configured. It composes the exact restart resolver, encrypted
checkpoint continuation, active-Attempt attachment, frame validation, Run
accounting/terminal authority and original-grant closure from the existing
production stores and frozen adapters.

Before resumed output is accepted, production rebuilds the projection and
restores the existing Team evidence and NodeOutput observer for exactly one node
Attempt selected by WorkItem, Run, generation, Runtime and Agent. Missing or
ambiguous ownership fails closed.

Daemon startup now repairs a crash after Run terminal commit but before grant
revocation. It revokes only one exact unrevoked grant whose complete execution
tuple matches the terminal Run, ignores already revoked grants, and is
idempotent. Tuple drift or duplicate pending authority stops startup. The
reconciler has no Runtime, adapter, credential or Provider-dispatch port.

Each successful repair writes a persistent, content-free
`authorization_reconcile` record at `agent_attempt_reconcile` with the startup
Incident ID and non-secret frozen execution identity. No Capsule, Prompt,
transcript, Provider body, credential, grant token or ciphertext is recorded,
and the success is not projected as an Agent failure.

The focused RED/GREEN, ten-run recovery race, complete daemon package, serial
full repository Go, full vet, exact gofmt and diff checks pass. This remains
source verification: no authenticated preview/confirm/resume IPC or Swift
governance action exists yet, so users cannot invoke recovery. V19C, installed
CV6, mixed-Team ATL9, broader Runtime recovery, ATL3-ATL8 and COMP2-E remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real
Provider, real credential, user workspace or external Runtime was accessed.
Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-production-recovery-lifecycle-v19b.md`.

`CURRENT / P2D-W2C/W2D AUTHENTICATED RECOVERY IPC AND SWIFT GOVERNANCE V19C
SOURCE VERIFIED (2026-08-14)`: the V19B production lifecycle is now reachable
through one typed authenticated private-UDS route with distinct Preview,
Confirm and Resume operations. Preview is read-only. Confirm binds the
authenticated principal to the exact current candidate and restart capability.
Only the separate Resume action consumes that decision and can enter Provider
continuation.

The Swift Store requires fresh authoritative candidate state, refuses Resume
without the exact confirmation, suppresses duplicate in-flight Resume and
invalidates uncertain local authority after failure. Strict wire models reject
unknown or malformed recovery data. Mission Inspector limits controls to the
affected Agent, displays Harness, Provider Account, Model and credential
revision, and keeps shield confirmation separate from the Resume command.

App and daemon operational diagnostics share the request Incident ID and record
only operation, stage, elapsed time, result, stable error and retryability. They
exclude candidate/capability digests, principal ID, Prompt, transcript,
Provider body, credential, Authorization header and API key.

The focused RED/GREEN suite, ten-run cross-package recovery race, real Go-server
to strict Swift contract probe, complete 214-test Swift suite, serial full
repository Go, full vet, exact gofmt, privacy and diff checks pass. One
visual-export-only Swift test was skipped by design. This remains source
verification: installed App recovery and real Provider continuation were not
run. Broader Runtime restart support, CV6, mixed-Team ATL9, ATL3-ATL8 and
COMP2-E remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App,
network, real Provider, real credential, user workspace or external Runtime was
accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-authenticated-recovery-ipc-swift-governance-v19c.md`.

`CURRENT / P2D-W2C/W2D TOOLCALL RECOVERY DECISION GOVERNANCE V20 SOURCE
VERIFIED (2026-08-14)`: ToolCalls terminalized as `side_effect_unknown` now have
a separate content-free authenticated Preview/Resolve authority. The exact
candidate is domain-separated and stream-head CAS fenced. Abort, trusted
observed-effect acceptance and trusted replacement-Attempt authorization all
close the original execution; none can rerun the original ToolCall.

Production currently injects no observation or replacement resolver, so the
daemon advertises only Abort. Swift requires a fresh exact candidate and one of
the daemon's `available_actions`, suppresses duplicate decisions and discards
uncertain local state after failure. Mission Inspector keeps ToolCall recovery
separate from Agent restart recovery, shows only Tool/Job/Incident identity,
requires confirmation and offers privacy-safe diagnostics and Incident copy.

Focused RED/GREEN, ten-run race, typed authenticated UDS, strict Go-to-Swift
contract, complete 223-test Swift suite plus 10 Swift Testing cases, affected
Go package tests and diff checks pass. One visual-export-only Swift test was
skipped by design. This remains source verification: trusted observed-result
delivery, replacement-Attempt startup, installed App recovery, CV6, mixed-Team
ATL9, remaining ATL3-ATL8 and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-tool-recovery-decision-governance-v20.md`.

`CURRENT / P2D-W2C/W2D LOOM NATIVE SEQUENTIAL CONTEXT TOOLS V21 SOURCE
VERIFIED (2026-08-14)`: one Loom Native Provider exchange now supports up to
four strictly sequential governed `loom_read_context` calls. Every call keeps
the exact frozen Attempt, Route Segment, Provider Account, credential revision
and Model, receives a distinct monotonic ToolCall/payload lineage, and must be
authoritatively delivered before the next exclusive call can be admitted.

The native adapter aggregates usage across all Provider rounds, zeroes its
owned mutable result buffers, stops advertising the tool after the fourth call
and fails closed on a nonconforming fifth tool response. Product integration
proves two real Provider continuations through admission, dispatch,
accepted/delivered facts and successful Step/Turn terminal state. Context and
credential content stay out of Journal and Bridge frames.

Focused RED/GREEN, ten-run cross-package race, complete `internal/work`,
`nativeadapter` and `cmd/loomd` package tests pass. This is source verification
with simulated Provider responses. General Read/Grep/Web/MCP, parallel tools,
other Runtime transports, installed CV6, mixed-Team ATL9 and COMP2-E remain
open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real
Provider, real credential, user workspace or external Runtime was accessed.
Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-loom-native-sequential-context-tools-v21.md`.

Supplemental whole-repository runs remain non-green because existing local
process/socket fixtures intermittently returned a Pi health timeout or
`local product unavailable` under package load. Each affected exact test passed
alone, including 10 consecutive runs for both Local IPC cases. These failures
are outside the V21 source set and are retained as an explicit repository test
stability residual rather than reported as a passing full-repository gate.

`CURRENT / P2D-W2C/W2D CODEX AND CLAUDE BOUNDED MULTI-CONTEXT V22 SOURCE
VERIFIED (2026-08-14)`: Codex and Claude Code may now perform up to four
distinct governed `loom_read_context` calls through one Attempt-scoped private
MCP service. Every call receives a monotonic encrypted payload lineage and
remains `accepted/pending`; a later MCP call is not treated as consumption
proof. Only the validated Harness final output acknowledges all prepared
bindings in order with `harness_final_output` proof.

Duplicate, fifth, concurrent and post-seal calls fail closed. Final-output ACK
cannot race an in-flight Prepare, and a partial ACK failure resumes at the first
undelivered binding without repeating earlier acknowledgements. Product policy
grants four parallel pending Context slots only to Codex/Claude. Loom Native,
Pi and default Runtime types retain exclusive/one; V21's Provider-continuation
proof remains unchanged.

Real private-MCP adapter tests cover two calls for both Codex and Claude.
Product authority tests observe two Codex results pending together before final
output and delivered afterward. Complete `harnessadapter` and `cmd/loomd`
packages, ten-run cross-package race, vet and exact formatting checks pass.
This is source verification only. General tools, installed CV6, mixed-Team
ATL9, broader Runtime restart, accounting/UI and COMP2-E remain open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, Provider, credential,
user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-codex-claude-bounded-multi-context-v22.md`.

`CURRENT / P2D-W2C/W2D CODEX AND CLAUDE GOVERNED READ/GREP V23 SOURCE
VERIFIED (2026-08-15)`: Tool Gateway envelope, binding, result, encrypted
delivery and acknowledgement contracts are now Runtime-neutral. Pi preserves
its existing API and error identity through compatibility aliases; the
production bridge no longer depends on Pi-owned type names.

Codex and Claude expose only Context, Read and Grep through one Attempt-scoped
private MCP. All four Harness process modes derive the same exact allowlist;
separate executable-byte conformance and complete Attempt/claim/generation/
Route/Execution Binding/Capsule/Incident validation fail closed before
credential access. Loopback HTTP cancellation is merged into the daemon-owned
Attempt context and cannot replace its execution authority.

The existing production permission/execution bridge and Attempt Loop authority
persist Read/Grep output in the encrypted payload store. Results remain pending
until validated final output provides ordered `harness_final_output` proofs.
Distinct paths may use read-only parallel slots; Read/Grep on the same path
conflict through a content-free digest. Paths, patterns and result content do
not enter Journal or operational diagnostics.

Focused cross-runtime tests, complete runtime/Harness/Pi/daemon package tests,
ten-run race, vet, gofmt and diff checks pass. This is source verification only:
Harness Bash/Edit/Web/MCP, restart reattachment, TTL/compaction, accounting/UI,
installed CV6, mixed-Team ATL9 and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, Provider, real credential,
user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-codex-claude-governed-read-grep-v23.md`.

`CURRENT / P2D-W2C/W2D CODEX AND CLAUDE NATIVE-TOOL BYPASS CLOSURE V24
SOURCE VERIFIED (2026-08-15)`: the V23 private MCP no longer coexists with an
uncontrolled Harness-native workspace path. Every Codex/Claude credential
gateway invocation now freezes the exact Loom MCP tool names derived from that
Attempt's lease. OpenAI and Anthropic request tool catalogs, duplicate JSON
keys and successful Provider tool-call output are validated against that
frozen set before a Provider request or Harness delivery can continue.

Codex runs from the private Attempt temp directory with read-only sandboxing;
shell, unified exec, freeform apply-patch and dynamic tool search feature paths
are disabled in both exec and app-server configuration. Claude one-shot and
stream-json run from the same private boundary with `dontAsk`, an MCP-only
allowlist and an explicit native-tool denylist. Both Harness system prompts now
describe only the Loom-governed MCP capability instead of advertising direct
Bash/Edit/Read authority.

Native or mixed catalogs, `tool_search`, unapproved Loom tools, duplicate
`model/tools/type` keys, Codex local shell calls and Claude native tool-use
responses fail closed. Rejected requests do not reach the Provider; rejected
responses do not reach the Harness. Provider credentials, loopback bearer
tokens, Prompt, workspace paths and Provider bodies are not added to Journal,
Evidence or diagnostics.

Focused RED/GREEN, complete Harness package, affected runtime/Pi/daemon
packages, ten-run race, vet, gofmt, privacy and diff checks pass. This remains
source verification. The exact external Codex/Claude executables were not
launched, so strict-config acceptance and eager MCP behavior require a later
explicit component/live gate. Web/MCP expansion, broader Runtime restart,
installed CV6, mixed-Team ATL9, accounting/UI and COMP2-E remain open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-codex-claude-native-tool-bypass-closure-v24.md`.

`CURRENT / P2D-W2C/W2D ROLE DEPENDENCY CONTEXT CAPSULE V25 SOURCE VERIFIED
(2026-08-15)`: an ordinary multi-Agent DAG no longer treats dependency
completion as scheduling-only. Before a dependent Agent Attempt is admitted,
the coordinator resolves every exact succeeded dependency Attempt from the
authoritative Team Projection and Evidence Store, verifies the complete
Attempt/receipt lineage, and extracts only authorized output events.

The target Agent receives a new immutable Role Context Capsule and Route
Segment. Each source's lineage is a role-restricted authoritative item; its
model-produced output is a separate role-restricted
`untrusted_model_output`. A peer node, Agent, receipt, Evidence digest, output
summary or plan substitution fails closed. Provider Account, credential
reference/revision and peer Capsule content are not copied into the dependent
prompt. Parallel Route aggregation now uses the same deterministic Capsule
extension primitive without losing existing budget, policy or access
omissions and retrievable references.

The four-Provider DAG canary proves that the Codex Main Agent receives the
authorized outputs of the Claude, Kimi and MiniMax dependency Attempts while
their account and credential identities remain undisclosed. Dynamic approved
fallback proves that retry Runtime changes retain the same dependency
authority while freezing a new Capsule/Segment beside the exact fallback
Execution Binding.

Focused RED/GREEN, direct substitution tests, ten-run race, complete affected
packages and daemon, serial full repository Go, vet, gofmt and diff checks
pass. This is source verification only. Authoritative observed test-state
assembly, model-specific tokenizers/wire adapters, user-visible disclosure
inspection, installed CV6, mixed-Team ATL9 and COMP2-E remain open under the
sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-role-dependency-context-capsule-v25.md`.

`CURRENT / P2D-W2C/W2D OBSERVED ACCEPTANCE STATE V26 SOURCE VERIFIED
(2026-08-15)`: dependency and aggregation Role Context Capsules now distinguish
projected execution facts from model output. Before a source can cross into a
dependent Attempt, Loom requires the exact projected Attempt to be succeeded,
its versioned Output Contract and digest to be valid, its output classification
to be `valid_nonempty` or `valid_empty`, and its acceptance decision to be an
exact accepted digest with a UTC decision time.

Those facts are emitted as a compact, role-restricted
`observed_execution_state` item with observed trust and authoritative
provenance. The adjacent source-lineage item remains authoritative, while the
Provider-generated output remains a separate role-restricted
`untrusted_model_output`. Classification or acceptance substitution fails
closed. The compact shape preserves Pi's existing 6 KiB dispatch boundary;
Output Contract and Evidence digests are validated before construction and
remain bound by the adjacent authoritative lineage rather than being duplicated
into the prompt.

Focused RED/GREEN, complete affected packages, ten-run race, vet, gofmt and diff
checks pass. A fresh serial full-repository run passed every package except
`cmd/loomd`, where the package hit its 10-minute timeout after
`TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC` reported an
IPC protocol failure; that test and the interrupted Loom Native recovery test
both passed immediately in isolated reruns. The full-repository gate is
therefore recorded as a daemon suite timing/stability residual, not as passing.
This remains source verification only: Loom does not yet ingest a structured,
authoritative `test_report` or tool-result document, and model claims never
become observed facts. Installed CV6, mixed-Team ATL9, disclosure UI,
model-specific adapters and COMP2-E remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider, real
credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-observed-acceptance-state-v26.md`.

`CURRENT / P2D-W2C/W2D GOVERNED TEST REPORT V27 SOURCE VERIFIED
(2026-08-15)`: Loom no longer relies on Provider prose to tell a dependent
Agent that a controlled test command passed. The common Tool Gateway now
recognizes a strict single-command subset of Go, Swift, Cargo, Pytest, npm,
pnpm, Yarn and Bun test runners. It commits a typed immutable report only after
the exact Bash ToolCall was authorized, dispatched and its result payload was
accepted by the Attempt Loop authority.

Each report freezes runner, coarse scope, exit-derived outcome, ToolCall and
execution identities, arguments/output digests and duration. The Attempt Loop
Journal stores only canonical report metadata and digests, not raw command or
output content. Replay, idempotence, payload substitution and Attempt authority
drift fail closed. A report becomes queryable to Team coordination only after
the matching result reaches `delivered`.

Dependent and aggregation Agents receive these reports inside the existing
role-restricted `observed_execution_state` item. The coordinator derives the
query only from the exact projected Attempt, frozen Execution Binding and
Context Capsule; requests cannot inject report state. The compact projection
contains runner, scope, outcome, call sequence, report digest and set digest.
Provider output remains an adjacent `untrusted_model_output`. Ordinary Bash is
not classified as a test.

Complete `internal/verification`, `internal/work`, `internal/app` and
`cmd/loomd` suites, affected race tests, vet and formatting checks pass. A fresh
full-repository run passed V27 and daemon packages but reported one existing
Darwin Keychain helper stage-timing failure in `internal/credentials`; the
isolated test then passed and the complete credentials package passed three
consecutive runs. The full-repository gate is therefore not reported as wholly
passing. This remains source verification: reports are command-level exit
observations, not per-test-case parsing or a general verifier artifact. UI,
installed CV6, mixed-Team ATL9, Provider live acceptance and COMP2-E remain open
under the sole `ACTIVE / PARTIAL` Phase 2D Goal. No App, network, real Provider,
real credential, user workspace or external Runtime was accessed. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-governed-test-report-v27.md`.

`CURRENT / P2D-W2C/W2D GOVERNED TEST REPORT BOARD V28 SOURCE VERIFIED
(2026-08-15)`: the Agent Team Board now projects the V27 governed test reports
onto the exact current Agent Attempt. Each Agent row exposes only report count,
passed/failed counts, the latest closed runner/scope/outcome values and stable
report/set digests. Raw command text, arguments, stdout/stderr, Prompt,
Provider body and credential content do not enter the Board payload or Swift
presentation.

The read service and product Bundle facade receive the existing Attempt Loop
authority as an observational report source. Board queries are rebuilt from
the projected Team, conversation, WorkItem, Run, claim generation, Runtime,
Agent, Incident, frozen Execution Binding and Capsule identities. Missing,
invalid or duplicate report state is omitted rather than fabricated. The
strict Swift decoder rejects contradictory availability/counts, unknown
runner/scope/outcome values and malformed digests while preserving older
payloads that contain no report fields.

Mission Inspector now shows each affected Agent's compact pass/fail summary
and latest governed test classification beside its existing Runtime,
Provider, context, failure and accounting metadata. Go API and focused daemon
tests, ten-run race, vet/gofmt, three focused Swift tests and the full macOS
package suite (`223` XCTest cases with `1` skipped plus `10` Swift Testing
cases) pass. This is source verification only. It is not per-test parsing,
raw-output inspection, installed App validation, CV6, mixed-Team ATL9 or real
Provider acceptance. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-governed-test-report-board-v28.md`.

`CURRENT / P2D-W2C/W2D PRODUCTION REMOTE TOOL BROKER COMPOSITION V29 SOURCE
VERIFIED (2026-08-15)`: remote Web/MCP execution is no longer limited to an
already-assembled executor injected from outside Composition. `loom-work` now
constructs the Loom-owned Broker during Bundle Start from explicit bounded
Search and MCP client ports plus an opt-in hardened WebFetch transport. The
product builder passes configuration only; it does not construct the Broker or
retain its runtime resource.

Capabilities are published from actual backends. Search-only, Fetch-only and
allowlisted MCP can be admitted independently; an absent backend, MCP client /
allowlist mismatch, invalid timeout/size ceiling or empty capability set fails
before Bundle Ready. The default production configuration remains `nil`, so
Loom does not advertise Web/MCP merely because code exists.

The Broker owns a revocable lifecycle context and an idempotent Composition
Effect. Work startup rollback and normal Dispose cancel active calls, close the
owned system HTTP transport, clear the delegate and make retained references
fail closed. Query, URL, MCP arguments/results, endpoint credentials, Prompt,
Provider body and secret material are not added to descriptors, snapshots,
Journal or diagnostics by this composition path.

Focused RED/GREEN, complete Tool Broker and Execution packages, complete
`cmd/loomd`, twenty-run daemon race coverage, vet/gofmt/diff checks and a fresh
serial full-repository Go run pass. No network request, external MCP client,
App build, installed live run, Provider, real credential or user workspace was
used. Production Search/MCP enrollment UI/config, installed ATL9, CV6 and
COMP2-E remain open under the sole `ACTIVE / PARTIAL` Phase 2D Goal. Evidence:
`.loom-evidence/phase2d/P2D-W2C-W2D-production-remote-tool-broker-composition-v29.md`.

`CURRENT / P2D-W2D PERSISTED REMOTE TOOL BACKEND ENROLLMENT V30 SOURCE
VERIFIED (2026-08-15)`: Search and MCP backend intent is now an append-only,
versioned governance fact instead of process-only configuration. Each
Enrollment freezes one exact Provider Account Policy version, revision and
digest, adapter, endpoint fingerprint, MCP server/tool allowlist and bounded
concurrency, per-Attempt calls, timeout, result bytes and budget. Configure
atomically fences both the Enrollment and policy stream heads; stale policy,
cross-account substitution and malformed Search/MCP shapes fail closed.
Revocation remains available after policy drift and affects only the selected
Enrollment.

The Governance Bundle owns the authority. Setup projects deterministic active
or revoked records under the exact Provider Account, including an explicit
`policy_current` signal; an orphan account without configured/verified
credential metadata cannot enroll. Authenticated UDS configure/revoke routes
inject the trusted Request ID as correlation, map conflict/stale/not-found/
unavailable separately and emit only content-free operational diagnostics.
The strict Swift setup decoder accepts legacy accounts with no Enrollments,
accepts stale records only when marked non-current, and rejects unknown fields,
raw endpoints, contradictory policy state and malformed allowlists/digests.

Focused authority/projection/App/API/daemon tests, ten-run race checks, the
complete daemon suite, a fresh serial full-repository Go run, vet/gofmt/diff
checks, the exact Go-to-Swift setup contract and the complete macOS package
suite (`223` XCTest cases with `1` skipped plus `11` Swift Testing cases) pass.
This is `SOURCE VERIFIED / CLIENT EDITING, RUNTIME MATERIALIZATION AND
INSTALLED LIVE OPEN`: no Swift configure/revoke action, Broker client
materialization, per-Agent preflight/UI, network, external MCP, installed App,
Provider or real credential was used. The default production Broker remains
unconfigured and publishes no remote capability. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal. Evidence:
`.loom-evidence/phase2d/P2D-W2D-remote-tool-backend-enrollment-v30.md`.

`CURRENT / P2D-W2D REMOTE TOOL ENROLLMENT CLIENT GOVERNANCE V31 SOURCE
VERIFIED (2026-08-15)`: the native Provider Account surface now consumes the
V30 authority instead of treating remote tools as hidden daemon state. Strict
Swift commands and result decoders cover exact account-scoped configure and
revoke operations, reject unknown or secret-shaped fields, preserve immutable
adapter/endpoint/server identity, and safely reject invalid revision bounds.
The concrete UDS client sends only the closed non-secret wire keys and requires
the authoritative response to echo the requested Provider Account and
Enrollment lineage.

`LocalProductStore` permits edits only for an Enrollment already present in the
current authoritative setup projection. It refreshes and verifies the returned
revision after each operation, keeps conflict/stale/not-found/unavailable
failures local to that Enrollment, and exposes stage, retryability and Incident
ID without taking setup or the Team offline. The Provider Account policy sheet
now shows Web Search/MCP state, limits and policy drift; its native editor can
change bounded limits and MCP allowlists, rebind a stale record, restore a
revoked record, or revoke an active record after explicit confirmation. Failure
states offer Retry, diagnostic preview and Incident ID copy.

The UI intentionally has no arbitrary Adapter ID, raw endpoint, dynamic plugin,
or new-backend form. Trusted built-in backend candidates and runtime client
materialization do not yet exist, so inventing those values in Swift would not
be an executable or secure configuration path. The complete macOS package
suite passes (`226` XCTest cases with `1` intentional visual-export skip plus
`12` Swift Testing contracts), as do the exact Go-to-Swift setup contract,
focused daemon Enrollment wire tests and diff checks. This is `SOURCE VERIFIED /
TRUSTED NEW ENROLLMENT, AGENT PREFLIGHT, RUNTIME MATERIALIZATION AND INSTALLED
LIVE OPEN`. No App bundle, network, external MCP, Provider, real credential or
user workspace was used; default production still publishes no remote
capability. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Evidence:
`.loom-evidence/phase2d/P2D-W2D-remote-tool-enrollment-client-governance-v31.md`.

`CURRENT / P2D-W2D AGENT REMOTE TOOL ENROLLMENT BINDINGS V32 SOURCE VERIFIED
(2026-08-16)`: the ExecutionProfile -> Agent-selected Enrollment -> Preflight ->
Frozen Attempt Binding -> Work Bundle materialization chain is now source
complete. Each per-Agent ExecutionProfile and FrozenExecutionBinding may carry
an optional both-or-neither Enrollment ID/digest pair in a new explicit digest
domain, so retry, replay and recovery reject Enrollment drift. The setup
builder accepts `none` or `<enrollment_id>:<digest>` per Agent, validates it
against the authoritative projection, and clears the selection when the
Provider Account route changes. Per-Agent preflight blocks revoked,
policy-drifted, account-mismatched, digest-conflicted, adapter-unsupported or
unavailable Enrollments with closed safe codes and never relabels peers or the
Team offline.

A trusted built-in backend candidate registry (`builtin.search.deepseek.v1`,
`builtin.mcp.stdio.v1`) is the only source of materializable Adapter IDs, and
the Work Bundle materialization boundary (`internal/toolbroker/enrollment`)
is the only path from a persisted, active, policy-current Enrollment to a typed
`execution.RemoteToolExecutor` with the exact allowlist and bounded limits;
unknown adapters, revoked records, drifted policy and missing typed ports fail
closed. The native Agent editor gains a per-Agent "Remote tools" picker (active
+ policy-current Enrollments or none) with strict Swift models that reject
one-sided Enrollment pairs and unknown fields. No real Search/MCP transport,
App bundle, network request, Provider, real credential or user workspace was
used; default production still publishes no remote capability. Verification:
focused/package/race/vet/gofmt/diff plus the complete daemon suite, the full
Go repository suite (only the recorded pre-existing full-load harness flake,
which passes in isolation) and the complete macOS package suite (`227` XCTest
cases with `1` intentional visual-export skip plus `15` Swift Testing
contracts). Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Evidence:
`.loom-evidence/phase2d/P2D-W2D-agent-remote-tool-enrollment-bindings-v32.md`.

`CURRENT / P2D-W2D LIVE ACCEPTANCE READINESS V33 (2026-08-16)`: every remaining
Phase 2D completion gate is now operator-executable. The installed-live runbook
and matrix (`G1` installed credential import/CV6, `G2` real conversation, `G3`
mixed Provider Team ATL9, `G4` single-Agent failure isolation matrix, `G5`
installed Web/MCP diagnostics, `G6` per-Account accounting/governance UI) with
per-gate steps, pass criteria and evidence rules live at
`.loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md`. A safe source
library (`scripts/phase2d-live-acceptance.sh`) checks installed App/daemon
prerequisites, prints a PASS/OPEN summary and exports bounded `0600`
privacy-safe gate evidence without ever touching credentials, secrets, Prompts
or Provider bodies; `scripts/test-phase2d-live-acceptance.sh` validates it in a
private temp dir and `scripts/test-build-loom-local-app.sh` re-confirms the
native App build fixture PASS. The six gates remain OPEN: executing them
requires an installed App, real Provider credentials, runtimes and explicit
operator approval; no credential or secret enters source, scripts, logs or
evidence. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal. Evidence:
`.loom-evidence/phase2d/P2D-W2D-live-acceptance-readiness-v33.md`.

`CURRENT / P2D-W2D FAILURE ISOLATION SOURCE MATRIX V34 (2026-08-16)`: the
consolidated source-level single-Agent failure isolation matrix is now a
first-class test. `TestPhase2DPerAgentFailureIsolationMatrix` drives a
four-Provider Team and injects, per cell, one runtime failure on exactly one
sub-agent — `provider_rate_limited`, `provider_timeout`,
`provider_insufficient_balance`, `credential_unavailable` — asserting the
affected Agent's Attempt fails with the exact closed reason and the node is
recovery-blocked while every peer sub-agent and the main Agent succeed with no
inherited failure. The Team result is `blocked` only because aggregation waits
on the failed node; healthy peers are never relabelled. Preflight-block classes
(credential revision conflict, Enrollment revoked/policy drift) remain proven
by the V32 preflight tests and W2C binding tests. Complete `internal/app`
package, race on the matrix/four-Provider tests, vet, gofmt, diff and
`go build ./...` pass. The installed-live G4 matrix remains operator-executable
via the V33 acceptance runbook. Evidence:
`.loom-evidence/phase2d/P2D-W2D-failure-isolation-source-matrix-v34.md`.

`CURRENT / P2D-W2D PRODUCTION WORK BUNDLE MATERIALIZATION V35 (2026-08-16)`: the
default production daemon composition now consumes persisted Remote Tool Backend
Enrollments through the trusted Work Bundle materialization boundary, closing
the last open "runtime client materialization" source gap. `GlobalReadView`
gains a sorted all-Enrollment catalog accessor; the daemon materializes only
active + policy-current Enrollments whose Adapter ID is in the trusted built-in
catalog and whose typed ports are available, skipping revoked/drifted/
unsupported/port-less records per-Enrollment (their bound Agents are already
preflight-blocked) so no capability is ever exposed accidentally. A composite
remote tool executor fans proposals over the materialized set, zeroes content if
the shared lifecycle closes mid-call, and its Close is idempotent; the factory
combines the injected broker and enrollment executors under joined close
effects. Default production (no Enrollment, no injected ports) remains
remote-tool unavailable. Verification: focused materializer tests (trusted-only
materialization, port-less nil, post-revoke nil, idempotent close), race on the
materializer/broker tests, the complete daemon suite, the complete projection
suite (with catalog assertions), vet, gofmt, diff, build and a full `go test
./...` re-run with no new failures. Evidence:
`.loom-evidence/phase2d/P2D-W2D-production-work-bundle-materialization-v35.md`.

`CURRENT / P2D-W2D OPENCODE PROVIDER V36 SOURCE VERIFIED (2026-08-16)`: OpenCode
is now a first-class Loom Provider in the catalog (`opencode`, native_auth /
native_runtime, protocol `opencode_agent`) with
`conversation-opencode-default-v1`, plus a bounded native conversation client
and the real OpenCode 1.18 JSON event decoder (`message.part.updated` text
parts, `session.idle`, `auth.error`/`session.error` fail-closed). The system
runner spawns `opencode run --format json --pure --model <provider/model>` with
private TMPDIR, real HOME for OpenCode's own auth, process-group cancellation
and executable identity checks; prompts and conversation content are never
persisted. Tests: complete `internal/provider` (client bounded/fail-closed +
event-stream decoder fixtures + catalog), race, harnessadapter/loomd focused,
vet, gofmt, diff, build and a full `go test ./...` with no new failures. The
daemon wiring (conversation profile/router responder, runtime discovery and the
team-attempt harness adapter) is the next slice V37 and should land with live
validation against the real OpenCode CLI. Evidence:
`.loom-evidence/phase2d/P2D-W2D-opencode-provider-v36.md`.

`CURRENT / P2D-W2D OPENCODE MULTI-MODEL ADAPTATION V37 SOURCE VERIFIED
(2026-08-16)`: OpenCode is now a fully adapted multi-model Provider. Loom
Provider + Model bindings map to OpenCode's `provider/model` identity
(`OpenCodeModelIdentity`, OpenRouter gateway pass-through, cross-Provider
models fail closed) and the bound key is injected through the exact env
variable OpenCode 1.18 reads per Provider (`OpenCodeCredentialEnv`, grounded
in the installed binary: OPENAI/ANTHROPIC/DEEPSEEK/MOONSHOT(kimi)/MINIMAX/XAI/
ZHIPU/STEPFUN/OPENROUTER/GOOGLE_GENERATIVE_AI/DASHSCOPE/OLLAMA/LMSTUDIO).
A team-attempt Harness adapter (`opencode_adapter.go` + `opencode_process.go` +
`system_opencode.go`) runs `opencode run --format json --pure --model
<provider/model>` with the system prompt prepended, shared JSON event decoding,
and unobserved accounting; daemon wiring adds `--opencode-executable`, the
`runtime.opencode.local` discovery observation, attempt-loop adapter
construction, the `productOpenCodeConversationResponder`, the
`conversation-opencode-default-v1` native binding in the profile router, and
the OpenCode conversation profile in the setup snapshot (Swift accepts the
generic protocol/adapter). Verification: complete provider + harnessadapter
(+race) + app + daemon suites, vet/gofmt/diff/build, full `go test ./...` and
the complete macOS package suite with no new failures. Live validation against
real OpenCode Providers remains operator-driven. Evidence:
`.loom-evidence/phase2d/P2D-W2D-opencode-multimodel-adaptation-v37.md`.

`CURRENT / V37 LIVE DISCOVERY (2026-08-16)`: the rebuilt App daemon now
discovers `runtime.opencode.local | opencode | online` alongside Loom Native
(deepseek-chat) and Pi, and `opencode providers list` confirms the environment
supplies `DEEPSEEK_API_KEY`, `MINIMAX_API_KEY` and `ZHIPU_API_KEY` (existence
only) — the exact env variables the V37 harness adapter injects from a Loom
lease. DeepSeek / MiniMax / Zhipu OpenCode bindings are live-ready; a paid
`opencode run` validation is intentionally deferred pending explicit approval.

`CURRENT / P2D-W2D CONVERSATION THREE-LAYER SELECTION AND CHAT UX V38 SOURCE
VERIFIED (2026-08-16)`: conversation selection in the App is now three
dependent layers — Provider -> Model -> Reasoning Effort — mirroring the
DeepSeek model/reasoning separation. The server owns the contract:
`provider/conversation_catalog.go` (models per Provider, reasoning efforts per
model, OpenCode dynamic `provider/model`), `ModelID`/`ReasoningEffort` on the
chat request and conversation request, `RespondConfigured` on the
OpenAI-compatible/Anthropic/OpenCode clients (per-request model,
`reasoning_effort`/`--variant` mapping, fail-closed validation), and router
validation before the credential lease. The client renders Provider/Model/
Reasoning pickers with dependency filtering (client catalog mirrors the Go
catalog; server stays authoritative) and sends the selection per message.
Chat UX fixes: per-message Copy button + context menu, and Enter-to-send via
`onSubmit`. Verification: complete provider/api/app/daemon suites, vet, gofmt,
diff, build, full `go test ./...`, and the complete macOS package suite; the
App was rebuilt/reinstalled and the daemon serves with OpenCode, Loom Native
and Pi online. Paid conversation turns on selected model/reasoning combinations
remain operator-driven. Evidence:
`.loom-evidence/phase2d/P2D-W2D-conversation-three-layer-ux-v38.md`.

`CURRENT / V38 FOLLOW-UP FIXES (2026-08-16)`: incident
`loom-chat-d69fd342-aff2-44ad-b4dc-e8e62d6e39e1`
(`input_admission / invalid_request`, OpenCode profile) was traced to the Swift
client pre-validating `model_id` with `validIdentifier`, which rejects `/` in
OpenCode identities; the client now uses `validModelID` (slash allowed) with a
regression test. The Provider picker no longer carries a model —
`conversationProfileMenuLabel` shows Provider + Account only, Model stays a
separate layer (label test updated). The chat send protocol requires the
context-carrying 7-parameter method alongside the 8-parameter
model/reasoning method, with extension defaults delegating correctly (store
mocks + unavailable client conform without recursion). Full macOS package suite
green (`227` XCTest + `15` Swift Testing); App rebuilt/reinstalled and the
daemon serves with OpenCode / Loom Native / Pi online.

`CURRENT / V38 FOLLOW-UP (DEFAULTS + CODEX MODELS + VAULT-LOCK) (2026-08-16)`:
incident `loom-chat-1ea6e228-...` (`vault_encrypt / state_unavailable /
retryable`) is a locked Credential Vault after App restart — the operator must
Unlock the vault for conversation context-capsule encryption (not a routing
bug). Codex now has a model picker: the conversation model catalog gained the
`openai` Provider (`codex-default` / GPT-5.5 Codex) in both Go and Swift. All
three selectors always carry a default: `effectiveConversationModelID` falls
back to the selected profile's model and
`effectiveConversationReasoningEffort` falls back to `medium` (or the first
supported effort) when the model supports reasoning; model selection resets
reasoning to the default and the reasoning menu adds an explicit "Provider
default" option. Regression tests added; full macOS package suite green
(`228` XCTest with `1` intentional skip + `15` Swift Testing); App rebuilt /
reinstalled and the daemon serves with OpenCode / Loom Native / Pi online.

`CURRENT / V38 FOLLOW-UP (CODEX MULTI-MODEL) (2026-08-16)`: Codex now offers
more than gpt-5.5-codex. Loom's Codex conversation path previously used
`--ignore-user-config` through the OpenAI gateway (hence only gpt-5.5-codex);
the user's Codex is actually configured via cc-switch with a local custom
provider (127.0.0.1:15721, wire_api responses) serving DeepSeek V4 Flash / Pro.
The `openai` conversation model catalog now lists `codex-default` /
`gpt-5.5-codex` (gateway) plus `deepseek-v4-flash` / `deepseek-v4-pro`
(native, reasoning none/high grounded in the cc-switch catalog);
`CodexConversationClient.RespondConfigured` + system runner add a native mode
(no `--ignore-user-config`, `--model <slug>`,
`-c model_reasoning_effort="<effort>"`) while the gateway mode stays unchanged,
and the daemon codex responder forwards the three-layer selection. Tests:
native-vs-gateway args + expanded catalog; complete Go and macOS package suites
green; App rebuilt/reinstalled with OpenCode / Loom Native / Pi online. Live
Codex turns still require the Credential Vault to be Unlocked.

`CURRENT / V38 FOLLOW-UP (VAULT-LOCK CHAT RECOVERY) (2026-08-16)`: incidents
`loom-chat-34f593b4-...` and `...-1ea6e228-...` are `vault_encrypt /
state_unavailable / retryable=true` — the Credential Vault is locked after App
restart, so conversation capsule encryption fails. The chat failure banner hid
the vault recovery action because `conversationVaultRecoveryAvailable` gated on
`!recoverable` and omitted `.vaultEncrypt`. Fixed: vault-stage failures
(including `.vaultEncrypt`/`.vaultCommit`) always offer recovery even when
retryable, and the banner now has a direct **Unlock Vault** button (passphrase-
free LocalKeyFile unlock) plus "Open Credential Vault"; tests added/updated;
full macOS package suite green; App rebuilt/reinstalled with OpenCode / Loom
Native / Pi online.

`CURRENT / V38 FOLLOW-UP (CODEX GPT CATALOG CORRECTED) (2026-08-16)`: the
`openai` conversation model catalog was corrected to the real Codex CLI 0.144.1
catalog (`gpt-5.5` default, `gpt-5.5-pro`, `gpt-5.4`, `gpt-5.4-mini`,
`gpt-5.2`, `gpt-5.1-codex-max`, `gpt-5.6-terra/sol/luna`, `o3`, plus
`codex-default` alias and native `deepseek-v4-flash/pro`). Native mode now keys
on the `deepseek-v4-` prefix; GPT models run through the user's Codex OpenAI
auth with an explicit `--model`. Go + Swift catalogs and tests updated; full Go
and macOS package suites green; App rebuilt/reinstalled with OpenCode / Loom
Native / Pi online.

`CURRENT / V38 FOLLOW-UP (AUTO-UNLOCK VAULT ON SEND) (2026-08-16)`: incident
`loom-chat-74849682-...` was the locked-Vault wall again. A direct daemon probe
proved `credential_vault_unlock` succeeds passphrase-free, so the blocker was
UX. `LocalProductStore.sendChatMessage` now auto-unlocks the vault first when
locked (LocalKeyFile, no passphrase), so a locked vault after App restart never
blocks a conversation; the chat failure banner keeps the explicit Unlock Vault
action. macOS package suite green; App rebuilt/reinstalled with OpenCode / Loom
Native / Pi online.

`CURRENT / V38 FOLLOW-UP (STALE-PROFILE CHAT RECOVERY) (2026-08-16)`: incident
`loom-chat-d0838d99-...` (`conversation_dispatch / invalid_request`) happened
after the vault was unlocked. Live probes proved the vault was unlocked, the
DeepSeek r6 profile existed in the authoritative snapshot, and a chat with the
real thread + profile succeeded — the 22:51 failure was transient stale-state
during a DeepSeek credential re-import. `LocalProductStore.sendChatMessage`
now refreshes the setup snapshot and retries once with the current profile on
`invalid_request / conversation_dispatch` before surfacing the failure. macOS
package suite green; App rebuilt/reinstalled with OpenCode / Loom Native / Pi
online.

`CURRENT / V38 FOLLOW-UP (REPEATED ROUTE-TRANSITION SHEET FIXED) (2026-08-16)`:
after switching the conversation Provider, the "Change conversation route"
sheet re-appeared on every send because the switch branch of
`confirmConversationRouteTransition` never set `forceNewConversationSegment`
(thread kept the old profile → pre-send `requestConversationDispatchTransition`
returned a fresh transition each time). Confirming a switch/rebind now sets
`forceNewConversationSegment = true` and clears the pending-route state, so the
next send proceeds with a new segment and the sheet never re-pops; regression
test added. macOS package suite green; App rebuilt/reinstalled with OpenCode /
Loom Native / Pi online.

`CURRENT / V38 FOLLOW-UP (CONVERSATION CONFLICT SELF-HEAL) (2026-08-16)`:
incident `loom-chat-7404eed7-...` was `conversation_dispatch / conflict /
retryable` on deepseek-r6: the server rejects with conflict when the thread's
last segment binding differs from the current resolved binding and the request
lacks ExpectedExecutionBinding + ContextMode; live probes confirmed the current
daemon accepts the sends (transient stale-binding during credential
re-import). `LocalProductStore.sendChatMessage` now self-heals conflict by
aligning to the current profile binding, forcing a new segment, and retrying
once; a failed retry resets state so the normal route-transition sheet stays
available. Regression + updated tests; macOS package suite green; App
rebuilt/reinstalled with OpenCode / Loom Native / Pi online.

`CURRENT / P2D-W2D OPENCODE LIVE CONVERSATION E2E + USER-VISIBLE FAILURE CODES
V39 (2026-08-16)`: the installed App now completes a real OpenCode conversation
end-to-end through the daemon and surfaces every failure with an actionable,
non-technical message.

- **Live OpenCode E2E (real paid calls)**: `TestLiveOpenCodeConversationE2E`
  (gated by `LOOM_LIVE_OPENCODE_E2E=1`) sends
  `Reply with exactly: E2E-OK` on the opencode profile
  (`conversation-opencode-default-v1`, model `deepseek/deepseek-chat`,
  binding `{SchemaVersion:3, ProviderID:"opencode"}`, `ContextModeStartClean`)
  through the installed daemon UDS socket and asserts the thread's final loom
  message is exactly `E2E-OK`. Passes repeatedly against the rebuilt App with
  OpenCode / Loom Native / Pi online.
- **Root cause fixed (V39)**: the OpenCode responder was constructed before
  `leases` was resolved, so it captured a nil `productCredentialLeaseAccess`
  and fell back to native auth with no injected Provider key; OpenCode then
  failed model resolution with `ProviderModelNotFoundError` and the router
  surfaced `provider_unavailable / provider_connect`. Responder binding now
  happens after `leases` (vault) is resolved, so the bound model's Provider
  credential (e.g. `DEEPSEEK_API_KEY`) is leased from the Credential Vault and
  injected into the OpenCode process env. Verified by in-process runner,
  lease, and daemon E2E probes.
- **Routing fixed (V39)**: `conversation-opencode-default-v1` now resolves a
  valid Context Capsule target (`opencode / opencode-default / native_auth /
  context:loom-native:v1`) and routes to the default responder (previously the
  profile fell through `resolveBrokeredProfile` → `state_unavailable /
  vault_encrypt`). Unit tests cover Respond routing, binding resolution, and
  context-target resolution for the OpenCode profile.
- **User-visible failure codes (V39)**: the App maps every chat failure
  code/stage to a non-technical, actionable title + detail + incident ID.
  New cases: `state_unavailable+vault_encrypt` ("Conversation context could
  not be secured … Unlock the vault or re-verify the Provider credential"),
  `state_unavailable+vault_key_load/vault_open/vault_commit`,
  `provider_unavailable+provider_connect` ("Conversation Provider could not
  start … Open Runtime & Providers"), `conversation_unavailable+
  conversation_dispatch`, and `invalid_request+conversation_dispatch`.
  Regression tests assert the new presentation.
- **OpenCode decoder**: `opencode run --format json` event stream decoding
  accepts the real 1.18.3 `text` / `step_finish` / `session.idle` shape (plus
  legacy `message.part.updated`); verified against live CLI output.
- **Verification**: full Go suite green (`go test ./...`), macOS package
  `231` tests executed, `0` failures (`1` visual-export skip by design), live
  OpenCode E2E `E2E-OK`, `git diff --check` clean. Evidence:
  `.loom-evidence/phase2d/P2D-W2D-opencode-live-conversation-v39.md`.

`CURRENT / P2D-W2D OPENCODE DEFAULT MODEL + MULTI-CONVERSATION SESSIONS V40
(2026-08-16)`:

- **Actionable `provider_auth` (incident `loom-chat-81e0ea64-...`)**: the App
  defaulted the OpenCode profile to `openai/gpt-5.5`, but the Vault only had a
  verified DeepSeek account, so OpenCode ran without a key and the user saw an
  opaque failure (and, earlier, an untracked native-auth reply). Fixes:
  - `setupConversationProfiles` now picks the OpenCode profile default model
    from the first catalog model whose Provider has a verified Loom account
    (`deepseek/deepseek-chat` when only DeepSeek is verified; falls back to
    the catalog default when none is).
  - The OpenCode responder now fails fast with
    `provider_auth / provider_connect` and a user message ("The selected model
    requires a verified <provider> Provider credential…") when the selected
    model's Provider has no verified Vault credential, instead of silently
    running OpenCode natively.
  - The conversation router preserves the responder's dispatch failure info
    (code/stage/message) instead of collapsing it to a generic
    `provider_unavailable`.
  - Swift shows "Model Provider credential required — the selected model
    belongs to a Provider that has no verified Loom account…".
- **Multi-conversation sessions**: the App now supports distinct conversations.
  `LocalProductStore` keeps a `chatSessions` registry (thread id, auto-title
  from the first user message, timestamps) persisted to
  `~/Library/Application Support/Loom/chat-sessions.json` (file-based; the App
  core avoids UserDefaults per the native-app source gate). `newConversation()`
  creates a fresh thread, `selectChatSession(_:)` switches, the workspace
  thread anchor follows the active session, and the chat header shows a
  conversation switcher menu plus a New Conversation button.
- **Verification**: Go suite green (incl. dynamic-default + responder
  `provider_auth` + router-preservation tests); macOS package `234` tests, `0`
  failures (`1` visual-export skip); live OpenCode E2E still `E2E-OK`; the
  installed App's header shows the session switcher and the registry file is
  written. Evidence:
  `.loom-evidence/phase2d/P2D-W2D-opencode-default-model-and-sessions-v40.md`.

`CURRENT / V41 FOLLOW-UP (VISIBLE CONVERSATION SESSIONS) (2026-08-16)`: the
App now makes conversations visibly distinct: the navigation rail shows a
"CONVERSATIONS" section listing every session (title + relative time, active
session highlighted with a checkmark) with a New Conversation button, in
addition to the header switcher menu. Sessions persist across App restarts via
`~/Library/Application Support/Loom/chat-sessions.json`; a new test proves a
fresh store instance (simulated relaunch) restores all sessions and the
previously selected one. macOS package `235` tests, `0` failures (`1`
visual-export skip); live OpenCode E2E still `E2E-OK`; App reinstalled and
running.

`CURRENT / V42 FOLLOW-UP (OPENCODE MODEL CATALOG CORRECTED) (2026-08-16)`: the
OpenCode model configuration was wrong versus the installed OpenCode CLI
1.18.3 (`opencode models` + models.dev cache), and CCswitch's OpenCode
integration confirmed the correct provider/model shape:
- `zhipu/glm-4.5` was the wrong provider prefix — the CLI exposes GLM under
  `zai/` (`zai/glm-4.5`, `zai/glm-5.2`, …) with `ZHIPU_API_KEY`.
- `openai/gpt-5.5` is not usable through the OpenCode CLI in this environment
  (no openai provider / key) and is removed from the OpenCode catalog (the
  Codex profile still covers OpenAI models).
- Missing real CLI models added: `deepseek/deepseek-reasoner`,
  `deepseek/deepseek-v4-flash`, `deepseek/deepseek-v4-pro`,
  `minimax/MiniMax-M2.7`, `zai/glm-4.5`, `zai/glm-5.2`,
  `opencode/deepseek-v4-flash-free`.
- Reasoning efforts now follow each model's real capability
  (v4-flash/v4-pro → low/high/max, glm-5.2 → high/max, toggle-only models →
  none) instead of a fake `low/medium/high/minimal` for every model.
- `OpenCodeConversationDefaultModel` is now `deepseek/deepseek-chat` (a real
  CLI model) and `OpenCodeCredentialEnv` maps `zai` → `ZHIPU_API_KEY` and
  `opencode` → `OPENCODE_API_KEY`.
- Verified live: `deepseek/deepseek-v4-flash` returns `FLASH-OK` through the
  CLI; OpenCode E2E still `E2E-OK`; Go + macOS `235` tests green.

`CURRENT / V43 FOLLOW-UP (CODEX ROUTING + USAGE-LIMIT MESSAGE) (2026-08-16)`:
"Conversation Provider runtime is unavailable." on the Codex profile was two
bugs:
- The Codex profile was served by the OpenCode responder whenever both runtimes
  were configured (the router had a single default-responder slot and the Codex
  client was only built when OpenCode was absent). The router now carries a
  dedicated `codexResponder` and routes `conversation-openai-codex-default-v1`
  to the Codex responder even when OpenCode is the default conversation
  runtime; unit test `TestConversationProfileRouterRoutesCodexToCodexResponder`
  locks this in.
- The official OpenAI Codex account has exhausted its credits, so
  `codex-default` (gateway/`--ignore-user-config`) fails with "You've hit your
  usage limit". The Codex runner now classifies stderr
  (`usage limit/credits/quota` → usage-limit, auth markers → auth) and the
  client propagates those typed errors; the responder maps them to
  `provider_insufficient_balance` / `provider_auth` with a clear Chinese
  message ("Codex 官方账号用量已达上限…可切换 DeepSeek V4 使用你的
  cc-switch 配置"). Swift shows "Codex account usage limit reached".
- The user's working Codex path (cc-switch local proxy → DeepSeek) now works:
  selecting `deepseek-v4-flash` on the Codex profile runs native auth
  (no `--ignore-user-config`) and returns a real model reply.
- Verified live: `codex-default` → clear usage-limit message;
  `deepseek-v4-flash` → real reply; OpenCode E2E still `E2E-OK`; Go + macOS
  `236` tests green.

`CURRENT / V44 FOLLOW-UP (PROVIDER/MODEL/EFFORT MATRIX + OFFICIAL CATALOGS + DELETE
CONVERSATION) (2026-08-16)`: ran the full Provider/Model/Reasoning-effort matrix
live on the installed App and grounded every catalog in each Provider's official
docs/API:

- **Official catalog verification**: OpenAI/Codex grounded in the installed Codex
  CLI 0.144.1 `~/.codex/models_cache.json` (replaced invented gpt-5.5-pro/gpt-5.2/
  gpt-5.1-codex-max/o3; added per-model reasoning levels); DeepSeek grounded in
  api-docs.deepseek.com + live API (deepseek-v4-flash/v4-pro with low/high/max);
  OpenCode grounded in the installed CLI 1.18.3 models.dev cache (v4-pro high/max
  corrected, glm-5.2 high/max); Kimi kimi-k3 low/high/max added; MiniMax and
  Anthropic confirmed toggle/adaptive-only. Swift catalog mirrors Go.
- **Live matrix E2E**: `TestLiveProviderModelEffortMatrixE2E` (gated by
  `LOOM_LIVE_MATRIX_E2E=1`) drives the installed daemon across every selectable
  Provider/Model/effort (Codex native DeepSeek V4, OpenCode deepseek/minimax,
  DeepSeek brokered v4-flash/v4-pro low/high/max + legacy, MiniMax M3),
  mid-conversation Provider/Model/effort switching, and multi-session isolation —
  all cells pass with real replies (MiniMax classified rate-limit on repeat runs).
- **Two real bugs fixed by the matrix**: (1) the 128-thread store cap returned an
  opaque error with no recovery — now a typed `conversation_limit` with an
  actionable Swift message plus a bounded encrypted delete-conversation capability
  (`chat_thread_delete` route, `DeleteThread`, vault document+capsule delete,
  Swift client/store/UI); (2) continuing a long thread failed after ~5 turns with
  an opaque "conversation unavailable" because the context capsule dispatch payload
  (allowed up to 32 KiB) exceeded the provider clients' 4096-byte per-message cap —
  the per-message wire cap now matches the adapter allowance, with a regression
  test.
- **Verification**: Go suite green (only `internal/runtime/harnessadapter`
  child-process tests are flaky under full parallel load, 3/3 pass in isolation,
  unrelated); macOS `236` tests, `0` failures (`1` visual-export skip); live matrix
  E2E PASS; OpenCode E2E `E2E-OK`; `git diff --check` clean. Evidence:
  `.loom-evidence/phase2d/P2D-W2D-provider-model-effort-matrix-v44.md`.

`CURRENT / V45 FOLLOW-UP (TEAM CREATION + SIDEBAR FRICTION REDUCTION) (2026-08-16)`:
- **Form-first blank Team builder**: `BuilderSourceBlank` now presents the full
  editable draft immediately (default Main + SubAgent roles pre-selected) instead
  of forcing the 4-step sequential Q&A (team_name → purpose → main_role →
  subagent_role). Only the two required fields (name, purpose) gate confirmation;
  explicit confirmation semantics are unchanged. Verified live on the installed
  daemon: blank start returns no question + 2 default roles, and two inline
  `builder_edit` calls flip `can_confirm=true`.
- **Inline Name/Purpose editors**: the Team builder panel renders editable "Team
  name" and "Bounded purpose" fields (commit via editBuilder) so users fill the
  required fields in place and confirm.
- **Sidebar "AGENT TEAMS" rail section**: confirmed Teams are listed in the nav
  rail (click to open), with an empty state and a "New Agent Team" button that
  opens Teams and starts a blank draft in one action.
- **Multica research resolved**: the earlier "no network access" claim was wrong
  for this environment — web search, GitHub API and docs fetch all work.
  Live-verified `multica-ai/multica` (~46k stars, 20 agent CLIs, Apache 2.0 +
  conditions, 4 trigger methods, creation requires only Name + Runtime), which
  the form-first blank builder now mirrors.
- **Verification**: Go `internal/app` + `cmd/loomd` full suites green; macOS
  `236` tests, `0` failures (`1` visual-export skip); App rebuilt/reinstalled;
  `git diff --check` clean. Evidence:
  `.loom-evidence/phase2d/P2D-W2D-team-sidebar-friction-v45.md`.

`CURRENT / V46 FOLLOW-UP (NEW MISSION CLARITY) (2026-08-16)`: the sidebar "New
Mission" action was a dead end without a confirmed Agent Team (empty Team
picker, unusable form, no guidance). The New Mission sheet now explains in plain
language what a Mission is, and when no executable Team exists it shows a
"Create Agent Team first" guided empty state with a one-click button that opens
Teams and starts the blank Team builder. The full form only appears once an
executable Team exists. Verified: macOS `236` tests, `0` failures (`1` visual-
export skip); live snapshot shows `saved_teams=0` so the guidance path is what
users see; App rebuilt/reinstalled; `git diff --check` clean. Evidence:
`.loom-evidence/phase2d/P2D-W2D-new-mission-clarity-v46.md`.

`CURRENT / V47 FOLLOW-UP (LOWER-FRICTION QUICK START) (2026-08-16)`: the empty
conversation now shows a quick-start guide with live status — "Chat is ready
with <Provider>" (or "Connect a Provider"), "1. Open Folder", "2. Create an
Agent Team when needed" — each with a checkmark once the prerequisite is met, so
a first-run user sees the exact next step. Proposal messages now carry a
**Create Agent Team** button (opens Teams + blank builder) instead of the
"press u" hint. Verified: macOS `236` tests, `0` failures (`1` visual-export
skip); live snapshot shows DeepSeek ready + no Team, so the guide renders the
"Chat is ready with DeepSeek / create a Team when needed" state; App rebuilt +
reinstalled; `git diff --check` clean. Evidence:
`.loom-evidence/phase2d/P2D-W2D-lower-friction-quickstart-v47.md`.

`CURRENT / V48 FOLLOW-UP (CHAT → MISSION MERGE) (2026-08-16)`: Mission is now
reachable directly from the conversation. A Loom proposal message shows a
**Run as Mission** button (when an executable Team exists) that pre-fills the New
Mission objective from the proposal / latest user message and opens the sheet —
no re-typing and no separate New Mission hunt. `LocalProductStore` now exposes
`executableTeams` (shared with `MissionWorkbench`), and `MissionWorkbench`
accepts `initialMissionObjective` for pre-fill. Verified: macOS `237` tests, `0`
failures (`1` visual-export skip); new test
`testExecutableTeamsExposesOnlyConfirmedRunnableTeams`; App rebuilt +
reinstalled; `git diff --check` clean. Evidence:
`.loom-evidence/phase2d/P2D-W2D-chat-to-mission-v48.md`.

`CURRENT / V49 (MIXED-PROVIDER TEAM LIVE GATE) (2026-08-16)`: drove the
installed daemon through the full Team lifecycle live — verified DeepSeek +
MiniMax broker accounts, built a 4-Agent mixed-provider Team (DeepSeek main +
MiniMax + DeepSeek reviewer + MiniMax researcher) via the form-first builder,
confirmed it (`status=active`, confirmed + executable in snapshot). This
exposed the real G3 installed-live blocker: the installed App daemon builds no
Mission execution runtime (`Execution == nil` because
`missionExecutionConfigFromDaemonBuild` requires `LocalModelCatalog`, and the
installed Pi runtime bundle ships no local GGUF model + llama-server);
`buildProductMissionExecutionAPI` + `newProductMissionExecutor` require the
local model as the primary Pi supervisor adapter, so Mission preflight returns
`state_unavailable` and no TeamInstance is materialized. Added
`TestLiveMixedProviderTeamE2E` (`LOOM_LIVE_TEAM_E2E=1`) as the installed-live
G3 gate that re-verifies accounts, builds+confirms the mixed Team, then runs
preflight+start, skipping by default and failing fast at the precise blocking
stage. G3/G4 source proofs remain green (`TestFourProviderTeam*`,
`TestPhase2DPerAgentFailureIsolationMatrix`). No remote capability is
published by default. Evidence:
`.loom-evidence/phase2d/P2D-W2D-mixed-provider-team-live-gate-v49.md`.
