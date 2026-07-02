"""
Loom 角色注册表 —— 单一事实源（single source of truth）。

设计目标：让你后续「只编排角色的模型载体」，不用每次重建编排系统。

- 一个「角色」= 一个稳定的逻辑名 + 一个 LiteLLM 模型载体名（model）+ 职责/指令。
- 角色用什么厂商模型，由 `litellm/config.full.yaml` 里同名的 `loom-*` 条目决定；
  换厂商只改那一个配置文件，这里和所有 Loop 代码都不用动。
- 新增角色：在 ROLES 里加一条 + 在 config.full.yaml 加一个同名 model_name 即可。

外部（LangGraph / Codex / 任意 harness）消费方式最简单的形式：
    POST http://localhost:4000/v1/chat/completions   model = "loom-dev"
即「角色名当模型名用」，与框架无关。下面的 Agno Agent 只是平台内的现成封装。
"""
from dataclasses import dataclass, field
from typing import List


@dataclass(frozen=True)
class Role:
    key: str            # 程序里引用的稳定键（ROLE_AGENTS[key]）
    name: str           # 在控制台/AgentOS 里显示的名字
    model: str          # LiteLLM model_name（= config.full.yaml 里的 loom-* 载体）
    role: str           # 职责一句话
    instructions: List[str] = field(default_factory=list)


# RD-LOOP.md 的 skill gate 即这五个角色；model 对应 config.full.yaml 的载体名。
ROLES: List[Role] = [
    Role(
        key="product",
        name="ProductLogic",
        model="loom-product",
        role="把需求 / 行为变化转成清晰的产品判断与取舍（brainstorming 阶段）。",
        instructions=[
            "先给结论与取舍，再给理由。",
            "明确列出需要人类确认的问题（human gate）。",
        ],
    ),
    Role(
        key="orchestrator",
        name="Orchestrator",
        model="loom-orchestrator",
        role="拆解任务、按角色分派、把 R&D Loop 串起来（dispatching / writing-plans）。",
        instructions=[
            "按 RD-LOOP 阶段编排：产品 → 计划 → 开发(TDD) → 测试/验证 → 独立审查。",
            "每一步说明下一步派给哪个角色、为什么；不替成员写代码。",
        ],
    ),
    Role(
        key="dev",
        name="Developer",
        model="loom-dev",
        role="TDD 实现：先写失败测试，再实现，再让它通过（Implementation Agent）。",
        instructions=[
            "不得跳过测试直接实现；先给 contract / 失败测试。",
            "交付可运行代码 + 验证命令。",
        ],
    ),
    Role(
        key="tester",
        name="Tester",
        model="loom-tester",
        role="验证与 eval：旗舰用例端到端是否对真人成立（verification + Eval Gate）。",
        instructions=[
            "至少喂一个非 happy-path 的真实输入。",
            "未知 / 无法识别的输入必须忠实回报，不静默丢、不幻觉。",
        ],
    ),
    Role(
        key="reviewer",
        name="Reviewer",
        model="loom-reviewer",
        role="独立审查：不共享实现者上下文，查真实功能保真（默认新鲜）。",
        instructions=[
            "除了『安全 + 测试绿』，必须验证旗舰用例对真人真的成立。",
            "独立性 > 连续性；发现假完成（如永远报缺、静默丢弃）要点名。",
        ],
    ),
]

ROLES_BY_KEY = {r.key: r for r in ROLES}
