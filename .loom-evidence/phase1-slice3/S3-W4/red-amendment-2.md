# S3-W4 Amendment 2 Behavioral RED

- Date: `2026-07-26`
- Baseline: S3-W4 preliminary Candidate on `47b4b50`
- Contract amendment:
  `.loom-evidence/phase1-slice3/S3-W4/contract-amendment-2.md`
- Reviewed amendment SHA256:
  `6f9330b2d549f70808a207c5d5c58518cf9a879e6e95b93a23a19a7ec6c152e9`

## Command

```text
go test ./internal/supervisor -run '^TestManagedWorkspaceRejectsHardlinksSymlinksAndBounds/intermediate_directory_replacement_during_rooted_open$' -count=1
```

## Result

`RED` — command exited `1`.

The test deterministically replaced the already-inspected `nested` directory
with a symlink to the renamed original directory immediately before the old
final-component open. The old lexical `WalkDir` plus absolute-path
`O_NOFOLLOW` implementation accepted and copied the tree, so the test failed
with a non-nil managed workspace and nil error.

This is the exact frozen Amendment 2 gap: final-component no-follow did not
bind intermediate path components to the enumerated root.

VERDICT: RED
