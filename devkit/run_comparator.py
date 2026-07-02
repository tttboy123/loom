"""Compare runs by gate / iterations / tokens. Pure stdlib."""

def compare(runs: list[dict]) -> list[dict]:
    """Return a NEW sorted list. Sort key:
       1) gate == 'GO'  (True ranks first via bool→int)
       2) iterations    (asc)
       3) tokens        (asc)
    Original input is not mutated (sorted() returns a new list; Python sort is stable).
    """
    return sorted(
        runs,
        key=lambda r: (
            r.get('gate') != 'GO',
            r.get('iterations', 0),
            r.get('tokens', 0),
        ),
    )

def winner(runs: list[dict]) -> dict | None:
    """Return the best run per compare()'s ordering, or None if runs is empty."""
    if not runs:
        return None
    return compare(runs)[0]

def compare_summary(runs: list[dict]) -> str:
    """One-line summary. Empty list -> '(no runs)'.
    Otherwise: 'N runs: best={run_id} gate={gate} iter={iterations}'.
    """
    if not runs:
        return '(no runs)'
    w = winner(runs)
    return f"{len(runs)} runs: best={w['run_id']} gate={w['gate']} iter={w['iterations']}"
