# Phase 4 · harness loom_context MCP 挂载 web 工具（模型可见可调）

Status: `IMPLEMENTED + UNIT VERIFIED`（全链路 live 见边界说明）
Date: 2026-08-17

## 变更
- `internal/runtime/harnessadapter/context_mcp.go`：`newHarnessAttemptMCPWithContext`
  从 gateway `AllowedToolCalls()` 里把 `ToolWebSearch/ToolWebFetch` 映射为
  `loom_web_search` / `loom_web_fetch` 工具；lease 增加
  `WebSearchEnabled/WebFetchEnabled`；`decodeHarnessToolCall` 支持
  `{"query":...}` / `{"url":...}`（拒绝未知字段/空参）。
- `claude_code_process.go` `harnessMCPToolNames`：注入给 Agent 的
  `--tools/--allowedTools` 与 gateway allowlist 现在包含 web 工具
  （`mcp__loom_context__loom_web_search/fetch`）。
- daemon gateway（`bridgeExecutionHook.AllowedToolCalls`）本就会追加
  `adapter.RemoteToolKinds()`（broker 开启时含 WebSearch/WebFetch）——
  无需改动。

## 单元验证（RED-first）
- `TestHarnessMCPToolNamesIncludesWebTools`：lease 含 web → 工具名单含
  loom_web_search/loom_web_fetch。
- `TestHarnessDecodeWebToolCalls`：query/url 解码正确；空参/未知字段被拒。
- `TestHarnessAttemptMCPPublishesAndExecutesWebTools`：MCP 发布
  [loom_web_fetch, loom_web_search]；`tools/call` 把 `{"query"}`/`{"url"}`
  转成 gateway 的 `ProposedCall{WebSearch, Path=query}` /
  `{WebFetch, Path=url}` 并回传有界结果。
- `internal/runtime/harnessadapter` 全包 + race 绿；全量 Go 套件（串行）绿。

## 链路与边界
```
模型（Agent CLI）→ 注入的 loom_context MCP：tools/list 见 loom_web_search/fetch
  → tools/call → decodeHarnessToolCall → gateway.ExecuteToolCall
  →（daemon bridgeExecutionHook）→ adapter.RemoteToolKinds（broker 开启时）
  → toolbroker（DDG Search / SSRF-safe Fetch）→ 真实互联网 → 有界结果回传
```
- “真实 attempt 里模型自己调用”这一步仍需完整 Mission/attempt 运行
  （Pi runtime + Team/enrollment 物化）——属下一块；本切片把模型可见、
  可调用的工具面补上了，且执行路径的每一段（MCP→gateway→broker→互联网）
  都分别有测试/live 证据。
