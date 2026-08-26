# Phase 4 · 真实模型工具调用循环 E2E（模型自己决定联网，无 Codex）

Status: `LIVE PASS`
Date: 2026-08-17
Gate: `LOOM_LIVE_NET=1 DEEPSEEK_API_KEY=... go test ./internal/toolbroker/ -run TestLiveModelToolCallLoopWebSearch`

## 结果
```
round 0: model called web_fetch("https://example.com") -> 558 bytes
live model tool-loop ok: search_calls=0 fetch_ok=1
answer=The page at https://example.com displays the title "Example Domain" ...
```

- **模型（DeepSeek）自己决定调用工具**（真实 function-calling，非脚本模拟）。
- **Loom toolbroker 执行**（SSRF-safe HTTPS fetch，有界 558 字节）。
- 结果回传 → 模型产出最终答案并引用真实 URL。
- 早前采样（端点未封禁时）：模型也主动调用过 `web_search("github.com/multica-ai/multica repository")`（搜索执行被外部端点封禁时记录并让模型继续）。

## 链路
真实模型（DeepSeek, function-calling）→ 提出 web_search/web_fetch →
`toolbroker.Broker`（DDG Search / SSRF-safe Fetch）→ 真实互联网 →
有界结果回传 → 模型作答。全程无 Codex、无外部 harness CLI。

## 说明
- 公开 HTML 搜索端点对本机数据中心 IP 间歇封禁（外部依赖）；web_fetch
  （example.com）作为稳定真实联网证明；web_search 执行在端点放行时有
  成功采样（1715 字节）。
- 本测试 env-gated（`LOOM_LIVE_NET=1` + `DEEPSEEK_API_KEY`），默认套件
  hermetic；`go test ./... -p 1` 全绿。
