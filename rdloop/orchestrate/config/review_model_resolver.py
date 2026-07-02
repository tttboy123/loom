from __future__ import annotations
from typing import Iterable, Optional, Protocol

from .exceptions import ConfigurationError

class ModelListingProvider(Protocol):
    """与 provider 后端解耦的最小契约：能列出 /v1/models 即可。"""
    def list_models(self) -> list[str]: ...

# 旧硬编码值；显式列出以便一处拒绝
LEGACY_HARDCODED_IDS = frozenset({"gpt-5.4"})

# tier -> 偏好前缀，按 provider 实际可用集合就近匹配
_TIER_PREFIXES: dict[str, tuple[str, ...]] = {
    "gpt-5.x": ("gpt-5.2", "gpt-5.1", "gpt-5"),  # 由新到旧择优
    "gpt-5":   ("gpt-5",),
    "gpt-4.x": ("gpt-4.1", "gpt-4o", "gpt-4"),
    "claude":  ("claude-opus", "claude-sonnet"),
}

class ReviewModelResolver:
    """
    把"想要的 reviewer 模型"解析为 provider 后端 /v1/models 中的合法 id。
    - 优先使用 explicit_id；若它不在 provider.models 中则拒绝。
    - 否则按 preferred_tier 在 provider.models 中按 _TIER_PREFIXES 顺序挑首个匹配。
    - 任何路径下都不得返回 provider 列表外、或属于 LEGACY_HARDCODED_IDS 的值。
    """

    def __init__(
        self,
        provider: ModelListingProvider,
        explicit_id: Optional[str] = None,
        preferred_tier: Optional[str] = "gpt-5.x",
    ):
        self._provider = provider
        self._explicit_id = explicit_id
        self._preferred_tier = preferred_tier

    def resolve(self) -> str:
        available = list(self._provider.list_models() or [])
        available_set = set(available)

        if not available_set:
            raise ConfigurationError(
                "INVALID_REVIEWER_MODEL",
                "provider /v1/models returned empty; cannot select reviewer model",
            )

        # 路径 A：调用方显式指定
        if self._explicit_id is not None:
            if self._explicit_id in LEGACY_HARDCODED_IDS:
                raise ConfigurationError(
                    "INVALID_REVIEWER_MODEL",
                    f"explicit reviewer model_id '{self._explicit_id}' is a known "
                    f"invalid hardcoded value; use provider /v1/models to pick a real id",
                )
            if self._explicit_id not in available_set:
                raise ConfigurationError(
                    "INVALID_REVIEWER_MODEL",
                    f"explicit reviewer model_id '{self._explicit_id}' not in "
                    f"provider.models={available}",
                )
            return self._explicit_id

        # 路径 B：按 tier 在可用列表中择优
        prefixes = _TIER_PREFIXES.get(self._preferred_tier or "", ())
        for prefix in prefixes:
            for model_id in available:  # 保留 provider 返回顺序作为 tie-breaker
                if model_id.startswith(prefix):
                    return model_id

        raise ConfigurationError(
            "INVALID_REVIEWER_MODEL",
            f"no model matching tier '{self._preferred_tier}' in "
            f"provider.models={available}",
        )
