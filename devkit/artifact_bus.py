# artifact_bus.py
"""artifact_bus.py —— 结构化 Artifact 交接总线。

把各阶段产出物（Plan/Patch/VerifyReport）序列化为 Bus 消息，
使阶段间用结构化 JSON 交接，避免 markdown 字符串拼接造成结构丢失。
"""
import hashlib

def _digest(text: str) -> str:
    """对 text UTF-8 编码后 SHA-256，取前 8 个 hex 字符。"""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:8]

def to_bus_message(art: dict) -> dict:
    """把 artifact.make() 的产物字典序列化成 Bus 消息格式。

    字段：
      type / stage / role / title：直通；
      carrier / task_type：缺失时默认 ""；
      tokens / cost：None/0/0.0 统一归零；
      verdict / tests_passed：保留原值（含 None）；
      body_digest：body 的 SHA-256 前 8 hex。
    """
    return {
        "type": "artifact",
        "stage": art["stage"],
        "role": art["role"],
        "title": art["title"],
        "carrier": art.get("carrier", ""),
        "task_type": art.get("task_type", ""),
        "tokens": art.get("tokens") or 0,
        "cost": art.get("cost") or 0.0,
        "verdict": art.get("verdict"),
        "tests_passed": art.get("tests_passed"),
        "body_digest": _digest(art.get("body", "")),
    }

def from_bus_message(msg: dict) -> dict:
    """把 Bus 消息还原为 artifact 字典（body 默认 ""）。

    body 不在 bus 消息里，需调用方自行补回。
    """
    return {
        "stage": msg["stage"],
        "role": msg["role"],
        "title": msg["title"],
        "carrier": msg.get("carrier", ""),
        "task_type": msg.get("task_type", ""),
        "tokens": msg.get("tokens") or 0,
        "cost": msg.get("cost") or 0.0,
        "verdict": msg.get("verdict"),
        "tests_passed": msg.get("tests_passed"),
        "body": "",
    }

def merge(arts: list[dict]) -> dict:
    """把多个阶段的 artifact 合并成一个汇总 dict。

    返回：
      stages: list[str]，按入参顺序保留；
      total_tokens: int，tokens 之和（None/缺失按 0 计）；
      total_cost: float，cost 之和（None/缺失按 0.0 计）；
      go_count / nogo_count: 仅统计 verdict == "GO" / "NO-GO" 的项；
      verdicts: {stage: verdict}，只收录 GO/NO-GO，过滤 None。
    """
    stages = [a["stage"] for a in arts]                         # 关键修复：list 而非 set
    total_tokens = sum((a.get("tokens") or 0) for a in arts)
    total_cost = sum((a.get("cost") or 0.0) for a in arts)
    go_count = sum(1 for a in arts if a.get("verdict") == "GO")
    nogo_count = sum(1 for a in arts if a.get("verdict") == "NO-GO")
    verdicts = {
        a["stage"]: a.get("verdict")
        for a in arts
        if a.get("verdict") in ("GO", "NO-GO")
    }
    return {
        "stages": stages,
        "total_tokens": total_tokens,
        "total_cost": total_cost,
        "go_count": go_count,
        "nogo_count": nogo_count,
        "verdicts": verdicts,
    }
