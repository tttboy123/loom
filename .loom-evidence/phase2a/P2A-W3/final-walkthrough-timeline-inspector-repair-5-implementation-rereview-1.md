# P2A-W3 Timeline and Inspector Repair 5 Implementation Re-review 1

Date: 2026-08-03

Verdict: `FAIL`

## Findings

- P0: none.
- P1: Changes and Evidence accepted Timeline pages with `gap != nil` or
  `has_more=true`. A legitimate stream-gap page can contain no records, and a
  first page can omit later matching records. The UI could therefore render
  “No ... recorded” while authoritative history was unavailable or incomplete.
- P2: no separate finding; the missing gap and pagination regressions are part
  of the P1.

The Reviewer independently reproduced all four file hashes and combined source
lock `d616d7ac1e5b98db887a65fcdffd02a14222f185767cd63cd56ef448ce12c405`,
ran focused Go and Swift tests plus complete `internal/api` and Swift tests,
and confirmed all other requested boundaries:

- present-empty, malformed, mismatched and unbound lineage fail closed;
- attempt-only rejection traverses the complete mapper path;
- the read-only interface seam creates no second authority;
- the four Inspector sections are distinct and safe for complete history;
- no schema, writer, command, CAS, Grant, generation, Evidence, Projection
  authority or P2A-W4 expansion occurred.

root005 was not authorized.
