"""
把 CrewAI 子服务包成一个 Agno Tool（服务边界混用，不做框架嵌套）。

这样 Agno 主系统可以在需要 crew 式协作时“调用”CrewAI，
但两套框架各自独立运行、独立部署、独立扩缩容。
"""
import httpx
from agno.tools import tool

from app.settings import CREW_SERVICE_URL


@tool(name="run_research_crew", description="把一个复杂的深度研究任务交给 CrewAI 子服务处理，返回研究报告。")
def run_research_crew(topic: str) -> str:
    """调用独立部署的 CrewAI 服务执行一个 research crew。

    Args:
        topic: 要研究的主题/问题。
    """
    try:
        resp = httpx.post(
            f"{CREW_SERVICE_URL}/run",
            json={"topic": topic},
            timeout=300,
        )
        resp.raise_for_status()
        return resp.json().get("result", "")
    except Exception as e:
        # 子服务不可用时优雅降级：让上层 Agent 用自身能力兜底。
        return f"[crew 子服务不可用，已降级为本地处理] 错误: {e}"
