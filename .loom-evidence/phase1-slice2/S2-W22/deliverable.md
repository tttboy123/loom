# S2-W22 Candidate Deliverable

- WorkItem: `S2-W22`
- Title: Observed Runtime Status Reconciliation Candidate
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `366bc48`
- Contract SHA-256:
  `5f8f2a1e06950d927925fb49b7c70f7c177ebf1229193d47b30cff603e989de9`

## Candidate

The Candidate is a pure bounded Runtime-domain reconciliation result. It
revalidates a zero-to-32 S2-W21-derived status baseline and a zero-to-32
accepted S2-W2 snapshot, then returns ordered transitions only where the same
stable Runtime identity is explicitly observed with a different accepted
status.

New current IDs and absent baseline IDs produce no transition. Device or
adapter drift fails closed. Non-status inventory changes do not create status
transitions. The Candidate and complete baseline have independent
lowercase-SHA256 digests and copied accessors.

## Mandatory RED

The focused command exited `1` only because the frozen baseline, transition,
Candidate, reconciliation, and digest symbols did not exist. No syntax,
dependency, environment, or unrelated failure occurred.

## Final file digests

```text
c81658d781889d4b0e240539db45bc93cd9579263797184d59eb28a40e12c3ee  internal/runtime/status_reconciliation.go
5b9fc0b01e6717b0e2a0d1807d3410218536ea3cddcc7bb29acde24d621e6e84  internal/runtime/status_reconciliation_test.go
```

## Repair 1

Implementation Review 1 found no product defect and returned `FAIL` for two
test-proof gaps. Repair 1 is frozen at
`.loom-evidence/phase1-slice2/S2-W22/repair-1-contract.md`
(`8461af09f56b0de5bb5220b80c8fcdeb5157497fbec930d45f2d9dae08fc3f9d`).
It changes tests only:

- adding a second valid baseline entry changes both `BaselineDigest` and
  `CandidateDigest`, while reordering the expanded set changes neither; and
- independently changing display name, executable version, canonical
  capabilities, capacity, canonical model IDs, or source probe ID while stable
  identity and status are unchanged returns a valid zero-transition Candidate.

The product digest remains exactly
`c81658d781889d4b0e240539db45bc93cd9579263797184d59eb28a40e12c3ee`.

## Controller verification

| Check | Result |
|---|---|
| focused S2-W22 test | `PASS` |
| runtime package | `PASS` |
| runtime/state/projection impact | `PASS` |
| focused race `-count=50` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `366bc48` |

The first concurrent repository-test invocation encountered only the pre-existing
three-second Pi metadata-command timeout in
`TestPiLocalRuntimeProbeFactoryUsesOnlyFixedOrderedConfiguredCandidates`; the
same concurrent matrix's repository race passed. The target test then passed
sequentially with `-count=20`, and `go test ./... -count=1` passed sequentially.
No Pi adapter or other out-of-scope file was changed.

Focused evidence covers:

- same status and all twelve distinct accepted status pairs;
- independent same-status display-name, executable-version, capabilities,
  capacity, model-ID, and source-probe changes as zero-transition no-ops;
- deterministic multi-transition ordering and baseline reorder equality;
- baseline-set addition sensitivity for both baseline and Candidate digests;
- completed empty discovery, current-only, and baseline-only no-inference
  behavior;
- device/adapter drift, invalid/duplicate/oversized baseline, invalid/forged/
  oversized source, and nil/canceled/deadline context matrices;
- input/source/Candidate/accessor mutation isolation;
- every baseline and transition semantic field, source/baseline digest, and
  transition-count digest sensitivity; and
- static pure-domain import, no-write/no-probe/no-execution, non-disclosure,
  and scope checks.

## Trust and residual state

S2-W2 remains discovery normalization/digest authority. S2-W21 remains the
rebuildable historical inventory source; the caller owns bounded conversion to
the untrusted baseline. This Candidate is not an Event, a state transition, a
status-from-absence inference, Runtime selection, reservation, or activation.

No Event/Journal/projection/schema/file/config mutation, probe, Pi/S2-W18/
S2-W19 process, installed Pi, user Pi state, credential, network, package
manager, scheduler, daemon, Agent session, prompt, model call, Runtime
activation, push, merge, release, or external mutation was used.

## Review result

Fresh independent Implementation Review 1 found no product defect and returned
`FAIL` for the two test-proof gaps recorded above. Fresh independent Repair 1
Implementation Review 2 independently reran the full strict matrix, confirmed
both gaps closed with test-only changes, confirmed the product hash remained
unchanged, and returned `PASS` with no blocking findings:
`.loom-evidence/phase1-slice2/S2-W22/implementation-review-2.md`.

VERDICT: PASS
