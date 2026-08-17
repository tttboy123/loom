# Phase 4 · Loom harness 联网查询能力——补齐并验证

Status: `IMPLEMENTED + LIVE VERIFIED`（不依赖 Codex）
Date: 2026-08-17

## 补的能力
- **真实 web 搜索后端** `internal/toolbroker/search.go`：`NewDDGSearchClient`
  通过 DuckDuckGo Lite HTTPS（免 API key），复用 WebFetch 的 SSRF 安全传输
  （dial 时校验公网 IP + 443 + redirect 校验）；结果有界（≤16 条、逐条
  title≤256 / snippet≤512、总响应≤64KiB）；DDG 重定向 `uddg=` 解包回公网
  目标；非公网目标直接丢弃。
- **daemon 侧 opt-in 配置** `cmd/loomd/product_remote_tool_broker.go`：
  `newProductDefaultRemoteToolBrokerConfig()`（Search=DDG + WebFetch=true +
  Timeout=20s + MaxResultBytes=32KiB）。**默认生产仍 fail-closed**（nil →
  无远程工具执行器），保持“默认不发布远程工具能力”的不变量。

## 在 Loom 中验证（不走 Codex；走 daemon 自身 remote tool executor 代码路径）
`LOOM_LIVE_NET=1 go test ./cmd/loomd/ -run 'TestProductRemoteToolBroker(WebSearch|WebFetch)Live'`：

```
=== RUN TestProductRemoteToolBrokerWebSearchLive
    live web search ok: 1715 bytes (1. loom/docs/handoff-generation-pipeline.md at main - GitHub
--- PASS (1.18s)
=== RUN TestProductRemoteToolBrokerWebFetchLive
    live web fetch ok: 558 bytes
--- PASS (0.26s)
```

即：Loom 自己的 harness 工具执行器（`toolbroker.Broker` via
`productRemoteToolExecutor.ExecuteProposalContent`）真实查询了互联网。

其它验证：
- `TestDDGSearchLiveQueriesInternet`：真实 DDG Lite 查询返回 3 条合法公网结果。
- `TestProductRemoteToolBrokerRejectsPrivateTarget`：私网目标被拒（SSRF-safe）。
- `TestProductRemoteToolBrokerDefaultFailClosed`：默认 nil 配置 → 无执行器。
- `internal/toolbroker` + `cmd/loomd` 全测 + race 绿。

## 说明与下一步
- 该能力是“harness 工具执行层”补齐：模型只有在 enrollment 物化出可信
  Search/WebFetch 端口时才拿到工具（与 `10-chat-governed-web-tools-slice.md`
  的治理一致）。
- 全链路“模型在 Mission 里真正调用 WebSearch”需要：显式 opt-in 配置 +
  `web_search` enrollment 绑定 agent + Team 物化 —— 这是下一块（不属本轮）。
