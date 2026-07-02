"""集中配置。所有 Agent 通过 LiteLLM 网关访问模型，不直接连厂商。"""
import os


# LiteLLM 网关地址（OpenAI 兼容）。Agno 的 model.id == LiteLLM 的 model_name。
LITELLM_BASE_URL = os.getenv("LITELLM_BASE_URL", "http://litellm:4000")
LITELLM_API_KEY = os.getenv("LITELLM_MASTER_KEY", "sk-local-dev")

# CrewAI 子服务（可选）。默认不依赖，按需开启。
CREW_SERVICE_URL = os.getenv("CREW_SERVICE_URL", "http://crew-service:8001")

# 数据库（AgentOS session / 历史 / 审计持久化）
DATABASE_URL = os.getenv("DATABASE_URL", "postgresql+psycopg://agno:agno@postgres:5432/agno")


# -----------------------------------------------------------------------------
# 逻辑模型名 —— 与 litellm/config.yaml 的 model_name 一一对应。
# 业务里只引用这些常量，换厂商 / 调降级只改 LiteLLM 配置。
# -----------------------------------------------------------------------------
class Models:
    # 与 litellm/config.full.yaml 的 5 个 model_name 对齐（降级链在 LiteLLM 配置里）。
    SMART = "codex-sub"            # 高质量主力：Codex 订阅（控制面主载体）
    REASON = "deepseek"            # 推理/思考：DeepSeek API（降级 -> glm -> minimax）
    FAST = "glm"                   # 便宜/后台：智谱 GLM API（降级 -> deepseek -> minimax）
    LONG = "codex-sub"             # 长上下文：Codex 订阅（控制面长文判断）
    CHEAP = "minimax"              # 轻量总结：MiniMax API（降级 -> glm -> deepseek）
