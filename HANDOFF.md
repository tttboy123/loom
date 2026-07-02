# Loom - Agent Handoff

> 当前版：2026-07-01。给新的 Claude Code / Codex / 其他开发 Agent 接续使用。

## 0. 先读什么

- 项目根：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform/`
- 当前目录不是 git 仓库，不要假设能用 `git status` 复核。
- 先读当前真相：
  - `README.md`
  - `LOOM-ROLES.md`
  - `devkit/README.md`
  - `loom.roles.example.toml`
  - `litellm/config.full.yaml`
  - `docs/autonomous-agent-team.md`
- 历史设计背景：
  - `STRATEGY-2026-06-27-eval-and-pipeline.md`
  - `ROADMAP-INTEGRATED-2026-06-27.md`
  - `DESIGN-P0-artifact-bus.md`

## 1. 当前定位

Loom 是本地、额度感知、可验证的自治软件工厂。它不是普通聊天工具，也不是产品 runtime；它把产品判断、编排、开发、验证、独立审查组织成可回放的多模型研发闭环。

## 2. 当前模型分工

当前默认策略以 `litellm/config.full.yaml` 和 `loom.roles.example.toml` 为准：

| 角色 | carrier | 当前默认模型 |
| --- | --- | --- |
| 需求发现 / 方向 | `loom-product` | `codex-sub` |
| 产品判断 | `loom-product` | `codex-sub` |
| 编排 | `loom-orchestrator` | `codex-sub` |
| 开发 | `loom-dev` | MiniMax-M3 |
| 测试 / 验证 | `loom-tester` | MiniMax-M3 |
| 独立审查 | `loom-reviewer` | `codex-sub` |

保留的订阅备用：

| carrier | 用途 |
| --- | --- |
| `codex-sub` | ChatGPT/Codex 订阅代理备用 |

不要再使用旧口径 `loom-tester / loom-reviewer = gpt-5.3-codex-spark`。当前控制面统一走 `codex-sub`，执行面走 `MiniMax-M3`。

## 3. 开发方法

Claude 已废弃移除。当前推荐分工是：

- `codex-sub`：需求发现、产品判断、编排、独立审查。
- `MiniMax-M3`：开发、测试。
- `MiniMax-M2.7-highspeed / MiniMax-M2.7`：统一 fallback。
- GLM：管理/审查层首选 fallback。
- Codex / Claude 订阅：作为备用 executor / carrier，按需接入。

`discover` 是自动迭代的前置角色，不默认插入每个开发任务。它负责在 backlog 为空或定期复盘时发现下一批发展需求，再交给 `valuer` 排序。

常用命令：

```bash
./loom up
./loom doctor
./loom run "实现一个小功能，要求有测试和独立审查"
./loom status
```

跑指定阶段：

```bash
python3 -m devkit "任务" --stages brainstorm,plan,implement,verify,review
```

按阶段临时覆盖：

```bash
python3 -m devkit "任务" \
  --carrier implement=loom-dev \
  --carrier verify=loom-tester \
  --carrier review=loom-reviewer
```

## 4. 两套自动迭代

### Loom 自身 / Agent Team 队列

使用 `devkit/backlog.json`：

```bash
python3 -m devkit auto --backlog devkit/backlog.json --dry-run
python3 -m devkit auto --backlog devkit/backlog.json --yes
python3 -m devkit auto --backlog devkit/backlog.json --yes --loop
```

状态机：`pending -> running -> done/failed`。

### 项目级特性迭代

使用 `devkit/projects/<project>/backlog.json`：

```bash
python3 -m devkit backlog "做一个本地任务管理 CLI" --dir devkit/projects/todo-cli --features 8
python3 -m devkit feature --dir devkit/projects/todo-cli --count 3 --iterate 2 --commit
python3 -m devkit backlog --status --dir devkit/projects/todo-cli
```

状态机：`todo -> done`，测试通过才标记完成。

## 5. 外部需求池

`external-requests/` 是外部项目给 Loom 的上游能力需求池，不是直接执行队列。

处理节奏：

1. 扫描 `external-requests/requests.yaml` 中的 `pending_scan`。
2. Agent Team 做 accept / reject / split。
3. 把通用 Loom 能力拆成 `devkit/backlog.json` 或直接最小实现切片。
4. 更新 `external-requests/intake/*.md` 和 `requests.yaml` 状态。

当前已处理：

- `loom-upstream-001`：accepted_split。
- `loom-upstream-002`：slice_1_implemented，新增 `devkit/materialization_contract.py`。
- `loom-upstream-003`：accepted_deferred。
- `loom-upstream-004`：accepted_deferred。
- `loom-upstream-005`：slice_1_mapping_implemented，新增 `devkit/failure_classification.py`。

## 6. 验证命令

文档或配置修改后至少运行：

```bash
ruby -e 'require "yaml"; YAML.load_file("litellm/config.full.yaml"); YAML.load_file("external-requests/requests.yaml"); puts "yaml ok"'
```

代码或 devkit 变更后运行相关测试：

```bash
PYTHONPATH=. python3 -m unittest devkit.test_features -v
```

快速健康检查：

```bash
./loom doctor
```

## 7. 重要边界

- 默认 L1 / report-only，不自动改真实仓库、不 push、不发布。
- apply / apply-git / commit / push 都是人类门。
- Claude / ChatGPT 订阅代理可能触及服务条款和限流风险，是否使用由用户决定。
- 历史设计文档可能有过时模型名，当前模型真相以 `litellm/config.full.yaml`、`loom.roles.example.toml` 和本文件为准。
