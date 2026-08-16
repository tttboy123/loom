# Governed Handoff — P2B Evidence Map and Storyboard

Date: 2026-08-08

Status: `DRAFT` — read-only evidence packaging. No canary, daemon, product IPC,
Provider, credential, network, staging, commit, or publication action is
authorized by this document.

Accepted implementation: local commit
`6d380233b5b89309a1a7ce3919aa611654e0f4ee`
(`feat(phase2b): add governed side-task handoff`), bound by the
[final Candidate lock](../.loom-evidence/phase2b/P2B-W1/final-candidate-lock.json).
The corresponding GitHub commit URL returns 404 to unauthenticated readers as
of 2026-08-08. B3 must provide a reachable public commit or evidence permalink
before community publication; this draft does not present the private/local
commit as publicly verifiable.

## Evidence index

| Evidence | Role | Current SHA-256 / binding | Verification on 2026-08-08 |
|---|---|---|---|
| [Source lock](../.loom-evidence/phase2b/P2B-W1/source-lock.json) | Binds 37 product/source paths, 18 governance/evidence paths, shared module inputs, and one offline canary budget | File `411593c724dfc78424dbaf99c4936b2367b92dd65b402b6841220d5e738ef97c` | MATCH |
| [Post-canary presentation delta lock](../.loom-evidence/phase2b/P2B-W1/post-canary-presentation-delta-lock.json) | Binds the two reviewed Swift presentation files without reopening the consumed authority canary | File `8e19132d75d4ae10da25f07d11f1e211a4b12ee40eee764961adf610ea75e970` | MATCH |
| [Controlled offline canary](../.loom-evidence/phase2b/P2B-W1/controlled-offline-canary.md) | Records the only authorized authority execution and retained invariants | File `12c4f20ebfe5b717fe3e431b89cfc15dc033747f487221a95e55ddbde3cf594e` | Present; attempt `canary-001` remains consumed |
| [Independent Result Review](../.loom-evidence/phase2b/P2B-W1/result-review.md) | Independently reproduces retained SQLite/Event/Artifact and visual evidence | File and final-lock binding `f22f294964e8b46e6f4d6e338444960026de56f752e1a820b4eee8d3671e1685` | MATCH |
| [Native visual-only Result](../.loom-evidence/phase2b/P2B-W1/native-drawer-visual-only-result.md) | Proves final reviewed SwiftUI presentation from bounded read-only retained data | File `c457fd5eb0cc3dc46e05cff85158e0f7b9913ccafdff00a4e0f1c428c3131292` | Present; explicitly not a second authority canary |
| [Whole-Candidate Review](../.loom-evidence/phase2b/P2B-W1/whole-candidate-review.md) | Reviews the exact whole Candidate | Final-lock binding `f3d5305dac1af691a6a6f9d24468274b3384c77cdee87c2f17c4a51d61ceea1e` | MATCH |
| [Clarification Re-review](../.loom-evidence/phase2b/P2B-W1/whole-candidate-review-clarification-rereview.md) | Closes the final review clarification | Final-lock binding `2dcdfe1ae4de4bb761c921831f8fc1973d23ec1c4c952ca097c4a2975e10c119` | MATCH |
| [Exact Candidate boundary](../.loom-evidence/phase2b/P2B-W1/exact-candidate-boundary.md) | Freezes owned paths and exclusions | Final-lock binding `b204fdf5e8a46ea9b150e9c3633e71d7f49130861bd3e56ab364a762b1537ae4` | MATCH |
| [Final Candidate lock](../.loom-evidence/phase2b/P2B-W1/final-candidate-lock.json) | Binds accepted source, reviews, retained Result evidence, and staging policy | File `b1cae3ab5581e8e0f6c11a96ca65466697b1136ac1b7693de85f129c7a68fc2e` | MATCH commit `6d380233` |
| [Input Artifact restart amendment](../.loom-evidence/phase2b/P2B-W1/input-artifact-restart-amendment.md) | Freezes input Artifact v2 and restart fail-closed semantics | File `4b61e6c03dc97aab013b4f027911c3acba9c07005a40c15a1b2468fd948c4755` | Present and consistent with current code |

The final lock binds `docs/CURRENT.md` at historical accepted-candidate hash
`5d5a11c0b03dae73ac43faa406810f6e140868c48b71a9c445e897c9f6722306`.
`git show 6d380233:docs/CURRENT.md` still reproduces that hash. The current dirty
worktree's later `docs/CURRENT.md` bytes are intentionally not treated as P2B
evidence drift.

## Retained result assets

All retained assets below were checked read-only on 2026-08-08.

| Asset | Mode | SHA-256 | Availability and permitted use |
|---|---:|---|---|
| `.../phase2b-live/canary-001/vertical-fixture/state.db` | `0600` | `f47e4095ad8da5799ce07ca6ceb8fe0ac29e13dc491b5c3ab3854adc996d037a` | Available. Retained authority result; do not mutate or rerun the canary. |
| `.../phase2b-live/canary-001/manifest.json` | `0600` | `ece9b16eeab8738013f04d36db96d3b5011ddba54e5d91ef5af3f0cb1bbeede2` | Available. Records attempt 1, offline loopback, no Provider credentials, no replacement. |
| `.../phase2b-visual/side-task-drawer-002/native-side-task-drawer-pass.png` | `0600` | `39fbb354bff36997283a0cd96a3bfc2a1092a633accaf279fc5580ef136832e6` | Available. Visual-only screenshot; may illustrate the reviewed Drawer, not authority execution. |
| `.../phase2b-visual/side-task-drawer-002/native-side-task-drawer-pass-ax.txt` | `0600` | `54c5e888507fff82a2a30bd4f6124ee8b9a8a3bac851223fd9bc02a1317f5f08` | Available. Visual-only accessibility transcript. |

The visual host had no product write client. The screenshot and AX transcript
prove presentation only; the consumed source-locked vertical canary remains the
authority and real Go IPC-to-Swift proof.

## Accepted canary invariants

The retained authority result proves:

| Invariant | Result |
|---|---:|
| Total Event count | 296 |
| Duplicate Event IDs | 0 |
| Duplicate idempotency keys | 0 |
| `ContextPacketCommitted` | exactly 1 |
| `ParentContinuationAuthorized` | exactly 1 |
| `ParentHandoffEffectCompleted` | exactly 1 |
| `SideTaskDecisionCommitted` | exactly 1 |
| Retained content-addressed Artifacts | 18 |
| Artifact filename/digest failures | 0 |
| Known credential/Bearer sentinel hits in SQLite payloads | 0 |
| Remaining canary socket, lock, or process | 0 |

These numbers describe the accepted controlled fixture, not throughput,
benchmark, production scale, or a reusable public demo.

## Governed path and evidence mapping

```text
zero-write proposal
  -> explicit confirmation
  -> independent child execution lineage
  -> read-verified Evidence + summary Artifact
  -> explicit typed parent decision
  -> optional bounded ContextPacket / continuation effect
  -> Journal + Projection rebuild after restart
```

| Step | Product truth | Evidence to show |
|---|---|---|
| Proposal | `ProposeSideTask` validates parent/view bindings and returns a proposal digest without authority write | Contract, source lock, TUI/native proposal tests; use a diagram or labeled UI mock unless a separately retained screenshot exists |
| Confirm | `CreateSideTask` requires explicit `confirmed=true`, publishes and read-verifies the input Artifact, then admits the child | Contract and controlled vertical canary; do not imply free text auto-admits work |
| Child lineage | The side-task compiles and runs under its own lineage and accepted Evidence boundary | Controlled vertical canary and whole-Candidate Review |
| Summary | Loom publishes an authorized summary Artifact with finding/risk/Evidence references and a digest | Native visual-only screenshot plus retained canary authority evidence |
| Decision | Parent selects one of seven typed decisions: `absorb`, `continue`, `request_followup`, `pivot`, `discard`, `archive`, `cancel_parent` | Contract/API/TUI/native tests and `SideTaskDecisionCommitted` exactly-one result |
| Context/effect | `absorb` may publish one bounded parent-specific ContextPacket; `continue` may authorize one continuation; other decisions produce their typed effect | Exactly-one Event counts in retained SQLite and independent Result Review |
| Restart | Journal facts rebuild the same handoff state; pending continuation/cancellation effects reconcile without duplicate completion | Controlled canary, restart amendment, Result Review, projection tests |

## Schema clarification: preserve history, state current truth

The frozen parent [contract](../.loom-evidence/phase2b/P2B-W1/contract.md)
contains a historical section titled `Side-task input Artifact v1`. That text
must not be rewritten.

The accepted
[Input Artifact Restart Closure Amendment](../.loom-evidence/phase2b/P2B-W1/input-artifact-restart-amendment.md)
supersedes new-write behavior with **input Artifact schema version 2**, adding
the required `decision_timeout_seconds` field. Current accepted code constructs
the input with `SchemaVersion: 2` and restart accepts v2 only for P2B-created
state. Summary Artifact and parent-specific ContextPacket remain schema version
1; the local product IPC request/response schema is also version 1.

Public wording should therefore be:

> Accepted P2B writes use input Artifact v2; summary Artifact v1 and
> parent-specific ContextPacket v1 remain unchanged. The original contract's
> v1 input section is historical and is clarified—not rewritten—by the accepted
> restart amendment.

## Storyboard — 75 seconds

The storyboard reuses retained evidence and diagrams. It does not require or
authorize a second canary.

### Beat 1 — The boundary problem (0–8s)

Visual: two independent task lineages with a raw transcript blocked between
them.

Narration: “A useful result should cross a task boundary without bringing the
whole conversation, hidden reasoning, or source permissions with it.”

Asset rule: diagram only; do not fabricate a product screenshot.

### Beat 2 — Preview before write (8–17s)

Visual: proposal card showing parent, purpose, mode, scope, and destination.

Narration: “Loom first creates a zero-write proposal. The user reviews purpose
and scope before admission.”

Asset rule: use a labeled mock/diagram unless an accepted retained proposal
capture is later added under a separate visual-only review.

### Beat 3 — Confirm an independent child (17–26s)

Visual: parent lineage branches to a side-task lineage after explicit confirm.

Narration: “Confirmation admits an independent child lineage. Ordinary chat
cannot create it implicitly.”

Evidence overlay: commit `6d380233`; source lock `411593c…`.

### Beat 4 — Evidence-linked result (26–38s)

Visual: retained native Side-task Drawer screenshot, cropped only for layout and
never altered to add fields.

Narration: “The child returns a structured summary: finding, risk, Evidence and
Artifact references, uncertainty, scope, and next action.”

Evidence overlay: screenshot `39fbb354…`; label **visual-only presentation
evidence**.

### Beat 5 — Typed parent decision (38–49s)

Visual: decision menu with the seven accepted choices, followed by **Absorb**.

Narration: “The parent chooses what the result means. It can absorb, continue,
request follow-up, pivot, discard, archive, or request cancellation.”

Asset rule: a diagram may enumerate all seven choices; do not claim one
screenshot captured all decision effects.

### Beat 6 — Transfer only the authorized packet (49–59s)

Visual: summary Artifact digest enters a bounded ContextPacket while raw
transcript, credentials, raw Grant, and hidden reasoning remain outside.

Narration: “Absorb transfers one bounded, digest-bound ContextPacket—not the
whole session.”

Evidence overlay: `ContextPacketCommitted = 1`, credential/Bearer sentinel
hits `= 0`.

### Beat 7 — Single winner, restart-safe (59–68s)

Visual: two competing clicks converge on one committed decision; daemon restart
then rebuilds the same state in macOS and TUI.

Narration: “CAS makes the decision single-winner. Journal and Projection rebuild
the same state after restart without continuing twice.”

Evidence overlay: duplicate Event IDs `0`, duplicate idempotency keys `0`,
continuation authorization `1`, effect completion `1`.

### Beat 8 — Category close and limitation (68–75s)

Visual: `Relay ≠ Resume ≠ Relocate ≠ Governed Handoff`.

Narration: “Loom does not move the whole session. It governs what may cross.
Today that accepted path is parent/side-task; generic task-to-task and
Roundtable come next.”

## Capture and production checklist

- Keep the screenshot's original SHA-256 and `visual-only` label in production
  notes.
- Never show the retained SQLite, raw Artifact bodies, paths, prompts, or
  credential-shaped data on screen.
- Use diagrams for proposal, confirm, competition, and restart beats unless
  separately reviewed retained captures exist.
- Do not animate a second authority execution or imply `canary-001` was rerun.
- Attribute numeric assertions to the controlled offline fixture.
- End with the parent/side-task limitation, not a generic session claim.
- A repeatable executable demo requires a separate G1 contract and fresh
  fixture; it is not part of this storyboard.
