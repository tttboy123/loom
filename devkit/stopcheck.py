# stopcheck.py
"""死循环检测：同一错误签名在列表末尾连续重复 max_repeats 次即应停。"""

def should_stop(signatures, max_repeats=2):
    """判定自治体是否因同一错误反复出现而应停下挂起。

    参数:
        signatures: 字符串列表，每轮一个签名；非空串=该轮错误，空串=该轮通过。
        max_repeats: 触发停止所需的末尾连续相同非空签名数，默认 2。

    返回:
        dict: {"stop": bool, "reason": str}
    """
    if max_repeats <= 0:
        return {"stop": False, "reason": "ok"}

    if not signatures:
        return {"stop": False, "reason": "ok"}

    tail = signatures[-max_repeats:]

    # 末尾不足 max_repeats 个，不可能构成"连续重复 max_repeats 次"
    if len(tail) < max_repeats:
        return {"stop": False, "reason": "ok"}

    first = tail[0]

    # 空串代表通过，不算错误，不触发停止
    if first == "":
        return {"stop": False, "reason": "ok"}

    if all(sig == first for sig in tail):
        return {"stop": True, "reason": "same error repeated"}

    return {"stop": False, "reason": "ok"}
