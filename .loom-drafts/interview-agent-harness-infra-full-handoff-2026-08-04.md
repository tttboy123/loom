# Agent Harness / Agent Infra 求职研究与简历编写完整交接

> 日期：2026-08-04（Asia/Singapore）  
> 工作区：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`  
> 性质：新会话交接材料；不属于 Loom 产品合同或 accepted 状态。  
> 安全说明：本文不复制父线程中的 API Key、凭据或与本次面试无关的历史。

## 1. 用户目标

用户准备面试以下岗位方向：

1. Agent Harness 研究/工程；
2. Agent Infra / Agent Runtime；
3. Agent 沙箱与开发者运行环境；
4. DSec 控制面、可观测性与可靠性；
5. Kubernetes 节点与容器运行时；
6. 异构集群调度与训练基础设施；
7. 高性能网络、RDMA 与模型-集群协同（作为对比方向）。

希望基于：

- 腾讯 TKE 与 AgentRuntime 工作经历；
- 旧校招简历；
- 当前 Loom 项目；
- 公开 GitHub；
- 网络上的公开职位和技术资料；

完成整体能力评估、面试问题准备、GitHub 审计和一版可投递简历。

## 2. 已提供材料

### 2.1 旧校招简历

文件：

`/Users/lune/Downloads/周勃译-18645757689.pdf`

PDF 为一页 A4，创建于 2024-09-01。简历事实：

- 姓名：周勃译；
- 哈尔滨理工大学，计算机科学与技术本科，2021.09-2025.06；
- 腾讯科技 CSIG/TKE 产品组后端实习，2023.11-2024.07；
- 恒安嘉新 C 开发实习，2023.06-2023.08；
- 技术栈：Go、C/C++、Linux、Docker、Kubernetes、MySQL、Redis、网络；
- 做过 mini-docker；
- TKE 节点版本统计与升级；
- 使用 pprof 将相关组件内存占用降低约 20%；
- 项目包括 RTP/RTSP/QUIC 流媒体、Raft 分片 KV；
- 完成过 CSAPP、MIT 6.S081、CS144 课程实验。

旧简历主要问题：

- 仍然是校招/学生表述；
- 没有腾讯正式工作和 AgentRuntime；
- 大量“熟悉/了解”，缺少生产结果；
- Loom 和公开性能成果没有进入简历；
- 技术主线不突出；
- 部分项目与目标岗位相关性低。

### 2.2 腾讯正式工作经历（用户陈述）

入职腾讯科技后，参与 AgentRuntime 和 TKE 研发。

TKE 期间主要负责：

- Pod 启动速度观测和优化；
- 节点原地升级；
- TencentOS Rainbow 接入；
- 节点相关研发。

AgentRuntime 期间主要负责：

- 围绕 CubeSandbox 建设监控 Sidecar。

重要更正：不要只写“沙箱监控开发”，准确定位是：

> 面向 CubeSandbox 微虚拟机运行环境的可观测性 Sidecar 工程。

当前仍缺少以下内部经历细节，不能编造：

- 正式入职、转组时间；
- Sidecar 部署位置；
- 每 Sandbox 一个还是每节点共享；
- 使用语言；
- 与 CubeSandbox 通信方式；
- 采集的数据类型；
- 节点、沙箱和日任务规模；
- Sidecar CPU/RSS/延迟开销；
- 典型事故和量化结果；
- Pod 启动优化的分阶段数据与 p95/p99；
- Rainbow 接入的准确 owned boundary。

## 3. 用户给出的目标岗位要求摘要

### 3.1 Agent Harness

岗位关注：

- Context management；
- 长期记忆；
- Subagent / Multi-Agent；
- 自进化 Agent；
- 超长程任务；
- 模型和 Harness 协同；
- benchmark、数据与标注；
- 真实世界反馈闭环；
- LLM API、KV Cache、Agent Loop、Tool Use、Reasoning、Planning、Skills、MCP、Memory；
- 研究能力、快速原型和实验迭代。

学历和科研成果是显著筛选风险：用户为本科，公开材料未显示论文或科研成果。

### 3.2 DSec / Agent Infra

岗位关注：

- VM、容器、临时存储、虚拟网络；
- 控制面、日志、可观测性、高可用；
- 无监管 Agent 带来的安全挑战；
- 系统编程；
- 分布式系统；
- 压测、跨层诊断、事故防复发；
- Rust/C/Python/Go；
- 沙箱、弹性调度、容器存储。

### 3.3 异构集群与训练基础设施

岗位关注：

- CPU/GPU/NPU 资源抽象、池化和拓扑；
- 优先级、吞吐、排队延迟、利用率；
- 训练环境、镜像、监控；
- RDMA、RoCEv2、InfiniBand、拥塞控制；
- 快慢节点、性能抖动和自动容灾；
- cgroup、namespace、Kubernetes、sched_ext、QoS；
- DPU/P4、GPU/AI 加速器；
- HPC。

## 4. Loom 项目真实能力映射

本次检查以 accepted baseline `6d380233` 为准，不把当前 Phase 3A dirty/uncommitted 内容算作已交付。

本地 accepted baseline 快照：

- 117 个提交；
- 1316 个 tracked files；
- 233 个 Go 文件；
- 106 个 Go 测试文件；
- 30 个 Swift 文件；
- 13 个 Swift 测试文件；
- 约 149842 行 Go；
- 约 15928 行 Swift；
- Phase 2A 和 P2B 已接受；
- Phase 3A 仍处于 RED/开发阶段，不能作为完成能力宣称。

Loom 可以证明：

- local-first Agent Harness/control plane；
- Event Journal 单一状态权威；
- append-only SQLite 与 content-aware idempotency；
- stream-head CAS；
- transaction-consistent `ReadStreamSet`；
- rebuildable Projection / GlobalReadView；
- Team DAG 与 deterministic ready set；
- 一个 Main、最多两个 SubAgent；
- Runtime capacity；
- Run / Attempt / Generation / Grant / Evidence lineage；
- stale generation rejection；
- Supervisor frame validation；
- tentative streaming 与 terminal Evidence 分离；
- bounded retry、fallback、degraded、blocked、human_required；
- 原生 macOS GUI、Bubble Tea TUI、CLI 共用 daemon IPC/application service；
- RED、race、独立 Reviewer、controlled live canary。

不能宣称：

- 成熟的长期语义记忆；
- 已完成上下文压缩实验；
- 生产级 RAG benchmark；
- GPU/NPU 调度；
- VM/VPC/分布式存储生产规模；
- Phase 3A Versioned Asset/Skill 已完成。

## 5. Agent 架构面试核心答案

对需要 4-5 个工具的任务，推荐回答：

> 不按工具数量决定架构。默认采用外层 Plan-and-Execute、节点内 bounded ReAct。先形成带依赖、预算、权限和验收条件的计划；节点内部根据工具结果做有限观察和行动。只有当子任务存在可验证的并行、权限隔离、上下文隔离或独立评审价值时才拆成多 Agent。

选型：

- ReAct：短任务、环境不确定、下一步强依赖 observation、低副作用；
- Plan-and-Execute：长依赖、预算、审计、并行；
- Hierarchical：子任务可隔离、不同权限/Runtime、需要并行或独立验证；
- 多 Agent 不是因为“更复杂”或“更高级”。

执行链：

```text
Intent/Admission
→ Versioned Plan/DAG
→ Ready-set
→ Grant
→ Tool/Runtime execution
→ Observe/validate
→ Recovery
→ Verify
→ Evidence/Journal terminal
```

## 6. Memory / Context 面试核心答案

重要观点：

> Event Journal 不等于模型长期记忆。Journal 是状态权威；模型记忆是从事实、Evidence 和用户许可数据构造的派生上下文。

四层：

- Working Context：当前目标、计划、观察；
- Episodic Memory：Run、Attempt、Evidence、终态；
- Semantic Memory：派生索引/向量召回，不是权威；
- Procedural Memory：Skills、模板、策略。

写入长期记忆只接受：

- 用户确认的稳定偏好；
- accepted outcome；
- 可复用事实；
- 有来源、scope、时间、版本和置信度的信息。

不写：

- secret/Grant；
- hidden reasoning；
- tentative output；
- 每 token；
- 未验证推断。

压缩触发不是简单“80%”：

```text
current input + reserved output + worst-case tool output + safety margin
接近 context window 或成本/TTFT/cache 预算时触发
```

摘要污染防护：

- summary 不是 authority；
- claim-level provenance；
- source Event/Evidence IDs；
- generation/version；
- contradiction detection；
- 可回退原始事实重建；
- corrupted-summary eval。

## 7. RAG、MCP、工具容错要点

RAG 完整链：

```text
trusted source/scope
→ parsing
→ structure-aware chunk
→ BM25 + vector
→ ACL/metadata filter
→ RRF/rerank
→ context packing
→ citation
→ groundedness/task eval
```

不能宣称 Loom 已经完成生产 RAG 实验。

Function Calling 与 MCP：

- Function Calling：模型按已知 schema 选择工具和生成参数；
- MCP：Client 与 Tool/Resource Server 的发现、传输、会话和能力协议；
- MCP Server 不能成为 Loom authority；
- 执行仍需 policy、Grant、generation、validation、Evidence。

重试：

- 幂等读：可对 transient error 退避；
- 幂等写：必须 idempotency key；
- 非幂等写：先 reconcile operation 状态，不能盲重试；
- schema/semantic error：修正或 re-plan；
- auth/policy denial：不重试，进入 approval/config/human_required；
- Agent retry：新 Attempt、新 generation、新 Grant、新 Evidence。

## 8. Harness 评测体系

四层：

1. Deterministic conformance：schema、CAS、generation、idempotency、Grant、Evidence、replay；
2. Model behavior：任务成功、工具选择、参数、规划、澄清、abstention；
3. Safety：越权、stale result、prompt injection、secret、重复副作用；
4. Product journey：GUI/TUI/IPC/Journal/Projection/cleanup。

评测集字段：

```text
task
environment
initial_state
allowed_tools
forbidden_actions
expected_invariants
acceptable_outcomes
required_evidence
failure_labels
cost/latency_budget
```

当前最需要补：

- Context/Memory Evaluation Lab；
- Loom Harness Benchmark；
- Scheduler Load/Chaos Report。

## 9. 系统与调度面试要点

一天 10 万 Agent 请求平均约 1.16 req/s，但如果平均任务持续 120 秒，根据 Little's Law，平均并发约 139；10 倍峰值约 1400。Agent Infra 应按任务驻留时间和 tool fan-out，而不是 HTTP QPS 设计。

架构：

```text
Ingress/Admission
→ Durable Journal/Queue
→ Queue Projection
→ Event-driven Scheduler
→ Leased Ephemeral Workers
→ Sandbox/Tool/Model Gateway
→ Artifact/Evidence
→ Projection/API
```

分布式语义：

- at-least-once dispatch；
- idempotent CAS；
- lease + generation fencing；
- stale result rejection；
- backpressure、quota、fairness；
- 不宣称网络 exactly-once。

异构调度仍需补：

- CPU/memory/GPU/NPU vector；
- NUMA/PCIe/NVLink/RDMA topology；
- gang scheduling；
- DRF、aging、backfill、preemption；
- fragmentation 与 trace-driven simulation。

## 10. GitHub 公开审计（2026-08-04）

公开账号：<https://github.com/tttboy123>

### 10.1 主页问题

- Bio 仍是哈尔滨理工大学学生；
- Company 仍是学校；
- 无 pinned repositories；
- Popular repos 混有 Kubernetes/Karpenter/containerd 等 fork；
- 公开身份没有体现腾讯、AgentRuntime、TKE 和 Loom。

### 10.2 最强公开证据：OpenUsage PR #269

链接：<https://github.com/janekbaraniewski/openusage/pull/269>

公开事实：

- 找到 daemon 每轮重建 providers 导致 cache 丢失；
- 反复解析 Codex/Claude Code 历史 JSONL；
- 建立 long-lived source registry；
- SQLite change log；
- incremental candidate/winner projection；
- 34 个文件，4 commits；
- 174 MB SQLite、约 73k logical events；
- cold build 约 3.0s → 0.47-0.69s；
- 单事件增量更新约 1ms；
- CPU 最大值 34.6% → 7.6%；
- average CPU 约 0.44%-0.58%；
- 有 race、equivalence、pruning、invalidation 测试；
- maintainer 确认 underlying issue 仍存在且改动是 wanted；
- PR 当前 open，不能写“已合入”。

### 10.3 OpenUsage Bar

链接：<https://github.com/tttboy123/openusage-bar>

公开能力：

- SwiftUI + Python；
- SQLite、Keychain、Unix socket；
- CI、Release、Security、Contributing；
- 277 files、约 95 个 test paths；
- v0.6.0 RC；
- 外部 canary 仍为 0/5，不能宣称生产成熟。

### 10.4 Cloud Skills MCP

链接：<https://github.com/tttboy123/cloud-skills-mcp>

准确能力：

- Tencent Cloud Phase 1.5；
- Go stdio MCP Server；
- CVM list/describe/start/stop；
- mutation 双重门禁；
- credential redaction；
- input validation；
- CI/race/vet/build。

不能写“已实现六云 MCP”。

### 10.5 Loom 公开断层

公开链接：<https://github.com/tttboy123/loom>

公开主分支只显示 rebuild in progress，和本地 accepted implementation 严重脱节。公开前，简历只能写“个人项目，代码可面试展示”，不要附旧链接。

### 10.6 AgentX

有多笔 merged PR，涉及 MCP Server Manager、Team、Access Management、Go、React、Docker/Helm。但 PR 标题如 `Dev/boyce`，部分 diff 过大且包含生成/二进制文件。不能作为第一代表作，需先整理个人贡献说明。

## 11. CubeSandbox Sidecar 更正与研究

CubeSandbox：<https://github.com/TencentCloud/CubeSandbox>

公开架构：

- RustVMM + KVM；
- single-node 与 multi-node；
- E2B SDK compatible；
- public claim：fully serviceable sandbox cold start <60ms；
- public claim：per-instance memory overhead <5MB；
- 组件包括 CubeAPI、CubeMaster、Cubelet、CubeShim、VMM、network-agent、cube-proxy。

用户个人成果不能扩大为 CubeSandbox 核心 Runtime、RustVMM/KVM、60ms、5MB、snapshot cloning 或多节点调度，除非确实参与。

准确简历表述：

> 围绕 CubeSandbox 建设监控 Sidecar，为 Agent 沙箱提供运行状态、生命周期和诊断信息采集能力。

保守 bullet：

- 围绕 CubeSandbox 建设监控 Sidecar，为 Agent 沙箱提供运行状态、生命周期和诊断信息采集能力；
- 将监控能力与沙箱核心执行链路解耦，通过独立 Sidecar 承载数据采集与上报；
- 参与沙箱创建、运行、终止和清理等生命周期的可观测性建设；
- 处理沙箱身份关联、短生命周期数据完整性和监控组件故障隔离。

面试追问必须准备：

- Sidecar 在 guest、host、控制组件容器还是节点；
- per-sandbox 还是 per-node；
- 通信协议；
- 数据来源；
- Agent 能否篡改；
- backend 不可用时 fail-open/fail-closed；
- bounded buffer、backpressure、flush；
- sandbox destroy 尾部数据；
- high-cardinality sandbox_id；
- Sidecar CPU/RSS 预算；
- 残留进程/socket/file 清理；
- 区分 Sandbox、VMM 和监控故障。

## 12. 更新后的岗位匹配度

| 方向 | 匹配度 | 说明 |
|---|---:|---|
| AI Agent 运行环境/沙箱 | A+ | CubeSandbox Sidecar + TKE + Loom |
| Agent Sandbox Observability | A | 直接生产经历 |
| Agent Infra 控制面 | A- | Sidecar + Journal/CAS/Recovery |
| Kubernetes 节点/Runtime | A- | Pod 启动、升级、RainbowOS |
| Agent Harness 工程 | B+ | Loom 很强，公开证据落后 |
| 微虚拟机核心 Runtime | B-/C+ | 未证明 RustVMM/KVM core ownership |
| 异构集群调度 | B- | 缺 GPU/NPU/topology/gang |
| 训练基础设施 | C+ | 缺训练 workload/GPU telemetry |
| RDMA/NCCL | C- | 无直接证据 |
| Harness 研究员 | C+/B- | 工程强，本科/论文/eval research 风险 |

## 13. 推荐新版简历草稿

### 周勃译

**Agent Runtime / Kubernetes 基础设施工程师**

电话、邮箱沿用原简历；GitHub：<https://github.com/tttboy123>；现居地待确认。

### 个人概述

腾讯 AgentRuntime/TKE 后端工程师，具备 CubeSandbox 监控 Sidecar、Kubernetes 节点生命周期、Pod 启动性能、节点原地升级和 TencentOS Rainbow 接入经验。主要使用 Go，熟悉 Linux、容器、Kubernetes 和分布式系统。独立设计 Loom Agent Harness，覆盖 Runtime Adapter、Team DAG 调度、generation fencing、Grant、Evidence、故障恢复和多客户端一致性；具备真实 daemon 性能诊断与增量 read model 优化经验。

### 腾讯科技｜AgentRuntime 后端开发工程师

`[正式入职时间] – 至今`

- 负责围绕 CubeSandbox 的监控 Sidecar 设计与开发，为 Agent 沙箱提供运行状态、生命周期和诊断信息采集能力；
- 通过独立 Sidecar 解耦监控采集与 Sandbox Runtime 核心执行链路，降低监控故障对 Agent workload 的影响；
- 基于 `[实际通信方式]` 采集 `[实际指标]`，以 `[sandbox_id/node_id/tenant_id 中实际字段]` 关联生命周期与监控数据；
- 针对短生命周期和高密度场景，实现 `[buffer/batch/backpressure/retry 中实际机制]`，将 Sidecar 开销控制在 `[真实数据]`；
- 解决 `[最典型故障]`，将 `[定位时间/数据完整性/告警延迟]` 从 `[X]` 优化到 `[Y]`。

### 腾讯科技｜TKE 节点方向后端开发工程师

`[开始时间] – [转组时间]`

- 负责 TKE 节点侧研发，参与 Pod 启动速度观测与优化、节点原地升级和 TencentOS Rainbow 接入；
- 建设 Pod 启动阶段化观测能力，围绕 `[真实阶段]` 定位长尾，并通过 `[方案]` 将 `[p95/p99]` 优化 `[数据]`；
- 设计/参与节点原地升级状态机，覆盖预检、迁移、升级、恢复和终态，保证幂等、可重入和故障可恢复；
- 升级前对节点与 Pod 进行预检和风险评分，联合腾挪组件迁移业务；
- 参与 Rainbow 接入，负责 `[真实 owned boundary]` 的适配、验证和故障处理；
- 建立升级监控和告警，支持 Pod 异常、Node NotReady 和升级卡滞处理。

### 腾讯科技｜TKE 后端开发实习生

2023.11-2024.07

- 建设公有云节点信息统计能力，采集 runtime、Kernel、Kubelet、Architecture 和镜像数据；
- 设计数据对账与修正逻辑；
- 使用 pprof 将相关组件内存占用降低约 20%；
- 参与节点升级的状态机、预检、Pod 迁移和告警。

### OpenUsage daemon 增量 Read Model

- 诊断 provider 周期重建导致 cache 丢失和历史 JSONL 重复解析；
- 实现 long-lived source registry、SQLite change log 和 incremental winner projection；
- 73k events 上 cold build 约 3.0s → 0.47-0.69s，单事件约 1ms，CPU max 34.6% → 7.6%；
- 提交 upstream PR #269；maintainer 已确认 issue 与方案价值，当前仍 open。

### Loom｜Local-first Agent Harness

- 将 Agent、Runtime、Team、Run、Attempt、Grant、Evidence 分离建模；
- 实现 deterministic Team DAG、capacity ready-set 和 stream-head CAS；
- 每 Attempt 独立 generation/Grant/Evidence，拒绝 stale result；
- 实现 append-only Journal、idempotency、Artifact/Evidence 和 GlobalReadView；
- Runtime output 经过 binding/sequence/generation/Grant 校验后才可观察；
- macOS GUI、TUI、CLI 共用 daemon IPC/application service；
- RED、race、Reviewer、controlled canary 验证。

### OpenUsage Bar

- SwiftUI + Python 的 local-first AI usage monitor；
- Keychain、SQLite、Unix socket；
- CI、Release、Security；
- 当前 RC，不宣称生产成熟。

### Raft 分片 KV

- Leader election、log replication、persistence、snapshot/log compaction；
- Shard config、Join/Leave/Move/Query；
- 分片迁移期间避免阻塞其他分片读写。

### 技术能力

- Go、C/C++、Python；
- Agent Runtime、CubeSandbox Sidecar、Runtime Adapter、DAG、generation fencing、Grant/Evidence、MCP；
- Kubernetes、Pod/Node lifecycle、Kubelet/CRI 基础、Docker、containerd 基础、cgroup、namespace、Helm；
- Linux、状态机、幂等、CAS、故障恢复、可观测性、pprof、增量 projection；
- Raft、日志复制、分片迁移、Event Journal；
- TCP/IP、HTTP/HTTPS、QUIC、RTP/RTSP；不写熟悉 RDMA/NCCL。

### 教育

哈尔滨理工大学｜计算机科学与技术｜本科｜2021.09-2025.06  
课程实践：CMU 15-213、MIT 6.S081、Stanford CS144。

## 14. GitHub 投递前整改

1. Profile bio 改为 `Agent Runtime / Kubernetes Infrastructure Engineer | Sandbox Observability | Agent Harness | Go`；
2. Company 是否写 Tencent 需遵守公司披露规则；
3. 增加 Profile README；
4. Pin openusage-bar、cloud-skills-mcp、整理后的 Loom、新 sandbox/scheduler 项目；
5. 不 pin Kubernetes/Karpenter/containerd 纯 fork；
6. 发布无内部信息、无 secret、可独立构建的 Loom snapshot；
7. 统一提交身份，避免 Boyce/tttboy123/lune/coder/Codex 混杂；
8. 透明说明 AI-assisted workflow，但强调 owner-defined architecture/acceptance/human review；
9. 推动 OpenUsage PR #269 完成 review；
10. 整理 AgentX 个人贡献说明。

## 15. 新会话建议起始提示词

```text
请读取：
/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild/.loom-drafts/interview-agent-harness-infra-full-handoff-2026-08-04.md

这是我上一会话的完整求职研究交接。请不要重复基础调研，从文件的“当前仍缺少的内部经历细节”开始，先帮助我补齐 CubeSandbox 监控 Sidecar、Pod 启动优化、节点原地升级和 RainbowOS 接入的真实数据，再分别生成：
1. Agent Runtime/沙箱岗位一页中文简历；
2. Agent Harness 岗位一页中文简历；
3. 集群调度/训练基础设施岗位一页中文简历；
4. 每份简历对应的 90 秒自我介绍和追问题库；
5. GitHub Profile README 与公开 Loom 发布整改计划。

所有结论必须区分：公开可验证、本人陈述、需要补证；不得编造腾讯内部规模、指标或 ownership。
```

