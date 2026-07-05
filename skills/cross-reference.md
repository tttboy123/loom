# Cross-reference index

> Where every concept in the two skills maps to (a) Loom code that
> exists today, (b) the planned extension doc, or (c) the missing
> runtime code that needs to be written.

## Status legend

- **EXISTS** — code is present in `main` or in a merged branch.
- **PLANNED** — referenced in a blueprint or extension doc but not
  yet implemented.
- **PROPOSED** — needed for these skills to be deployable end-to-end.
  No code exists yet; this is a recommendation, not a plan.

## Data objects

| Object | Skill that produces | Status | Loom anchor |
|---|---|---|---|
| `TeamSpec` | agentteam-runtime | PROPOSED | `team.yaml` template only |
| `TaskGraph` / `TaskState` | agentteam-runtime | PROPOSED | uses Loom `backlog.json` for persistence |
| `RoleProfile` | agentteam-runtime | PROPOSED | informed by `loom.roles.example.toml` |
| `HandoffEnvelope` | agentteam-runtime | PROPOSED | consumes `devkit/state_writer.py` events |
| `MailboxMessage` | agentteam-runtime | EXISTS (A2A) | `devkit/protocol.py` (Phase C already wired AgentCard interchange) |
| `EvidencePacket` | both (Loom primitive) | EXISTS | `devkit/protocol_schemas/evidence_packet.schema.json` |
| `GatePolicy` | agentteam-runtime | EXISTS (Phase D) | `devkit/gatekeeper.py`; this skill adds binding convention |
| `RunTrace` | agentteam-runtime | EXISTS | `devkit/state_writer.py` event log |
| `ImprovementCandidate` | both | EXISTS (template) | `.loom/runs/<id>/improvement/candidates/` |
| `UnknownsMap` | both | PROPOSED | new; consumed by `plan_gate` and `goal_controller.future` |
| `BoundaryCard` | boundary-aware-agent | PROPOSED | new; consumed by `devkit/repairer.dispatch` for stop-conditions |
| `ModelCapabilityProfile` | boundary-aware-agent | PROPOSED | new; inform future model-routing code |
| `EvalPatch` | boundary-aware-agent | PROPOSED | new; consumed by `eval` automation |

## Existing Loom code that consumes these skills

| Loom file | Skill output it should consume | Status |
|---|---|---|
| `devkit/repairer.py` | `BoundaryCard.stop_conditions` | PLANNED (Phase D / control-theory v2 doc) |
| `devkit/state_writer.py` | `HandoffEnvelope`, `ImprovementCandidate` | EXISTS — event log already records transitions |
| `devkit/protocol.py` (Phase C A2A) | `MailboxMessage`, AgentCard interchange | EXISTS |
| `devkit/gatekeeper.py` (Phase D) | `GatePolicy` binding | EXISTS — needs convention for binding to producer |
| `devkit/goal_controller.future` (control-theory v2 doc) | `UnknownsMap.acceptance_unknowns` | PLANNED |
| `devkit/safety_filter.future` (control-theory v2 doc) | `BoundaryCard.risk_domain` | PLANNED |
| `devkit/admission.future` (control-theory v2 doc) | `cost_ceiling` from `UnknownsMap` or `BoundaryCard` | PLANNED |
| `devkit/repairer.dispatch` (Phase 4) | `ImprovementCandidate` events as incidents | EXISTS — could route candidate patches here |

## Object schemas that must remain identical between the two skills

The following YAML types appear in both skills' templates. They
**must** remain identical so a runtime can consume either skill's
output:

| Type | agentteam-runtime template | boundary-aware-agent template | Required fields |
|---|---|---|---|
| `unknowns-map.yaml` | `assets/templates/unknowns-map.yaml` | `assets/templates/unknowns-map.yaml` | `user_unknowns`, `agent_unknowns`, `execution_unknowns` |
| `improvement-candidate.yaml` | `assets/templates/improvement-candidate.yaml` | `assets/templates/improvement-candidate.yaml` | `finding`, `proposed_change`, `risk`, `eval_plan`, `approval` |

If you change either, change **both** — and update `cross-reference.md`.

## Companion document mapping

This skill set stands alongside the **control-theory extension doc**
on branch `feat/docs-control-theory-extension`:

| Skill concept | Control-theory doc anchor |
|---|---|
| `BoundaryCard.stop_conditions` | §5.2 Control Barrier Functions (safety_filter) |
| `BoundaryCard.risk_domain` | §5.1 SCDES Supervisor (admissible action check) |
| `BoundaryCard.cost_ceiling` | §5.3 Queueing-theoretic admission control |
| `ImprovementCandidate.finding.type` | §9 Failure Modes table |
| `EvalPatch.test_type` | §10 Calibration methodology |
| `MailboxMessage.routing` | §4 3-tier architecture (Runtime tier) |
| `HandoffEnvelope.status=review` | §5.4 Ranking function (priority scheduling) |
| `GatePolicy.gates.improvement_gate` | §11 critical path policy |

The skills and the control-theory doc form a **layered system**:

- Skills describe **what** the runtime must do.
- Control-theory doc describes **how** the runtime stays safe while
  doing it.

## Implementation work implied (when the user is ready)

If the user decides to implement the runtime side:

| Priority | Work | Estimated effort |
|---|---|---|
| P0 | `devkit/profile_loader.py` (load ModelCapabilityProfile YAML) | ~150 lines |
| P0 | `devkit/unknowns.py` (UnknownsMap serialization + lint) | ~150 lines |
| P0 | `devkit/boundary.py` (BoundaryCard validation + decision) | ~200 lines |
| P1 | `devkit/improvement.py` (loop 8-step; emits candidates) | ~300 lines |
| P1 | `devkit/eval_patch.py` (eval harness runner) | ~200 lines |
| P2 | Cockpit UI (TUI or web) | ~1500-3000 lines |
| P2 | `scripts/validate_team_spec.py` and `scripts/lint_handoff.py` | ~200 lines |
| P3 | `scripts/replay_<name>.py` per finding type | ~50 lines each |

These are **not in scope of this commit**. They're listed for
orientation when implementation starts.

## How to add a new reference doc

When adding a new `references/<topic>.md` to either skill:

1. Update that skill's `README.md` routing table.
2. If the new doc introduces a new data object, add it to the
   schema table above.
3. If the new doc maps to existing Loom code, add a row to the
   "Existing Loom code that consumes these skills" table.
4. If the new doc has a counterpart in the control-theory doc,
   add a row to the "Companion document mapping" table.
