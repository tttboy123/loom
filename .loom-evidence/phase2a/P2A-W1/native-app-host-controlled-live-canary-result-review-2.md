# P2A-W1 Native App Host Controlled Live Canary Result Re-review 2

**Date**: `2026-07-28`
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `PASS`
**Findings**: none

## Reproduced checks

The Reviewer independently matched the frozen hashes and modes:

| Surface | Hash prefix | Mode |
|---|---|---|
| original `loom` | `60c90ada...d47698` | `0755` |
| original `loomd` | `e5ab283c...b0014a` | `0755` |
| original `loomd-clean` | `ff556602...4f3e4` | `0700` |
| original LaunchAgent plist | `2a6dc3e1...bf1f4` | `0600` |
| resident SQLite | `91ae07e0...c8a4` | `0600` |

It also reproduced:

- original observer loaded and `running` from `loomd-clean`;
- target-process five-marker presence-only count `0`;
- SQLite `integrity_check=ok` and Event count `1`;
- `com.apple.provenance` xattr name present on original `loomd` and wrapper,
  without emitting its bytes;
- Candidate app, product run/socket, compatibility launcher, and
  Candidate/native app process absent;
- Git staging empty.

The Review performed no edit, stage, commit, install, bootstrap, restart,
Computer Use action, or secret-value inspection.

## Result accounting

`PASS` validates only coherent exact rollback evidence for the failed native
window canary. It does not turn the live result into product acceptance and
does not authorize retry, install, bootstrap, commit, or another canary.

P2A-W1 remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`, native allowance `0`,
and P2A-W2 remains locked.
