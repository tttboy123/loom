# Attention（Needs You）列表可点击直达 Mission（2026-08-20）

## 问题
"Needs you" 列表只读展示 kind/severity/status/actionRequired 文本，用户看到
"需要处理"却无法直接跳到对应 Mission，必须手动去 Board 找。

## 修复（Swift）
- `attentionWorkspace`：每个 Attention 卡片改为可点击 Button，点击调用
  `store.openMissionAndActivate("mission/<teamInstanceID>")` 直接打开该
  Mission 房间；卡片底部加 "Open Mission →" 提示，accessibility/help 同步。

## 验证
- `swift build --build-tests` 0 error；`go build ./...` OK；
  `git diff --check` 干净；安装版（LoomBuild28）构建并安装成功。
- 打开逻辑复用已有的 `openMissionAndActivate`（store 层已有覆盖）；
  UI 接线为代码审查确认（盲 UI 自动化无法可靠点击导航）。
