# RoundTable 用户视角全旅程验证 + 引导按钮修复（2026-08-20）

## 发现（用户视角走查）
RoundTable "Run full journey" 引导按钮在 Insert 后停住，永远到不了 Conclude：
- `currentStep` 在 message.status == "inserted" 时落回 default → 一直返回 .insert；
- `runFullJourney` 的 `while currentStep != .conclude` 在到达 Conclude 前一刻退出，
  而 UI 在 currentStep == .conclude 时隐藏 "Next:" 按钮且行不可点 →
  最后一步无法从 UI 触发。

## 修复（commit b193c0dc）
- `currentStep`：inserted/dropped → .conclude。
- `runFullJourney`：循环到 `session.concluded`（包含最后一步 Conclude）。

## 安装版 live 验证
用真实 UI 点 "Run full journey"，Journal 记录完整 9 步：
RoundtableSessionCreated → SeatAdded×2 → RoundOpened → MessageProposed →
MessageRelayed → MessageAcknowledged → MessageInserted → **RoundtableConcluded**
（summary_digest / artifact_digest = 0a204e8d…，digest-bound AlignmentSummary）。
截图：ux12–ux22（Roundtable 旅程各步）。

## 测试
Swift 254 绿（1 跳过）。安装版已重装。
