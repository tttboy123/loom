"""自我修改护栏：判定文件路径是否必须人类 apply。"""
from __future__ import annotations

import os
import pathlib
from dataclasses import dataclass

# harness 核心文件名集合——自治体绝不能自动修改这些文件。
CRITICAL_BASENAMES = {
    "rdloop.py",
    "evals.py",
    "autoloop.py",
    "evidence.py",
    "ratchet.py",
    "stopcheck.py",
    "applylock.py",
}
DEFAULT_ALLOWED_TEST_PREFIXES = (
    "tests/unit/",
    "tests/contract/",
    "tests/test_diag",
    "harness/tests/",
    "build/",
    "sandbox/build/",
    "devkit/tests/",
)
DEFAULT_APPLY_MODE_ALLOWED_TEST_PREFIXES = (
    *DEFAULT_ALLOWED_TEST_PREFIXES,
    "tests/",
)
DEFAULT_ALLOWED_NONTEST_PREFIXES = (
    "runs/",
    "build/",
)
SELF_EXEMPT_PATHS = frozenset(
    {
        "harness/applylock.py",
        "harness/agent_runner.py",
        "harness/conftest.py",
    }
)
_ENV_ALLOW = "DEV_APPLYLOCK_ALLOW"


def _normalize(path: str) -> str:
    return str(path or "").strip().replace("\\", "/").lstrip("./")


def _split_env_list(raw: str | None) -> tuple[str, ...]:
    if not raw:
        return ()
    return tuple(_normalize(part) for part in raw.split(",") if _normalize(part))


def env_allowed_paths() -> tuple[str, ...]:
    return _split_env_list(os.environ.get(_ENV_ALLOW))


@dataclass(frozen=True)
class RunContext:
    run_id: str
    runs_dir: pathlib.Path

    @property
    def override_log_path(self) -> pathlib.Path:
        return self.runs_dir / self.run_id / "applylock_override.log"

    @classmethod
    def get(cls, run_id: str | None = None, runs_dir: str | os.PathLike[str] = "devkit/runs") -> "RunContext":
        rid = str(run_id or "default-run")
        root = pathlib.Path(runs_dir)
        return cls(run_id=rid, runs_dir=root)


class ApplyLock:
    def __init__(
        self,
        *,
        critical_basenames: set[str] | None = None,
        allowed_test_prefixes: tuple[str, ...] | None = None,
        allowed_non_test_prefixes: tuple[str, ...] | None = None,
    ) -> None:
        self.critical_basenames = set(critical_basenames or CRITICAL_BASENAMES)
        self.allowed_test_prefixes = tuple(allowed_test_prefixes or DEFAULT_ALLOWED_TEST_PREFIXES)
        self.allowed_non_test_prefixes = tuple(allowed_non_test_prefixes or DEFAULT_ALLOWED_NONTEST_PREFIXES)
        self._run_overrides: dict[str, set[str]] = {}

    def _is_test_file(self, path: str) -> bool:
        base = os.path.basename(path)
        return base.startswith("test_") and base.endswith(".py")

    def _is_test_like_path(self, path: str) -> bool:
        base = os.path.basename(path)
        return base.startswith("test_") or base == "conftest.py"

    def _matches_prefix(self, path: str, prefixes: tuple[str, ...]) -> bool:
        return any(path.startswith(prefix) for prefix in prefixes)

    def _is_allowlisted_test_path(self, path: str) -> bool:
        base = os.path.basename(path)
        if base == "conftest.py":
            return any(path == f"{prefix.rstrip('/')}/conftest.py" for prefix in self.allowed_test_prefixes)
        if not self._is_test_file(path):
            return False
        return self._matches_prefix(path, self.allowed_test_prefixes)

    def _is_explicitly_allowed(self, path: str, ctx: RunContext | None) -> bool:
        if path in SELF_EXEMPT_PATHS:
            return True
        if path in env_allowed_paths():
            return True
        if ctx and path in self._run_overrides.get(ctx.run_id, set()):
            return True
        return False

    def exemption_reason(self, path: str, ctx: RunContext | None = None) -> str | None:
        norm = _normalize(path)
        if not norm:
            return None
        if norm in SELF_EXEMPT_PATHS:
            return "harness-self-exempt"
        if norm in env_allowed_paths():
            return "env-override"
        if ctx and norm in self._run_overrides.get(ctx.run_id, set()):
            return "run-override"
        if self._is_allowlisted_test_path(norm):
            return "test-prefix-allowlist"
        if self._matches_prefix(norm, self.allowed_non_test_prefixes):
            return "non-test-prefix-allowlist"
        return None

    def is_protected(self, path: str, ctx: RunContext | None = None) -> bool:
        norm = _normalize(path)
        if not norm:
            return False
        if self._is_explicitly_allowed(norm, ctx):
            return False
        base = os.path.basename(norm)
        if base in self.critical_basenames or norm.endswith(".golden.json"):
            return True
        if self._is_test_like_path(norm):
            return not self._is_allowlisted_test_path(norm)
        if self._matches_prefix(norm, self.allowed_non_test_prefixes):
            return False
        return False

    def allow_once(self, path: str, ctx: RunContext) -> pathlib.Path:
        norm = _normalize(path)
        if not norm:
            raise ValueError("path is required")
        base = os.path.basename(norm)
        if base in self.critical_basenames or norm.endswith(".golden.json"):
            raise ValueError(f"cannot bypass critical applylock path: {norm}")
        bucket = self._run_overrides.setdefault(ctx.run_id, set())
        bucket.add(norm)
        log_path = ctx.override_log_path
        log_path.parent.mkdir(parents=True, exist_ok=True)
        with log_path.open("a", encoding="utf-8") as fh:
            fh.write(f"ALLOW {norm} run_id={ctx.run_id}\n")
        return log_path


_DEFAULT_LOCK = ApplyLock()


def is_protected(path: str, ctx: RunContext | None = None) -> bool:
    return _DEFAULT_LOCK.is_protected(path, ctx)


def allow_once(path: str, ctx: RunContext) -> pathlib.Path:
    return _DEFAULT_LOCK.allow_once(path, ctx)


def requires_human(path: str) -> bool:
    """判断路径是否必须人类 apply。"""
    return is_protected(path)


def classify(path: str) -> str:
    """将路径分类为 'human'（需人类 apply）或 'auto'（可自动 apply）。"""
    return "human" if requires_human(path) else "auto"


def classify_manifest(entries: list[dict], *, lock: ApplyLock | None = None, ctx: RunContext | None = None) -> dict:
    active_lock = lock or _DEFAULT_LOCK
    blocked: list[dict] = []
    allowed: list[dict] = []
    for entry in entries or []:
        path = _normalize(entry.get("path", ""))
        if not path:
            continue
        kind = str(entry.get("kind", "candidate") or "candidate")
        applyable = bool(entry.get("applyable", False))
        if not applyable:
            allowed.append({"path": path, "kind": kind, "reason": "non_applyable"})
            continue
        if active_lock.is_protected(path, ctx):
            blocked.append({"path": path, "kind": kind, "reason": _blocked_reason(kind)})
        else:
            allowed.append({"path": path, "kind": kind, "reason": active_lock.exemption_reason(path, ctx) or "auto"})
    return {"blocked": blocked, "allowed": allowed}


def _blocked_reason(kind: str) -> str:
    if kind == "test":
        return "blocked_test_artifact"
    if kind in {"verify", "evidence", "report"}:
        return "blocked_verify_artifact"
    return "blocked_prod_code"
