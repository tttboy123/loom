# Loom Governed Handoff — Community Message Pack

Date: 2026-08-08

Status: `DRAFT` — community copy for review. It does not change product status,
authorize publication, or claim generally available packaging or activated
production use.

Evidence basis: local accepted Phase 2B commit `6d380233`, the
[final Candidate lock](../.loom-evidence/phase2b/P2B-W1/final-candidate-lock.json),
the [controlled offline canary](../.loom-evidence/phase2b/P2B-W1/controlled-offline-canary.md),
and the [independent Result Review](../.loom-evidence/phase2b/P2B-W1/result-review.md).
The repository commit permalink is not anonymously accessible as of this
review, so it must not be used as public evidence until B3 provides a reachable
commit or evidence permalink.

## Category claim

Chinese:

> Claude 可以跨 Session 发消息，Codex 可以迁移 Chat 与 Git state；Loom
> 让交接本身可预览、可批准、可拒绝、可验证、可重放。

English:

> Don't move the whole session. Transfer only the context you can authorize and
> verify.

Category name: **Governed Handoff / 受治理交接**

What it is: a governed context transfer between independently accountable task
lineages.

What it is not: session sync, transcript copy, hidden-reasoning export, or a
shortcut around policy and approval.

## Competitive frame

| Product primitive | What crosses the boundary | Precise category |
|---|---|---|
| [Claude Code cross-session `SendMessage`](https://code.claude.com/docs/en/cross-session-messaging) | Plain-text message between independent sessions; the receiving session keeps its own context and permissions | Relay |
| [Claude Code teleport](https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md#2024) | Continuation of a cloud/web session in local CLI | Resume / teleport |
| [Codex Remote Handoff](https://developers.openai.com/codex/remote-connections#hand-off-a-chat-between-hosts) | The same chat and Git state move to a matching connected host/worktree | Relocation |
| [Loom Phase 2B Governed Handoff evidence](../.loom-evidence/phase2b/P2B-W1/final-candidate-lock.json) | Evidence-linked summary plus an explicitly chosen, bounded parent decision/effect | Governed context transfer |

Release pinning for the competitor claims is recorded in the
[first-party research note](session-handoff-competitive-research-2026-08-08.md):
Claude Code 2.1.224 introduced cross-session messaging; Claude Code 2.0.24
introduced web-to-CLI teleport; Codex documented remote task handoff in 26.616.

## Product truth

| Status | Claim |
|---|---|
| `CURRENT` | Loom has an accepted **parent/side-task** Governed Handoff path: zero-write proposal, explicit confirmation, independent child lineage, Evidence-linked summary Artifact, seven typed parent decisions, CAS single-winner semantics, restart/projection recovery, and macOS/TUI surfaces. |
| `PARTIAL` | The chat-first Phase 2C experience is still completing its cross-client acceptance gates. Governed Handoff is not yet the primary chat workflow. |
| `TARGET` | Generic task-to-task transfer and a moderated multi-session Roundtable. Neither is implemented by Phase 2B. |
| `EXPERIMENTAL` | External Claude, Codex, or A2A seats/adapters. No public endpoint, cross-account identity, or credential contract is claimed. |

Current Phase 2B means parent/side-task only. It must not be described as
generic session-to-session handoff; Roundtable remains `TARGET`.

## Fifteen-second pitch

Multi-agent work usually ends in copy-paste: too much context, unclear origin,
and no record of what the next task actually accepted. Loom's Governed Handoff
turns a side-task result into an Evidence-linked summary and asks the parent to
make an explicit typed decision. The result is bounded, digest-bound, and
rebuildable after restart.

## Sixty-second pitch

A useful diagnosis can come from a separate Agent task without giving that task
authority over the parent Mission. Loom first creates a zero-write proposal.
Only explicit confirmation admits the side-task. When it finishes, Loom stores
a structured summary Artifact with findings, risks, Evidence references, and a
digest. The parent can absorb it, continue from it, request follow-up, pivot,
discard, archive, or request cancellation. CAS and idempotency ensure competing
clicks do not continue the parent twice. The accepted offline canary retained
296 Events with zero duplicate Event IDs or idempotency keys, and exactly one
ContextPacket, continuation authorization, effect completion, and decision.

This is not a claim that Loom can hand any arbitrary session to any other
session today. The accepted capability is deliberately narrower:
parent/side-task Governed Handoff.

## README hero draft

### Context should cross task boundaries only with consent and proof

Loom is a local-first platform for composing and governing Agent teams. Its
Governed Handoff path lets a parent Mission delegate bounded work to an
independent side-task, inspect an Evidence-linked result, and choose exactly how
that result affects the parent lineage.

- Preview before write.
- Confirm before admission.
- Transfer structured findings, not raw transcripts.
- Bind every decision to source, digest, task, and generation.
- Rebuild the same state after restart without duplicate continuation.

**Development Preview:** the accepted flow currently supports parent/side-task
handoff. Generic task-to-task transfer, Roundtable, external seats, stable
one-click installation, and production activation remain outside this claim.

## Community post draft

Claude Code recently made independent sessions able to send messages. Codex can
move the same chat and Git state between connected hosts. Both are useful—but
they answer different questions.

Loom is exploring a third category: **Governed Handoff**.

Instead of copying a transcript or moving a whole session, a side-task returns
an Evidence-linked, digest-bound summary. The parent previews it and makes an
explicit typed decision. The accepted Phase 2B path is restart-safe,
single-winner under competing decisions, and visible in both macOS and TUI.

The important limitation: this is parent/side-task today, not generic
session-to-session transfer and not Roundtable yet.

Our product bet is simple: the competitive edge will not be moving more
context. It will be proving exactly which context was allowed to cross.

## Short social variants

### Variant A — category

Session relay sends a message. Session relocation moves the same chat.

Governed Handoff transfers only the context a destination can explicitly accept
and verify.

Loom Phase 2B already proves this for parent/side-task workflows: Evidence-linked
summary, typed decision, CAS single winner, restart-safe projection.

### Variant B — evidence

One offline Loom handoff canary:

- 296 retained Events
- 0 duplicate Event IDs
- 0 duplicate idempotency keys
- exactly 1 ContextPacket
- exactly 1 continuation authorization
- exactly 1 completed effect
- 18 content-addressed Artifacts
- 0 credential/Bearer sentinel hits

The goal is not “move everything.” It is “prove what crossed.”

### Variant C — limitation-led

What Loom Governed Handoff is today: accepted parent/side-task transfer with
explicit confirmation and Evidence-linked decisions.

What it is not yet: arbitrary task-to-task routing, Roundtable, or an external
Claude/Codex/A2A bridge.

Sharp boundaries are a feature when context can change what an Agent does.

## Demo narration

> A parent Mission hits an unknown. The user proposes a diagnosis side-task and
> reviews its purpose and scope. Confirmation creates an independent child
> lineage. The child returns a structured summary with findings, risk, and
> Evidence references. The user chooses **Absorb**. Loom commits one bounded
> ContextPacket and one continuation authorization. After daemon restart, macOS
> and TUI rebuild the same decision; repeating the action cannot continue the
> parent twice.

The narration describes the accepted P2B semantics. Any public recording must
use retained evidence or a separately contracted demo harness; the consumed
`canary-001` must not be rerun for marketing footage.

## FAQ

### Does Loom copy the full conversation?

No. The accepted P2B path transfers bounded structured Artifacts and digest
references. Raw transcripts, credentials, raw Grants, and hidden reasoning are
not part of the handoff packet.

### Is this the same as Claude cross-session messaging?

No. Claude's primitive is a relay between independent sessions. Loom's accepted
primitive also records a governed parent decision and its exact authoritative
effect.

### Is this the same as Codex Remote Handoff?

No. Codex moves the same chat and Git state between matching hosts/worktrees.
Loom P2B keeps two lineages distinct and governs how a side-task result affects
its parent.

### Can any Loom task hand off to any other task?

Not yet. Generic source-to-destination task transfer is `TARGET`; the current
accepted implementation is parent/side-task.

### What is the status of Roundtable?

No. Moderated multi-session Roundtable is `TARGET`. There is no accepted
Roundtable product implementation in this message pack.

### Can the accepted canary be rerun for a demo?

No. `canary-001` is consumed. A new repeatable public harness would be a
separate G1 contract with a new fixture and evidence budget.

## Claim guardrails

Before reuse, reject any copy that:

- promotes Roundtable beyond `TARGET`;
- promotes generic session or task-to-task handoff beyond `TARGET`;
- Loom synchronizes raw transcripts or hidden reasoning;
- Loom has a stable public installer or production activation;
- the consumed authority canary may be replayed;
- visual-only evidence is a second authority canary;
- external Claude/Codex/A2A adapters are available.

Public application of this pack remains gated on an accepted clean Phase 2C
baseline, a reachable public commit/evidence permalink, and a separate docs-only
B3 review.
