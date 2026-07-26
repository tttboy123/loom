# Phase 1 Slice 3 Exit Contract Amendment 2 Review 1

- Date: `2026-07-26`
- Reviewer: `/root/s2_w4_contract_review`
- Mode: fresh independent read-only

## Findings

None.

## Evidence

The Reviewer confirmed:

- `agent-grant-identity/v1` replaces the missing global Grant-ID/token-hash
  collision proof with same-transaction Journal facts and never persists raw
  token material;
- only the separately invoked compatibility initialization may call
  `ReadAll`; normal `Issue`, `Authorize`, and `Revoke` commands replay touched
  streams;
- the identity stream remains inside the accepted AgentGrant
  `AppendBatchIfStreamHeads` writer rather than becoming a second authority;
- raising the existing head-expectation bound from eight to sixteen covers the
  maximum 13 streams for one Main plus two SubAgents on distinct Runtime
  instances, while 17 or more remains invalid before transaction start;
- the amendment adds no W6, hidden retry, checkpoint/cache authority, raw-token
  persistence, Runtime activation, or second writer; and
- the unrelated user-owned `AGENTS.md` and historical `PROGRESS.md` changes do
  not grant S3-W5 authority.

The Reviewer inspected the accepted S3-W3 collision implementation, current
Journal limit, both Exit Contract amendments, TECH-PLAN alignment, and the
surrounding diff. No files were edited and no long verification was run.

VERDICT: PASS
