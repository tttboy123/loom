# C-W1 Progress

Updated: 2026-08-05

- Gate 1 FROZEN（C-W1-CONTRACT.md；flash 独立评审开放项）。
- 核心实现提交 `d4a1b9ee`：internal/production 显式激活/安装回滚/恢复 +
  daemon/app/api/TUI/Swift 表面。
- Controller 修复：activation preview digest 确定性化（去时间戳）；
  TUI preview digest 经消息回传（闭包副本赋值失效修复）。
- 跨客户端旅程 **PASS**：`/private/tmp/cw1-journey-final2`
  （journey_id `9568c47d-6852-4eed-832a-78c3771247c8`）——Production 屏
  preview→confirm→激活，ProductionActivationActivated +
  ProductionConfigWritten 事实齐全（配置写入注入的沙箱根），
  `verify-cw1-cross-client-journey.sh` PASS。
- 全矩阵：Go build/vet/test 全 PASS；Swift 98 tests PASS。
