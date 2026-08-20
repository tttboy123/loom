# 归档团队的 Mission 不再计入 active（2026-08-20）

## 问题
已归档（不可执行）团队的 Mission 仍计入首页 "active" 和 Board 活跃车道。
用户归档团队后，其 Mission 应视为历史。

## 修复（Swift）
- `countActiveMissions(teams:missions:)`：只统计 lane != Complete 且团队
  executable 的 Mission（首页 active 计数）。
- Board `missionLane`：活跃车道（非 Complete）排除归档团队 Mission，Complete
  车道保留全部作为历史。
- `missionOnArchivedTeam`：由 mission id "mission/<teamID>" 反查团队可执行性。

## 安装版 live 验证（LoomBuild31）
- 首页 active：**6 → 5**（UI Verify Team 6 为归档团队，其 awaiting_recovery
  不再计入）。
- 5 个 active 均属可执行团队（3 blocked + 2 awaiting_recovery）。
- App 健康（Local service ready / Agent Team ready）。

## 测试
- `testCountActiveMissionsExcludesArchivedTeamMissions`：归档团队 Mission 不计、
  Complete 不计、可执行团队活跃 Mission 计 1。
- `swift build --build-tests` 0 error；`go build ./...` OK；
  `git diff --check` 干净。

## 备注
- 重装后 App 偶发 "Local service unavailable" 为陈旧 socket/连接 artifact
  （App 见旧 socket 不拉 daemon）；清 socket + 干净重启即恢复，非代码回归
  （LoomBuild30 与 31 在干净环境均健康）。
