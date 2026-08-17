# Phase 4 · Harness 网络与工具能力调研（Multica / Grok-build / DeepSeek-Harness）

Status: `RESEARCH COMPLETE` — 基于三个仓库的源码浅读（shallow clone, 2026-08-17）
Date: 2026-08-17
Scratch: `/tmp/loom-harness-research/{multica,grok-build,deepseek-harness}`

## 结论先行：harness 层确实是“通用”的

三个项目给“agent CLI 联网 + 用工具”的实现是同一套模式：

1. **按 CLI 的进程适配器**（spawn 子进程、流式 stdout、生命周期/取消/超时）；
2. **工具面 = MCP 注入**（把 MCP server 配置写进该 CLI 的配置命名空间，agent 就能调工具）；
3. **网络面 = 远程 MCP**（streamable HTTP/SSE + OAuth 凭据，daemon 侧 broker/客户端）；
4. **凭据 = 环境变量/密钥注入**（每 Provider 的 well-known env，从密钥/凭据存储取）；
5. **沙箱/策略**（文件根 allow-list、escalation、session mode、超时/重复工具护栏）；
6. **失败隔离**（circuit breaker / 重连退避 / retry）。

## 各仓库机制（文件级）

### Multica（Go, `multica-ai/multica`）
- 每 CLI 适配器：`server/pkg/agent/{codex,claude,opencode,grok,dsh,kimi,pi,...}.go`；
  `codex.go` 直接 spawn `codex app-server --listen stdio://` 并通过
  `NormalizeCodexLaunchArgs` 注入 `-c mcp_servers.*`（**MCP 注入**）。
- MCP 语义：`server/pkg/agent/mcp_config.go`（managed 三态：继承 / 显式空 / 管理集）。
- 执行环境：`server/internal/daemon/execenv/`（`codex_home.go`、`codex_sandbox.go`、
  `isolation.go`、`runtime_config.go`、`codex_multi_agent.go`）——HOME/沙箱/运行时配置。
- 远程 MCP：`server/internal/daemon/runtime_mcp.go`、`remote_mcp_broker.go`、
  `server/pkg/remotemcp/client.go` + `oauth.go`（**网络 + OAuth**）。
- daemon↔backend：`internal/daemonws`、`wsrpc.go`（WS 实时）。

### Grok-build（Rust, `xai-org/grok-build`）
- 工具契约：`crates/common/xai-tool-protocol`（JSON-RPC 2.0 envelope、工具注册、
  capabilities、error_wire）+ `crates/common/xai-tool-runtime`（`Tool` trait、
  `ToolDispatch`、`ToolCallContext`（Cwd/WorkspaceBindMetadata）、streaming）。
- 远程 MCP：`crates/codegen/xai-grok-mcp/`——`mcp_http_client.rs`（streamable HTTP/SSE，
  **SSE 洪泛防护 + 指数退避重连**：STABLE_STREAM_THRESHOLD=2s，BASE=500ms，MAX=30s）、
  `servers.rs`、`oauth.rs`+`credentials.rs`（远程 MCP OAuth 凭据存储）、`acp_transport.rs`。
- 失败隔离：`crates/common/xai-circuit-breaker`；shell 会话 MCP：`xai-grok-shell-session-support/managed_mcp.rs`。

### DeepSeek-Harness（TS+Py+native, `deepseek-ai/deepseek-harness`，“Everything is a Plugin”）
- MCP 客户端：`packages/mcp/mcp-client/`——`transport.ts`（stdio/SSE/streamable HTTP）、
  `connection.ts`（重连）、`tools.ts`（**工具桥：把 MCP 工具注册到 ToolRuntime，
  公开名 `mcp__<server>__<tool>`，SHA-256 归一化到 64 字符函数名**）、`load-path`、`apply`。
- 沙箱：`packages/sandbox/`——`sandbox/src/roots.ts`（canonicalPath 解析、
  workspace-write = workspace + temp 的 allow-list）、`escalation.ts`、`session-mode.ts`、
  `sandbox-policy/`、`sandbox-windows-acl/`、`native/landlock-run`（Linux Landlock）。
- 护栏：`packages/guard/{timeout-policy, repeat-tool-reminder}`；执行：`subprocess`、
  `shell`、`terminal`、`code-runtime`、`e2b`（云沙箱）；凭据：`packages/credentials`。

## Loom 现状对照（已具备 vs 缺口）

已具备（`loom-pi-rebuild`）：
- 进程适配器：`internal/runtime/harnessadapter/`（codex/claude/opencode/system runner）、
  `internal/runtime/nativeadapter/`（DeepSeek/Kimi/…）；`codex_process.go` 等。
- 凭据 env 注入：`internal/provider/opencode_adapt.go OpenCodeCredentialEnv` +
  Vault lease；`harnessadapter/opencode_process.go openCodeEnvironment`。
- Web Search / MCP Enrollment：`internal/app/remote_tool_backend_enrollment.go`、
  `internal/work/remote_tool_backend_enrollment.go`（V34，绑定 Mission Agent）。
- 失败隔离/治理：attempt governance、incident diagnostics、CAS journal。

缺口（本次要补的“抄”点）：
- **Chat/conversation responder 无工具/网络**：`productOpenCodeConversationResponder` /
  `productCodexConversationResponder` 只发文本 prompt，没有 MCP 工具面、没有网络查询。
- 缺一个**通用 MCP 工具注入层**（像 Multica `NormalizeCodexLaunchArgs` + DSH tool
  bridge）：把 enrollment 的 `web_search` / `mcp_server` 统一写进任意 runtime 的配置，
  让 Codex/OpenCode/DeepSeek/Claude 都能用同一套工具。
- 缺**远程 MCP 客户端 + OAuth 凭据存储**（像 grok `mcp_http_client` + `credentials.rs`
  或 multica `pkg/remotemcp`）：连接远端 MCP server 做工具调用。
- 缺 **SSE 洪泛/重连退避**与 **工具超时/重复护栏**（grok circuit-breaker / dsh guard）。

## 建议采纳顺序（都是受治理的，不引入第二权威）
1. **候选切片（见 10-chat-governed-web-tools-slice.md）**：把 enrollment 的
   `web_search` 接进 conversation responder，MCP 配置注入 + policy/evidence 生命周期；
   远程 MCP 客户端 + OAuth 凭据存储（复用 Vault）。
2. 通用 harness 工具注入层：任意 runtime 统一挂 enrollment 工具（对齐
   Multica/DGH 的“一个 harness 服务所有 CLI”模型）。
3. 网络护栏：SSE 退避、工具超时/重复护栏、circuit breaker（对齐 grok/dsh）。
