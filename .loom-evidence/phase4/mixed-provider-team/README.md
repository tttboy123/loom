# Phase 2D 核心验收：混合 Provider Team（2026-08-20 · LoomBuild46）

同一 Team 的不同 Agent 绑定独立 Provider/Harness/模型，Mission 真实执行并成功。

## 背景缺口（本轮关闭）
Team 构建目录此前只为"已验证的 brokered 凭证"（DeepSeek）生成角色选项；
OpenCode 原生运行时模型发现为空，进不了 Team 角色选择 → 混合 Provider Team
无法构建。诊断出三个叠加根因：
1. `buildProductSetupCatalog` 跳过 `model_ids` 为空 的运行时；
2. `contextAdapterPromptLimit` 白名单缺 `context:opencode:v1` →
   `RenderDispatchPayload` 拒绝 opencode 角色的 Context Capsule →
   preflight invalid_request；
3. opencode harness 适配器硬编码 `AuthBrokered`（validateRequest /
   UseCredential / RunHarness 均要求 secret）→ 原生 opencode 节点运行时
   `runtime_process_failed`。

## 修复（Go）
1. **目录**：`product_daemon.go` 为 opencode 运行时合成对话目录模型
   （`ProviderConversationModels("opencode")`）；生成 opencode 原生角色选项
   （provider=opencode, adapter=opencode, auth=native,
   model=opencode/deepseek-v4-flash-free, runtime=runtime.opencode.local）；
   definitions 门加入 opencodeRuntime。
2. **Context Capsule**：`contextAdapterPromptLimit` 加入
   `context:opencode:v1`（64 KiB）。
3. **opencode 适配器原生路径**：`HarnessProcessRequest` 增
   `RequiresCredential`；`validateRequest` 接受 AuthNative；
   `Execute` 对原生直接 RunHarness（不租用凭证）；`RunHarness` 仅在
   `RequiresCredential` 时要求非空 secret。

## 测试（RED-first）
- `TestProductSetupCatalogPublishesOpenCodeNativeAgentProfile`：
  目录发布 5 个 opencode 角色选项（profiles≥2, options≥2, 可冻结）。
- `TestDispatchAcceptsOpenCodeContextAdapter`（internal）：
  `context:opencode:v1` 有上限、未知 adapter 仍拒绝。
- `TestOpenCodeAdapterRunsNativeAuthWithoutCredential`：
  原生 binding 执行成功、不租用凭证、模型身份正确。
- 全量 `go test ./... -p 1` 绿；`gofmt` 干净；`git diff --check` 干净。

## 安装版 live 验证（LoomBuild46）
- `setup` 快照：role_options 10 个（5 deepseek loom-native + 5 opencode native）。
- 构建混合 Team（builder）：main=deepseek-coordinator（loom-native），
  subagent=opencode-bounded-worker（opencode）→ `team_instance_created=true`。
- Preflight 节点：
  - `main | harness=loom-native | provider=deepseek | model=deepseek-chat | ready`
  - `subagent | harness=opencode | provider=opencode | model=opencode/deepseek-v4-flash-free | ready`
- Mission "What is 9*9?" → **Complete / succeeded ~10s**。
- 治理链完整提交：AttemptLoopStarted×2 / ModelRequestAdmitted×2 /
  **TeamNodeAcceptanceCommitted×2（main + opencode 子代理均接受）** /
  WorkItemVerificationCommitted×2 / TeamExecutionTerminal。
- 归档两个混合测试团队 → 环境回到 0 active / 0 need you。

## 回归门
`go test ./... -p 1` 全绿；`swift build --build-tests` 0 error；
`git diff --check` 干净；daemon healthy（serving_request / projection current）。
