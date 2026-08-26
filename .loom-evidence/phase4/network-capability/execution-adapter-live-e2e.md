# Phase 4 · 权威 execution adapter 联网工具 E2E（Loom 内验证，无 Codex）

Status: `IMPLEMENTED + LIVE VERIFIED`
Date: 2026-08-17

## 目标
证明 Loom **权威 execution adapter**（`internal/execution.Adapter`）在
模型/提案提出 WebSearch/WebFetch 时，通过 Loom 自身 `toolbroker` 真实查询
互联网，并走 dispatch/result 提交门禁落账——全程不调用 Codex。

## Live 结果（`LOOM_LIVE_NET=1 go test ./internal/execution/ -run TestExecutionAdapter`）
```
--- PASS: TestExecutionAdapterWebSearchLive
    adapter live web search ok: 1715 bytes committed
    (1. loom/docs/handoff-generation-pipeline.md at main - GitHub
--- PASS: TestExecutionAdapterWebFetchIsGoverned
    adapter web fetch governed ok: verdict=ask reason=action requires approval
```

- **WebSearch**（只读工具）：adapter 执行真实治理搜索 → 结果经
  `DispatchGate` + `ResultCommitGate` 提交（`dispatch.wasCommitted()`、
  `resultGate.calls==1`、提交内容与返回一致、含真实源 URL）。
- **WebFetch**（非只读）：默认治理为 `VerdictAsk`（需人工审批）——
  “受治理的远程能力”按设计工作。

## 稳定性说明
- 公开 HTML 搜索端点（DuckDuckGo Lite / Bing / Mojeek）对本机数据中心 IP
  间歇性封禁（HTTP 202/302/503），这是**外部依赖**，不是 Loom 缺陷。
- live 搜索测试在端点封禁时 `Skip` 并注明，取到结果时严格断言；WebFetch
  （https://example.com）作为稳定的真实联网证明保持严格断言。
- 早前成功采样（端点未封禁时）：`TestDDGSearchLiveQueriesInternet` 返回 3 条
  真实结果；`TestProductRemoteToolBrokerWebSearchLive` 1715 字节。
- 默认（非 live）套件完全 hermetic：`go test ./... -p 1` 全绿。

## 链路
模型/提案 `ProposedCall{WebSearch/WebFetch}` →
`execution.Adapter.Execute`（校验 + dispatch gate + result commit gate）→
`toolbroker.Broker`（DDG Search / SSRF-safe HTTPS Fetch）→ 真实互联网 →
有界结果落账 → 模型可用结果作答（模型侧合成属 Pi attempt-loop 切片）。
