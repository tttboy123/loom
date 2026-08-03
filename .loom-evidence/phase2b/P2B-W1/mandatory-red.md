# P2B-W1 Mandatory RED

**Date**: 2026-08-03  
**Frozen contract SHA-256**:
`2bc98c99c301a014d8dabe7491bd53287148c10803537b6de9d420492c22a866`  
**Status**: `RED ACCEPTED`

Only frozen test-owned paths changed before this evidence:

- `internal/localipc/protocol_test.go` SHA-256
  `736a1c61d4391eaf5ac21d7d6fd1df97df2f0db8a43854a14a6c7747b228b0b9`;
- `internal/api/local_product_read_test.go` SHA-256
  `9184376fc2674d7bbba028eb8588add936405d77a7cd994fbec2974cc5614565`.

## Behavior failure 1: strict method absent

```text
go test -count=1 ./internal/localipc \
  -run TestSideTaskHandoffIsTheOnlyAcceptedSideTaskMethod

--- FAIL: TestSideTaskHandoffIsTheOnlyAcceptedSideTaskMethod
side_task_handoff decode error = invalid local IPC request: unknown_method
```

The strict protocol still rejects the one frozen P2B method. Aliases remain
rejected. This is the expected missing-product behavior, not a syntax or
fixture failure.

## Behavior failure 2: authoritative collection absent

```text
go test -count=1 ./internal/api \
  -run TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView

--- FAIL: TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView
snapshot SchemaVersion:2
```

The authoritative snapshot is still schema 2 and contains no required
`side_tasks: []` collection. Existing projection, Team and Mission fixture
construction succeeded before the assertion, so this is a direct product
capability failure.

`gofmt` and `git diff --check` pass for both RED test files. No product,
authority, schema implementation, Runtime, Provider, live, staging or commit
action occurred before RED.

VERDICT: RED
