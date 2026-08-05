# W-AUTONOMY RED Spec

Date: 2026-08-06

| RED | 场景 | 断言 |
|---|---|---|
| 1 | 定义 order 默认 inactive | Journal define 事实；零 dispatch |
| 2 | Autopilot 默认 off；显式激活 | off 零行为；human 批准后才 dispatch |
| 3 | 触发器/scope/budget/max 全满足 | dispatch 一次；任一不满足 fail-closed |
| 4 | deny/admin lock/危险/沙箱优先 | order 不绕过；Eval 门禁不变 |
| 5 | budget 耗尽 | stop 事实；不再 dispatch；消耗 append-only |
| 6 | max_iterations 达到 | stop；不超发 |
| 7 | revoke | 后续拒绝；in-flight 不重复执行 |
| 8 | scope/generation/触发器失配 | 拒绝 dispatch |
| 9 | 全生命周期事实可重建 | 无第二权威 |
| 10 | 双 order 隔离 + 幂等 | 不越界；重放不重复消耗/执行 |
