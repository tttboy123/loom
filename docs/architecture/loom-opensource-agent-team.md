# Loom 开源维护 Agent Team 与开源 Agent Gate

> 版本：2026-07-04。本文回答两个问题：
> 1. Loom 现有 Agent Team 是否足以支撑开源维护？不够的部分怎么补？
> 2. 如何把"开源维护"作为一个一等 Agent Gate，使其可被 Loom 自身调用、被 CI 调用、被本地脚本调用？

---

## 0. 一句话

**不重写 Loom Agent Team，而是在它之上加一条 `LoomReleaseLoop`（开源自维护流水线）和一个 `opensource_gate`（开源合规门）。** 两者都设计成可被 Loom 自己调用 + 可被 CI 调用 + 可被开发者本地脚本调用。

---

## 1. 现有 Agent Team 盘点

| 层 | 现有实现 | 落点 |
|---|---|---|
| 角色定义 | `STAGES: List[Stage]`（5 个：brainstorm / plan / implement / verify / review） | `devkit/rdloop.py:83` |
| 团队 | `rd_loop_team = Team(...)`（Agno Team） | `app/teams.py` |
| Gate | `evaluate_final_gate()` 三态判定（materialize 阶段） | `devkit/gate/__init__.py` |
| 角色配置 | `loom.roles.toml` / `loom.roles.json` | `devkit/roles.py` |
| Stage 模型 | `Stage(key, role, carrier, title, system_prompt)` | `devkit/rdloop.py` |

**形状**：研发流水线——`brainstorm → plan → implement(TDD) → verify(test) → review(独立)`。

**载体分工**（HANDOFF 当前真相）：

| 角色 | carrier | 默认厂商 | 用途 |
|---|---|---|---|
| brainstorm / plan / review | `loom-product / loom-orchestrator / loom-reviewer` | `codex-sub` | 控制面 |
| implement / verify | `loom-dev / loom-tester` | `MiniMax-M3` | 执行面 |
| fallback | — | `MiniMax-M2.7-highspeed` / GLM / DeepSeek | — |

---

## 2. 为什么现有 Agent Team 不够做开源维护

| 维度 | 研发流水线 | 开源维护 |
|---|---|---|
| 主要产物 | `.py` + `test_*.py` + 审查报告 | `LICENSE` / `.md` / `.yml` / `.gitignore` / mock 配置 |
| 验证方式 | pytest / contract / fuzz / schema | yaml lint / md lint / link check / 合规检查 / mock smoke |
| 决策标准 | 测试通过 + review APPROVE | LICENSE 在 / `.github/` 齐 / 路径脱敏 / mock 可跑 / 安全声明 |
| 失败归因 | `test_failed` / `lint_error` | `missing_license` / `path_leak` / `ci_broken` / `mock_smoke_failed` |
| 工具调用 | 文件读写 + shell | 文件读写 + shell + 许可证扫描 + 链接爬取 |
| 反复程度 | 单任务为主 | 周期性强（每次发布前） |

**强行套 5 阶段做开源维护会出现**：

1. `brainstorm` 不知道写什么 spec → 输出空泛
2. `verify` 的 system 提示是"跑测试"，对 LICENSE / .gitignore 完全无感
3. `review` 角色被大段 `.md` 文档吓到，给无意义 APPROVE

**所以**：复用现有 **角色 + 载体**，**新增一条流水线** 专门做开源维护。

---

## 3. 改进建议：加 `LoomReleaseLoop`（开源维护流水线）

### 3.1 Stage 设计

| Stage key | 复用角色 | 任务 | 推荐 carrier | 与 LoomRDLoop 的差异 |
|---|---|---|---|---|
| `audit` | `loom-product` | 跑 `opensource_gate` 审计，列缺失项 + 严重级 | `MiniMax-M3` | 完全新增 |
| `plan` | `loom-orchestrator` | 把缺失项按 (a) 纯模块 / (b) 接线 拆 + 排 sprint | `MiniMax-M3` | 复用 |
| `implement` | `loom-dev` | 写 `LICENSE` / `.md` / `.yml` / `.gitignore` / mock 配置 | `MiniMax-M3` | system 提示换：禁用代码 TDD 模板，改"开源制品实现"模板 |
| `comply` | `loom-tester` | 跑 `opensource_gate` + yaml/md lint + mock smoke | `MiniMax-M3` | system 提示换：不再跑 pytest，跑合规检查 |
| `review` | `loom-reviewer` | 跨厂商独立审查（默认用与 implement 不同厂商） | `codex-sub` | 复用 |

### 3.2 与 LoomRDLoop 的关系

- **不互替**：研发流水线继续做"产代码"，开源流水线继续做"产合规"
- **共享基础**：共用 `Stage` / `STAGES_BY_KEY` / `roles.toml` / `app.teams.py` 的 Agent 注册
- **独立开关**：`./loom release` 单独触发，不污染 `./loom run`

### 3.3 实现入口（落到代码）

```python
# devkit/rdloop.py  (新增)
STAGES_RELEASE: List[Stage] = [
    Stage("audit",     "product",       "loom-product",      "开源就绪度审计", AUDIT_SYS),
    Stage("plan",      "orchestrator",  "loom-orchestrator", "缺失项拆解",    PLAN_SYS),
    Stage("implement", "dev",           "loom-dev",          "开源制品实现",  IMPL_RELEASE_SYS),
    Stage("comply",    "tester",        "loom-tester",       "合规校验",      COMPLY_SYS),
    Stage("review",    "reviewer",      "loom-reviewer",     "独立审查",      REVIEW_SYS),
]
```

```python
# app/teams.py  (新增)
loom_release_team = Team(
    name="LoomReleaseLoop",
    model=leader_model,
    members=[
        ROLE_AGENTS["product"],
        ROLE_AGENTS["dev"],
        ROLE_AGENTS["tester"],
        ROLE_AGENTS["reviewer"],
    ],
    instructions=[ ... ],
)
```

实施成本：~50 行。**不在本次 PR 范围内做**，留作后续（避免过度耦合）。

---

## 4. 核心新增：`opensource_gate`

这是本文的重点 —— **把开源合规做成 Agent Gate**，与 `evaluate_final_gate` 同级但正交。

### 4.1 设计原则

1. **可组合**：每条 check 是独立 `OpenSourceCheck` dataclass
2. **可独立调用**：既可被 Loom pipeline 调用，也可被 `./loom doctor --opensource` / CI / pre-commit hook 调用
3. **严重级可分**：`blocker`（必须修） vs `warning`（建议修）
4. **判定可读**：输出结构化 dict，能直接进 RUNS.md / 控制台
5. **零依赖**：标准库 + pathlib，可离线跑

### 4.2 检查清单（v0.1）

| Check | severity | 说明 |
|---|---|---|
| `LicensePresent` | blocker | 根目录 `LICENSE` / `LICENSE.md` / `LICENSE.txt` 存在，且非占位 |
| `ContributingPresent` | blocker | `CONTRIBUTING.md` 存在 |
| `CodeOfConductPresent` | blocker | `CODE_OF_CONDUCT.md` 存在 |
| `SecurityPresent` | blocker | `SECURITY.md` 存在 |
| `GithubTemplatesPresent` | blocker | `.github/ISSUE_TEMPLATE/` + `.github/PULL_REQUEST_TEMPLATE.md` 存在 |
| `CIWorkflowPresent` | blocker | `.github/workflows/*.yml` 至少 1 个 |
| `GitignoreComplete` | blocker | 必须排除：`.schemathesis/` `.hypothesis/` `_diag/` `*.log` `devkit/MEMORY.md` `devkit/RUNS.md` |
| `WorktreeClean` | warning | `git status --porcelain` 为空（除 `.example` 文件外） |
| `MockModeRunnable` | warning | `litellm/config.mock.yaml` 可被 LiteLLM 加载 |
| `PathsScrubbed` | blocker | 提交文档中无 `./`, `$HOME/`, or `./scripts/...` 这类绝对路径 |
| `ConsentMentions` | warning | `SECURITY.md` 或 README 提及订阅代理的 ToS 风险 |

### 4.3 公开 API

```python
from devkit.gate.opensource import (
    evaluate_opensource_gate,    # → dict (verdict + checks)
    OPEN_GO, OPEN_NO_GO, OPEN_GO_WITH_WARNINGS,
    OpenSourceCheck,
    default_checks,
)

result = evaluate_opensource_gate(Path("/path/to/repo"))
# {
#   "verdict": "GO-WITH-WARNINGS",
#   "blockers": [],
#   "warnings": [{"check": "WorktreeClean", "detail": "..."}],
#   "checks": [...]
# }
```

### 4.4 调用入口

| 入口 | 命令 |
|---|---|
| CLI | `./loom doctor --opensource` |
| Loom pipeline stage | `python3 -m devkit gate opensource` |
| CI | `.github/workflows/ci.yml` 跑一次 |
| Pre-commit hook | `.git/hooks/pre-commit` 自定义 |

### 4.5 实现落点

- `devkit/gate/opensource.py` —— 检查实现 + 判定函数
- `tests/test_gate_opensource.py` —— 单元测试
- 与 `devkit/gate/__init__.py` 平行（**不破坏现有 `evaluate_final_gate`**）

---

## 5. 与 AUTONOMY-PLAN 的对齐

按照 `AUTONOMY-PLAN.md` §1「模块/接线分离」原则：

| 工作 | 类型 | 谁做 |
|---|---|---|
| `devkit/gate/opensource.py` 的纯函数（每条 check） | **(a) 纯模块** | 便宜模型（已派发过同类 gate 模块：`evaluate_final_gate`） |
| `opensource_gate` 接进 `app/teams.py` / `loom` CLI / CI workflow | **(b) 接线** | Claude（隐藏不变量：必须和现有 gate 正交、不破坏） |
| `tests/test_gate_opensource.py` | **(a) 纯模块** | 便宜模型 |
| `.github/` 模板、`.md` 文档、LICENSE | **(a) 纯制品** | 便宜模型 / 模板生成 |

> 本次 v0.1 范围：因为 Claude（即我）执行了所有工作，所以走"全部由 Claude 做"的快速通道；正式版本应该走 AUTONOMY-PLAN 的派发模式。

---

## 6. 不做的事（边界）

- ❌ 不重写现有 `LoomRDLoop` 团队
- ❌ 不替换现有 `evaluate_final_gate`
- ❌ 不在本次引入 `LoomReleaseLoop` 完整流水线（仅设计，不实施）—— 等 `opensource_gate` 稳定后单独 PR
- ❌ 不引入新依赖（保持 stdlib only）
- ❌ 不做跨平台差异（macOS first，Linux 通过 CI 验证，Windows 暂不支持）

---

## 7. 后续 PR 路径建议

1. **PR #1（本设计落地）**：LICENSE + CONTRIBUTING + COC + SECURITY + .github/ + .gitignore + 路径脱敏 + mock mode + `opensource_gate`
2. **PR #2（数据脊 + 协议）**：把 `opensource_gate` 的结果结构化成 `EvidencePacket`，接进 RUNS.md / 控制台
3. **PR #3（LoomReleaseLoop）**：实现完整 5 阶段流水线，roles.toml 默认启用
4. **PR #4（自动发布）**：tag → CI → GitHub Release → Docker image 自动化