# Loom 开发流程套件（devkit）

`devkit` 是 Loom 的执行内核：用「角色模型载体」跑研发闭环，产出 task / plan / build / verify / review / cost / gate 等可回放证据。

它只负责“怎么开发”，不碰任何产品 runtime 的真实用户、权限、支付、生产数据或线上发布。

## 怎么跑

```bash
cd agent-platform
# 全 5 阶段
python3 -m devkit "实现一个小功能，要求有测试和审查"

# Loom 推荐入口：自动 backlog / loop / 续跑
./loom run "实现一个小功能，要求有测试和审查"

# 只跑订阅就能跑通的 3 个阶段
python3 -m devkit "……" --stages brainstorm,plan,implement
```

只依赖 Python 标准库；打的是本机 LiteLLM 网关 `:4000`，master key 自动从 `.env` 读。

## 五个阶段（= 角色 = 载体）

| 阶段 | 角色 | 载体（model） | 默认厂商 |
| --- | --- | --- | --- |
| brainstorm | 产品逻辑 | `loom-product` | GPT-5.4 |
| plan | 编排 | `loom-orchestrator` | GPT-5.4 |
| implement | 开发(TDD) | `loom-dev` | MiniMax-M3 |
| verify | 测试/eval | `loom-tester` | MiniMax-M3 |
| review | 独立审查 | `loom-reviewer` | GPT-5.4 |

换某阶段的厂商：只改 `litellm/config.full.yaml` 的对应 `loom-*` 载体，本套件不动。

## 产物

每次运行在 `devkit/runs/<时间戳>/` 下生成：

- `00-task.md`
- 每阶段 `NN-<stage>.md`
- `artifact.json`
- `build/`
- `run-log.md`
- verify / review / gate 结论

`run-log.md` 会记录**每阶段实际由哪个模型服务**、用时、状态、token、成本和 go/no-go 建议。

## 当前能力

- `--iterate N`：验证或审查 NO-GO 时，把失败细节回灌给构建者，最多 N 轮。
- `--contract N`：实现前生成可机器验证的验收点。
- `--golden cases.json`：运行质量回归。
- `--budget 0.05`：超过软预算即停止剩余阶段。
- `--recipe cheap-dev`：套用低成本开发预设。
- `--ponytail`：启用最小 diff / 零新依赖 / 无多余抽象的审查门。
- `--executor implement=hermes|openclaw|codex`：按阶段接入外部 agent harness。
- `--apply DIR` / `--apply-git REPO`：测试绿后经人类门落盘，不自动 push。

## 重要发现 / 设计说明

- **Claude Code 订阅是「带工具的 agentic 人格」**，当成 flat chat 调用时会吐工具调用语法
  （`<write_file>`、`<list_files>` 等）。套件内置 `normalize()`：把 `write_file` 内容抽成代码块、
  在**代码围栏之外**清掉一切伪工具标签——既保留真实代码，又得到干净产物。
- 推荐用法上的分工：**Claude Code 订阅最适合做真正“执行”的 dev**（接真实工具/worktree 时它的
  工具调用是特性而非噪音）；**纯文字推理/审查类角色**（product/orchestrator/tester/reviewer）
  用 Codex / API 模型更干净。载体层让你随时切换，不动 Loop 代码。
- 支持 **L1 / report-only** 与 **L2 / autonomous**：前者只产出建议与草案，后者可自治执行到工作树；是否走 git 仍需显式选择。
- 配合 Loom 宪章：换载体/厂商后按 golden 集重跑 **Eval Gate**；`:4000/ui` 是统一用量/成本/降级日志收口。
