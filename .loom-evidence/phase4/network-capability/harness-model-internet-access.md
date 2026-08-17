# Phase 4 · Loom harness 层：模型能否访问/查询互联网（代码权威实测）

Status: `TEST COMPLETE（代码权威）`
Date: 2026-08-17
Scope: 安装版 Loom 的 harness 层（`internal/runtime/harnessadapter`）让**模型/Agent**
去访问查询互联网的能力。

## 结论：不能（当前版本）

Loom 的 harness 层把 Agent 进程内的工具面**严格限制为 loom_context（本地上下文）**，
并且**显式禁用 WebSearch / WebFetch**；`web_search` / `mcp_server` enrollment 目前
只是策略元数据（preflight/冻结），尚未构造可执行的真实搜索/MCP 客户端。

## 证据（源码）

1. **原生 Web 工具被显式禁用**
   `internal/runtime/harnessadapter/claude_code_process.go:19`
   `claudeCodeDisallowedNativeTools = "Bash,Read,Edit,Write,Glob,Grep,WebFetch,WebSearch,Task,Skill,NotebookEdit,TodoWrite"`
   → Claude 的 WebFetch / WebSearch 被列入 disallowed。

2. **harness 只注入 loom_context MCP**
   `codex_app_server.go` / `codex_process.go` / `claude_code_process.go`：
   通过 `-c mcp_servers.loom_context.url=...`（或 `--mcp-config`）注入唯一 MCP，
   `--tools`/`--allowedTools` 仅含 `mcp__loom_context__*`。

3. **loom_context 只服务本地工具（无网络）**
   `context_mcp.go` `toolNames()`：`loom_read_context`、`loom_read_file`、
   `loom_grep_files`（对应 `permissions.ToolRead` / `ToolGrep`）——全部是
   本地上下文/文件读取，无任何 HTTP/搜索/网络工具。

4. **Gateway 拒绝 loom_context 以外的工具调用**
   `attempt_gateway.go` `validOpenAIFunction` / `validAnthropicAttemptTool`：
   仅放行 `mcp__loom_context__*`；其它 namespace 一律 `false`；
   `gatewayProviderResponseUsesFrozenTools` 校验 Provider 返回里只含冻结工具。

5. **enrollment 无运行时客户端**
   `internal/work/remote_tool_backend_enrollment.go`：`web_search`（mcpServerID 空、
   allowedTools 空）与 `mcp_server` 是策略/形状校验 + preflight 门禁；V31 边界明确
   “不构造 Search/MCP client、不发布远程能力”。G5 live 只验证了 enrollment
   生命周期（配置/冻结/撤销阻断 preflight），未验证真实联网查询。

## 与对话层一致
`cmd/loom-net-probe` 实测 Chat 层模型也回答无 web/浏览工具。两层一致：
**Loom 的模型目前拿不到“访问查询互联网”的工具**，只能读本地上下文/文件。

## 要让它能联网查询（harness 层）
即 `4.2-harness-tools/10-chat-governed-web-tools-slice.md` 要做的：
- 通用 MCP 注入层（web_search / 远端 MCP 进 harness，参考 Multica `NormalizeCodexLaunchArgs`
  / Grok `xai-grok-mcp` / DSH `tools.ts` tool bridge）；
- 远程 MCP 客户端 + OAuth 凭据（Vault）；
- 从 disallowed/allowlist 中放开并策略化 web 工具（替代“一刀切禁用 WebFetch/WebSearch”）；
- SSE 退避 / 工具超时 / circuit breaker 护栏。
