"""
可选的 CrewAI 子服务（独立部署）。
默认不启用 —— 只有当你确实需要 crew 式深度协作时才上。

它同样通过 LiteLLM 网关取模型（统一降级、统一计费），
对外只暴露一个 HTTP 接口，被 Agno 主系统当 Tool 调用。
"""
import os
from fastapi import FastAPI
from pydantic import BaseModel
from crewai import Agent, Task, Crew, Process, LLM

LITELLM_BASE_URL = os.getenv("LITELLM_BASE_URL", "http://litellm:4000")
LITELLM_API_KEY = os.getenv("LITELLM_MASTER_KEY", "sk-local-dev")


def gw(model_id: str) -> LLM:
    # CrewAI 内置 LiteLLM；这里显式指向自建网关，复用同一套降级链。
    return LLM(
        model=f"openai/{model_id}",
        base_url=LITELLM_BASE_URL,
        api_key=LITELLM_API_KEY,
    )


researcher = Agent(
    role="资深研究员",
    goal="对给定主题做深入、可验证的调研",
    backstory="擅长拆解问题、交叉验证多个来源。",
    llm=gw("claude-sonnet"),     # 与 Agno 侧共享同一逻辑模型名
)
analyst = Agent(
    role="分析师",
    goal="把调研材料提炼成结构化洞察",
    backstory="擅长归纳与权衡。",
    llm=gw("deepseek-reasoner"),
)

app = FastAPI(title="CrewAI Research Service")


class RunRequest(BaseModel):
    topic: str


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/run")
def run(req: RunRequest):
    t1 = Task(description=f"研究主题：{req.topic}", expected_output="带来源的研究要点", agent=researcher)
    t2 = Task(description="把研究要点提炼成结论与建议", expected_output="结构化结论", agent=analyst)
    crew = Crew(agents=[researcher, analyst], tasks=[t1, t2], process=Process.sequential)
    result = crew.kickoff()
    return {"result": str(result)}
