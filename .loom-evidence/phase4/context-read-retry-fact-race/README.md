# context-read 重试 attempt 事实竞态修复（commit a19afb6d）——live 验证

## 根因（完整 goroutine dump + 分阶段 debug）
- 重试 attempt 使用确定性 Context call id（DeliveryCallID(proposal, seq)）。
- 重试 attempt 的第二次 context-read 派发后，
  `DeliveryCoordinator.Prepare` 的 fact lookup 与上一 attempt 的 payload index
  竞态/冲突 → 已派发调用无 delivered fact → `AttemptLoopAuthority.EndStep`
  报 `Attempt loop conflict` → attempt 永久卡在 working。
- debug 证据（WRAP-DEBUG/SUP-DEBUG）：delegate.Execute err=nil（adapter 成功），
  但 wrapper 的 EndStep 返回 Attempt loop conflict。

## 修复（RED-first）
`DeliveryCoordinator.Prepare`：
- fact lookup 错误路径 → 尝试 `writeBoundedDeniedPayload`（写 + accept
  "context_item_unavailable" fact），成功即返回 payload；
- retriever 有界拒绝路径统一走同一 helper。
这样每个已派发 Context 调用都有 delivered fact，step 可 finalize。
新测试：`TestDeliveryCoordinatorWritesDeniedPayloadWhenFactLookupRaces`。

## live 验证（debug daemon，UI Final Team 等真实 Mission）
- 首次 attempt：context-read 有界拒绝 → denied payload 写入 → ack ok →
  StepEnded/TurnEnded → attempt 成功（err=nil）。
- 重试 attempt 不再卡死（此前永久 running），可推进到 awaiting_recovery。
- 安装版 App 冷启动正常：Local service ready / Chat ready / Agent Team ready。

## 测试
Go contextcapsule/nativeadapter/app/work/supervisor 全绿；
Swift 254 绿（1 跳过）；gofmt + git diff --check 干净。
