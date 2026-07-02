# devkit/result_merger.py
# Pure stdlib. Merge results from multiple runs.

from typing import Dict, List, Any

def merge(results: list) -> dict:
    """Merge multiple run results into a single summary dict.

    Args:
        results: list of dicts with keys 'gate' (str), 'tokens' (int),
                 'ok' (bool), 'notes' (str).

    Returns:
        dict with keys 'gate', 'total_tokens', 'ok_count', 'consensus', 'notes'.
    """
    notes_list: List[str] = []
    total_tokens: int = 0
    ok_count: int = 0
    ok_values: List[bool] = []

    for r in results:
        total_tokens += int(r.get("tokens", 0))
        if r.get("ok", False):
            ok_count += 1
        ok_values.append(bool(r.get("ok", False)))
        n = r.get("notes", "")
        if n:
            notes_list.append(n)

    # Gate decision: majority 'GO' -> 'GO', else 'NO-GO'; empty -> 'UNKNOWN'
    if not results:
        gate = "UNKNOWN"
    else:
        go_count = sum(1 for r in results if r.get("gate") == "GO")
        gate = "GO" if go_count > len(results) / 2 else "NO-GO"

    # Consensus: all ok values identical
    consensus = len(ok_values) > 0 and all(v == ok_values[0] for v in ok_values)

    # Deduplicated, sorted notes
    merged_notes = sorted(set(notes_list))

    return {
        "gate": gate,
        "total_tokens": total_tokens,
        "ok_count": ok_count,
        "consensus": consensus,
        "notes": merged_notes,
    }

def merge_notes(results: list) -> list:
    """Return all non-empty notes from results, deduplicated and sorted."""
    notes_set = set()
    for r in results:
        n = r.get("notes", "")
        if n:
            notes_set.add(n)
    return sorted(notes_set)

def merge_summary(merged: dict) -> str:
    """Return a one-line summary string from a merged result dict."""
    return (
        f"gate={merged.get('gate', 'UNKNOWN')} "
        f"tokens={merged.get('total_tokens', 0)} "
        f"ok={merged.get('ok_count', 0)}"
    )
