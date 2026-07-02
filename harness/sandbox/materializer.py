"""
Devkit sandbox materializer.

Bug fixed: include glob was `devkit/test_*.py`, which excluded the
runtime support modules (preflight.py, ponytail.py, wallet.py, retry.py).
Now uses `devkit/*.py` so every top-level Python source under devkit/ is
materialized into build/<run_id>/devkit/.
"""
from __future__ import annotations

import fnmatch
import os
import shutil
from pathlib import Path
from typing import Iterable, Tuple

# Was: ("devkit/test_*.py",)
DEFAULT_INCLUDE_GLOBS: Tuple[str, ...] = ("devkit/*.py",)

def is_included_by_globs(
    path: str,
    include_globs: Iterable[str] = DEFAULT_INCLUDE_GLOBS,
) -> bool:
    """
    Return True iff `path` matches any glob in `include_globs`.

    Globs are applied as fnmatch patterns against the POSIX-normalized
    relative path. They are intentionally NOT recursive: `devkit/*.py`
    matches `devkit/preflight.py` but not `devkit/sub/nested.py`.
    Callers needing recursion must add `devkit/**/*.py`.
    """
    norm = path.replace(os.sep, "/")
    return any(fnmatch.fnmatch(norm, g) for g in include_globs)

def materialize(
    src_root: str,
    build_root: str,
    run_id: str,
    include_globs: Iterable[str] = DEFAULT_INCLUDE_GLOBS,
) -> list[str]:
    """
    Copy files under src_root that match include_globs into
    build_root/<run_id>/, preserving relative paths. Returns the list
    of materialized relative paths.
    """
    src = Path(src_root)
    dst = Path(build_root) / run_id
    dst.mkdir(parents=True, exist_ok=True)

    materialized: list[str] = []
    for root, _dirs, files in os.walk(src):
        for name in files:
            abs_path = Path(root) / name
            rel_to_src = abs_path.relative_to(src).as_posix()
            if not is_included_by_globs(rel_to_src, include_globs):
                continue
            out = dst / rel_to_src
            out.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(abs_path, out)
            materialized.append(rel_to_src)
    return materialized
