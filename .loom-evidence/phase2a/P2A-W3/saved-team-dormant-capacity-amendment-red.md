# P2A-W3 Saved-Team Dormant Capacity Amendment RED

**Date**: 2026-08-02  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Contract Review**: `PASS`  
**Production change at RED**: none

## Command

```text
go test ./internal/teams \
  -run '^(TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding)' \
  -count=1
```

## Exact failure

```text
--- FAIL: TestBuildSavedTeamRuntimeBindingCardinalityDormantCapacityResolutionAndReorder
    --- FAIL: .../2_roles
        BuildSavedTeamRuntimeBinding() error = saved team runtime capacity exceeded
    --- FAIL: .../3_roles
        BuildSavedTeamRuntimeBinding() error = saved team runtime capacity exceeded
--- FAIL: TestValidateSavedTeamRuntimeBindingFailures
    --- FAIL: .../capacity_source
        error = saved team runtime capacity exceeded,
        want saved team runtime binding source mismatch
FAIL
```

The one-role Main case passed on capacity `1`. Main plus one and Main plus two
dormant SubAgents failed only because the unchanged authority counted every
selected role. The validation case also proved the old count rejected capacity
source drift before current-source Candidate comparison.

**VERDICT**: `RED / CAUSAL / IMPLEMENTATION UNLOCKED / LIVE LOCKED`
