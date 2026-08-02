# P2A-W3 Authoritative Terminal and Observer Closure Contract

**Date**: 2026-08-02  
**Status**: `FROZEN / PENDING CONTRACT REVIEW`  
**Risk**: `STRICT`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Parent WorkItem**: unique `P2A-W3 Controlled Execution Experience`  
**New WorkItem**: none; `P2A-W4` does not exist

## 1. Authorization and causal basis

The fresh Product Owner instruction to reprocess the failed P2A-W3 replacement
results authorizes this one governed repair boundary. It does not reinterpret or
retry the consumed lineages.

The independent Result Review accepted the evidence but stopped the product at
`HUMAN_REQUIRED` because:

1. one explicit MiniMax Test produced a local terminal `Unavailable` state but
   no new authoritative Journal fact; and
2. the Pi replacement stopped before the product socket with only the lossy
   fallback `observer_unknown`.

The read-only diagnosis at
`complete-live-failure-diagnosis.md` traces the first failure to the
pre-Provider Secret Store read boundary and proves that the second failure's
causal leaf is unrecoverable from the retained public reason. The two failures
share one product invariant: a user-visible terminal or blocking outcome must
be attributable to one existing authority operation without disclosing private
state.

This contract therefore closes both outcomes together inside W3. It is not a
retry-only, wrapper-only, or single-point Amendment.

## 2. Accepted architecture remains binding

- The Event Journal remains the only state authority. Projection remains a
  rebuildable cache. No second Journal, StateWriter, Projection, Scheduler,
  queue, credential database, or client authority may be introduced.
- ADR-0004 remains binding. Raw MiniMax credentials stay inside the OS Secret
  Store/Credential Broker and never enter process arguments, Journal, UI, logs,
  evidence, prompts, or source control.
- ADR-0010 remains binding. Pi remains behind the accepted Runtime observer,
  Supervisor, Grant, authorized Frame, and Evidence boundaries.
- Existing Event types and payload schema remain unchanged. The existing
  `ProviderCredentialVerified` metadata transition is the only permitted
  credential terminal fact.
- Native and TUI clients continue to use the one private typed product IPC.
- The unrelated resident observer is not signalled, restarted, or modified.
- No final walkthrough, commit, or live claim is permitted until the gates in
  this contract pass in order.

## 3. Boundary A — authoritative MiniMax Test terminal

Every accepted explicit MiniMax Test operation must resolve to exactly one of:

```text
verified | rejected | unavailable | conflict
```

and every non-conflict terminal must be backed by exactly one next-revision
`ProviderCredentialVerified` fact for that operation ID.

Required behavior:

1. The existing strict UUID operation ID remains the sole command/idempotency
   identity. Same-operation recovery returns the immutable prior result without
   a second Secret Store read, Provider call, or append. A different stale
   operation returns conflict.
2. When Secret Store read succeeds, the existing Provider verification behavior
   is unchanged: valid, provider-rejected, timeout, and unavailable results
   append exactly one existing terminal metadata fact.
3. When Secret Store read fails with not-found, denied, unavailable, helper
   failure, timeout, or another closed store error, the Broker must not call the
   Provider. It must commit exactly one existing metadata fact at
   `expected_revision + 1` with status `rejected` and reason `unavailable`.
   This records that the requested verification was unavailable; it must not
   claim that the Provider rejected or observed the credential.
4. The terminal commit uses the same one-second bounded
   `context.WithoutCancel` commit window already used after a Provider
   observation. If the commit itself fails, the operation remains failed and no
   client-only authoritative result may be invented.
5. The returned result and rebuilt setup snapshot must match provider,
   credential reference, revision, status, and reason before Swift renders the
   terminal. Store class, helper output, local paths, raw error text, secret,
   Authorization header, and Provider body remain absent.
6. Existing configure, replace, and revoke rollback semantics are unchanged.

Mandatory causal REDs:

- Broker Secret Store `not_found|denied|unavailable|unknown` failures cause zero
  Provider calls and one unavailable terminal commit with exact command ID,
  reference, revision, UTC time, status, and reason;
- commit failure after a store failure returns closed failure and does not
  fabricate a result;
- same-operation lost-response recovery yields one append total; different
  operation stale revision yields conflict and zero additional work;
- real Swift Client -> Go UDS Server -> LocalProductSetupService -> Broker
  fixture starts from `Verified`, forces a process-store read failure, and
  proves `Testing -> Unavailable`, one new Journal fact, revision `N+1`, and a
  matching refreshed strict Swift snapshot;
- no secret or raw store/helper error occurs in returned wire bytes or evidence.

## 4. Boundary B — complete safe observer attribution

Every expected fail-closed Runtime observation leaf must produce exactly one
stable public reason code. The original error remains internal and no path,
command output, model body, environment, or local identity is rendered.

Required reason families:

```text
observer_probe_factory
observer_probe_candidate
observer_probe_construction
observer_metadata_binding
observer_version_process|timeout|output_limit|stderr|output
observer_models_process|timeout|output_limit|stderr|output|duplicate
observer_inventory
observer_projection
observer_plan
observer_identity_metadata
observer_write
observer_unknown
```

Required behavior:

1. The error-tree classifier walks wrapped and joined errors once and derives
   codes only from typed sentinel identity. String matching is forbidden.
2. One unique known leaf produces its exact allowlisted public code. Repeated
   wrappers of the same typed leaf do not make it ambiguous.
3. Multiple incompatible known leaves, an unknown leaf mixed with a known leaf,
   or an actually unknown leaf remains fatal as `observer_unknown`. Unknown is
   never contained or retried.
4. Only the already accepted uniquely tagged Pi metadata timeout containment
   remains containable. Classification changes do not expand containment,
   retry, compaction, or scheduler behavior.
5. Probe factory wrapping must retain the typed candidate/construction/binding
   leaf so it can be classified more precisely than the generic factory code.
6. Runtime instance/model normalization and status identity failures map to
   `observer_inventory`; invalid write-plan/reconciliation orchestration maps to
   `observer_plan`; daemon identity/time input failures map to
   `observer_identity_metadata`; StateWriter/Journal append and commit failures
   map to `observer_write`.
7. `daemonFailureMessage` accepts only the complete frozen allowlist and emits
   exactly `daemon failed: <reason>` on stderr. It continues to reject unknown
   codes, multiline content, paths, and arbitrary text.

Mandatory causal REDs:

- table-driven unit proof for every frozen code, duplicate same-leaf wrappers,
  joined incompatible leaves, known-plus-unknown, nil, and raw unknown errors;
- complete public allowlist proof in `run.go` plus exact one-line stderr output;
- deterministic loopback Pi 0.82.1 component fixture using a copied SQLite
  state containing the exact prior `RuntimeInstanceDiscovered` shape proves one
  unchanged observation cycle is `none` and keeps the product socket serving;
- the same fixture injects every version/models/process/binding/inventory/plan/
  identity/write failure class and proves exact attribution, no append on
  pre-write failure, and no hidden retry;
- real locked Pi component tests continue to prove exact 0.82.1 metadata and
  local model identity without using a network Provider.

## 5. Boundary C — vertical closure and no terminal illusion

1. A native terminal label is a presentation of the authoritative returned
   result plus refreshed snapshot, never a local replacement for a missing
   fact.
2. A product-daemon startup failure is externally actionable through one safe
   reason code before another live allowance is frozen.
3. The copied Runtime state and saved-Team authority remain immutable during
   deterministic diagnostics. No new Team, Mission, Run, Grant, Frame, or
   Evidence fact is created before explicit Start.
4. No raw Grant, credential, hidden reasoning, per-token output, local path, or
   private command output becomes Journal or evidence.

## 6. Exact owned production and test files

Only these existing files may change after Contract Review PASS:

```text
internal/credentials/credential_broker.go
internal/credentials/credential_broker_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
cmd/loomd/run.go
cmd/loomd/run_test.go
internal/localipc/swift_contract_test.go
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
```

`LocalProductStore.swift` may change only if the causal RED proves that the
existing returned-result plus refresh check cannot present the new
authoritative unavailable fact. Otherwise it remains byte-locked and only its
test file may change.

Governance/evidence may be added only under:

```text
.loom-evidence/phase2a/P2A-W3/
docs/CURRENT.md
```

All Event schemas, StateWriter implementations, Projection reducers,
Credential Keychain implementations, Provider verifier, Supervisor, Grant,
Evidence, Rules, Runtime protocol, Pi RPC bridge, TUI, daemon service-manager,
and P2A-W1/W2 accepted files remain locked.

If a mandatory RED proves that one of those locked authority files must change,
stop `HUMAN_REQUIRED`; do not silently expand this contract.

## 7. Verification gates

The Controller must record exact commands, exit status, and hashes for:

1. contract source lock and independent Contract Review PASS;
2. mandatory RED before behavior change;
3. focused Go credential, daemon, run, localipc/Swift-contract tests;
4. focused Swift core/store tests under normal and Thread Sanitizer modes;
5. repeated focused tests and focused race tests;
6. all owned Go packages, full `go test ./...`, full `go test -race ./...`,
   `go vet ./...`, formatting, module, diff, secret, protocol, and locked-file
   checks;
7. macOS release build plus strict Swift decoder/wire checks;
8. fresh independent Implementation Review PASS against an immutable source
   lock.

Implementation Review must fail for a local-only terminal, false Provider
claim, missing operation idempotency, unsafe reason text, incomplete error-tree
matrix, widened timeout containment, hidden retry, locked-file drift, secret
exposure, or insufficient real Go-UDS-to-Swift evidence.

## 8. Live allowance after Implementation Review PASS only

No live action is currently authorized by this frozen contract. After an
Implementation Reviewer PASS, the Controller may separately freeze fresh
one-shot manifests for at most:

1. one isolated MiniMax explicit Test using the already configured opaque
   credential reference; and
2. one isolated Pi saved-Team preflight plus explicit controlled execution.

Each attempt requires a fresh 0700 root, 0600 pre-created SQLite, independent
manifest, exact artifact/source locks, no retry/compaction, one user action, and
complete postflight. The MiniMax attempt may perform at most one Provider call;
the Pi attempt performs zero network Provider calls and keeps the exact local
model. The unrelated resident observer remains untouched.

Any failed attempt consumes its lineage. No second attempt may be improvised.
The final no-terminal walkthrough, Result Review, and atomic commit require both
replacement attempts to PASS. Otherwise stop `HUMAN_REQUIRED` with evidence.

## 9. Exit condition

This repair is complete only when all deterministic gates, independent
Implementation Review, both newly frozen live results, independent Result
Review, final no-terminal walkthrough, source/evidence lock, and one atomic
P2A-W3 commit PASS.

Until then:

```text
P2A-W3 = HUMAN_REQUIRED / REPAIR CONTRACT PENDING REVIEW
P2A-W4 = DOES NOT EXIST
NO LIVE / NO WALKTHROUGH / NO COMMIT
```
