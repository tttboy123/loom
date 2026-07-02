# Autonomous Agent Team

本文说明如何把 Loom 部署成一个可自动迭代的本地自治 Agent Team。

## 1. 启动服务

```bash
cd /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/agent-platform
./loom up
./loom doctor
```

默认 `./loom up` 启动 lite 核心：

- 控制台：`http://localhost:8899`
- LiteLLM 网关：`http://localhost:4000`
- 订阅代理：`http://localhost:8317`

需要聊天 UI / AgentOS API 时再启动 full profile：

```bash
./loom up full
```

## 2. 模型分工

当前执行链路默认是 5 个成员；另有 1 个“需求发现 / 方向”成员用于自动迭代时补充 backlog。

| 阶段 | 角色 | carrier | 模型 |
| --- | --- | --- | --- |
| `discover` | 需求发现 / 方向 | `loom-product` | GPT-5.4 |
| `brainstorm` | 产品判断 | `loom-product` | GPT-5.4 |
| `plan` | 编排 | `loom-orchestrator` | GPT-5.4 |
| `implement` | 开发 | `loom-dev` | MiniMax-M3 |
| `verify` | 测试 | `loom-tester` | MiniMax-M3 |
| `review` | 审查 | `loom-reviewer` | GPT-5.4 |

`discover` 不默认插入每个开发 run。它在这些时机工作：

- backlog 为空，需要发现下一批项目发展需求。
- 定期做 roadmap / fitness / failure-classification 复盘。
- 扫描 `external-requests/`，把外部需求拆成通用 Loom 上游能力。
- 项目级迭代时，根据 `progress.md`、测试缺口、用户目标生成下一批 feature。

当前代码里已有低阶能力：

- `devkit/discover.py`：从历史 fitness / suggestions 生成候选任务。
- `devkit/valuer.py`：给候选任务评分。
- `devkit/__main__.py::_refill_backlog()`：backlog 清空时自动补任务。

但它还不是完整的 LLM 产品战略角色。要做到真正无人值守演进，下一步应把 `discover` 升级为正式成员：读取 vision / roadmap / run history / external requests，输出结构化候选需求，再交给 `valuer` 排序。

修改入口：

- `litellm/config.full.yaml`：carrier 到真实模型的映射和 fallback。
- `loom.roles.example.toml` / `loom.roles.toml`：阶段顺序、角色提示词、executor、token 上限。

修改模型配置后重启：

```bash
docker compose restart litellm console
./loom doctor
```

管理/审查层的 fallback 顺序是 `GLM -> MiniMax`；开发/测试层是 `GLM -> DeepSeek`。这样保留 GPT-5.4 作为产品/编排/审查主力，同时避免编排阶段在 GPT 不可用时直接落到 MiniMax 的 thinking-only 空正文。

## 3. 跑一次 Agent Team

```bash
./loom run "实现一个小功能，要求有测试和独立审查"
```

`./loom run` 当前包装的是：

```bash
python3 -m devkit auto "<任务>" --yes --loop
```

它会把自然语言任务转成 backlog，并启动自治循环。

## 4. Loom 自身的自动迭代

Loom 自身能力迭代使用 `devkit/backlog.json`：

```bash
# 只看下一轮选中什么
python3 -m devkit auto --backlog devkit/backlog.json --dry-run

# 跑一轮
python3 -m devkit auto --backlog devkit/backlog.json --yes

# 连续跑，直到没有就绪任务
python3 -m devkit auto --backlog devkit/backlog.json --yes --loop

# 新流程：每轮执行 1 个任务 -> MiniMax 反思 -> 写回 backlog -> 继续下一轮
python3 -m devkit iterate --backlog devkit/backlog.json --max-rounds 20
```

状态机：

```text
pending -> running -> done
pending -> running -> failed
```

每轮产物：

```text
devkit/runs/<run-id>/
devkit/decisions.jsonl
devkit/RUNS.md
```

当 `--loop` 遇到无就绪任务时，会尝试 `_refill_backlog()`。目前这个 refill 主要根据历史 model fitness 发现“能力缺口”，不是完整产品战略。因此自动迭代的真实闭环应是：

```text
discover 发现候选需求
  -> valuer 评分排序
  -> backlog 写入 pending task
  -> brainstorm / plan / implement / verify / review 执行
  -> run history / fitness / failures 回流给 discover
```

现在推荐把这个闭环交给 `devkit iterate` 跑，而不是只用 `auto --loop`：

- `auto --loop`：连续消费现有 backlog，遇到无就绪任务时只尝试 `_refill_backlog()`。
- `iterate`：每轮执行完后读取最新 `run-log.md`、`backlog.json`、`decisions.jsonl`，用反思代理决定是否：
  - 把失败任务重新入队
  - 提升已有任务优先级
  - 新增一个具体、可测试的修复/拆分任务
  - 停止本轮自治

`iterate` 默认用 `minimax` 做反思，不使用 codex 额度。反思产物会落在：

```text
devkit/reflections/<round>-<run-id>.md
```

如果当前 backlog 没有就绪任务，但存在 `failed` 或被依赖卡住的任务，`iterate` 会先触发一次 `stalled reflection`，尝试自动补出下一条可执行任务，而不是直接退出。

## 5. 后台无人看守持续迭代（推荐）

`devkit iterate` 本身是前台阻塞命令。要放到后台长期跑，可用标准 Shell 方式。

```bash
# 一次提交命令到后台（推荐）
nohup python3 -m devkit iterate \
  --backlog devkit/backlog.json \
  --max-rounds 20 \
  --reflect-carrier minimax \
  --compact-model deepseek \
  > devkit/logs/iterate-daemon.log 2>&1 &

tail -f devkit/logs/iterate-daemon.log
```

如果希望“自动补空后等待再尝试”，建议用仓库内置脚本：

```bash
./scripts/loom-iterate-daemon.sh \
  --backlog devkit/backlog.json \
  --max-rounds 20 \
  --sleep 90 \
  --log-file devkit/logs/iterate-daemon.log

# 后台运行：
nohup ./scripts/loom-iterate-daemon.sh --sleep 90 --log-file devkit/logs/iterate-daemon.log \
  > /tmp/loom-iterate-daemon.out 2>&1 &
```

如果希望“跑起来后每 5 分钟自检并自动修复”，用新的监督器入口：

```bash
nohup ./loom autopilot \
  --sleep 90 \
  --check-interval 300 \
  --log-file devkit/logs/iterate-daemon.log \
  --supervisor-log-file devkit/logs/iterate-supervisor.log \
  > /tmp/loom-autopilot.out 2>&1 &
```

监督器会自动处理：

- 迭代子进程（`loom-iterate-daemon.sh`）挂掉或消失则重启
- backlog 长时间处于 running 且 `_attempts >= 2` 触发 `backlog stuck` 时重启
- iterate 日志新增出现明显异常（`Traceback` / `Exception` / `selected model is at capacity`）时尝试 `./loom up` + 重启

检查/停机：

```bash
tail -f devkit/logs/iterate-supervisor.log
ps -ef | grep "loom-iterate-supervisor\|loom-iterate-daemon" | grep -v grep
kill <supervisor pid>
./loom task-queue devkit/backlog.json       # 查看当前队列与“正在开发任务”
tail -f devkit/logs/task-queue-status.log  # 每次巡检已记录的任务序列（JSONL）
```

如果只想先“验通命令”，先用 `--once` 跑一次单批次：

```bash
./scripts/loom-iterate-daemon.sh --once --max-rounds 3 --backlog devkit/backlog.json
./loom autopilot --once --max-rounds 3 --backlog devkit/backlog.json
```

终止后台任务可用：

```bash
ps -ef | grep "loom-iterate-daemon\|devkit iterate --backlog" | grep -v grep
kill <PID>
```

## 6. 具体项目的自动迭代

项目级自动迭代使用 `devkit/projects/<project>/backlog.json`，适合“给一个产品想法，自己一轮轮长出来”。

```bash
# 初始化项目特性池
python3 -m devkit backlog "做一个本地任务管理 CLI" --dir devkit/projects/todo-cli --features 8

# 连续实现 3 个特性，失败时每个特性最多修 2 轮，测试绿后本地 commit
python3 -m devkit feature --dir devkit/projects/todo-cli --count 3 --iterate 2 --commit

# 查看进度
python3 -m devkit backlog --status --dir devkit/projects/todo-cli
```

状态机：

```text
todo -> done
```

项目级 feature loop 会读取当前项目代码，要求模型产出测试，物化文件，运行测试；测试通过才标记 done。

项目级自动迭代同样需要 `discover`。当前 `devkit backlog "想法"` 是一次性 initializer，`devkit feature` 只消费已有特性池；如果要长期演进一个项目，需要增加“项目 strategist”循环：读 `progress.md`、现有代码、测试报告、用户目标，补充下一批 `features`。

## 7. 外部需求池到 Agent Team

`external-requests/` 是外部项目给 Loom 的上游能力需求池。它不是直接执行队列。

标准流程：

1. 扫描 `external-requests/requests.yaml`。
2. 用 Agent Team 做 accept / reject / split。
3. 只接受通用 Loom 能力，不接受下游项目专用 glue。
4. 把可执行小切片写入 `devkit/backlog.json` 或直接实施。
5. 更新 `external-requests/intake/*.md` 和 `requests.yaml`。

当前状态：

- `loom-upstream-001`：accepted_split。
- `loom-upstream-002`：slice_1_implemented。
- `loom-upstream-003`：accepted_deferred。
- `loom-upstream-004`：accepted_deferred。
- `loom-upstream-005`：slice_1_mapping_implemented。

## 8. 最小部署检查清单

```bash
./loom up
./loom doctor
python3 -m devkit roles list
python3 -m devkit auto --backlog devkit/backlog.json --dry-run
```

期望：

- console / litellm / cliproxy 至少返回健康状态。
- `loom-product`、`loom-orchestrator`、`loom-dev`、`loom-tester`、`loom-reviewer` 可服务或有明确 fallback。
- `devkit auto --dry-run` 能选出下一条就绪任务，或明确报告无就绪任务。

## 9. 安全边界

- 默认 L1 / report-only。
- 不自动 push、不自动发布、不自动合并。
- `--apply`、`--apply-git`、真实项目 commit / push 都是人类门。
- 订阅代理有服务条款和限流风险，需用户确认后使用。

## 10. 当前失败队列分组与自治处理顺序

截至 `2026-07-02`，`devkit/backlog.json` 中失败任务共 `25` 条。不要平均处理，应该按“先恢复自治主链，再清理下游任务”推进。

### A. 需求发现与 backlog 回灌主链

任务：

- `#178 discover-contract`
- `#180 external-verification-surface`
- `#181 audit-failed-decisions`
- `#223 audit-failed-decisions-retry-2`

为什么优先：

- 这是“发现需求 -> 评分 -> 入队 -> 可信验证”的最小主链。
- 这组不恢复，自治循环只能消费旧 backlog，不能稳定长出下一轮需求。

当前判定：

- `discover-contract` 与 `external-verification-surface` 都已经跑过自动执行，但停在 `REQUEST-CHANGES` / `NO-GO`。
- 这组应优先转成 backlog-ready、test-backed contract，而不是继续补零散 util。

### B. sandbox / materialize / shadowing 主故障链

任务：

- `#184 sandbox-module-shadowing-guard`
- `#207 sandbox-module-shadowing-guard-fix`
- `#213 diagnose-stuck-task-pattern`
- `#214 diagnose-stuck-task-pattern-v2`
- `#215 diagnose-implement-zero-token-shortcircuit`
- `#217 fix-sandbox-build-injection`
- `#225 locate-rdloop-build-materialize`
- `#239 fix-rdloop-report-skip-build`
- `#246 setup-test-isolation-fix`
- `#247 setup-keys-rename-and-test`
- `#260 diag-materializer-detection`
- `#270 setup-keys-rename-and-test-2`

为什么第二优先：

- 这是当前自治循环反复卡住的真实执行故障链。
- 症状已经很明确：`implement/verify` 阶段存在 `0 tok / 0 文件 / 0 费用 / 仍判 OK`、sandbox build 注入、`setup.py` 测试隔离冲突、report 任务仍误走 build。

处理策略：

- 先做诊断与契约修正，再做功能修复。
- 先保证 report 类任务能跳过 build，再修 materializer，再处理 shadowing 返回值细节。

### C. env / API key / fallback 观测链

任务：

- `#224 diag-missing-api-keys`
- `#234 env-setup-fail-loud`
- `#235 setup-required-keys-export`
- `#236 diag-env-presence-no-key`
- `#237 seed-env-template`
- `#249 env-audit-strict`

为什么第三优先：

- 这组影响自治是否“失败得足够响亮”。
- 它很重要，但不是当前 stuck 的第一根因；当前更大的问题是执行链假阳性和物化路径错误。

### D. gate / 非代码任务判定链

任务：

- `#250 adjust-noncode-gate`

作用：

- 解决 `.md`、`.json`、`.env.example` 等非代码任务即使实现正确也可能被错误打回的问题。
- 这项应该在 B 组稳定后尽快处理，否则 report-only 和配置类任务会持续误判。

### E. 低层 utility 尾部任务

任务：

- `#174 ip-utils`
- `#176 unit-converter`

说明：

- 这两项不是当前自治能力恢复的关键路径。
- 除非它们被用于回归验证某条主链能力，否则应继续后置。

## 11. 推荐自治执行顺序

当前推荐顺序不是按任务编号，而是按主链恢复价值：

1. `#239 fix-rdloop-report-skip-build`
2. `#225 locate-rdloop-build-materialize`
3. `#260 diag-materializer-detection`
4. `#246 setup-test-isolation-fix`
5. `#178 discover-contract`
6. `#180 external-verification-surface`
7. `#181 audit-failed-decisions`
8. `#223 audit-failed-decisions-retry-2`
9. `#224 #234 #235 #236 #237 #249` 这一组 env 诊断与 loud-fail
10. `#250 adjust-noncode-gate`

执行原则：

- 优先恢复“report 任务可跑、materializer 可诊断、verify source-of-truth 可证明”。
- 对同一根因的重复任务，只保留一个主任务继续推进，其余作为诊断证据保留，不要并行写同一故障面。
- 每轮巡检都记录：当前运行任务 id、队列序号、所属故障组、最近 run-id、结果状态。

## 12. 当前自治运行面的已知问题

当前仓库的 `autopilot` / `iterate daemon` 入口已经具备：

- 后台拉起 iterate
- 300 秒巡检
- backlog stuck 检测
- 错误日志扫描
- 任务队列快照落盘

但本地实测显示一个现实问题：

- 日志显示监督器成功启动过 worker，并进入首轮巡检。
- 当前 `pid` 文件仍存在，但 `ps` 查不到对应进程，说明后台进程存在非稳定退出面。

因此当前自治策略应是：

1. 保留 `task-queue-status.log` 作为每轮巡检账本。
2. 每次重拉后台循环后，立刻验证 `ps`、`iterate-supervisor.log`、`iterate-daemon.log` 三处信号。
3. 在主链任务恢复同时，把守护进程稳定性当作自治基础设施的一部分持续修。
