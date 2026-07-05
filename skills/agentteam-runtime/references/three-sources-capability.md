# Three-source capability absorption

> Why this skill looks the way it does. Source: `loom-agent-team-runtime-proposal.md` §0, §2.

## The principle

**Don't choose between MiniMax, Claude, or Codex. Absorb all three at
the right layer.**

| Source | What it gives | Loom absorbs as |
|---|---|---|
| MiniMax Agent Team Skill | Governance protocol (roles, state machine, handoff, KPI, templates) | TeamSpec, governance-protocol.md, HandoffEnvelope |
| Claude Agent Teams | Team UX (Lead, Teammates, Task Board, Mailbox, user intervention) | Cockpit views, MailboxMessage, "loom team message --to <role>" |
| Codex | Engineering execution + acceptance (worktree, diff, test, review, gate) | GatePolicy, CandidateWorkspace, evidence-driven acceptance |

The resulting shape:

> A higher-level Skill emits team contracts; Loom runtime executes and
> visualizes them; Codex (or any equivalent high-reliability agent)
> runs critical gates.

## Why this is the right cut

- **MiniMax's governance spec is well-engineered** — hierarchical /
  mesh / hybrid topologies, role profiles, handoff envelopes, KPI
  dashboards, scenario templates. Loom should adopt the schema, not
  reinvent it. See `references/governance-protocol.md`.
- **Claude's UX is the natural team feel** — a visible Lead, named
  Teammates, a shared Task Board, inter-agent messages, the user can
  intervene. Loom must render this in its cockpit
  (`references/cockpit-views.md`), not just in a prompt.
- **Codex is best at the engineering close** — context-first reads,
  worktree isolation, test/lint commands, reviewer stance, final
  acceptance. Loom's gate policy should institutionalize the
  reviewer/closer pattern, see `references/gate-policy.md`.

## What we explicitly do NOT do

- Lock Loom to one model family. MiniMax / Claude / Codex / Claude
  Fable 5 / Claude Mythos 5 / local opensource all pluggable.
- Let agent self-approve. The reviewer must be a fresh-context agent
  that did not produce the artifact.
- Make gate policy a prompt convention. Gates are runtime objects with
  hard refusal rules; see `runtime-contract.md`.

## Anti-patterns

- "We built our own MiniMax!" — Don't. Use MiniMax as a *reference for
  governance schema*; write Loom-native code that follows the same
  shape.
- "We'll skip the gate for trusted agents." — No. Producer cannot
  self-approve is a hard rule.
- "Mailbox messages are JSON over stdin." — Mailbox is a runtime-
  mediated artifact with persistence, ordering, threading. See
  `runtime-contract.md`.
