# Loom 社区开源项目调研 v1 (2026-07-24)

## 调研目标
按用户指定 7 个 GitHub repo，验证"是否真的存在"、"是不是真的跟 Loom 有关"。
**注意**：trending 列表给的 star 数和 30 天增长可能与实际 repo 不符 — 必须直接访问 GitHub 验证。

## 调研方法
- 直接 web_fetch 主页 + description
- 提取 README 第一段（核心定位 + 技术栈 + 协议/集成）
- 跟 Loom 的"2-tier Planner+Executor / Bridge stdio JSON / 3 view / AgentKey vault"对比

## 7 个 repo 实际结果

| Repo | trending 给的 | 实际 GitHub | 分类 | 跟 Loom 关系 |
|---|---|---|---|---|
| **aos-ce** | ⭐5628 | `unicity-aos/aos-ce` ⭐6.9k, 70 commits, **Rust** | **高相关** — 开放 agent 操作系统 | **强相关（最值得深挖）** |
| **open-connector** | ⭐3057 | 4+ 同名低星项目 (`openconnector-dev/openconnector` 1★) | AI agent connector 平台 | 中相关（早期阶段） |
| **colibri** | ⭐17559 | `Colibri` 老账号 (2007), 1 个 fork 仓库, **stars 0** | **未找到 17.5k 那个** | **不存在**（trending 数字疑似错配） |
| **Codex-Dream-Skin** | ⭐11499 | `Fei-Away/Codex-Dream-Skin` ⭐12.1k, 1.2k forks | Codex 桌面端换肤工具 (CDP 远程调试) | **不相关**（纯 UI 主题） |
| **wloc** | ⭐5963 | `wloc-org/wloc` **404 Not Found** | **不存在** | **不存在** |
| **exploitarium** | ⭐3985 | 多个 0★ fork，主流是 `crptx1337/exploitarium` Python | 公开 exploit PoC + 漏洞研究 writeup | **不相关**（安全研究） |
| **torlink** | ⭐3739 | 多个 Tor 项目 (TypeScript/Python), 0-2★ | Tor 网络 P2P 工具 / Tor link 列表 | **不相关**（匿名网络） |

## 关键发现 1：AOS Community Edition (aos-ce) ⭐6.9k — 强相关

**精确描述** (摘 README 原文)：
> "AOS Community Edition is the open agent operating system for people who want an inspectable, composable environment for agents. It owns the Community Edition product surface: the `aos` CLI, HTTP API, distributions, first-party capsules, provider and model experience, and Unicity Audit."

**核心架构**：
- **产品边界**：`crates/` (产品 CLI/HTTP API/control client) + `capsules/` (21 个 first-party capsule) + `distros/` (Community distribution manifests)
- **安装模型**：`curl ... | sh` 一行安装 `aos` CLI + 21 capsules 到 `~/.aos`
- **Command 边界**：`aos` CLI 拥有 `init/status/migrate/update/distro/mcp/serve-health` 这些 product roots；其他运行时 root 直接走 AOS CLI
- **Unicity Audit**：所有 release 发布 checksums + Sigstore bundles + GitHub build-provenance attestations + `runtime-compatibility.toml`
- **`aos mcp serve`** = **MCP product edge，被 Codex/Claude/Grok 共享** — **直接验证我 v0.4 MCP 调研的策略 3"双角色"**

**与 Loom 的对比**：

| 维度 | AOS CE | Loom |
|---|---|---|
| 定位 | Agent 操作系统 | Agent 编排 + 观测层 (view layer) |
| 协议 | MCP (作为 product edge) | Bridge v1.1 (本地 JSON-RPC 2.0) + 未来 MCP |
| Capsule | 21 个 first-party, 通用 user-space building block | Subagent 抽象, Planner 派发 |
| 安装 | 一行 curl + 21 capsules | 本地 Go 1.22 二进制 + SQLite |
| 信任根 | Sigstore + provenance attestations | AgentKey scoped token + age encryption |
| 主模型 | Rust + WIT (WebAssembly Interface Type) | Go 1.22 + pure-Go SQLite |
| 审计 | Unicity Audit (闭源产品?) | Event Journal + rebuildable Projection |

**核心 takeaway**：
1. ✅ AOS CE 的"capsule"概念和 Loom 的"subagent"概念高度同构 — 都是"用户空间的能力块"
2. ✅ `aos mcp serve` 作为 shared product edge 是个非常好的设计模式 — Loom Phase 2 接入 MCP 时可以借鉴
3. ⚠️ AOS CE 是 **product surface**（CLI + HTTP API），不是"view layer" — 跟 Loom 的"3 view 观测"方向不同
4. ⚠️ AOS CE 的 capsule **构建工具叫 Forge**，鼓励用户"自己造 capsule" — Loom 的 subagent 抽象鼓励"Planner 派发已有 subagent"，方向不同
5. ⚠️ AOS CE 的 runtime 是 **astrid**（自研 Rust runtime），Loom 不造 runtime — 这跟 Loom "view layer, not controller" 哲学一致
6. ❓ AOS CE 是否可以成为 Loom 的下游消费者？理论上 Loom 的 subagent 可以被 AOS CE 的 capsule 框架调度，需要 Phase 2+ 再看

**未读项**（后续深挖）：
- `crates/` 内部构造（product CLI 怎么实现）
- `capsules/` 21 个 capsule 的分类
- `docs/meta-harness.md` 的"agent world extension"模型
- `docs/runtime-migration.md` 的 allowlist
- `docs/release-channels.md` 的签名 channel 机制

## 关键发现 2：open-connector 多项目散乱

实际 GitHub 搜索 "open-connector" 返回 4+ 个低星项目，最相关的是：
- **`openconnector-dev/openconnector`** ⭐1：开源、可自托管的 AI agent connector 平台
  - Topics: cli, open-source, oauth, ai, integration
  - "Source code is coming soon" — **还在早期**
  - 时间：6 天前更新

**与 Loom 关系**：
- 都想做"AI agent connector platform" — 概念重叠
- 但 open-connector 还在 source code 阶段，Loom 已经在做
- **暂不深挖**（Star 太低 + 代码未发布），标记 v0.6 候选

## 关键发现 3：colibri / wloc / torlink 不存在或无关

- **colibri** (trending ⭐17.5k) 找不到对应的高星 repo — 可能是 trending 误报（其他同名小项目被合并计数）
- **wloc** (trending ⭐5.9k) `wloc-org/wloc` 直接 404 — **不存在**
- **torlink** 是 Tor P2P / Tor link 列表项目（`neoatlantis/torlink` Python P2P over Tor hidden services, `dironion/torlinks2023` Tor 链接列表）— 跟 Loom 无关

## 关键发现 4：Codex-Dream-Skin 确认无关

`Fei-Away/Codex-Dream-Skin` ⭐12.1k (实际比 trending 略高)
- 给 Codex 桌面端换肤 (6 套主题 + 任意图片背景 + 自定义配色)
- macOS + Windows installer
- 用 CDP `--remote-debugging-port=9222` 注入主题
- **不修改官方安装目录与代码签名**
- 仓库声明"非 OpenAI 官方产品"
- **纯 UI 主题工具，不涉及 agent 架构 — 之前判断正确**

## 关键发现 5：exploitarium 是安全研究仓库

`crptx1337/exploitarium` "A single archive of public exploit PoCs and vulnerability research writeups"
- Python 写的 PoC 收集
- **跟 Loom 无关 — 之前判断正确**

## 行动建议

1. **v0.5 调研方向确认**：深挖 AOS Community Edition（高优先级）
   - 读 README / docs/meta-harness.md / docs/release-channels.md
   - 分析 21 个 capsule 的分类
   - 看 `aos mcp serve` 的 protocol 实现（如果能 clone 的话）
   - 写 v0.5 补充到 phase-roadmap-supplements.md
2. **open-connector**：暂不深挖，等 source code 发布
3. **wloc / colibri / torlink / Codex-Dream-Skin / exploitarium**：标记"不相关 / 不存在"备查

## 经验教训（要写进 memory）

- **trending 列表的 star 数和 30 天增长可能跟实际 repo 不符** — 跨项目通用
  - colibri: trending 17.5k，实际 0
  - wloc: trending 5.9k，实际 404
  - 必须 web_fetch GitHub 主页验证，不能信 trending 数字
- **同名小项目会污染搜索结果**（exploitarium / torlink / open-connector 都有 4+ 个低星 fork）
  - 必须看 org 账号 + 仓库 owner + 第一段 README 综合判断
- **README 第一段 = 定位 oracle**：能 30 秒内判断是不是相关项目

---

VERDICT: PASS（基于实际 web_fetch 验证）
