from rdloop.orchestrate.config.review_model_resolver import ReviewModelResolver
from rdloop.orchestrate.config.exceptions import ConfigurationError

class ReviewCarrier:
    def __init__(
        self,
        provider,
        review_model_id: str | None = None,
        review_model_tier: str | None = "gpt-5.x",
        **kwargs,
    ):
        # 关键：解析失败必须以 ConfigurationError(failure_code="INVALID_REVIEWER_MODEL")
        # 抛出，review 阶段入口据此写失败事件并 abort。
        resolver = ReviewModelResolver(
            provider=provider,
            explicit_id=review_model_id,
            preferred_tier=review_model_tier,
        )
        try:
            self.model_id: str = resolver.resolve()
        except ConfigurationError:
            # 透传 failure_code，不在此层吞掉
            raise

        # ... 其余原有初始化逻辑保持不变 ...
