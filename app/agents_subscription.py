"""
示例：让不同 Agent 分别走「订阅产品 / API」后端 —— Codex 订阅 / MiniMax API。

前提：CLIProxyAPI 已完成 Codex 登录，且 LiteLLM 已加载当前 config.full.yaml。

这里的 model id 仍然只是 LiteLLM 的逻辑名；
"用订阅还是用 API"、"挂了切谁" 全在 LiteLLM 配置里，业务代码不感知。
"""
from agno.agent import Agent
from app.agents import gw   # 复用同一个走网关的工厂函数


# 这个 Agent 用你的 ChatGPT/Codex 订阅，适合作为控制面高质量判断载体
codex_sub_agent = Agent(
    name="CodexSubControl",
    model=gw("codex-sub"),
    role="控制面高质量 Agent，优先消耗 Codex 订阅额度。",
    markdown=True,
)

# 这个 Agent 走 MiniMax API，适合作为执行面或控制面降级载体
minimax_exec_agent = Agent(
    name="MiniMaxExec",
    model=gw("minimax"),
    role="执行面 / 降级 Agent，优先保证稳定性与吞吐。",
    markdown=True,
)

SUBSCRIPTION_AGENTS = [codex_sub_agent, minimax_exec_agent]
