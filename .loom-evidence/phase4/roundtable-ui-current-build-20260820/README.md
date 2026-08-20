# RoundTable 全旅程 UI live 复验（当前构建 · LoomBuild50 · 2026-08-20）

`CURRENT / VERIFIED`：在安装版 App（LoomBuild50 → `/Users/lune/Applications/Loom.app`，
managed daemon PID 27032，socket
`/Users/lune/Library/Application Support/Loom/run/loomd.sock`）里，用**真实 UI 交互**
（鼠标点击 + AppleScript 置前 + 键入/paste 填入字段）完整走通 RoundTable 受治理交接
旅程，并独立核对 daemon Journal 与内容寻址 AlignmentSummary Artifact 的 digest 一致。

## 前置澄清（本轮最重要的走查结论）
上轮 AX drive 曾让 "Create session" 看起来无效（snapshot 返回 `not_found`）。经复现排查，
根因是 **AX 措施假象而非用户缺陷**：
- `AXUIElementSetAttributeValue` 写 SwiftUI `TextField` 的 `$sessionID` binding 不会生效，
  所以表单里 session id 看似已填、实际 binding 仍为空，按钮保持 disabled；
- `/usr/bin/open -a` 置前不稳定（本会话 ChatGPT pid 95714 常抢前），接力事件会点到错误的 app。
- 用 **AppleScript `tell application "Loom" to activate`** 置前 + **真实鼠标点击 + 键入/paste**
  后，Create 正常：会话在 daemon 上创建、视图自动从 create 表单切换到旅程视图。

结论：RoundTable create 对真实用户可用；此前 "not_found" 纯属自动化 harness 的
binding/焦点测量噪声，不是产品缺陷。修复后的完整 journey 命令工具
`cmd/rt-live-journey` 与 daemon 直连均一直可用。

## 本次 live 复验（当前构建）

### 1) Create（真实 UI 点击）
- 置前：`osascript -e 'tell application "Loom" to activate'` → `/tmp/front` 返回
  `frontmost: Loom pid=26965`（`ps -p 26965` 确认为 `LoomLocalApp`）。
- 聚焦 Session ID 字段（sheet 内 `AXTextField frame=(361,315 784x24)`），`Cmd+A` + `Cmd+V`
  paste `rt-realuser` → AX 确认该字段 `value='rt-realuser' focused=true`。
- 点击 "Create session"（`AXButton` center `(429,403)`）→ AXSheet 从 create 表单切换到
  旅程视图（出现 "Next: Open round" 与 "Run full journey"）。
- daemon 核对：`roundtable_snapshot {"schema_version":1,"session_id":"rt-realuser"}`：
  ```json
  {"session":{"id":"rt-realuser","moderator_seat":"seat-moderator","title":"Diagnosis handoff",
   "created_at":"2026-08-20T09:46:02.677343Z","concluded":false},
   "seats":{... seat-moderator / seat-target / seat-writer ...},
   "rounds":[],"messages":{},"digest":"ac783d161e7102c68881e886f06f86746c22076f5d348052d6986169517fbde5"}
  ```
  （3 个 seat 由 UI 自动添加。）

### 2) Run full journey（真实 UI 点击）
- 点击 "Run full journey"（`AXButton` center `(1076,184)`）→ AXSheet 中 7 个步骤标记
  `AXImage :: Selected`，journey 驱动到 conclude。
- daemon 核对（截取）：
  ```json
  {"session":{...,"concluded":true},
   "rounds":[{"id":"round-1","sequence":1,"message_count":1,"messages":[
      {"id":"msg-1","status":"inserted","proposed_at":"...22.071Z",
       "relayed_at":"...22.117Z","acknowledged_at":"...22.162Z"}]}]}
  ```
  完整 9 步 Journal：SessionCreated → SeatAdded×2 → RoundOpened → MessageProposed →
  MessageRelayed → MessageAcknowledged → MessageInserted → **RoundtableConcluded**。

### 3) AlignmentSummary Artifact digest 一致
- 证据：`/Users/lune/Library/Application Support/Loom/state/evidence/artifacts/sha256/a9/
  a9b00ec39e255c5c00143c8cc8e09033493f41bd26018f808b2771766b70ac43`
  （`state/evidence` 为 managed daemon `--state .../state/loom.db` 的 evidence root）。
- `shasum -a 256 <file>` = `a9b00ec39e255c5c00143c8cc8e09033493f41bd26018f808b2771766b70ac43`
  **== 其自身内容寻址路径**（content-addressed、digest-bound）。
- 内容为 canonical AlignmentSummary：`schema_version / session_id=rt-realuser /
  moderator_seat / title / concluded_at / seats[3] / rounds[1].messages[1].status=inserted`。

## 测试 / 门禁
本 walk 为纯 live 复验（无代码改动、无新增提交）。回归基线（上一轮全绿）：
`swift test` 277 通过 / 1 跳过 / 0 失败；`go test ./... -p 1` 全绿；
`go vet` / `gofmt -l` / `git diff --check` 干净。本轮新增证据仅文档/截图，无源码变更。

## 附件
- `screen-concluded.png` —— 全旅程 concluded 后的安装版界面截图。
- `snapshot-rt-realuser.json` —— daemon `roundtable_snapshot` 原始输出。
- `artifact-alignment-summary.json` —— 内容寻址 AlignmentSummary artifact 原文。
