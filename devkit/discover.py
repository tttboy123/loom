# discover.py
"""从历史数据发现下一个该建的候选任务（纯分析层，无 LLM 调用）。"""


def from_fitness(fitness_rows):
    """从 model_fitness 行找出能力缺口候选，按 ok_rate 升序排列。"""
    best = {}  # (type, task_type, backend) -> lowest ok_rate row
    coverage = {}  # task_type -> seen?

    for row in fitness_rows:
        t = row.get("task_type", "")
        b = row.get("backend", "")
        r = row.get("ok_rate", 100)
        runs = row.get("runs", 0)

        if runs == 0:
            if t not in coverage:
                coverage[t] = True
        elif r < 50 and runs >= 2:
            key = ("improve_carrier", t, b)
            if key not in best or r < best[key]["ok_rate"]:
                best[key] = {"type": "improve_carrier", "task_type": t, "backend": b, "ok_rate": r}

    result = list(best.values())
    for t in coverage:
        key = ("add_coverage", t, "")
        if not any(c.get("type") == "add_coverage" and c.get("task_type") == t for c in result):
            result.append({"type": "add_coverage", "task_type": t})

    result.sort(key=lambda c: c.get("ok_rate", 100))
    return result


def from_suggestions(suggestions):
    """从 learn.analyze() 的 suggestions 提取可行任务候选（仅 high/medium）。"""
    _MAP = {"carrier": "switch_carrier", "golden": "fix_golden", "quota": "manage_quota"}
    result = []
    for s in suggestions:
        p = s.get("priority", "low")
        if p not in ("high", "medium"):
            continue
        stype = s.get("type", "")
        mapped = _MAP.get(stype, stype)
        result.append({"type": mapped, "detail": s.get("detail", ""), "priority": p})
    return result


def merge(candidates, max_total=10):
    """多个候选来源 round-robin 交织、去重，返回至多 max_total 项。"""
    seen = set()
    result = []
    iters = [iter(lst) for lst in candidates]
    while iters and len(result) < max_total:
        next_iters = []
        for it in iters:
            if len(result) >= max_total:
                break
            try:
                item = next(it)
                key = (item.get("type", ""), item.get("task_type", ""), item.get("backend", ""))
                if key not in seen:
                    seen.add(key)
                    result.append(item)
                next_iters.append(it)
            except StopIteration:
                pass
        iters = next_iters
    return result
