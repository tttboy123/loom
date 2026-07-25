# S2-W26 Candidate Deliverable

- WorkItem: `S2-W26`
- Title: Bounded Configured Runtime Discovery Scan
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `38d914b`
- Contract SHA-256:
  `5593c5de01b91c0937907e863f864e7edb686390274b8492e4fff0e32f2dcdd0`
- Contract review SHA-256:
  `0bbbeedca1f79ec1a6f6181701468efebf05ccc2a8a5580775fedb84b08134e7`

## Contract and review

The frozen contract adds one caller-triggered, bounded coordination layer
between configured Runtime probe factories and accepted S2-W2 discovery. Fresh
independent contract review returned `PASS` with no blocking findings before
any product or test change.

The boundary does not choose status-versus-rediscovery persistence order. It
does not schedule a scan, query or write the Journal, build a baseline,
reconcile status, infer status from absence, start a Runtime, or activate
execution authority.

## Mandatory RED

Before `scan.go` existed, the focused command exited `1`. Compilation failed
only because the frozen `ProbeFactory`, error, limit, and
`DiscoverConfiguredRuntimes` symbols did not exist. No test syntax, existing
package, dependency, or environment failure occurred.

## Candidate

The Candidate:

- rejects nil context, more than 32 factories, and any nil or typed-nil factory
  before a factory call;
- copies and prevalidates the entire factory slice;
- invokes each factory exactly once in caller order;
- accepts only canonical absent `(nil, false, nil)` and present
  `(nonnil, true, nil)` results;
- collects every present probe before any observation;
- preserves exact context errors and wraps non-context factory errors with both
  the frozen sentinel and source error;
- delegates the complete collected probe set once to accepted
  `runtime.DiscoverRuntime`; and
- returns only its accepted immutable snapshot or a zero snapshot on failure.

An empty deterministic temporary search configuration verifies that the
accepted S2-W19 Pi factory satisfies the frozen interface and returns explicit
absence without starting a process.

## Candidate file digests

```text
2e8cbdc007e5ab31c5f95e1c70afabf006f02c5682820ab28ab057de3f1411d4  internal/runtime/discoveryscan/scan.go
5bbfb4f6f7c911ad928403bbf74e52e8c103aeb7d648daf75e91fca440089a74  internal/runtime/discoveryscan/scan_test.go
```

## Implementation Repair 1

Fresh independent Implementation Review 1 returned `FAIL` on one test-evidence
gap: the Candidate rejected 33 factories but did not directly prove that the
inclusive exact limit of 32 is accepted.

Repair 1 is test-only. Its fresh independent contract review returned `PASS`
with `scan.go` explicitly read-only. Mandatory Repair RED exited `1` because no
exact-maximum test existed. The repair adds exactly 32 distinct canonical-absent
factories, requires success and a valid empty snapshot, checks caller order, and
requires exactly one call per factory. The existing 33-factory rejection proof
remains unchanged.

`scan.go` retains its original SHA-256
`2e8cbdc007e5ab31c5f95e1c70afabf006f02c5682820ab28ab057de3f1411d4`.
The complete strict matrix below was rerun after Repair 1 and passed.

## Controller verification

| Check | Result |
|---|---|
| focused S2-W26 | `PASS` |
| discoveryscan + runtime packages | `PASS` |
| runtime + Pi adapter + discoveryscan impact | `PASS` |
| focused race `-count=50` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt -d` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| import/static/scope boundary | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `38d914b` |

Focused evidence covers:

- empty/all-absent deterministic snapshot equality with no fabricated status;
- factory call order, input-slice copying, present filtering, delayed
  observation, accepted probe-ID observation order, exact snapshot content,
  digest equality, and accessor isolation;
- count, nil, typed-nil, context, contradictory-result, source-error, later-call
  suppression, and no-partial-observation failures;
- accepted S2-W2 invalid/duplicate probe, observation/content/cancellation, and
  mutation behavior through the new boundary; and
- concrete S2-W19 interface/absence integration plus static no-write,
  no-persistence, no-scheduler, no-execution-authority boundaries.

## Trust and residual state

Accepted S2-W2 remains the only discovery coordinator and snapshot authority.
Accepted S2-W19 remains concrete local Pi factory authority. The new package
does not import that adapter in product code and creates no second discovery,
status, persistence, or execution authority.

No installed Pi executable, user Pi state, credential, network, package
manager, process, Journal/state/projection write, scheduler, daemon, Agent
session, prompt, model call, Runtime selection/reservation/activation, push,
merge, rebase, reset, release, or external mutation was used.

## Review gate

Fresh independent Implementation Review 1 returned `FAIL` only for the missing
exact-32 proof. Repair 1 contract review returned `PASS`, Repair RED was
observed, and the test-only repair and complete strict matrix are GREEN. Fresh
independent Repair 1 implementation review independently passed the complete
matrix, confirmed the only finding is closed with product unchanged, and
returned `PASS` with no findings:
`.loom-evidence/phase1-slice2/S2-W26/implementation-review-2.md`.

VERDICT: PASS
