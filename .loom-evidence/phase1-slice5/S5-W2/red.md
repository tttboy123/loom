# S5-W2 Mandatory RED

Command:

```sh
go test ./internal/work ./internal/app \
  -run 'WorkPackage|Phase1EngineeringDemo|Phase1LiveGate'
```

Result: expected failure (`exit 1`).

```text
--- FAIL: TestWorkPackageMandatoryREDRejectsDuplicateAndMutation (0.00s)
    work_package_test.go:36: duplicate AgentDefinition IDs were accepted
FAIL
FAIL	loom-pi-rebuild/internal/work	0.801s
--- FAIL: TestPhase1LiveGateManifestIsBoundedAndNonExecuting (0.00s)
    phase1_engineering_demo_test.go:21: live-gate manifest missing: open ../../.loom-evidence/phase1-slice5/S5-W2/live-demo-manifest.json: no such file or directory
FAIL
FAIL	loom-pi-rebuild/internal/app	1.304s
FAIL
```

The failures are behavioral:

- the temporary WorkPackage seam accepts duplicate AgentDefinition IDs and
  retains caller-controlled slices;
- the bounded, non-executing live-gate manifest does not exist.

No product implementation existed when this evidence was captured.
