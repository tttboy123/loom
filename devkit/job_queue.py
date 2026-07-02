# job_queue.py
def create() -> dict:
    return {"jobs": [], "processed": 0}

def enqueue(queue: dict, job: dict, priority: int = 0) -> dict:
    new_queue = {
        "jobs": list(queue["jobs"]),
        "processed": queue["processed"]
    }
    # 插入 job，附带 priority
    new_queue["jobs"].append({"priority": priority, **job})
    # 按 priority 降序排列（高优先级在前）
    new_queue["jobs"].sort(key=lambda j: j["priority"], reverse=True)
    return new_queue

def dequeue(queue: dict) -> tuple:
    new_queue = {
        "jobs": list(queue["jobs"]),
        "processed": queue["processed"]
    }
    if not new_queue["jobs"]:
        return (None, new_queue)
    job = new_queue["jobs"].pop(0)
    # 从 job dict 中移除 priority 字段
    job_clean = {k: v for k, v in job.items() if k != "priority"}
    new_queue["processed"] += 1
    return (job_clean, new_queue)

def queue_summary(queue: dict) -> dict:
    pending = len(queue["jobs"])
    next_id = None
    if pending > 0:
        next_id = queue["jobs"][0].get("id")
    return {"pending": pending, "processed": queue["processed"], "next_id": next_id}
