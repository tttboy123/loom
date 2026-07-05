# TeamSpec

> How a team run is declared. This file replaces the TopSpec / AgentSpec
> of various ad-hoc team implementations with a Loom-native format.
> Source: `loom-agent-team-runtime-proposal.md` §4 team-spec.md, §10
> team.yaml.

## Purpose

`TeamSpec` is the contract under which a team runs. It declares:

- The **operating mode** (`single-agent` | `agent-team` | `cluster`).
- The **topology** (hierarchical / mesh / hybrid).
- The **roles** and their capability / permission profiles.
- The **task graph** as a DAG of dependencies.
- The **required gates** with explicit reviewers.
- The **acceptance contract** that defines done.

## Topology selection

- **`hierarchical`** — one Lead (orchestrator) plus a small team of
  specialists. Best when task decomposition is clear.
- **`mesh`** — peers review each other. Best for debate, comparison,
  and analysis tasks where multiple perspectives reduce blindspots.
- **`hybrid`** — Lead orchestrates, but specialist peers may escalate
  to other specialists via mailbox.

The choice is a recommendation, not a hard rule. Cost and risk
should bias toward `hierarchical` for low-trust task types and `mesh`
for high-stakes reviews. See `references/model-routing.md`.

## Role profile

A role has four fields that the runtime inspects at scheduling time:

| Field | Meaning | Notes |
|---|---|---|
| `id` | unique name within team | required |
| `purpose` | one-line role description | required |
| `model_policy` | which model tier to use | required; see `model-routing.md` |
| `can_write` | whether the role can write production artifacts | bool |
| `must_be_non_producer` | cannot self-review | bool; default true |
| `review_rules` | per-role review expectations | optional list |

## Task graph

DAG. A task node carries:

```yaml
- id: T1
  title: "..."
  owner: orchestrator
  state: inbox            # inbox | assigned | in_progress | review | done | failed | repair_requested
  depends_on: []         # list of task IDs
  acceptance:             # the tests / criteria that decide done
    - "..."
  artifacts: []           # filled at handoff time
```

The runtime forbids a transition to `done` if `acceptance` is empty.
This is enforced by `devkit/state_writer.py` for terminal states and
by `references/gate-policy.md` for explicit gates.

## Gate binding

A required gate binds to a reviewer role that *cannot* be the
producer. The skill will refuse to evaluate a gate if the binding is
violated.

## Example

The minimal working team spec is in `assets/templates/team.yaml`.
See also `references/runtime-contract.md` for fields that appear in
more complex TeamSpecs (SubAgent spawning, environment requirements,
cost budgets).

## Common errors when authoring TeamSpec

| Symptom | Cause |
|---|---|
| Task stays in `in_progress` forever | Owner role's `review_rules` not set or `must_be_non_producer=false` |
| Gate never fires | No `required_gates` listed; gate defaults to optional |
| Producer self-approves | Reviewer role configured as the same agent as producer — runtime forbids this; fix the binding |
| Cluster mode = agent-team | `cluster` is a composition primitive, not a team primitive; see `references/runtime-contract.md` |

## Validation

Run `scripts/validate_team_spec.py` on the team's `team.yaml`. The
script enforces:

- Required fields present.
- Gate reviewer ≠ producer.
- At least one required gate before any task can transition to `done`.
- Acceptance non-empty for every terminal-eligible task.

(See the companion script in the implementation phase, not yet
provided in this skill draft.)
