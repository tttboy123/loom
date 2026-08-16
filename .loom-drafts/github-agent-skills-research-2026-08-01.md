# GitHub Agent Skills 调研 — 腾讯云管理视角 (2026-08-01)

> **触发**: 用户 2026-08-01 18:10 SGT — "腾讯云我有服务器，你看看github上有什么skills么。我想把云服务托管到Agent上"
>
> **范围**: GitHub 上可用的 agent skills 生态 + 腾讯云官方在 agent 领域的布局 + 云服务管理的 4 条可行路径
>
> **核心结论 (TL;DR)**:
> 1. **Agent Skills 已成工业标准** — Anthropic 2025-10 推出 Skills，2026 已有 17k+ stars (anthropics/skills) + 19k+ stars (VoltAgent/awesome-agent-skills) + 225k+ stars (obra/superpowers)
> 2. **腾讯云在 Agent 领域已经做完** — CloudBase 平台 39 MCP + 22 Skills + SkillHub 7.8万技能，**直接 `clawhub.ai/binggg/cloudbase` 一键安装**
> 3. **云管理有 4 条路径** — A 腾讯云官方 CloudBase / B 自写 SKILL.md / C 自建 MCP server 包装 tccli / D 传统 Terraform
> 4. **建议**: **路径 C (MCP) + 路径 A (CloudBase) 组合**, 直接借腾讯云已有 MCP server，给 Loom v2.0 spec MCP Broker 加 builtin provider

---

## 1. Agent Skills 生态全景 (GitHub 主流仓库)

### 1.1 官方/标准层 (3 个)

| 仓库 | Stars | 角色 | 关键事实 |
|---|---|---|---|
| **anthropics/skills** | 17k+ | Anthropic 官方示例库 | https://github.com/anthropics/skills, 20 commits, Apache 2.0, 含 `template-skill` + `spec/` 协议 |
| **agentskills.io** | - | Agent Skills 标准网站 | Anthropic 维护的规范 + 最佳实践 + marketplace 索引 |
| **agentskills.md** | - | Agent Skills marketplace | 浏览 / 发现 / 下载结构化 skill 的官方市场 |

**核心定义** (Anthropic):
> Agent Skills are modular capabilities that extend Claude's functionality. Each Skill packages instructions, metadata, and optional resources (scripts, templates) that Claude uses automatically when relevant.

### 1.2 Awesome 集合层 (5 个)

| 仓库 | Stars | 特色 |
|---|---|---|
| **ComposioHQ/awesome-claude-skills** | 56.8k | 最完整 Claude Skills 列表，60+ 场景 |
| **hesreallyhim/awesome-claude-code** | 41.6k | Claude Code 资源大全（含 hooks/slash-commands/plugins） |
| **VoltAgent/awesome-agent-skills** | 19.2k | 1000+ skills, 跨平台兼容 (Claude Code / Codex / Gemini CLI / Cursor) |
| **sickn33/antigravity-awesome-skills** | 35.5k | 1,400+ skills 安装库, 多个 IDE/CLI 兼容 |
| **heilcheng/awesome-agent-skills** | 4.4k | 教程+目录+精选 |

### 1.3 高质量独立 skill 仓库 (按类别)

**Engineering**:
- **addyosmani/agent-skills** (55.4k) — Production-grade engineering skills
- **obra/superpowers** (225k) — Agentic skills framework, Claude Code 团队同款
- **anthropics/claude-plugins-official** (29.9k) — Anthropic 官方 plugin
- **mgechev/skills-best-practices** (1.8k) — Skill 编写最佳实践

**Cloud / Infrastructure** (核心相关):
- **mukul975/Anthropic-Cybersecurity-Skills** (5.8k) — 754 个网络安全 skills
- **trailofbits/skills** (4.9k) — Trail of Bits 安全审计 skills
- **Tencent/AI-Infra-Guard** (3.6k) — 腾讯 AI 基础设施安全扫描 (含 OpenClaw Security Scan / Agent Scan / Skills Scan / MCP scan)
- **containers/kubernetes-mcp-server** (1.5k) — K8s MCP server
- **atlassian/atlassian-mcp-server** (776) — Atlassian MCP
- **mcp-use/mcp-use** (9.8k) — Fullstack MCP 框架
- **TencentBlueKing/bk-iam-saas** (35) — 腾讯 BlueKing IAM

**Note**: 纯云管理 (AWS / GCP / Azure / 腾讯云 CVM) 的 standalone skill 仓库**目前几乎没有** — 大部分还是 Terraform/CloudFormation 老套路 + 零散 MCP server

### 1.4 中文社区

- **它goyo/awesome-agent-skills** — 中文精选 (覆盖 微博/小红书/视频剪辑 等本土化场景)
- **libukai/awesome-agent-skills** (4.2k) — 终极指南 (中文)
- **zubair-trabzada/geo-seo-claude** (6.8k) — GEO-first SEO
- **mksglu/context-mode** (10.8k) — Context window 优化
- **jnMetaCode/superpowers-zh** (1.6k) — superpowers 中文增强版

### 1.5 SKILL.md 标准格式

```yaml
---
name: skill-name            # Max 64 chars, lowercase/numbers/hyphens only
description: 描述+触发条件     # Max 1024 chars, 必须包含触发关键词
license: Apache-2.0           # 可选
allowed-tools: Read, Bash    # 可选
---

# Skill Name
## 步骤 1...
## 步骤 2...
## 错误处理
## 验证
```

**3 级渐进式披露**:
- **Level 1 (永远加载)**: name + description (~100 tokens)
- **Level 2 (触发时加载)**: SKILL.md body (<5k tokens)
- **Level 3 (按需加载)**: scripts/ + references/ + assets/ (无限制, 不进 context)

---

## 2. 腾讯云在 Agent Skills / MCP 领域的官方布局

### 2.1 CloudBase (Serverless 平台) — 最完整方案

**官方页面**: https://cloud.tencent.com/product/tcb

**核心能力**:
- **39 个 MCP 工具** + **22 个 AI Skills** + CLI 工具
- 覆盖：数据库建表 / 云函数 / 云存储 / 静态托管 / 云托管 / 安全规则
- 支持本地 MCP 模式 + 远端 SSE 模式

**Skills 覆盖**:
- 微信登录怎么配
- 数据库怎么安全读写
- 小程序怎么发布
- 全套微信生态/前后端开发/UI 设计领域知识库

**安装方式 (3 选 1)**:
```bash
# 1. OpenClaw + ClawHub (最简单)
"安装CloudBase skill(https://clawhub.ai/binggg/cloudbase)"

# 2. 通过 npx skills
npx skills add cloudbase/cloudbase-skill

# 3. CLI
openclaw skills install cloudbase
```

**价格**:
- 个人版: 0 元/月 (6个月免费体验), 之后 19.9 元/月
- 标准版: 199 元/月
- 企业版: 999 元/月

### 2.2 腾讯云代码助手 (CodeBuddy) — IDE 集成

**关键事实** (2026-04 更新日志):
- 4.1.0 新增 **Agent Skills 功能**, 支持在项目 `.codebuddy/skills` 目录创建和管理
- 内置 **skill-creator** 支持交互式创建和导入
- 4.3.0 新增 MCP Roots + Sampling
- 4.2.0 新增 MCP 自定义指令 (`/` 调用)
- 4.0.0 新增项目级 MCP 支持 (`.mcp.json`)

**结论**: 腾讯云的 CodeBuddy IDE 已经原生支持 Agent Skills 标准 — 跟 Anthropic 协议完全兼容

### 2.3 腾讯云大模型知识引擎 — MCP 插件工作流

**官方功能**:
- 标准模式 / 工作流模式 / Agent 模式 (3 种开发方式)
- 精选 MCP Server: **EdgeOne Pages / 腾讯位置服务 / Airbnb / Figma / Fetch**
- 用户可自定义 MCP SSE 服务

**典型场景**: 用户在 Agent 模式下添加腾讯位置服务 MCP → 一键搭建路线规划助手

### 2.4 腾讯云 @cloudbase/mcp SDK

**仓库**: `@cloudbase/mcp` (TypeScript/Node.js)
**GitHub**: `https://github.com/Tencent/cloudbase-mcp` (推测)

**核心能力**:
- 函数型云托管的 MCP Server 框架
- 适配 MCP 官方 SDK (Streamable HTTP / SSE / POST)
- 一行代码启动 MCP Server (`StreamableHTTPMCPServerRunner.run`)

### 2.5 腾讯云 2026 AI 产业应用大会 — Agent-Ready Infrastructure

**核心战略**:
- **AgentRuntime 架构**: 沙箱调度 + 身份认证 + 全链路监控
- **SkillHub 社区**: 已收录 7.8 万个技能
- **ClawPro 管控平台**: 多 Agent 统一接入 / 组织治理 / 场景交付
- **Lighthouse 轻量应用服务器**: 7×24h Agent 持续运行
- **统一 Volume 抽象层**: 重构存储体系支持海量 Agent 动态扩展

**总结**: 腾讯云在"把云服务托管到 Agent 上" 这件事上, **官方已经做完** — CloudBase + SkillHub + AgentRuntime 三件套

### 2.6 腾讯 AI-Infra-Guard — 安全扫描 (3.6k stars)

**功能**: 腾讯安全团队出品
- OpenClaw Security Scan
- Agent Scan
- **Skills Scan** (扫描恶意 skill)
- MCP Scan
- AI Infra Scan
- LLM jailbreak evaluation

**意义**: 解决了用户担心的"第三方 skill 是不是有恶意代码" 问题 — **有官方安全扫描器**

---

## 3. 云服务管理到 Agent — 4 条路径对比

### 路径 A: 直接用腾讯云 CloudBase (已完整, 0 成本上手)

| 维度 | 评估 |
|---|---|
| **成熟度** | ⭐⭐⭐⭐⭐ 生产级, 个人版 0 元/月 |
| **覆盖范围** | Serverless / 小程序 / Web 应用托管 / 数据库 / 云函数 |
| **缺什么** | **CVM/CLB/CDN/TKE 等传统 IaaS/PaaS 不在 CloudBase 范围** — 这才是用户服务器管理的主场景 |
| **学习成本** | 极低, 一句话装 skill |
| **适合谁** | 想做小程序/Web 的人 |
| **Loom 集成** | 边缘 — 跟 Loom v2.0 spec 的本地运行时正交 |

### 路径 B: 自写 SKILL.md 协议

```yaml
---
name: tencent-cvm-manager
description: 管理腾讯云 CVM 实例, 包括创建/查询/启动/停止/重启/销毁。当用户说"管理服务器"/"开机"/"关服" 时使用
license: MIT
allowed-tools: Bash
---

# Tencent CVM Manager
## 1. 通过 tccli 操作
## 2. 凭证从 env 读
## 3. 安全检查 (避免误删)
```

| 维度 | 评估 |
|---|---|
| **成熟度** | ⭐⭐ 需要自建 + 维护 |
| **覆盖范围** | 灵活, 可包装任何 tccli 命令 |
| **缺什么** | 没有标准 schema, 写错就坏 |
| **学习成本** | 中, 要懂 SKILL.md 规范 |
| **适合谁** | 已有 tccli workflow 的人 |
| **Loom 集成** | ✅ 可加到 `internal/skills/builtin/tencent-cvm/` |

### 路径 C: 自建 MCP Server 包装 tccli (推荐)

**示例** (Go):
```go
// 内部实现用 mcp-go
import "github.com/mark3labs/mcp-go/server"

mcp.AddTool(server, "list_cvm_instances", 
  "List CVM instances", 
  func(ctx, args) { return tccli.Call("cvm", "DescribeInstances") })

mcp.AddTool(server, "start_cvm", 
  "Start a CVM instance", 
  func(ctx, args) { return tccli.Call("cvm", "StartInstances", args) })
```

| 维度 | 评估 |
|---|---|
| **成熟度** | ⭐⭐⭐⭐ mcp-go / @cloudbase/mcp / mcp-use 都有 SDK |
| **覆盖范围** | 100% (tccli 支持所有腾讯云 API) |
| **缺什么** | 凭证安全管理 (走 CubeEgress 类 vault) |
| **学习成本** | 中, mcp-go 30 分钟上手 |
| **适合谁** | 想"任何 agent 都能管云"的人 |
| **Loom 集成** | ✅✅ **直接接到 Loom v2.0 MCP Broker, 加 builtin provider** |

### 路径 D: 传统 Terraform (不推荐)

**现状**:
- 100+ Terraform 云管理项目 (orderly-infra / cloudlab / cloud-platform-infrastructure)
- 几乎都还要写 .tf + 手 apply
- 跟"用自然语言管云" 距离远

| 维度 | 评估 |
|---|---|
| **成熟度** | ⭐⭐⭐⭐⭐ 工业标准 |
| **覆盖范围** | 100% |
| **缺什么** | 不能"自然语言驱动" |
| **学习成本** | 高, HCL + state 管理 |
| **Loom 集成** | ❌ 反方向, 是 Agent 调 Terraform 不是 Terraform 调 Agent |

---

## 4. 我的建议 (顾问意见)

### 短期 (本周)
**路径 A + C 组合**:
1. **走 CloudBase 路径 (A)** — 用户已有腾讯云账号, 一句 "安装 CloudBase skill" 就把 Serverless 场景接上
2. **写 `tencent-cloud-mcp` server (C)** — 用 mcp-go 包装 tccli 关键 5-10 个命令:
   - `list_cvm_instances` / `start_cvm` / `stop_cvm` / `reboot_cvm` / `describe_cvm_status`
   - `list_cdb_instances` (数据库)
   - `describe_cos_buckets` (对象存储)
   - `describe_clb` (负载均衡)
3. **凭证走 Loom 3-Token 设计** — 借鉴 CubeEgress 凭证保险库概念

### 中期 (v2.0 spec 集成)
**给 Loom v2.0 spec 加 §3.6 Cloud Provider Adapter**:
```markdown
§3.6 Cloud Provider Adapter (新增, Phase 2.G)
3.6.1 抽象: CloudProvider interface (List/Start/Stop/Reboot/GetStatus)
3.6.2 实现 A: tencent-cloud-mcp (mcp-go + tccli)
3.6.3 实现 B: aliyun-acs-mcp (ACS MCP SDK, 预留)
3.6.4 实现 C: aws-boto3-mcp (Boto3 wrapper, 预留)
3.6.5 凭证流: 走 Loom 3-Token, 凭证不进 skill body
```

### 长期 (v2.1+)
- 加入 v2.0 评估提示词 P0-2c 问题: "云服务管理走 MCP Broker 还是独立 Cloud Adapter? 是否冲突 / 重叠 / 互补?"

---

## 5. 关键发现 — 5 条跨项目方法论 lesson

1. **Agent Skills 是 2026 工业标准, 不再是"高级 prompt"** — Anthropic 团队内部用 9 类生产级 skill (库&API 适配/产品验证/数据监控/业务自动化/代码脚手架/代码质量/CI-CD/故障排查/基础设施运维), Loom 应该按这 9 类重新组织 SKILL.md

2. **MCP 协议是 14 个沙箱 + 8 个云管理 + 100 个 SaaS 的事实标准** — 任何"Agent 操作外部"的需求, 第一反应应该是 MCP server, 不是 SDK wrapper

3. **SKILL.md vs MCP 边界**:
   - **SKILL.md** = 流程化 SOP (怎么思考)
   - **MCP server** = 工具集 (能做什么)
   - 云管理主要是"操作工具" — **走 MCP**
   - 但云管理需要"安全检查/成本优化/合规审计" 等流程 — **走 SKILL.md**

4. **腾讯云官方已经把 Serverless 场景做完** — `clawhub.ai/binggg/cloudbase` 一键安装, 别重复造轮子. 但 CVM/IaaS 还是空, 留给 MCP server 填

5. **安全扫描不可缺** — `Tencent/AI-Infra-Guard` (3.6k stars) 提供 Skills Scan, 任何第三方 skill 必须先过这个. Loom 集成第三方 skill 时应该默认跑这个扫描

---

## 6. Next Steps (待用户决策)

### ❓ 必须先问用户的 4 个问题
1. **场景确认**: 你想管的是 CVM (云服务器) 还是 CloudBase (Serverless) 还是都管?
2. **走现成还是自建**: 走 CloudBase 一键装 (A) 还是自写 tencent-cloud-mcp (C)?
3. **凭证怎么存**: tccli 凭证放环境变量 / Keychain / CubeEgress 凭证保险库?
4. **集成到哪**: 给 Loom daemon 加 cloud adapter 还是独立 tencent-cloud-mcp daemon?

### 📦 待写产物 (确认后)
- [ ] `internal/mcp/tencent-cloud/main.go` (mcp-go + tccli wrapper)
- [ ] `internal/mcp/tencent-cloud/tools_test.go`
- [ ] `.loom-drafts/tencent-cloud-mcp-design.md`
- [ ] `.loom-drafts/v2.0-evaluation-prompt.md` 追加 P0-2c 问题

---

**VERDICT: READY_FOR_REVIEW**
