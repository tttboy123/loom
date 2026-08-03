# P2B-W1 Controlled Offline Canary

Date: 2026-08-03

Status: `PASS`

Attempt: `canary-001` — consumed; it must not be rerun or replaced without a
reviewed same-contract reopening.

Root:

```text
/Users/lune/Library/Application Support/Loom/phase2b-live/canary-001
```

Preflight:

- root and vertical fixture directories: `0700`;
- manifest and SQLite: `0600`;
- source lock SHA-256:
  `411593c724dfc78424dbaf99c4936b2367b92dd65b402b6841220d5e738ef97c`;
- Implementation Review: PASS, P0/P1/P2 all zero;
- Provider credential variables and proxy variables removed from the canary
  process environment;
- deterministic loopback only; no Provider, OAuth, paid call or network action.

Executed once:

```text
LOOM_P2B_CANARY_ROOT=<canary-001> \
go test -count=1 -v ./cmd/loomd \
  -run '^TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage$'
```

Result: `PASS` in 15.61 seconds.

Retained authority and Artifact evidence:

- SQLite integrity: `ok`;
- Event count: 296;
- duplicate Event IDs: 0;
- duplicate idempotency keys: 0;
- `ContextPacketCommitted`: exactly 1;
- `ParentContinuationAuthorized`: exactly 1;
- `ParentHandoffEffectCompleted`: exactly 1;
- `SideTaskDecisionCommitted`: exactly 1;
- retained content-addressed Artifacts: 18;
- Artifact filename/digest failures: 0;
- SQLite SHA-256:
  `f47e4095ad8da5799ce07ca6ceb8fe0ac29e13dc491b5c3ab3854adc996d037a`;
- manifest SHA-256:
  `ece9b16eeab8738013f04d36db96d3b5011ddba54e5d91ef5af3f0cb1bbeede2`;
- SQLite payload scan for known credential/Bearer sentinels: 0;
- no remaining canary socket, lock or process.

Same-attempt source-locked surface checks:

- TUI explicit proposal -> confirm -> typed decision: PASS;
- Swift strict Side-task models: 3 tests PASS;
- Swift authoritative reconnect decision binding: PASS;
- Swift native sheet host/window architecture: 2 tests PASS.

The real socket-to-Swift Side-task read is part of the one vertical test and
passed. After the Mac was manually unlocked, a read-only Computer Use
inspection exposed a presentation completeness defect. The defect was repaired
under the same P2B-W1 boundary with mandatory RED, full verification and an
independent Implementation Re-review PASS. The consumed authority canary was
not rerun.

The reviewed presentation delta is bound by:

- supplemental lock SHA-256:
  `8e19132d75d4ae10da25f07d11f1e211a4b12ee40eee764961adf610ea75e970`;
- Implementation Re-review: PASS, P0/P1/P2 all zero.

A replacement visual-only native inspection then passed against bounded,
read-only retained-canary data. It showed purpose, status, summary, finding,
risk, Evidence/Artifact references, usage, explicit empty uncertainty and
scope, next action and decision availability. Evidence:

- screenshot SHA-256:
  `39fbb354bff36997283a0cd96a3bfc2a1092a633accaf279fc5580ef136832e6`;
- accessibility transcript SHA-256:
  `54c5e888507fff82a2a30bd4f6124ee8b9a8a3bac851223fd9bc02a1317f5f08`;
- result record: `native-drawer-visual-only-result.md`.

The replacement was visual-only: no authority write, dispatch, product socket,
Provider, credential, network action or second authority canary occurred. The
retained SQLite, manifest and original source-lock hashes remained unchanged,
and both temporary visual host processes were terminated.
