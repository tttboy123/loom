# P2D-W4 · Loom Mission 独立 Verifier + 双节点收敛安装版 live 验收（V33c）

## 结论
安装版 Loom（`/Users/lune/Applications/Loom.app`）真实 Mission 已**完整收敛到
`succeeded`**：子 Agent 模型主动调用受治理的 `loom_web_search`/`loom_web_fetch` →
Attempt-Loop 准入 → 授权 → 真实执行 → 结果投递 → 独立 Verifier 真实跑模型并验收 →
主节点派发并验收 → `TeamExecutionTerminal`。验收测试
`TestLiveMissionAgentCallsWebSearch`（`LOOM_LIVE_WEB_MISSION=1 LOOM_LIVE_TEAM_E2E=1 LOOM_LIVE_NET=1`）
**PASS（30.88s）**，Journal 记录 WebSearch/WebFetch 工具事实 20 条。

## 本轮修复（把 V33 已知剩余缺陷全部闭合）
1. **独立 verifier dispatch 无法被真实 adapter 解码**（V33 遗留 "verifier 重试报
   binding_changed" 的根因）：
   - `runIndependentVerifier` 之前发送的是纯数据 payload（TeamInstanceID/Criteria…），
     DeepSeek native adapter 的 `decodeDeepSeekAgentDispatch` 无法解码（既非
     `loom_context_capsule_prompt` 也非 `pi_rpc_prompt`）→ verifier run 恒为
     `bridge_protocol_failed` → `VerifierCandidateFromTerminal` 报
     `invalid verifier candidate` → Mission flight 静默死亡。
   - 修复：verifier dispatch 改为自包含单 prompt 信封
     `{"schema_version":1,"kind":"pi_verifier_prompt","prompt":…}`；prompt 由
     `renderVerifierPrompt` 生成，携带验收标准、source 摘要摘要（digest-only）与
     **受控的 source 输出摘录**（`evidence.ReadArtifact` 按 digest 读取，8 KiB 有界）。
   - `decodeDeepSeekAgentDispatch` 接受 `pi_verifier_prompt` 并把模型输出解析为
     verdict（`criteria_satisfied`/`criteria_not_satisfied`/`insufficient_evidence`），
     未解析或缺失时 fail-closed 为 `insufficient_evidence`。
2. **Verifier 证据提交 replay 缺账户流**：`CommitVerifierEvidence` /
   `CommitTeamNodeAcceptance` / `validateVerifierAcceptanceLineage` 的快照缺少
   `provider-account-policy` / `provider-account-capacity` / `provider-model-rate-card`
   流，而 claim contract v2 的 RunClaimed 携带 rate-card 引用 → replay 报
   `run authority conflict` → verifier 证据永远提交不上。修复：三处均按
   `CommitTeamAttemptEvidence` 的模式扩展账户流后重读快照。
3. **Verifier prompt 缺少 Mission objective**：验收标准含 "result satisfies the
   confirmed Mission objective"，但 verifier 看不到 objective 文本 → 模型判
   `insufficient_evidence` → 触发 recovery。修复：`TeamExecutionRequest.Objective`
   携带 objective 并写入 verifier prompt。
4. **web_fetch 失败是致命错误**：`toolbroker.Broker.webFetch` 对传输错误/非 2xx/空内容
   返回 `ErrToolFailed` → 整个 attempt 失败 → attempt-loop `EndStep` 因未投递事实
   报 `Attempt loop conflict` → run 变 `runtime_process_failed` → Mission 停在 running。
   修复：与 webSearch 一致，把 fetch 失败转为受控 `fetch_*` 错误消息（bounded tool result），
   模型可恢复，attempt-loop 可闭合。
5. **主节点 WorkItem 标题超长不可重放**：Mission objective 标题（132 字节）超过
   `maxAuthorityIDBytes=128`，`DispatchTeamReadySet` 照写，但 `replayWorkItemStream`
   用 `validOpaqueID` 校验拒绝 → `Snapshot()` 报 `run authority conflict` → 主节点
   supervisor `validateInput` 失败 → 主节点永不派发。修复：新增
   `maxWorkItemTitleBytes=4096` + `validWorkItemTitle`，写入端与重放端一致。

## Live 证据（2026-08-17 23:47 SGT，安装版）
```
=== RUN   TestLiveMissionAgentCallsWebSearch
    deepseek verified
    governance configured for deepseek/deepseek.primary (deepseek-chat)
    mission start status=running note=""
    mission board status=succeeded
    journal web tool facts: WebSearch/WebFetch = 20
--- PASS: TestLiveMissionAgentCallsWebSearch (30.88s)
```

Journal 关键事件计数（同一 loomb.db）：
- `ToolCallAdmitted / ToolDispatchCommitted / ToolExecutionAllowed /
  ToolExecutionCompleted / ToolResultAccepted / ToolResultDelivered` 各 10
- `EvidenceSubmitted` 5（subagent×2 尝试、verifier×2、main）
- `WorkItemVerificationCommitted` 2、`WorkItemDone` 2、`TeamNodeAcceptanceCommitted` 2
- `TeamExecutionTerminal` 1、`TeamNodeRecoveryRecorded` 1（首次尝试被 verifier 拒后
  recovery 成功）
- `RunClaimed / RunStarted / RunTerminalCommitted` 各 5

## 测试门禁
- `go test ./... -count=1` 全绿（无 FAIL）。
- `apps/macos/swift test` 通过。
- `git diff --check` 干净。
- 安装版 live 测试通过（见上）。
