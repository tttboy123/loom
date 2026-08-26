# 错误码/错误提示统一（2026-08-20）

## 问题
用户反馈"要把所有的错误码和错误提示都处理好"。审查发现一个泄漏点：
Side Task 面板直接把 `store.sideTaskOperationStatus` 显示给用户，出错时是
`closedClientReason` 的原始错误码（如 `invalid_request`、`state_unavailable`）。

## 修复（Swift）
- 渲染层新增 `sideTaskStatusLabelText(_:)`：把 daemon 拒绝码映射为可读、
  可操作文案（invalid_request/state_unavailable/conflict/stale_view/
  timeout/capability_gap/not_found 等）；友好进行态（Idle/Proposing/
  Creating/Confirmation required）原样透传；未知码保留稳定原因便于诊断。
- `Text(store.sideTaskOperationStatus)` → `Text(sideTaskStatusLabelText(...))`。

## 验证
- 独立 Swift 校验：invalid_request → "Loom rejected the side task request…"，
  capability_gap → "This Agent Team cannot run that kind of side task yet."，
  Proposing 透传，未知码保留。
- 单元测试 `testSideTaskStatusLabelMapsRawCodesToReadableCopy`。
- 安装版（LoomBuild25）正常渲染；`swift build --build-tests` 0 error；
  `go build ./...` OK；`git diff --check` 干净。
- 复查其它错误面：聊天失败横幅、Mission start 失败、连接条、setup 均已
  有人话文案；`evolutionAssetStatus` 的 Journey 行是诊断用途（monospaced），
  保留原始状态码。
