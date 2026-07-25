# S2-W32 Implementation Repair 1 Contract

- WorkItem: `S2-W32`
- Status: `REPAIR_CONTRACT_FROZEN`
- Active contract SHA-256:
  `b6c594213350e7a0c1d5bf1dbf5bc7de429d02c8cc04833efbba6f49625ea77b`
- Pre-repair product SHA-256:
  `f50a6d5adb7940dfa44578c40d6956ce15bfe4f952dec5fc9158f05cfd9e9c97`
- Pre-repair test SHA-256:
  `c60cbd01dc3a5c7414a7fdab68285429770c80a415fe22a3e7bb5859a203018c`
- Trigger: S2-W32 Implementation Review 1 `FAIL`

## Scope

One bounded digest-completeness repair:

1. add the existing private `planned` fact to the canonical versioned digest
   payload;
2. populate it from the Candidate during digest construction; and
3. add a direct single-field mutation proving `planned` changes the digest.

Only `internal/app/runtime_write_plan.go`,
`internal/app/runtime_write_plan_test.go`, and S2-W32 repair/deliverable
evidence may change. ADR-0007, the frozen public API, classification,
validation, context, inventory, counts, errors, and trust boundary remain
unchanged.

## Mandatory Repair RED

Before product repair, extend the per-fact sensitivity table with one
`planned` mutation from true to false. Focused S2-W32 tests must fail because
the digest is unchanged. No other test may be weakened or removed.

## Acceptance

- `runtimeObservationWritePlanDigestPayload` contains version plus every
  exposed Candidate fact, including `Planned`.
- The digest constructor copies the exact Candidate `planned` value.
- Isolated mutation of `planned` changes the digest.
- All original focused and strict matrix checks remain green.
- Product/test imports and pure no-writer/no-authority boundary remain
  unchanged.
- Fresh independent Repair 1 implementation review `PASS` is required before
  acceptance or commit.

This is Repair 1 of at most three.

VERDICT: REPAIR_CONTRACT_FROZEN
