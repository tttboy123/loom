"""Archive run results into compressed storage.

Pure stdlib only. Moves run_dir into archive_dir/basename(run_dir)
as a gzip-compressed tarball, preserving the original directory name.
"""

from __future__ import annotations

import os
import shutil
import tarfile
import tempfile
from typing import Any

def archive_run(run_dir: str, archive_dir: str) -> dict[str, Any]:
    """Archive a run directory into a compressed tarball.

    Moves run_dir into archive_dir/basename(run_dir) as a .tar.gz file.
    Returns a dict describing the outcome.
    """
    result: dict[str, Any] = {
        "ok": False,
        "src": run_dir,
        "dst": "",
        "error": None,
    }

    if not os.path.isdir(run_dir):
        result["error"] = "not found"
        return result

    os.makedirs(archive_dir, exist_ok=True)

    base = os.path.basename(os.path.abspath(run_dir))
    dst_path = os.path.join(archive_dir, base + ".tar.gz")

    if os.path.exists(dst_path):
        result["dst"] = dst_path
        result["error"] = "already archived"
        return result

    # Build the tar.gz atomically: write to a temp file in archive_dir,
    # then move into place. This avoids leaving a half-written archive
    # if the process is interrupted mid-compression.
    fd, tmp_path = tempfile.mkstemp(
        prefix=".archive-", suffix=".tar.gz", dir=archive_dir
    )
    os.close(fd)

    try:
        with tarfile.open(tmp_path, "w:gz") as tar:
            # arcname preserves the basename so extraction recreates the
            # original directory name (e.g. "run-42/" not full path).
            parent = os.path.dirname(os.path.abspath(run_dir)) or "."
            tar.add(run_dir, arcname=base, recursive=True)

        shutil.move(tmp_path, dst_path)
        # Remove the original directory only after a successful archive.
        shutil.rmtree(run_dir)

        result["ok"] = True
        result["dst"] = dst_path
        return result
    except Exception as exc:  # noqa: BLE001 - report any failure to caller
        try:
            if os.path.exists(tmp_path):
                os.remove(tmp_path)
        except OSError:
            pass
        result["dst"] = dst_path
        result["error"] = f"archive failed: {exc!s}"
        return result

def list_archives(archive_dir: str) -> list[str]:
    """Return sorted list of archive names in archive_dir.

    Returns [] if archive_dir does not exist.
    """
    if not os.path.isdir(archive_dir):
        return []
    entries = os.listdir(archive_dir)
    return sorted(entries)

def archive_summary(archive_dir: str) -> dict[str, Any]:
    """Return {count, names} for archive_dir.

    count = len(names). Both are 0/[] when archive_dir is missing.
    """
    names = list_archives(archive_dir)
    return {"count": len(names), "names": names}
