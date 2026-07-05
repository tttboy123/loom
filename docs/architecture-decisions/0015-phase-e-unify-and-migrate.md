# ADR 0015 — Phase E: Unify and migrate to the typed Phase D core

- **Status**: Accepted (2026-07-05). **Deciders**: Loom architecture (Mavis plan `plan_e78d4693`).
- **Scope**: `devkit/{protocol,gatekeeper,scheduler,evidence_writer,failure_codes,lease,lease_sync,__main__,rdloop}.py` and `tests/`.

## Context

Phase C/D shipped but were never integrated into the live autopilot.
Seven integration blockers (none were code bugs — all were gaps left
behind when Phase B/C/D landed on separate branches):

1. `protocol.write_run_protocol_bundle` missing — `rdloop.py` wrapped every call in `try/except Exception: pass`; every run silently dropped its bundle.
2. Dual `evaluate_final_gate` API — Phase B kwarg-shape and Phase D typed `evaluate_run_gate` co-existed; `run_loop` kept calling Phase B.
3. Three failure-code vocabularies (Phase B free-strings, Phase D typed enums, Phase A repairer codes) with no translator.
4. Dual lease implementation — in-band `task["lease"]` (1800s) and out-of-band `scheduler.lease.json` (300s) with no bridge.
5. Evidence path mismatch — Phase B writes `devkit/runs/<id>/`, Phase C reads `devkit/evidence/<id>/evidence_packet.json`, Phase D reads `<evidence_dir>/evidence.json`.
6. `run_loop` not wired to `evaluate_run_gate`; autopilot `devkit auto` not wired to `select_next_pending`.
7. No architecture decision record — strategy was implicit in the branch graph.

## Decision

Adopt the unification strategy as a single sequenced wave:

1. **Canonical run-protocol writer** — `protocol.write_run_protocol_bundle` is the single writer of `protocol_bundle.json` (Draft 2020-12, atomic, fail-open).
2. **Deprecate-but-keep Phase B** — `rdloop.evaluate_final_gate` wrapped in `_deprecated` emitting `DeprecationWarning` pointing at `gatekeeper.evaluate_run_gate`. Body unchanged.
3. **Vocabulary translator** — `devkit/failure_codes.py` is the single mapping layer (`phase_b_to_phase_d` / `phase_d_to_phase_b` / `phase_a_to_phase_d`).
4. **Lease shim** — `devkit/lease_sync.py` bridges in-band (1800s) and out-of-band (300s); `lease.py` gains opt-in `lease_path=`.
5. **Canonical evidence writer** — `evidence_writer.write_run_evidence` writes `devkit/evidence/<run-id>/evidence_packet.json`; `run_loop` calls it after gate decision / before protocol bundle (fail-open).
6. **Run-loop + autopilot wiring** — `rdloop.run_loop` calls `_gatekeeper.evaluate_run_gate(...)` + persists `GateVerdict`; `_cmd_auto` uses `scheduler.select_next_pending` + claim/release lease. Legacy preserved behind `--no-scheduler`.
7. **Default agent registration** — `protocol.register_default_agents` registers the three Loom A2A agents (`observer`, `triager`, `repairer`); `ProtocolServer(auto_register_agents=True)` is default.

## Consequences

**Positive** — single source of truth for gate / scheduler / evidence / lease; working A2A / MCP cluster via `default_server()`; every run's `evidence_packet.json` / `verdict.json` / `protocol_bundle.json` atomic-written + schema-validated; runner decoupled from gate/scheduler implementations; `iterate.infer_failure_code` reads typed `GateVerdict` (Phase F bridge).

**Negative** — Phase B API surface kept for compatibility (tech debt until Phase F removes it); lease timeout defaults diverge intentionally (1800s vs 300s, deliberately not unified); three `try/except Exception: pass` wrappers remain in `rdloop.py` until Phase F.

## Follow-up (Phase F candidates)

- Remove `_deprecated` wrappers and delete the Phase B surface.
- Tighten the three `try/except Exception: pass` wrappers in `rdloop.py`.
- Add event-driven scheduler observer.
- Expand gatekeeper's failure-code coverage from 4 to the 15 codes in the v2 blueprint.

## References

- `docs/loom-protocol-layer.md` — hot-path policy.
- `docs/loom-gatekeeper-scheduler.md` — integration status.
- `docs/loom-task-graph-2026-07-05.md` — task graph.
- Mavis plan `plan_e78d4693` — 9-task integration plan.