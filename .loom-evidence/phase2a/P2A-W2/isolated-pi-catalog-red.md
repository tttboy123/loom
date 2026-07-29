# P2A-W2 Isolated Pi Catalog RED Evidence

Status: RED — expected compile failure before implementation

Command:

```text
go test ./internal/runtime/piadapter ./cmd/loomd -run 'TestPiLocalModelCatalog|TestPiMetadataProcessRunnerMaterializesBoundLocalCatalog|TestRunAcceptsLocalModelCatalogOnlyAsCompleteTuple' -count=1
```

Observed:

- `PiLocalModelCatalogConfig` and `bindPiLocalModelCatalog` were undefined.
- `PiMetadataProcessRunnerConfig.LocalModelCatalog` and the digest-injected
  constructor seam were undefined.
- `LocalRuntimeObservationDaemonConfig.LocalModelCatalog` was undefined.

The failures prove that no pre-existing implementation could satisfy the frozen
catalog materialization, Runner visibility, or CLI all-or-none acceptance
boundary.

## Pre-review Repair 1 RED

Controller audit found that the first Candidate retained the bound catalog only
inside each Runner, while the probe factory retained paths and rebound them at
`BuildProbe`. A same-digest replacement after factory construction was therefore
accepted as a new identity.

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiLocalRuntimeProbeFactoryRejectsSameDigestCatalogIdentityReplacement$' \
  -count=1
```

The regression failed because `BuildProbe` returned a non-nil probe with
`found=true`. Repair 1 must retain the factory-construction binding and hand that
exact binding to every Runner.
