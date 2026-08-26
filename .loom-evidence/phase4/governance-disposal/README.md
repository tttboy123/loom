# 治理处置验收：归档团队 → Mission 移入历史（2026-08-20）

## 验证
用户视角的"失败恢复/治理处置"路径：对 blocked/不可恢复的 Mission，
用户处置方式是归档其 Team（Mission 移入历史，不再占 active）。

- `team_archive`（daemon IPC）对 blocked 测试团队生效：
  - Mission Verify 3 / 4 / Team → status archived（stream_head 2）。
- 归档后（daemon 快照）：
  - 非 Complete Mission：6 → **可执行团队仅 2**（UI Trace / UI Wrap2 的
    awaiting_recovery，属真实 UI 团队，保留）；
  - 4 个移入历史（3 个归档 blocked + UI Verify Team 6 的归档 awaiting_recovery）。
- 这验证了 `88318570`（归档团队 Mission 不计 active）在真实治理流程中生效。

## 结论
用户能通过归档 Team 干净处置无法恢复的 Mission（Board/首页不再显示为
active），真实待处理项（awaiting_recovery）仍清晰保留。

## 边界
- App 首页计数按刷新节奏更新（daemon 侧已为 2，UI 下次刷新即同步）。
- 剩余 2 个 active 为真实 UI 团队（UI Trace / UI Wrap2）的 awaiting_recovery，
  未做处置（属用户团队）。
