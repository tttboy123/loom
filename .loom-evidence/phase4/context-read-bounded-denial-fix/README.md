# context-read 有界拒绝修复（commit 3f529e7e）——live 验证

## 根因（goroutine dump + 分阶段 debug 定位）
无真实会话胶囊启动的 Mission，首次 context-read 派发后停滞。两层根因：
1. `DeliveryCoordinator.Prepare`：retriever 有界拒绝（ErrContextItemNotRetrievable）
   直接返回错误，Attempt-loop step 里已派发的 Context call 没有 delivered fact，
   EndStep 报 `Attempt loop conflict`，attempt 永久卡死。
2. `deepSeekAgentAdapter`：同一步内第二次 context-read 无法派发
   （ErrInvalidContextDelivery 且已有投递）时终态失败，同样触发
   `Attempt loop conflict`。

## 修复（RED-first，各有新测试）
- coordinator：有界拒绝时写入 + accept "context_item_unavailable" payload fact，
  使 step 可 finalize。
- adapter：第二次无法派发时以有界 content-free 结果结束 exchange。

## live 验证（安装版 daemon）
- 修复前：attempt-loop 永久停在第一个 ToolDispatchCommitted。
- 修复后：IPC Mission 第一次 attempt 完成 4 次有界 context-read
  （ToolCallAdmitted/ToolDispatchCommitted ×4）→ **StepEnded → TurnEnded**，
  attempt 正常推进（此前永远卡死）。
- App 冷启动正常：Local service ready / Chat ready / Agent Team ready。

## 测试
Go：contextcapsule / nativeadapter / app / work / supervisor 全绿；
Swift 254 绿（1 跳过）；gofmt + git diff --check 干净。

## 已知边界（诚实）
重试的 attempt 在测试环境（叠加多个历史卡死 Mission、journal 并发恢复）
下仍可能在首次 context-read 派发后停滞；Mission 到 succeeded 的完整收敛
仍需一次聚焦排查（重试路径 / journal 并发竞争）。
