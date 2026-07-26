# Phase 1 Slice 3 Exit Contract Amendment 1 Review 1

- Date: `2026-07-26`
- Reviewer: `/root/s2_w2_contract_review`
- Mode: fresh independent read-only

## Findings

### P1 — AgentGrant global collision authority was not preserved

Amendment 1 prohibited `ReadAll` in AgentGrant write commands but did not
replace the accepted global Grant-ID and token-hash collision proof. Existing
S3-W3 replays every historical Grant stream into global `byID` and `byHash`
indexes before `Issue`. Reading only the current Run's Grant stream cannot
detect collision with another Run's historical Grant.

Required repair: either preserve the accepted full replay for that command or
freeze a Journal-authoritative global uniqueness mechanism, including a
bounded compatibility path for pre-index Grant Events.

### P1 — two-Runtime ready-set dispatch exceeds the accepted CAS head limit

Two SubAgents bound to distinct Runtime instances touch at least nine streams:
one Team execution, two WorkItem, two Run, two Runtime status, and two Runtime
capacity streams. The accepted Journal limit is eight head expectations.

Required repair: reopen and bound the expectation limit or constrain the
dispatch shape without weakening the authorized concurrency proof.

## Preserved constraints

The Reviewer found that Amendment 1 otherwise:

- kept S3-W5 as the fifth and final WorkItem;
- prohibited W6 and thin wrapper decomposition;
- preserved the Phase 1 checkpoint/cache exclusions; and
- aligned its high-level Main/SubAgent, generation, terminal, and independent
  lineage result with `TECH-PLAN.md`.

No files were edited and no long verification was run by the Reviewer.

VERDICT: FAIL
