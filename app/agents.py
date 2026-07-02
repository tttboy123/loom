"""
Agent 定义 —— 从 `app/roles.py` 注册表自动构建「角色 Agent」。

每个角色 Agent 通过 OpenAILike 指向 LiteLLM 网关，id = 角色的模型载体名
（loom-*）。降级（fallback）不在这里写，由 LiteLLM 按 model_name 统一处理：
框架层只管「这个角色用哪个载体」，网关层管「载体挂了切哪个厂商」，职责分离。

要新增/改角色：改 `app/roles.py`（和 config.full.yaml 的同名载体），这里无需改动。
"""
from agno.agent import Agent
from agno.models.openai.like import OpenAILike

from app.settings import LITELLM_BASE_URL, LITELLM_API_KEY
from app.roles import ROLES


def gw(model_id: str) -> OpenAILike:
    """构造一个走 LiteLLM 网关的模型句柄（OpenAI 兼容）。"""
    return OpenAILike(
        id=model_id,
        base_url=LITELLM_BASE_URL,
        api_key=LITELLM_API_KEY,
    )


def build_role_agent(r) -> Agent:
    return Agent(
        name=r.name,
        model=gw(r.model),
        role=r.role,
        instructions=list(r.instructions),
        markdown=True,
    )


# key -> Agent，供 teams.py 和外部按角色取用。
ROLE_AGENTS = {r.key: build_role_agent(r) for r in ROLES}

# AgentOS / 控制台里直接可对话的全部角色 Agent。
ALL_AGENTS = list(ROLE_AGENTS.values())
