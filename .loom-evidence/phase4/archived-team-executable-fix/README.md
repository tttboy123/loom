# 归档 Team 变为不可执行 + 侧边栏清理（2026-08-20 · commit 3659f38e）

## 发现
- `projection.TeamTimelineAnchor` 对任意 team instance 无条件返回
  Executable=true，导致 team_archive 归档后该团队仍可执行、仍在侧边栏
  AGENT TEAMS rail 里，用户无法真正"归档掉"一个团队。
- 侧边栏 rail 列出全部团队实例（含已归档），污染界面。

## 修复（RED-first）
- `TeamTimelineAnchor`：定义 Status == archived → Executable=false
  （团队仍可见/可读历史，但不能再启动 Mission）。
- macOS 侧边栏 AGENT TEAMS rail 仅显示 executable 团队。

## live 验证（安装版）
- 归档 15 个测试团队后，daemon snapshot 恰好 1 个可运行团队
  （UI Mission Team，executable=True），其余 15 个 executable=False。
- 恢复用户团队 team-ui-mission-1（team_restore → active）。
- 侧边栏 rail 只显示 UI Mission Team；欢迎页 "Agent Team ready"。

## 测试
Go projection/app/api/work 全绿；Swift 254 绿。
