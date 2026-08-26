# 混合 Provider Team 的 App 可视化复验（2026-08-20 · LoomBuild47）

用户视角确认：已成功的混合 Mission 在 App Mission 工作台里，两个 Agent 各自
的 Provider/Harness/Model 清晰可见。

## 验证
打开 "Mixed Verify 2" 的 succeeded Mission（main=deepseek/loom-native，
subagent=opencode/opencode）：
- "2 node(s), 2 complete, 0 in review"
- 节点 1：Coordinate bounded work and review with **DeepSeek** ·
  deepseek.primary · deepseek-chat
- 节点 2：Coordinate bounded work and review with **OpenCode** ·
  OpenCode · opencode/deepseek-v4-flash-free
- "Main · Terminal · Attempt 1"

截图：mixed-mission-workbench.png
