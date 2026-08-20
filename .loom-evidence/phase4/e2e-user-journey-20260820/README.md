# 端到端用户旅程复验（2026-08-20 · LoomBuild39）

在安装版 App（LoomBuild39 → /Users/lune/Applications/Loom.app）上用真实
daemon + DeepSeek 走完整条用户旅程，确认累计 9 个 UI/UX 修复共同工作。

## 旅程
1. 冷启动 → "Local service ready / Chat is ready with DeepSeek · deepseek.primary /
   Agent Team ready"（干净重启后正常；期间一次 "Showing last loaded..." 为
   已知环境 artifact——kill+清 socket+重开即恢复，非代码回归）。
2. 发送 "E2E journey: what is 7*8? Answer briefly." →
   Loom 回复 "7 * 8 = 56"；路由标签 "Loom Native · deepseek.primary ·
   deepseek-chat"，上下文披露 "Context shared 1 · 0 omitted" 正常
   （chat-deepseek-reply.png）。
3. Selectable text 模式：整段会话一块可选文本
   "You: E2E journey... / Loom: 7 * 8 = 56"；"Copy conversation" →
   剪贴板一次得到两行（selectable-text-copy.png）。
4. 多会话隔离：新会话 B 发 "name the color of the sky in one word" →
   "blue"；切回会话 A（E2E）内容仍为 "7 * 8 = 56"；再切回 B 仍为 "blue"，
   无串扰（session-isolation.png）。

## 结论
- 三层选择（Provider/Model/推理强度）、路由标签、上下文披露、Selectable
  复制、多会话隔离在当前构建全部工作。
- Board / Attention / Mission 工作台 / TUI 的文案与计数修复（前几轮）保持生效。
