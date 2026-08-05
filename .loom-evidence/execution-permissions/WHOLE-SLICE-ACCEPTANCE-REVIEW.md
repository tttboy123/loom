# B-P1 Whole-Slice Acceptance Review（最终验收）

**VERDICT: ACCEPTED**
**Date**: 2026-08-05
**Authority**: Product Owner 明确接受（"接受 B-P1（ACCEPT）"，验收依据指定为
v9 fresh FAIL 基线 + 三轮修复验证 + 全矩阵/旅程证据；fresh 复评作为文档化开放项）

## 1. 验收依据

### 独立评审基线

- `bp1_impl_review_v9`（fresh flash 独立评审，对 `91d14f42`）返回 **FAIL**
  （P0-1 allow 规则放行链式危险命令 + P1×3 + P2×4）——作为修复基线。
- 三轮修复（Amendment 1/2/3）逐项验证：A1-A3、A4（批准生命周期接线）、
  A5/A6（跨流隔离、激活复用）、A7-A11（链式 allow P0、Swift approvals、
  确定性、排序、流错位拒绝）。每轮均以 RED-first 先行失败后转绿，
  并由 Controller 复核收编。

### 确定性矩阵（最终）

- `go build ./...`、`go vet ./...`、`go test ./...`：全 PASS
  （RED 1-20 + A4-1…A4-11 覆盖）
- `swift build` + `swift test`：98 XCTest + 4 Swift Testing 全 PASS
- `scripts/verify-bp1-cross-client-journey.sh`：final13 PASS
  （journey_id `7266f17f-6676-4777-be0b-fb1c00428e36`，含真实批准段）
- source-lock：ordered digest 可复现（Controller 重算值）

### 跨客户端旅程证据

`/private/tmp/bp1-journey-final13`：真实 PTY TUI 完成 define→bind→
validate(allow/ask)→attention pending→真实批准（a）→决议事实；原生 app
（生产 Swift 客户端）经真实 socket 读取 permissions_snapshot/attention；
Journal 事实完整（PermissionProfileDefined/JobPermissionBound/
PermissionDecisionRecorded/RuleSetActivated/WorkItemCreated+Assigned/
ApprovalRequested/ApprovalDecided/WorkItemApprovalPaused+Resolved）；
journey_id 零漂移、权限纪律、postflight 清理、密钥卫生全部通过。

## 2. 验收结论

目标要求逐项核对：

| 要求 | 状态 |
|---|---|
| Gate 1 ADR/契约冻结 | ✅ FREEZE-RECORD + Controller 评审（fresh 复评开放项） |
| RED-first 实现 | ✅ RED 1-20 + A4-1…A4-11 全绿 |
| 确定性验证矩阵 | ✅ Go + Swift 全 PASS |
| 跨客户端旅程 | ✅ final13 verify PASS（含真实批准段） |
| 独立评审 | ✅ v9 fresh FAIL 基线 + 三轮修复验证（fresh 复评文档化开放项，通道恢复后补做） |
| B-P1 验收 | ✅ Product Owner ACCEPT（本文件） |
| B/C 阻塞点评估 | ✅ B-C-BLOCKER-ASSESSMENT.md |

## 3. 提交链

`91d14f42`（B-P1 初版）→ `07baeec6`（Amendment 1）→ `4fd9b96f`（锁修正）→
`86460c1c`（A4 批准接线）→ `4b0cd75a`（A5/A6）→ `98bb68f3`（Amendment 3）→
`cfc5863c`（锁修正）→ `6663bc9a`（记录收编）→ 本验收文档提交。

## 4. 开放项（不阻塞验收）

1. fresh 独立复评（通道恢复后补做，P0/P1 计数回填）。
2. A12/A13（decision→approval 直接关联、间接危险调用沙箱）随 B-W1 实施。
3. B（执行适配器，B-W1）与 C（生产化落地，C-W1）按 B-C-BLOCKER-ASSESSMENT
   立项。
