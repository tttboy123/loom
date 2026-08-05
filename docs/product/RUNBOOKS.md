# Loom Runbook 全集

本页索引全部正式 runbook。所有 runbook 使用同一套已验收的替代验证方法
（Product Owner 指令 2026-08-04）：生产 Swift 客户端经真实 daemon socket
驱动变更与读取、真实 PTY TUI、真实原生窗口以 `--socket --journey-id`
启动并由 `screencapture` 截图；不使用 Computer Use 窗口自动化。

每个 runbook 的配套验证脚本做最终冻结校验：证据模式、journey_id 无漂移、
双客户端 IPC、TUI transcript、截图、SQLite 完整性、零重复 Event 身份、
零流缺口、投影与 Journal 一致、postflight 全空、无密钥类材料。

## 全集索引

| Runbook | 场景 | 验证脚本 | 已消费旅程根 | journey_id | 状态 |
|---|---|---|---|---|---|
| [Phase 3A cross-client journey](../runbooks/phase3a-cross-client-journey.md) | P3A-W1 演化资产/材料化（8 场景：happy、cancel/reject/retain、stale-view、并发单赢、crash-before/after-CAS、投影失败、慢客户端重连） | `scripts/verify-phase3a-cross-client-journey.sh` | `.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md` 列出的 8 个 `-r2` 根 | 8 个（见库存清单） | PASS |
| [SF-W1 cross-client journey](../runbooks/sf1-cross-client-journey.md) | Queue 状态/投影 + admission/eligibility/冲突仲裁 | `scripts/verify-sf1-cross-client-journey.sh` | `/private/tmp/sf1-journey-final` | `70862374-d9cd-4ac6-a02f-2be68344bdac` | PASS |
| [SF-W2 cross-client journey](../runbooks/sf2-cross-client-journey.md) | Worker Pool + lease/fencing/reconciler/routing/有界恢复 | `scripts/verify-sf2-cross-client-journey.sh` | `/private/tmp/sf2-journey-final` | `d0d5f598-617b-4bb9-b0bb-81162c83dbbc` | PASS |
| [SF-W3 cross-client journey](../runbooks/sf3-cross-client-journey.md) | 单写集成 + 受控 canary + Timeline/Attention | `scripts/verify-sf3-cross-client-journey.sh` | `/private/tmp/sf3-journey-final` | `addcee11-8224-40db-b665-fa6a40a82e22` | PASS |
| [v0.4.1-W1 cross-client journey](../runbooks/v041-w1-cross-client-journey.md) | 产品主线闭环：Queue/Workers/Integration 三屏真实操作 | `scripts/verify-v041-w1-cross-client-journey.sh` | `/private/tmp/v041-w1-journey` | `105723c8-0595-4f6d-9890-0b411f16cb01` | PASS |
| [v0.4.1-W2 cross-client journey](../runbooks/v041-w2-cross-client-journey.md) | 两个真实并行开发 Candidate（框架驱动） | `scripts/verify-v041-w2-cross-client-journey.sh` | `/private/tmp/v041-w2-journey` | `63d4ea90-588d-4359-bb0c-942a2c767020` | PASS |
| [v0.4.1-W3 self-host canary](../runbooks/v041-w3-self-host-canary.md) | 可重复受控 canary（幂等 CAS、旧视图保留、崩溃恢复） | `scripts/verify-v041-w3-self-host-canary.sh` | `/private/tmp/v041-w3-journey` | `fbae6a98-3a88-44f7-8d65-67b62626d55b` | PASS |

## 逐本说明

### Phase 3A cross-client journey

P3A-W1 的最终用户旅程门禁，覆盖 8 个故障/并发场景。最终根集合与二进制
摘要记录在
`.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md`。断言包括
演化资产生命周期（create/evaluate/activate/bind/execute）、stale-view
与错误 digest 拒绝、并发单赢、crash-before/after-CAS、投影失败保留旧
视图、慢客户端干净重启。

### SF-W1 / SF-W2 / SF-W3 cross-client journey

v0.4.0 三个垂直 WorkItem 各自的门禁旅程：

- **SF-W1**：两个 Job 入队 + Gap Proposal 收敛 + 重启重建队列状态。
- **SF-W2**：worker claim/lease、test_defect 失败进 Repair、崩溃回收
  （新 generation）、重启后 attempt 状态一致。
- **SF-W3**：单写集成（竞争 CAS 输家零副作用）、一次性 canary + 重复
  拒绝、授权/未授权流式帧、later-Run adoption 与 rollback、投影故障
  保留旧视图。

### v0.4.1-W1 / W2 / W3

产品主线闭环与最强证明：

- **W1**：真实 PTY TUI 完成 queue→claim→test→review→integrate→adopt→
  rollback 全闭环；Swift 客户端读回同一投影；daemon 重启重建一致。
- **W2**：两个 Job 并行 claim（TUI 与 Swift 客户端各驱动一个 worker），
  B 的 `product_defect` 失败在 A 完成前记录并路由到 Repair lane，单
  Integrator 只落一个版本化 Release，竞争集成被 CAS 拒绝零副作用。
- **W3**：一次离线 canary（锁定 runtime/model/skill 绑定）；重复启动被
  幂等 CAS 拒绝；投影故障期间旧视图保留；重启后无重复重建。

## 如何执行一次跨客户端旅程

1. 新建旅程根（0700 目录、`state/loom.db` 0600 先 touch），manifest
   purpose 必须为 `phase3a-cross-client-e2e`，生成唯一 UUID journey_id。
2. 在 PTY 会话启动 daemon（`--state`/`--isolation-root`/`--probe-id`/
   `--socket` + runtime-dir ×2 + 可选 local-model 三件套），环境变量
   `LOOM_P3A_CONTROLLED_JOURNEY_MANIFEST` 指向 harness manifest。
3. 启动原生 app（`open ROOT/app/Loom.app --args --socket ... --journey-id ...`），
   确认 IPC 日志出现 `loom-swift-*` 启动读。
4. 用 `LoomLocalAppContractProbe`（Swift 生产客户端）或真实 PTY TUI
   驱动操作；`screencapture -x -l <wid>` 截关键检查点。
5. 执行重启检查点（停 daemon→重启→双客户端读回重建状态）。
6. 冻结证据（actions/transcript/keystrokes/timeline/IPC/daemon log/
   journal 摘要/投影摘要/digest 校验/preflight+postflight/cleanup-proof/
   result.md），跑配套 verify 脚本。

> 已消费的旅程根是证据而非可重跑 fixture；重跑需要新的独立 Review PASS
> 与新的 journey_id/根。失败轮次保留为历史材料，不覆盖。
