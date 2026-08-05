# W-BRIDGE RED Spec

Date: 2026-08-06

| RED | 场景 | 测试 | 断言 |
|---|---|---|---|
| 1 | 完整信封 allow → 执行一次 | TestBridgeAllowExecutesOnce | 桥接事件→Execute→Completed；结果回桥接 |
| 2 | ask → 批准后恢复一次 | TestBridgeAskApprovedResumes | 零执行→批准→同一 digest 执行一次 |
| 3 | deny/stale/未知工具/解析失败 | TestBridgeDenyAndMalformedNeverExecute | 零执行零事实 |
| 4 | 多余字段/嵌套/注入拒绝 | TestBridgeStrictEnvelope | 严格解码拒绝 |
| 5 | 系统提示无 hidden 指令 | TestBridgeSystemPromptContract | 常量断言 |
| 6 | audit 只含已批准结果 | TestBridgeAuditOnlyApproved | transcript 过滤 |
| 7 | generation 围栏 | TestBridgeStaleGenerationRejected | 拒绝不执行 |
| 8 | 双 Job 隔离 + 幂等 | TestBridgeParallelAndIdempotent | 不越界、不重执行 |
