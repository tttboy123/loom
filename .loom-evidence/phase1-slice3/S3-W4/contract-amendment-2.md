# S3-W4 Contract Amendment 2 — Rooted Directory-FD Traversal

- Baseline: `47b4b50`
- Amended Contract SHA256:
  `0157df19ca32255c8356dbf34c12b5de682592845286ec1e6bfbb6e01d82b811`
- Trigger: pre-implementation-review security self-audit
- Date: `2026-07-26`

## Problem

Amendment 1 requires every source and final-workspace path component to remain
beneath one bound tree root while a regular file is opened without following
symlinks. A lexical `filepath.WalkDir` plus final-component `O_NOFOLLOW`,
pre/post `Lstat`, and device/inode comparison does not fully prove that
intermediate directory components were not replaced by symlinks between the
walk and final open.

The frozen owned-file list has no Supervisor platform-specific file in which
to implement Unix directory-fd traversal while retaining a compiling,
fail-closed non-Unix build.

## Bounded owned-file amendment

Add exactly two S3-W4 product files:

- `internal/supervisor/managed_workspace_unix.go`
- `internal/supervisor/managed_workspace_other.go`

No existing owned file is removed. No public API, dependency, Event schema,
WorkItem, Slice exit item, or capability changes.

## Exact replacement proof

On Unix, source and final-workspace enumeration must:

1. bind the final root directory with a no-follow directory open and exact
   path-versus-descriptor identity;
2. enumerate each directory from its already-open descriptor;
3. inspect each child with descriptor-relative `fstatat` and
   `AT_SYMLINK_NOFOLLOW`;
4. open every child directory or regular file with descriptor-relative
   `openat`, `O_NOFOLLOW`, and `O_CLOEXEC`;
5. compare device, inode, type, link count, mode, and size before open, on the
   opened descriptor, after the bounded read/recursion, and at the parent
   descriptor after use;
6. reject any symlink, hardlink, device, socket, FIFO, identity change,
   unsupported type, limit violation, or uncertainty;
7. skip top-level `.git` from the root directory listing before any stat/open;
8. derive every manifest path only from the descriptor-rooted traversal and
   validated entry names.

The existing direct dependency `golang.org/x/sys` may be used; no dependency
is added.

On non-Unix, the platform file must compile and return the existing typed
fail-closed managed-workspace error before source copy or process start.

Tests must retain the controlled final-component replacement proof and add an
intermediate-directory symlink-replacement proof. Unix and non-Unix compile
evidence remain mandatory.

## Scope

All other S3-W4 contract and Amendment 1 clauses remain frozen. No shell,
network, daemon, credential, Provider/model, container, or filesystem-sandbox
claim is added.

VERDICT: FROZEN
