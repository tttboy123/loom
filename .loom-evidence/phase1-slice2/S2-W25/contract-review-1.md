# S2-W25 Contract Review 1

- Reviewer: fresh independent read-only Contract Reviewer
- Reviewed head: `9838779`
- Reviewed contract SHA-256:
  `a771b8438fdf7293609d687b06d3426d8d5dc153e6b47d80618dc034e6112dbd`
- Reviewed S2-W22 amendment SHA-256:
  `737585e1e8084e21e3287b145211b145e2bff4eac692b221bb5c88e5162cbcc7`

## Blocking finding

The frozen status-provenance validation accepted a forged first-link or
consecutive record that accepted S2-W24 replay cannot produce. It required
positive exact-next sequences and distinct Event IDs, but did not bind the
previous status-bearing Event ID to the current discovery Event ID when their
sequences were equal, or reject the inverse mismatch.

Concrete invalid record admitted by the reviewed wording:

```text
DiscoveryEventID=d1
DiscoverySequence=1
StatusPreviousEventID=other
StatusPreviousSequence=1
StatusEventID=s1
StatusSequence=2
```

Accepted S2-W24 requires the previous pair to match the latest projected
status-bearing fact exactly. Before the first status Event that pair is the
current discovery Event ID/sequence. The required bounded repair is:

```text
StatusPreviousEventID == DiscoveryEventID
if and only if
StatusPreviousSequence == DiscoverySequence
```

RED must cover both forged equality-mismatch directions.

The Reviewer found the S2-W22 provenance rename and baseline digest version `2`
otherwise warranted. S2-W24 supports consecutive status Events, so the old
discovery-specific names would be stale or misleading. The Candidate digest
version may remain unchanged because it binds the versioned baseline digest.

VERDICT: FAIL
