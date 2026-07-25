# S2-EXIT-1 Repair 1 Contract

- Parent contract:
  `.loom-evidence/phase1-slice2/S2-EXIT-1/contract.md`
- Trigger: Implementation Review 1 `FAIL`
- Repair class: missing direct test proof; no product defect found
- Risk: `STRICT`

## Owned repair files

- `internal/app/runtime_daemon.go`
- `internal/app/runtime_daemon_test.go`
- `.loom-evidence/phase1-slice2/S2-EXIT-1/**`
- `docs/CURRENT.md`

All other product/test files remain locked.

## Product constraint

Production behavior and exported API remain unchanged. The only permitted
production edit is extraction of the existing discovery next-sequence
calculation into one unexported pure helper accepting projected Runtime facts
and returning the same sequence/error. This makes `math.MaxInt64` overflow
directly testable without forging an impossible contiguous Journal history.

## Mandatory RED markers

Before repair implementation, one test-only marker guard must fail only because
these unique case-local markers are absent:

```text
s2_exit_metadata_duplicate_identity_zero_append
s2_exit_metadata_invalid_identity_zero_append
s2_exit_metadata_non_utc_zero_append
s2_exit_metadata_cancellation_zero_append
s2_exit_metadata_sequence_overflow
s2_exit_configuration_complete_rejection_matrix
s2_exit_configuration_typed_nil_identity
```

## Required proof

1. Duplicate and invalid identity sources fail before append; real SQLite Event
   count remains zero.
2. Non-UTC clock time fails before append; Event count remains zero.
3. Identity-source cancellation returns exact `context.Canceled`; Event count
   remains zero.
4. Direct pure-helper proof rejects next-sequence overflow and preserves new,
   discovery-latest, and status-latest sequence results.
5. Table-driven config proof covers:
   absent/non-`0700`/non-directory/symlink state parent; directory, symlink, and
   wrong-mode state path; invalid/non-`0700`/symlink isolation root; absent and
   duplicate Runtime directories; interval, timeout, and max-cycle lower/upper
   bounds; and typed-nil identity.
6. Every rejected config proves no state lock or new state file was created.
7. Existing focused, status, restart, recovery, concurrency, command, live
   canary, full repository, race, vet, format, and diff checks remain green.

## Review gate

Fresh Repair 1 contract Reviewer `PASS` is required before marker RED. After
GREEN and the full matrix, a fresh independent Repair 1 Implementation
Reviewer is required.
