# S2-W26 Implementation Repair 1 Contract

- WorkItem: `S2-W26`
- Risk: Strict
- Repair count: `1`
- Status: `REPAIR_CONTRACT_FROZEN`
- Trigger: `implementation-review-1.md`
- Product/test state before repair review: unchanged from reviewed Candidate
- Active S2-W26 contract SHA-256:
  `5593c5de01b91c0937907e863f864e7edb686390274b8492e4fff0e32f2dcdd0`
- Pre-repair product SHA-256:
  `2e8cbdc007e5ab31c5f95e1c70afabf006f02c5682820ab28ab057de3f1411d4`
- Pre-repair test SHA-256:
  `e8e2f297a84c2bffa300b2bb2d64627b06c084b838978e93b4e22653dd512e12`

## Owned files

- `internal/runtime/discoveryscan/scan_test.go`
- `.loom-evidence/phase1-slice2/S2-W26/deliverable.md`

`internal/runtime/discoveryscan/scan.go` is explicitly read-only for this
repair. All other accepted product/test files remain unchanged. Controller
evidence/docs remain outside Developer ownership.

## Frozen repair

Add one focused exact-upper-bound case under
`TestDiscoverConfiguredRuntimesPrevalidatesInputs`:

1. construct exactly `MaxProbeFactories` distinct valid canonical-absent fake
   factories;
2. call `DiscoverConfiguredRuntimes` once;
3. require success and the accepted valid nonzero-digest empty snapshot;
4. require every factory to have exactly one `BuildProbe` call; and
5. preserve caller-order behavior without adding product instrumentation.

No product change is authorized. The test must not weaken or replace the
existing `MaxProbeFactories+1` rejection proof.

## Mandatory Repair RED

Before adding the exact-bound case, run a deterministic source assertion that
fails only because no test currently constructs a factory slice of length
`MaxProbeFactories` and asserts successful discovery with every factory called
once.

The repair RED is evidence of the reviewed test gap, not a request to make
correct product behavior fail. After observing it, add only the frozen test
case and rerun the full S2-W26 strict matrix.

## Unchanged boundary

All original S2-W26 behavior, API, imports, errors, delegation, absence
semantics, explicit exclusions, and strict checks remain unchanged. The repair
adds no factory/probe behavior, persistence, scheduler, process, activation,
authority, external action, or Slice 3 scope.

VERDICT: REPAIR_CONTRACT_FROZEN
