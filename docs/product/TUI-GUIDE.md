# Loom TUI 使用说明

Loom 终端 UI（TUI）是产品的第一操作面：真实 PTY 中通过键盘完成导航与
操作，所有状态来自真实 daemon socket 上的投影读，所有变更走生产 IPC
action 写入 Event Journal。

## 启动与退出

```sh
# 显式指定 daemon socket（推荐）
loom app --socket /path/to/loomd.sock

# 不传参数时使用默认 socket（~/Library/Application Support/Loom/run/loomd.sock）
loom
```

- 退出：`q` 或 `ctrl+c`。
- 帮助开关：`?` 显示/隐藏按键帮助。
- 刷新：`r` 重新加载当前屏（daemon 重启后用它读回重建状态）。
- 最小尺寸：终端至少 60 列 × 16 行，否则显示尺寸提示。

## 屏幕总览（`tab`/`shift+tab` 循环）

共 12 屏，`tab`（或右方向键）前进、`shift+tab`（或左方向键）后退：

```text
Board → New Mission → Mission → Team Builder → Runs/History → Compare
→ Attention → Team Timeline → Evolution Assets → Development Queue
→ Worker Pools → Integration → (Board)
```

## 全局按键

| 键 | 作用 |
|---|---|
| `tab` / `→` | 下一屏 |
| `shift+tab` / `←` | 上一屏 |
| `q` / `ctrl+c` | 退出 |
| `r` | 刷新当前屏 |
| `?` | 帮助开关 |
| `j`/`k` 或 `↓`/`↑` | 选择（Board 为任务选择） |
| `enter` | 确认/进入 |
| `esc` | 返回/取消待确认动作 |
| `g` 然后 `b` | 直接回 Board |
| `g` 然后 `t` | 直接进 Mission |

## 各屏说明与按键

### Board（任务看板）

展示 Mission 列表。`/` 进入任务搜索输入；`enter` 新建 Mission（选中
"new"）或打开选中 Mission；`j/k` 移动选择。

### New Mission（新建 Mission）

描述目标 → 选择 Team（`t` 循环）→ 选择 Work Package（`w` 循环）→
`p` 预检（preflight）→ `s` 启动。`enter` 编辑目标文本；`esc` 放弃。

### Mission（任务详情）

展示选中 Mission 的侧任务、决策与注意力：

- `n` 新建侧任务请求；`y` 循环 purpose；`o` 循环 mode；
- `f` 创建侧任务（有 proposal 时）；`d` 提交决策；`[`/`]` 切换可用决策；
- `a` 打开注意力决策；`c` 取消（有取消绑定）；`x` 撤销 proposal；
- `esc` 回 Board。

### Team Builder（团队构建器）

一次一问题的构建流程（Candidate 边界，不隐式创建 Run）：

- `n` 空白构建器；`enter` 提交当前问题选项或输入答案；
- `e` 编辑名称；`p` 编辑 purpose；`m` 循环 main role；`s` 循环 subagent；
- `c` 确认（CanConfirm 时）；`g` 存储/替换 Credential；
- 保存团队区：`u` 归档恢复、`a` 取消归档、`v` 验证凭证、`x` 撤销凭证。

### Runs/History 与 Compare

Runs 列出历史 Run；`enter` 切换选中 Run 进 Compare（选择两个 Run 对比
Runtime/Evidence/恢复历史）。

### Attention 与 Team Timeline

Attention 是需要你处理的可执行事实投影；Team Timeline 展示团队的
Context/Changes/Evidence 活动页。`r` 翻页刷新（有 next cursor 时）；
`esc` 回 Board。

### Evolution Assets（演化资产）

版本化 Skill/Team/WorkPackage/Recovery 模板的精确修订血统：

| 键 | 作用 |
|---|---|
| `/` | 搜索资产 |
| `n` | 新建 local Skill |
| `i` | 导入 Skill |
| `1`/`2`/`3`/`4` | 新建 agent / team / work_package / recovery 模板 |
| `p` | 提升 Run（promote） |
| `e` | 评估（evaluate） |
| `a` | 激活（activate，需 `y` 确认） |
| `z` | 实例化（instantiate） |
| `b` | 绑定 Coding（bind） |
| `v` | 对比所选版本（diff） |
| `x` | 拒绝（reject） |
| `h` | 保留（retain） |
| `d` | 归档/恢复（archive_toggle） |
| `u` | 回滚（rollback） |
| `]` | 下一页 |
| `y` | 提交待确认动作（activate/reject/retain/promote/...） |
| `esc` | 取消待确认动作 |

### Development Queue（开发队列）

Journal 权威的队列投影：

- `n` 进入路径输入模式 → 输入 owned source path → `enter` 创建 Job
  （`queue_command create_job`，admission/eligibility 校验后入队）；
- `j/k` 选择；`r` 刷新；`q` 退出。

### Worker Pools（工作池）

短暂 worker 池 + lease/generation fencing：

- `c` 显式 claim 第一个非终态 Job（`workers_command claim`，生成
  generation 1 attempt）；
- `t` 记录确定性测试成功（`workers_command result succeeded` +
  Candidate ready for review）；
- `v` 记录只读 Reviewer 评审 PASS（`ReviewVerdictRecorded`）；
- `r` 刷新（观察并行 claim、失败分类、repair lane 状态）。

### Integration（集成）

单写 Integrator + Timeline/Attention：

- `i` 集成第一个已评审 Candidate（`integration_command integrate`，
  同目标分支竞争集成会被 CAS 拒绝）；
- `a` 由 later Run 采纳已发布 Release（`adopt`）；
- `b` 回滚到先前版本（`rollback`）；
- `r` 刷新。

## 输入模式（entry mode）

部分按键进入文本输入模式（屏幕底部提示），此时普通导航键失效：

- `enter` 确认输入；`esc`/`ctrl+c` 取消；`backspace` 删除；
- 输入后自动提交对应动作（如 Queue 的 Job 路径、Assets 的资产名、
  Board 的任务搜索、Mission 的目标与侧任务请求）。

## 只读与权威边界

- Team Builder 产出的是 Candidate；只有显式确认后才可能创建实例。
- Reviewer 评审（`v`）是只读事实记录，Reviewer 无产品写入权。
- 一切变更都进入 Event Journal；TUI 渲染只读投影，重启/重连后 `r`
  即可恢复一致视图。
