# 近 3 个月爆火 Memory 项目深度分析 (2026-05~08)

> **作者**: Mavis · **日期**: 2026-08-04 09:10 SGT
> **窗口**: 2026-05-01 ~ 2026-08-04 (近 3 个月)
> **承接**: `.loom-drafts/loom-memory-capability-classification-2026-08-04.md` (19 维度) + `loom-memory-usage-guide-2026-08-04.md` (19 实战手册) + `loom-memory-latest-2026-08-04.md` (5 大趋势)
> **目的**: 选 7-8 个真实"爆火"(月增 stars>1K 或 行业刷屏) 的 Memory 项目深挖,看 Loom 能借鉴什么/不吸什么
> **Loom 立场**: 跟 D3 必决 (拒代理 LLM / 拒 SaaS) 不冲突的才展开, 冲突的标"不吸"

---

## 0. TL;DR

| 排名 | 项目 | 爆火数据 | 核心创新 | Loom 立场 |
|----|------|---------|---------|----------|
| 1 | **claude-mem** | 89.4K⭐, 日增 ~150 | 跨 7 个 Agent (Claude/Codex/Gemini/Hermes/Copilot/OpenClaw/OpenCode) 统一持久化层 | ✅ **核心借鉴**: 通用 memory 协议层, Loom 可走 S1 兼容 |
| 2 | **MemPalace** | 58.0K⭐, 日增 855 | wing/room/drawer 三层结构 + 29 MCP 工具 + 96.6% R@5 召回 + **无 API key** | ✅ **吸架构**: 三层目录 + 纯本地 + SK-2 兼容层, 跟 SK-1/SK-2 一起落 |
| 3 | **Cognee** | 29.7K⭐, 持续增长 | Knowledge Graph + Vector + Graph-RAG 混合检索, ECL 2026 论文 | ⚠️ **借鉴 Graph-RAG**: 跟 TK-1 时序 KG 自研结合, 不吸 LLM-decision 部分 |
| 4 | **Mem0** | ~70K⭐ (4.8万→7万 H1), $24M 融资 | 4 月新算法 LoCoMo 92.5 / LongMemEval 94.4 (SOTA) | ⚠️ **借鉴指标 + Entity Linking**: 不吸 LLM 决策事实提取 |
| 5 | **TencentDB Agent Memory** | 9K⭐ (2026-05 开源) | L0/L1/L2/L3 四层渐进式架构, 跟 OpenClaw/Kimi 集成, PersonaMem 76.10% | ⚠️ **借鉴四层架构**: 不吸 SaaS 部分 (D3 必决拒) |
| 6 | **oracle-memory-by-yhw** | 持续维护 (DB-native) | Oracle AI Database 26ai 三位一体 (标量+向量+属性图) + JRD 视图层 | ✅ **PG-11 借鉴**: 跟 P0-11 DBOS Postgres 路线一致, 三位一体 + 视图层 |
| 7 | **memvid** | 16.1K⭐, v2.0 (v1 QR deprecated) | Serverless single-file memory layer, 视频编码 + 帧索引 | ⚠️ **不吸架构 (over-engineered)**: 借鉴"单文件便携"思想给 Loom Agent 备份用 |
| 8 | **Hermes Agent 三层记忆** | 6.2万⭐ (NousResearch) | 短期+长期+技能三型记忆, SQLite + FTS5, 自学习闭环 | ✅ **吸三层架构**: 跟 S1-W2 Event Journal 复用 |

**Loom 落地 4 抓手 (本轮新增)**:

1. **SK-3** (Phase 1 Slice 2 W5-W6, 1-2 天): 兼容 claude-mem 的 7-Agent 协议, 让 Loom Skill 跟 Claude Code/Codex/Gemini 等互通
2. **ME-2** (Phase 1 Slice 3, 3-5 天): 借鉴 MemPalace wing/room/drawer 三层 + Mem0 的 entity linking + 混合检索 (BM25+vector+entity)
3. **TK-1** (Phase 2, 2-3 周): 时序 KG 自研 + 借鉴 Cognee Graph-RAG 双层检索 (entity 检索 + chunk 检索)
4. **ME-3** (Phase 2, 1 周): 借鉴 TencentDB 四层架构 (L0 原文/L1 事实/L2 场景/L3 画像) + 借 Hermes 三层记忆分类

---

## 1. 调研方法

**窗口**: 2026-05-01 ~ 2026-08-04 (近 3 个月)
**入选标准**:
- 月增 stars > 1K OR
- 行业刷屏 (HN 首页 / 头部 KOL 推荐 / 行业大会演讲) OR
- 大厂新开源 (TencentDB / Google Agent Memory)

**数据源**:
- GitHub Trending 日榜 / 周榜 / 月榜 (2026-05~08)
- Hacker News Top 100 (每日)
- 阿里云 / 腾讯云 / 华为云大模型发布 (5-8 月)
- CSDN GitHub 开源项目日报 (8.04 累计)
- arXiv cs.AI (近 3 个月)
- Anthropic / OpenAI 官方博客

**"爆火" 定义** (本报告):
- 极强: claude-mem (89K⭐ 行业第一, 多 Agent 跨平台)
- 强: MemPalace (58K⭐, 本地优先代表) / Mem0 (4.8万→7万 + 融资)
- 中: Cognee (29.7K⭐, 持续) / Hermes Agent (6万, 跟 Skills 生态绑定) / memvid (16K)
- 新: TencentDB Agent Memory (9K⭐, 2026-05 开源) / oracle-memory-by-yhw (DB-native)

---

## 2. claude-mem ⭐ 89.4K (超级爆火)

### 2.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | Alex Newman (@thedotmack) |
| **创建** | 156 天前 (~2026-02) |
| **Stars** | 89,400 (日增 ~150, 趋势持续) |
| **Forks** | 7,800 |
| **Watchers** | 281 |
| **License** | Apache-2.0 |
| **语言** | TypeScript |
| **GitHub** | github.com/thedotmack/claude-mem |
| **官网** | claude-mem.ai |
| **技术栈** | ChromaDB + SQLite + Bun + Claude Agent SDK |

### 2.2 核心创新 (做什么)

**1. 跨 7 个 Agent 的统一持久化层**:
- Claude Code / OpenClaw / Codex / Gemini / Hermes / Copilot / OpenCode
- 同一个 memory 协议, 7 个客户端都能用
- "Persistent Context Across Sessions for Every Agent" 是核心 Slogan

**2. 自动捕获 + AI 压缩 + 上下文注入**:
- 用 Claude Agent SDK 在工具调用前后自动捕获
- LLM 压缩 (用 Claude 做摘要 / 关键信息提取)
- 后续会话自动注入相关上下文

**3. 4 个 MCP 工具 + 3 层工作流**:
- `mcp__claude-mem__search` — 跨会话搜索
- `mcp__claude-mem__timeline` — 时序视图
- `mcp__claude-mem__get_observations` — 关键观察回放
- `mcp__claude-mem__*` — 自定义工具扩展
- 3 层: capture → compress → inject

**4. Endless Mode + Beta Channel**:
- "Endless Mode" 持续压缩, 突破 context window 限制
- beta channel 灰度发布新功能

### 2.3 效果 (数据)

- **实际效果**: 89.4K stars, 跨 7 大 agent 平台 (不是单一 Claude Code)
- **平均日增 stars**: ~150 (在 156 天里保持稳定)
- **forks 转化率**: 7,800 / 89,400 = 8.7% (说明确实有人用, 不是纯 star 收藏)
- **Watchers 281**: 实际关注率 0.3%, 比较健康

### 2.4 怎么用

```bash
# Claude Code 用户: 一行安装
npx claude-mem

# OpenClaw 用户: 类似
openclaw plugins install claude-mem

# 配置环境变量 (用 Claude Agent SDK 压缩)
ANTHROPIC_API_KEY=sk-ant-... # 必填
```

### 2.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装难度 | 🟢 易 | 一行 npm 命令 |
| 配置成本 | 🟡 中 | 必须有 Anthropic API key (压缩用) |
| 跨平台支持 | 🟢 优 | 7 大 agent 都支持 |
| 文档 | 🟢 优 | claude-mem.ai + GitHub docs/ 完整 |
| License | 🟢 Apache-2.0 | 友好 |
| D3 兼容性 | 🟡 中 | 用了 Claude 做压缩 (代理 LLM), 但 Loom 可仅借鉴协议层 |

### 2.6 Loom 落地路径

**核心借鉴**: 跨 Agent 协议层 + MCP 工具接口

**SK-3: 兼容 claude-mem 协议** (Phase 1 Slice 2 W5-W6, 1-2 天)

```go
// internal/mem/claudemem_compat.go
// 兼容 claude-mem 的 4 个 MCP 工具, 让 Loom Agent 跟 Claude Code/Codex 等互通

type ClaudeMemCompat struct {
    journal *journal.Journal  // Loom S1-W2 Event Journal
    vector  *vector.Store     // pgvector
}

func (c *ClaudeMemCompat) Search(ctx context.Context, query string) ([]Observation, error) {
    // 1. 用 Loom 自己的 retrieval (BM25+vector+entity)
    chunks := c.vector.HybridSearch(query, 10)
    // 2. 转换格式: claude-mem 期望 {type, content, timestamp, session_id}
    return convertToClaudeMemFormat(chunks), nil
}
```

**兼容性测试**:
- 装 Claude Code + claude-mem
- 配 Loom 作为 backend (替代默认 ChromaDB)
- 验证 4 个 MCP 工具 (search/timeline/get_observations) 都能用

**额外兼容 agent**:
- Codex: codex CLI 已经有 `AGENTS.md` 集成, 走 Loom S1-W5 已有路径
- Gemini CLI: 走 `GEMINI.md` 路径, 跟 AGENTS.md 同构
- Hermes: 已经有三层记忆, 协议可对齐

---

## 3. MemPalace ⭐ 58.0K (本地优先代表)

### 3.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | MemPalace Team |
| **Stars** | 58,000 (日增 855, 持续爆) |
| **Forks** | 7,500 |
| **Watchers** | 327 |
| **License** | MIT |
| **语言** | Python |
| **GitHub** | github.com/MemPalace/mempalace |
| **官网** | mempalaceofficial.com |
| **技术栈** | ChromaDB + embeddinggemma-300m (默认, 100+ 语言) + Python 3.9+ |
| **Topics** | ai / chromadb / llm / mcp / memory / python |

### 3.2 核心创新 (做什么)

**1. wing / room / drawer 三层目录结构** (核心!):
- **wing**: 项目级 (一个 wing = 一个项目)
- **room**: 会话级 (一个 room = 一次完整对话)
- **drawer**: 消息级 (一个 drawer = 一条 user/assistant message, 原文保存)
- 类似文件系统的层级, 直观且可移植

**2. 29 个 MCP 工具深度集成 Claude Code**:
- 跟 claude-mem 一样走 MCP, 但工具集更大
- 核心 4 个: `search_conversations` / `get_observation` / `timeline` / `mine`
- 配套 25 个辅助: `list_wings` / `open_drawer` / `sweep` / `backfill` / etc.

**3. 96.6% R@5 召回率** (公开 benchmark, 自称 "the best-benchmarked"):
- 在 LoCoMo / LongMemEval 等标准测试集上
- 用 embeddinggemma-300m (Google EmbeddingGemma, 100+ 语言, ~300MB)
- 默认 ChromaDB 后端, 可换

**4. 无需 API key**:
- 跟 Mem0/Letta/Cognee 不同, 不强制 LLM 决策
- 完全本地跑, ChromaDB + 本地 embedding
- 跟 D3 必决高度兼容!

### 3.3 效果 (数据)

- **58K stars, 日增 855**: 在 3 个月 window 里持续高增长
- **R@5 96.6%**: 自称 benchmark 第一 (跟 Mem0 的 LoCoMo 92.5 / LongMemEval 94.4 比略高)
- **29 个 MCP 工具**: 是 claude-mem (4 个) 的 7 倍
- **0 依赖 API key**: Loom D3 必决最强兼容

### 3.4 怎么用

```bash
# 装
pip install mempalace
# 或 onboarding
python -m mempalace.onboarding

# Claude Code hooks 配置 (自动 capture)
# ~/.claude/settings.json:
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "*",
      "hooks": [{"type": "command", "command": "mempalace hook"}]
    }]
  }
}

# 一次性 backfill 旧 sessions
mempalace mine ~/.claude/projects/ --mode convos
mempalace sweep <transcript-dir>  # per-message
```

### 3.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装难度 | 🟢 易 | pip install 即用 |
| 配置成本 | 🟢 **零** | 无 API key, 0 额外配置 |
| 跨平台支持 | 🟢 Claude Code 主, 其他逐步 | 跟 Loom Phase 1 一致 |
| 文档 | 🟢 优 | mempalaceofficial.com 全套 |
| License | 🟢 MIT | 最友好 |
| D3 兼容性 | 🟢 **满** | 无 LLM 决策, 全部本地 |

### 3.6 Loom 落地路径

**核心借鉴**: wing/room/drawer 三层 + 0 API key 纯本地 + 29 个 MCP 工具

**ME-2: 借鉴三层结构 + 混合检索** (Phase 1 Slice 3, 3-5 天)

```go
// internal/mem/store.go
// 借鉴 MemPalace 三层, 改造为 Loom schema

type MemoryItem struct {
    ID         string
    Wing       string  // 项目 / workspace
    Room       string  // session_id
    Drawer     string  // message_id
    Type       string  // fact / conversation / preference
    Content    string
    Embedding  []float32  // BGE-M3 1024 维
    Source     string  // claude-code / codex / loom-native
    CreatedAt  time.Time
    LastAccess time.Time
    AccessCount int
    Importance  float64
}

// 跟 Loom S1-W2 Event Journal 集成
func (s *Store) AppendToJournal(item MemoryItem) error {
    return s.journal.Append(Event{
        Type: "memory.write",
        Data: item,
        Timestamp: item.CreatedAt,
    })
}
```

**Hybrid Retrieval (BM25 + vector + entity)**:
```go
func (s *Store) HybridRetrieve(query string, topK int) []MemoryItem {
    // 1. Vector 检索 (pgvector HNSW)
    vectorHits := s.vector.HNSW(query, topK*2)
    // 2. BM25 全文检索 (Postgres FTS)
    bm25Hits := s.bm25.FTS(query, topK*2)
    // 3. Entity 检索 (jieba NER + 已知实体库)
    entities := jieba.Extract(query)
    entityHits := s.entity.FindRelated(entities, topK*2)
    // 4. RRF 融合
    return rrf.Merge(vectorHits, bm25Hits, entityHits, topK)
}
```

**SK-2 兼容层扩展** (跟 SK-1/SK-2 一起):
- 兼容 MemPalace 的 29 个 MCP 工具 (子集核心 4 个)
- 落 `internal/skills/mempalace_compat/`

**额外价值**:
- **3 个 Loom 不可吸点**:
  1. "最好 benchmark" 自称 — 没看到独立第三方验证, 仅作者声明
  2. 强依赖 ChromaDB — Loom 已定 pgvector, 不切
  3. 强依赖 Claude Code 生态 — Loom 是 runtime, 不是 Claude Code 插件

---

## 4. Cognee ⭐ 29.7K (Knowledge Graph Memory)

### 4.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | topoteretes (Markovic 等) |
| **Stars** | 29,700 |
| **Forks** | 2,900 |
| **Watchers** | 102 |
| **License** | Apache-2.0 |
| **语言** | Python |
| **GitHub** | github.com/topoteretes/cognee |
| **官网** | cognee.ai |
| **技术栈** | Python + Graph Database + Vector + LLM (默认) |
| **Topics** | agent-memory / knowledge-graph / graph-rag / vector-database / memory-management |

### 4.2 核心创新 (做什么)

**1. Knowledge Graph + Vector + Graph-RAG 混合**:
- 不是单一存储, 是 Graph DB (NetworkX / Neo4j) + Vector DB (LanceDB) 双层
- Graph-RAG 思路: 先走实体关系检索, 再走 chunk 检索
- 跟 LightRAG 思路一致, 比纯 vector 多跳推理强

**2. LLM 决策的事实提取** (D3 冲突):
- 默认用 LLM 从文档提取 entity / edge / episode
- 支持 OpenAI / Anthropic / Ollama / 自定义 LLM
- 增量更新 KG, 不重建

**3. Cognitive Architecture 范式**:
- 模仿人类记忆分层: sensory / short-term / long-term
- 跟 Phase 1 S1-W2 分类 (Episodic/Semantic/Procedural) 高度一致

**4. 学术研究背景**:
- arXiv 2505.24478 "Optimizing the Interface Between Knowledge Graphs and LLMs for Complex Reasoning"
- 持续发表 KG + LLM 优化论文

### 4.3 效果 (数据)

- **29.7K stars, 持续增长**: 在 GitHub Trending 经常出现
- **102 watchers**: 健康关注率
- **2.9K forks**: 8.7% fork 转化率 (跟 claude-mem 一样, 真用)

### 4.4 怎么用

```bash
pip install cognee
# 或用 LLM 跑
cognee.add("Alice works at Google since 2020")  # LLM 提取
cognee.cognify()  # 构建 KG
cognee.search("Where does Alice work?")  # 检索
```

### 4.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装难度 | 🟡 中 | pip install + LLM key 配置 |
| 配置成本 | 🟡 需 LLM key | 强制依赖 LLM (默认 OpenAI) |
| 跨平台支持 | 🟡 Python only | Loom Go 路线不友好 |
| License | 🟢 Apache-2.0 | 友好 |
| D3 兼容性 | 🟡 **部分冲突** | 默认 LLM 决策, 可配置但复杂 |

### 4.6 Loom 落地路径

**核心借鉴**: Graph-RAG 双层检索算法 (entity + chunk)

**借鉴但不全吸**:
- ✅ 借: 双层检索 (entity layer + chunk layer)
- ❌ 不吸: 默认 LLM 决策 (D3 拒 LLM-decision)
- ❌ 不吸: Python-only (Loom Go)
- ❌ 不吸: 完整框架 (Loom 走自研, 借算法思想)

**TK-1 + Graph-RAG 增强** (Phase 2, 2-3 周):

```go
// internal/kg/retrieval.go
// 借鉴 Cognee Graph-RAG, 自研无 LLM 决策版本

func (kg *KG) DualLayerRetrieve(query string, topK int) []Chunk {
    // 1. Entity layer: 提取查询中的实体 (jieba NER)
    entities := jieba.NER(query)
    // 2. 在 KG 中找相关实体 (向量相似度 + 关系遍历)
    relatedEntities := kg.FindSimilarEntities(entities, topK*2)
    // 3. 通过 KG 边, 找到相关 chunks
    relatedChunks := kg.FindChunksViaEdges(relatedEntities, topK*2)
    // 4. Chunk layer: 传统向量检索
    chunkHits := kg.vector.HNSW(query, topK*2)
    // 5. RRF 融合 + 重排
    return rrf.Merge(relatedChunks, chunkHits, topK)
}
```

---

## 5. Mem0 ~7万⭐ (4 月新算法 SOTA)

### 5.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | mem0ai |
| **Stars** | ~7万 (H1 从 4.8 万增长) |
| **融资** | $24M (2026 H1) |
| **License** | Apache-2.0 |
| **GitHub** | github.com/mem0ai/mem0 |
| **技术栈** | Python + Qdrant/pgvector + LLM (OpenAI gpt-5-mini) |
| **论文** | arXiv 2504.19413 (Chhikara et al.) |

### 5.2 核心创新 (4 月新算法)

**1. Single-pass ADD-only extraction**:
- 一次 LLM 调用, 只 ADD, 不 UPDATE/DELETE
- 记忆累积, 不覆盖
- 解决 Mem0 旧版"频繁覆盖"问题

**2. Agent-generated facts first-class**:
- 当 agent 确认动作, 信息同等权重存储
- 不再 LLM-only 决定重要度

**3. Entity linking** (关键!):
- 提取实体, embedding, 在 memories 间链接
- 检索时 entity 命中 boost

**4. Multi-signal retrieval** (3 路并行):
- semantic (向量)
- BM25 (关键词)
- entity matching (实体)
- 并行打分, 融合

**5. Temporal Reasoning**:
- 时间感知, 排序"当前状态 / 过去事件 / 未来计划"
- 跟 TK-1 时序 KG 思路对齐

### 5.3 效果 (公开 benchmark 数据)

| 基准 | 旧算法 | 新算法 (4 月) | 提升 |
|------|--------|---------------|------|
| **LoCoMo** | 71.4 | **92.5** | +21.1 |
| **LongMemEval** | 67.8 | **94.4** | +26.6 |
| **BEAM (1M)** | — | **64.1** | 新基准 |
| **BEAM (10M)** | — | **48.6** | 新基准 |
| **Token/latency** | — | 7.0K / 0.88s | 比 1M 增 5x |

**关键数据**:
- 94.4 LongMemEval = 接近满分
- 6.7K tokens (每次 add 极低)
- 0.88s p50 (生产可用)

### 5.4 怎么用

```python
pip install mem0ai
# 或 npm
npm install mem0ai
```

```python
from mem0 import Memory
m = Memory()
m.add("我们团队用 FastAPI + PostgreSQL", user_id="alice")
results = m.search("技术栈", user_id="alice")
```

### 5.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装 | 🟢 易 | pip/npm install |
| 配置 | 🟡 需 LLM key | 强制 OpenAI gpt-5-mini |
| License | 🟢 Apache-2.0 | 友好 |
| D3 兼容性 | 🟡 **部分冲突** | 强制 LLM-decision, 但 open-source SDK 可拆 |

### 5.6 Loom 落地路径

**核心借鉴**: 4 个算法创新 (Entity Linking / Multi-signal / Temporal / Single-pass)

**借鉴但不全吸**:
- ✅ 借: **Entity linking 思路** (重要! 跟 ME-2 hybrid retrieval 完美契合)
- ✅ 借: **Multi-signal 3 路融合** (vector+BM25+entity 跟 MemPalace 同思路)
- ✅ 借: **Temporal reasoning** (跟 TK-1 时序 KG 对齐)
- ❌ 不吸: 完整 SDK (Go 路线不友好)
- ❌ 不吸: 强制 LLM (D3 拒)
- ❌ 不吸: 商业 SaaS 模式 (D3 拒 SaaS)

**ME-2 集成** (跟 MemPalace 一起):
- 把 Mem0 的 **entity linking** 算法思路落地到 Loom `internal/mem/entity.go`
- 把 **temporal reasoning** 思路落地到 TK-1 (时序 KG 自带)

---

## 6. TencentDB Agent Memory ⭐ 9K (2026-05 新开源)

### 6.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | Tencent Cloud (腾讯云数据库团队) |
| **开源** | 2026-05-14 |
| **Stars** | 9,000+ (3 个月增长) |
| **License** | Apache-2.0 (推测, 仓库标准) |
| **GitHub** | github.com/TencentCloud/TencentDB-Agent-Memory |
| **技术栈** | TencentDB + VectorDB + 4 层架构 |
| **集成** | OpenClaw / Lighthouse / ClawPro / Kimi-K2.5 |

### 6.2 核心创新: L0/L1/L2/L3 四层渐进式架构

| 层级 | 名称 | 内容 | Loom 借鉴度 |
|------|------|------|-----------|
| **L0** | 原始对话全量保存 | 完整对话原文, 不丢细节 | ✅ 跟 S1-W2 Event Journal 已有 |
| **L1** | 原子记忆自动提取 | 事实与约束, 可检索 | ⚠️ 借架构, 不吸 LLM 提取 |
| **L2** | 场景分块按项目聚类 | 按项目 / 场景分组 | ✅ 跟 MemPalace wing 一致 |
| **L3** | 用户画像 | 个性化认知 | ✅ 跟 Hermes 三层记忆顶层一致 |

**核心逻辑**: "碎片化 → 结构化 → 场景化 → 个性化"

### 6.3 效果 (公开数据)

- **PersonaMem 评测**: 76.10% 总体准确率, 较原生 OpenClaw 提升 59%
- **用户事实召回**: 从 < 30% 提升到 79%+
- **评测集**: 20 个独立画像 / 6462 条上下文 / 589 道高难推理题

### 6.4 怎么用

```bash
# OpenClaw 一键集成
openclaw plugins install @tencentdb-agent-memory/memory-tencentdb

# 控制台开启 (云版)
# Lighthouse → 打开 OpenClaw 实例 → 应用管理 → 记忆管理 → 启用 Agent Memory
```

### 6.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装 | 🟢 易 (云) / 🟡 中 (本地) | 本地需手动配 |
| 配置 | 🟡 需腾讯云账号 (云版) | 强绑 SaaS |
| D3 兼容性 | 🔴 **冲突** (云版) / 🟢 **可借鉴 (架构)** | SaaS 拒, 但四层架构值得自研 |

### 6.6 Loom 落地路径

**核心借鉴**: L0/L1/L2/L3 四层架构 (无 SaaS 依赖)

**ME-3: 自研四层 + 借 Hermes 三层分类** (Phase 2, 1 周)

```go
// internal/mem/4layers.go
// 借鉴 TencentDB 四层, 自研本地版

type MemoryLayer int

const (
    L0_Raw MemoryLayer = iota  // 原始对话全量 (跟 Event Journal 重合)
    L1_Fact                     // 原子事实 (jieba NER + 规则提取, 无 LLM)
    L2_Scene                    // 场景分块 (按 project_id / workspace_id 聚类)
    L3_Profile                  // 用户画像 (聚合 L1/L2, 重要性评分)
)

// 跟 Hermes 三层记忆对齐
//   Hermes 短期 (L0) + 长期 (L2) + 技能 (L3)
//   Loom   Event Journal + L1 + L2/L3
```

**为什么 Loom 不需要 L1 用 LLM 提取**:
- 规则 NER (jieba) 准确率 70-80%, 够用
- 关键信息 (技术栈/项目背景) 通过用户显式录入更准
- 避免 LLM 决策带来的 secrets 风险

---

## 7. oracle-memory-by-yhw (DB-native 代表)

### 7.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | Haiwen-Yin (胖头鱼的鱼缸) |
| **版本** | v0.5.1 (2026-07) |
| **License** | (待查, 通常 Oracle 自家协议) |
| **GitHub** | github.com/Haiwen-Yin/oracle-memory-by-yhw |
| **技术栈** | Oracle AI Database 26ai (原生 VECTOR + Property Graph + JSON) |
| **配套** | memory-pg18-by-yhw / memory-tidb8-by-yhw / memory-ob4-by-yhw |

### 7.2 核心创新: Oracle 26ai 三位一体

**1. 标量 + 向量 + 属性图 三位一体**:
- 一个 DB 存标量 (关系数据) + 向量 (embedding) + 属性图 (KG 关系)
- 不需要外挂 3 个组件
- 跟 P0-11 DBOS Postgres 路线高度一致 (但 Loom 走 PG, 思路同)

**2. JRD 视图层 (JSON Relational Duality)**:
- 通过视图提供 JSON 接口 (`memory_nodes_jdv`, `memory_graph_v`)
- 底层结构化, 上层 JSON 调用
- 适合 AI Agent 直接消费

**3. Memory Fusion Engine**:
- 语义相似检测 (`VECTOR_DISTANCE`)
- 智能合并策略 (保留最新 / 保留完整)
- 内容补全
- 操作全链路溯源

**4. 多 Agent 权限管理**:
- 禁用 Agent 自动降级
- 待恢复标识
- 增强清理框架 (每日镜像 + 每周全周期)

### 7.3 效果 (数据)

- **v0.5.1 已具备生产落地条件** (作者声明)
- 跨 4 个数据库 (Oracle / PG 18 / TiDB 8 / OceanBase 4) 同源

### 7.4 怎么用

```sql
-- Oracle 26ai 原生 SQL/PGQ
CREATE TABLE memory_nodes (
    id BIGINT,
    name VARCHAR(4000),
    type VARCHAR(100),
    embedding VECTOR(1024, FLOAT32)
);
CREATE TABLE memory_edges (
    subject_id BIGINT, object_id BIGINT,
    relation VARCHAR(200),
    valid_from TIMESTAMP, valid_to TIMESTAMP
);
-- 用 SQL/PGQ 做图遍历
SELECT * FROM MEMORY_PROPERTY_GRAPH ...
```

### 7.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装 | 🟡 中 (Oracle 26ai 需装) | 重型 DB |
| License | 🟡 需确认 | Oracle 协议一般限制商用 |
| D3 兼容性 | 🟢 优 (DB-native) | 跟 Loom P0-11 路线完全一致 |
| Loom 借鉴度 | 🟢 **极强** | 三位一体 + JRD + Fusion 全部可借鉴 |

### 7.6 Loom 落地路径

**核心借鉴**: 整个架构 (DB-native 三位一体)

**P0-11 DBOS Postgres 借鉴清单** (整合进 Loom S2-W3 计划):

```sql
-- Loom internal/mem/schema.sql (跟 P0-11 DBOS 复用)
CREATE TABLE memory_nodes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,  -- 'person' / 'org' / 'project' / 'concept'
    embedding vector(1024),  -- BGE-M3
    metadata JSONB,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    access_count INT NOT NULL DEFAULT 0,
    importance REAL NOT NULL DEFAULT 0.5,
    source_event_id BIGINT REFERENCES journal_events(id)  -- 跟 S1-W2 Event Journal 关联
);

CREATE TABLE memory_edges (
    id BIGSERIAL PRIMARY KEY,
    subject_id BIGINT REFERENCES memory_nodes(id),
    object_id BIGINT REFERENCES memory_nodes(id),
    relation_type TEXT NOT NULL,  -- 'WORKS_AT' / 'CREATED' / 'PART_OF'
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_to TIMESTAMPTZ,  -- NULL = 当前有效 (跟 Graphiti 同)
    confidence REAL NOT NULL DEFAULT 1.0,
    source_event_id BIGINT REFERENCES journal_events(id)
);

-- 索引
CREATE INDEX memory_nodes_embedding_idx ON memory_nodes 
    USING hnsw (embedding vector_cosine_ops);
CREATE INDEX memory_edges_valid_idx ON memory_edges 
    (subject_id, relation_type, valid_from, valid_to);
```

**借 Fusion Engine** (Phase 2, 跟 TK-1 一起):
- `internal/mem/fusion.go` — 语义相似检测 + 智能合并
- 操作溯源写到 Event Journal (跟 S1-W2 已有)

**为什么不直接装 oracle-memory-by-yhw**:
- Loom 走 P0-11 Postgres, 不是 Oracle 26ai
- 但思路 (三位一体 + 视图层) 完全可借鉴
- 跟 TK-1 时序 KG 是同一方向

---

## 8. memvid ⭐ 16.1K (Serverless 单文件)

### 8.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | memvid |
| **Stars** | 16,100 |
| **Forks** | 1,400 |
| **License** | Apache-2.0 |
| **GitHub** | github.com/memvid/memvid |
| **技术栈** | Python + FAISS + OpenCV + 视频编码 |
| **官网** | memvid.com |

### 8.2 核心创新 (做什么)

**1. Serverless, single-file memory layer**:
- 把所有 memory 编码到**单个视频文件** (.mp4)
- 帧索引代替向量索引
- 一个文件 = 一个完整 memory store

**2. v1 → v2 重构** (重要信号!):
- v1: QR-code 编码 (奇怪, 已被 deprecated)
- v2: 视频帧编码 (生产可用)

**3. RAG 替代方案**:
- "Replace complex RAG pipelines with a serverless, single-file memory layer"
- 给 instant retrieval + long-term memory

### 8.3 效果 (数据)

- 16.1K stars, 1.4K forks (转化率 8.7%)
- v1 deprecated 说明项目还在快速迭代
- **Benchmark 暂未独立验证**

### 8.4 怎么用

```bash
pip install memvid
memvid encode data.txt memory.mp4
memvid search "query" memory.mp4
```

### 8.5 开箱度

| 维度 | 评级 | 说明 |
|------|------|------|
| 安装 | 🟢 易 | pip install |
| 配置 | 🟢 零 | 0 额外配置 |
| License | 🟢 Apache-2.0 | 友好 |
| D3 兼容性 | 🟢 优 (本地优先) | 无 SaaS 依赖 |
| 实用度 | 🟡 **存疑** | 视频编码 over-engineered, 性能/压缩率不如专用向量库 |

### 8.6 Loom 落地路径

**核心判断**: **不吸**主架构, **借鉴**单文件便携思路

**不吸原因**:
- 视频编码是 over-engineered (向量库 + Postgres + sqlite 都比 .mp4 强)
- 检索延迟不可控 (跟 pgvector 5ms 比, 视频解码是 50-200ms)
- 没看到 benchmark 数据支持核心声明

**借鉴单文件思路** (Phase 3+, 暂缓):
- Loom Agent 备份 / 导出场景, 可以"打包成单文件"
- 但不作为主存储
- 仅作 portable backup 用

---

## 9. Hermes Agent 三层记忆 (6.2万⭐, Skills 生态绑定)

### 9.1 项目基础

| 字段 | 值 |
|------|---|
| **作者** | NousResearch |
| **Stars** | 6.2万 (4 月爆 6 万, 持续) |
| **License** | MIT |
| **GitHub** | github.com/NousResearch/hermes-agent |
| **技术栈** | Python 93.6% + TypeScript TUI |

### 9.2 三层记忆架构

| 层级 | 存储 | 生命周期 | 容量 | 用途 |
|------|------|---------|------|------|
| **短期 (STM)** | 内存 | 单次对话 | 受限 context | 维持对话连贯 |
| **长期 (LTM)** | SQLite + FTS5 | 永久 | 无限制 | 记住项目/偏好/习惯 |
| **程序化技能 (PSM)** | YAML 文件 | 永久 | 无限制 | 自动化复杂工作流 |

**配套文件**:
- `MEMORY.md`: 环境事实 (项目路径/API key/配置)
- `USER.md`: 用户画像 (工作习惯/偏好/常用命令)

### 9.3 自学习闭环 (E-A-A-S)

- 5+ 工具调用的复杂任务
- 5 分钟内触发 Curator 后台
- 经验提取 → GEPA 优化 → 技能生成
- 新技能立即可用

### 9.4 Loom 落地路径

**核心借鉴**: 三层记忆分类 + MEMORY.md / USER.md 文件

**ME-3 整合** (跟 TencentDB 四层一起):
- Hermes STM ≈ Loom Event Journal (L0 短期)
- Hermes LTM ≈ Loom 长期 vector store (L2 长期)
- Hermes PSM ≈ Loom Skill 库 (L3 技能)
- 文件结构借鉴 (`.loom/memory/{user,env,project}.md`)

**Loom 比 Hermes 优势**:
- Go 性能 > Python (尤其 Embedding / 检索热路径)
- DB-native (Postgres) > SQLite (生产可扩展)
- DBOS workflow 集成 (Hermes 无)
- 8 个 invariant 保障 (Hermes 无)

---

## 10. 横向对比矩阵

### 10.1 核心能力对比

| 维度 | claude-mem | MemPalace | Cognee | Mem0 | TencentDB | oracle-mem | memvid | Hermes |
|------|------------|-----------|--------|------|-----------|------------|--------|--------|
| **Stars (K)** | 89.4 | 58.0 | 29.7 | ~70 | 9.0 | (DB-native) | 16.1 | 62 |
| **语言** | TS | Python | Python | Python | TS | SQL/PL | Python | Python |
| **LLM 决策** | ✅ Claude | ❌ 无 | ✅ 默认 | ✅ OpenAI | ✅ 默认 | ❌ 无 | ❌ 无 | ✅ 提取 |
| **D3 兼容** | 🟡 | 🟢 满 | 🟡 | 🟡 | 🟡 | 🟢 | 🟢 | 🟡 |
| **向量库** | ChromaDB | ChromaDB | LanceDB | Qdrant/PG | VectorDB | Oracle 26ai | FAISS | SQLite+FTS5 |
| **KG 关系** | ❌ | ❌ | ✅ Graph | ❌ | ❌ | ✅ Property Graph | ❌ | ❌ |
| **混合检索** | 🟡 | ✅ 3 路 | ✅ Graph+Vec | ✅ 3 路 | 🟡 | ✅ | ❌ | 🟡 |
| **MCP 工具** | ✅ 4 个 | ✅ 29 个 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **时序支持** | 🟡 | 🟡 | ❌ | ✅ 4月 | ❌ | ✅ | ❌ | ❌ |
| **跨 Agent** | ✅ 7 个 | ❌ | ❌ | 🟡 | OpenClaw | ❌ | ❌ | ❌ |
| **开源协议** | Apache-2.0 | MIT | Apache-2.0 | Apache-2.0 | Apache-2.0 | (查) | Apache-2.0 | MIT |
| **生产可用** | ✅ | ✅ | 🟡 | ✅ | ✅ | ✅ v0.5.1 | 🟡 v2 新 | ✅ |

### 10.2 借鉴度排序 (Loom 视角)

| 排序 | 项目 | 借鉴度 | 关键借鉴 |
|------|------|--------|---------|
| 1 | **claude-mem** | ⭐⭐⭐⭐⭐ | 跨 7 Agent 协议层 |
| 2 | **MemPalace** | ⭐⭐⭐⭐⭐ | wing/room/drawer + 0 API key + R@5 96.6% |
| 3 | **oracle-mem** | ⭐⭐⭐⭐⭐ | DB-native 三位一体 (跟 P0-11 一致) |
| 4 | **Mem0** | ⭐⭐⭐⭐ | entity linking + temporal + multi-signal 算法 |
| 5 | **Cognee** | ⭐⭐⭐⭐ | Graph-RAG 双层检索 (无 LLM 决策版本) |
| 6 | **TencentDB** | ⭐⭐⭐ | L0/L1/L2/L3 四层架构 (拒 SaaS) |
| 7 | **Hermes 三层** | ⭐⭐⭐ | STM/LTM/PSM 三层分类 + MEMORY.md |
| 8 | **memvid** | ⭐⭐ | 单文件 portable backup (Phase 3 暂缓) |

---

## 11. Loom 落地 4 抓手 (汇总)

按 Phase 1 → Phase 2 顺序, 跟现有 work item 对齐:

### 抓手 1: SK-3 兼容 claude-mem 协议 (Phase 1 Slice 2 W5-W6, 1-2 天)

- 目的: 让 Loom Agent 跟 Claude Code / Codex / Gemini / Hermes / Copilot 等 7 大 agent 互通
- 集成: `internal/mem/claudemem_compat.go` + 4 个 MCP 工具 stub
- 落地: 跟 SK-1/SK-2 一起, 1-2 天
- 优先级: 🟢 **P0**

### 抓手 2: ME-2 三层 + Hybrid Retrieval (Phase 1 Slice 3, 3-5 天)

- 目的: 借鉴 MemPalace wing/room/drawer + Mem0 entity linking + multi-signal
- 集成: `internal/mem/{store,retrieval,entity}.go`
- 落地: 跟 S1-W2 Event Journal 集成, 跟 P0-11 DBOS Postgres 复用
- 优先级: 🟢 **P0**

### 抓手 3: TK-1 时序 KG 自研 (Phase 2, 2-3 周)

- 目的: 借鉴 oracle-mem 三位一体 + Cognee Graph-RAG 双层 + Mem0 temporal + 规则 NER (无 LLM)
- 集成: `internal/kg/{extract,relations,storage,retrieval}.go`
- 落地: 跟 S1-W2 Event Journal 集成 (source_event_id 外键)
- 优先级: 🟡 **P1**

### 抓手 4: ME-3 四层 + 三层分类 (Phase 2, 1 周)

- 目的: 借鉴 TencentDB L0/L1/L2/L3 + Hermes STM/LTM/PSM 分类
- 集成: `internal/mem/4layers.go` + 文件结构 `.loom/memory/`
- 落地: 跟 ME-2 集成
- 优先级: 🟡 **P1**

---

## 12. 不吸清单 (跟 D3 必决 / 8 invariant 冲突)

| 项目/部分 | 冲突原因 | 替代 |
|----------|---------|------|
| **Mem0 LLM 决策部分** | D3 拒 LLM 决策 | 自研规则 NER (ME-2) |
| **Cognee 默认 LLM** | D3 拒 LLM 决策 | 借 Graph-RAG 算法, 自研无 LLM 版 |
| **TencentDB Agent Memory 云版** | D3 拒 SaaS | 借四层架构, 自研本地版 (ME-3) |
| **Cognee 完整 SDK** | Python-only, Go 路线不友好 | 借算法, 自研 Go 版 |
| **memvid 视频编码** | Over-engineered, 性能差 | 借"单文件 portable"思路, 主存储用 PG |
| **Mem0 商业 SaaS 模式** | D3 拒 SaaS | 自研本地版, 不做托管 |
| **Mem0 / Hermes 自学习闭环** | D3 拒 LLM 决策 + secrets 风险 | 自研规则 consolidation worker, 无 LLM |

---

## 13. 兼容性矩阵 (跟 Phase 1 / Phase 2 已有 work item 对齐)

| 抓手 | Phase 1 Slice | 复用现有 | 新增代码 | 测试覆盖 |
|------|---------------|---------|---------|---------|
| **SK-3 协议兼容** | S2-W5-W6 | SK-1 + SK-2 | `internal/mem/claudemem_compat.go` 200行 | 4 测试 (4 MCP 工具) |
| **ME-2 三层+Hybrid** | S2-S3 | S1-W2 Event Journal + P0-11 DBOS | `internal/mem/{store,retrieval,entity}.go` 800行 | 8 测试 (3 路检索+RRF) |
| **TK-1 时序 KG** | Phase 2 | S1-W2 + P0-11 + 跟 Cognee 借 | `internal/kg/{extract,relations,storage,retrieval}.go` 1500行 | 12 测试 (NER+时序+双层) |
| **ME-3 四层** | Phase 2 | ME-2 + Hermes 借鉴 | `internal/mem/4layers.go` + `.loom/memory/` 600行 | 6 测试 (4层一致性) |

---

## 14. 风险与未决议题

### 14.1 技术风险

1. **MemPalace 96.6% R@5 是自报数据**: 没看到独立第三方验证, 谨慎参考, 落地以 Loom 自己的 benchmark 为准
2. **Cognee LLM 决策剥离难度**: 借算法容易, 借架构要重写 (Python → Go)
3. **oracle-mem 协议不是开放标准**: 借思路, 协议自研
4. **memvid benchmark 缺失**: 不吸主架构, 仅参考

### 14.2 优先级未决 (等用户拍)

1. **SK-3 优先级**: 跟 SK-1/SK-2 一起挤进 S2-W5-W6, 还是单独排期?
2. **ME-2 时序 KG (TK-1) 顺序**: 先 TK-1 再 ME-2, 还是反过来?
3. **ME-3 跟 ME-2 关系**: 合并到 ME-2 一个 work item, 还是分两个?

### 14.3 监管风险

- Mem0 商业 SaaS 模式 (云托管 user memory) 跟 GDPR / 个保法冲突
- Loom 落地全部本地化, 零云依赖, 监管最稳

---

## 15. 关联调研产物 (8.04 系列)

- `.loom-drafts/loom-memory-capability-classification-2026-08-04.md` (25KB, 19 维度分类)
- `.loom-drafts/loom-memory-usage-guide-2026-08-04.md` (24.5KB, 19 实战手册)
- `.loom-drafts/loom-memory-latest-2026-08-04.md` (34.5KB, 5 大最新趋势)
- `.loom-drafts/loom-memory-hot-projects-2026-08-04.md` (本报告, 7-8 项目深挖)

**4 份合计**: ~110KB, 覆盖"分类 → 实战 → 最新 → 爆火项目"全链路

---

**报告完**

> **下一步建议** (等用户拍):
> 1. **A. 直接落 SK-3 + ME-2**: 1-2 周内可完成 (跟 S2-W5/S3 整合), 产出 `loom-mem-compat` + `loom-mem-3layer` 2 个 work item
> 2. **B. 写 Phase 2 Memory 综合 spec**: 把 ME-2 / TK-1 / ME-3 整合成 1 个大 work item, 估 6-8 周, 1-2 人可完成
> 3. **C. 维持调研存档**: 4 份合计 110KB 作为"Phase 2 Memory work item 启动输入", 暂不动
> 4. **D. 写完整 cross-project KB**: 把 8.04 4 份 + 7.25 5 份合到 Loom `docs/memory/` 目录, 跟项目主仓一起维护
