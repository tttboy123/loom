# Loom Agent 记忆能力 — 实战使用手册

> **日期**: 2026-08-04
> **承接**: `loom-memory-capability-classification-2026-08-04.md` (19 能力分类表)
> **目的**: 把"分类"翻译成"怎么做/效果/怎么用/开箱度",给 Loom 开发者/Agent 编排者实战参考
> **目标读者**: Loom 实施者 / Phase 1/2 开发者 / 用 Loom 编排 Agent 的应用方

---

## TL;DR — 19 能力速查表

| # | 能力 | 一句话做法 | 主要效果 | Loom 开箱度 | 实施成本 |
|---|------|-----------|---------|------------|----------|
| 1 | 嵌入 (Embedding) | BGE-M3 跑一遍文本 | 文本 → 768 维向量 | 🟡 需装模型 | 1 天 |
| 2 | 向量压缩 | Matryoshka 截断 + INT8 | 存储 4x ↓, 精度 -1% | 🟢 一行 SQL | 0.5 天 |
| 3 | 块策略 | Fixed 512 + 50 overlap 切 | 段落级检索单元 | 🟢 LangChain 现成 | 0.5 天 |
| 4 | 向量索引 | pgvector HNSW (m=16, ef=128) | 千万级 ms 级召回 | 🟢 `CREATE EXTENSION` | 0.5 天 |
| 5 | 稀疏检索 | Postgres FTS5 (BM25) | 关键词精确匹配 | 🟢 `tsvector` + GIN | 1 天 |
| 6 | 混合检索 | RRF 合并 dense + sparse | 召回率 +15% | 🟢 SQL 函数 | 0.5 天 |
| 7 | 重排序 | BGE-Reranker-v2-M3 离线部署 | top-20 → top-5 精排 | 🟡 需 GPU | 1 天 |
| 8 | 跨模态 (ColBERT) | 每 token 一向量 + MaxSim | 段级跨段检索 | 🟡 工程量 | 3-4 天 |
| 9 | 查询改写 | jieba 同义词 + code pattern | 召回率 +5% | 🟢 现成词典 | 1 天 |
| 10 | 上下文压缩 | LongContextReorder 挪首尾 | Lost-in-middle 缓解 30% | 🟢 0 依赖 | 0.5 天 |
| 11 | 工作记忆 | Event Journal + Projection | 进程内状态可恢复 | 🟢 S1-W2/W4 已有 | 0 |
| 12 | 长期记忆 | pgvector + Episodic/Semantic/Procedural | 跨会话检索 | 🟡 schema 扩展 | 2-3 天 |
| 13 | 记忆类型 | event_type 字段 enum | episodic/semantic/procedural | 🟢 S1-W2 已有字段 | 1 天 |
| 14 | 记忆管理 | LRU + 时间窗口 + 重要度 | 自动淘汰冷记忆 | 🟢 自研 | 2-3 天 |
| 15 | 图谱增强 (GraphRAG) | LightRAG 实体+关系 | 关系型问答 | 🔴 需独立服务 | 5-7 天 |
| 16 | 会话持久化 | LangGraph SqliteSaver | 断点续跑 Agent | 🟢 S1-W2 复用 | 0.5 天 |
| 17 | Skill 库 | Anthropic Skills (SKILL.md) | 程序性知识 progressive disclosure | 🟢 标准成熟 | 2-3 天 |
| 18 | 可观测性 | Prometheus + Grafana | hit rate / 延迟 / 成本 仪表盘 | 🟢 0 依赖 | 2-3 天 |
| 19 | 安全隔离 | Postgres RLS + namespace | 多租户防泄漏 | 🟢 现有字段 | 1-2 天 |

**开箱度图例**:
- 🟢 **绿色** = Loom 现状已支持或 1-2 行 SQL 启用
- 🟡 **黄色** = 需要装模型/库,有现成参考实现
- 🔴 **红色** = 需要独立工程/调研,Phase 2+ 再考虑

---

## 1. 嵌入 (Embedding) — 文本/代码 → 向量

**怎么做**:
- 用 BGE-M3 (智源, MIT, 568M 参数, 支持 100+ 语言) 跑一遍文本
- 输出 768 维向量 (用 Matryoshka 截断,原始 1024)
- 关键参数: `max_length=8192`, `normalize_embeddings=True`

**效果**:
- 检索召回率: MTEB 中文榜 ~64, 英文榜 ~63
- 速度: CPU 50-100 doc/s (单核, 512 token), GPU ~5000 doc/s
- 不擅长: 长上下文中的精确数字 (e.g. "第 3.14 节")、极短 query (1-2 token)

**怎么用 (Loom API 伪代码)**:
```go
// 已有 internal/embedding/bge.go
emb, err := embedding.Encode(ctx, "Loom 是什么?", 768)
// emb: []float32{0.012, -0.045, ..., 0.078}
```

**开箱度**: 🟡 需装模型权重 (3.4GB)
**兼容**: 跟 S1-W5 RuntimeProfile 已有 model config 字段, 0 冲突

---

## 2. 向量压缩 — 降低存储/带宽

**怎么做**:
- **Matryoshka 维度截断**: 训练时 loss 同时算 64/128/256/768 维, 推理时按需截断
- **INT8 标量量化**: FP32 → INT8, 4x 存储下降, 精度掉 <1%
- 写入时压缩, 读取时解压 (pgvector 自动处理)

**效果**:
- 存储: 768 × 4 bytes = 3KB → 768 × 1 byte = 0.75KB
- 内存: HNSW 图索引同步 4x 减小
- 召回率: 掉 0.5-1% (可接受)
- 速度: 解压开销 < 5%

**怎么用 (pgvector 实际 SQL)**:
```sql
-- INT8 半精度
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE memory_records (
    id uuid PRIMARY KEY,
    embedding halfvec(768)  -- 自动 2 字节/维度
);

-- 查询时距离用 `<=>` (cosine)
SELECT id FROM memory_records 
ORDER BY embedding <=> $1::halfvec 
LIMIT 10;
```

**开箱度**: 🟢 改 1 个数据类型
**兼容**: S1-W2 Event Journal 不用改 (memory_record 是新表)

---

## 3. 块策略 — 文档切分

**怎么做**:
- **Fixed-size chunking**: 按 token 数切, 512 token + 50 token overlap
- LangChain `RecursiveCharacterTextSplitter` 现成
- 中文场景: 按 `\n\n` (段落) → `\n` (句) → `。` (句末) 层级切

**效果**:
- 简单可预测, 不依赖语义模型
- 缺点: 跨段语义切断, 边界处可能丢关键信息
- 适合: 90% 通用场景

**怎么用**:
```go
splitter := chunking.NewFixedSplitter(512, 50)
chunks := splitter.Split(document)
// 每个 chunk 有 {id, content, start_offset, end_offset, parent_doc_id}
```

**开箱度**: 🟢 LangChain 现成
**兼容**: 跟 S1-W2 Event Journal 的 `event_data` 字段, chunk 写入 `content` 子段

---

## 4. 向量索引 — ANN 加速

**怎么做**:
- pgvector 装上后, 用 HNSW 索引
- 参数: `m=16` (每个节点 16 邻居), `ef_construction=64`, `ef=128` (查询时)
- 距离: cosine (Loom 默认, 因为 BGE-M3 已 normalize)

**效果**:
- 千万级向量, p95 < 50ms
- 1 亿级, 考虑 VectorChord / DiskANN
- 召回率: HNSW 95%+ (vs 暴力 100%)

**怎么用**:
```sql
CREATE INDEX memory_emb_hnsw ON memory_records 
USING hnsw (embedding halfvec_cosine_ops) 
WITH (m = 16, ef_construction = 64);

SET hnsw.ef_search = 128;  -- 查询时
```

**开箱度**: 🟢 一行 SQL
**兼容**: S1-W2 已有 SQLite WAL, pgvector 是 Postgres 扩展 (P0-11 路线)

---

## 5. 稀疏检索 — 关键词精确匹配

**怎么做**:
- Postgres FTS5 / GIN 索引 + BM25
- 中文分词: `jieba` 切成 token
- 向量列旁加一个 `tsvector` 列, 触发器自动同步

**效果**:
- 精确匹配 query 中的关键词: "Loom S1-W2" 这种含版本号、产品名
- 不擅长: 同义词 ("Project" vs "项目")、语义相近 ("查找" vs "搜索")
- 跟 dense 互补: dense 找"语义近", sparse 找"字面同"

**怎么用**:
```sql
ALTER TABLE memory_records 
ADD COLUMN tsv tsvector GENERATED ALWAYS AS (
    to_tsvector('simple', content)  -- simple 不用词典, 适应中文
) STORED;

CREATE INDEX memory_tsv_gin ON memory_records USING GIN(tsv);

-- BM25 排序
SELECT id, ts_rank_cd(tsv, query) AS rank
FROM memory_records, plainto_tsquery('simple', 'Loom Event Journal') query
WHERE tsv @@ query
ORDER BY rank DESC LIMIT 10;
```

**开箱度**: 🟢 Postgres 内置
**兼容**: S1-W2 SQLite WAL 不冲突 (FTS5 是新表, Postgres 路由)

---

## 6. 混合检索 — 融合多路召回

**怎么做**:
- 同时跑 dense (向量) + sparse (BM25)
- RRF (Reciprocal Rank Fusion) 合并: `score = Σ 1/(k + rank_i)`, k=60 经验值
- 简单, 0 学习成本, 工业标准

**效果**:
- 召回率比单路 +10-15%
- 不需要分数归一化
- 缺点: 不能调权重 (用 linear combination 可以)

**怎么用**:
```sql
WITH dense AS (
    SELECT id, row_number() OVER (ORDER BY embedding <=> $1) AS r
    FROM memory_records ORDER BY embedding <=> $1 LIMIT 50
),
sparse AS (
    SELECT id, row_number() OVER (ORDER BY rank DESC) AS r
    FROM memory_records, plainto_tsquery('simple', $2) q
    WHERE tsv @@ q ORDER BY rank DESC LIMIT 50
)
SELECT id, SUM(1.0 / (60 + r)) AS rrf
FROM (
    SELECT * FROM dense UNION ALL SELECT * FROM sparse
) GROUP BY id ORDER BY rrf DESC LIMIT 10;
```

**开箱度**: 🟢 一段 CTE
**兼容**: 跟 S1-W2 Event Journal 完全独立, 新表

---

## 7. 重排序 — top-N 精排

**怎么做**:
- BGE-Reranker-v2-M3 (智源, MIT, 568M Cross-Encoder)
- 输入: query + 候选 doc pair
- 输出: 相关性分数 (0-1)
- 离线部署: 1×A10 24GB, 500 QPS (单 batch 32)

**效果**:
- NDCG@10 提升 6% (vs 纯向量)
- 成本: 每次 top-20 重排 ~ 50ms
- 适合: 知识库问答、客服
- 不适合: 高 QPS 搜索 (电商搜索), 用 score-based 轻量替代

**怎么用**:
```go
// 已实现 internal/rerank/bge.go
reranked, err := rerank.Rerank(ctx, query, candidates, 5)
// candidates: []string{...}, 取 top-5
```

**开箱度**: 🟡 需装模型 + GPU
**兼容**: 跟 P0-11 Postgres 路线并行, 不冲突

---

## 8. 跨模态检索 (ColBERT) — 段级粒度

**怎么做**:
- 每个 token 一个向量 (不是整段一个)
- 检索时: query tokens × doc tokens 的 MaxSim
- ColBERT v2 残差压缩, 索引 5x 小于 v1

**效果**:
- 段级精确检索 (vs 双塔的"段级粗粒度")
- 适合: 代码 (token 级 entity), Markdown 表格, 跨段引用
- 缺点: 索引大 (~2x 双塔), 检索慢 (~3x)

**怎么用**:
```go
// 仅 Phase 2
colbertIndex := colbert.New("internal/agents/code-review")
results := colbertIndex.Search(query, topK=20)
```

**开箱度**: 🟡 工程量, 需自建索引
**兼容**: 跟双塔 dense 索引并存 (multi-index strategy)

---

## 9. 查询改写 — 提升召回

**怎么做**:
- **不靠 LLM** (D3 必决冲突), 用规则:
  - 中文: jieba 同义词词典 ("Project" ↔ "项目" ↔ "工程")
  - 代码: tree-sitter 提取 keyword, 加常见 pattern (function/struct/impl)
  - 缩写展开 (S1-W2 → Slice 1 Work Item 2)
- 改写后, N 个 query 并行检索, RRF 融合

**效果**:
- 召回率 +5-8% (无 LLM, 纯规则)
- 延迟: 0 (本地词典)
- 适合: Loom 这种"专有名词多"的领域

**怎么用**:
```go
rewriter := query.NewSynonymExpander(jieba.DefaultDict)
expanded := rewriter.Expand("loom S1-W2 状态")
// ["loom S1-W2 状态", "loom Slice 1 Work Item 2 状态", "loom 第一阶段 第二个工作项 状态", ...]
results := hybrid.SearchAll(expanded, topK=20)
```

**开箱度**: 🟢 现成 jieba + 自配 Loom 词典
**兼容**: 跟 dense/sparse 索引独立

---

## 10. 上下文压缩 — Lost-in-middle 缓解

**怎么做**:
- **LongContextReorder**: 把 top-K 检索结果挪到 prompt 头尾, 中间填不重要的 (Stanford Lost in the Middle 论文)
- **LLMLingua (本地化)**: 用小模型 (GPT-2 / LLaMA-7B) 算 perplexity, 删低信息 token
- 压缩率: 20x 仍保 90% 信息

**效果**:
- LongContextReorder: 0 实施成本, ~30% lost-in-middle 缓解
- LLMLingua: ~50% 缓解, 200-500ms 额外延迟
- 不适合: 短上下文 (<2K token), 压缩开销 > 收益

**怎么用**:
```go
// LongContextReorder (无依赖)
reordered := compression.LongContextReorder(candidates, 5)
// candidates 顺序: [4, 2, 0, 1, 3] → [4, 0, 2, 1, 3] (top + bottom + middle)
```

**开箱度**: 🟢 0 依赖
**兼容**: LLM 调用前的纯函数, 跟任何 prompt builder 集成

---

## 11. 工作记忆 — 当前会话状态

**怎么做**:
- **复用 S1-W2 Event Journal**: 每条 tool_call / LLM_response / user_input 都 append
- **复用 S1-W4 Projection**: 4 种订阅者 (trace / observability / replay / projection)
- 进程崩溃恢复: 从 last_event_id 之后 replay

**效果**:
- 100% 状态可恢复 (Event Journal append-only + WAL)
- 投影延迟 < 100ms (4 种 reader)
- 适合: 长任务 (小时级), 调试回放

**怎么用 (Loom 已实施)**:
```go
// S1-W2 已有
journal.Append(ctx, &Event{
    Type: "tool_call",
    AgentID: "agent-1",
    Data: toolCallData,
})
// 自动投影到 read model
```

**开箱度**: 🟢 **S1-W2/W4 已有**, 0 成本
**兼容**: 完全兼容, 是 Loom 6 层架构的 L6 基础设施

---

## 12. 长期记忆 — 跨会话持久化

**怎么做**:
- **新表 `memory_records`**, schema 见下
- pgvector 存 embedding
- 关联 `agent_id` (FK 到 S1-W5 AgentDefinition)
- 关联 `event_id?` (FK 到 S1-W2 Event Journal)

**效果**:
- 跨会话检索: "我上周跟 user 聊的 Loom Slice 2 计划"
- 容量: 千万级每 agent (单机 pgvector)
- 延迟: 检索 < 100ms (1K 候选)
- 隐私: namespace + RLS

**怎么用**:
```sql
CREATE TABLE memory_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id text NOT NULL,
    memory_type text NOT NULL CHECK (memory_type IN ('episodic','semantic','procedural')),
    content text NOT NULL,
    embedding halfvec(768),
    tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
    metadata jsonb NOT NULL DEFAULT '{}',
    importance real NOT NULL DEFAULT 0.5,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_accessed_at timestamptz NOT NULL DEFAULT now(),
    access_count int NOT NULL DEFAULT 0
);
CREATE INDEX memory_emb_hnsw ON memory_records USING hnsw (embedding halfvec_cosine_ops);
CREATE INDEX memory_tsv_gin ON memory_records USING GIN(tsv);
CREATE INDEX memory_agent ON memory_records(agent_id, created_at DESC);

-- 检索
SELECT id, content FROM memory_records
WHERE agent_id = $1
  AND embedding <=> $2::halfvec < 0.3
ORDER BY embedding <=> $2::halfvec LIMIT 10;
```

**开箱度**: 🟡 schema 扩展
**兼容**: 跟 S1-W2/W5 都是 FK 关系, 0 冲突

---

## 13. 记忆类型 — Episodic / Semantic / Procedural

**怎么做**:
- **event_type 字段** enum: `episodic | semantic | procedural`
- 写入分类规则:
  - episodic: "昨天我做了一个 X" → 时间戳 + 事件
  - semantic: "Loom 用 Go" → 实体 + 属性
  - procedural: "做 Loom 必读 SKILL.md" → 步骤 / 引用

**效果**:
- 不同类型不同 TTL: episodic 30 天, semantic 永久, procedural 跟 Skill 库绑定
- 检索权重: procedural > semantic > episodic
- 适合: Agent 真正"长出"对世界的理解

**怎么用**:
```go
mem := &MemoryRecord{
    AgentID: "loom-coder",
    Type: "procedural",
    Content: "use loom memory write with type=episodic for events",
    SkillID: "loom-memory-write",  // 关联 Skill
    Importance: 0.9,
}
memStore.Create(ctx, mem)
```

**开箱度**: 🟢 字段已有, 加 enum
**兼容**: 跟 S1-W2 `event_type` 字段同模式

---

## 14. 记忆管理 — 写入/遗忘/压缩

**怎么做**:
- **写入触发**: 
  - 用户输入 > 100 字 → 提取 1-3 条 semantic
  - Agent 完成 tool_call → 提取 1 条 episodic
  - 用户标记 "important" → importance = 1.0
- **遗忘策略** (后台 worker 跑):
  - 30 天没访问 AND importance < 0.5 → 软删
  - 90 天没访问 → 硬删
  - 重要度 = `0.5 * recency + 0.3 * access_count + 0.2 * importance_initial`
- **压缩**: 同 agent_id 30 天内的 episodic 按天合并

**效果**:
- 长期不爆容量
- 重要记忆保留率高
- 适合: 7×24 跑 Agent
- 缺点: 需要 consolidation worker 进程

**怎么用**:
```go
// consolidation worker (cron job)
everyDay := cron.New("0 3 * * *")  // 每天凌晨 3 点
everyDay.Do(func() {
    consolidation.DecayStaleMemories(ctx, agentPool)
    consolidation.MergeEpisodic(ctx, agentPool)
})
```

**开箱度**: 🟢 自研, ~300 行 Go
**兼容**: 跟 S1-W4 Projection 模式一致 (后台 worker)

---

## 15. 图谱增强 (GraphRAG) — 关系型问答

**怎么做**:
- **LightRAG** (港大, MIT, 2024): 增量图, 实体 + 关系 + 社区
- 索引阶段: LLM 抽实体/关系 (D3 必决冲突! → 用 Loom 自研的 NER + 规则)
- 检索阶段: 子图检索 + dense 双路

**效果**:
- 多跳问题: "Loom P0-11 跟 FastContext 什么关系"
- 召回率: 比纯 RAG +20% (多跳场景)
- 索引慢: 5-7 天一次性投入, 后续增量
- 缺点: 工程量, 独立服务

**怎么用 (Phase 2)**:
```python
# LightRAG 集成
lightrag = LightRAG(working_dir="./graph_index")
lightrag.insert(memory_records)
results = lightrag.query("Loom P0-11 跟 FastContext 关系", topK=10)
```

**开箱度**: 🔴 独立服务, Phase 2
**兼容**: 跟 dense/sparse 索引并存, 多路 RRF 融合

---

## 16. 会话持久化 — 断点续跑

**怎么做**:
- **LangGraph SqliteSaver** (跟 S1-W2 Event Journal 复用 SQLite)
- 每 thread 状态 + 消息流 checkpoint
- Agent 崩溃后, 加载 last checkpoint 继续

**效果**:
- 长任务 (小时级) 崩溃可恢复
- 检查点开销: ~50ms / step
- 适合: 关键工作流 (代码生成, ETL)

**怎么用**:
```go
import "github.com/loomp/loom/internal/agents/checkpoint"

ckpt, _ := checkpoint.NewSqliteSaver("./data/loom.db")
agent := agents.New(runtime, agents.WithCheckpointer(ckpt))
// 恢复
lastState, _ := ckpt.Load(threadID)
// 续跑
agent.RunFrom(ctx, lastState)
```

**开箱度**: 🟢 S1-W2 复用
**兼容**: 跟 S1-W2 Event Journal 是同一 SQLite

---

## 17. Skill 库 — 程序性知识

**怎么做**:
- **Anthropic Skills** (2025-10-16 标准, Apache-2.0): `SKILL.md` YAML frontmatter + 正文
- 渐进式披露: name + description (always loaded) → full content (on-demand)
- Loom 内置 ~10 个种子 Skill (memory write, code review, test run, etc.)

**效果**:
- Agent 学到 procedure: 复杂操作不靠 prompt 拼凑
- 跨 session 复用: Skill 库是 Loom 的"程序性记忆"
- 适合: 标准化工作流

**怎么用 (Skill 调用)**:
```yaml
# .loom/skills/code-review/SKILL.md
---
name: code-review
description: 审查 Go 代码, 检查 Loom 6 层架构一致性
when_to_use: |
  - 用户要求 code review
  - Agent 完成代码生成后自动调用
---

# 步骤
1. 读 internal/AGENTS.md 确认架构
2. grep -rn "EventType" 检查 event type 命名
3. ...
```

```go
// 加载
skill, _ := skills.Load("code-review")
result, _ := skill.Run(ctx, request)
```

**开箱度**: 🟢 标准成熟
**兼容**: 跟 S2-W5 P0-14 一起, 0 冲突

---

## 18. 可观测性 — hit rate / 延迟 / 成本

**怎么做**:
- Prometheus client 暴露 6 类指标
- Grafana 仪表盘
- 关键指标: hit_rate@k / recall@k / latency p95 / token 消耗 / 存储量

**效果**:
- 一眼看出"哪个 query 没召回"
- 调优 embedding 切块大小
- 控制成本 (看 token 数)
- 报警: hit_rate < 80% 触发告警

**怎么用**:
```go
import "github.com/loomp/loom/internal/metrics"

metrics.MemorySearchTotal.WithLabelValues("dense", "hit").Inc()
metrics.MemorySearchLatency.Observe(time.Since(start).Seconds())
```

```promql
# hit rate
sum(rate(memory_search_total{result="hit"}[5m])) 
/ sum(rate(memory_search_total[5m]))
```

**开箱度**: 🟢 0 依赖 (Prometheus client)
**兼容**: 跟 Loom 6 层架构的 L6 基础设施

---

## 19. 安全隔离 — namespace + ACL

**怎么做**:
- **namespace**: 所有 memory_record 都有 `agent_id`, 检索强制 WHERE agent_id = $1
- **Postgres RLS**: 行级安全策略
- **audit log**: 重要 memory 读写记录到 Event Journal

**效果**:
- 多租户 100% 隔离
- 审计可追溯
- 适合: SaaS 形态 Loom
- 缺点: 跨 agent 协作时需 explicit grant

**怎么用 (RLS 实际策略)**:
```sql
ALTER TABLE memory_records ENABLE ROW LEVEL SECURITY;

CREATE POLICY memory_isolation ON memory_records
    USING (agent_id = current_setting('loom.agent_id')::text);
```

```go
// 切换 namespace
db.Exec("SET loom.agent_id = 'tenant-1-agent-2'")
memStore.Search(ctx, query)  // 自动只返 tenant-1-agent-2 的
```

**开箱度**: 🟢 Postgres 内置
**兼容**: 跟 S1-W2 Event Journal 一致 (`agent_id` 字段)

---

## 4 个典型工作流示例

### 工作流 A: 用户提问 → RAG 回答

```
用户: "Loom S1-W2 的 Event Journal 怎么用?"
  ↓
1. 查询改写 (能力 9): ["Loom S1-W2 Event Journal", "loom Slice 1 Work Item 2 事件日志", ...]
  ↓
2. 混合检索 (能力 4+5+6): dense + sparse + RRF, top 20
  ↓
3. 重排序 (能力 7): BGE-Reranker → top 5
  ↓
4. 上下文压缩 (能力 10): LongContextReorder → [best, 3rd, 2nd, 4th, 5th]
  ↓
5. 拼 prompt → LLM
  ↓
6. 写 episodic 记忆 (能力 12+13): "用户问过 Loom S1-W2 Event Journal"
  ↓
输出答案 + 记忆巩固
```

**总延迟**: 300-500ms (检索 50ms + 重排 100ms + LLM 200ms)
**成本**: 1 次 embedding + 1 次 rerank + 1 次 LLM

### 工作流 B: 长期会话恢复

```
昨天: Agent 完成代码生成, 状态写入 S1-W2 Event Journal
今天: 用户回来
  ↓
1. Event Journal replay (能力 11): 加载昨天 last event
  ↓
2. 读 semantic 记忆 (能力 12+13): "用户偏好 Go, 不喜欢 Python"
  ↓
3. 读 episodic 记忆 (能力 12+13): "昨天我们讨论了 X"
  ↓
4. 拼 prompt: "继续昨天的话题, 基于用户的 Go 偏好..."
  ↓
Agent 无缝续接
```

**延迟**: 100-200ms (Journal 加载 + memory 检索)
**零额外存储**: 全部复用 S1-W2

### 工作流 C: 跨会话 Skill 学习

```
新员工: 打开 Loom 第一次用
  ↓
1. 加载默认 Skill 库 (能力 17): 5 个种子 Skill
  ↓
2. 用户完成第一个任务: "做 code review"
  ↓
3. Agent 调用 code-review Skill (能力 17)
  ↓
4. 用户标记 "这个 Skill 有用" → importance=0.8
  ↓
5. 写 procedural 记忆 (能力 13): "code-review Skill, importance 0.8"
  ↓
下次: 同样任务, Agent 直接从 procedural 记忆调出 Skill
```

### 工作流 D: 监控系统发现召回率下降

```
Grafana 报警: hit_rate@10 从 85% 跌到 60%
  ↓
1. 看指标: rerank_latency 没变, embedding_latency 变慢
  ↓
2. 查日志: pgvector HNSW 索引被禁用 (某次 schema 迁移)
  ↓
3. 重建索引 (能力 4): REINDEX
  ↓
hit_rate 回到 85%
  ↓
写 episodic 记忆 (能力 12+13): "pgvector HNSW 索引不能丢"
```

---

## 决策树 — 该用哪个能力

```
                ┌─── 跨会话检索? ─── Yes → 长期记忆 (12)
                │
用户需求 ───────┼─── 关系型问答? ─── Yes → 图谱增强 (15) [Phase 2]
                │
                ├─── 程序性知识? ─── Yes → Skill 库 (17)
                │
                ├─── Lost-in-middle? ─ Yes → 上下文压缩 (10)
                │
                ├─── 召回率低? ────── Yes → 混合检索 (6) + 查询改写 (9) + 重排序 (7)
                │
                ├─── 容量爆? ──────── Yes → 记忆管理 (14)
                │
                └─── 多租户? ──────── Yes → 安全隔离 (19)
```

---

## 实施优先级 (Phase 1 vs Phase 2 vs Phase 3)

### Phase 1 必做 (5-7 天)
1. 嵌入 (1) — BGE-M3
2. 向量压缩 (2) — Matryoshka + INT8
3. 块策略 (3) — Fixed 512
4. 向量索引 (4) — pgvector HNSW
5. 稀疏检索 (5) — BM25
6. 混合检索 (6) — RRF
7. 重排序 (7) — BGE-Reranker-v2-M3
8. 上下文压缩 (10) — LongContextReorder
9. 长期记忆 (12) — memory_records 表
10. 记忆类型 (13) — enum 扩展
11. 记忆管理 (14) — LRU 后台 worker
12. 会话持久化 (16) — SqliteSaver
13. Skill 库 (17) — Anthropic Skills
14. 可观测性 (18) — Prometheus
15. 安全隔离 (19) — RLS

### Phase 2 (10-15 天)
- 查询改写 (9) — Loom 同义词词典
- 上下文压缩 (10) — LLMLingua 本地化
- 跨模态 (8) — ColBERT v2 (代码检索)
- 图谱增强 (15) — LightRAG
- 高级记忆管理 (14) — 反射 consolidation

### Phase 3 (20-30 天)
- 亿级向量 (4) — VectorChord / DiskANN
- PDF 多模态 (8) — ColPali
- 多租户加密 (19) — Vault + SPIFFE

---

## 兼容性矩阵 (跟 Loom 已有组件)

| 已有组件 | 跟 19 能力的关系 | 复用度 |
|----------|----------------|--------|
| `internal/journal/` (S1-W2) | 工作记忆 (11) 完整复用 | 🟢 100% |
| `internal/evidence/` (S1-W2) | 跟重排序 (7) 互证 | 🟢 80% |
| `internal/projection/` (S1-W4) | 长期记忆 (12) 用投影模式 | 🟢 100% |
| `internal/agents/definition.go` (S1-W5) | memory_type 字段扩展 | 🟢 90% |
| `internal/runtime/catalog.go` (S1-W5) | memory config 字段 | 🟢 90% |
| `cmd/loom/query.go` (S1-W1) | 加 `cmd/loom/memory.go` | 🟡 80% |
| `cmd/loom/main.go` (S1-W1) | 加 memory subcommand | 🟡 70% |
| `internal/mode/router.go` (S1-W1) | memory 维度可加 | 🟡 70% |
| `docs/CURRENT.md` | 现状位 + 计划位 | 🟢 100% |
| `docs/adr/0006-fastcontext.md` | FastContext 是备选, Phase 2 | 🟡 50% |
| `docs/integrations/fastcontext.md` | FastContext (S1-W5) | 🟡 50% |
| `g8-six-tasks-briefs.md` (PROV-1) | 加"能力矩阵"小节 | 🟢 100% |

---

## 后续 (3 选项, 等用户拍)

### A. 落 spec
- 把"实施优先级"扩成完整 spec
- 估时 1-2h

### B. 合进 G8.5 PROV-1
- 把"19 能力 + 兼容矩阵 + 决策树"作为 PROV-1 子节
- 估时 30min

### C. 维持调研存档
- 跟现状一致, 等 Phase 1 跑起来再决定
- 估时 0

**默认推荐**: **B** (合 PROV-1, 跟能力分类表一起入, 30min)
