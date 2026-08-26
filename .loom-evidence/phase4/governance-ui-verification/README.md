# 治理/账户 UI 用户视角验证（2026-08-20）

## 验证路径（安装版，真实 daemon）
切 Provider 到 Codex → 发送消息 → Codex 余额不足失败 →
Chat 失败横幅（标题/详情/Incident ID/Switch Provider/View diagnostics）→
点 Switch Provider → Runtime & Providers 治理页打开。

## 已验证（截图）
1. **Chat 失败横幅**：Codex account usage limit reached + 可操作详情 +
   Incident loom-chat-2abd… + Switch Provider + View diagnostics。
2. **Runtime & Providers**：CREDENTIAL VAULT（Loom Credential Vault ·
   Local Key File · Encrypted · Unlocked · Rotate key · View diagnostics）、
   AGENT RUNTIMES（Codex · Native login · Available · Reconnect）、
   MODEL PROVIDERS（OpenAI/Anthropic/Google Gemini/DeepSeek Verified/Kimi/
   MiniMax，带搜索 + 各 Provider 添加账户 +）。
3. **Vault key rotation**：点 Rotate key → "Vault key rotated"；
   旋转后 loom-native(deepseek-chat)/opencode/pi runtimes 仍 online，
   encrypted_credentials 93 条完整，DeepSeek 账户仍 Verified。
4. 回车发送消息（composer 清空 → 消息进入 transcript）正常。
5. Provider 切换（DeepSeek→Codex→）与 "Change conversation route" 确认正常。

## 结论
治理/账户类 UI（账户级 accounting、凭据 vault、runtime/provider 目录）功能
正常且信息层级清晰。Swift 254 绿；安装版已重装（含 RoundTable 全旅程修复
b193c0dc 与模型选择器 UX f409e1fb）。
