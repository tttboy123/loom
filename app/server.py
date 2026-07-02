"""
AgentOS 入口 —— 把所有 Agent / Team 暴露成多用户 API。
自带 session、tracing、调度、RBAC、审计（控制面）。

本地运行:   python -m app.server
容器内:     uvicorn app.server:app --host 0.0.0.0 --port 8000
"""
from agno.os import AgentOS
from agno.db.postgres import PostgresDb

from app.agents import ALL_AGENTS
from app.teams import ALL_TEAMS
from app.settings import DATABASE_URL

# 持久化层：session / run 历史 / 审计统一落库，便于云上多副本共享状态。
db = PostgresDb(db_url=DATABASE_URL)

agent_os = AgentOS(
    name="multi-vendor-agent-platform",
    description="Agno 主编排 + LiteLLM 多厂商网关 + 可选 CrewAI 子服务",
    agents=ALL_AGENTS,
    teams=ALL_TEAMS,
    db=db,
)

# FastAPI app，可直接交给 uvicorn / gunicorn。
app = agent_os.get_app()


if __name__ == "__main__":
    # 本地快速起服务（含自带控制台）。
    agent_os.serve(app="app.server:app", host="0.0.0.0", port=8000, reload=True)
