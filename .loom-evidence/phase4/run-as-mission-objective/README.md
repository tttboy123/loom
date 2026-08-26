# "Run as Mission" objective 边界改进（2026-08-20）

## 问题
聊天提案转 Mission 时，若提案内容超过 4096 上限会回退到最新用户消息；若
两者都超长/为空则 objective 为空（用户点 "Run as Mission" 得到空字段）。

## 修复（Swift）
- 提取 `chatMissionObjectiveText(around:thread:)`（可测）：
  - 提案非空 → 截断到 4096 作为起点（不再回退到空/用户消息）；
  - 提案空 → 回退最新用户消息（同样截断到 4096）；
  - 全空 → 返回 ""（New Mission sheet 会显示 "Enter a Mission objective first"）。

## 验证
- 单元测试 `testChatMissionObjectivePrefersProposalAndTruncatesLongText`：
  优先提案、空提案回退用户消息、超长截断到 4096。
- `swift build --build-tests` 0 error；`go build ./...` OK；
  `git diff --check` 干净；安装版（LoomBuild30）构建并安装成功，App 健康
  （Local service ready / Agent Team ready）。
