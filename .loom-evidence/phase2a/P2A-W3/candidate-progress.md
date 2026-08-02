# P2A-W3 Candidate Progress

**Date**: 2026-08-01  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Status**: `IMPLEMENTATION REVIEW PASS / LIVE GATES FAILED / HUMAN_REQUIRED / NO COMMIT`

## Implemented under the frozen boundary

- one strict local IPC method, `mission_execution`, with closed
  `preflight|start|control` operations and strict Go/Swift decoding;
- exact built-in Coding/Knowledge WorkPackage identity, confirmed-Team,
  Runtime/model/auth/capacity, current-view and canonical preflight binding;
- zero-write preflight and explicit Start with a fresh client correlation ID;
- production composition through the accepted TeamCoordinator, Work Authority,
  Grant Authority, Supervisor, Pi adapter, Evidence Store, GlobalReadView and
  TeamExecutionStream; no second writer, scheduler, queue or state authority;
- one bounded daemon-owned in-flight registry, exact cancel, generation fencing,
  concurrent-start single-lineage behavior, terminal-flight reaping and
  read-only reconnect;
- a five-minute server-owned preflight lease, included in the canonical sheet
  and strict Swift wire, so an old digest cannot Start even when the view is
  unchanged;
- exact prepared-decision routing for every closed control action through the
  accepted Decision service, with refresh-current, lineage/generation matching,
  one-match-only selection and no hidden retry;
- restart reconciliation from Journal-visible non-terminal TeamExecution plan,
  semantic digests, workflow paths and built-in recipe identity; unknown recipes
  fail closed before IPC exposure and never guess from display text;
- authorized source/Verifier Frame handling, bounded tentative delivery,
  source plus independent Verifier Evidence and canonical terminal aggregation;
- Timeline evidence digest enrichment only from the matching GlobalReadView
  `evidence/<id>` record, preserving Journal/Projection as the sole truth;
- native New Mission sheet and Bubble Tea journey: visible WorkPackage and Team
  selection, exact preflight review, explicit Start, same-Mission navigation,
  tentative output label, prepared-decision routing and exact cancel without
  typed internal IDs or Provider environment variables;
- strict closed execution statuses, truthful `awaiting_recovery`, bounded
  product service health and canonical empty wire collections;
- the production `LoomLocalAppContractProbe --execution` now performs strict
  preflight/start through the real Go UDS server and rejects unknown fields,
  duplicate keys, null collections and identity drift.
- native, TUI, contract-probe and restart paths now share the rebuildable
  `mission/<team_instance_id>` identity; Go rejects Mission/Team drift before
  any backend, cancel or prepared-decision call.

## Deterministic vertical closure

`TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage` uses a real
SQLite Journal and the production compiler/backend/composition with a
deterministic loopback Pi fixture. It proves:

- Start accepts a fresh correlation ID while remaining bound to the exact
  preflight digest;
- the first authorized source delta is visible as `tentative` while the node is
  running;
- exactly one TeamExecution plan and ready-set dispatch commit;
- exactly two independent WorkItem/Run/Grant/Evidence lineages: source and
  Verifier;
- exactly one successful canonical Team terminal;
- Snapshot and Timeline agree on two Evidence digests and the terminal;
- reconnect performs no write and does not redispatch;
- ten consecutive focused runs pass.

## Final deterministic verification

- focused Go app/API/IPC/daemon/TUI suites: `PASS`;
- focused and repeated concurrency/race suites: `PASS`;
- whole repository serialized `go test`: `PASS`;
- whole repository serialized `go test -race`: `PASS`;
- `go vet ./...`: `PASS`;
- `go mod verify`: `PASS` (`all modules verified`);
- Go format: all W3-owned Go files clean; only four unrelated excluded
  `internal/mcp/sdk/**` files are reported by the repository-wide listing;
- full Swift package: 56 XCTest, 1 explicit opt-in visual-export skip,
  0 failures; 4 Swift Testing checks pass;
- Swift Thread Sanitizer package run: `PASS`;
- Release build of native `LoomLocalApp`: `PASS`;
- `git diff --check`: `PASS`;
- Candidate secret/non-disclosure scan: `PASS`; no raw credential, OAuth,
  Grant, hidden reasoning or sensitive prompt was added.
- Implementation Review 1 findings are repaired inside the frozen owned
  boundary; all deterministic gates above were rerun after the final
  `submit`-operation correction.
- Implementation Re-review 2 identity findings are repaired inside the same
  boundary; the complete deterministic matrix was rerun again after Repair 2.

## Final gate status

The reviewed `Saved-Team Dormant Capacity Amendment` and Repair 3 remain
deterministically complete. Implementation Review 4 returned `PASS` with no
P0/P1. The exact 42-entry source lock is
`03d213d9862ce1f7be95d4547bfaae67893e58a8c98018d6a6cd651bd251c3ce`.

The three independent one-shot live lineages were then consumed without hidden
retry:

- Codex proved the ordinary product surface can observe `Available`, but the
  daemon stopped at `observer_models_timeout` before exact preflight;
- MiniMax displayed the inherited `Verified` state, but the one `Test` action
  committed no new `ProviderCredentialVerified` terminal fact; and
- Pi live-proved one online Runtime plus one confirmed executable saved Team,
  one Main AgentInstance and one retained dormant SubAgent binding. Exact
  Mission preflight then failed closed because live metadata used the
  namespaced model ID `loom-local/qwen...` while the source-locked mission
  binding authority accepts only `qwen...`.

No Mission Start occurred in any lineage. All retained databases are
integrity-valid; the Pi database contains zero WorkItem, Run, Grant, Evidence,
dispatch or TeamExecution fact. Product sockets/locks are cleaned, isolation is
empty, controlled processes are absent and the bounded retained-file secret
scan is clean.

Fresh independent Result-Evidence Reviews returned `PASS` for the trustworthiness
of the failed records with no P0/P1. They do not convert those product failures
into acceptance. The contract-required no-terminal walkthrough is ineligible
because its three live prerequisites did not pass.

P2A-W3 therefore stops `HUMAN_REQUIRED`. Fixing the Pi model-identity boundary
or MiniMax verification lifecycle requires a reviewed complete W3 reopen; it
must not be patched inside the consumed live lineage, split into P2A-W4 or
hidden behind a retry. No Candidate acceptance commit is permitted.

No Event schema or accepted Journal, Projection, Rules, Work, Grant, Evidence,
Supervisor, Runtime adapter or bridge authority file changed.
