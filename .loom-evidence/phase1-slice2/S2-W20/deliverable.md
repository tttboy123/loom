# S2-W20 Candidate Deliverable

- WorkItem: `S2-W20`
- Title: Runtime Discovery Event Writer
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `1b2c486`
- Contract SHA256:
  `d403da481f25754d144f3721e696d21ba67479626b2a91c1551f6f87a199e0b6`
- Repair 1 contract SHA256:
  `6cead182bd9c872cbedc386c3596580610a24b8b2b608d2dc9fad67dc17a596a`
- Repair 2 contract SHA256:
  `b717d5ee61c9c012f849908f7432c8b2c75008409110de36143023ba0d9c699a`

## Candidate

The Candidate adds only the frozen `internal/state` writer. It revalidates one
non-empty accepted S2-W2 snapshot and exact caller-owned Event metadata, builds
one canonical `RuntimeInstanceDiscovered` Event per observation, appends the
complete copied batch once through the accepted S2-W14 port, requires an exact
immutable same-order result, and returns a copied digest-bound commit
Candidate.

Empty snapshots are rejected without fabricating offline state. Payloads
contain the discovery digest, source probe, normalized RuntimeInstance,
capabilities, and model IDs, but no executable/search/isolation path,
environment, output, credential, prompt, session, or user Pi state.

## Mandatory RED

The focused command exited `1` only because the frozen S2-W20 writer, input,
Candidate, and typed-error symbols did not exist. No syntax, dependency,
environment, or unrelated failure occurred.

## Final file digests

```text
35740067104d3d50fe6160a1bb554f19e894f4fe3ccffd602825b943785cea6c  internal/state/runtime_discovery_writer.go
f4358ab98b4f95ecfa6624dc9640e1f5d12027fbfa7b9c7903f43a65fbd5e701  internal/state/runtime_discovery_writer_test.go
```

## Controller verification

| Check | Result |
|---|---|
| focused S2-W20 test | `PASS`, `ok internal/state 0.236s` |
| `go test ./internal/state ./internal/runtime ./internal/journal -count=1` | `PASS` |
| focused race `-count=30` | `PASS`, `ok internal/state 2.354s` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `1b2c486` |

Repair 1 repeated the complete strict matrix with the same passing results.
Its focused RED failed exactly because the previous comparison accepted the
same `EmittedAt` instant with a non-UTC location. The minimal repair requires
UTC locations for both requested and returned Events.

Repair 2 is test-only. It adds the frozen direct proof that an already-expired
deadline returns `context.DeadlineExceeded`, a zero Candidate, and zero
appender calls. Production remains byte-for-byte unchanged, and the complete
strict matrix passes again.

Focused evidence covers:

- nil/typed-nil appender, nil/canceled context, all invalid metadata, exact
  observation coverage, empty and oversized snapshots, and zero append calls;
- stable multi-observation Event order and complete exact payload/envelope;
- one copied batch, exact result validation, and nil/short/long/reordered/
  sequence/payload/timestamp-location mutation rejection;
- input/source/appender/accessor/payload mutation isolation;
- CommitDigest sensitivity to source digest, event count, every immutable
  envelope field, and payload bytes;
- real SQLite exact retry, idempotency conflict, sequence conflict,
  partial-existing conflict, and forced second-insert all-or-none rollback; and
- static absence of concrete SQL, process, network, projection, Team, daemon,
  scheduler, credential, UI, or activation imports.

No Pi/S2-W18 process, installed Pi, user state, credential, network, package
manager, scheduler, daemon, projection update, RuntimeProfile selection, Agent
session, prompt, model call, Runtime activation, push, merge, release, or
external mutation was used.

## Trust and residual state

S2-W2 remains the discovery normalization/digest authority. The caller owns
unique Event/idempotency IDs and correct next per-instance sequence. S2-W14
remains the atomicity/conflict authority. A discovered Event is inventory
evidence, not RuntimeProfile selection or Run authority. Empty/absent scans
cannot infer offline state without historical reconciliation and are therefore
not committed by this WorkItem.

## Implementation review

Fresh independent Implementation Review 1 returned `FAIL` on the missing
UTC-location exactness check. Repair Review 2 confirmed that product repair but
returned `FAIL` on missing direct deadline-context proof. Fresh independent
Repair 2 implementation review returned `PASS` with no blocking findings. It
confirmed both prior findings are closed, Repair 2 is test-only, hashes match,
and the complete contract, state-authority, non-disclosure, and scope
boundaries remain intact.

Evidence:

- `.loom-evidence/phase1-slice2/S2-W20/implementation-review-1.md`
- `.loom-evidence/phase1-slice2/S2-W20/repair-1-contract.md`
- `.loom-evidence/phase1-slice2/S2-W20/implementation-review-2.md`
- `.loom-evidence/phase1-slice2/S2-W20/repair-2-contract.md`
- `.loom-evidence/phase1-slice2/S2-W20/implementation-review-3.md`

VERDICT: PASS
