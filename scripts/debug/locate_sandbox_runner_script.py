SCRIPT = r'''
# -*- coding: utf-8 -*-
"""
Pure-stdlib diagnostic: locate sandbox_runner*.py modules and report
verify_command lines. Writes to devkit/runs/locate-sandbox-runner.log.

Constraints (L1, no carrier, no sandbox build/):
- No shell, no template variables.
- Stdlib only.
- Wrapped as a -c payload; no module-level side effects on import.
"""
import sys
from pathlib import Path

LOG_REL = Path("devkit") / "runs" / "locate-sandbox-runner.log"

def _write_log(lines):
    out = LOG_REL
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(lines) + ("\n" if lines else ""), encoding="utf-8")
    return out

def main():
    repo_root = Path(".").resolve()
    matches = sorted(
        (p for p in repo_root.rglob("sandbox_runner*.py") if p.is_file()),
        key=lambda p: str(p),
    )

    log_lines = []

    if not matches:
        log_lines.append("NO_MATCH")
        out_path = _write_log(log_lines)
        print(out_path.as_posix())
        return 0

    # (1) Write each match's path + mtime + size
    for p in matches:
        try:
            st = p.stat()
            mtime = st.st_mtime
            size = st.st_size
        except OSError as exc:
            log_lines.append(f"HIT {p.as_posix()}\tSTAT_ERROR={exc.__class__.__name__}")
            continue
        log_lines.append(f"HIT {p.as_posix()}\tmtime={mtime:.6f}\tsize={size}")

    # (3) For the first match, extract verify_command lines with line numbers
    first = matches[0]
    try:
        text = first.read_text(encoding="utf-8", errors="replace")
    except OSError as exc:
        log_lines.append(f"VCMD_READ_ERROR first={first.as_posix()} err={exc.__class__.__name__}")
    else:
        for idx, line in enumerate(text.splitlines(), start=1):
            # str.contains equivalent: simple substring test
            if "verify_command" in line:
                log_lines.append(f"VCMD first={first.as_posix()} line={idx}\t{line}")

    out_path = _write_log(log_lines)
    print(out_path.as_posix())
    return 0

if __name__ == "__main__":
    try:
        sys.exit(main())
    except SystemExit:
        raise
    except BaseException as exc:  # pragma: no cover — defensive, no carrier
        # L1 / report-only: never fail loudly during diagnostic
        try:
            err_path = LOG_REL
            err_path.parent.mkdir(parents=True, exist_ok=True)
            err_path.write_text(
                f"NO_MATCH\nUNEXPECTED={exc.__class__.__name__}: {exc}\n",
                encoding="utf-8",
            )
            print(err_path.as_posix())
        finally:
            sys.exit(0)  # acceptance is content-driven; keep exit 0
'''
