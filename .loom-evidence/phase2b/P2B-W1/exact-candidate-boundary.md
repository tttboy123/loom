# P2B-W1 Exact Candidate Boundary

Baseline: `7a27149b31db7ffeefefb86c449ba448c3250fca`

Candidate paths are limited to the frozen contract-owned files that differ
from the baseline, plus the P2B-W1 evidence directory:

```text
PRODUCT-PLAN.md
TECH-PLAN.md
docs/CURRENT.md
.loom-evidence/phase2a/PHASE2A-PRODUCT-OWNER-SIGNOFF.md
.loom-evidence/phase2a/WHOLE-PHASE-EXIT-MATRIX.md
.loom-evidence/phase2a/WHOLE-PHASE-REVIEW.md
.loom-evidence/phase2a/WHOLE-PHASE-SOURCE-EVIDENCE-LOCK.json
.loom-evidence/phase2a/WHOLE-PHASE-VERIFICATION.md
.loom-evidence/phase2a/whole-phase-swift-timeline-fixture-race-repair.md
.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment.md
.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-1.md
.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-2.md
.loom-evidence/plan-amendments/2026-08-03-phase2b-side-task-handoff-roadmap-amendment-review-3.md
.loom-evidence/phase2b/P2B-W1/**
apps/macos/Sources/LoomLocalAppContractProbe/main.swift
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductHandoffModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductHandoffModelsTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
internal/api/local_product_handoff.go
internal/api/local_product_handoff_test.go
internal/api/local_product_mission_test.go
internal/api/local_product_read.go
internal/api/local_product_read_test.go
internal/app/local_product_execution.go
internal/app/local_product_handoff.go
internal/app/local_product_handoff_test.go
internal/evidence/side_task_handoff.go
internal/evidence/side_task_handoff_test.go
internal/evidence/side_task_handoff_windows.go
internal/localipc/protocol.go
internal/localipc/protocol_test.go
internal/localipc/swift_contract_test.go
internal/projection/global_read_view.go
internal/projection/global_read_view_test.go
internal/projection/projection.go
internal/projection/side_task_handoff.go
internal/projection/side_task_handoff_test.go
internal/tui/model.go
internal/tui/model_test.go
internal/work/side_task_handoff.go
internal/work/side_task_handoff_test.go
```

Explicit exclusions include all other dirty or generated paths. In particular:

```text
internal/projection/team_execution_test.go
.loom-evidence/phase1-final-live-gate/**
AGENTS.md
PROGRESS.md
README.md
.codex/**
.loom-drafts/**
apps/macos/.build/**
```

The excluded `internal/projection/team_execution_test.go` modification predates
P2B-W1 and remains user-owned. It must not appear in the P2B-W1 source lock,
staging set or atomic commit.
