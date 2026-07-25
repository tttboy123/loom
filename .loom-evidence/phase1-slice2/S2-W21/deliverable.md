# S2-W21 Candidate Deliverable

- WorkItem: `S2-W21`
- Title: Runtime Discovery Read-Model Projection
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `501ac33`
- Contract SHA-256:
  `9061a1c1c40aa3be77a85933a77627af8b93a57cc75ea787996faff37a13cfbb`
- Amendment 1 SHA-256:
  `275ac81c5b1511f26f4a2f50c18fdf198af813753376d6fe89d7bc288936309a`

## Candidate

The Candidate extends the accepted rebuildable Snapshot with one deep-copied
Runtime inventory map. Only committed schema-v1
`RuntimeInstanceDiscovered` Events create or update records.

The Runtime-specific projector:

- rejects unknown/missing payload fields and invalid Event envelopes;
- revalidates RuntimeInstance content through accepted
  `runtime.NewRuntimeInstance` and requires canonical equality;
- validates canonical model ordering and discovery/source metadata;
- preserves caller-owned nonempty Event/correlation metadata;
- permits higher-sequence rediscovery while rejecting device/adapter identity
  drift; and
- preserves exact empty executable versions allowed by the accepted Runtime
  contract.

`RuntimeInstanceStatusChanged` remains an unknown-event no-op. Empty/absent
Journal input does not delete inventory or infer offline state.

## Mandatory RED

The focused RED command exited `1` only because `Snapshot.RuntimeInstances`,
projection-local `RuntimeInstance`, and the discovery apply behavior did not
exist. No syntax, dependency, environment, or unrelated failure occurred.

## Final file digests

```text
1c9e0bd26d0a6e5673a892a8deb80d87ad5ec50509333d79c22c98e20a602206  internal/projection/projection.go
9e5077130bb7f9a2f1711ea960b734c0a58cce63ae746827fabc6e35a29aa29c  internal/projection/projection_test.go
e74677aad7353ac75af6421141d272463aebe77099ed214c5d35408734d5a751  internal/projection/runtime_discovery.go
07e350178d4780351e6121850f7c536ce229a0a42b57111744277a31f08c8c64  internal/projection/runtime_discovery_test.go
```

## Controller verification

| Check | Result |
|---|---|
| focused S2-W21 test | `PASS` |
| projection package | `PASS` |
| projection/state/runtime/journal impact | `PASS` |
| focused race `-count=30` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| changed-file `gofmt` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `501ac33` |

Focused evidence covers:

- real SQLite S2-W20 writer-to-Journal-to-new-Projection rebuild equivalence;
- exact multi-instance fields, empty executable version, order independence,
  exact duplicate replay, empty/unrelated Journal behavior, and no
  status-from-absence inference;
- valid higher-sequence rediscovery and device/adapter identity-drift failure;
- malformed, missing, extra, invalid Runtime/capability/model/digest/envelope
  matrices, including caller-owned metadata semantics;
- failed, canceled, concurrent, and closed-Journal prior-Snapshot behavior;
- Runtime map/value/capability/model/input mutation isolation; and
- static import, non-disclosure, no-status-handler, and trust-boundary checks.

The accepted `projection.go` changes are limited to the frozen map, dispatch,
initialization, and clone integration. The two legacy test edits only add the
new empty map to literal expected Snapshots.

## Trust and residual state

The Event Journal remains the only fact authority. The Runtime catalog remains
the RuntimeInstance validation authority. This Candidate is a rebuildable read
model and cannot append Events, infer unobserved transitions, select a
RuntimeProfile, reserve capacity, run discovery, start a process, or activate a
Runtime.

No Pi/S2-W18/S2-W19 process, installed Pi, user Pi state, credential, network,
package manager, scheduler, daemon, Agent session, prompt, model call, Runtime
activation, push, merge, release, or external mutation was used.

## Implementation review

Fresh independent implementation review returned `PASS` with no blocking
findings. The Reviewer confirmed contract and Amendment 1 compliance, existing
projection preservation, Runtime/catalog authority, exact
Event/payload/rediscovery behavior, deep-copy isolation, test coverage, hashes,
and scope.

Evidence:
`.loom-evidence/phase1-slice2/S2-W21/implementation-review.md`

VERDICT: PASS
