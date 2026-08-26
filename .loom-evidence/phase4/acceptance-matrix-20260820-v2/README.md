# 用户视角验收矩阵 v2（2026-08-20 · 安装版 LoomBuild47）

> 全部在安装版 App（/Users/lune/Applications/Loom.app，HEAD 序列
> 54aa33bb…25c0acc8）的真实 daemon 上验证；daemon healthy
> （serving_request / projection current）。v1（LoomBuild25）13 项全部仍成立，
> 本矩阵为合并后的权威验收记录。

## 矩阵

| # | 验收项 | 结果 | 证据 |
|---|--------|------|------|
| 1 | 开箱即用：冷启动 → Local service ready / Chat ready / Agent Team ready | ✅ | quickstart-checklist-wording / e2e-user-journey |
| 2 | 聊天（DeepSeek，真实回复） | ✅ | "7 * 8 = 56"，路由标签 + disclosure |
| 3 | 聊天（OpenCode，真实回复） | ✅ | v1 矩阵 ACC-OC |
| 4 | 三层选择：Provider/Model/推理强度 + 实际值展示 | ✅ | attempt 携带 deepseek-v4-flash/high |
| 5 | 多会话隔离 | ✅ | A=56 / B=blue 无串扰（e2e-user-journey） |
| 6 | 跨消息拖选复制（Selectable text）+ Copy conversation | ✅ | 剪贴板两行（conversation-multiselect-copy） |
| 7 | Team 构建器（含 OpenCode 角色选择，正确命名） | ✅ | opencode-role-label / mixed-provider-team |
| 8 | **混合 Provider Team：main=DeepSeek(loom-native) + subagent=OpenCode(opencode)** | ✅ | preflight 双绑定 ready；Mission succeeded ~10s；治理链 TeamNodeAcceptanceCommitted×2 + TeamExecutionTerminal（mixed-provider-team） |
| 9 | 混合 Mission 在 App 工作台可视化（每 Agent 的 Provider/Harness/Model） | ✅ | mixed-team-ui-display |
| 10 | Mission → succeeded 全治理链 | ✅ | 10 事件链（v1 + mission-e2e-current-build） |
| 11 | Mission 失败隔离（blocked + 原因） | ✅ | v1：3 blocked 带 context_retrieval_denied/provider_http |
| 12 | Board/工作台/TUI 状态文案统一（无 "Complete · Failed"） | ✅ | mission-board-detail-label / mission-workbench-detail-label / tui-board-detail-label |
| 13 | Attention 可操作/历史分离 + 计数统一（App + TUI） | ✅ | attention-panel-actionability / tui-attention-actionability；首页 need you=0 |
| 14 | RoundTable 全旅程（9 步） | ✅ | v1：conclude=True，2 seats |
| 15 | 治理 UI / 重启一致性 / 归档处置 | ✅ | governance-disposal / archived-team-missions-history |
| 16 | 错误文案人话化（Side Task / 聊天 / Mission start） | ✅ | error-copy-unification + 单测 |
| 17 | 全量测试基线 | ✅ | Go `./... -p 1` 全绿；Swift XCTest 268 用例 0 失败（1 视觉导出跳过） |

## 关键输出摘录（本轮新增）
- Mixed Team preflight：
  - `main | harness=loom-native | provider=deepseek | model=deepseek-chat | ready`
  - `subagent | harness=opencode | provider=opencode | model=opencode/deepseek-v4-flash-free | ready`
- Mixed Mission：`Complete / succeeded ~10s`；journal 含
  `TeamNodeAcceptanceCommitted`×2（main + opencode 子代理）、
  `WorkItemVerificationCommitted`×2、`TeamExecutionTerminal`。
- App Mission 工作台："2 node(s), 2 complete, 0 in review"；两行分别显示
  DeepSeek·deepseek.primary·deepseek-chat 与
  OpenCode·OpenCode·opencode/deepseek-v4-flash-free。

## 运行态
- runtimes: loom-native / opencode / pi 均 online。
- 环境干净：首页 0 active / 0 need you（本轮混合测试团队已归档）。
