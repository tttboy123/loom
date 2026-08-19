# Mission 生命周期验证 + context-read 卡点调查（2026-08-19）

## 目标
闭环上一轮遗留项：一个可执行 Team 的 Mission 从 preflight 走到 running/succeeded
的用户视角验证；并调查卡死 Mission 对 daemon 的影响。

## 已验证（真实 daemon + 真实 DeepSeek + Journal 事件）
1. **IPC 创建真实 Team**：builder_start→edit(team_name/purpose)→validate→confirm
   创建 "UI Verify Team 2"（executable，DeepSeek main+subagent）。
2. **真实 Mission 启动**：mission_execution preflight（2 nodes ready，DeepSeek，
   capacity 2）→ start → status=running；subagent attempt 1 实际调用 DeepSeek：
   AttemptLoopStarted→TurnStarted→StepStarted→ModelRequestAdmitted→
   ToolCallAdmitted→ToolDispatchCommitted（context-read 工具）。
3. **daemon 健康**：含 2 个卡死 Mission 的 DB 上 daemon 正常启动并服务
   （socket 1s 出现，ping ok，App "Local service ready"）。
   - 关键：此前 App 反复 "Local service unavailable" 的主因是**陈旧 socket 文件**
     （App 见 socket 存在即不再拉起 daemon）与启动竞态，非产品缺陷；
     清理 run/ 后 App 正常。
4. **团队治理**：team_archive 可归档卡死团队（team-ui-mission-1→archived）。

## 发现（待修，真实缺陷候选）
**IPC/无真实会话胶囊启动的 Mission，在首次 context-read 工具派发后停滞**：
- attempt-loop 事件停在 ToolDispatchCommitted（context-read，call_id
  context-read-*），>30 分钟无新事件；mission 一直 running/working。
- 两个卡死 Mission（UI Mission Team、UI Verify Team 2）均为 IPC 启动、
  其 context capsule 不在 credential-vault（vault 中无
  mission:mission/team-instance-36bd 胶囊；而之前成功的 Mission 3515ca/7ebe2e
  有胶囊）。
- 链路：delivery.Prepare → retriever.Retrieve → VaultStore.RetrieveContextItem
  → ReadContextCapsule → 会话 key 不存在返回非 ErrContextRetrievalDenied 错误
  → deepseek adapter 的 bounded-error 分支不命中 → 期望终态失败但实际停滞。
- 影响：Mission 无法收敛 succeeded；需要聚焦修复
  （缺失胶囊的 context-read 应按 bounded tool error 处理并继续，
  或 attempt 应有超时/恢复路径）。

## 测试
- Swift 254 绿（1 跳过）；Go app/work/supervisor/nativeadapter/toolbroker 全绿
  （上一轮提交 473995d4/0e4be93d 后无新代码改动）。
