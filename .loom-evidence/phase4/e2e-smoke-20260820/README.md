# E2E Smoke：安装版开箱即用（2026-08-20）

## 验证（安装版，真实 daemon + DeepSeek）
1. 冷启动 App → "Local service ready / Chat is ready with DeepSeek /
   Agent Team ready"（环境已清理：仅 UI Mission Team 可运行）。
2. 发送 "Say hello in one short sentence" → DeepSeek 真实回复渲染在对话中
   （route: Loom Native · deepseek.primary · deepseek-chat）。
3. 前置轮次已验证：Provider/Model/推理强度三层选择、多会话、Team 构建器、
   Mission preflight/start、RoundTable 全旅程、治理 UI、失败横幅 + Switch
   Provider、网络工具空结果有界处理、归档团队可执行性修复。

## 结论
用户打开 App 即可完成真实对话与核心治理操作。Swift 254 绿（1 跳过）；
Go 相关包全绿；git 工作树干净。
