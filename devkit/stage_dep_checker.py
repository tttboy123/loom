# devkit/stage_dep_checker.py
"""Pure standard-library stage dependency checker."""

from __future__ import annotations

from typing import Dict, List, Set

def check(stage: str, deps: List[str], completed: Set[str]) -> Dict:
    """Check whether a single stage's dependencies are satisfied."""
    missing = [d for d in deps if d not in completed]
    return {
        "stage": stage,
        "ready": len(missing) == 0,
        "missing": missing,
    }

def check_all(stages: Dict[str, List[str]], completed: Set[str]) -> List[Dict]:
    """Check all stages' dependencies."""
    return [check(name, deps, completed) for name, deps in stages.items()]

def ready_stages(stages: Dict[str, List[str]], completed: Set[str]) -> List[str]:
    """Return names of stages whose dependencies are satisfied, sorted."""
    return sorted(
        name for name, deps in stages.items()
        if check(name, deps, completed)["ready"]
    )
