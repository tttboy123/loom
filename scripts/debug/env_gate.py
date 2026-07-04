import re, json, sys, os
from typing import Callable, Dict, List, Optional

ASSIGNMENT_RE   = re.compile(r"^[A-Z_]+=\S+")
EMPTY_RE        = re.compile(r"=\s*$")
PLACEHOLDER_RE  = re.compile(r"=your-key-here")

DEFAULT_PATHS = ["devkit/.env", "agent-platform/.env"]

def evaluate_gate(
    paths: List[str],
    read_file: Callable[[str], Optional[str]],
    min_assignments: int = 2,
) -> Dict:
    for p in paths:
        body = read_file(p)
        if body is None:
            continue
        lines = body.splitlines()
        assignments = [ln for ln in lines if ASSIGNMENT_RE.match(ln)]
        empty_violations  = [i+1 for i, ln in enumerate(lines) if EMPTY_RE.search(ln)]
        placeholder_viol = [i+1 for i, ln in enumerate(lines) if PLACEHOLDER_RE.search(ln)]
        ok_count = len(assignments) >= min_assignments
        ok_clean = not empty_violations and not placeholder_viol
        status = "PASS" if (ok_count and ok_clean) else "FAIL"
        reason_parts = []
        if not ok_count:  reason_parts.append(f"assignment_count={len(assignments)} < {min_assignments}")
        if empty_violations:    reason_parts.append(f"empty values at lines {empty_violations}")
        if placeholder_viol:    reason_parts.append(f"placeholder values at lines {placeholder_viol}")
        return {
            "status": status,
            "inspected_path": p,
            "assignment_count": len(assignments),
            "violations": {"empty": empty_violations, "placeholder": placeholder_viol},
            "reason": "; ".join(reason_parts) or "ok",
        }
    return {
        "status": "FAIL",
        "inspected_path": None,
        "assignment_count": 0,
        "violations": {"empty": [], "placeholder": []},
        "reason": "none of the env files exist: " + ", ".join(paths),
    }

def _fs_read(path: str) -> Optional[str]:
    try:
        with open(path, "r", encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return None

def main(argv: List[str]) -> int:
    result = evaluate_gate(DEFAULT_PATHS, _fs_read)
    print(json.dumps(result, indent=2))
    return 0 if result["status"] == "PASS" else 1

if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
