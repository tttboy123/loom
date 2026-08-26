# 会话回复展示实际模型与推理强度（2026-08-20）

## 用户问题
"模型不准确，Provider、Model、思考程度对不齐"。此前每条回复的路由标签
只显示 profile 的默认 model（如 deepseek-chat），会话中切换到
v4-pro/high 后标签仍是默认值——用户无法核对"这条回复到底用的哪个模型和
推理强度"。

## 修复（Go + Swift，向前兼容）
- `internal/api/local_product_chat.go`：`LocalProductConversationAttempt`
  新增 `model_id` / `reasoning_effort`（omitempty），创建 attempt 时从请求
  填充实际派发值。
- `LocalProductModels.swift`：attempt 模型新增 `modelID` / `reasoningEffort`
  （可选，`decodeIfPresent`；旧 daemon 响应缺字段 → nil，不破坏解码）。
- `LoomWorkspaceShell.swift`：`conversationRouteLabel` 优先取该回复对应
  attempt 的实际 model + effort（按 segment 内用户轮次匹配 attempt），
  无实际值时回退 profile 默认 model。标签示例：
  `Loom Native · deepseek.primary · deepseek-v4-flash · high`。

## 安装版 live 验证
- probe 发 `deepseek-v4-flash` + `high` → 回复 LABEL-OK；
  attempt 返回 `model_id=deepseek-v4-flash`、`reasoning_effort=high`。
- 同线程切换 v4-pro/high 继续回复成功（segment 延续）。
- 无效组合（v4-flash + ultra）被 fail-closed（conversation_unavailable）。

## 测试
- Swift 新增 2 个测试：attempt 解码实际 model/effort、缺失时 nil
  （`swift build --build-tests` 0 error；本机 swift test 发现 15-16 条）。
- Go `go test ./... -p 1` 全绿；`gofmt`/`git diff --check` 干净。
