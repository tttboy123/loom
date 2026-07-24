# S2-W2 Developer Candidate Handoff

- WorkItem: `S2-W2`
- Title: Deterministic Runtime Discovery Coordination
- Original frozen contract SHA256:
  `8ab4fdfadda9808a4ed2c0f750efc850d60db48e8f147758c8356e5702d53ed1`
- Active normalized contract SHA256:
  `7f8dab95bf7a98d3e7615252fc3d490fa5108fad956b29eea82c3b049e96d3c9`
- Contract Amendment 1 removed only the final empty line and passed a fresh
  independent amendment review.
- Developer-owned product files changed:
  - `internal/runtime/discovery.go`
  - `internal/runtime/discovery_test.go`
- Developer-owned evidence file changed:
  - `.loom-evidence/phase1-slice2/S2-W2/deliverable.md`

## RED and GREEN Record

- Original mandatory RED:
  - Command: `go test ./internal/runtime -run 'TestDiscoverRuntime|TestRuntimeDiscovery' -count=1`
  - Exit: `1`
  - Classification: valid RED; failure was solely due to undefined frozen symbols including `RuntimeObservation`, `RuntimeProbe`, `RuntimeDiscoverySnapshot`, and related discovery API symbols.
- Initial GREEN:
  - `internal/runtime/discovery.go` SHA256: `ddf2431a2f5ec44249fc587f5811a7207f513c63d6099308c507532ec56ebf07`
  - `internal/runtime/discovery_test.go` SHA256: `1c711afde5d41bcb981e7d8a95975ae0eabcf719e6eb3aa88971437d06506378`
- Controller pre-review:
  - Finding 1: snapshot immutability needed a public accessor API instead of direct mutable fields.
  - Finding 2: probe ID capture needed to happen exactly once per valid probe during prevalidation, then be reused for sorting, source assignment, and error labels.
- Regression test-only substep:
  - `internal/runtime/discovery_test.go` SHA256: `9482ce30d9a19887270a3a087ad1528e73afbc693616969489c55c704bda0895`
  - Focused regression RED exit: `1`
  - Classification: valid RED; failure was because immutable snapshot accessor methods were missing.
- Production repair:
  - `internal/runtime/discovery.go` SHA256: `2543d956b4656982c8ec0661a8cdd68d4e049f203e49b63e6ed2b4818e0dbc24`
  - Final `internal/runtime/discovery_test.go` SHA256: `9482ce30d9a19887270a3a087ad1528e73afbc693616969489c55c704bda0895`

## Verification Record

- Focused GREEN: exit `0`
- Package: exit `0`
- Focused race, count 50: exit `0`
- Impact: exit `0`
- Repository race: exit `0`
- `go vet`: exit `0`
- `gofmt`: `PASS`
- `git diff --check`: `PASS`
- Import-boundary check: `PASS`
- Scope check: `PASS`

## Candidate Summary

The Candidate implements the frozen deterministic Runtime discovery coordination boundary:

- defines the injected `RuntimeProbe` port;
- prevalidates nil, empty-ID, and duplicate-ID probes before observation;
- captures each valid probe ID exactly once and reuses that cached ID;
- invokes probes in deterministic probe-ID order;
- revalidates each reported `RuntimeInstance` through the accepted S2-W1 constructor boundary;
- normalizes capabilities and model IDs into deterministic order;
- rejects invalid model IDs, duplicate model IDs, and duplicate RuntimeInstance IDs with typed errors;
- fails closed on cancellation and probe failure with no usable partial snapshot;
- returns an immutable `RuntimeDiscoverySnapshot` through `Digest()` and deep-copying `Observations()` accessors; and
- computes a lowercase SHA-256 digest over the canonical catalog-relevant fields frozen by the contract.

## Trust Boundary

This Candidate adds only side-effect-free coordination over injected in-memory probe results. It does not add a concrete local Runtime probe, daemon scheduling, filesystem discovery, process execution, network access, persistence, SQLite or Journal writes, Provider SDK integration, credential access, Team Draft composition, Runtime binding, capacity allocation, Run creation, or Runtime execution.

`RuntimeProbe` remains an untrusted observation port. `RuntimeObservation` and `RuntimeDiscoverySnapshot` are Candidate catalog inputs only, not Event Journal facts, execution authority, Provider entitlement, credential authority, or proof of live Runtime availability.

VERDICT: PASS
