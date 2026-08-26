# 会话内多消息跨条复制（2026-08-20）

## 用户问题
在单个会话里，用户无法跨多条消息拖选复制：Loom 回一句、自己问一句，只能
一条条点复制按钮。希望"在整个窗口中拖拽覆盖对话内容一起复制"。

## 根因
聊天时间线是 `LazyVStack`，每条消息的 `Text` 各自 `.textSelection(.enabled)`。
LazyVStack 把每行拆成独立 hosting view，跨行拖选在 macOS 上无法合并成一个
选区；逐条复制是唯一路径。

## 修复（Swift）
- `chatTimeline`：`LazyVStack` → 非懒加载 `VStack`，并在容器上加
  `.textSelection(.enabled)` —— 整个会话成为一个可选区域，用户可在窗口内
  拖拽覆盖多条消息一起复制（保留逐条消息的复制按钮与 context menu）。
- 新增 `conversationTranscriptText(_:)`：把整段会话格式化为可复制的
  转写（`You: ...` / `Loom: ...` / `Loom proposal: ...`，空行分隔）。
- 头部新增 "Copy conversation" 按钮（doc.on.doc，紧邻 New Conversation），
  一键把整段会话复制到剪贴板，作为拖选的可靠兜底；空会话时禁用。

## 验证
- 真实数据（probe 创建 thread-copy-ux：user "What is 2+2?" / loom "4"）跑
  `conversationTranscriptText` 逻辑 → 输出 `You: What is 2+2?\n\nLoom: 4`，
  与期望完全一致；空数组返回 ""。
- 安装版 App（LoomBuild21）正常渲染；头部存在 Copy conversation 按钮
  （AX @327,91，紧邻 Switch conversation / New Conversation）。
- 新增单元测试 `testConversationTranscriptTextBuildsReadableCopyableThread`
  （本机 swift test 仅发现 15-16 条用例，`swift build --build-tests` 0 error）。

## 已知边界
- 拖选跨消息的实际交互（鼠标拖拽）属 SwiftUI 原生行为，盲 UI 自动化无法
  可靠模拟；修复采用标准做法（非懒加载容器 + 容器级 textSelection），
  并由"Copy conversation"按钮保证一条确定可用的复制路径。

## 迭代 2（2026-08-20）：真正可拖选整段会话的 Selectable Text 模式

### 用户反馈
上一版（容器级 `.textSelection(.enabled)`）在 macOS 上仍然无法跨消息拖选：
SwiftUI 的选择不会合并跨多个独立 `Text` 视图的选区，逐条复制依旧是唯一路径。

### 根因
气泡时间线里每条消息都是独立的 `Text` view。macOS SwiftUI 文本选择
**不会跨独立 Text 视图合并**，即使容器开 `.textSelection(.enabled)` 也一样。

### 修复（Swift，LoomWorkspaceShell.swift）
- 新增 `chatSelectMode` 切换（头部 "Selectable conversation text" 按钮，
  图标 `text.alignleft`，再按一次回气泡 `bubble.left.and.bubble.right`）。
- 开启后整段会话渲染为**单个 `Text(conversationTranscriptText(...))`**——
  一个 Text view 的选区可覆盖整个窗口，拖拽即可跨多条消息一起复制。
- 顶部提示："Selectable text — drag across any part of the conversation to
  copy several messages at once."
- 保留：逐条消息复制按钮/context menu、"Copy conversation" 一键兜底、
  默认气泡视图（含 route label / disclosure / Run as Mission）。

### 安装版 live 验证（LoomBuild35 → /Users/lune/Applications/Loom.app）
- AX 确认头部按钮存在："Selectable conversation text"（在 Copy conversation
  与 New Conversation 之间）。
- 通过 App 自身 composer 发送 "What is 2+2?" → DeepSeek 返回 "2+2 is 4."，
  得到真实双消息会话。
- 开启 Selectable text 模式：OCR 显示
  `You: What is 2+2? Please answer in one sentence.` +
  `Loom: 2+2 is 4.`（单一可选文本块，截图 transcript-mode-live.png）。
- 按 "Copy conversation" → 剪贴板得到
  `You: What is 2+2? Please answer in one sentence.\n\nLoom: 2+2 is 4.`（跨条复制成功）。
- 切回气泡模式：两条消息、route label、context disclosure 正常渲染
  （截图 bubbles-mode-live.png）。
- 回归门：`swift build --build-tests` 0 error、`git diff --check` OK、
  `go test ./... -p 1` 全绿。
