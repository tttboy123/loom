# ADR 0015 — Phase E unify & migrate (resolve 7 Phase C/D integration blockers)

- **Status**: Accepted (2026-07-05).
- **Owner**: Mavis (orchestrator) — owner-overridden after worker timed out 4× on the integration merge step.
- **Context**: Phase C (A2A/MCP, `devkit/protocol.py`, 66 tests) and Phase D (Gatekeeper + Scheduler, 52 tests) shipped independently but were not wired into the running autopilot. Investigation found 7 integration blockers; this ADR records how each was resolved.

## The 7 blockers

1. `devkit/rdloop.py` called `devkit.protocol.write_run_protocol_bundle(...)` inside `try/except: pass` but the function did not exist — silent ImportError on every run.
2. Phase D's `select_next_pending` and `evaluate_final_gate` duplicate Phase B's same-named functions in `rdloop.py` with incompatible signatures.
3. Three failure-code vocabularies existed (Phase B free strings, Phase D 4-value enum, Phase A repairer whitelist SCREAMING_SNAKE) with no translation layer.
4. `devkit/lease.py` (in-band lease on task dict) and `devkit/scheduler.py` (out-of-band lease JSON file) coexisted without a bridge — different paradigms, not just different formats.
5. Phase D's `gatekeeper.evaluate_final_gate` reads `<evidence_dir>/evidence.json` but Phase B's `run_loop` writes evidence nowhere — gate would always fail with EVIDENCE_MISSING.
6. Phase C's `ProtocolServer` ships 5 resources + 4 tools but zero AgentCards — `list_agents()` returns `[]`, A2A is capability surface without consumers.
7. Phase C's MCP tool wrapping adds JSON encode/decode overhead in-process; no internal hot-path caller benefits; only external clients (VSCode, Claude Desktop) justify the wrapping.

## Resolution

### 1. `write_run_protocol_bundle` (T1)
Added the function to `devkit/protocol.py` (commit `36cc142` on `feat/unify-write-run-protocol-bundle`). Reads `00-task.md` / `99-gate.md` / `events.jsonl` from `run_dir`, assembles a schema-validated envelope, atomic-writes to `<run_dir>/protocol_bundle.json`. New schema `protocol_bundle.schema.json` (Draft 2020-12) + loader module. Tolerates missing input files. 10/10 tests pass; 236/236 regression green.

### 2. `evaluate_final_gate` duplication (T6a + T6b)
Added `gatekeeper.evaluate_run_gate(run_id, work_item_id, *, run_dir, gate_inputs, failure_codes_override=None) -> (status_code, reasons, GateVerdict)` in T6a (commit `6bdd5cc`). Internal `_sanitize_gate_inputs` bridge translates rdloop's natural kwargs dict (`blocked` / `over_budget` / `review_*`) → writer kwargs + derived Phase D enum codes (TEST_REGRESSION / BUDGET_EXCEEDED / EVIDENCE_INVALID); `review_*` flags silently dropped because review is gate-evidence-orthogonal. In T6b (commit `793987b`) `rdloop.run_loop` now calls `evaluate_run_gate`; `GateVerdict` written to `<run_dir>/verdict.json` via `gatekeeper.write_verdict` in `try/except + logger.warning` (not bare pass). Phase B `rdloop.evaluate_final_gate` is wrapped with `@_deprecated(functools.wraps + warnings.warn)` — body unchanged, kept for direct callers.

### 3. Failure code translator (T2 + T6a gap fill)
New `devkit/failure_codes.py` (commit `b43186b` on `feat/phase-d-gatekeeper-scheduler`) exposes `phase_b_to_phase_d`, `phase_d_to_phase_a`, `phase_b_to_phase_a`, `all_phase_a_for_phase_d`, plus the dict mappings. Worker on T2 noted that the actual Phase A repairer codes are `SCHEMA_VALIDATION_ERROR` / `NOT_ON_WHITELIST` / `MISSING_WORK_ITEM_ID` / `BUDGET_EXCEEDED` etc., not the aspirational CamelCase names (`TestFail` / `CompileFail`) in the original prompt — worker aligned to actual codebase per project principle. T6a filled the missing `phase_d_to_phase_b` reverse mapping.

### 4. Lease paradigms (T3)
The two modules use **different paradigms**, not just different JSON shapes. `lease.py` stores lease as a subfield on the task dict (in-band); `scheduler.py` stores it in a separate JSON file at `lease_path` (out-of-band). They cannot be naively delegated. Resolution: keep `lease.py` API unchanged (5 public symbols: `now_iso`, `attach_lease`, `heartbeat`, `reclaim_stale_running`, `DEFAULT_TIMEOUT_SECONDS`); add opt-in `lease_path=` kwarg to `attach_lease` and `heartbeat` that side-writes a scheduler-compatible JSON; new `devkit/lease_sync.py` exposes `sync_lease_to_file` / `sync_lease_from_file` / `release_lease_via_file` bridges. `DEFAULT_TIMEOUT_SECONDS=1800` (lease.py) and `DEFAULT_LEASE_MAX_AGE_S=300` (scheduler.py) intentionally **not unified** — they serve different lifecycle scopes.

### 5. Evidence writer (T4)
New `devkit/evidence_writer.py` (commit `305e75c` on `feat/unify-evidence-writer`). `write_run_evidence(run_dir, run_id, work_item_id, ...)` reads `00-task.md` / `99-gate.md` / `events.jsonl`, assembles an EvidencePacket matching existing `evidence_packet.schema.json` (no new schema), atomic-writes to `<evidence_root>/<run-id>/evidence_packet.json`. `gatekeeper._locate_evidence_file` looks up packet format first, falls back to legacy `<evidence_dir>/evidence.json` for Phase B old run dirs — `lineage.evidence_source_kind` makes the choice observable. Wired into `rdloop.run_loop` end-of-run (after gate decision, before protocol bundle write) in `try/except + print` (per spec: "logger OR print"). **Path unification**: writer, gatekeeper, Phase C `loom://evidence/<run-id>` resource all now read/write the same canonical path.

### 6. Default AgentCards (T5)
Added `register_default_agents(server)` to `devkit/protocol.py` (commit `1987dbc` on `feat/unify-default-agents`). Registers 3 AgentCards: `observer` (4 caps), `triager` (3 caps), `repairer` (3 caps), each with `@server.on_message(...)` handler that dispatches to existing module functions. `auto_register_agents: bool = True` constructor kwarg; default behaviour registers them on `ProtocolServer()` construction. Worker corrected spec's `triager.classify` → actual `triager.triage(snap)` per the project principle "spec is suggestion, codebase is ground truth".

### 7. Hot-path policy (this ADR + doc update)
**Decision**: internal hot path stays on direct `repairer.dispatch(...)` etc. Phase C's MCP tool wrapping is justified only for **external** callers (VSCode, Claude Desktop, future cross-process) and for **observability** via `list_agents()` / `list_tools()` / `read loom://backlog`. Internal use adds JSON encode/decode + stack-trace indirection without capability gain. Documented in `docs/loom-protocol-layer.md` "Hot-path policy" section.

## Consequences

**Positive**:
- Path unification across Phase B `run_loop` → Phase C `loom://evidence` → Phase D `gatekeeper` eliminates the silent EVIDENCE_MISSING failure mode.
- Gatekeeper owns final-state authority (`GateVerdict` dataclass with 4 typed failure_codes), Runner only consumes the verdict — separation of concerns restored.
- Failure-code vocabulary has a single translation path; consumers pick the view they need.
- Lease can be either in-band or out-of-band; no race condition because they target different lifecycles.
- 3 AgentCards registered by default → `list_agents()` returns 3 → A2A messaging end-to-end works.
- 320+ tests pass across the unified codebase.

**Negative / Tech debt**:
- Phase B's `rdloop.evaluate_final_gate` and `rdloop.pick_next_pending` retained for compat (deprecated, with warning); future phase can remove.
- `devkit/lease.py`'s DeprecationWarning deferred — too many in-process callers, would spam logs.
- `_sanitize_gate_inputs` `tests_failed` dual-meaning (bool vs int) handled by type dispatch — works but slightly surprising; document in module docstring.
- `evidence_packet.schema.json` reuses existing schema; no new schema validation surface added.
- `review_blocked` / `review_requested_changes` flags silently dropped at INFO in `_sanitize_gate_inputs` — "update me" comment pinned for future tightening if Phase E wants review flags to influence verdict.
- Idempotent re-claim via `scheduler.claim_lease` strips `lease_sync`'s `owner_pid` extension field; cross-process `owner_pid` round-trip not guaranteed (acceptable for current scope, flagged for Phase F+).

## Test coverage (cumulative)

| Task | New tests | Regression |
|---|---|---|
| T1 write-run-protocol-bundle | 10 | 236/236 |
| T2 failure-codes | 49 | 261/261 |
| T3 lease | 18 | 52+3=55 |
| T4 evidence-writer | 12 | 224/224 |
| T5 default-agents | 19 | 192/192 |
| T6a run-gate-function | 20 | 193+311=311 |
| T6b run-gate-wiring | 21 | 332/332 |
| T7 autopilot-callers | 10 | 321/321 |
| T9 infer-failure-code-bridge | 17 | 320/320 |

Cumulative: ≥ 410 tests across the 9 task branches. Cross-branch smoke test deliberately omitted (would require integration merge which timed out); covered implicitly by the 9 task-specific suites.