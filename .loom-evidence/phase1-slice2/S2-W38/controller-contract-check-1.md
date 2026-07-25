# S2-W38 Controller Contract Check 1

- WorkItem: `S2-W38`
- Reviewed contract SHA-256:
  `81e7cb6cd1b4d1b8e7edb631028faf0b58ee2248aec4494a03efb2e9f99d3a3b`
- Frozen head: `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`
- Check timing: after fresh Contract Review 1 `PASS`, before mandatory RED

## Finding

The original recurrence contract is not implementable with its required real
discovery→discovery→status proof while preserving its no-projection-rebuild
boundary.

Accepted S2-W36 binds one in-memory `*projection.Projection`. Each `RunOnce`
reads a copied snapshot and accepted committers append authoritative Events,
but no accepted observation layer rebuilds that projection after a successful
write. Existing real-chain tests therefore rebuild explicitly between
discovery and status phases.

An S2-W38 loop that only repeats S2-W37 would read stale projected Runtime
facts on the next trigger. A discovery write could therefore be selected again
where ADR-0007 requires status-only selection. Hiding a projection rebuild in
the external trigger would give that trigger an undeclared lower-layer
authority and would not close the product boundary.

## Required correction

No product or mandatory-RED test file has been created. Contract Repair 1 must
precede implementation and must:

1. postpone recurrence;
2. insert an explicit accepted projection refresh after the underlying trigger
   and before observation;
3. refresh again after a successful authoritative write so the bound read
   model exposes the committed fact immediately;
4. distinguish pre-write failure (five zero outputs) from post-commit refresh
   failure (successful outputs plus the exact refresh error); and
5. retain no time, scheduler, config, daemon, retry, or activation authority.

VERDICT: FAIL
