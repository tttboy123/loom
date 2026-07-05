# Unknowns Map (collaborative preflight)

> Surfacing user + agent + execution unknowns before planning. Source:
> `loom-agent-team-runtime-proposal.md` §7, plus `boundary-aware-agent`
> skill for the per-agent Blindspot Pass that fills it.

## Why this comes first

A team run starts with unknowns, not tasks. Producers (the builder
role especially) cannot do good work if the goal is ambiguous.
Gates cannot verify "done" if there is no acceptance contract.

The rule (loose but enforced by the skill): **every team run emits an
Unknowns Map as its first artifact, before any task transitions to
`In Progress`.**

## Three categories

```yaml
unknowns_map:
  run_id: run_20260706_001

  user_unknowns:           # "what the user didn't say that should change the answer"
    - id: acceptance_criteria
      question: "What does the user accept as 'done'?"
      default_action: infer_then_confirm_if_high_risk

    - id: audience
      question: "Who consumes this artifact? Internal team, customer, regulator?"
      default_action: infer_from_context

    - id: scope_boundary
      question: "Which files / directories are in scope? Out of scope?"
      default_action: ask_user_or_default_to_repo

    - id: time_budget
      question: "When is 'late'? Are there deadlines?"
      default_action: infer_then_assume

  agent_unknowns:          # "what the agent can't see without being told"
    - id: repo_truth
      question: "Do we need to read repo/doc structure to verify the answer?"
      default_action: read_before_design

    - id: prior_run
      question: "Was this attempted before? What broke?"
      default_action: query_run_trace

    - id: date_sensitivity
      question: "Does this become stale quickly (citations, prices, API state)?"
      default_action: browse_or_mark_as_assumption

  execution_unknowns:      # "what the runtime can't decide without permission"
    - id: write_scope
      question: "Which files / branches can be modified? Sandbox only, or main?"
      default_action: default_to_restricted

    - id: tools_required
      question: "What tools does the team need? (search, run tests, fetch URL, write diff)"
      default_action: list_in_team_spec

    - id: cost_ceiling
      question: "How much is the user willing to spend?"
      default_action: assume_default_ceiling

## What requires asking vs inferring
- ask_user: when the answer changes irreversibility or cost
- infer: when default can be safely tried
- verify: when cheap (e.g., a single tool call) and produces a falsifiable result
- mark_assumption: when all else fails; gate records assumption as risk

## Resolution workflow
1. Orchestrator emits Unknowns Map on team.yaml creation.
2. Map is reviewed at plan_gate. Each unknown either:
   - **Resolved** (value filled in).
   - **Assumption** (default chosen and recorded, with explicit risk).
   - **Escalated** (asks user; blocks execution).
3. After plan_gate, Map becomes part of the run root and is reused by every handoff.
4. Improvement auditor counts which unknowns caused rework and adjusts templates.