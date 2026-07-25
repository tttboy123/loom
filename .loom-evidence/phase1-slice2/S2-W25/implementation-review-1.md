# S2-W25 Implementation Review 1

- Reviewer: fresh independent read-only implementation Reviewer
- Reviewed head: `9838779`
- Reviewed Candidate: `deliverable.md`

## Blocking finding

`BuildRuntimeStatusBaselines` accepted cross-record reuse of untrusted Event IDs
and could mint S2-W22 baseline provenance that accepted S2-W24 replay cannot
produce.

The implementation validated each record independently but did not track
`DiscoveryEventID`, `StatusEventID`, or `StatusPreviousEventID` reuse across
Runtime records. Accepted projection replay globally deduplicates/conflict-checks
Event IDs before applying discovery or status Events.

The bounded repair is to reject any Event ID reused across different Runtime
records in any of those three roles. The legitimate same-record first-status
alias remains allowed only when previous and discovery Event ID/sequence match
as a pair.

Required direct proof includes duplicate discovery, duplicate current status,
and duplicate previous status Event IDs across records. The Controller expands
that proof to all nine same-role and cross-role combinations.

## Independent verification

The Reviewer independently ran once:

- focused adapter;
- renamed S2-W22 reconciliation;
- writer/projection regression;
- runtime/state/projection package and impact tests;
- focused race `-count=30`;
- repository and repository-race tests;
- `go vet ./...`;
- frozen-file `gofmt -d`; and
- `git diff --check`.

All commands passed. Import/scope inspection found no Journal/write/
orchestration/Slice 3 expansion. The failure is the unproven and unimplemented
cross-record Event identity invariant.

VERDICT: FAIL
