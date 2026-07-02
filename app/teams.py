"""
Team 定义 —— 把角色 Agent 编排成 Loom R&D Loop（Agno 原生多 Agent 协作）。

这就是「现成的编排器」：你后续不用再自己搭编排系统，直接对这个 Team 发任务，
由 orchestrator（leader）按 RD-LOOP 阶段分派给 product / dev / tester / reviewer。
想换某个角色的模型，只改 config.full.yaml 的对应 loom-* 载体；这里不动。
"""
from agno.team import Team

from app.agents import ROLE_AGENTS, gw
from app.roles import ROLES_BY_KEY


# leader 用「编排角色」的模型载体来做协调/分派。
leader_model = gw(ROLES_BY_KEY["orchestrator"].model)

rd_loop_team = Team(
    name="LoomRDLoop",
    model=leader_model,
    members=[
        ROLE_AGENTS["product"],
        ROLE_AGENTS["dev"],
        ROLE_AGENTS["tester"],
        ROLE_AGENTS["reviewer"],
    ],
    instructions=[
        "按 RD-LOOP 阶段编排：先 ProductLogic 定产品判断，再分派实现计划，",
        "Developer 走 TDD（先失败测试再实现），Tester 做验证/eval，",
        "最后交给 Reviewer 做独立审查（不共享实现者假设）。",
        "汇总各角色结果，产出统一、可验证、标注待人类确认项的最终答复。",
        "独立性 > 连续性：审查发现假完成必须如实点名，不橡皮图章。",
    ],
    markdown=True,
)

ALL_TEAMS = [rd_loop_team]
