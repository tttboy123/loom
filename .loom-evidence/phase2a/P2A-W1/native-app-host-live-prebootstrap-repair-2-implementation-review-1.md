# P2A-W1 Native App Host Live Pre-Bootstrap Repair 2 Implementation Review 1

Date: 2026-07-28  
Reviewer: independent read-only Reviewer `p2a_w1_implementation_review3`  
Verdict: `FAIL`

## Finding

Severity: `MEDIUM`

The Reviewer independently reproduced every published individual Candidate
hash, both live destination dry-runs, strict signed bundle checks, and clean
build behavior. The published complete bundle manifest digest could not be
reproduced from the evidence description because that description did not
freeze the exact field separators, mode representation, row terminator, or
canonical command.

The product builder was not found defective. This is a result-evidence
methodology defect.

## Verified non-findings

The Reviewer confirmed:

- the pre-repair clean mismatch is real and scoped to LC_UUID plus N_OSO
  timestamp metadata;
- the repaired order is correct: UUID-free and unsigned link, `strip -S`,
  then one final ad-hoc signature;
- package reset occurs before each build, so the test cannot pass through link
  cache reuse;
- forbidden-source, secret-negative, mode, architecture, bundle ID,
  no-UUID, no-symlink, and strict-signature checks remain;
- exact `loom`, `loomd`, app executable, Info.plist, and CodeResources hashes
  reproduce;
- no live allowance was consumed.

## Required repair

Freeze one canonical complete manifest byte format and use the same algorithm
inside the build fixture and evidence. A fresh Reviewer must reproduce the
published digest from a clean build before activation.

No production behavior or live state may change.
