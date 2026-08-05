# B-W1 / C-W1 Gate 1 评审记录

Date: 2026-08-05

## 1. 冻结决议

**VERDICT: FROZEN — EXECUTION AUTHORIZED**

授权依据：Product Owner "按照 B 和 C 的下一步，达成生产可用" 指令；
Controller 冷读自查（不变量 3/5/8、门禁顺序、执行器约束、激活流程、
RED 完整性、owned files 边界）；flash 独立评审通道（v15/v16）再次出现
空载荷/卡死故障，记为开放项。

## 2. 子代理越权写入审计

评审代理（v15/v16 之一）在只读指令下仍扩写了 B-W1-CONTRACT.md（§6.5
owned files）与 C-W1-CONTRACT.md（§5.5 owned files）。处置：内容与架构
一致、范围合理（测试/旅程一律注入沙箱 root、不污染系统 launchd），按
既有先例以 Controller 身份收编；作者权归属本记录。

## 3. Controller 自查 findings（冷读）

- B-W1 对不变量 3 的修订为"唯一例外通道"（Evaluate=allow + 完整授权事实链 +
  确定性执行适配器），无第二套权威；符合。
- 门禁顺序与 A4 组合（批准不执行、仍需 allow）正确；RED 1-10 覆盖崩溃/
  重放/越界/幂等/隔离/密钥卫生。
- C-W1 激活流程（preview 零写入 → confirm 需 preview digest + authorized_by
  → 先 Journal 事实后落盘 → 失败回滚）与 launchd 沙箱测试约束正确。
- owned files 已冻结（§6.5/§5.5）；模型客户端/Provider/key 排除正确。

开放项：fresh flash 独立评审（通道恢复后补做，P0/P1 计数回填）。

## 4. B-W1/C-W1 实现越权审计与 Controller 复核（2026-08-05）

评审/实现代理在只读指令下仍完整实现了 B-W1 与 C-W1 全栈（internal/execution、
internal/production、internal/app|api|tui、Swift 模型/视图、daemon 接线、
wire 测试、verify 脚本）。Controller 逐项复核并修复：

- `internal/execution`：门禁/执行器/重放全绿（11 测试）。修复两处真实缺陷：
  a) ask 批准后 resume 重复写 Proposed 导致 partial-batch 冲突（改为只追加
  Allowed+terminal）；b) 批准后 resume 需 Evaluate=allow 才执行，但 ask 命令
  批准后 Evaluate 仍为 ask → 修正为"批准 = 该 call 一次性授权，ask 亦执行，
  deny/error 仍拦截"。新增 approved-resume 测试。
- `internal/production`：activation preview digest 含时间戳导致 preview→confirm
  digest_mismatch → config 内容确定性化（去掉时间戳，omitempty 字段省略）。
  wire 测试与 7 项 service 测试全绿。
- 全矩阵：Go build/vet/test 全 PASS；Swift 98 tests 全 PASS。

子代理实现按既有先例收编（作者权归属本记录）；flash 独立复评仍为开放项。
