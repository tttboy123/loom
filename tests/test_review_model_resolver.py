import pytest
from rdloop.orchestrate.config.review_model_resolver import ReviewModelResolver
from rdloop.orchestrate.config.exceptions import ConfigurationError

class FakeProvider:
    """模拟 /v1/models 响应。"""
    def __init__(self, models):
        self._models = list(models)

    def list_models(self):
        return list(self._models)

# ---- 旗舰用例：合法等价 id 可被解析 ----
def test_resolver_picks_legal_equivalent_for_gpt5_family():
    provider = FakeProvider([
        "gpt-5.1", "gpt-5.2", "gpt-5-mini", "claude-opus-4.7",
    ])
    resolver = ReviewModelResolver(
        provider=provider,
        preferred_tier="gpt-5.x",
    )
    resolved = resolver.resolve()
    assert resolved in provider.list_models()
    assert resolved.startswith("gpt-5")

# ---- 非 happy-path：请求的 tier 在 provider 中无任何候选 ----
def test_resolver_raises_when_no_candidate_in_provider():
    provider = FakeProvider(["claude-opus-4.7", "llama-3.1-70b"])
    resolver = ReviewModelResolver(
        provider=provider,
        preferred_tier="gpt-5.x",
    )
    with pytest.raises(ConfigurationError) as exc_info:
        resolver.resolve()
    assert exc_info.value.failure_code == "INVALID_REVIEWER_MODEL"

# ---- 非 happy-path：明确指定非法 id 且 provider 不含 ----
def test_resolver_rejects_hardcoded_illegal_id_gpt_5_4():
    provider = FakeProvider(["gpt-5.1", "gpt-5.2"])
    resolver = ReviewModelResolver(
        provider=provider,
        explicit_id="gpt-5.4",  # 旧硬编码值
    )
    with pytest.raises(ConfigurationError) as exc_info:
        resolver.resolve()
    assert exc_info.value.failure_code == "INVALID_REVIEWER_MODEL"
    assert "gpt-5.4" in str(exc_info.value)

# ---- 边界：provider 返回空列表 ----
def test_resolver_raises_on_empty_provider_models():
    provider = FakeProvider([])
    resolver = ReviewModelResolver(provider=provider, preferred_tier="gpt-5.x")
    with pytest.raises(ConfigurationError) as exc:
        resolver.resolve()
    assert exc.value.failure_code == "INVALID_REVIEWER_MODEL"
