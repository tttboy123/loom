# priority_queue.py
import heapq

def create(mode: str = "min") -> dict:
    if mode not in ("min", "max"):
        raise ValueError("mode must be 'min' or 'max'")
    return {"mode": mode, "heap": [], "size": 0, "_negate": mode == "max"}

def push(pq: dict, item, priority: float) -> dict:
    new_pq = pq.copy()
    new_pq["heap"] = list(pq["heap"])
    if pq.get("_negate", False):
        heapq.heappush(new_pq["heap"], (-priority, item))
    else:
        heapq.heappush(new_pq["heap"], (priority, item))
    new_pq["size"] = pq["size"] + 1
    return new_pq

def pop(pq: dict) -> tuple:
    if pq["size"] == 0:
        return (None, dict(pq))
    new_pq = pq.copy()
    new_pq["heap"] = list(pq["heap"])
    _, item = heapq.heappop(new_pq["heap"])
    new_pq["size"] = pq["size"] - 1
    return (item, new_pq)

def peek(pq: dict):
    if pq["size"] == 0:
        return None
    # peek without popping
    return pq["heap"][0][1]

def pq_summary(pq: dict) -> dict:
    top = peek(pq)
    return {"mode": pq["mode"], "size": pq["size"], "top": top}
