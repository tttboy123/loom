"""
run_planner.py — Pure stdlib planner for backlog tasks.

Provides dependency-aware batching and duration estimation for a backlog of
tasks. No external dependencies required.
"""

from typing import Dict, List

def plan(backlog: list, max_parallel: int = 1) -> list:
    """
    Batch tasks based on dependency relationships.

    Within each batch, tasks are independent of each other (no intra-batch
    dependency edges). Batches themselves run sequentially: a task in batch
    N+1 only appears once all of its dependencies in earlier batches have
    been scheduled.

    Args:
        backlog: List of task dicts. Each task must have at least:
                  - 'id':  unique task identifier (str)
                  - 'deps': list of task ids this task depends on
        max_parallel: Maximum number of tasks allowed within a single batch.
                      Defaults to 1 (strictly serial). If a batch would
                      exceed this size, surplus ready tasks are deferred
                      to subsequent batches.

    Returns:
        List of batches, where each batch is a list of task ids, e.g.
            [[batch1_ids], [batch2_ids], ...]
        Empty backlog -> [].
    """
    if not backlog:
        return []

    # Index tasks by id for quick lookup and validate inputs.
    tasks: Dict[str, dict] = {}
    for t in backlog:
        tid = t.get("id")
        if tid is None:
            raise ValueError("backlog task missing 'id'")
        if tid in tasks:
            raise ValueError(f"duplicate task id: {tid!r}")
        # Normalise deps to a list; tolerate missing field.
        deps = t.get("deps", []) or []
        if not isinstance(deps, list):
            raise ValueError(f"task {tid!r}: 'deps' must be a list")
        tasks[tid] = {"id": tid, "deps": deps}

    # Validate that deps reference known tasks (and detect unknown refs).
    for tid, t in tasks.items():
        for d in t["deps"]:
            if d not in tasks:
                raise ValueError(
                    f"task {tid!r} depends on unknown task {d!r}"
                )

    # Detect cycles via DFS colouring (0=white,1=gray,2=black).
    WHITE, GRAY, BLACK = 0, 1, 2
    color: Dict[str, int] = {tid: WHITE for tid in tasks}

    def visit(node: str, stack: List[str]) -> None:
        c = color[node]
        if c == GRAY:
            cycle = " -> ".join(stack + [node])
            raise ValueError(f"dependency cycle detected: {cycle}")
        if c == BLACK:
            return
        color[node] = GRAY
        for d in tasks[node]["deps"]:
            visit(d, stack + [node])
        color[node] = BLACK

    for tid in tasks:
        if color[tid] == WHITE:
            visit(tid, [])

    # Kahn-style topological batching: repeatedly emit all currently-ready
    # tasks (respecting max_parallel), then unlock their dependents.
    remaining = set(tasks.keys())
    completed: set = set()
    batches: List[List[str]] = []
    total = len(tasks)

    while len(completed) < total:
        # Ready = no remaining unmet dependencies.
        ready = sorted(
            tid for tid in remaining
            if all(d in completed for d in tasks[tid]["deps"])
        )
        if not ready:
            # Should not happen because we ruled out cycles, but be safe.
            raise ValueError("planner stuck: no ready tasks but work remains")

        # Honour max_parallel: only take up to max_parallel from the front
        # of the ready list each batch; defer the rest.
        take = ready[:max_parallel] if max_parallel > 0 else ready
        batches.append(list(take))

        for tid in take:
            remaining.remove(tid)
            completed.add(tid)

    return batches

def estimate_duration(
    plan: list,
    task_map: dict,
    secs_per_task: float = 60.0,
) -> float:
    """
    Estimate total wall-clock duration for a planned schedule.

    Simplified model: batches run sequentially, tasks within a batch run
    in parallel. Total time = len(batches) * secs_per_task.

    Args:
        plan: The schedule returned by plan(), i.e. list[list[str]].
        task_map: {id: task_dict}. Unused in the simplified model but
                  accepted for API symmetry / future use.
        secs_per_task: Per-batch cost. Defaults to 60.0.

    Returns:
        Estimated duration in seconds (float). Empty plan -> 0.0.
    """
    return float(len(plan)) * float(secs_per_task)

def plan_summary(plan: list) -> str:
    """
    Produce a one-line summary of a plan.

    Args:
        plan: The schedule returned by plan(), i.e. list[list[str]].

    Returns:
        Formatted string: '{n} batches, {total} tasks'.
            - n: number of batches
            - total: total number of tasks across all batches
        Empty plan -> '0 batches, 0 tasks'.
    """
    n_batches = len(plan)
    total_tasks = sum(len(b) for b in plan)
    return f"{n_batches} batches, {total_tasks} tasks"
