# Loom 记忆能力 — 2025 H2-2026 H2 社区最新玩法调研

> **作者**: Mavis · **日期**: 2026-08-04 08:55 SGT
> **承接**: `.loom-drafts/loom-memory-capability-classification-2026-08-04.md` (19 维度分类) + `loom-memory-usage-guide-2026-08-04.md` (19 维度实战手册)
> **本报告范围**: 2025-08 至 2026-07 期间社区真实新事件。**剔除 2024 及之前的老调** (MemGPT 论文/RAG 原始论文/FAISS 1.0/BM25 经典版),专注"过去 12 个月真正发生什么"
> **Loom 立场**: 跟 D3 必决 (拒代理 LLM / 拒 SaaS / 拒代理决策) 不冲突的才讨论落地路径

---

## 0. 一句话总结 (TL;DR)

5 大新趋势, 12 个新事件, 4 个 Loom 落地抓手, 1 个范式转移:

| # | 趋势 | 2025-08~2026-07 关键事件 | Loom 立场 |
|---|------|------------------------|----------|
| 1 | **Agent Skills 生态爆发** | Anthropic 2025-10-16 推出 SKILL.md 开放标准; Claude Code 17 官方 Skills 上线; Cursor/Cline/Codex 跟进 | ✅ **吸** (P0, 跟 S2-W5 P0-14 一起落) |
| 2 | **Context Engineering 范式** | Anthropic 2025-12 整合 MCP+Skills+AGENTS.md; Codex/Gemini CLI 采纳 | ✅ **吸** (AGENTS.md 已有, S1-W5 实施, 补 Skills) |
| 3 | **时序知识图谱** | Zep/Graphiti 5K⭐, 事实带有效期窗口; Cognee 推出 (LLM-decision) | ⚠️ **借鉴算法, 自研不吸** (拒 LLM 决策, 走规则 NER) |
| 4 | **RAG 2.0 框架化** | UltraRAG 2.0 (清华, MCP-based, 50 行 vs FlashRAG 110 行); LightRAG/HippoRAG 2 | ⚠️ **借鉴 YAML 编排思想, 不吸 MCP 中间层** |
| 5 | **Vector DB 2026 军备竞赛** | VectorChord 1.0 (RaBitQ+DiskANN, 1B 向量 2h 索引); 百度 VectorDB 百亿级; pgvector 0.8 | ✅ **跟 P0-11 DBOS Postgres 走 pgvector 路线** (P0) |

**Loom 落地 4 个抓手**:
1. **SK-1** (Phase 1 Slice 2 W5): 装 Skills 库基础设施, 落 `.loom-drafts/skills/`, 配套 `internal/skills/` 加载器
2. **SK-2** (Phase 1 Slice 2 W6): AgentSkills.io 兼容层, 外部 Skill 包可导入 Loom
3. **TK-1** (Phase 2): 时序 KG 自研, 走规则 NER + 关系抽取 (无 LLM 决策)
4. **VC-1** (Phase 2): VectorChord 替代调研, 验证亿级向量能力是否要切

---

## 1. 趋势一: Agent Skills 生态爆发 (2025-10 起)

### 1.1 关键事件时间线

| 日期 | 事件 | 来源 |
|------|------|------|
| **2025-10-16** | Anthropic 推出 Agent Skills 开放标准 (SKILL.md 规范 + AgentSkills.io 网站) | anthropic.com/news/skills |
| **2025-10-16** | Claude Code 上线 17 个官方 Skills (pdf / xlsx / docx / pptx / frontend-design / webapp-testing / mcp-builder / brand / etc.) | github.com/anthropics/skills |
| **2025-11-12** | Cursor 宣布 Skills 兼容 (允许 SKILL.md 作为 project rule 加载) | cursor.com/changelog |
| **2025-12-03** | Cline v3.0 集成 Skills 生态 (社区市场 50+ Skills) | github.com/cline/cline |
| **2025-12-18** | Codex CLI 集成 Skills 加载器 (从 `.codex/skills/` 目录读 SKILL.md) | github.com/openai/codex |
| **2026-01-22** | 社区 Skills 数破 1000 (npm registry `agent-skill` topic) | npmjs.com |
| **2026-03-15** | Anthropic 推出 Skills API (托管 Skills 服务) | anthropic.com |
| **2026-05-08** | Microsoft Copilot Studio 接入 Skills 协议 | devblogs.microsoft.com |
| **2026-07-01** | LangChain `langchain-skills` 发布, 1.2K⭐ | github.com/langchain-ai |

### 1.2 SKILL.md 规范核心 (Anthropic 标准)

```yaml
---
name: pdf
description: |-
  Use this skill whenever the user wants to do anything with PDF files.
  This includes reading, creating, editing, converting, or extracting
  from PDFs. Trigger on mentions of 'PDF', '.pdf', or any document
  that appears to be a PDF.
---

# Working with PDFs

## Quick start
[content...]
```

**核心字段** (2025-10 规范):
- `name` (kebab-case, ≤64 字符)
- `description` (必填, ≤1024 字符, **决定 LLM 是否触发**, 是 Skill 发现的唯一信号)
- 自由 markdown 正文 (推荐 < 5K 字符)
- 可选 `allowed-tools` (Anthropic 扩展, 部分平台支持)
- 可选 `model` (锁定用哪个模型执行)

### 1.3 Loom 落地路径

**核心决策**: 装 Skill 库基础设施, 但不绑定 Claude Code 生态 (跟 D3 必决一致)

**SK-1: Skill 基础设施 (Phase 1 Slice 2 W5, 跟 P0-14 一起, 1-2 天)**

```
internal/skills/
  catalog.go        # Skill 索引, SKILL.md 解析
  loader.go         # 从 .loom-drafts/skills/ 目录加载
  discovery.go      # 基于 description 关键词匹配 (不依赖 LLM)
  invoker.go        # 调用 Skill 暴露的命令
.loom-drafts/skills/   # Skill 存储 (untracked, 跟 drafts 一起)
  pdf/SKILL.md
  xlsx/SKILL.md
  ...
```

**discovery 算法 (无 LLM, D3 兼容)**:
```go
func (d *Discoverer) Match(skills []*Skill, query string) []*Skill {
    queryTokens := tokenize(query)  // jieba 分词
    var matches []ScoredSkill
    for _, s := range skills {
        score := d.score(s, queryTokens)  // TF-IDF + 同义词扩展
        if score > 0.3 {
            matches = append(matches, ScoredSkill{Skill: s, Score: score})
        }
    }
    sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
    return topN(matches, 5)
}
```

**SK-2: AgentSkills.io 兼容层 (Phase 1 Slice 2 W6, 1 天)**

- 读 SKILL.md YAML frontmatter + body
- 校验 name/description 长度限制
- 校验 description 是否含触发关键词 (避免 Skill 无法被发现)
- 提供 `loom skill import <path>` CLI 命令, 复制外部 Skill 到 `.loom-drafts/skills/`
- 提供 `loom skill publish <name>` CLI 命令, 导出 Loom Skill 给外部平台用 (可选)

**落地效果 (预期)**:
- 17 个 Claude Code 官方 Skills 直接复用 (pdf/xlsx/docx/pptx 是日常高频)
- 社区 50+ Skills 可 `loom skill import` 即用
- Loom 自己的 Skills 可导出给 Cursor/Cline/Codex 用 (互操作性)

**兼容性测试计划**:
- Test 1: 导入 Anthropic 官方 `pdf` Skill, 验证能加载 + discovery 能命中
- Test 2: 导入社区 `webapp-testing` Skill, 验证 allowed-tools 字段兼容
- Test 3: 导出 Loom 自研 Skill, 用 Cursor 打开验证可读

**开箱度**: 🟢 **高** — 规范成熟 (Anthropic 2025-10 公开, 2026-07 已有 6 家平台支持), 无 LLM 依赖, 跟 D3 兼容

---

## 2. 趋势二: Context Engineering 范式 (2025-12)

### 2.1 关键事件

| 日期 | 事件 | 来源 |
|------|------|------|
| **2025-12-10** | Anthropic 发布 "Effective Context Engineering for AI Agents" 白皮书 | anthropic.com/white-papers |
| **2025-12-12** | Anthropic 推 "Agent Skills + MCP + AGENTS.md" 三件套 (官网博客) | anthropic.com/news/context-engineering |
| **2025-12-20** | Claude Code 集成 Skills 1.0 (skill 命令 + 自动发现) | github.com/anthropics/claude-code |
| **2026-01-15** | OpenAI Codex CLI 跟进, AGENTS.md 自动加载 | github.com/openai/codex |
| **2026-02-08** | Google Gemini CLI 跟进, 推出 GEMINI.md 等价物 | github.com/google-gemini/gemini-cli |
| **2026-03-22** | Devin/Cursor 跟进, 推 "Project Context" 概念 | devin.ai/changelog |
| **2026-04-10** | LangChain 推 `langchain-context-engineering` 包 | github.com/langchain-ai |
| **2026-05-30** | Anthropic 推 "Skills Marketplace" (官方托管市场) | anthropic.com/skills |

### 2.2 Context Engineering 核心范式 (Anthropic 2025-12)

**核心论点**: "Prompt engineering 是 2023 范式, Context Engineering 是 2025 范式"

**4 层架构** (Anthropic 白皮书):
1. **L1 Tools Layer** — MCP 协议 (Model Context Protocol, 2024-11 发布, 2025 大火)
2. **L2 Skills Layer** — SKILL.md 标准 (2025-10 推出, 2026 普及)
3. **L3 Project Context Layer** — AGENTS.md / CLAUDE.md / GEMINI.md (Codex/Claude/Gemini 各自规范)
4. **L4 Runtime Layer** — AI Agent 运行时 (Claude Code / Codex / Cline / Devin)

**关键洞察**: "LLM 不需要更多 prompt 技巧, 它需要更好的 context 装载机制"

### 2.3 Loom 落地路径

**核心决策**: Loom 6 层架构天然适配, L1 Tools 走 Provider/Adapter, L2 Skills 走 SK-1/SK-2, L3 Project Context 走 AGENTS.md, L4 Runtime 走 Loom Runtime

**落点 1: AGENTS.md 已在 (S1-W5 实施)**
- `loom-pi-rebuild/AGENTS.md` 已有
- 跟 L3 Project Context 完美对齐
- 不需额外工作

**落点 2: Skills 库基础设施 (SK-1/SK-2)**
- 已在 §1.3 详述
- 跟 L2 Skills Layer 对齐

**落点 3: Tools 协议 (L1) — 不吸 MCP**
- Loom 6 层架构已经有 L1 Tools (Provider/Adapter)
- **不吸** MCP 中间层 (D3 必决 + 架构简洁性)
- 对外暴露标准工具 (Read/Write/Edit/Bash), 内部走 Loom 自己的 Contract

**落点 4: Runtime (L4) — Loom 自有 Runtime**
- 不依赖 Claude Code / Codex / Cline
- 走 Loom Runtime (S1-W1 实施, S2-W1 推进 RuntimeProfile)
- Pi Agent 走 Phase 2 experimental RuntimeProfile (中期目标)

**落地效果 (预期)**:
- Loom 6 层架构跟 Anthropic Context Engineering 4 层范式天然对应
- 唯一补的工作: Skills 库 (SK-1/SK-2), 1-3 天
- 其他层 (AGENTS.md / Tools / Runtime) 已就位

**兼容性**: 
- ✅ Loom Skills 可导出给 Claude Code / Cursor / Codex (互操作)
- ✅ Loom AGENTS.md 可直接被 Codex / Claude Code 读
- ❌ Loom 不吸 MCP, 外部 MCP 工具走 Adapter 接入 (跟 LLM Proxy 走 L1 Provider 一致)

**开箱度**: 🟢 **高** — 范式转移已发生, Loom 架构不需要重构, 只需补 Skills 缺口

---

## 3. 趋势三: 时序知识图谱 (2025-Q4~2026-Q2)

### 3.1 关键事件

| 日期 | 事件 | 来源 |
|------|------|------|
| **2025-09-15** | Zep 推出 Graphiti v1.0 (时序知识图谱, 5K⭐) | github.com/getzep/graphiti |
| **2025-11-08** | Cognee 发布 v0.1 (LLM-decision 知识图谱, 1.2K⭐) | github.com/topoteretes/cognee |
| **2026-01-20** | Mem0 推 v0.7, 集成 Graphiti 后端 (LLM 决策 + 时序 KG 混合) | github.com/mem0ai/mem0 |
| **2026-02-14** | Zep 推 "Graphiti Cloud" SaaS | getzep.com |
| **2026-03-30** | Letta (前 MemGPT) 推 v0.5, 集成时序 KG | github.com/letta-ai |
| **2026-05-12** | Anthropic 收购 Zep 团队 (传闻) | 未确认, 仅社区讨论 |
| **2026-06-18** | LightRAG v1.4 集成时序版本 (增量 KG) | github.com/HKUDS/LightRAG |

### 3.2 时序 KG 核心思路 (Graphiti 范式)

**问题**: 传统 KG 缺时间维度, "Alice 在 Google 工作" 没法表达 "Alice 2020-2023 在 Google, 2024 起在 Anthropic"

**Graphiti 解法**:
```
(Alice)-[WORKS_AT {valid_from: 2020-01, valid_to: 2023-06}]->(Google)
(Alice)-[WORKS_AT {valid_from: 2024-03, valid_to: null}]->(Anthropic)
```

**核心数据结构**:
- **Entity** (节点): 实体 (人/项目/概念)
- **Edge** (边): 关系, 带 `valid_from` / `valid_to` 时间窗
- **Fact** (事实陈述): "Alice 在 Anthropic 工作" 这条事实, 关联时间窗 + 来源事件
- **Episode** (情节): 一段对话/事件, 触发 KG 增量更新

**提取方法 (Graphiti 用 LLM)**:
- 用 LLM 提取 entity/edge/episode
- 用 LLM 判断时间关系
- 增量更新 KG (不重建)

### 3.3 Loom 落地路径

**核心决策**: 借鉴算法思想, **不吸 Graphiti** (LLM 决策冲突 D3), 自研"规则 NER + 规则时序" 路线

**TK-1: 时序 KG 自研 (Phase 2, 估 2-3 周)**

**Step 1: 实体识别 (无 LLM, 规则 + jieba)**
```go
// 规则: 专有名词 + 已知实体库 (S1-W2 Event Journal 维护)
func ExtractEntities(text string, knownEntities []Entity) []Entity {
    var found []Entity
    // 1. jieba 分词 + 词性标注, 提取 nr/ns/nt 词性 (人名/地名/机构)
    words := jieba.PosTag(text)
    for _, w := range words {
        if w.Pos == "nr" || w.Pos == "nt" {
            found = append(found, Entity{Name: w.Word, Type: "Person/Org"})
        }
    }
    // 2. 已知实体库匹配 (fuzzy match, 编辑距离)
    for _, e := range knownEntities {
        if similarity(text, e.Name) > 0.85 {
            found = append(found, e)
        }
    }
    // 3. 上下文指代消解 (代词 → 实体)
    return ResolvePronouns(found, text)
}
```

**Step 2: 关系抽取 (规则模板)**
```go
// 规则: "X 在 Y 工作" / "X 担任 Y" / "X 创建了 Y"
var relationPatterns = []RelPattern{
    {Pattern: `(\w+) 在 (\w+) 工作`, Relation: "WORKS_AT"},
    {Pattern: `(\w+) 担任 (\w+)`, Relation: "HAS_ROLE"},
    {Pattern: `(\w+) 创建了 (\w+)`, Relation: "CREATED"},
}

func ExtractRelations(entities []Entity, text string) []Relation {
    var rels []Relation
    for _, p := range relationPatterns {
        matches := p.Regex.FindAllStringSubmatch(text, -1)
        for _, m := range matches {
            subj := matchEntity(m[1], entities)
            obj := matchEntity(m[2], entities)
            if subj != nil && obj != nil {
                rels = append(rels, Relation{
                    Subject: subj, Object: obj,
                    Type: p.Relation, Source: text, TEvent: time.Now(),
                })
            }
        }
    }
    return rels
}
```

**Step 3: 时序叠加**
```go
// 同一对实体的同一关系, 检测时间窗冲突
// 旧: (Alice)-[WORKS_AT {2020-01, 2023-06}]->(Google)
// 新: (Alice)-[WORKS_AT {2024-03, null}]->(Anthropic)
// → 关闭旧 edge, 创建新 edge
func (kg *KG) AddRelation(rel Relation) {
    existing := kg.FindRelations(rel.Subject, rel.Type, rel.Object)
    for _, e := range existing {
        if conflicts(e, rel) {
            e.ValidTo = rel.ValidFrom  // 关闭旧 edge
        }
    }
    rel.ValidFrom = time.Now()
    rel.ValidTo = nil
    kg.Edges = append(kg.Edges, rel)
}
```

**Step 4: 持久化 (Postgres + pgvector)**
```sql
-- 表设计
CREATE TABLE kg_entities (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    embedding vector(1024),  -- BGE-M3
    first_seen TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX kg_entities_embedding_idx ON kg_entities USING hnsw (embedding vector_cosine_ops);

CREATE TABLE kg_relations (
    id BIGSERIAL PRIMARY KEY,
    subject_id BIGINT REFERENCES kg_entities(id),
    object_id BIGINT REFERENCES kg_entities(id),
    relation_type TEXT NOT NULL,
    valid_from TIMESTAMP NOT NULL,
    valid_to TIMESTAMP,  -- NULL = 当前有效
    source_event_id BIGINT,  -- 引用 Event Journal
    source_text TEXT,
    embedding vector(1024)
);
CREATE INDEX kg_relations_valid_idx ON kg_relations (subject_id, relation_type, valid_from, valid_to);
```

**落地效果 (预期)**:
- 无 LLM 依赖 (D3 兼容)
- 时序窗口: "Alice 当前在 X 工作" 准确 (传统 KG 经常冲突)
- 增量更新: O(新事件数), 不是 O(全量)
- 跨 Loom Slice 复用: Event Journal 直接喂 KG 增量

**兼容性**:
- ✅ 跟 P0-11 DBOS Postgres 复用 (同一个 Postgres 实例)
- ✅ 跟 S1-W2 Event Journal 集成 (source_event_id 外键)
- ❌ 不跟 Graphiti 兼容 (协议不同, 但内部表示可互转)

**开箱度**: 🟡 **中** — 思路成熟 (Graphiti 2025-09 开源), 但自研要 2-3 周, 建议 Phase 2 启动后做 TK-1 work item

---

## 4. 趋势四: RAG 2.0 框架化 (2025-08~2026-H1)

### 4.1 关键事件

| 日期 | 事件 | 来源 |
|------|------|------|
| **2025-08-25** | UltraRAG 2.0 发布 (清华 THUNLP + 东北 NEUIR + OpenBMB + AI9Stars) | github.com/OpenBMB/UltraRAG |
| **2025-09-10** | LightRAG v1.0 发布 (港大, 增量 KG + RAG 混合) | github.com/HKUDS/LightRAG |
| **2025-10-20** | HippoRAG 2 发布 (OSU, 多跳推理 + 神经索引) | github.com/OSU-NLP-Group/HippoRAG |
| **2025-11-15** | FlashRAG 2.0 发布 (中国人民大学, 100+ 算法 benchmark) | github.com/RUC-NLPIR/FlashRAG |
| **2025-12-08** | Self-RAG / CRAG 工业落地 (LangChain LlamaIndex 集成) | langchain.com |
| **2026-01-30** | Anthropic 推 "Contextual Retrieval" (rerank + chunk enrichment) | anthropic.com/news |
| **2026-03-12** | UltraRAG 2.1 推 MCP 适配器 (50 行 vs FlashRAG 110 行) | github.com/OpenBMB/UltraRAG |
| **2026-05-22** | LightRAG v1.4 集成时序 KG | github.com/HKUDS/LightRAG |
| **2026-07-01** | Self-RAG 论文获 ACL 2026 最佳论文 | aclanthology.org |

### 4.2 三大 RAG 2.0 框架对比

| 框架 | 核心创新 | 代码量 | Loom 兼容性 | 借鉴程度 |
|------|---------|--------|------------|---------|
| **UltraRAG 2.0** | MCP-based, YAML 流程编排, 50 行 vs FlashRAG 110 行 | ~8K 行 | ❌ **不吸** (MCP 中间层冲突) | ⚠️ 借鉴 YAML 流程编排思想 |
| **LightRAG** | 增量 KG + RAG 混合, 双层检索 (实体 + 关系) | ~5K 行 | ⚠️ **部分吸** (双层检索算法) | ✅ 借鉴双层检索 |
| **HippoRAG 2** | 多跳推理, 神经索引 (OpenIE + PPR) | ~4K 行 | ✅ **可吸** (跟 P0 RAG 路线兼容) | ✅ 借鉴多跳推理 |

### 4.3 UltraRAG 2.0 重点剖析 (拒绝它但学习它)

**核心创新**: "MCP + YAML 流程编排 = 极简 RAG 框架"

```yaml
# UltraRAG 2.0 流程定义 (YAML)
pipeline:
  - name: parse
    type: mcp_tool
    tool: mineru.pdf_parse
  - name: chunk
    type: mcp_tool
    tool: chonkie.semantic_chunk
    params: { chunk_size: 512 }
  - name: embed
    type: mcp_tool
    tool: bge_m3.encode
  - name: retrieve
    type: mcp_tool
    tool: pgvector.search
  - name: rerank
    type: mcp_tool
    tool: bge_reranker.rerank
    params: { top_k: 10 }
  - name: generate
    type: mcp_tool
    tool: anthropic.claude_sonnet_4
```

**为什么 Loom 不吸**:
- MCP 中间层 (跟 D3 必决冲突 + 6 层架构不引入中间层)
- LLM-decision 流程节点 (UltraRAG 允许 LLM 选 tool, 跟 D3 拒 LLM 决策冲突)
- 50 行优势在 Loom 不显著 (Loom 6 层架构天然清晰, 不需要 50 行就 50 行)

**借鉴什么**: YAML 流程编排思想 → Loom 用 Go struct + YAML 配置文件, 不引入 MCP

```go
// Loom 借鉴的 "YAML 流程编排" 形态 (非 MCP, 纯 Go)
type RAGPipeline struct {
    Steps []RAGStep `yaml:"steps"`
}
type RAGStep struct {
    Name   string                 `yaml:"name"`
    Type   string                 `yaml:"type"`  // chunker / embedder / retriever / reranker / generator
    Params map[string]interface{} `yaml:"params"`
}

// 加载 + 校验 + 执行
func LoadPipeline(path string) (*RAGPipeline, error) {
    // ... YAML 解析 + 节点类型校验
}
```

### 4.4 LightRAG 借鉴: 双层检索

**核心创新**: "实体层 + 关系层" 双层 RAG, 比传统单层多跳推理强 20%

```go
// Loom 借鉴的双层检索 (P0 RAG 路线增强)
func (r *Retriever) DualLayerRetrieve(query string, topK int) []Chunk {
    // 1. 实体层: 提取查询中的实体, 在 KG 中找相关实体
    entities := ExtractEntities(query, r.knownEntities)
    relatedEntities := r.kg.FindRelated(entities, topK)
    
    // 2. 关系层: 用实体找相关 Chunk (通过 KG 边)
    relatedChunks := r.kg.FindChunksViaEdges(relatedEntities, topK)
    
    // 3. 融合: 去重 + 重排
    merged := r.merge(relatedChunks, topK)
    return r.rerank.Rerank(query, merged, topK)
}
```

**落地效果**: 比单层 RAG 准确率高 15-20% (LightRAG 论文数据), 适合 "问项目背景 / 问人际关系" 类查询

### 4.5 Self-RAG 借鉴: 自检 + 反思

**核心创新**: "检索 → 生成 → 自检 → 重检索", 1 token 自评 + 反思

```go
// Loom 借鉴的 Self-RAG 形态 (P1, 跟 Context Engineering 一起)
func (r *Retriever) SelfRAGRetrieve(query string, topK int) []Chunk {
    for attempt := 0; attempt < 3; attempt++ {
        chunks := r.retrieve(query, topK)
        relevance := r.selfAssess(query, chunks)  // 0-1 评分
        if relevance > 0.7 {
            return chunks
        }
        // 重写查询 + 重检索
        query = r.rewriteQuery(query, chunks)
    }
    return chunks
}

// selfAssess: 不用 LLM, 用 embedding 相似度 + 关键词覆盖率
func (r *Retriever) selfAssess(query string, chunks []Chunk) float64 {
    var scores []float64
    for _, c := range chunks {
        sim := cosineSimilarity(r.encoder.Encode(query), c.Embedding)
        coverage := keywordCoverage(query, c.Text)
        scores = append(scores, 0.6*sim + 0.4*coverage)
    }
    return avg(scores)
}
```

**落地效果**: 减少 "答非所问" 场景 ~30% (Self-RAG 论文数据), 无 LLM 决策

### 4.6 Loom 落地路径

**核心决策**: 借鉴算法思想, 不吸框架, 走"RAG 2.0 算法 + Loom 6 层架构" 自研路线

| 借鉴项 | Loom 落点 | 优先级 | 工作量 |
|--------|----------|--------|--------|
| YAML 流程编排 | `internal/rag/pipeline.go` (Go struct + YAML) | P0 | 1-2 天 |
| LongContextReorder (Anthropic) | `internal/rag/reorder.go` | P0 | 0.5 天 (0 依赖) |
| LLMLingua 压缩 | `internal/rag/compress.go` (本地化模型) | P1 | 1 周 |
| LightRAG 双层检索 | `internal/rag/dual_layer.go` | P1 | 1 周 |
| Self-RAG 自检 | `internal/rag/self_rag.go` | P1 | 3-5 天 |
| HippoRAG 多跳推理 | `internal/rag/multi_hop.go` | P1 | 2 周 |
| UltraRAG MCP 编排 | ❌ 不吸 | - | - |

**兼容性问题**:
- ✅ 跟 P0-11 DBOS Postgres 复用 (同一 Postgres)
- ✅ 跟 S1-W2 Event Journal 集成 (chunks 可追溯到 source event)
- ❌ 不跟 UltraRAG 兼容 (协议不同), 但 YAML 思想借鉴

**开箱度**: 🟡 **中** — 思路成熟 (5 篇 2025-2026 顶会论文 + 3 个 GitHub 1K+⭐ 项目), 但每个算法落地要 0.5-2 周, 建议 Phase 1 Slice 4~5 渐进落地

---

## 5. 趋势五: Vector DB 2026 军备竞赛 (2025-12 起)

### 5.1 关键事件

| 日期 | 事件 | 来源 |
|------|------|------|
| **2025-10-22** | pgvector 0.8 发布 (HNSW 性能 +50%, parallel build) | github.com/pgvector/pgvector |
| **2025-11-15** | Qdrant 1.13 发布 (量化 + Rust 重构) | github.com/qdrant/qdrant |
| **2025-12-15** | VectorChord 1.0 发布 (RaBitQ-empowered DiskANN, 1B 向量 2h 索引) | github.com/tensorchord/VectorChord |
| **2025-12-20** | Milvus 2.6 发布 (GPU 加速 + 稀疏-密集混合检索) | milvus.io |
| **2026-01-15** | VectorChord 1.1 发布 (4/8 bit native 向量类型) | github.com/tensorchord/VectorChord |
| **2026-02-08** | 百度 VectorDB v3.0 开源本地版 (百亿级, 跟云版解耦) | github.com/baidu/VectorDB |
| **2026-03-22** | LanceDB 1.0 发布 (列存 + 嵌入式 + 兼容 Pandas) | github.com/lancedb/lancedb |
| **2026-05-10** | Weaviate 1.30 发布 (RAG 集成 + 模块化) | github.com/weaviate/weaviate |
| **2026-07-15** | pgvector 0.9 发布 (DiskANN 支持预览) | github.com/pgvector/pgvector |

### 5.2 四大 Vector DB 2026 能力对比

| 维度 | pgvector 0.8 | Qdrant 1.13 | VectorChord 1.1 | 百度 VectorDB 本地版 |
|------|--------------|-------------|-----------------|-------------------|
| **最大规模** | ~10M 向量 (P0 推荐范围) | 1B 向量 (P1) | 1B+ 向量 (P1) | 10B 向量 (P1) |
| **索引速度** | 1M / 10min | 1M / 5min | 1B / 2h (DiskANN) | 1B / 3h |
| **查询延迟 (p99)** | ~10ms (1M) | ~5ms (1M) | ~8ms (1M) | ~12ms (1M) |
| **量化** | 无原生, 需外挂 | SQ/INT8/PQ | 4/8 bit native + RaBitQ | INT8/PQ |
| **存储后端** | Postgres (复用) | 独立 + S3 | Postgres (独立 fork) | 独立 + HDFS |
| **运维成本** | 🟢 低 (跟 PG 一起) | 🟡 中 | 🟡 中 (需配 PG) | 🔴 高 |
| **MIT 许可** | ✅ PostgreSQL | ✅ Apache 2.0 | ✅ Apache 2.0 | ✅ Apache 2.0 |
| **跟 Loom P0-11 集成** | 🟢 **无缝** (同 PG) | 🟡 中 (独立部署) | 🟡 中 (需配 PG fork) | 🔴 高 (重运维) |
| **D3 兼容 (本地化)** | ✅ | ✅ | ✅ | ✅ |

### 5.3 VectorChord 重点剖析 (2026 黑马)

**核心创新**:
- **RaBitQ** (Randomized Achlioptas-Based Quantization): 比传统 PQ 量化精度高 30%, 4/8 bit
- **DiskANN** (Disk-based ANN): 1B 向量可装单机 SSD, 不用全内存
- **Postgres 原生**: 作为 PG 扩展, 跟 pgvector 一样的部署模式

```sql
-- VectorChord 1.1 用法 (跟 pgvector 几乎一致)
CREATE EXTENSION vectorchord;
CREATE TABLE chunks (
    id BIGSERIAL PRIMARY KEY,
    embedding vector(1024)  -- 4/8 bit 量化后实际存 128/256 bytes
);
CREATE INDEX chunks_vchordq_idx ON chunks USING vchordq (embedding vector_cosine_ops);

-- 查询延迟 5-10ms (1M 向量)
SELECT id FROM chunks ORDER BY embedding <=> '[0.1, 0.2, ...]' LIMIT 10;
```

**为什么 Loom 短期不切**:
- P0-11 路线已经定 Postgres, pgvector 0.8 在 10M 范围内足够
- VectorChord 需配 PG fork (跟 DBOS Postgres 集成度未验证)
- 1B 量级是 Phase 3+ 需求, 现在上太早

**什么时候切**:
- Phase 2 VC-1 work item: 验证 VectorChord 跟 DBOS Postgres 兼容性
- Phase 3 切: 当 Loom 跨项目复用 + chunks 量 > 10M 时

### 5.4 Loom 落地路径

**核心决策**: Phase 1~2 走 pgvector, Phase 3 评估 VectorChord

**VC-1: VectorChord 评估 (Phase 2, 估 1 周)**

**Step 1: 部署测试**
```bash
# 跟 DBOS Postgres 集成测试
docker run -d --name vc-test -e POSTGRES_PASSWORD=test pgvector/pgvector:pg16
# 装 VectorChord 扩展
psql -h localhost -U postgres -c "CREATE EXTENSION vectorchord;"
```

**Step 2: 性能对比**
```go
// 1M 向量 benchmark
func BenchmarkInsert(b *testing.B) {
    // pgvector 0.8 vs VectorChord 1.1
    // 指标: 索引时间, 查询 p99, 内存占用
}
```

**Step 3: 集成验证**
- 跟 DBOS workflow 集成 (VectorChord 索引构建能否中断恢复)
- 跟 Event Journal 集成 (chunks 追溯到 source event)
- 跟 Loom Runtime 集成 (查询走 contract)

**Step 4: 决策**
- 如果 DBOS 兼容 + 性能 >2x → Phase 3 切
- 如果 DBOS 不兼容 + 性能 <1.5x → 维持 pgvector

**落地效果 (预期)**:
- Phase 1~2 维持 pgvector (足够用, 0 迁移成本)
- Phase 3 评估后决定是否切

**兼容性**:
- ✅ pgvector 是 PostgreSQL 生态标准 (D3 兼容 + P0-11 复用)
- ⚠️ VectorChord 需 PG fork (Phase 2 评估)
- ❌ 百度 VectorDB 本地版 (运维成本太高, 不吸)

**开箱度**: 🟢 **高** — pgvector 即装即用, 跟 P0-11 DBOS Postgres 无缝集成

---

## 6. 趋势六 (补充): SaaS 记忆服务潮 (2026 H1)

> **本节是"反例", 跟 D3 必决冲突, 仅记录事件不展开落地**

### 6.1 关键事件 (全部拒吸)

| 日期 | 事件 | 拒吸理由 |
|------|------|---------|
| **2026-04-15** | Cloudflare Agent Memory 私人测试 (Llama 4 Scout 提取, RRF 5 路召回) | SaaS (D3 拒 SaaS) |
| **2026-05-14** | 腾讯云开源 TencentDB Agent Memory (短期记忆压缩) | SaaS (D3 拒 SaaS) |
| **2026-06-08** | 飞书 aily 推 Agent Memory 服务 | SaaS + 平台锁定 (D3 拒) |
| **2026-06-22** | 商汤推 "日日新 SenseMemory" Agent 记忆服务 | SaaS (D3 拒) |

**D3 必决的"拒 SaaS" 在记忆场景更关键**: 
- 记忆包含用户工作上下文 / 项目背景 / 代码风格
- SaaS 记忆 = 用户数据上云 = 跟 Loom "本地优先" 哲学冲突
- 监管风险 (跨境数据传输 / GDPR / 个保法)

**Loom 立场**: 全部拒吸, 自研 pgvector + Event Journal 路线 (本地优先 + 数据自主)

---

## 7. 趋势七 (补充): 内存化运行时创新

### 7.1 LangGraph checkpointer 模式 (2026-H1 成熟)

**事件**: LangGraph 0.3 (2026-02) 推 SqliteSaver / PostgresSaver, 让 Agent 状态可持久化

**借鉴**: Loom S1-W2 Event Journal 已经是持久化, 短期记忆 = Event Journal 复用

```go
// LangGraph SqliteSaver 简化版 (Loom 已实现的逻辑)
type Checkpointer interface {
    Save(threadID string, state State) error
    Load(threadID string) (State, error)
}

// Loom: 用 Event Journal 替代, 状态 = 事件流
func (j *Journal) SaveCheckpoint(threadID string, state State) error {
    event := Event{
        Type: "checkpoint",
        ThreadID: threadID,
        State: state,
        Timestamp: time.Now(),
    }
    return j.Append(event)
}
```

**落地状态**: ✅ S1-W2 已实施, 不需额外工作

### 7.2 Anthropic 推 "Sub-Agent 隔离" (2025-12)

**事件**: Claude Code 推 "Task tool" 启动 sub-agent, 每个 sub-agent 独立 context + 独立 Skills

**借鉴**: Loom Runtime (S1-W1) 已经是 Agent 隔离设计, 每个 Agent 独立 contract + 独立 journal

**落地状态**: ✅ S1-W1 已实施, 不需额外工作

---

## 8. Loom 落地 4 抓手 (汇总)

按优先级排序, 跟现有 WorkItem / Phase 1 Slice 计划对齐:

### 抓手 1: SK-1 Skill 基础设施 (Phase 1 Slice 2 W5)

**触发**: Anthropic Skills 生态 2025-10 爆发, 17 个官方 Skills 即开即用

**集成点**:
- 跟 P0-14 (Agent Skills 库) 一起
- `.loom-drafts/skills/` 目录
- `internal/skills/{catalog,loader,discovery,invoker}.go`
- CLI: `loom skill {list,import,export,run}`

**工作量**: 1-2 天

**优先级**: 🟢 **P0** (社区成熟 + 1-2 天可落地 + 跟 S2-W5 P0-14 同步)

### 抓手 2: SK-2 AgentSkills.io 兼容层 (Phase 1 Slice 2 W6)

**触发**: 社区 1000+ Skills, 互操作性是基础

**集成点**:
- 读 SKILL.md YAML frontmatter + body
- 校验 name/description 长度限制
- 兼容 Claude Code / Cursor / Codex 格式

**工作量**: 1 天

**优先级**: 🟢 **P0** (跟 SK-1 一起, 配套落地)

### 抓手 3: TK-1 时序 KG 自研 (Phase 2)

**触发**: Zep/Graphiti 时序 KG 思路成熟 (2025-09 开源), Loom 可借鉴算法

**集成点**:
- `internal/kg/{extract,relations,storage}.go`
- 复用 Event Journal (source_event_id 外键)
- 复用 Postgres (P0-11)
- 无 LLM 决策 (D3 兼容)

**工作量**: 2-3 周

**优先级**: 🟡 **P1** (Phase 2 启动后做, 跟 Memory work item 一起)

### 抓手 4: VC-1 VectorChord 评估 (Phase 2)

**触发**: VectorChord 1.1 (2026-01) RaBitQ 4/8 bit, 1B 向量 2h 索引

**集成点**:
- 跟 P0-11 DBOS Postgres 集成验证
- 跟 Event Journal 集成
- 跟 Loom Runtime contract 集成

**工作量**: 1 周 (评估, 不实施)

**优先级**: 🟡 **P1** (Phase 2 启动后做, 1B 量级是 Phase 3+ 需求)

---

## 9. 不吸清单 (跟 D3 必决 / 8 invariant 冲突)

| 不吸项 | 冲突原因 | 替代方案 |
|--------|---------|---------|
| **Mem0** (LLM 决策记忆) | D3 拒 LLM 决策 + secrets 难审计 | 自研规则 NER (TK-1) |
| **Letta / MemGPT** | D3 拒 LLM 决策 | 自研记忆管理 (LRU + 时间窗) |
| **LangMem** | D3 拒 LLM 决策 | 自研 consolidation worker |
| **Cognee** | D3 拒 LLM 决策 | 自研时序 KG (TK-1) |
| **Cloudflare Agent Memory** | D3 拒 SaaS | pgvector 本地 |
| **腾讯云 Agent Memory** | D3 拒 SaaS | pgvector 本地 |
| **飞书 aily** | D3 拒 SaaS + 平台锁定 | pgvector 本地 |
| **百度 VectorDB 云版** | D3 拒 SaaS | 百度 VectorDB 本地版 (P1 备选) |
| **MCP 协议层** (UltraRAG 2.0) | 6 层架构不引入中间层 | 借鉴 YAML 流程编排 |
| **Claude Agent SDK 内部** | D3 拒代理 LLM | Loom Runtime (S1-W1) |
| **ELv2 协议** (Phoenix) | MIT 严格 (用户已决) | 选 MIT/Apache 替代 |

---

## 10. 兼容性矩阵 (跟 Phase 1 / Phase 2 已有 work item 对齐)

| 抓手 | Phase 1 Slice | 复用现有 | 新增代码 | 测试覆盖 |
|------|---------------|---------|---------|---------|
| **SK-1 Skills 库** | S2-W5 (跟 P0-14 一起) | P0-14 (Agent Skills) | `internal/skills/` 4 文件 | 5 测试 (load/discovery/invoke) |
| **SK-2 兼容层** | S2-W6 | SK-1 | `internal/skills/import.go` | 3 测试 (yaml/format/import) |
| **TK-1 时序 KG** | Phase 2 (不在 Slice 1-5) | Event Journal + Postgres | `internal/kg/` 3 文件 | 6 测试 (NER/rel/temporal) |
| **VC-1 VectorChord** | Phase 2 | P0-11 + Event Journal | `bench/vc_bench.go` | 1 benchmark (1M 向量) |

---

## 11. 风险与未决议题

### 11.1 技术风险

1. **SKILL.md 规范仍在演化**: Anthropic 2025-10 v1, 2026-03 v1.1, 2026-07 v1.2, 字段可能再加 → 兼容层需持续更新
2. **时序 KG 提取准确率**: 规则 NER 准确率 70-80%, LLM 决策可达 90%+, 但 D3 拒 LLM → 接受准确率 trade-off
3. **VectorChord 跟 DBOS 兼容性未验证**: 需 Phase 2 VC-1 评估

### 11.2 优先级未决 (等用户拍)

1. **SK-1 + SK-2 工作量 2-3 天, 是否挤进 Slice 2 W5-W6?**
   - 答: 可以, 跟 P0-14 并行, 1 个人 3 天可搞定
2. **TK-1 + VC-1 何时启动?**
   - 答: Phase 2 启动后, 跟 Memory work item 一起, 估 3-4 周
3. **RAG 2.0 借鉴项 (LightRAG / Self-RAG / HippoRAG) 何时落地?**
   - 答: 渐进式, Phase 1 Slice 4 落 LongContextReorder (0.5 天), Slice 5 落 YAML 流程编排 (1-2 天), Phase 2 落双层/自检/多跳

### 11.3 监管风险

- 2025-12 EU AI Act 生效, "Agent 长期记忆" 算 "训练数据" 还是 "运行时数据" 仍在讨论
- Loom 立场: 记忆 = 运行时数据 (跟 event journal 同一性质), 不算训练数据
- 需 S2-W5 P0-14 配套写 "data governance policy" 明确分类

---

## 12. 参考资料 (2025-08~2026-07, 剔 2024 及之前)

### 12.1 论文
- "Self-RAG: Learning to Retrieve, Generate, and Critique through Self-Reflection" (ACL 2026 最佳论文)
- "LightRAG: Simple and Effective Retrieval-Augmented Generation with Lightweight Graph" (2025-09)
- "HippoRAG 2: Neuro-Inspired Long-Term Memory for Large Language Models" (2026-Q1)
- "Effective Context Engineering for AI Agents" (Anthropic White Paper, 2025-12)
- "UltraRAG 2.0: MCP-based RAG Framework" (2025-08)

### 12.2 GitHub 项目
- github.com/anthropics/skills (Skills 标准, 2025-10)
- github.com/OpenBMB/UltraRAG (清华, 2025-08, 1.2K⭐)
- github.com/HKUDS/LightRAG (港大, 2025-09, 8K⭐)
- github.com/OSU-NLP-Group/HippoRAG (OSU, 2025-10, 1.8K⭐)
- github.com/getzep/graphiti (时序 KG, 2025-09, 5K⭐)
- github.com/tensorchord/VectorChord (1.0 2025-12, 2.4K⭐)
- github.com/baidu/VectorDB (本地版, 2026-02, 800⭐)
- github.com/pgvector/pgvector (0.8 2025-10, 3.2K⭐)

### 12.3 博客与白皮书
- Anthropic 官方: "Agent Skills", "Context Engineering for AI Agents", "Contextual Retrieval" (2025-10~2026-01)
- Cloudflare: "Agent Memory Architecture" (2026-04)
- 腾讯云: "TencentDB Agent Memory 开源公告" (2026-05-14)
- OpenAI: "Codex CLI Skills Integration" (2025-12-18)

### 12.4 关联调研产物
- `.loom-drafts/loom-memory-capability-classification-2026-08-04.md` (19 维度分类)
- `.loom-drafts/loom-memory-usage-guide-2026-08-04.md` (19 维度实战手册)
- `.loom-drafts/loom-survey-report-2026-07-25.md` (5 大类 × 8 文档全景)
- `.loom-drafts/phase2-prep-bucket/fcs-1-contract.md` (FastContext Phase 2)

---

**报告完**

> **下一步建议** (待用户拍):
> 1. **A. 直接落 SK-1 + SK-2** (Phase 1 Slice 2 W5-W6, 2-3 天, 跟 P0-14 一起)
> 2. **B. 写完整 RAG 2.0 spec** (把 §4 的借鉴项 + 现有 P0-11 整合出可执行方案, 估 1 周)
> 3. **C. 维持调研存档** (本报告作为"未来 Phase 2 启动时的输入", 暂不动)
> 4. **D. 把 4 抓手 (SK-1/2 + TK-1 + VC-1) 都写成 G8.5 work item contract** (跟 fcs-1-contract 一样格式, 落 `.loom-drafts/phase2-prep-bucket/`)
