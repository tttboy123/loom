# B-P1 Execution Permission Pipeline — Progress

Updated: 2026-08-05

## Status

- Gate 0：PASS（身份/排除清单/前置能力，见 GATE0-AUDIT.md）。
- Gate 1：FROZEN（FREEZE-RECORD.md；外部 fresh Contract Review 为开放项）。
- RED-first：`internal/permissions` 17 项 RED 中 16 项在本包绿
  （RED 14 在 `internal/app` 服务层绿）；25 个单元测试全 PASS。
- 产品服务层：`internal/app/local_permission.go` + `internal/api/local_permission.go`
  （只读 snapshot/attention + 命令路由 + validate_call）测试 PASS。
- daemon 挂载：`cmd/loomd/product_daemon.go` 三方法分发 +
  `internal/localipc/protocol.go` 白名单；cmd/loomd 全测试 PASS。
- 全仓 `go build ./...`、`go vet ./...`、`go test ./...` 全绿。

## 已完成清单（对照契约 owned files）

- [x] `internal/permissions/model.go`（类型/常量/校验/digest）
- [x] `internal/permissions/compile.go`（CompileProfile + DefaultProjectProfileTemplate）
- [x] `internal/permissions/evaluate.go`（Evaluate/白名单/危险表/链式/glob）
- [x] `internal/permissions/replay.go`（Replay 两阶段/ResolveEffectiveProfile/EffectiveMode/ActiveGrants）
- [x] `internal/permissions/authority.go`（全部权威方法 + CAS + 幂等短路）
- [x] `internal/permissions/{authority,evaluate,replay}_test.go`（RED 1-17 覆盖）
- [x] `internal/app/local_permission.go` + test（RED 14 + validate_call）
- [x] `internal/api/local_permission.go`（IPC API 包装）
- [x] `cmd/loomd/product_daemon.go`（挂载 + 分发）
- [x] `internal/localipc/protocol.go`（方法白名单 + journey 要求）
- [x] `internal/tui/permissions.go`（ScreenPermissions Explorer + Attention 合并
  a/x/g/v 键）+ `internal/tui/model.go` 路由 + `permissions_test.go`
- [x] Swift：`LocalPermissionModels.swift`（PermissionWire 严格解码）、
  `LocalPermissionViews.swift`（只读 Explorer/Attention 视图）、
  `LocalIPCClient.swift` 只读方法（permissions_snapshot/attention，白名单）+ 测试
- [x] `scripts/verify-bp1-cross-client-journey.sh`（B-P1 冻结验证门，含
  permissions IPC/Journal 事实/Transcript 断言）
- [x] **跨客户端旅程证据**：`/private/tmp/bp1-journey-final8`（journey_id
  `1565bdca-59af-4f18-8a70-044d68d8260f`）真实 PTY TUI 全流程 + 原生 app
  （生产 Swift 客户端读取 explorer/attention）+ 截图 + Journal 事实，经
  `verify-bp1-cross-client-journey.sh` **PASS**

## 待办（下一步）

- [ ] 独立评审（Implementation/dual-Result/Whole-Candidate，子代理可用后补做；
  期间以 Controller 自查 + 确定性矩阵 + 旅程验证为准）
- [ ] 原子提交（exact staging per source-lock）
- [ ] B（执行适配器）/ C（生产化）阻塞点评估

## 确定性矩阵（当前全绿）

- `go build ./...`、`go vet ./...`、`go test ./...` 全 PASS
- `internal/permissions` 25 测试（RED 1-17 覆盖）
- `internal/app`、`internal/api`、`internal/tui`、`internal/localipc`、
  `cmd/loomd` 全 PASS
- `swift build` + `swift test`（98 tests，含权限模型解码）全 PASS
- `scripts/verify-bp1-cross-client-journey.sh` 语法通过（待真实证据根）
- B-P1 跨客户端旅程 verify **PASS**（final8）

## 旅程发现并修复的实现缺陷

1. `permissions.Replay` 曾对共享 Journal 中的非权限域事件（Queue/Work/
   Runtime/AgentGrant）报错 → 改为只处理权限流事件，权限流内未知类型仍
   fail-closed（RED #10/#15 语义保持）。
2. TUI Attention 屏合并权限决策后 `emptyOrLines` 计数未含决策 → 修正计数，
   决策行真实渲染（transcript 含 `• Permission …`）。

## 实现中记录的偏差/开放项

1. 决策事实 `Tool` 字段由 validate_call 路径填充，`RecordDecision` 签名（冻结
   符号）不含 Tool，当前为空并跳过校验（契约 §5.1 注记）。
2. `EffectiveMode` 的 personal/project 激活解析依赖 job→project scope 映射，
   由 daemon 服务层在 B-P1 集成/后续 Amendment 提供；当前支持 profile mode +
   root activation。
3. `resolve_approval` 为 typed forward（`ErrApprovalForwardOnly`）；既有
   `internal/rules` 批准权威的实线接线在 B-P1 集成测试阶段完成（RED 3 已在
   permission 层证明"不重复写批准事件"）。
