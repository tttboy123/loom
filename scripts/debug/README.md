# scripts/debug/

本目录是诊断 / 临时工具脚本。历史原因：早期它们放在仓库根目录，便于
直接 `python3 ast.parse` 调试。后来把它们统一挪到这里，避免污染根目录。

## 这些脚本的来源

| 脚本 | 用途 | 何时用 |
|---|---|---|
| `ast.parse` | 单文件 AST 解析触发点 | 调试 devkit rdloop 的 preverify 路径 |
| `env_gate.py` | 环境变量 / 路径门控判定 | 被 `tests/unit/test_env_gate.py` 引用 |
| `looper.py` | 占位符名称校验 | 被 `tests/test_verify_placeholder.py` 引用 |
| `report_env.py` | 报告 run 环境（基于 env_gate） | 被 `tests/contract/test_report_only_assertions.sh` 引用 |
| `locate_sandbox_runner_script.py` | 定位沙箱 runner | 被 `tests/test_locate_sandbox_runner.py` 引用 |
| `materialize_fs_truth.py` | 一次性 fs-truth 探针 | 临时诊断用，可删 |
| `sandbox_env.json` | 沙箱环境快照（pytest 版本等） | 一次性输出，**不入仓**（已 gitignore） |

## 长期计划

这些脚本应该被整合进 `devkit/` 的合适模块：

- `env_gate.py` + `report_env.py` → `devkit/env_gate.py` + `devkit/report_env.py`
- `looper.py` → 已被 `devkit/looper.py` 取代，但 stub 保留供老测试用
- `locate_sandbox_runner_script.py` → 整合进 `devkit/sandbox/`
- `materialize_fs_truth.py` → 删
- `sandbox_env.json` → 删

跟踪 issue：[#TBD]
