# Loom Memory 层实施手册 — 怎么用起来 + 怎么做

> **作者**: Mavis · **日期**: 2026-08-04 15:35 SGT
> **承接**: 5 份 8.04 调研产物 (~180KB, 全部 untracked)
> **本报告专注**: 把调研落成可 ship 的 Loom Memory 层 — **怎么用** (集成模式 / 用户角色) + **怎么做** (实施路径 / 代码 / 验证)
> **目标读者**: 写代码的工程师 (你 / 我 / Loom 后续 work item owner)
> **状态**: 从 0 到 1 的实施手册, 不是架构设计

---

## 0. TL;DR

**3 句话讲清楚**:
1. **怎么用**: Memory 层作为 Loom Runtime 的内置服务, 三种集成模式 (Agent 透明调用 / 显式 Skill / CLI 调试)
2. **怎么做**: 4 个阶段, 6-8 周, 1-2 个工程师
3. **核心代码**: 4 个 Go 文件 (`store.go` / `retrieval.go` / `extract.go` / `temporal.go`), ~3000 行

**关键决策 (一锤定音)**:
- 存储: **P0-11 DBOS Postgres + pgvector + Apache AGE** (标量+向量+图 三位一体)
- 提取: **规则 + jieba NER** (无 LLM, D3 兼容)
- 检索: **5 路混合 + RRF 融合** (Vector + BM25 + Entity + Graph + Recency)
- 时序: **valid_from / valid_to** (借 oracle-mem + Mem0 v0.7)
- 文件: **`.loom/memory/{user,env,project}.md`** (借 Hermes)
- 协议: **claude-mem 4 工具兼容** (SK-3)

---

## 1. 怎么用起来

### 1.1 三种集成模式 (从透明到显式)

#### 模式 1: Agent 透明调用 (90% 场景, 默认)

```yaml
# Loom agent contract (在 .loom/agents/<name>.yaml)
agent:
  name: coder
  memory: auto    # <-- 透明模式, 不需要 Agent 写代码
  hooks:
    pre_run: "loom memory inject --session ${session_id}"   # 自动注入
    post_run: "loom memory record --session ${session_id}"  # 自动记录
```

**用户体验**:
```
用户: 帮我重构 /orders 接口
Agent: (自动调 memory inject, 拿到 "FastAPI + PostgreSQL + Redis, /orders 加了索引")
       (自动调 memory record, 存"用户偏好 black 格式化")
输出: (输出答案)
```

用户**完全不知道** memory 层存在, 但实际享受跨会话上下文延续。

#### 模式 2: 显式 Skill (10% 场景, 高级用户)

```markdown
<!-- .loom/skills/memory-query/SKILL.md -->
---
name: memory-query
description: "用户明确问记忆/历史/之前做过什么时触发"
triggers:
  - "上次"
  - "之前"
  - "记得"
  - "之前提过"
---

# Memory Query Skill

当用户明确询问历史信息时, 调用 memory 层:
1. `loom memory search "<query>" --top-k 5 --wing <wing_id>`
2. 把结果按时间倒序呈现
3. 如果有冲突 (旧 vs 新), 提示用户
```

**用户体验**:
```
用户: 上次那个慢的接口怎么优化的?
Agent: (Skill 触发, 调 memory search "慢 接口 优化")
       → 找到 3 条历史: "加了索引" / "EXPLAIN 验证" / "CI 补检测"
输出: 上次 /orders 慢问题... (1) 加索引 (2) EXPLAIN 验证 (3) CI 补检测
```

#### 模式 3: CLI 调试 (5% 场景, 开发者 / 用户自查)

```bash
# 查 memory
loom memory search "FastAPI 项目" --top-k 5
loom memory list --wing loom-pi-rebuild
loom memory show <memory_id>

# 编辑
loom memory edit <memory_id> --content "updated fact"
loom memory delete <memory_id>
loom memory merge <id1> <id2>   # 合并重复

# 治理
loom memory stats              # 总数 / 命中率 / 重复率
loom memory consolidate         # 触发 consolidation worker
loom memory export --wing X     # 导出单 wing
loom memory import <file>       # 导入 (跨设备迁移)
```

### 1.2 三类用户角色

| 角色 | 用 Memory 的方式 | 典型场景 | Loom 需要 |
|------|-----------------|---------|----------|
| **Agent (90%)** | 透明调用, 自动注入 + 记录 | 所有 run | hooks + auto-inject |
| **Power user (8%)** | 显式 Skill + CLI 查/改 | 跨会话查"之前干过啥" | `loom memory` CLI + Skills |
| **Casual user (2%)** | 完全不感知 | 让 Loom 跑就行 | 模式 1 自动覆盖 |

**Loom 默认 = 模式 1 (透明)**, 其他两种可选开启。

### 1.3 真实使用示例 (3 个场景)

#### 场景 1: 跨会话技术栈延续 (最常见)

```
会话 1 (周一):
  用户: 我们项目用 FastAPI + PostgreSQL
  Agent: 好的
  (后台: memory 记录 "技术栈=FastAPI+PostgreSQL", wing=loom, room=session-1)

会话 2 (周三, 新会话):
  用户: 加个 API
  Agent: (后台自动 inject "技术栈=FastAPI+PostgreSQL")
  Agent: (用 FastAPI 风格写, 用 PostgreSQL 存)
  用户: 嗯, 对味
```

#### 场景 2: 跨任务的经验复用

```
会话 1: 用户让重构 /orders 慢接口 → Agent 加索引 → CI 补检测
       (memory 存: "订单慢修复=加索引+EXPLAIN+CI")

会话 2 (1 个月后): 用户让重构 /payments 慢接口
  Agent: (memory 检索 "慢 修复")
  Agent: "上次 /orders 你用的 3 步法: 1) 加索引 2) EXPLAIN 3) CI 补检测, 要套用到 /payments 吗?"
  用户: 对
```

#### 场景 3: 时序事实更新

```
会话 1: "Alice 帮我们做前端"
       (memory: Alice-relation=COLLABORATES)

会话 2 (3 个月后): "Alice 离职了"
       (memory 检测冲突, 关旧边 valid_to=NOW, 开新边: Alice-status=FORMER)
       (查询 "Alice 当前状态" → 命中"已离职")
```

---

## 2. 怎么做的 (实施路径)

### 2.1 4 个阶段, 6-8 周

```
┌──────────────────────────────────────────────────────────┐
│ 阶段 1: P0-11 DBOS + schema (1 周)                         │
│   - internal/mem/schema.sql                              │
│   - DBOS workflow 集成                                    │
│   - 跟 S1-W2 Event Journal FK 关联                       │
├──────────────────────────────────────────────────────────┤
│ 阶段 2: ME-2 三层 + Hybrid Retrieval (1-2 周)              │
│   - internal/mem/{store,retrieval,entity}.go            │
│   - 5 路并行 + RRF 融合                                  │
│   - jieba NER (无 LLM)                                   │
├──────────────────────────────────────────────────────────┤
│ 阶段 3: SK-3 协议兼容 (1 周)                                │
│   - internal/mem/claudemem_compat.go                     │
│   - 4 个 MCP 工具 stub                                    │
│   - 兼容测试 (Claude Code / Codex)                       │
├──────────────────────────────────────────────────────────┤
│ 阶段 4: TK-1 时序 KG (Phase 2, 2-3 周, 暂缓)                │
│   - internal/kg/{extract,relations,storage}.go           │
│   - Apache AGE 图查询                                    │
│   - temporal boost                                       │
└──────────────────────────────────────────────────────────┘
```

### 2.2 阶段 1: P0-11 DBOS + Schema (Week 1)

#### 任务清单

| Day | 任务 | 输出 |
|-----|------|------|
| 1-2 | 写 `internal/mem/schema.sql` | 3 张表 (nodes/edges/facts) + 索引 |
| 3-4 | 写 `internal/mem/migrate.go` (DBOS workflow) | migration 工具 |
| 5 | 写 `internal/mem/store.go` (CRUD) | 基础读写 |
| 6-7 | 写测试 (sqlmock + DBOS 集成) | 覆盖率 > 80% |

#### SQL Schema (具体可执行)

```sql
-- internal/mem/schema.sql (跟 P0-11 DBOS 复用)

-- 1. 三层目录 (借 MemPalace)
CREATE TABLE memory_nodes (
    id BIGSERIAL PRIMARY KEY,
    wing_id UUID NOT NULL,         -- 项目 workspace
    room_id UUID NOT NULL,         -- session_id
    drawer_id UUID NOT NULL,        -- 单条 message/tool result
    name TEXT NOT NULL,            -- 实体名
    type TEXT NOT NULL,            -- person/org/project/concept/tool
    embedding vector(1024),        -- BGE-M3
    importance REAL DEFAULT 0.5,
    valid_from TIMESTAMPTZ DEFAULT NOW(),
    valid_to TIMESTAMPTZ,          -- NULL = 当前有效
    source_event_id BIGINT REFERENCES journal_events(id),  -- S1-W2 集成
    metadata JSONB,
    UNIQUE(wing_id, room_id, drawer_id)
);

-- 2. 时序关系 (借 oracle-mem)
CREATE TABLE memory_edges (
    id BIGSERIAL PRIMARY KEY,
    subject_id BIGINT REFERENCES memory_nodes(id),
    object_id BIGINT REFERENCES memory_nodes(id),
    relation_type TEXT NOT NULL,
    valid_from TIMESTAMPTZ DEFAULT NOW(),
    valid_to TIMESTAMPTZ,
    confidence REAL DEFAULT 1.0,
    source_event_id BIGINT REFERENCES journal_events(id)
);

-- 3. 文件落地 (借 Hermes MEMORY.md)
CREATE TABLE memory_files (
    wing_id UUID PRIMARY KEY,
    user_md TEXT,                   -- 用户画像
    env_md TEXT,                    -- 环境事实
    project_md TEXT,                -- 项目背景
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 索引
CREATE INDEX memory_nodes_wing_idx ON memory_nodes(wing_id);
CREATE INDEX memory_nodes_room_idx ON memory_nodes(room_id);
CREATE INDEX memory_nodes_valid_idx ON memory_nodes(valid_from, valid_to);
CREATE INDEX memory_nodes_embedding_idx ON memory_nodes
    USING hnsw (embedding vector_cosine_ops);
CREATE INDEX memory_nodes_fts_idx ON memory_nodes
    USING gin (to_tsvector('simple', name || ' ' || COALESCE(metadata->>'description', '')));
CREATE INDEX memory_edges_subject_idx ON memory_edges(subject_id, valid_to);
CREATE INDEX memory_edges_object_idx ON memory_edges(object_id, valid_to);
```

#### Go Store (CRUD)

```go
// internal/mem/store.go
package mem

import (
    "context"
    "encoding/json"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Node struct {
    ID         int64
    WingID     string
    RoomID     string
    DrawerID   string
    Name       string
    Type       string
    Embedding  []float32
    Importance float64
    ValidFrom  time.Time
    ValidTo    *time.Time
    SourceEventID int64
    Metadata   map[string]any
}

type Store struct {
    db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
    return &Store{db: db}
}

func (s *Store) CreateNode(ctx context.Context, n *Node) (int64, error) {
    meta, _ := json.Marshal(n.Metadata)
    var id int64
    err := s.db.QueryRow(ctx, `
        INSERT INTO memory_nodes
        (wing_id, room_id, drawer_id, name, type, embedding, importance, source_event_id, metadata)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id
    `, n.WingID, n.RoomID, n.DrawerID, n.Name, n.Type,
       pgvector.NewVector(n.Embedding), n.Importance, n.SourceEventID, meta).Scan(&id)
    return id, err
}

func (s *Store) GetNode(ctx context.Context, id int64) (*Node, error) {
    var n Node
    var validTo *time.Time
    var meta []byte
    var emb pgvector.Vector
    err := s.db.QueryRow(ctx, `
        SELECT id, wing_id, room_id, drawer_id, name, type, embedding,
               importance, valid_from, valid_to, source_event_id, metadata
        FROM memory_nodes WHERE id = $1 AND valid_to IS NULL
    `, id).Scan(&n.ID, &n.WingID, &n.RoomID, &n.DrawerID, &n.Name, &n.Type,
                 &emb, &n.Importance, &n.ValidFrom, &validTo, &n.SourceEventID, &meta)
    if err != nil {
        return nil, err
    }
    n.Embedding = emb.Slice()
    n.ValidTo = validTo
    json.Unmarshal(meta, &n.Metadata)
    return &n, nil
}

func (s *Store) CloseNode(ctx context.Context, id int64) error {
    // 关掉节点 (valid_to = NOW)
    _, err := s.db.Exec(ctx,
        `UPDATE memory_nodes SET valid_to = NOW() WHERE id = $1`, id)
    return err
}
```

### 2.3 阶段 2: ME-2 5 路混合检索 (Week 2-3)

#### 任务清单

| Day | 任务 | 输出 |
|-----|------|------|
| 8-9 | 写 `internal/mem/vector.go` (pgvector HNSW) | 1 路检索 |
| 10 | 写 `internal/mem/bm25.go` (PG tsvector) | 1 路检索 |
| 11-12 | 写 `internal/mem/entity.go` (jieba NER) | 1 路检索 |
| 13-14 | 写 `internal/mem/retrieval.go` (5 路 RRF) | 主检索 |
| 15-16 | 写 `internal/mem/extract.go` (规则 NER) | 提取器 |
| 17-18 | benchmark + 调优 (latency 50ms p99) | 性能达标 |

#### 5 路并行检索 (核心代码)

```go
// internal/mem/retrieval.go
package mem

import (
    "context"
    "sync"
)

type Retriever struct {
    store  *Store
    vector *VectorIndex
    bm25   *BM25Index
    entity *EntityIndex
    kg     *KGIndex
}

type Chunk struct {
    Node     *Node
    Score    float64
    Source   string  // vector / bm25 / entity / graph / recency
    Distance float32
}

func (r *Retriever) HybridRetrieve(ctx context.Context, query string, topK int) ([]Chunk, error) {
    var (
        wg            sync.WaitGroup
        vectorHits    []Chunk
        bm25Hits      []Chunk
        entityHits    []Chunk
        graphHits     []Chunk
    )

    // 1. 5 路并行 (含 recency boost 在最后)
    wg.Add(4)

    // 路 1: Vector (pgvector HNSW)
    go func() {
        defer wg.Done()
        qEmb, _ := r.embed(query)
        vectorHits, _ = r.vector.HNSWSearch(ctx, qEmb, topK*2)
    }()

    // 路 2: BM25 (PG tsvector)
    go func() {
        defer wg.Done()
        bm25Hits, _ = r.bm25.FTSSearch(ctx, query, topK*2)
    }()

    // 路 3: Entity (jieba NER + 已知实体)
    go func() {
        defer wg.Done()
        entities := jieba.NER(query)
        entityHits, _ = r.entity.FindByName(ctx, entities, topK*2)
    }()

    // 路 4: Graph (Apache AGE 2 跳, 借 Cognee)
    go func() {
        defer wg.Done()
        entities := jieba.NER(query)
        graphHits, _ = r.kg.FindRelatedChunks(ctx, entities, depth=2, topK*2)
    }()

    wg.Wait()

    // 2. RRF 融合 (Reciprocal Rank Fusion)
    merged := r.rrf.Merge([][]Chunk{vectorHits, bm25Hits, entityHits, graphHits},
        []float64{1.0, 0.8, 1.2, 1.5},  // 路 4 (Graph) 权重最高
        topK*2)

    // 3. Recency boost (借 Mem0 temporal, 但用规则检测意图)
    boosted := r.recency.Boost(ctx, query, merged)

    return boosted[:topK], nil
}

func (r *Retriever) detectIntent(query string) string {
    q := strings.ToLower(query)
    if strings.Contains(q, "现在") || strings.Contains(q, "当前") || strings.Contains(q, "currently") {
        return "current"
    }
    if strings.Contains(q, "之前") || strings.Contains(q, "上次") || strings.Contains(q, "before") {
        return "past"
    }
    if strings.Contains(q, "即将") || strings.Contains(q, "打算") || strings.Contains(q, "going to") {
        return "future"
    }
    return "none"
}

func (r *Retriever) recencyFactor(intent string, n *Node) float64 {
    switch intent {
    case "current":
        if n.ValidTo == nil {
            return 1.5  // 强 boost 当前有效
        }
    case "past":
        if n.ValidTo != nil {
            return 1.3  // 强 boost 已关闭
        }
    case "future":
        if n.Metadata["planned"] == true {
            return 1.3
        }
    }
    return 1.0
}
```

#### 规则 NER 提取 (无 LLM)

```go
// internal/mem/extract.go
package mem

import (
    "regexp"
    "github.com/yanyiwu/gojieba"
)

type Extractor struct {
    jieba *gojieba.Jieba
    patterns []RelPattern
}

type RelPattern struct {
    Regex      *regexp.Regexp
    RelType    string
}

func NewExtractor() *Extractor {
    return &Extractor{
        jieba: gojieba.NewJieba(),
        patterns: []RelPattern{
            {regexp.MustCompile(`(\w+) 在 (\w+) 工作`), "WORKS_AT"},
            {regexp.MustCompile(`(\w+) 创建了 (\w+)`), "CREATED"},
            {regexp.MustCompile(`(\w+) 是 (\w+) 的`), "IS_PART_OF"},
            {regexp.MustCompile(`(\w+) 担任 (\w+)`), "HAS_ROLE"},
            {regexp.MustCompile(`(\w+) 负责 (\w+)`), "RESPONSIBLE_FOR"},
        },
    }
}

func (e *Extractor) Extract(text string) ([]Entity, []Relation) {
    // 1. jieba NER 提取实体 (Person/Org/Location)
    words := e.jieba.Tag(text)
    var entities []Entity
    entityByName := make(map[string]*Entity)
    for _, w := range words {
        if w.Pos == "nr" || w.Pos == "nt" || w.Pos == "ns" {
            ent := &Entity{
                Name: w.Word,
                Type: posToEntityType(w.Pos),
            }
            entities = append(entities, *ent)
            entityByName[w.Word] = ent
        }
    }

    // 2. 关系抽取 (规则模板)
    var relations []Relation
    for _, p := range e.patterns {
        matches := p.Regex.FindAllStringSubmatch(text, -1)
        for _, m := range matches {
            subj := entityByName[m[1]]
            obj := entityByName[m[2]]
            if subj != nil && obj != nil {
                relations = append(relations, Relation{
                    Subject:   *subj,
                    Object:    *obj,
                    Type:      p.RelType,
                    Source:    text,
                    ValidFrom: time.Now(),
                })
            }
        }
    }
    return entities, relations
}

func posToEntityType(pos string) string {
    switch pos {
    case "nr": return "person"
    case "nt": return "org"
    case "ns": return "location"
    default:   return "concept"
    }
}
```

### 2.4 阶段 3: SK-3 claude-mem 协议兼容 (Week 4)

```go
// internal/mem/claudemem_compat.go
package mem

import (
    "context"
    "github.com/metoro-io/mcp-go/server"
)

// 兼容 claude-mem 的 4 个 MCP 工具

type ClaudeMemCompat struct {
    retriever *Retriever
    store     *Store
    journal   *journal.Journal
}

func (c *ClaudeMemCompat) Register(s *server.MCPServer) {
    s.RegisterTool("mcp__loom-mem__search", c.Search)
    s.RegisterTool("mcp__loom-mem__timeline", c.Timeline)
    s.RegisterTool("mcp__loom-mem__get_observations", c.GetObservations)
    s.RegisterTool("mcp__loom-mem__inject_context", c.InjectContext)
}

// 1. search - 跨会话搜索
func (c *ClaudeMemCompat) Search(ctx context.Context, req SearchReq) (SearchResp, error) {
    hits, err := c.retriever.HybridRetrieve(ctx, req.Query, req.TopK)
    return SearchResp{
        Observations: convertToClaudeMemFormat(hits),
        ScoreBreakdown: ScoreBreakdown{
            Vector: avg(hits, "vector"),
            BM25:   avg(hits, "bm25"),
            Recency: avg(hits, "recency"),
        },
    }, err
}

// 2. timeline - 时序视图
func (c *ClaudeMemCompat) Timeline(ctx context.Context, roomID string) ([]TimelineNode, error) {
    rows, _ := c.store.db.Query(ctx, `
        SELECT drawer_id, name, valid_from, valid_to, importance
        FROM memory_nodes
        WHERE room_id = $1
        ORDER BY valid_from DESC
        LIMIT 50
    `, roomID)
    return rowsToTimeline(rows), nil
}

// 3. get_observations - 取单条
func (c *ClaudeMemCompat) GetObservations(ctx context.Context, ids []string) ([]Observation, error) {
    var obs []Observation
    for _, id := range ids {
        n, _ := c.store.GetNode(ctx, parseID(id))
        obs = append(obs, nodeToObservation(n))
    }
    return obs, nil
}

// 4. inject_context - 注入上下文 (Endless Mode)
func (c *ClaudeMemCompat) InjectContext(ctx context.Context, roomID string, maxTokens int) ([]Observation, error) {
    // 1. 拿最近 turn (10 条)
    recent, _ := c.store.GetRecentByRoom(ctx, roomID, 10)
    // 2. 反查相关历史
    query := joinContents(recent)
    hits, _ := c.retriever.HybridRetrieve(ctx, query, 20)
    // 3. token 预算挑选
    budget := NewTokenBudget(maxTokens)
    var picked []Observation
    for _, h := range hits {
        if budget.Remaining() < estimateTokens(h) {
            break
        }
        picked = append(picked, nodeToObservation(h.Node))
        budget.Consume(h)
    }
    return picked, nil
}
```

### 2.5 阶段 4: TK-1 时序 KG (Phase 2, Week 5-8)

```go
// internal/kg/extract.go
// 借 oracle-mem Property Graph + Mem0 temporal

func (kg *KG) AddRelation(ctx context.Context, rel Relation) error {
    // 1. 检测冲突 (同 subject + relation_type, 旧 edge 未关闭)
    existing, _ := kg.findActiveEdges(ctx, rel.SubjectID, rel.RelationType, rel.ObjectID)
    for _, e := range existing {
        if e.ValidTo == nil {
            // 关闭旧 edge
            e.ValidTo = rel.ValidFrom
            kg.updateEdge(ctx, e)
        }
    }
    // 2. 创建新 edge
    rel.ValidFrom = time.Now()
    rel.ValidTo = nil
    return kg.insertEdge(ctx, rel)
}

// Apache AGE 图查询 (Cypher 语法, 不是 SQL/PGQ)
func (kg *KG) FindRelatedChunks(ctx context.Context, entities []Entity, depth int) ([]Chunk, error) {
    query := `
        MATCH (a:Entity {name: $name})-[r:RELATION*1..2]->(b:Entity)
        RETURN b.name
    `
    // ...
}
```

---

## 3. 验证 (3 层测试)

### 3.1 单元测试 (覆盖率 > 80%)

```go
// internal/mem/store_test.go
func TestStoreCreateAndGet(t *testing.T) {
    db := testutil.NewTestDB(t)
    s := NewStore(db)

    n := &Node{
        WingID: uuid.New(), RoomID: uuid.New(), DrawerID: uuid.New(),
        Name: "Alice", Type: "person", Embedding: randomVector(1024),
        Importance: 0.8, SourceEventID: 42,
    }
    id, err := s.CreateNode(context.Background(), n)
    require.NoError(t, err)

    got, err := s.GetNode(context.Background(), id)
    require.NoError(t, err)
    assert.Equal(t, n.Name, got.Name)
    assert.InDelta(t, n.Embedding[0], got.Embedding[0], 0.001)
}

// internal/mem/retrieval_test.go
func TestHybridRetrieve(t *testing.T) {
    r := newTestRetriever(t)
    // 插入 3 类: vector-matched / bm25-matched / entity-matched
    seedTestData(t, r)
    // 查 "Alice 在哪工作" → 应该命中 entity 路径
    hits, err := r.HybridRetrieve(ctx, "Alice 在哪工作", 5)
    require.NoError(t, err)
    require.Len(t, hits, 5)
    assert.Contains(t, hits[0].Source, "entity")
}
```

### 3.2 集成测试 (DBOS + Event Journal)

```go
// internal/mem/integration_test.go
func TestMemoryEventJournalIntegration(t *testing.T) {
    // 1. 启动 DBOS Postgres
    db := testutil.NewDBOS(t)
    s := mem.NewStore(db)
    j := journal.NewJournal(db)

    // 2. 写 Event Journal
    eventID, _ := j.Append(ctx, journal.Event{
        Type: "user.message",
        Data: "我们用 FastAPI + PostgreSQL",
    })

    // 3. 从 Event 提取 memory
    extractor := mem.NewExtractor()
    entities, relations := extractor.Extract("我们用 FastAPI + PostgreSQL")

    // 4. 落 memory (FK 关联 event)
    for _, e := range entities {
        s.CreateNode(ctx, &mem.Node{
            Name: e.Name, Type: e.Type,
            SourceEventID: eventID,
        })
    }

    // 5. 反查: 拉所有 event_id 关联的 memory
    memories, _ := s.GetByEvent(ctx, eventID)
    assert.Contains(t, memories, "FastAPI")
}
```

### 3.3 Benchmark (性能验收)

```go
// internal/mem/benchmark_test.go
func BenchmarkHybridRetrieve(b *testing.B) {
    r := newBenchRetriever(b)
    // 灌 1M fake nodes
    seedFakeData(b, r, 1_000_000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        r.HybridRetrieve(ctx, "FastAPI PostgreSQL 慢查询 优化", 5)
    }
}

// 验收标准:
// HybridRetrieve-1M    50000 ns/op  (50ms p99)
// Vector-Search-1M     5000 ns/op   (5ms p99)
// BM25-Search-1M       3000 ns/op   (3ms p99)
// Entity-Lookup-1M     2000 ns/op   (2ms p99)
// Graph-2hop-1M       20000 ns/op  (20ms p99)
```

### 3.4 端到端测试 (用户场景)

```go
// tests/e2e/memory_test.go
func TestCrossSessionContinuity(t *testing.T) {
    // 场景 1: 跨会话技术栈延续
    s1 := loom.NewSession("alice")
    s1.Run("我们用 FastAPI + PostgreSQL")
    s1.Close()

    s2 := loom.NewSession("alice")  // 新 session
    s2.Run("加个 API")
    assert.Contains(t, s2.Response, "FastAPI")
    assert.Contains(t, s2.Response, "PostgreSQL")
}
```

---

## 4. 部署

### 4.1 本地开发

```bash
# 1. 启 Postgres + pgvector + Apache AGE
docker run -d --name loom-pg \
  -e POSTGRES_PASSWORD=loom \
  -p 5432:5432 \
  pgvector/pgvector:pg16

# 2. 装 Apache AGE
docker exec loom-pg apt-get update
docker exec loom-pg apt-get install -y postgresql-16-age

# 3. 跑 migration
psql -h localhost -U postgres -f internal/mem/schema.sql

# 4. 跑 Loom
./loom serve
```

### 4.2 集成到 Loom Runtime (S1-W3)

```go
// cmd/loom/main.go
import (
    "github.com/loom/loom/internal/mem"
    "github.com/loom/loom/internal/runtime"
)

func main() {
    // 1. 启 DB
    db := pgxpool.New(...)

    // 2. 启 Memory 层
    memStore := mem.NewStore(db)
    retriever := mem.NewRetriever(memStore, embedder)
    extractor := mem.NewExtractor()

    // 3. 注入到 Runtime
    runtime.RegisterMemoryService(memStore, retriever, extractor)

    // 4. 启 HTTP / TUI
    ...
}
```

### 4.3 部署到 macOS 本地 (生产路径)

```bash
# 1. DBOS Postgres 启动 (Phase 1 S2)
brew services start dbos-postgres@16

# 2. Loom 服务
brew services start loom

# 3. 验证
curl http://localhost:7432/health  # Loom API
psql -h localhost -d loom -c "SELECT COUNT(*) FROM memory_nodes"
```

---

## 5. 风险与缓解

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| jieba NER 准确率 70-80% | 高 | 中 | 跟用户确认 + 高频实体手动录入 |
| pgvector HNSW 内存大 (1M 100GB+) | 中 | 高 | 启用 INT8 量化 (P1 VC-1) |
| Apache AGE 生态不成熟 | 中 | 中 | 备选 Neo4j (Phase 3) |
| 5 路并行 latency > 50ms | 低 | 高 | 实测 benchmark, 调权重或砍路 |
| claude-mem 协议漂移 | 中 | 中 | 锁 v1 协议, 跟 S1-W5 AGENTS.md 同 |
| DBOS + pgvector + AGE 集成坑 | 中 | 中 | Week 1 早期 PoC, 留 1 周 buffer |

---

## 6. 验收标准 (Definition of Done)

### 6.1 功能验收

- [ ] 阶段 1: schema + migration + CRUD 通过测试
- [ ] 阶段 2: 5 路检索 50ms p99 (1M 数据集)
- [ ] 阶段 3: claude-mem 4 工具在 Claude Code 实际可用
- [ ] 阶段 4: 时序 KG, 测 "Alice 现在在哪工作" 准确

### 6.2 质量验收

- [ ] 单元测试覆盖率 > 80%
- [ ] benchmark 1M 节点 5 路 < 50ms p99
- [ ] 集成测试覆盖 8 个 invariant 不破
- [ ] 端到端测试覆盖 3 个用户场景

### 6.3 部署验收

- [ ] docker compose 一键起 (Postgres + pgvector + AGE)
- [ ] 集成到 Loom Runtime 启动 < 5s
- [ ] macOS brew services 启动 < 3s
- [ ] 文档齐 (本手册 + API doc + CLI help)

---

## 7. 关键资源 (整合自前面 5 份)

| 主题 | 参考 |
|------|------|
| **19 维度分类** | `.loom-drafts/loom-memory-capability-classification-2026-08-04.md` |
| **19 实战手册** | `.loom-drafts/loom-memory-usage-guide-2026-08-04.md` |
| **5 大最新趋势** | `.loom-drafts/loom-memory-latest-2026-08-04.md` |
| **7-8 项目总览** | `.loom-drafts/loom-memory-hot-projects-2026-08-04.md` |
| **怎么做 deep dive** | `.loom-drafts/loom-memory-how-they-built-2026-08-04.md` |
| **本报告 (实施)** | `.loom-drafts/loom-memory-implementation-handbook-2026-08-04.md` |

**6 份合计**: ~200KB, **完整覆盖 "分类 → 实战 → 最新 → 爆火 → 怎么做 → 怎么用"**

---

**报告完**

> **下一步建议**:
> 1. **落 work item**: 跟 S2-W3 整合, 把阶段 1+2 写成 `loom-mem-p0` 任务卡, 估 2 周
> 2. **写 benchmark harness**: 拿 1M fake node 数据, 验证 50ms p99 目标
> 3. **维持存档**: 6 份合计 200KB 作为 Phase 1 Slice 3 / Phase 2 输入
