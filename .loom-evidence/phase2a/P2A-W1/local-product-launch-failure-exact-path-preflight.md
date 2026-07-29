# P2A-W1 Local Product Launch Failure Exact-path Preflight

**Date**: 2026-07-29  
**Status**: `PASS - REVIEWED REPLACEMENT PREFLIGHT`  
**Production live allowance**: `0`

## Baseline

Before the exact-path execution:

- the resident observer was PID `85936`, state `running`, runs `1`;
- installed `loom`, `loomd`, wrapper, plist, and Journal hashes matched the
  frozen originals;
- SQLite integrity was `ok` with one Event;
- the exact run root, socket, native App, and App process were absent;
- the two historical crash reports retained their frozen hashes;
- Git staging was empty.

Two preliminary read-only aggregation commands had shell-harness errors before
the preflight: one reused zsh's special `path` variable, and one incorrectly
escaped positional references inside an embedded Bash command. Neither
created the run root, socket, private process, or any state mutation. The
corrected baseline audit then passed.

## Invalid exact-path evidence attempt

The bounded execution used:

- the exact retained failed Candidate;
- a fresh private SQLite copy and symlink-free private isolation copy;
- an empty environment with no Provider credential;
- `max-cycles=1`;
- the exact production socket path;
- no launchd bootstrap, bootout, restart, App launch, or installed-file change.

The harness cleaned all private files and the exact run root, but it did not
reach its final evidence summary. Its typed-status predicate required
`.local_product_snapshot` to be a nested object. Current accepted
`onlineStatusOutput` embeds `LocalProductSnapshot`, so the snapshot fields are
flattened at the top level. Cleanup removed the private stdout before the
invalid assertion was classified.

Therefore this execution is neither PASS nor a product FAIL. Its output is not
reconstructed or inferred.

## Post-cleanup preservation

The immediate post-cleanup audit proves:

```text
resident_pid=85936
resident_state=running
resident_runs=1
run_root_exists=no
candidate_processes=0
app_processes=0
journal_sha256=91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
sqlite_integrity=ok
events=1
staging_empty=yes
```

No replacement preflight is permitted until the same contract's bounded
harness reopen receives a fresh independent Contract Re-review PASS.

## Contract Re-review 3

Fresh independent Contract Re-review 3 returned `PASS` with no P0, P1, or P2
findings. It authorized exactly one replacement preflight using the corrected
top-level status envelope while retaining all bounded outputs through
classification.

## Replacement preflight result

The single authorized replacement used the same retained Candidate, private
SQLite and symlink-free isolation copies, empty environment, one-cycle bound,
and exact production socket path. It recorded the bounded measurements before
cleanup:

```text
result=PASS
socket_ready=1
candidate_processes=1
status_exit=0
typed_status=1
status_stdout_bytes=753
status_stderr_bytes=0
daemon_exit=0
daemon_timed_out=0
daemon_stdout_bytes=267
daemon_stderr_bytes=0
private_integrity=ok
private_events=1
app_processes=0
```

The typed predicate verified one top-level object with
`command=status`, `source_mode=daemon_api`, numeric `schema_version`, string
`view_version`, and top-level `runtimes`, `teams`, `runs`, `evidence`, and
`attention` arrays.

Post-cleanup preservation also passed:

```text
initial_pid=85936
final_pid=85936
resident_state=running
resident_runs=1
run_root_absent=yes
journal_sha256=91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
resident_integrity=ok
resident_events=1
report_one_sha256=f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
report_two_sha256=4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
staging_empty=yes
```

No launchd bootstrap, bootout, restart, App launch, installed-file change,
resident Journal write, Provider/Runtime execution, credential access, or live
allowance occurred. The invalid first harness result remains recorded and is
not reinterpreted. The replacement preflight is final and will not be repeated.
