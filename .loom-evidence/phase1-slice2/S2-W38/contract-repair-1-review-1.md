# S2-W38 Contract Repair 1 Fresh Review 1

- WorkItem: `S2-W38`
- Repair contract SHA-256:
  `d143ccd98fc63a9c87d9009894aeca941e31fe8336ba05703790fc2469b019b3`
- Frozen head: `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`
- Reviewer: fresh independent read-only contract Reviewer

## Findings

### High: separate projection can differ from the observer's bound projection

Repair 1 accepts both a `readModel *projection.Projection` and a prepared
observer, but does not prove pointer identity. A caller could refresh
projection A while S2-W37 observes stale projection B through
`observer.readModel`, reopening the exact stale-projection flaw that Repair 1
was intended to close.

Repair 2 must remove the separate projection parameter and synchronize only the
observer's actual bound projection, or reject pointer mismatch directly.

### Medium: partial-success scope exceeds S2-W37 observability

Repair 1 correctly requires five zero outputs for any S2-W37 error, but its
broader context-cancellation language can be read as requiring successful
outputs for cancellation detected inside S2-W37 or lower layers. Accepted
S2-W37 cannot recover those values because accepted downstream coordination
already returns five zero outputs.

Repair 2 must limit successful outputs plus error to failures observed only
after S2-W37 has returned its successful outputs: post-success rebuild failure
or a later context check. All S2-W37 errors remain five zero.

## ADR and authority

No new ADR or wider StateWriter authority is required. Projection rebuild
remains a read-model operation that swaps only after complete successful
replay; Journal append remains authoritative.

VERDICT: FAIL
