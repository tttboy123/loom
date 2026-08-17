# Phase 4 · 候选切片：Chat / Harness 治理化 Web + MCP 工具能力

Status: `CANDIDATE / FROZEN BOUNDARY — 未动代码`
Date: 2026-08-17
依据：`research-harness-network-tools.md`（Multica / Grok-build / DeepSeek-Harness 的
通用 harness 模式：进程适配 + MCP 注入 + 远程 MCP 客户端 + 凭据 env + 沙箱/护栏）。

## Goal 一句话

把 Loom 的 Chat/conversation 从“纯文本模型会话”升级为“治理化的工具型会话”：
让用户在 Chat 里也能让模型真正联网查询（web_search / 远端 MCP），全部走 Loom 既有
的 Enrollment + Policy + Evidence 生命周期，且不引入第二写入权威。

## In scope
- conversation responder 增加可选工具面：当用户为某 Profile 绑定了已验证的
  `web_search` / `mcp_server` Enrollment 时，模型可调用这些工具；否则保持纯文本
  （fail-closed）。
- 通用 MCP 注入层（对齐 Multica `NormalizeCodexLaunchArgs` + DSH `tools.ts` tool
  bridge）：把 enrollment 的工具以 MCP server 配置注入 Codex/OpenCode/DeepSeek/
  Claude 任一 runtime，公开名 `mcp__<server>__<tool>`（64 字符、SHA-256 归一化）。
- 远程 MCP 客户端（对齐 grok `mcp_http_client.rs` / multica `pkg/remotemcp`）：
  streamable HTTP/SSE，OAuth 凭据存 Vault（复用凭据生命周期，不落源码/日志）。
- 护栏：SSE 指数退避重连（STABLE=2s/BASE=500ms/MAX=30s）、工具超时、重复工具护栏
  （对齐 grok circuit-breaker / dsh guard）。
- macOS + TUI 会话视图展示“模型用了哪些工具 / 引用来源”；Evidence 记录工具调用
  摘要（digest-only，无 prompt/凭据）。

## Out of scope（冻结）
- 不建第二套工具执行权威；所有写仍走 Journal CAS。
- 不做任意 Adapter / 原始 endpoint / 动态插件；只用 Loom 已收录的
  `web_search` / `mcp_server` enrollment 类型（延续 V31/V34 边界）。
- 不做跨账户/多用户/自动批准；外部 A2A 仍属 4.4。

## 验收门槛
1. Chat 未绑定 enrollment → 模型无法调用工具（行为与现在一致，fail-closed）。
2. 绑定已验证 `web_search` → 模型真实联网返回有界结果并在会话中展示来源；
   结果以 Evidence 工件落盘（digest 一致、0600）。
3. 绑定远端 `mcp_server` → MCP 客户端连接成功并完成一次工具调用；OAuth 凭据
   只在 Vault，不外泄；SSE 断线按退避重连。
4. strict IPC 三处一致 + strict decode；错误码：`tool_unavailable`、
   `tool_timeout`、`enrollment_revoked`、`provider_auth`。
5. 全量矩阵 + 安装版 live E2E（macOS + TUI 各一次真实联网）+
   独立评审 + operator sign-off。

## 与 4.2/4.3/4.4/4.5 的关系
- 独立于 4.2（导入/导出合同）；可与 4.3（可替换传输）共享“MCP 注入层”这个接缝。
- 是 4.4（外部 A2A 席位）的工具面前提之一，但 4.4 不并入本切片。
