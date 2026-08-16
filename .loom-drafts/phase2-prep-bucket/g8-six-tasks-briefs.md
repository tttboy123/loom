# G8 Phase 2 P0 准备桶 — 6 Task Briefs

> **替代**: `.loom-drafts/loom-graph-engineering-plan.md` G8 (目前 "Phase 2 P0 准备" 1 行, 0 task 拆)
> **目的**: 把调研 100% 齐但 0 work item 的 6 个 Phase 2 准备项拆成 task, 替代 G8 空白
> **状态**: DRAFT, 等你审/freeze 后并入 Graph Plan v2

---

## G8 6 task 总览

| G# | ID | 标题 | 调研齐度 | 估时 | 跟 Slice 1/2 关系 |
|---|---|---|---|---|---|
| G8.1 | **FCS-1** | FastContext Local Spike | 100% (ADR-0006 + integrations/fastcontext.md 209 行) | 2 周 | 独立, 不阻塞 S2 |
| G8.2 | HOOK-1 | Bridge Hook 协议 (Phase 2 增补 ①) | 100% (Claude Code 22 events + grok-cli + Codex App) | 1 周 | 跟 S2-W2 daemon 关联 |
| G8.3 | SAND-1 | Sandbox 后端选型 (Phase 2 增补 ②) | 100% (AgentScope 2.0 + Docker + gVisor) | 1 周 | 跟 S2-W3 Runtime Adapter 关联 |
| G8.4 | PI-1 | pi-coding-agent RuntimeProfile (Phase 2 增补 ④) | 100% (Earendil Pi rebrand 2026-07) | 3-4 天 | 跟 S2-W1 RuntimeProfile 关联 |
| G8.5 | PROV-1 | Provider Routing 边界 (Phase 2 增补 ③) | 100% (v0.4 决策 + ADR-0004 已有) | 1 周 | 跟 ADR-0004 Credential Broker 关联 |
| G8.6 | CH-1 | ClickHouse OLAP (P0-19) | 100% (Langfuse 16k★ + ClickHouse acquired 2026-01) | 2-3 周 | 独立, Phase 2 cost view 基础 |

**总估时**: 6 task 串行 ~7-8 周, 部分可并行

---

## G8.1 FCS-1: FastContext Local Spike

**详细 contract**: `.loom-drafts/phase2-prep-bucket/fcs-1-contract.md` (本目录)

**核心**:
- 装 community FastContext SFT (loopback-only)
- 写 Loom-owned read-only adapter
- 跑 Loom-specific 评估 set
- 1-2 周 spike 输出 ADR-0006 修订决策

**Slice 1/2 关系**: 独立, 不阻塞

---

## G8.2 HOOK-1: Bridge Hook 协议 (Phase 2 增补 ①)

**调研来源**:
- v0.4 § 1 实证: Claude Code v2.1+ Hook 协议 22 事件
- v0.4 § 1 实证: grok-cli + Codex App hook 生命周期
- v0.3 § 1 SKILL.md 开放标准 (Anthropic 2025-10-16)
- v0.4 "待补": `docs/architecture/bridge-hooks.md` (Phase 2 增补 ①)

**核心**:
- Bridge v1.1 (S2-W1) 加 6-8 个 hook point (per Claude Code 22 events 选 6-8)
- Hook lifecycle: pre-tool / post-tool / pre-agent / post-agent / on-error / on-approval
- Hook payload schema: JSON-RPC 2.0 兼容, W3C Trace Context 传播

**Slice 1/2 关系**: 跟 S2-W1 (Bridge v1.1) 关联, S2-W1 已定 envelope, HOOK-1 加 hook 扩展

**Owned files**:
- `docs/architecture/bridge-hooks.md` (新建)
- `internal/bridge/hooks.go` (新建)

---

## G8.3 SAND-1: Sandbox 后端选型 (Phase 2 增补 ②)

**调研来源**:
- v0.4 § 152 Phase 2 增补 ②: Sandbox 后端选型
- AgentScope 2.0 sandbox 矩阵 (永久借鉴清单)
- Docker / gVisor / firecracker 候选

**核心**:
- 评估 3-5 个 sandbox 后端 (Docker / gVisor / firecracker / AgentScope 矩阵 / Monty 沙箱)
- 选 1 个作为 Phase 2 默认 (候选: gVisor + Monty 二选一, 轻量 + 沙箱化)
- 出 `docs/integrations/sandbox-backends.md` 选型报告

**Slice 1/2 关系**: 跟 S2-W3 (Coordinator) + S2-W5 (Capabilities) 关联, Runtime Adapter 选 backend

**Owned files**:
- `docs/integrations/sandbox-backends.md` (新建)
- `internal/sandbox/adapter.go` (新建)

---

## G8.4 PI-1: pi-coding-agent RuntimeProfile (Phase 2 增补 ④)

**调研来源**:
- v0.4 § 178 Phase 2 增补 ④: 第 2 种 RuntimeProfile 候选 = pi-coding-agent
- Earendil Pi rebrand 2026-07 (`badlogic/pi-mono` → `earendil-works/pi-mono`)
- Pi 极简哲学 (4 工具 + YOLO + Session tree)
- Pi supply-chain hardening (`min-release-age=2` / `save-exact=true`)

**核心**:
- 写 `internal/runtime/pi/profile.go` — Pi 作为第 2 种 RuntimeProfile (S2-W1 的 catalog 已经有 Codex + Claude, 加 Pi)
- 验证: Pi SDK 可嵌入式调用, 不绑 Provider
- 出 `docs/integrations/pi-coding-agent.md` 集成规范

**Slice 1/2 关系**: 跟 S2-W1 (RuntimeProfile catalog) 关联, 加 1 个 profile

**Owned files**:
- `docs/integrations/pi-coding-agent.md` (新建)
- `internal/runtime/pi/profile.go` (新建)
- `internal/runtime/pi/profile_test.go` (新建)

---

## G8.5 PROV-1: Provider Routing 边界 (Phase 2 增补 ③)

**调研来源**:
- v0.4 § 166 Phase 2 增补 ③: Provider routing 边界再明确
- ADR-0004 Credential Broker Authentication Modes (accepted 2026-07-24, 0 work item)
- D3 必决: 拒代理 LLM (fail closed sampling/createMessage)
- TECH-PLAN.md 798: "Phase 1 使用 Runtime 原生认证; Phase 2 实现 Broker"

**核心**:
- 出 `internal/credential/broker.go` — Loom Credential Broker
- 路由策略: Loom Broker 代理 vs Runtime 原生认证, 3 态 (ALLOW / DENY / ASK)
- Hook 拦截 sampling/createMessage → fail closed (D3 必决实施)
- 修订 `TECH-PLAN.md §11` 末段 (per v0.4 § 771 待补)

**Slice 1/2 关系**: 跟 S2-W2 (Daemon) 关联, broker 是 daemon 内的 Component

**Owned files**:
- `internal/credential/broker.go` (新建)
- `internal/credential/broker_test.go` (新建)
- `docs/integrations/provider-routing.md` (新建)
- `TECH-PLAN.md` §11 末段修订

---

## G8.6 CH-1: ClickHouse OLAP (P0-19)

**调研来源**:
- v3-deepdive: P0-19 "ClickHouse 作为底层 OLAP (Loom cost view 数据量大了可以借鉴)"
- Langfuse (16k★, ClickHouse acquired 2026-01)
- v3: 50M+ SDK installs/month, 10B+ observations/month (Langfuse 实证)

**核心**:
- 评估 ClickHouse vs DuckDB vs SQLite (OLAP 选型)
- 出 `internal/costview/store.go` — Cost view 持久化层
- 跟 P1-7 (MCP server built-in cost view) 关联, 暴露 MCP 给 Claude Code / Codex

**Slice 1/2 关系**: 独立, Phase 2 cost view 基础, 不阻塞 S2

**Owned files**:
- `docs/integrations/costview-store.md` (新建)
- `internal/costview/store.go` (新建, ClickHouse adapter)
- `internal/costview/store_test.go` (新建)

---

## 跟 Graph Plan v1 16 task 关系

Graph Plan v1 G8 = "Phase 2 P0 准备" (1 行, 0 task)。本 brief 拆 6 task, 替代 G8:

**替换方案 A** (推荐): 改 Graph Plan v1 G8, 拆 6 task, 重新 freeze
**替换方案 B**: 本 brief 单独存在, 不动 Graph Plan, 等 Phase 1 完再合并

**风险**: 改 DRAFT 文档需要 user review。如果你不想重 freeze Graph Plan, 走方案 B。

---

## 估时 + 依赖图

```
G8.1 FCS-1 (2 周) ──独立
G8.2 HOOK-1 (1 周) ──依赖 S2-W1 (Bridge v1.1 envelope)
G8.3 SAND-1 (1 周) ──依赖 S2-W3 (Runtime Adapter)
G8.4 PI-1 (3-4 天) ──依赖 S2-W1 (RuntimeProfile catalog)
G8.5 PROV-1 (1 周) ──依赖 S2-W2 (Daemon) + ADR-0004
G8.6 CH-1 (2-3 周) ──独立
```

**串行关键路径**: S2-W1 → G8.2 HOOK-1 + G8.4 PI-1 → S2-W2 → G8.5 PROV-1 → S2-W3 → G8.3 SAND-1
**总估时**: 关键路径 ~5-6 周, 加上 G8.1/G8.6 并行 ~7-8 周
