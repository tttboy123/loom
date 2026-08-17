# Phase 4 · 安装版 daemon 远程工具 broker 实机验证（enrollment 物化 + Team + 隔离）

Status: `INSTALLED-LIVE PASS`
Date: 2026-08-17

## 变更
- `cmd/loomd/run.go`：`missionExecutionConfigFromDaemonBuild` 在
  `LOOM_ENABLE_WEB_TOOLS=1` 时注入 `newProductDefaultRemoteToolBrokerConfig()`
  （DDG Search + WebFetch，有界）。默认生产仍 fail-closed。
- 安装版已重装并带该 env 启动（`launchctl setenv LOOM_ENABLE_WEB_TOOLS 1`）。

## Live 结果（G5 enrollment 隔离 gate，`LOOM_LIVE_TEAM_E2E=1`，安装版 socket）
```
deepseek already verified
governance configured for deepseek/deepseek.primary (deepseek-chat)
  node main provider=deepseek status=ready
  node mission-role-… provider=deepseek status=ready     ← web_search 绑定节点 ready
post-revoke directory enrollment enroll-web-live-… status=revoked
  node main … status=ready
  node mission-role-… status=blocked block="Remote tool enrollment was revoked. Restore or re-select it…"
bound subagent before="ready" after="blocked" (isolation confirmed)
--- PASS: TestLiveRemoteToolEnrollmentIsolationE2E
```

关键点：
- 带 web_search enrollment 的 subagent **preflight ready** → 证明安装版 daemon 的
  broker 已启用、enrollment 真正物化为可执行远程工具执行器（此前无 Search 后端时
  物化返回 port_unavailable）。
- 撤销 enrollment 后仅该 agent 被隔离阻塞（main 保持 ready）——隔离语义保持。

## 说明（诚实边界）
- 本 gate 验证的是“broker 开启 + enrollment 物化 + Team 就绪 + 撤销隔离”，
  未在本次 run 中让模型主动发起一次搜索（G5 的 prompt 不要求联网）。
- “模型在 attempt 里主动调用 web 工具并消费结果”由独立 live 工具循环证明
  （`live-model-tool-loop.md`：DeepSeek 自己调 web_fetch/web_search →
  toolbroker → 互联网 → 作答）。
- 默认（未设 env）生产保持 fail-closed（既有测试覆盖）。
