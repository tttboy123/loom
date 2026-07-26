# Post-Slice-3 Capability Roadmap Amendment Review 1

Reviewer: independent read-only plan reviewer

Date: 2026-07-26

## Findings

None blocking.

## Evidence

- The amendment is planning-only and authorizes no code, schema, Runtime or
  Provider use, daemon, network, publication, or extra Phase 1 WorkItem.
- S3-W5 remains closed and no S3-W6 may be created.
- Slice 4 separates output classification from bounded recovery decisions and
  distinguishes workflow degradation from automatic Provider/model fallback.
- Slice 5 freezes tentative bounded deltas, Journal-authoritative milestones,
  cursor reconnect, slow-consumer/gap behavior, and secret/reasoning filtering
  while limiting Phase 1 delivery to a local event contract and CLI timeline.
- Phase 2 query/UX surfaces consume Journal, Projection, and
  `GlobalReadView` without becoming a second authority or Agent trigger.
- Phase 3 versioned assets, Runtime materialization, promotion, evaluation, and
  pattern extraction preserve the Candidate-only Sidecar boundary.
- Later Autopilot, shared catalogs/notifications/multi-user surfaces, and
  Provider/session resume remain opt-in and separately governed.
- Copied-risk exclusions preserve third-party review, credential, completion,
  authority, checkpoint, raw-Grant, and hidden-reasoning boundaries.
- The planned PRODUCT/TECH routing is appropriate and does not claim any
  queued capability is implemented.

## Non-blocking wording repair

The frozen amendment:

- repeats the raw-Grant prohibition in the global copied-risk checklist; and
- names the P2 group “post-Phase-1 later opt-in” to avoid confusion with
  Phase 2 product surfaces.

## Recommendation

Freeze the amendment and apply only its planning-level changes to
`PRODUCT-PLAN.md` and `TECH-PLAN.md`.

VERDICT: PASS
