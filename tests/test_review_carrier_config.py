import pytest
from rdloop.orchestrate.carriers.review_carrier import ReviewCarrier
from rdloop.orchestrate.config.exceptions import ConfigurationError

class FakeProvider:
    def __init__(self, models):
        self._models = list(models)
    def list_models(self):
        return list(self._models)

def _build_carrier(provider, explicit=None, tier=None):
    return ReviewCarrier(
        provider=provider,
        review_model_id=explicit,
        review_model_tier=tier,
    )

# ---- 旗舰用例：review carrier 用合法 id 正常初始化 ----
def test_review_carrier_init_with_legal_resolved_id():
    provider = FakeProvider(["gpt-5.1", "gpt-5.2", "claude-opus-4.7"])
    carrier = _build_carrier(provider, tier="gpt-5.x")
    assert carrier.model_id in provider.list_models()

# ---- 非 happy-path：硬编码 'gpt-5.4' 被拒 ----
def test_review_carrier_rejects_hardcoded_gpt_5_4():
    provider = FakeProvider(["gpt-5.1", "gpt-5.2"])
    with pytest.raises(ConfigurationError) as exc_info:
        _build_carrier(provider, explicit="gpt-5.4")
    assert exc_info.value.failure_code == "INVALID_REVIEWER_MODEL"

# ---- 非 happy-path：解析结果不在 provider.models 中 ----
def test_review_carrier_rejects_resolved_id_not_in_provider():
    # 模拟 resolver 返回了 provider 已下线的 id
    class EvolvingProvider(FakeProvider):
        def list_models(self):
            return ["claude-opus-4.7"]  # resolver 仍倾向 gpt-5
    provider = EvolvingProvider(["gpt-5.1"])
    with pytest.raises(ConfigurationError) as exc_info:
        _build_carrier(provider, tier="gpt-5.x")
    assert exc_info.value.failure_code == "INVALID_REVIEWER_MODEL"
