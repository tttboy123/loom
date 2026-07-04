"""
devkit.gate.opensource
======================

Open-source readiness Agent Gate for Loom.

This is a first-class Gate that sits alongside
``devkit.gate.evaluate_final_gate`` (the materialize gate) but is
**orthogonal** to it: where the materialize gate answers
"is this run's artifact set ready to be evaluated?", this gate
answers "is the repository itself ready to be open-sourced?".

Design goals (see ``docs/architecture/loom-opensource-agent-team.md``):

1. **Composable**: every check is an independent ``OpenSourceCheck`` dataclass
2. **Invokable from anywhere**: Loom pipeline, ``./loom doctor --opensource``,
   CI, pre-commit hook — same code path
3. **Severity-aware**: ``blocker`` (must fix) vs ``warning`` (should fix)
4. **Readable verdict**: returns a structured dict, ready for RUNS.md / UI
5. **Zero deps**: stdlib + pathlib only, runs offline

Public API
----------
- :func:`evaluate_opensource_gate` → dict
- :data:`OPEN_GO` / :data:`OPEN_NO_GO` / :data:`OPEN_GO_WITH_WARNINGS`
- :class:`OpenSourceCheck`
- :data:`default_checks`
"""

from __future__ import annotations

import re
import shutil
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable, Iterable, List, Optional

__all__ = [
    "OPEN_GO",
    "OPEN_NO_GO",
    "OPEN_GO_WITH_WARNINGS",
    "OpenSourceCheck",
    "CheckResult",
    "evaluate_opensource_gate",
    "default_checks",
]

# ----------------------------------------------------------------------------
# Verdict constants — kept short and grep-able
# ----------------------------------------------------------------------------
OPEN_GO = "GO"
OPEN_NO_GO = "NO-GO"
OPEN_GO_WITH_WARNINGS = "GO-WITH-WARNINGS"

# Severity levels
BLOCKER = "blocker"
WARNING = "warning"

# ----------------------------------------------------------------------------
# Personal-path patterns — paths that absolutely should not land in a public
# commit (security / privacy). Be conservative; false positives are tolerable.
# ----------------------------------------------------------------------------
_PATH_LEAK_PATTERNS = [
    re.compile(r"/Users/[\w.\-]+/?", re.IGNORECASE),       # macOS user dir
    re.compile(r"/home/[\w.\-]+/?", re.IGNORECASE),        # Linux user dir
    re.compile(r"C:\\Users\\[\w.\-]+\\?", re.IGNORECASE),  # Windows user dir
    re.compile(r"/workspace/[\w.\-]+/?", re.IGNORECASE),   # devcontainer
]

# Default paths we always inspect for path leaks.
# (Tracked docs only — never auto-touch node_modules / .venv / runs.)
_DEFAULT_DOC_GLOBS = [
    "*.md",
    "*.rst",
    "*.txt",
    "*.yaml",
    "*.yml",
    "*.toml",
    "*.json",
]

# Files we never consider for path-leak check.
# These are runtime state or build artifacts — not committed docs.
# Values match individual path parts (e.g. "runs" matches "devkit/runs/...").
_PATH_LEAK_SKIP = {
    # build / tooling
    "node_modules",
    ".venv",
    "venv",
    "__pycache__",
    ".next",
    ".git",
    # Loom runtime state (regenerated on every run)
    "runs",
    "task-runs",
    "reflections",
    "work-items",
    "logs",
    ".cache",
    "backlog.archive",
    # legacy / experiment
    "agent-platform",
    "external-requests",
    "applylock",   # legacy applylock dir vs devkit/applylock.py collision
}


@dataclass
class CheckResult:
    """A single check outcome."""

    name: str
    severity: str            # BLOCKER | WARNING
    passed: bool
    detail: str = ""
    remediation: str = ""    # hint for the human when failed

    def to_dict(self) -> dict:
        return {
            "name": self.name,
            "severity": self.severity,
            "passed": self.passed,
            "detail": self.detail,
            "remediation": self.remediation,
        }


@dataclass
class OpenSourceCheck:
    """A single composable open-source check.

    ``run`` is a pure function ``(repo_root: Path) -> CheckResult``;
    no side effects beyond reading the FS.
    """

    name: str
    severity: str
    run: Callable[[Path], CheckResult]
    description: str = ""


def _file_exists(repo_root: Path, rel: str) -> bool:
    return (repo_root / rel).is_file()


def _read_text(repo_root: Path, rel: str) -> str:
    p = repo_root / rel
    if not p.is_file():
        return ""
    try:
        return p.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return ""


# ----------------------------------------------------------------------------
# Individual checks
# ----------------------------------------------------------------------------

def check_license_present(repo_root: Path) -> CheckResult:
    """Repository must have a LICENSE file at the root.

    Accepts: LICENSE / LICENSE.md / LICENSE.txt
    """
    candidates = ["LICENSE", "LICENSE.md", "LICENSE.txt"]
    found = [c for c in candidates if _file_exists(repo_root, c)]
    if not found:
        return CheckResult(
            name="LicensePresent",
            severity=BLOCKER,
            passed=False,
            detail="No LICENSE file at repo root.",
            remediation="Add MIT LICENSE (recommended for community adoption).",
        )
    # Sanity: not a placeholder
    text = _read_text(repo_root, found[0]).strip()
    if len(text) < 200:
        return CheckResult(
            name="LicensePresent",
            severity=BLOCKER,
            passed=False,
            detail=f"{found[0]} exists but is suspiciously short ({len(text)} chars).",
            remediation="Replace with a real LICENSE text.",
        )
    return CheckResult(
        name="LicensePresent",
        severity=BLOCKER,
        passed=True,
        detail=f"Found {found[0]} ({len(text)} chars).",
    )


def check_contributing_present(repo_root: Path) -> CheckResult:
    if not _file_exists(repo_root, "CONTRIBUTING.md"):
        return CheckResult(
            name="ContributingPresent",
            severity=BLOCKER,
            passed=False,
            detail="CONTRIBUTING.md not found at repo root.",
            remediation="Add CONTRIBUTING.md with setup, run, test, PR instructions.",
        )
    return CheckResult(
        name="ContributingPresent",
        severity=BLOCKER,
        passed=True,
        detail="CONTRIBUTING.md present.",
    )


def check_coc_present(repo_root: Path) -> CheckResult:
    if not _file_exists(repo_root, "CODE_OF_CONDUCT.md"):
        return CheckResult(
            name="CodeOfConductPresent",
            severity=BLOCKER,
            passed=False,
            detail="CODE_OF_CONDUCT.md not found at repo root.",
            remediation="Add a Contributor Covenant (v2.1) based CoC.",
        )
    return CheckResult(
        name="CodeOfConductPresent",
        severity=BLOCKER,
        passed=True,
        detail="CODE_OF_CONDUCT.md present.",
    )


def check_security_present(repo_root: Path) -> CheckResult:
    if not _file_exists(repo_root, "SECURITY.md"):
        return CheckResult(
            name="SecurityPresent",
            severity=BLOCKER,
            passed=False,
            detail="SECURITY.md not found at repo root.",
            remediation="Add SECURITY.md with disclosure policy + ToS notes (esp. subscription proxies).",
        )
    return CheckResult(
        name="SecurityPresent",
        severity=BLOCKER,
        passed=True,
        detail="SECURITY.md present.",
    )


def check_github_templates(repo_root: Path) -> CheckResult:
    issue_tpl_dir = repo_root / ".github" / "ISSUE_TEMPLATE"
    pr_tpl = repo_root / ".github" / "PULL_REQUEST_TEMPLATE.md"
    missing = []
    if not issue_tpl_dir.is_dir():
        missing.append(".github/ISSUE_TEMPLATE/")
    elif not any(issue_tpl_dir.glob("*.md")):
        missing.append(".github/ISSUE_TEMPLATE/*.md (at least one)")
    if not pr_tpl.is_file():
        missing.append(".github/PULL_REQUEST_TEMPLATE.md")
    if missing:
        return CheckResult(
            name="GithubTemplatesPresent",
            severity=BLOCKER,
            passed=False,
            detail=f"Missing: {', '.join(missing)}",
            remediation="Add GitHub issue + PR templates so community contributions land well.",
        )
    return CheckResult(
        name="GithubTemplatesPresent",
        severity=BLOCKER,
        passed=True,
        detail="GitHub templates present.",
    )


def check_ci_workflow(repo_root: Path) -> CheckResult:
    wf_dir = repo_root / ".github" / "workflows"
    if not wf_dir.is_dir():
        return CheckResult(
            name="CIWorkflowPresent",
            severity=BLOCKER,
            passed=False,
            detail=".github/workflows/ directory not found.",
            remediation="Add at least one CI workflow (lint + tests).",
        )
    yml_files = list(wf_dir.glob("*.yml")) + list(wf_dir.glob("*.yaml"))
    if not yml_files:
        return CheckResult(
            name="CIWorkflowPresent",
            severity=BLOCKER,
            passed=False,
            detail="No .yml/.yaml files under .github/workflows/.",
            remediation="Add at least one CI workflow (lint + tests).",
        )
    return CheckResult(
        name="CIWorkflowPresent",
        severity=BLOCKER,
        passed=True,
        detail=f"Found {len(yml_files)} workflow file(s).",
    )


def check_gitignore(repo_root: Path) -> CheckResult:
    """Verify .gitignore excludes the noisy directories Loom produces."""
    gi = repo_root / ".gitignore"
    if not gi.is_file():
        return CheckResult(
            name="GitignoreComplete",
            severity=BLOCKER,
            passed=False,
            detail=".gitignore not found.",
            remediation="Add a .gitignore that excludes generated artifacts.",
        )
    text = gi.read_text(encoding="utf-8", errors="replace")
    required_patterns = [
        (".schemathesis/", "schemathesis cache"),
        (".hypothesis/", "hypothesis cache"),
        ("_diag/", "diagnostic scratch"),
        ("*.log", "log files"),
        ("devkit/MEMORY.md", "runtime MEMORY"),
        ("devkit/RUNS.md", "runtime RUNS ledger"),
        ("devkit/runs/", "run artifact directory"),
        ("devkit/logs/", "log directory"),
    ]
    missing = []
    for pattern, label in required_patterns:
        if pattern not in text:
            missing.append(f"{pattern}  ({label})")
    if missing:
        return CheckResult(
            name="GitignoreComplete",
            severity=BLOCKER,
            passed=False,
            detail=f"Missing patterns: {', '.join(missing)}",
            remediation="Update .gitignore to exclude these generated artifacts.",
        )
    return CheckResult(
        name="GitignoreComplete",
        severity=BLOCKER,
        passed=True,
        detail="All required .gitignore patterns present.",
    )


def check_worktree_clean(repo_root: Path) -> CheckResult:
    """Warn if there are uncommitted / untracked files.

    Not a blocker for the first publish — many OSS projects ship with
    'will be cleaned up post-launch' branches. But it should be visible.
    """
    if not (repo_root / ".git").is_dir():
        return CheckResult(
            name="WorktreeClean",
            severity=WARNING,
            passed=True,
            detail="Not a git repo; skipping.",
        )
    try:
        out = subprocess.run(
            ["git", "status", "--porcelain"],
            cwd=str(repo_root),
            capture_output=True,
            text=True,
            check=False,
            timeout=10,
        ).stdout.strip()
    except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
        return CheckResult(
            name="WorktreeClean",
            severity=WARNING,
            passed=True,
            detail=f"git unavailable ({exc!r}); skipping.",
        )
    if not out:
        return CheckResult(
            name="WorktreeClean",
            severity=WARNING,
            passed=True,
            detail="Working tree clean.",
        )
    lines = out.splitlines()
    return CheckResult(
        name="WorktreeClean",
        severity=WARNING,
        passed=False,
        detail=f"{len(lines)} uncommitted/untracked line(s) — review before publishing.",
        remediation="git status; decide keep/discard before open-sourcing.",
    )


def check_mock_mode(repo_root: Path) -> CheckResult:
    """Verify a mock LiteLLM config exists so new users can try Loom without keys."""
    candidates = [
        "litellm/config.mock.yaml",
        "litellm/config.mock.yml",
    ]
    found = [c for c in candidates if _file_exists(repo_root, c)]
    if not found:
        return CheckResult(
            name="MockModeRunnable",
            severity=WARNING,
            passed=False,
            detail="No mock LiteLLM config found.",
            remediation=(
                "Add litellm/config.mock.yaml so new users can "
                "`./loom up` without any API keys."
            ),
        )
    # Basic YAML sanity (we don't have pyyaml in stdlib; just check the header
    # line is recognizable)
    text = _read_text(repo_root, found[0])
    if "model_list" not in text:
        return CheckResult(
            name="MockModeRunnable",
            severity=WARNING,
            passed=False,
            detail=f"{found[0]} doesn't look like a LiteLLM config (no model_list).",
            remediation="Re-export or hand-fix the mock config.",
        )
    return CheckResult(
        name="MockModeRunnable",
        severity=WARNING,
        passed=True,
        detail=f"{found[0]} present and looks well-formed.",
    )


def check_paths_scrubbed(repo_root: Path) -> CheckResult:
    """Blocker: ensure no committed docs contain personal absolute paths."""
    leaks: List[str] = []
    inspected = 0
    for pattern in _DEFAULT_DOC_GLOBS:
        for path in repo_root.rglob(pattern):
            # Skip noisy / generated dirs
            parts = set(path.parts)
            if parts & _PATH_LEAK_SKIP:
                continue
            # Skip individual large runtime files
            if path.name in {"MEMORY.md", "RUNS.md", "backlog.json"}:
                continue
            try:
                rel = path.relative_to(repo_root).as_posix()
            except ValueError:
                continue
            # Only check small-to-medium files (no point reading huge logs)
            try:
                if path.stat().st_size > 512_000:  # 500KB
                    continue
            except OSError:
                continue
            try:
                content = path.read_text(encoding="utf-8", errors="replace")
            except OSError:
                continue
            inspected += 1
            for rx in _PATH_LEAK_PATTERNS:
                m = rx.search(content)
                if m:
                    leaks.append(f"{rel}: {m.group(0)!r}")
                    break
    if leaks:
        return CheckResult(
            name="PathsScrubbed",
            severity=BLOCKER,
            passed=False,
            detail=(
                f"Found personal-path leaks in {len(leaks)} file(s) "
                f"(inspected {inspected}). Examples: {leaks[:3]}"
            ),
            remediation=(
                "Replace absolute personal paths with relative refs "
                "(`./scripts/...`) or environment variables (`$REPO_ROOT`)."
            ),
        )
    return CheckResult(
        name="PathsScrubbed",
        severity=BLOCKER,
        passed=True,
        detail=f"No personal-path leaks across {inspected} file(s).",
    )


def check_consent_mentions(repo_root: Path) -> CheckResult:
    """Warn: SECURITY.md or README should acknowledge subscription-proxy ToS risk."""
    targets = ["SECURITY.md", "README.md"]
    keywords = ["subscription", "ToS", "terms of service", "服务条款", "限流"]
    found_in = []
    for f in targets:
        text = _read_text(repo_root, f)
        if not text:
            continue
        if any(k.lower() in text.lower() for k in keywords):
            found_in.append(f)
    if not found_in:
        return CheckResult(
            name="ConsentMentions",
            severity=WARNING,
            passed=False,
            detail="No subscription-proxy / ToS notice in SECURITY.md or README.md.",
            remediation=(
                "Document that subscription proxies (cliproxy / CLIProxyAPI) "
                "may violate provider ToS; users opt in."
            ),
        )
    return CheckResult(
        name="ConsentMentions",
        severity=WARNING,
        passed=True,
        detail=f"ToS notice found in: {', '.join(found_in)}",
    )


# ----------------------------------------------------------------------------
# Default check registry — the canonical v0.1 list
# ----------------------------------------------------------------------------
def default_checks() -> List[OpenSourceCheck]:
    return [
        OpenSourceCheck(
            name="LicensePresent",
            severity=BLOCKER,
            run=check_license_present,
            description="Repo root has a non-placeholder LICENSE.",
        ),
        OpenSourceCheck(
            name="ContributingPresent",
            severity=BLOCKER,
            run=check_contributing_present,
            description="CONTRIBUTING.md at repo root.",
        ),
        OpenSourceCheck(
            name="CodeOfConductPresent",
            severity=BLOCKER,
            run=check_coc_present,
            description="CODE_OF_CONDUCT.md at repo root.",
        ),
        OpenSourceCheck(
            name="SecurityPresent",
            severity=BLOCKER,
            run=check_security_present,
            description="SECURITY.md at repo root.",
        ),
        OpenSourceCheck(
            name="GithubTemplatesPresent",
            severity=BLOCKER,
            run=check_github_templates,
            description=".github/ISSUE_TEMPLATE/ + PULL_REQUEST_TEMPLATE.md.",
        ),
        OpenSourceCheck(
            name="CIWorkflowPresent",
            severity=BLOCKER,
            run=check_ci_workflow,
            description="At least one .github/workflows/*.yml.",
        ),
        OpenSourceCheck(
            name="GitignoreComplete",
            severity=BLOCKER,
            run=check_gitignore,
            description=".gitignore excludes generated artifacts.",
        ),
        OpenSourceCheck(
            name="PathsScrubbed",
            severity=BLOCKER,
            run=check_paths_scrubbed,
            description="No personal-path leaks in committed docs.",
        ),
        OpenSourceCheck(
            name="WorktreeClean",
            severity=WARNING,
            run=check_worktree_clean,
            description="git status is clean (warn, not block).",
        ),
        OpenSourceCheck(
            name="MockModeRunnable",
            severity=WARNING,
            run=check_mock_mode,
            description="Mock LiteLLM config present (zero-key demo).",
        ),
        OpenSourceCheck(
            name="ConsentMentions",
            severity=WARNING,
            run=check_consent_mentions,
            description="Subscription-proxy ToS notice in README/SECURITY.",
        ),
    ]


# ----------------------------------------------------------------------------
# Top-level evaluator
# ----------------------------------------------------------------------------
def evaluate_opensource_gate(
    repo_root: Path,
    checks: Optional[Iterable[OpenSourceCheck]] = None,
) -> dict:
    """Run every check and return a structured verdict.

    Returns
    -------
    dict
        ``{
          "verdict": OPEN_GO | OPEN_GO_WITH_WARNINGS | OPEN_NO_GO,
          "blockers": [CheckResult, ...],   # failed blocker checks
          "warnings": [CheckResult, ...],   # failed warning checks
          "checks":   [CheckResult, ...],   # all results, in run order
        }``
    """
    repo_root = Path(repo_root).resolve()
    checks = list(checks) if checks is not None else default_checks()

    results: List[CheckResult] = []
    for chk in checks:
        try:
            r = chk.run(repo_root)
        except Exception as exc:  # noqa: BLE001 — gate must never crash on a single check
            r = CheckResult(
                name=chk.name,
                severity=chk.severity,
                passed=False,
                detail=f"Check raised {type(exc).__name__}: {exc}",
                remediation="Investigate the check itself.",
            )
        results.append(r)

    blockers = [r for r in results if r.severity == BLOCKER and not r.passed]
    warnings = [r for r in results if r.severity == WARNING and not r.passed]

    if blockers:
        verdict = OPEN_NO_GO
    elif warnings:
        verdict = OPEN_GO_WITH_WARNINGS
    else:
        verdict = OPEN_GO

    return {
        "verdict": verdict,
        "blockers": [r.to_dict() for r in blockers],
        "warnings": [r.to_dict() for r in warnings],
        "checks": [r.to_dict() for r in results],
    }


# ----------------------------------------------------------------------------
# Tiny CLI for local use:  python -m devkit.gate.opensource <repo_root>
# ----------------------------------------------------------------------------
def _print_human(result: dict) -> None:
    verdict = result["verdict"]
    badge = {
        OPEN_GO: "[GO]",
        OPEN_GO_WITH_WARNINGS: "[GO + warnings]",
        OPEN_NO_GO: "[NO-GO]",
    }[verdict]
    print(f"\nOpen-source gate verdict: {badge}\n")
    for r in result["checks"]:
        status = "PASS" if r["passed"] else "FAIL"
        print(f"  {status:4} [{r['severity']:7}] {r['name']}: {r['detail']}")
    if result["blockers"]:
        print("\nBlockers:")
        for r in result["blockers"]:
            print(f"  - {r['name']}: {r['detail']}")
            if r["remediation"]:
                print(f"      fix: {r['remediation']}")
    if result["warnings"]:
        print("\nWarnings:")
        for r in result["warnings"]:
            print(f"  - {r['name']}: {r['detail']}")
            if r["remediation"]:
                print(f"      fix: {r['remediation']}")


def _cli(argv: List[str]) -> int:
    import json
    import sys

    if len(argv) < 2:
        print("Usage: python -m devkit.gate.opensource <repo_root> [--json]", file=sys.stderr)
        return 2
    repo_root = Path(argv[1]).resolve()
    json_out = "--json" in argv[2:]
    result = evaluate_opensource_gate(repo_root)
    if json_out:
        print(json.dumps(result, indent=2, ensure_ascii=False))
    else:
        _print_human(result)
    return 0 if result["verdict"] != OPEN_NO_GO else 1


if __name__ == "__main__":  # pragma: no cover
    import sys
    raise SystemExit(_cli(sys.argv))