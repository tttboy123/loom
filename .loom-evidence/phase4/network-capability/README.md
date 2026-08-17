# Phase 4 · Loom 网络访问能力实测

Status: `TEST COMPLETE` — 安装版 `/Users/lune/Applications/Loom.app` v0.5.3
Date: 2026-08-17
工具：`cmd/loom-net-probe`（连安装版 daemon 的真实 socket）

## 结论

| 层 | 结果 | 说明 |
|---|---|---|
| 本地 IPC | ✅ | daemon socket 可达，`setup_snapshot` 返回 3 个会话资料档 |
| 对话层网络 egress | ✅ | `chat_message` 走 DeepSeek（brokered, deepseek.primary）真实联网，模型返回 `Loom-NET-OK` |
| Chat 工具/联网查询 | ❌（设计如此） | 模型被问到是否有 web 搜索/浏览工具时明确回答 `NO`；Chat 是纯文本 responder，无工具面 |

## 关键输出（`installed-network-probe.log`）

```
[1] local IPC reachable: ok (3 conversation profiles)
    profile=conversation-openai-codex-default-v1 provider=openai model=codex-default auth=native_auth
    profile=conversation-opencode-default-v1 provider=opencode model=opencode/deepseek-v4-flash-free auth=native_auth
    profile=conversation-deepseek-deepseek-chat-r2 provider=deepseek account=deepseek.primary model=deepseek-chat auth=brokered
[2] chat (network egress): got reply len=11 egress_ok=true
[3] chat tool surface: model answered "NO" -> tools_present=false (expected false)
```

## 含义
- **Loom 能联网**：daemon → Provider API 的网络 egress 正常（对话可用）。
- **Loom Chat 不能“联网查询”**：这是 4.2-harness-tools 候选切片要补的能力
  （治理化 web_search / 远端 MCP），不是故障；目前联网工具只存在于
  Mission/Agent 的 Web/MCP enrollment 路径（V31/V34）。
