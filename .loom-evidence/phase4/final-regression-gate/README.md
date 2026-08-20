# 最终回归门（2026-08-20 · HEAD ef76086d）

## 门禁
- `go test ./... -p 1` 全绿（20+ 提交后无回归）。
- `swift build --build-tests` 0 error。
- `git diff --check` 干净；工作树干净。
- daemon healthy（serving_request / projection current）；ping OK。

## 状态
- 环境干净：首页 0 active / 0 need you（App 视角）；daemon 快照
  {failed:23, blocked:3, succeeded:3}（3 个 blocked 为已归档测试团队，App
  视为历史）。
- 核心旅程在干净环境全部复验通过（见 acceptance-matrix-20260820 干净环境章节）。

## 覆盖（本会话 21 提交）
功能修复：Mission 收敛、幻影/awaiting_recovery 只读收敛、归档团队 Mission
不计 active。
UI/UX：实际模型/推理强度、blocked 原因（卡片/Room）、多消息复制、同名会话
消歧、错误文案人话化、Board Hide completed、RoundTable 会话恢复、Attention
直达 Mission、Run as Mission objective 截断、首页计数一致。
证据：13 项验收矩阵 + 干净环境复跑 + TUI 双端 + 治理处置 + 本门禁。
