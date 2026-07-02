# preflight.py
"""在任务开始前预估 token 用量，检查 LiteLLM 余额是否足够。"""
from __future__ import annotations

import json
import os
import pathlib
import urllib.request
from typing import Optional

_FALLBACK = {"plan": 1000, "implement": 3000, "verify": 1500, "review": 1500, "brainstorm": 800}
_RUNS_DIR = pathlib.Path(__file__).parent / "runs"


def _avg_tokens_by_stage(runs_dir: pathlib.Path) -> dict[str, float]:
    """从 runs/ 扫描历史 artifact JSON，计算每 stage 的平均 token 数。"""
    totals: dict[str, list[int]] = {}
    if not runs_dir.is_dir():
        return {}
    for run_dir in runs_dir.iterdir():
        if not run_dir.is_dir():
            continue
        for art_file in run_dir.glob("*.artifact.json"):
            try:
                art = json.loads(art_file.read_text(encoding="utf-8"))
                stage = art.get("stage", "")
                toks = art.get("tokens")
                if stage and toks and isinstance(toks, int) and toks > 0:
                    totals.setdefault(stage, []).append(toks)
            except Exception:  # noqa: BLE001
                pass
    return {s: sum(v) / len(v) for s, v in totals.items() if v}


def _fetch_balance(base_url: str, api_key: str) -> Optional[float]:
    """从 LiteLLM /spend/logs 读余额，失败返回 None。"""
    try:
        url = base_url.rstrip("/") + "/spend/logs"
        req = urllib.request.Request(url, headers={"Authorization": f"Bearer {api_key}"})
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            return float(data.get("balance", data.get("remaining_budget", 0.0)))
    except Exception:  # noqa: BLE001
        return None


def estimate(task: str, stages: list[str], carrier_map: dict) -> dict:
    """预估一次 run 的 token 用量 + 检查 LiteLLM 余额。

    返回 {ok: bool, estimated_tokens: int, balance: float | None, warning: str}
    """
    history = _avg_tokens_by_stage(_RUNS_DIR)
    total_tokens = 0
    for stage in stages:
        avg = history.get(stage) or _FALLBACK.get(stage, 1000)
        total_tokens += int(avg)

    base_url = os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
    api_key = os.environ.get("LITELLM_MASTER_KEY", "")
    balance = _fetch_balance(base_url, api_key) if api_key else None

    warning = ""
    ok = True
    if not history:
        warning = "历史数据不足，token 预估基于兜底值"
    if balance is not None:
        cost_est = total_tokens * 0.000002  # rough USD estimate at $2/1M tokens
        if cost_est > balance:
            ok = False
            warning = f"预估花费 ${cost_est:.5f} 可能超出余额 ${balance:.5f}"

    return {"ok": ok, "estimated_tokens": total_tokens, "balance": balance, "warning": warning}
