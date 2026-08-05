# C-W1 生产化落地 — 生产可用验收包

**Status**: 证据齐备，待 Product Owner 确认（ACCEPTANCE REQUESTED）
**Date**: 2026-08-06
**Branch**: `codex/loom-platform-slice2`
**Release target**: v0.2.1 experimental（显式激活默认关闭）

## 1. 验收范围（C-W1-CONTRACT.md 冻结语义）

- `internal/production`：显式激活流程（preview 零写入 → confirm 需 preview
  digest + authorized_by → 先 Journal 事实后落盘 → 失败回滚）、launchd
  安装/回滚/恢复、沙箱注入（测试不污染系统 launchd）。
- Journal 事实：ProductionActivationPreviewed/Activated + ProductionConfigWritten
  （配置写入注入的沙箱根）。
- 默认关闭：激活需显式 human 批准；无激活时零系统变更。

## 2. 验收证据

| 项 | 证据 | 状态 |
|---|---|---|
| Gate 1 冻结 | `C-W1-CONTRACT.md`（Controller 冷读 + Product Owner Goal 指令） | ✅ |
| 核心实现 | `d4a1b9ee`（internal/production + daemon/app/api/TUI/Swift 表面） | ✅ |
| Controller 修复 | activation preview digest 确定性化（去时间戳）；TUI preview digest 回传 | ✅ |
| 跨客户端旅程（当前分支新鲜重跑） | `/private/tmp/cw1-current.Sx27ug`（`5aa03cdf-76fd-41b9-92a4-177f7b11f158`）：真实 daemon + Pi 运行时 + 原生 app + TUI；Production 屏 preview→confirm→激活；事实集 ProductionActivationActivated=1 + ProductionConfigWritten=1（配置写入注入沙箱根）；`verify-cw1-cross-client-journey.sh` PASS | ✅ |
| 全矩阵 | Go build/vet/test 全 PASS；Swift 98 tests PASS（实现时） | ✅ |
| 不变量 | 激活需 human 授权；配置写入先事实后落盘；沙箱根注入；无自动激活 | ✅ |
| 默认关闭 | 无显式激活时系统状态不变 | ✅ |

## 3. 验收结论（待确认）

```text
C-W1 = 生产可用验收证据齐备（ACCEPTANCE REQUESTED）
实现提交 = d4a1b9ee（+ Controller 修复）
旅程 = cw1-current（5aa03cdf…）verify PASS（2026-08-06 当前分支重跑）

## 5. 当前分支重跑（2026-08-06）

- cw1_plan 的 14-tab 导航（Board→Production）在 17 屏布局下不跨回绕，驱动
  无需改动即 PASS（`5aa03cdf…`）；既有旅程 `9568c47d…` 亦复验 PASS。
全矩阵 = Go 全 PASS
开放项 = flash 独立评审（通道不可用，记录 REVIEW-NOTES）
```

## 4. 真实激活

真实机器激活（写 ~/Library/Application Support/Loom 配置、launchd 常驻、
显式激活）属 C 类外部变更，必须在 Product Owner 显式批准后才会执行；
本包只申请"验收确认"，不自动激活。
