# Loom 安装版用户验收清单（~5 分钟）

前置：App 已启动、`Local service ready`；Provider 里 DeepSeek 已验证（有余额）。

## 1. 聊天（~1 分钟）
- [ ] 底部默认 Provider = DeepSeek（或 OpenCode），Model / 推理强度可分别切换。
- [ ] 输入一句话按回车 → 出现模型真实回复（不是报错）。
- [ ] 双击回复文本 + ⌘C 可复制。
- [ ] 新建会话（侧边栏 New）→ 打开全新空白聊天，与旧会话区分。
- [ ] 切 Provider → 出现 "Change conversation route" 确认 → 确认后继续对话不再重复弹。

## 2. 失败可见性（~30 秒）
- [ ] 切到无余额/无凭据的 Provider 发消息 → 对话内出现明确错误 + Incident ID。
- [ ] 错误横幅有 "Switch Provider" 按钮 → 点击打开 Runtime & Providers。

## 3. Team（~1 分钟）
- [ ] 有 Team 时欢迎页主操作显示 "Start Mission"（而非 "Use Agent Team"）。
- [ ] 无 Team 时 "Use Agent Team" → 构建器打开；填 Team name/Purpose 后直接点
      "Confirm Agent Team"（不用先按 Return）→ 队伍出现在 AGENT TEAMS。
- [ ] Confirm 按钮始终可见（固定 footer），被禁用时有原因提示。

## 4. Mission（~2 分钟）
- [ ] 欢迎页 "Start Mission" → New Mission sheet 打开。
- [ ] 填目标 → 勾选 "I confirm this Mission context" → Review preflight → 节点 ready。
- [ ] Start Mission → 进入 Mission Room，Activity 有实时记录，board 从 running 到 succeeded。
- [ ] 结束后 Evidence 有 accepted 记录；可复制输出。

## 5. 多 Provider/模型（可选，~1 分钟）
- [ ] 依次用 DeepSeek / OpenCode / MiniMax（若有）各发一条消息，均能回复且模型标识正确。

## 预期问题对照
| 现象 | 预期 |
|---|---|
| 首次聊天失败 | 明确错误 + Switch Provider 入口 |
| 默认 Provider 无余额 | 切到已配置账号即好 |
| Team Confirm 被禁 | 提示按 Return 或补必填项 |
| Mission 卡 running | 看 Activity/错误；重启 App 后 Resume |
