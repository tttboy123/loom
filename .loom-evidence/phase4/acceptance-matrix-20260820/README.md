# 用户视角验收矩阵（2026-08-20 · 安装版 LoomBuild25）

> 全部在安装版 App（/Users/lune/Applications/Loom.app，重建自 HEAD `c1925b0c`）
> 的真实 daemon + DeepSeek/OpenCode 上验证。daemon healthy：
> `ping` protocol v1 / snapshot health `serving_request` / `projection current`。

## 矩阵

| # | 验收项 | 结果 | 证据 |
|---|--------|------|------|
| 1 | 开箱即用：冷启动 → Local service ready / Chat ready / Agent Team ready | ✅ | 首页 OCR：quick start 3 项；daemon ping OK |
| 2 | 聊天（DeepSeek，真实回复） | ✅ | `ACC-DS`；attempt succeeded |
| 3 | 聊天（OpenCode，真实回复） | ✅ | `ACC-OC`；attempt succeeded |
| 4 | 三层选择：Provider/Model/推理强度 + 逐条显示实际值 | ✅ | attempt 携带 `deepseek-v4-flash` / `high`；路由标签显示实际值 |
| 5 | 多会话隔离 | ✅ | T-A 回复 AAA，T-B 回复 BBB，重读 T-A 仍 AAA |
| 6 | Team 构建器（builder_start→edit→validate→confirm） | ✅ | `team_instance_created=true`（Accept Verify / Iter Verify 1 等） |
| 7 | Mission → succeeded 全治理链 | ✅ | 10 事件：Plan→Scheduled→Dispatch→AttemptTerminal→**AcceptanceCommitted×2**→**TeamExecutionTerminal**；~30s 收敛 |
| 8 | Mission 失败隔离（blocked + 原因） | ✅ | 3 个 blocked 任务带 `context_retrieval_denied` / `provider_http` |
| 9 | 幻影 running 收敛（active 26→6） | ✅ | 20 个 → failed/Complete + `runtime_process_failed` |
| 10 | RoundTable 全旅程（9 步） | ✅ | create→seat→round→propose→relay→ack→insert→**conclude=True**→snapshot(2 seats, concluded) |
| 11 | 治理 UI / 重启一致性 | ✅ | 重启前后 Mission 状态一致；聊天、Mission 均正常 |
| 12 | 复制：单条 + 整段会话（Copy conversation 按钮） | ✅ | 头部按钮在位；转录函数输出验证 |
| 13 | 错误文案人话化（Side Task / 聊天 / Mission start） | ✅ | 错误码→可操作文案映射已单测 |

## 关键输出摘录
- Chat DeepSeek: `reply: ACC-DS`, `attempt: succeeded model: deepseek-v4-flash effort: high`
- Chat OpenCode: `reply: ACC-OC`, `attempt: succeeded`
- Mission (Accept Verify, team-e8ccc36a): `succeeded / Mission completed / 2 nodes`,
  治理链含 `TeamNodeAcceptanceCommitted` ×2 + `TeamExecutionTerminal`
- RoundTable: `CONCLUDE concluded: True`, `SNAPSHOT concluded: True seats: 2`
- 幻影收敛: missions `{failed:20, awaiting_recovery:3, blocked:3, succeeded:2}`,
  active(non-Complete) = 6

## 视觉证据
- `01-home.png`：健康首页（"Local service ready / Chat is ready with
  DeepSeek · deepseek.primary / Agent Team ready / New task"）。

## 诚实发现（本轮回归扫尾）
- 多次重装/重启后，App 可能残留陈旧的 "Local service unavailable" 连接状态
  （daemon 实际健康，探针正常）；干净重启 App 后恢复。非代码回归——正常用户
  不会在 App 运行中反复重装。已记录为环境 artifact。
- 回归验证：`go test ./... -p 1` 全绿；`swift build --build-tests` 0 error；
  daemon healthy（serving_request / projection current）；状态稳定
  （20 failed / 3 blocked / 3 awaiting_recovery / 3 succeeded，active=6）；
  聊天 HEALTHY 回复成功。

## 运行态
- runtimes: loom-native / opencode / pi 均 online
- 最近对话 thread: thread-acc-*（DeepSeek / OpenCode / 多会话隔离）
- 本轮新增 Mission：Accept Verify（succeeded，evidence 链完整）
