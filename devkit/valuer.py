# valuer.py
"""候选任务价值评分器（纯标准库，无外部依赖）。

公式：base=50，再按 ok_rate / priority / runs 三组信号加减分，
最后 clamp 到 [0,100]；rank 稳定降序；top_n 取前 n。
"""

def _ok_rate_delta(ok_rate):
    """根据 ok_rate 返回 (delta, 描述串)。"""
    if ok_rate is None:
        return 0, ""
    if ok_rate < 30:
        return 25, f"ok_rate={ok_rate}(<30) +25"
    if ok_rate < 50:
        return 15, f"ok_rate={ok_rate}(<50) +15"
    if ok_rate < 70:
        return 5, f"ok_rate={ok_rate}(<70) +5"
    return -10, f"ok_rate={ok_rate}(>=70) -10"

def _priority_delta(priority):
    """根据 priority 返回 (delta, 描述串)。"""
    if priority == "high":
        return 20, "priority=high +20"
    if priority == "medium":
        return 5, "priority=medium +5"
    if priority == "low":
        return -10, "priority=low -10"
    return 0, ""

def _runs_delta(runs):
    """根据 runs 返回 (delta, 描述串)。"""
    if runs is None:
        return 0, ""
    if runs >= 5:
        return 10, f"runs={runs}(>=5) +10"
    if runs == 0:
        return -15, "runs=0 -15"
    return 0, ""

def score(candidate, evidence=None):
    """给单个候选打分，返回 {score:int, reason:str, candidate:dict}。

    priority 字段优先取 evidence，其次 fallback 到 candidate；
    evidence 取不到信号则该项不加减分。
    """
    cand = candidate if candidate else {}
    ev = evidence if evidence else {}

    parts = []
    total = 50

    d, desc = _ok_rate_delta(ev.get("ok_rate"))
    total += d
    if desc:
        parts.append(desc)

    priority = ev.get("priority")
    if priority is None:
        priority = cand.get("priority")
    d, desc = _priority_delta(priority)
    total += d
    if desc:
        parts.append(desc)

    d, desc = _runs_delta(ev.get("runs"))
    total += d
    if desc:
        parts.append(desc)

    clamped = max(0, min(100, total))
    if parts:
        reason = "base=50; " + "; ".join(parts) + f" -> {clamped}"
    else:
        reason = f"base=50 -> {clamped}"
    return {"score": clamped, "reason": reason, "candidate": cand}

def rank(candidates, evidences):
    """批量评分后按 score 降序排列（稳定排序，分数相同保持原顺序）。

    candidates 与 evidences 长度不一致时，短的一方按需补 {} 对齐。
    """
    n = max(len(candidates), len(evidences))
    scored = []
    for i in range(n):
        cand = candidates[i] if i < len(candidates) else {}
        ev = evidences[i] if i < len(evidences) else {}
        scored.append(score(cand, ev))
    scored.sort(key=lambda r: r["score"], reverse=True)
    return scored

def top_n(candidates, evidences, n=3):
    """返回评分最高的前 n 个候选；候选不足时返回全部；n<=0 返回 []。"""
    ranked = rank(candidates, evidences)
    if n <= 0:
        return []
    return ranked[:n]
