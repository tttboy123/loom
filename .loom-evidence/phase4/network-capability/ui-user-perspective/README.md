# Loom UI 用户视角验证与 UX 迭代（2026-08-19）

## 验证方法
用安装版 App（`/Users/lune/Applications/Loom.app`）真实 UI 操作：
System Events 辅助功能树 + macOS Vision OCR 读屏，逐项走用户流程并留截图。

## 已验证通过（用户视角）
- 回车发送消息（composer 清空 → daemon `chat_message` 派发）。
- OpenCode/DeepSeek V4 Flash 真实回复渲染在对话里。
- Provider/Model/推理强度三层选择（OpenCode → DeepSeek V4 Flash (OpenCode) → Low）。
- 切换 Provider 的 "Change conversation route" 确认流程。
- 复制文本（双击 + ⌘C；⌘A + ⌘C）。
- 多会话（侧边栏多个命名会话；新建会话打开全新空白聊天）。
- 失败对用户可见（Codex 余额不足 → 中文错误渲染在对话内）。
- Team 构建器（Agent Team draft：Main/Subagent、DeepSeek、凭据/预算/超时/Remote tools）。

## 本轮 UX 迭代（commit 跟随）
1. **Chat 失败横幅新增 "Switch Provider" 恢复动作**（Error Recovery 指南）：
   provider 路由类失败（insufficient_balance / auth / model_unavailable /
   provider_unavailable / conversation_unavailable）显示 "Switch Provider" 按钮，
   一键打开 Runtime & Providers 治理页。用户视角已验证：余额不足横幅显示该按钮，
   点击打开 Runtime & Providers sheet。
2. **Team 构建器 Confirm 被禁用时给出可操作提示**：
   `LocalProductBuilderPreview.confirmationBlockedMessage`（Core，纯函数 + 3 分支单测）：
   未提交编辑 → "Press Return…"；兼容性缺口 → "Resolve N compatibility issue(s)…"；
   否则 → "Complete the required fields…"。

## 发现但未修复的既有问题
- 默认 Provider 是 Codex（官方账号无余额）→ 开箱第一次聊天即失败；用户需手动切
  OpenCode/DeepSeek。已有明确错误提示 + Switch Provider 恢复，但"默认选一个可用
  账号"的默认值策略仍待迭代。
- 聊天 Provider 菜单仅 Codex + OpenCode（DeepSeek/MiniMax/GLM 以 OpenCode 模型接入）。
- 切换 Provider 每次都弹 "Change conversation route"（功能正常，体验偏重）。

## 截图
- 01-main-window.png：主窗口（侧边栏 + 欢迎 + Provider/Model 选择器）
- 02-opencode-chat-reply.png：OpenCode 真实回复
- 03-route-changed.png：路由切换确认
- 04-team-builder.png：Team 构建器
- 05-new-conversation.png：新建会话
- 06-codex-fail-switch-provider.png：Codex 余额不足失败横幅 + "Switch Provider" 按钮
