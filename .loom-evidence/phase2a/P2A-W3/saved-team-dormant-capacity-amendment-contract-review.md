# P2A-W3 Saved-Team Dormant Capacity Amendment Contract Review

**Date**: 2026-08-02  
**Reviewer**: fresh independent read-only Reviewer  
**Baseline/HEAD**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Reviewed contract SHA-256**:
`39824721399e2767fc7387202639e2829dcf0d1d65ee86b0bb9f71f806727e73`  
**Reviewed source-lock SHA-256**:
`ed9413d8c0082fa8d42353c892a539c3114e8c70ee47fa4c4f3d1b0c7c5d6844`

## Findings

- P0: none.
- P1: none.
- P2: none.

The Reviewer independently verified the repository identity and every frozen
digest. The two reopened authority files were unchanged before implementation.

The Reviewer confirmed:

1. current binding code counts every selected role, reproducing the accepted
   capacity-1 contradiction;
2. accepted S2-W12 creates one Main seed and only dormant SubAgent seeds, with
   `createSubAgentInstances=false`;
3. accepted S2-W13 creates exactly one Main AgentInstance and retains
   `activeSubAgentCount=0`;
4. retaining and fully validating every dormant SubAgent binding while counting
   only the active-on-materialization Main is semantically exact;
5. dispatch/run-time capacity and future activation remain behind current
   binding, CAS/capacity and generation-fencing authorities;
6. the two reopened product/test files are sufficient; no schema, caller,
   downstream record or StateWriter change is implied; and
7. the RED/gates preserve offline/incompatible/disabled/model/adapter/
   capability/malformed/source/digest/tamper failures and zero-output semantics.

## Section 9 answers

1. Active-on-materialization versus dormant is exact and consistent with
   accepted S2-W12/S2-W13 behavior: **yes**.
2. The wording weakens dispatch/run-time capacity or future activation checks:
   **no**.
3. Two reopened files are sufficient: **yes**.
4. RED and verification preserve the original binding trust checks: **yes**.
5. Safe to implement while live remains locked: **yes**.

No file was edited, staged or committed by the Reviewer. No Provider,
credential, Runtime process or live action was used.

**VERDICT**: `PASS`
