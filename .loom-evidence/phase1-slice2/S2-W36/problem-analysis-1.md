# S2-W36 Fresh Repeated-Failure Problem Analysis

- WorkItem: `S2-W36`
- Trigger: second same-class implementation proof failure
- Review 2 SHA-256:
  `e48b8c123edb2944c6ee853f5ebc32f4cabf9f2835b8b95243458f9ac04f2d8d`
- Product SHA-256:
  `a9421c3b58de00a9a9d44d02f9c511cdf603bf9423a78b18237ea783bd269213`
- Pre-Repair-2 test SHA-256:
  `8ce88b02ac23da4699da36ee17ad78908cc1517aab158978b03a45f4649d94e2`
- Analyst: fresh independent read-only problem analyst

## Root cause

This is a contract-to-test traceability and observability gap, not a product
defect. Repair 1 required thirteen direct error cases, including configured
discovery failure, but its mandatory marker guard listed only the twelve newly
added cases. Those twelve entered the table-driven matrix with retained
recorder pointers and exact call-count assertions.

The older configured-discovery failure stayed standalone with anonymous
inline committers. It proves the exact error and five zero outputs, but its
committer calls are unobservable. Therefore all checks can pass while the
required `0/0` writer proof is absent.

## Product assessment

The product performs one direct S2-W35 return-call and has no retry or fallback
path. Configured-discovery failure occurs downstream before writer selection.
No product, API, authority, concurrency, or typed-nil defect was found.

## Recommended bounded repair

Repair 2 should be test-only. Convert the configured-discovery proof to one
named direct case using an existing named `appDiscoveryFactory` that returns a
sentinel error plus named discovery/status recording committers. One
`observer.RunOnce` must prove the sentinel, five zero outputs, and the exact
call tuple `factory/discovery/status = 1/0/0`.

Mandatory RED must guard not merely case presence but all four behavior
markers: the named case and the three exact call-count assertions. Product
must remain byte-for-byte unchanged; the full parent matrix and a fresh
implementation review remain mandatory.

VERDICT: ANALYSIS_COMPLETE
