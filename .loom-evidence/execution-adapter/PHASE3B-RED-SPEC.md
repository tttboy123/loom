# Phase 3B RED Spec

Date: 2026-08-06

| RED | 场景 | 断言 |
|---|---|---|
| 1 | Required 且后端不可用 | 拒绝执行零副作用（fail-closed，不兜底） |
| 2 | 7 能力状态事实原子落 Journal | Create/Exec/Cancel/Pause/Resume/Destroy 事实 |
| 3 | Exec 失败/取消 | Failed/Cancelled 事实；无重复副作用 |
| 4 | stale generation 实例操作 | 拒绝 |
| 5 | 后端回调非权威 | Journal 重建 reconcile 一致 |
| 6 | 重启/重放未 Destroy 实例 | 标记并清理，不重复执行 |
| 7 | 无凭证泄漏 | env digest 无敏感值；workspace 仅绑定 Run |
| 8 | 双 Job 隔离 | 实例绑定 generation/workspace |
| 9 | InspectCapabilities 门禁 | Required 时不可用 → 拒绝 |
