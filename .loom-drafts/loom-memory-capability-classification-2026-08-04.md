# Agent 记忆 / 外挂存储 — 按技术能力分类

> **日期**: 2026-08-04
> **作者**: Mavis (web_search 4 轮 + 已落 G8.5 PROV-1 调研)
> **状态**: DRAFT (待用户拍: 落工作队列 / 合进现有 work item / 维持调研)
> **范围**: 按"技术能力"维度分类,不是按"产品/服务"分类
> **Loom 上下文**: D3 必决 = 拒代理 LLM / 拒 SaaS 记忆; L1-L6 6 层架构只占 4 层 (L1/L2/L4/L6); 8 invariant 仍生效
> **承接**: 上轮 (`loom-audit-research-vs-workitem-2026-07-25.md`) 调研 5 大类 SaaS/产品类目 (LangGraph checkpointer / pgvector / Qdrant / Milvus / Anthropic Skills), 本轮按 19 个技术能力维度下钻, 给出每个能力的"代表实现 + License + Loom 兼容性 + 落点 + 估时"

---

## TL;DR

| 维度 | 数量 | Loom P0 (直接吸) | Loom P1 (条件吸) | Loom 不吸 (D3 / SaaS / 复杂度) |
|------|------|-----------------|-------------------|--------------------------------|
| 嵌入层 | 1 | Matryoshka + BGE-M3 | OpenAI text-embed-3 | Instructor (API-only) |
| 向量压缩 | 1 | PQ (Loom 暂不吸) | 4/8-bit 二值化 | 纯 SQ (精度不够) |
| 块策略 | 1 | Fixed (FixedSizeChunker) | Semantic chunking (Slumber/Meru) | Late chunking (Jina 商业) |
| 向量索引 | 1 | HNSW (pgvector / VectorChord) | DiskANN / ScaNN | IVF (亿级再考虑) |
| 稀疏检索 | 1 | BM25 倒排 (FTS5 / Tantivy) | SPLADE | ColBERT v2 (单卡) |
| 混合检索 | 1 | RRF (Convex / Weaviate) | Linear / Convex | 自研 fusion |
| 重排序 | 1 | BGE-Reranker-v2-M3 (MIT) | Bocha Semantic (国内) | Cohere (闭源) / Jina (闭源) |
| 跨模态检索 | 1 | (Phase 2 文本+代码) | ColPali (PDF 图) | 多向量全自研 |
| 查询改写 | 1 | HyDE (无 LLM 改写) | Step-Back / Multi-Query (需 Loom 不学的 LLM, 否) | LLM 改写 (D3 冲突) |
| 上下文压缩 | 1 | LLMLingua (prompt 注入过滤) | LLMLingua-2 / RECOMP | ICAE / 500xCompressor (需重训) |
| 工作记忆 | 1 | Event Journal + Projection (S1-W2/W4 已有) | — | Redis / Memurai (本地化) |
| 长期记忆 | 1 | pgvector (P0-11 路线) | Qdrant | Mem0 / Letta / Cognee (D3) |
| 记忆类型 | 1 | Episodic + Semantic + Procedural (自分类) | Skill (程序性) | Graphiti (复杂度) |
| 记忆管理 | 1 | LRU + 时间窗口 + 重要度 (自研) | Generative Agents 反射 | MemGPT 层级 |
| 图谱增强 | 1 | (Phase 2 实验) | HippoRAG / LightRAG | Microsoft GraphRAG (巨慢) / GFM-RAG |
| 会话持久化 | 1 | LangGraph Checkpointer SQLite (S1-W2 复用) | Postgres checkpointer | — |
| Skill 库 | 1 | Anthropic Skills (SKILL.md, 2025-10-16) | — | Cloudflare Skills (SaaS) |
| 可观测性 | 1 | Hit rate + latency + token 计数 (自建) | Langfuse | LangSmith (SaaS) |
| 安全隔离 | 1 | namespace + ACL (Event Journal 已有) | — | 多租户加密 (Phase 2) |

---

## 1. 嵌入 (Embedding) — 文本/代码 → 向量

**代表实现**:
- **BGE-M3** (BAAI/智源, MIT, 568M, 100+ langs, 8K context, dense+sparse+multi-vector 三合一)
- **OpenAI text-embedding-3** (闭源 API, 3072 dim 可截断)
- **Nomic Embed v1.5** (Apache-2.0, 137M, 8K context, Matryoshka 支持)
- **Cohere embed-v3** (闭源 API)
- **SFR-Embedding-Mistral** (闭源 research, Salesforce)

**License 倾向**: BGE-M3 / Nomic / Instructor 都开源可商用 (Apache/MIT); OpenAI / Cohere 闭源且计 token。

**Loom 兼容性**:
- ✅ **P0**: BGE-M3 + Matryoshka 维度截断 (一层模型多粒度) — 跟 P0-11 DBOS Postgres 路线一致, 0 网费
- ⚠️ **P1**: OpenAI text-embed-3 只在多语种边缘 case 兜底
- ❌ **不吸**: Instructor (API-only, 跟"本地 + 自治" 冲突), Cohere (闭源), SFR-Embedding (未公开)

**落点**: L2 数据层 (projection + 索引)
**估时**: 1-2 天 (单模型集成 + benchmark)

---

## 2. 向量压缩 (Vector Quantization) — 降低存储/带宽

**代表实现**:
- **Product Quantization (PQ)**: 把 d 维向量拆成 m 段, 每段量化为 k 个 centroid, 压缩比 8-32x
- **Scalar Quantization (SQ)**: FP32 → INT8, 4x 压缩
- **Binary Quantization**: 浮点 → 1-bit, 32x 压缩但精度损失大
- **Matryoshka** (维度截断, 不是真压缩): 1024/512/256/128 dim 同向量, 检索按需用子段
- **VectorChord 1.1** (2026-01): Postgres 原生 4/8 bit 向量类型, 1.0 版本支持 RaBitQ-empowered DiskANN

**Loom 兼容性**:
- ✅ **P0**: **Matryoshka** + 标量 INT8 (精度可接受, 4x 存储)
- ⚠️ **P1**: PQ / Binary 留 Phase 2 备份索引, 千亿级再上
- ❌ **不吸**: 纯 SQ INT4 (MTEB 掉点 >3%)

**落点**: L2 数据层 (写时压缩, 读时解压)
**估时**: 2-3 天 (跟 pgvector / Qdrant 集成)

---

## 3. 块策略 (Chunking) — 文档 → 检索单元

**代表实现**:
- **Fixed-size chunking**: 按 token / char 切 (e.g. 512 token + 50 token overlap), 最简单最稳
- **Recursive character splitting**: 按 `\n\n` → `\n` → ` ` 层级切, 保留段落结构 (LangChain 默认)
- **Semantic chunking**: 用 embedding 计算相邻段相似度, 相似度低就切 (gregkamradt/Mercury)
- **Parent-Child / Small-to-big**: 小块检索, 拿回大块 (LangChain ParentDocumentRetriever)
- **Late chunking** (Jina, 商业): 全文先 embed, 再按段分, 上下文不丢
- **Code-aware chunking**: 按 AST 切 (function / class), tree-sitter / langchain-text-splitters

**Loom 兼容性**:
- ✅ **P0**: Fixed-size (512 token + 50 overlap) 作为默认
- ⚠️ **P1**: Semantic chunking (slumber) / Code-aware (按需)
- ❌ **不吸**: Late chunking (Jina 商业闭源)

**落点**: L2 数据层 (ingest 阶段)
**估时**: 1 天 (默认 + 2-3 个 override 切法)

---

## 4. 向量索引 (Vector Index) — 加速 ANN 检索

**代表实现**:
- **HNSW** (Hierarchical Navigable Small World): 图索引, 召回率高, 内存占用大, ms 级延迟
- **IVF** (Inverted File): 聚类分桶, 内存友好, 召回率略低
- **ScaNN** (Google): 量化版 HNSW, 工业级
- **DiskANN** (Microsoft): 磁盘版 ANN, 10x 内存节省
- **pgvector HNSW**: 跟 Postgres 集成, 0 运维
- **VectorChord 0.5**: RaBitQ + DiskANN + 1B 向量 2 小时索引
- **Qdrant HNSW + Product Quantization**: Rust 自研, 写优化
- **Milvus**: IVF/HNSW/DiskANN 全支持, 大规模首选

**License**: pgvector / Qdrant / Milvus / VectorChord 都开源 (Apache-2.0)

**Loom 兼容性**:
- ✅ **P0**: **pgvector HNSW** (跟 P0-11 DBOS Postgres 路线, 千万级足够)
- ⚠️ **P1**: VectorChord (亿级, 2026 出来)
- ❌ **不吸**: 单独 Milvus 集群 (运维成本, 千万级用不上)

**落点**: L2 数据层 (索引)
**估时**: 0.5 天 (pgvector HNSW 配置)

---

## 5. 稀疏检索 (Sparse Retrieval) — 关键词精确匹配

**代表实现**:
- **BM25 / BM25F**: 经典 TF-IDF 改进, 词频饱和 + 文档长度归一化
- **SPLADE** (Sparse Lexical and Expansion Model): 用 BERT 学稀疏扩展, 召回率 +15-20%
- **BGE-M3 sparse**: BGE-M3 内置稀疏表示, 跟 dense 共享模型
- **ColBERT v2** (Stanford): 上下文感知 late interaction, 多向量, 精度逼近 cross-encoder

**License**: BM25 是算法, 无 license; SPLADE / BGE-M3 / ColBERT 都开源

**Loom 兼容性**:
- ✅ **P0**: **BM25 倒排** (Postgres FTS5 或 Tantivy) + BGE-M3 sparse (双索引零额外成本)
- ⚠️ **P1**: SPLADE (精度优先时)
- ❌ **不吸**: ColBERT 全自研 (工程量, Phase 3+)

**落点**: L2 数据层 (跟 dense 索引并列)
**估时**: 1-2 天 (Postgres FTS5 + GIN 索引)

---

## 6. 混合检索 (Hybrid Search) — 融合多路召回

**代表实现**:
- **RRF (Reciprocal Rank Fusion)**: 倒数排名融合, 无需分数归一化, 工业标准
- **Linear combination**: 加权求和, 需要分数归一化
- **Convex combination**: 凸组合, RRF 的概率解释版
- **DPR (Dense Passage Retrieval)**: Facebook 2020, 经典双塔
- **Tie-breaker by score**: RRF 排序后再用原始 score 微调

**Loom 兼容性**:
- ✅ **P0**: **RRF** (Postgres pgvector + FTS5 双路)
- ❌ **不吸**: 自研 fusion 算法 (RRF 已经够用)

**落点**: L2 数据层 (查询合并层)
**估时**: 0.5 天 (RRF 函数 + benchmark)

---

## 7. 重排序 (Reranker) — 召回后精排

**代表实现**:
- **BGE-Reranker-v2-M3** (智源, MIT, 568M, Cross-Encoder, 100+ langs, 8K context)
- **BGE-Reranker-v2-MiniCPM-layerwise** (MIT, 2B, 支持层间 early-exit)
- **BGE-Reranker-v2-Gemma-2B** (MIT, 2B, 多语言更强)
- **Cohere Rerank 3** (闭源 API, $2/1K search, 4K context)
- **Jina Reranker v2** (闭源 API, 100+ langs, 6x v1)
- **Bocha Semantic Reranker** (国内, 80M, $0.001/search, 中文优化)
- **BCE-Reranker** (有道, 开源)

**Loom 兼容性**:
- ✅ **P0**: **BGE-Reranker-v2-M3** (MIT, 离线部署, 跟 S1-W5 国产化路径一致)
- ⚠️ **P1**: Bocha (国内用户请求时降级)
- ❌ **不吸**: Cohere / Jina (闭源 API, $ 计费 + 海外网络)

**落点**: L2 数据层 (精排阶段)
**估时**: 1 天 (单 GPU 部署 + benchmark)

---

## 8. 跨模态检索 (Late Interaction / 多向量)

**代表实现**:
- **ColBERT** (Stanford, 2020, MIT): 每个 token 一个向量, MaxSim 计算
- **ColBERT v2** (2022, MIT): 残差压缩 + 端到端, 性能 5x 提升
- **Jina-ColBERT-v2** (Apache-2.0, 多语言)
- **ColPali / ColQwen** (Visual RAG, 文档图像): PDF 转图像, 1024 个 patch 向量

**Loom 兼容性**:
- ⚠️ **P1**: ColBERT v2 用于代码 / Markdown 跨段检索
- ❌ **不吸**: ColPali (PDF 图检索是 Phase 3 才考虑的事)

**落点**: L2 数据层 (代码段级检索)
**估时**: 3-4 天 (ColBERT 集成 + 索引压缩)

---

## 9. 查询改写 (Query Rewriting) — 提升召回

**代表实现**:
- **HyDE** (Hypothetical Document Embeddings, CMU 2022, MIT): 用 LLM 生成假设答案, 用答案 embed 检索
- **Multi-Query**: LLM 生成 N 个改写, 并行检索后 RRF 融合
- **Step-Back Prompting**: LLM 先回答抽象问题, 再回答具体问题
- **Query2Doc**: 同 HyDE 但 prompt 更结构化
- **Rewrite-Retrieve-Read** (Microsoft 2023, MIT): 反馈循环, 用 LLM 改写检索失败的查询

**Loom 兼容性**:
- ✅ **P0**: **HyDE 的"无 LLM 改写"变体** — 用 query expansion (同义词/同义短语) 替代 LLM 生成
- ❌ **不吸**: 原版 HyDE / Multi-Query / Step-Back (都靠 LLM, D3 必决冲突)

**Loom 替代方案**:
- 用 `jieba` / `hanlp` 做 query expansion (同义词 + 实体识别)
- 用 code-aware 改写 (识别 code keyword, 加常见 pattern)
- 用规则改写 (下划线↔驼峰、缩写展开)

**落点**: L2 数据层 (查询入口)
**估时**: 1-2 天 (query expansion + RRF)

---

## 10. 上下文压缩 (Context Compression) — 解决"迷失在中间"

**代表实现**:
- **Lost in the Middle** (Stanford 2023, MIT): 证明长上下文中段召回率 < 边缘
- **LLMLingua** (Microsoft 2023, MIT): 用小模型 (GPT-2 small / LLaMA-7B) 删冗余 token, 20x 压缩
- **LongLLMLingua** (Microsoft 2024, MIT): 长上下文版, 文档重排序 + 子序列恢复
- **SelectiveContext** (2023, MIT): 用自信息 (self-information) 删冗余, 无外部模型
- **RECOMP** (2024, MIT): 双编码器压缩文档为摘要向量
- **LongContextReorder** (LangChain): 把关键文档挪到首尾, 简单但有效 (lost-in-middle 缓解)
- **LongAttnComp** (SambaNova 2026): cross-family 压缩器, Llama-3.1-8B 训练, 适配 DeepSeek/Qwen/Mistral
- **C3-Context-Cascade-Compression** (2025-11): 双 LLM 级联, 32 token latent 压缩 20x
- **ICAE / 500xCompressor** (Princeton, MIT): prompt → 32/64/128 latent tokens, 需重训编码器

**Loom 兼容性**:
- ✅ **P0**: **LongContextReorder** (零依赖, 挪到首尾即可, 0 实施成本)
- ⚠️ **P1**: **LLMLingua** (用本地 BGE-M3 做 perplexity 判别) + LongAttnComp
- ❌ **不吸**: ICAE / 500xCompressor (需重训编码器, 跟 Loom "数据流透明" 哲学冲突)

**落点**: L2 数据层 (LLM 调用前)
**估时**: 1 天 (LongContextReorder) + 3 天 (LLMLingua 本地化)

---

## 11. 工作记忆 (Working / Short-term Memory) — 当前会话上下文

**代表实现**:
- **In-context window**: LLM 上下文窗口, 临时性, 容量 100K-1M token
- **Event Journal (Loom S1-W2 已有)**: SQLite WAL, append-only, 已有"事实写入+投影"完整流程
- **Projection (Loom S1-W4 已有)**: read model 重建, 已有 4 种订阅者
- **Redis Streams** (BSD): 消息流 + consumer group
- **Kafka** (Apache-2.0): 分布式 log, 重量级
- **LangGraph Checkpointer** (MIT): 状态 + 消息流持久化

**Loom 兼容性**:
- ✅ **P0**: **Event Journal + Projection** (S1-W2/W4 已是这套, 0 额外成本)
- ❌ **不吸**: Redis (本地化 + 部署), Kafka (重量级)

**落点**: L1 Agent 内 + L6 基础设施 (Event Journal)
**估时**: 0 (已实施)

---

## 12. 长期记忆 (Long-term Memory) — 跨会话持久化

**代表实现**:
- **pgvector** (Postgres, MIT): 跟 P0-11 DBOS Postgres 路线一致, 0 运维
- **Qdrant** (Apache-2.0, Rust): 写优化, 过滤 + 向量一体化
- **Milvus** (Apache-2.0, Go/C++): 亿级, 集群复杂
- **Weaviate** (BSD-3, Go): 模块化, GraphQL 接口
- **Vespa** (Apache-2.0, Java): 工业级, 复杂
- **Chroma** (Apache-2.0, Python): 简单, 性能一般
- **LanceDB** (Apache-2.0, Rust): 列存, 嵌入式友好
- **Mem0** (Apache-2.0): 托管云 API, D3 冲突 (LLM 决策)
- **Letta / MemGPT** (Apache-2.0): 工具调用让 agent 自治记忆, 复杂度高 + LLM 决策
- **LangMem** (MIT): LangChain 集成, LLM 决策
- **Cognee** (Apache-2.0): 知识图谱 + LLM, D3 冲突

**Loom 兼容性**:
- ✅ **P0**: **pgvector** (Postgres, 跟 P0-11 路线一致, 0 额外组件)
- ⚠️ **P1**: Qdrant (亿级时)
- ❌ **不吸**: Mem0 / Letta / Cognee / LangMem (D3 必决冲突, LLM 决策)
- ❌ **不吸**: Milvus 集群 (亿级前不需要)

**落点**: L6 基础设施 (Postgres 扩展)
**估时**: 0 (pgvector 装在 P0-11 阶段就完成)

---

## 13. 记忆类型 (Memory Types) — Episodic / Semantic / Procedural

**代表实现**:
- **Tulving 1972 经典分类**: Episodic (事件) / Semantic (事实) / Procedural (技能)
- **Cloudflare Agent Memory (2026-04)**: 4 型分类 — Fact / Event / Instruction / Task
- **Anthropic Skills (2025-10-16)**: SKILL.md 标准, 程序性知识, 渐进式披露
- **Generative Agents (Stanford 2023)**: Memory Stream + Reflection + Planning
- **RL Agent Memory 分类 (2024-12)**: Long/Short × Declarative/Procedural 4 象限
- **LangMem** (LangChain): Type-tagged memories, semantic + episodic

**Loom 兼容性**:
- ✅ **P0**: **Episodic + Semantic + Procedural 三型** (跟 Event Journal 的 event type 对齐, 已有 `event_type` enum)
- ✅ **P0**: **Anthropic Skills** 作为程序性记忆的载体 (跟 S2-W5 P0-14 一起)

**Loom 数据模型扩展**:
```yaml
memory_record:
  id: uuid
  agent_id: string
  memory_type: episodic | semantic | procedural
  content: text
  embedding: vector(768)
  metadata: {
    event_id?: string,        # 关联 Event Journal
    skill_id?: string,        # 关联 Skill
    source: "user_input" | "reflection" | "tool_result",
    importance: 0-1,
    created_at: timestamp,
    last_accessed_at: timestamp,
    access_count: int,
    ttl?: timestamp
  }
```

**落点**: L2 数据层 (memory schema)
**估时**: 2-3 天 (schema 扩展 + 读写 API)

---

## 14. 记忆管理 (Memory Management) — 写入/遗忘/压缩

**代表实现**:
- **Generative Agents Reflection** (Stanford 2023, MIT): LLM 反思生成高阶 memory
- **MemGPT 层级记忆** (Berkeley 2023, Apache-2.0): main context / outer context / archival
- **LRU + 时间窗口**: 简单淘汰
- **重要度评分**: BM25 + 访问频率 + 衰减
- **Memory consolidation** (人类记忆模型): 短时记忆 → 长时记忆的巩固过程
- **Generative Replay**: 训练时重放, 防遗忘

**Loom 兼容性**:
- ✅ **P0**: **LRU + 时间窗口 + 重要度** 三因素 (自研, 0 外部依赖)
- ❌ **不吸**: Generative Agents 反思 (LLM 决策, D3 冲突)
- ❌ **不吸**: MemGPT 层级 (LLM 工具调用, D3 冲突)

**落点**: L2 数据层 (后台 compaction job)
**估时**: 2-3 天 (consolidation worker)

---

## 15. 图谱增强 (GraphRAG / Knowledge Graph) — 关系型检索

**代表实现**:
- **Microsoft GraphRAG** (MIT, 2024): 实体抽取 → 社区检测 → 摘要, 索引巨慢 (小时级)
- **LightRAG** (MIT, 2024): 增量图, 5x 快于 GraphRAG
- **HippoRAG** (MIT, 2024): 神经科学启发的 PageRank on KG
- **GFM-RAG** (MIT, 2025): Graph Foundation Model, 8M 参数, 跨数据集 zero-shot
- **Neo4j + LLM** (GPL-3.0): 传统图数据库
- **NetworkX** (BSD-3): Python 图算法库

**Loom 兼容性**:
- ⚠️ **P1**: LightRAG (增量, 工程友好) / HippoRAG (神经科学, 优雅)
- ❌ **不吸**: Microsoft GraphRAG (索引巨慢, 工业不行)
- ❌ **不吸**: GFM-RAG (零样本泛化,但工程量 + 评估难)

**落点**: L2 数据层 (关系层, Phase 2)
**估时**: 5-7 天 (LightRAG 集成 + benchmark)

---

## 16. 会话持久化 (Session Persistence / Checkpointer) — 断点续跑

**代表实现**:
- **LangGraph Checkpointer** (MIT, Python): 状态 + 消息流持久化
  - `MemorySaver`: 内存
  - `SqliteSaver`: SQLite (BSD-3, Loom 已有)
  - `PostgresSaver`: PostgreSQL
- **LangGraph Store** (MIT): 长期跨会话存储
- **Claude Agent SDK 状态** (Anthropic): 自动持久化

**Loom 兼容性**:
- ✅ **P0**: **LangGraph Checkpointer SQLite 模式** (跟 S1-W2 Event Journal 复用, 已有 SQLite WAL)
- ⚠️ **P1**: PostgresSaver (P0-11 之后)

**落点**: L1 Agent 内 (per-thread state) + L6 基础设施
**估时**: 0.5 天 (SqliteSaver 配置)

---

## 17. Skill 库 (Skills / Procedural Memory) — 程序性知识

**代表实现**:
- **Anthropic Skills** (2025-10-16 标准, Apache-2.0, 2026 主流): `SKILL.md` + progressive disclosure
- **Cloudflare Skills** (SaaS, 闭源): 托管, D3 冲突
- **SkillsMP / skills.sh** (开源, MIT): 第三方聚合, GitHub 拉取
- **Letta / MemGPT Skills** (Apache-2.0): 跟 Agent 记忆耦合
- **OpenAI GPTs Actions** (闭源): 跟 Loom 平台无关
- **MCP Servers** (MIT, 2024-11): 工具而非 Skill, 不同抽象层

**Loom 兼容性**:
- ✅ **P0**: **Anthropic Skills (SKILL.md)** (开放标准, 跟 S2-W5 P0-14 一起)
- ❌ **不吸**: Cloudflare Skills (SaaS, 本地化冲突)

**Skill 库目录结构**:
```
.loom-drafts/skills/                  # 调研阶段
loom-skills/                          # Phase 1 集成
├── SKILL.md                          # 全局索引
├── skills/
│   ├── code-review/
│   │   ├── SKILL.md                  # YAML frontmatter + 描述
│   │   ├── scripts/
│   │   └── references/
│   ├── test-runner/
│   ├── context-loader/
│   └── ...
```

**落点**: L4 工具层 + L1 Agent 内
**估时**: 2-3 天 (loader + 5 个种子 Skill)

---

## 18. 可观测性 (Observability) — Hit rate / 延迟 / 成本

**代表实现**:
- **OpenTelemetry** (Apache-2.0): 标准 trace/metric
- **Langfuse** (MIT, 2024): 开源 LLM observability, self-host
- **LangSmith** (闭源 SaaS): LangChain 官方, 数据出境
- **Phoenix (Arize)** (ELv2, 排除!): OpenLLMetry 兼容, ELv2 非 OSI
- **Helicone** (Apache-2.0): 代理 + 监控
- **Custom metrics**: Prometheus + Grafana

**Loom 兼容性**:
- ✅ **P0**: **自建指标** (hit rate / latency p50/p95/p99 / token 数 / 召回率), Prometheus 暴露
- ⚠️ **P1**: Langfuse (Phase 2, self-host)
- ❌ **不吸**: LangSmith (SaaS, 出境) / Phoenix (ELv2, MIT 兼容排除)

**关键指标**:
```
loom_memory_metrics:
  embedding_latency_ms: histogram
  vector_search_latency_ms: histogram
  rerank_latency_ms: histogram
  hit_rate@k: gauge(k=1,5,10,20)
  recall@k: gauge
  mrr: gauge
  token_budget: gauge
  memory_write_qps: gauge
  memory_eviction_rate: gauge
  embedding_cache_hit_rate: gauge
```

**落点**: L6 基础设施 (Prometheus + Grafana)
**估时**: 2-3 天 (埋点 + 仪表盘)

---

## 19. 安全隔离 (Security / ACL / Multi-tenancy) — 防泄漏

**代表实现**:
- **Postgres RLS (Row-Level Security)**: 多租户隔离
- **namespace 命名空间**: Event Journal / memory_record 都加 agent_id
- **Vector DB 过滤**: payload filter (Qdrant / Milvus / pgvector 都支持)
- **Vault** (HashiCorp, MPL-2.0): secrets 管理
- **OPA** (Apache-2.0): 策略引擎
- **SPIFFE/SPIRE** (Apache-2.0): 身份认证

**Loom 兼容性**:
- ✅ **P0**: **namespace + ACL + Postgres RLS** (跟 S1-W2 Event Journal 已有 `agent_id` 字段对齐)
- ⚠️ **P1**: OPA (Phase 2, 复杂策略)

**落点**: L6 基础设施
**估时**: 1-2 天 (RLS policy + audit log)

---

## 推荐 Loom 实施路径 (按 Phase 1 → 2 → 3)

### Phase 1 (现状 + 补全, ~5-7 天)
| 工作项 | 估时 | 落点 |
|--------|------|------|
| pgvector HNSW 配置 | 0.5 天 | L6 (P0-11 路线) |
| BM25 倒排 (Postgres FTS5) | 1-2 天 | L2 |
| RRF 混合检索 | 0.5 天 | L2 |
| BGE-M3 + Matryoshka 嵌入 | 1 天 | L2 |
| BGE-Reranker-v2-M3 部署 | 1 天 | L2 |
| LongContextReorder | 0.5 天 | L2 |
| memory schema 扩展 (episodic/semantic/procedural) | 2-3 天 | L2 |
| Anthropic Skills 集成 | 2-3 天 | L4 |
| 基础 observability 指标 | 2-3 天 | L6 |
| namespace + RLS | 1-2 天 | L6 |

### Phase 2 (优化 + 高级, ~10-15 天)
| 工作项 | 估时 | 优先级 |
|--------|------|--------|
| LLMLingua 本地化 | 3 天 | P1 |
| Semantic chunking | 2 天 | P1 |
| ColBERT v2 (代码检索) | 3-4 天 | P1 |
| LightRAG / HippoRAG | 5-7 天 | P1 |
| Generative Agents-style 反思 (无 LLM 版) | 3 天 | P1 |
| 记忆 consolidation worker | 2-3 天 | P1 |
| Langfuse 自托管 | 3 天 | P1 |
| Vector 压缩 (INT8 + Matryoshka) | 2-3 天 | P1 |
| Code-aware chunking (tree-sitter) | 3 天 | P1 |

### Phase 3 (亿级 + 多模态, ~20-30 天)
| 工作项 | 估时 | 优先级 |
|--------|------|--------|
| VectorChord / DiskANN | 3-4 天 | P1 |
| ColPali (PDF 图) | 5-7 天 | P2 |
| 多租户加密 | 5 天 | P1 |
| OPA 策略引擎 | 3-4 天 | P2 |

---

## 不吸 / 待观察 (D3 必决 + 复杂度 + 本地化冲突)

| 项目 | 拒因 | 替代方案 |
|------|------|----------|
| **Mem0** | D3 必决 (LLM 决策记忆) | 自研 reflection worker |
| **Letta / MemGPT** | D3 必决 + 工具调用复杂度 | pgvector + 自研 LRU |
| **LangMem** | D3 必决 (LangChain 集成) | 自建 memory schema |
| **Cognee** | D3 必决 (LLM 决策) | LightRAG 替代 |
| **Graphiti (Zep)** | 复杂度 + 调研薄 | LightRAG / HippoRAG |
| **Cloudflare Agent Memory** | SaaS + 本地化冲突 | 自建 Event Journal + pgvector |
| **Tencent Cloud Agent Memory** | SaaS | 自建 |
| **Cloudflare Skills** | SaaS | Anthropic Skills (自托管) |
| **LangSmith** | SaaS + 出境 | Langfuse 自托管 |
| **Phoenix (Arize)** | ELv2 (排除) | Langfuse (MIT) |
| **9Router / Helicone** | 代理 LLM, D3 必决 | 直连 API |
| **Cohere / Jina (API)** | 闭源 + 跨境 | BGE-M3 / BGE-Reranker |
| **Instructor Embeddings** | API-only | BGE-M3 (开源) |
| **ICAE / 500xCompressor** | 需重训编码器 | LongContextReorder + LLMLingua |
| **Microsoft GraphRAG** | 索引巨慢 | LightRAG |
| **Redis Streams** | 部署 + 运维 | SQLite WAL (S1-W2) |

---

## 跟 Loom 已有组件的关系

| 已有 | 可复用 / 扩展 | 新增 |
|------|---------------|------|
| `internal/journal/` (S1-W2) | ✅ 直接复用作 Event Journal | — |
| `internal/evidence/` (S1-W2) | ✅ 复用 S1-W5 重排证据 | — |
| `internal/projection/` (S1-W4) | ✅ 直接复用 projection 模式 | — |
| `internal/agents/definition.go` (S1-W5) | ✅ AgentDefinition 扩展 memory_type | — |
| `internal/runtime/catalog.go` (S1-W5) | ✅ RuntimeProfile 扩展 memory config | — |
| `cmd/loom/query.go` (S1-W1) | ✅ CLI 扩展 memory subcommand | — |
| `cmd/loom/main.go` (S1-W1) | — | 加 `cmd/loom/memory.go` |
| `internal/mode/router.go` (S1-W1) | — | 路由可加 memory 维度 |
| `docs/CURRENT.md` | ✅ 现状 + 计划位 | — |
| `docs/adr/0006-fastcontext.md` | ✅ 已有 FastContext (Phase 2 不吸) | — |
| `docs/integrations/fastcontext.md` | — | Phase 2 准备 |

---

## 后续动作 (3 选项, 等用户拍)

### 选项 A: 出完整 Spec
- 把"19 能力维度 + Loom 实施路径"扩成 4-5K 字 spec
- 每个能力给契约 (input/output/error/failure mode) + 接口签名
- 落 `.loom-drafts/loom-memory-cap-2026-08-04-spec.md`
- **估时**: 1-2 小时

### 选项 B: 合进 G8.5 PROV-1
- 当前 `g8-six-tasks-briefs.md` 第 5 个 task = PROV-1
- 把"19 能力分类"作为 PROV-1 的"能力矩阵"小节
- 触发: 把能力分类 + Phase 1 推荐表 + Phase 2 待评估项 一起入
- **估时**: 30 分钟

### 选项 C: 维持调研存档
- 落 `.loom-drafts/` 等 Phase 1 实际跑起来再决定
- 优点: 跟现状一致, 不过度承诺
- 缺点: Phase 1 实施时还得翻这份

**默认推荐**: **B** (合进 G8.5 PROV-1), 因为 19 能力维度 = 1 文档可装下, 不需要单独 spec, 而 G8 准备桶本来就在等这种"能力盘点"
