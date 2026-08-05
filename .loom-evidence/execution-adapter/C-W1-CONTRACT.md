# Gate 1 Exit Contract — C-W1 Production Landing

**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner "达成生产可用"指令 +
Controller 冷读自查；flash 独立评审为开放项，见 REVIEW-NOTES）
**Date**: 2026-08-05
**Baseline**: `f5b7009d`（B-P1 ACCEPTED）on `codex/loom-platform-slice2`
**Risk**: STRICT — 真实机器状态变更（launchd、`~/Library/Application
Support/Loom`）、激活流程、管理员锁持久化、回滚

## 1. 唯一垂直 WorkItem

```text
C-W1 Production Landing
```

显式激活流程（permission diff 预览 + 用户确认 + Journal 激活事实）、
launchd/Application Support 安装与回滚（复用 P2A-W1 install 模式）、常驻
daemon 的 fail-closed 恢复与降级。不包含：B-W1 执行适配器（并行 WorkItem）；
网络/付费/多用户；自动激活。

## 2. 产品结果

用户在 TUI/原生 app 看到"激活 Loom 常驻服务"：激活前展示 permission diff
（将写入哪些路径、注册哪些 launchd 项、改变哪些模式），确认后写入
Application Support 配置 + launchd plist + Journal 激活事实；可随时停用/
回滚（恢复原配置）；daemon 重启/崩溃后按 Journal 恢复且策略缺失时
fail-closed 到只读。管理员锁开启时 bypass 激活被拒。

## 2.5 客户端表面

- IPC 新增只读 `production_snapshot` 与写 `production_command`
  （actions: `activation_preview` / `activation_confirm` /
  `deactivation_preview` / `deactivation_confirm`）。
- TUI 新增 `ScreenProduction`（"Production"）：展示当前常驻状态与最近
  激活事实；`p` 预览（零写入 diff）、`c` 确认（diff digest + authorized_by）、
  `d` 停用预览/确认、`r` 刷新；管理员锁开启时 bypass 项展示拒绝原因。
- 生产 Swift 客户端只读新增 `productionSnapshot`（严格解码），不新增写
  方法；Journey probe 只读消费激活状态。

## 3. 精确语义

### 3.1 激活（显式 + 可审计）

- `activation preview`：读当前机器状态 + 目标状态，输出精确 diff
  （文件路径、权限、launchd 项、模式变更）——零写入。
- `activation confirm`：仅接受与 preview digest 一致的确认；写
  `ProductionActivationActivated`（scope、diff_digest、authorized_by、
  activated_at）事实，然后才落盘配置/launchd；失败回滚到 preview 状态。
- `deactivation preview`：零写入展示将删除的 launchd 项、将恢复的配置与
  deactivation diff digest。
- `deactivation confirm`：仅接受与 deactivation preview digest 一致的确认，
  且 authorized_by 非空；写 `ProductionActivationDeactivated` 事实后再恢复
  原配置（备份 + 校验 + 回滚，无残留）。

### 3.2 持久化与恢复

- `~/Library/Application Support/Loom/` 配置 0700、文件 0600、原子替换
  （tmp + rename），含 install/rollback 校验（digest 比对）。
- launchd plist 仅由激活流程写入/删除；写入前展示、删除时恢复。
- daemon 常驻：启动时按 Journal 重放激活事实恢复状态；策略缺失/损坏 →
  只读降级并告警，绝不带错误策略运行。
- 管理员锁：激活 bypass_permissions 被锁拒（B-P1 已实现，C 验证持久化）。
- 降级语义：daemon 启动时重放 production 流；若最后事实为 Activated 但
  磁盘 digest 不匹配/缺失，服务进入 `degraded_readonly`：production/
  permissions/execution/queue/workers/integration 写命令一律返回
  `degraded` 错误，只读方法照常；`production_snapshot` 暴露
  `degraded=true` 与原因。
- 管理员锁生效时 `activation_confirm` 的目标有效模式若为
  `bypass_permissions` → 拒绝；`activation_preview` 展示该拒绝原因。

## 4. Journal 事实

- `ProductionActivationActivated/Deactivated`（流 `production-activation`）
- `ProductionConfigWritten/RolledBack`（流 `production-config/<key>`，
  含 path、digest、pre/post 状态）

## 5. RED-first 矩阵

1. preview 零写入；diff 与实盘状态一致。
2. confirm 缺 preview digest/authorized_by → 拒绝；确认后先写 Journal 事实
   再落盘；任一步失败回滚且无残留。
3. deactivate 恢复原配置（备份校验），launchd 项删除；deactivation confirm
   缺 preview digest/authorized_by → 拒绝，零写入。
4. 重启后激活状态与 Journal 一致；策略缺失 → 只读降级 + 告警。
5. 管理员锁开启时 bypass 激活拒绝；锁关闭需 authorized_by。
6. 安装原子性（tmp+rename）；回滚 digest 校验失败 → 保留备份并 human_required。
7. 重复激活/停用幂等（无双份事实、无重复副作用）。
8. 0700/0600、journey_id、无 secret；跨客户端旅程（TUI 激活 + 原生 app
   读到激活状态）。

## 5.5 Exact owned files（冻结范围）

### 新增

```text
internal/production/model.go          // ActivationPreview/Diff/Digest/Journal 载荷/RecoveryStatus
internal/production/service.go        // preview/confirm/deactivate + Journal 事实 + 恢复判定 + admin-lock 门禁
internal/production/config.go         // Application Support 原子写入（tmp+rename、0700/0600、备份/回滚、digest）
internal/production/plist.go          // 用户级 launchd plist 生成/安装/删除（注入 root）
internal/production/service_test.go   // RED 1-8
internal/production/config_test.go
internal/production/plist_test.go
internal/app/local_production.go      // 产品服务：production_snapshot/production_command + 降级门禁
internal/app/local_production_test.go
internal/api/local_production.go      // API 别名（沿 local_permission.go 模式）
internal/tui/production.go            // ScreenProduction 激活/停用流程
internal/tui/production_test.go
apps/macos/Sources/LoomLocalAppCore/LocalProductionModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductionViews.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductionModelsTests.swift
scripts/verify-cw1-cross-client-journey.sh
cmd/loomd/cw1_production_wire_test.go
.loom-evidence/execution-adapter/C-W1-CONTRACT.md
.loom-evidence/execution-adapter/C-W1-PROGRESS.md（证据根）
```

### 修改（最小面）

```text
cmd/loomd/product_daemon.go           // 挂载 production 服务 + handler 路由 + 降级门禁
internal/localipc/protocol.go         // production_snapshot/production_command 方法白名单
internal/tui/model.go                 // ScreenProduction 路由
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift  // productionSnapshot 只读
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift  // refreshProduction 只读
apps/macos/Sources/LoomLocalAppUI/ContentView.swift        // Production 只读视图挂载
apps/macos/Sources/LoomLocalAppContractProbe/main.swift    // 只读 production probe 动作
internal/localipc/swift_contract_test.go // swiftc 探测文件清单补新模型文件
```

修改面之外的任何产品路径一律不动；真实 `~/Library/Application Support/Loom`
与用户级 launchd 仅在最终激活（用户显式确认 diff）时写入；测试与旅程一律
使用注入的沙箱 root。owned files 变更必须走独立评审的 bounded amendment。

## 6. 验证矩阵

Go full/race/vet/tidy/gofmt；真实 launchd 项只在旅程沙箱根内创建/删除
（不污染系统 launchd——测试用用户级临时 plist 目录）；跨客户端旅程 +
flash 独立评审；原子提交。

## 7. 非目标

系统级 launchd（仅用户级）；网络/付费/多用户；自动激活；绕过 human lane；
覆盖用户既有配置（始终备份+回滚）。

## 8. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty 无法
隔离、真实系统 launchd 污染 ⇒ 停止 HUMAN_REQUIRED。
