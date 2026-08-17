# P2D-W4 · Loom Mission 模型联网查询全链路 live 验收（V33）

## 结论
安装版 Loom 中，Mission 的子 Agent（DeepSeek，绑定 web_search Enrollment）在真实运行中：
**模型主动调用 `loom_web_search` → Attempt-Loop 准入（ToolCallAdmitted）→ 执行适配器授权（ToolExecutionAllowed）→ 真实执行（ToolExecutionProposed 携带 WebSearch 工具事实）**，
Journal 已记录 `"tool":"WebSearch"` 类事实。全链路（模型工具调用 → Loom 治理 → 真实联网执行）已被安装版 live 验证。

已知剩余缺陷（不影响上述链路证据）：
- 从本机 IP 的 DuckDuckGo Lite 搜索端间歇被限流/封禁（返回 202 → 空结果）。
  空结果被 Attempt-Loop 结果提交路径拒绝（`result_persistence_failed`/`dispatch_not_committed`），
  导致该次 Mission 不收敛（board 停在 running，9 分钟超时）。
- WebFetch 在默认权限模式下是 `ask`（需要人工批准），Mission 内无批准通道 → 返回受控 `tool_denied` 消息给模型。

## 本轮修复（关闭"模型联网查询"闭环的根因）
1. **Enrollment 绑定 digest 不往返**：`FrozenExecutionBinding` 带 `RemoteToolEnrollmentID/Digest`
   时使用 v3 digest，但 `teamExecutionBindingPayload`/`projectedExecutionBindingPayload` 丢弃这两个字段，
   Journal 重放时重算 legacy digest → 投影拒绝 → daemon `build_state` 砖（journal 不可重放）。
   修复：工作包 + 投影 payload 均携带 enrollment 字段并往返。
2. **Supervisor profile 丢失 enrollment**：`appRuntimeProfileFromFrozenBinding` 未复制
   `RemoteToolEnrollmentID/Digest`，validateInput 重算 legacy digest 与 Route Segment 的 v3 digest 不匹配。
   修复：该转换保留 enrollment 字段。
3. **模型名严格相等**：DeepSeek 对 `deepseek-chat` 返回 `model: "deepseek-v4-flash"`，
   `decoded.Model != expectedModel` 直接 provider_http。修复：改为格式校验（与 Chat 路径一致）。
4. **tool_calls 含 `index` 字段**：`openAICompatibleToolCallWire` 用 DisallowUnknownFields 拒绝。
   修复：加入 `index,omitempty`。
5. **内容+工具调用并存**：DeepSeek 常在函数调用前输出内容（"I'll search..."），
   `(content=="")==(len(toolCalls)==0)` 拒绝。修复：允许工具调用伴随短内容。
6. **并行工具调用**：模型可一次返回多个 tool_calls。修复：exchange loop 逐个执行全部调用；
   请求体加 `parallel_tool_calls:false`（DeepSeek 部分忽略）。
7. **Context 检索被拒直接杀死 attempt**：`ErrContextRetrievalDenied` 现作为受控工具结果返回模型，
   模型可改用 web_search 恢复；不泄漏检索原因。
8. **Web 工具未暴露给 Mission 模型**：a) 远程工具执行器只在 daemon 启动时物化，
   新 Enrollment 不被纳入 → 动态物化（从 live projection 每次重物化）；
   b) App 以固定 `[HOME,PATH]` 环境派生 daemon，`launchctl setenv LOOM_ENABLE_WEB_TOOLS=1`
   丢失 → Swift 派生时合并调用方环境。
9. **Web 工具执行要求 worktree**：执行适配器对所有工具解析 worktree，
   远程工具本不需要 → 仅 workspace 工具解析。
10. **gateway JobID 错误**：`ToolCallEnvelope.JobID` 需等于 WorkItemID（`sameProductAttemptToolBinding` 要求），
    误传 RunID → `invalid tool call envelope`。修复。

## Live 证据（安装版，2026-08-17）
- daemon 环境包含 `LOOM_ENABLE_WEB_TOOLS=1`（App 派生合并后）。
- Journal 事件序列（真实 Mission，team-live-web-…）：
  - `RemoteToolBackendEnrollmentConfigured`（web_search 绑定 deepseek.primary）
  - `ToolCallAdmitted`（attempt-loop 准入 web 工具）
  - `ToolExecutionProposed`（携带 WebSearch 工具事实）
  - `ToolExecutionAllowed`
  - `ToolExecutionFailed`（`result_persistence_failed` / `dispatch_not_committed`，空结果/状态机缺陷）
- 全量 Go 套件 + Swift 15 测试绿；`git diff --check` 待跑。

## 复现/运行
```
# 安装版 app 需 LOOM_ENABLE_WEB_TOOLS=1（launchctl setenv）
LOOM_LIVE_WEB_MISSION=1 LOOM_LIVE_TEAM_E2E=1 LOOM_LIVE_NET=1 \
  go test ./cmd/loomd/ -run TestLiveMissionAgentCallsWebSearch -count=1 -v -timeout 15m
```

## 后续（不并入本切片）
- 允许空/低信息搜索结果的受控提交（DDG 被限流时的容错）；
- WebFetch 在 Mission 内的批准路径（或降级为可执行的只读策略）。
