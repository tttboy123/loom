# Chat 发送/Session 用户流验收（2026-08-20 · LoomBuild50）

安装版 App（LoomBuild50 → /Users/lune/Applications/Loom.app，binary SHA-256
1ca32d47…688d）用户视角验收：Chat 发送按钮与回车发送均端到端打通，会话隔离、
多消息复制、三层选择、心跳/重连/partial 归一化保持生效。

## 本轮修复内容（工作区 7 个文件）

Swift（6 个，未提交 → 本轮提交）：
- `LocalIPCClient.swift`：新增 `supportsHeartbeatProbe`/`ping()`，生产客户端可用
  IPC ping 探测 resident socket；测试替身默认关闭，避免测试意外探测。
- `LocalProductStore.swift`：新增 `reconnectWhileUnavailable()`（后台轮询，daemon
  离线/partial/stale 后自动重连并在恢复 online 时重刷依赖表面；名义 online 时
  心跳探测静默 daemon 退出）；`isUserVisibleDegradation()` 归一化 partial——
  历史 Run/Evidence 列表 64 条溢出属于正常富数据（UI 已显示 "Showing the 64
  most recent …" 脚注），不再降级整个连接面；观察超时、导航截断、未知 partial
  保持降级态。
- `ContentView.swift`：冷启动完成基础刷新后启动 `reconnectWhileUnavailable()`。
- `LoomWorkspaceShell.swift`：Evidence 历史列表顶部 "Showing the 64 most recent
  records" 脚注（与 Run 脚注一致）。
- `MissionWorkbench.swift`：Runs 历史改用非懒加载 `VStack`（与 Library 修复同
  因：LazyVStack 会丢失未滚动到的历史脚注），并补 "Showing the 64 most recent
  Runs · older Runs remain in the Journal timeline." 脚注。
- `LocalProductStoreTests.swift`：新增 9 个测试（reconnect 恢复、静默 daemon 死
  亡探测、Run/Evidence 溢出归一化、导航截断/观察超时/未知 partial 保降级等）。

Go（1 个，本轮提交）：`cmd/loomd/run_test.go` — 执行配置测试不再依赖 ambient
`LOOM_ENABLE_WEB_TOOLS`：daemon 在 launchctl 环境变量下即使空 build config 也会
注入 remote-tool broker，破坏「空配置 → nil」断言；用 `t.Setenv` 固定关闭，测试
保持 hermetic（纯测试修复，不改生产行为）。

## 根因（两处，均已闭环）

1. **UI 点击落到错误 App**：`NSRunningApplication.activate` 后另一 App 仍保持
   frontmost，全局鼠标事件落在微信上 → 发送看似无效。解决：每次交互前用
   `/usr/bin/open -a /Users/lune/Applications/Loom.app` 确保 Loom 为 frontmost
   （`front` 确认 pid 26965）。
2. **Go 测试环境依赖**：`LOOM_ENABLE_WEB_TOOLS=1` 由 launchctl 注入，测试进程
   继承后使「空执行配置必须为 nil」断言失败。

## 安装版 live 验证（真实 socket + DeepSeek，均通过）

- 发送按钮路径：composer 用 AX 设值 → Send 点击 → composer 清空 → daemon 线程
  `thread-cd315d48`（当前选中 Session B）新增 user 消息并回填 loom 回复。
- 回车发送路径：`axsend3 "回车发送测试：7*8等于几？" enter` → 回车触发发送 →
  composer 清空 → thread 新增 user「回车发送测试：7*8等于几？」+ loom「7×8等于56。」
- 最终复核：发送「是否已显示回复确认：2+2等于几？」→ loom 回复「2+2等于4。」
  （axsend-button-output.txt / thread-tail-after-final-send.txt）。
- UI 渲染：axdumpall 逐条显示
  `You: … / Loom: …`（axdump-chat-render.txt，含全部 10 条消息）。
- Journey 数据完整落盘：thread-cd315d48-final.jsonl 含多个 segment/attempt 的
  执行绑定（deepseek.primary / deepseek-chat）、context capsule digest、
  disclosure 计数与 incident id（loom-chat-*）。

此前已验收并保持（不重做）：8 个 Session 可切换且内容隔离；多消息跨条复制双路径
（"Copy conversation" 按钮 + 反向拖选）；Provider/Model/推理强度三层菜单
（DeepSeek V4 Flash/V4 Pro/DeepSeek Chat/DeepSeek Reasoner）；错误码/错误提示 +
incident 后缀（失败场景由历史 round 的 live 证据覆盖）。

## 回归门禁（全部通过）

- `go test ./... -p 1` → PASS（含修复后的 cmd/loomd 全包）。
- `go vet ./...` → PASS；`gofmt -l` 无输出；`git diff --check` 干净。
- `swift test` → 277 通过 / 1 跳过（视觉导出测试按设计跳过）/ 0 失败。

## 约束遵守

- 未 push / 未 merge / 未发布 / 未改任何 Provider 凭据 / 未重启自治。
- 单 writer：本轮只动 Chat/Session 用户流 + 必需的 Go 测试 hermetic 修复。
- 安装版 = LoomBuild50 原始产物（bin 与 /Users/lune/LoomBuild50/Loom.app 一致）。
