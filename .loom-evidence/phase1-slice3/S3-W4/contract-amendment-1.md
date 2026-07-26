# S3-W4 Contract Amendment 1 — Hardlink Proof and Result Semantics

- Baseline: `47b4b50`
- Parent Contract SHA256:
  `459c488d6d8b556872d8e0aeaa68f4e9742df51a9c6c4beada14401d0a1e2896`
- Trigger: Contract Review 1 `FAIL`
- Date: `2026-07-26`

## 1. Hardlink and path-race closure

S3-W4 execution support is fail-closed on Unix. Before reading or returning any
regular source/workspace file, product code must prove:

- exact link count is one;
- the path remains beneath the resolved bound tree root;
- no path component is a symlink;
- a no-follow open succeeds;
- pre-open path identity and opened descriptor identity match by device,
  inode, type, and link count;
- size/mode/identity remain unchanged after the bounded read.

Any uncertainty, link count other than one, identity change, or unsupported
link-count/no-follow proof returns `ErrManagedWorkspace`. The top-level `.git`
entry is never opened.

The child is stopped and reaped before final workspace scanning, so it cannot
race final change collection. Original source pre-start and post-execution
manifest checks retain the same identity proof and detect concurrent source
changes. Unix tests must cover:

- a source hardlink to a file outside the source root;
- two in-tree names for one inode;
- a child-created workspace hardlink to an outside file;
- a child-created in-workspace hardlink;
- symlink replacement during a controlled open/read seam;
- cleanup and zero returned content after every rejection.

Non-Unix builds must compile but return a typed fail-closed unsupported
execution/workspace error before source copying or process start. They cannot
claim managed execution, hardlink safety, or process-group cleanup.

## 2. Child result semantics

A valid child may report either:

- `succeeded` with empty reason, which S3-W2 projects only to WorkItem
  `ready_for_review`; or
- `failed` with one bounded opaque reason, which commits the Run/WorkItem
  failure terminal.

Neither result can mark a WorkItem `done`. `cancelled` remains
Supervisor-generated only. Tests must cover both child result statuses and
prove exact terminal projection.

## Scope

No public API, Event schema, owned file, WorkItem, or capability changes. All
other S3-W4 contract clauses remain frozen.

VERDICT: FROZEN
