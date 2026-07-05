# Failure modes (catalog)

> 18 failure modes the team runtime must detect and handle. Source:
> `loom-agent-team-runtime-proposal.md` §12 (with extensions from
> cross-reviewer feedback in `loom-control-theory-extension-v2-2026-07-05.md`).

Each entry has: **Detector** / **Mitigation** / **Metric** / **Owner**.

## 1. Over-orchestration

- **Symptom**: A 5-line edit task triggers a 4-role team.
- **Detector**: `loom team plan` refuses to expand `team.yaml` if
  `acceptance_criteria` fits in a single-producer task.
- **Mitigation**: mode selector defaults to `single-agent` unless
  the user explicitly asks for team mode.
- **Metric**: `mode_dispatch_accuracy` — how often the chosen mode
  matches the user's intent (validated post-hoc).
- **Owner**: skill designer.

## 2. UX-shell deception

- **Symptom**: Cockpit looks alive but no real work happens; producer
  outputs Lorem Ipsum, gate approves.
- **Detector**: evidence gate refuses `unknown` or `inner_sandbox`
  source for any code_change / research class.
- **Mitigation**: hard-bind gate to EvidencePacket; refuse Done if
  gate is missing.
- **Metric**: `false_done_rate` (should be 0).
- **Owner**: gate agent + reviewer.

## 3. Cost runaway

- **Symptom**: Multi-agent loop re-fetches the same source 30 times.
- **Detector**: per-run cost ceiling at the GatePolicy layer.
- **Mitigation**: shared context cache + trace audit. Kill the loop
  when over budget.
- **Metric**: `cost_per_accepted_task`, `redundant_tool_call_rate`.
- **Owner**: orchestrator / runtime.

## 4. Self-modification runaway

- **Symptom**: Improvement loop proposes relaxing permissions / safety
  rules / routing / system prompt.
- **Detector**: candidate patches that touch those targets are
  flagged `prohibited_automation` and can never be auto-applied.
- **Mitigation**: human or `frontier_safety_consulted` approval
  required; default `no`.
- **Metric**: `prohibited_patch_auto_attempt_count` (must be 0).
- **Owner**: meta-tier.

## 5. Responsibility ambiguity

- **Symptom**: Two agents both claim to own a task; both think they
  can mark it Done.
- **Detector**: TeamSpec validator rejects tasks with multiple
  owners.
- **Mitigation**: enforce owner = single role; reviewer != producer.
- **Metric**: `owner_uniqueness_violations_per_run`.
- **Owner**: skill designer.

## 6. Runtime pollution of main workspace

- **Symptom**: Builder writes directly to `main` instead of a
  candidate workspace.
- **Detector**: write-scope enforcement on `loom team run --scope`.
- **Mitigation**: candidate workspace + integration_gate.
- **Metric**: `unscoped_writes_per_run`.
- **Owner**: runtime.

## 7. Context exhaustion

- **Symptom**: Long-running team run hits model context limits;
  drops earlier turns / evidence.
- **Detector**: token_count approaching window limit per call;
  trigger summarization.
- **Mitigation**: forced summarization ("Amnesia repair") + handoff
  with summary as artifact.
- **Metric**: `context_window_pressure_ratio` (target < 0.8).
- **Owner**: runtime.

## 8. Contract fragility

- **Symptom**: Predicate too strict rejects valid outputs.
- **Detector**: false-negative rate (manual review sampling).
- **Mitigation**: soft-contract mode for non-critical paths;
  producer can request predicate relaxation at plan_gate.
- **Metric**: `false_reject_rate` (target < 10%).
- **Owner**: skill designer + reviewer.

## 9. Verifier drift

- **Symptom**: Property tests / schema checkers become out of date;
  gate accepts garbage.
- **Detector**: gate self-test (gateway runs a known-good sample
  through itself).
- **Mitigation**: contract versioning + canary suite per release.
- **Metric**: `gate_self_test_pass_rate` (target 100%).
- **Owner**: meta-tier.

## 10. TOCTOU stale snapshot

- **Symptom**: Reviewer sees evidence from snapshot N while
  state_writer already advanced to snapshot N+1.
- **Detector**: snapshot age vs action age; reject if too large.
- **Mitigation**: short-lived leases; gate must re-read just before
  signing.
- **Metric**: `snapshot_staleness_seconds` (target ≤ 5).
- **Owner**: runtime.

## 11. Correlated agent failure

- **Symptom**: Multiple agents run same task and all make the same
  mistake.
- **Detector**: diversity check — 3 reviewers run different prompt
  seeds, not different model weights.
- **Mitigation**: diverse verifier quorum (not generator quorum).
- **Metric**: `agent_failure_correlation` (target < 0.3).
- **Owner**: gate agent + reviewer.

## 12. Sandbox / tool nondeterminism

- **Symptom**: Property tests flaky; same code passes sometimes,
  fails others.
- **Detector**: detect flaky rate; quarantine failing tests.
- **Mitigation**: deterministic sandbox; flakiness = fail.
- **Metric**: `flake_rate` (target 0 in critical path).
- **Owner**: runtime.

## 13. Security / prompt injection

- **Symptom**: Malicious instructions enter via repo file or
  mailbox message.
- **Detector**: source-content security filter; capability allowlist.
- **Mitigation**: shell commands restricted to allowlist; no
  arbitrary network calls.
- **Metric**: `injection_attempts_per_run` (track; design to zero).
- **Owner**: meta-tier (security).

## 14. Human gate overload

- **Symptom**: Too many tasks failing to human-in-loop, forming a
  queue.
- **Detector**: human-gate queue length.
- **Mitigation**: bounded retry → auto-quarantine instead of human.
- **Metric**: `human_gate_queue_depth` (target < 10).
- **Owner**: meta-tier.

## 15. Control chattering

- **Symptom**: Repair / re-review actions thrash in a tight loop.
- **Detector**: round-trip rate per task within a window.
- **Mitigation**: deadband + hysteresis on gate signals; max 3
  cycles then escalate.
- **Metric**: `gate_round_trip_count` (max 3).
- **Owner**: runtime.

## 16. Deadlock from strict barriers

- **Symptom**: A barrier prevents backstep, but backstep is the only
  way out of a bad state.
- **Detector**: time-since-last-progress on a task.
- **Mitigation**: timeout-based escalation; admit backstep if no
  forward move in T seconds.
- **Metric**: `task_idle_seconds_max` (configurable per role).
- **Owner**: runtime.

## 17. Reward hacking / goal gaming

- **Symptom**: Improvement candidate generator proposes trivial goals
  ("close incident as won't-fix") to game metrics.
- **Detector**: human review sample + reward-decomposition analysis.
- **Mitigation**: gating + lower-rate of self-generated goals; human
  approval for goal-generation steps.
- **Metric**: `trivial_goal_rate` (target 0).
- **Owner**: meta-tier.

## 18. Proof / signing key compromise

- **Symptom**: Evidence signatures no longer trustworthy.
- **Detector**: signature mismatch rate spike + key lifecycle audit.
- **Mitigation**: key rotation policy + threat-model spec.
- **Metric**: `signature_mismatch_rate`.
- **Owner**: meta-tier.

## Adding a new failure mode

Use the same template (Symptom / Detector / Mitigation / Metric /
Owner). File a YAML in `references/failure-modes/<id>.yaml` and add
a row to this index. Without a complete entry, the failure mode is
not "tracked" — only "noted".
