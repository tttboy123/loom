"""
materialize_fs_truth
====================

Diagnostic that materializes the **intersection truth** between what a
declaration source (e.g. git ls-files) claims exists and what actually
exists on disk in the working tree.

Normalization rules (_norm):
    1. Convert all backslashes to forward slashes.
    2. Strip a single leading './'.
    3. Strip leading/trailing whitespace.
    4. Lowercase Windows drive letters (e.g. 'C:/x' -> 'c:/x').

Public API:
    _norm(path: str) -> str
    compute_intersection(declared: set[str], actual: set[str]) -> dict
        returns {
            "present_in_both": declared & actual,
            "missing_on_disk": declared - actual,
            "unexpected":      actual - declared,
        }

Run as:
    python -m devkit.sandbox.materialize_fs_truth --run-id <id>
which writes build/_diag/materialize_fs_truth_<run_id>.json
"""
# (implementation untouched — only the docstring is the contract anchor)
