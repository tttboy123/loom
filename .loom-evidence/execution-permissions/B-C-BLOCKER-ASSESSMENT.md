# B / C 落地阻塞点评估（B-P1 验收后）

Date: 2026-08-05

B-P1（Execution Permission Pipeline）已落地并原子提交（`91d14f42`）。
本文件评估 B（有界本地执行适配器）与 C（生产化落地）的剩余阻塞点，
供下一阶段立项使用。结论先行：**权限管道已消除 B/C 的"授权面缺失"
阻塞；剩余阻塞主要是执行通道本体、显式激活授权与常驻化治理，均不再需要
扩大权限设计本身。**

## B — 有界本地执行适配器（模型真实动手改代码）

### 现状（B-P1 已提供）

- 工具调用级授权管道完整：5 种 mode、`deny>ask>allow` 跨作用域规则、
  只读白名单、危险命令表、per-Job profile + generation 围栏、
  fail-closed 校验器、管理员锁（Journal 事实唯一权威）。
- `validate_call` 已在产品服务层可用：任意 Job 提议一次工具调用即得
  allow/ask/deny + typed denial；ask 产生决策事实并复用既有 rules 批准。

### 剩余阻塞点

1. **执行通道本体不存在**：模型桥接仍硬编码 `Use no tools`
   （`internal/runtime/piadapter/rpc_bridge_adapter.go:46`）。B 需要新增
   "执行适配器"：把模型提议的工具调用送入 `permissions.Evaluate`，allow 才
   真正执行（文件读写/命令），执行事实与副作用证据进 Journal。这是新的
   垂直 WorkItem（B-W1），不是权限管道的扩项。
2. **不变量 3 的有界修订**：模型输出从"永远不执行"变为"仅在
   Evaluate=allow + 授权事实链下执行"。需要一份独立评审的 bounded
   Amendment（ADR 已预留方向，见 PERMISSION-ADR Consequences）。
3. **执行器与既有领域层的接线**：Test Executor 仍是确定性进程；B 的模型
   工具执行必须与 WorkItem/Attempt/Evidence 生命周期绑定（generation 围栏
   已就绪），不能出现"模型绕过领域层直接改盘"。
4. **Provider/模型客户端不在本目标**：Loom 不持有模型客户端、key 或
   agent loop；B 的模型调用面需走既有 Runtime 适配器，且不在本阶段引入
   网络 Provider。

### 建议

单垂直 WorkItem `B-W1 Execution Adapter`：工具调用提案 → Evaluate →
allow 执行（沙箱内）→ 结果/副作用作为 Journal 事实与 Evidence 落库 →
与 WorkItem 生命周期联动；RED 覆盖"deny/ask 绝不执行"、旧 generation
拒绝、撤销后 in-flight 拦截、双 Job 隔离。

## C — 生产化落地（常驻 daemon / launchd / 真实配置 / 显式激活）

### 现状（B-P1 已提供）

- daemon 已是常驻进程（含 resident demo 实例）；journey harness 证明
  真实 IPC + 跨客户端旅程可冻结。
- 权限模式激活、管理员锁、root policy 写入的 human 门禁已在权限层实现。

### 剩余阻塞点

1. **显式激活（最大治理阻塞）**：C 的"激活"是真实机器状态变更
   （launchd、`~/Library/Application Support/Loom` 配置、常驻运行），
   仓库规则与用户既有立场要求每次激活前展示 permission diff 并显式批准。
   B-P1 已把"激活是 Journal 事实 + `authorized_by` 非空"固化，但产品级
   激活 UI/流程（用户看到 diff → 确认 → 写入）尚未实现。
2. **launchd / Application Support 写权限**：这些路径在工作区之外，
   需要用户显式授权；实现侧需 install/rollback 原子性与 0700/0600 权限
   审计（沿 P2A-W1 install 模式）。
3. **管理员锁的持久化与升级路径**：root policy 由 human-authorized writer
   独占管理；C 需要把该 writer 落地为真实的"激活通道"（如专用 CLI 命令 +
   用户会话确认），并证明在线自演化无法触碰。
4. **恢复/回滚语义**：常驻化后的重启恢复、配置回滚、故障降级
   （fail-closed 到只读）需要随 C 的 RED 矩阵覆盖。

### 建议

单垂直 WorkItem `C-W1 Production Landing`：显式激活流程（permission diff
预览 + 确认 + Journal 激活事实）→ launchd/Application Support 安装与回滚
（复用 install 模式）→ 常驻恢复/降级 RED → 跨客户端旅程。

## 优先级建议

先做 B-W1（执行适配器），因为它直接解锁"Loom 像 Codex 一样真实改代码"
这一产品价值；C-W1 的激活治理可以并行设计但其实际写入需等用户显式授权
（与 B-P1 相同的 Gate 节奏）。
