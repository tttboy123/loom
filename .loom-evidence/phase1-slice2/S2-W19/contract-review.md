# S2-W19 Fresh Contract Review

- Reviewer: fresh independent read-only contract Reviewer
- Review date: 2026-07-25
- Branch/head: `codex/loom-platform-slice2` at `8c8fb9e`
- Contract SHA256:
  `fdf5cfad9c8f1c28fe0817d2f3bc98604a7dc78cca3d73688150c9763ac22b12`

## Findings

No blocking findings.

## Evidence

- Owned scope is limited to the child-package factory, its tests, and
  deliverable evidence; accepted S2-W18 and earlier files remain unchanged.
- The child package can call accepted `NewPiMetadataProcessRunner` and
  `runtime.NewPiRuntimeProbe` without a parent import or accepted-file edit.
- Current primary Pi package metadata publishes the fixed executable name
  `pi`; the contract freezes only that name and avoids ambient PATH and package
  manager trust.
- Root/search validation, ordered first-match behavior, normal absence, invalid
  shadow failure, and direct-name-only inspection are bounded and feasible.
- Symlink trust and residual risks remain consistent with S2-W18.
- Mandatory tests cover absence/error distinction, invalid shadows, drift,
  no-process construction, fake-only integration, non-disclosure, and static
  dependency boundaries without installed Pi.
- Construction is not observation, and observation is not activation. Daemon
  scheduling, persistence/projection, RuntimeProfile selection, Agent session,
  Run/Grant, and Slice 3 execution remain excluded.

No product tests, installed Pi command, file write, or external mutation was
performed by the Reviewer. `git diff --check` passed.

VERDICT: PASS
