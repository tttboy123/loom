# carrier_bench.py
"""对多个 carriers 跑标准 benchmark，量化 ok_rate/latency/cost，结果缓存供 carrier_router 参考。"""
from __future__ import annotations

import json
import pathlib
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone

_ROOT = pathlib.Path(__file__).parent
_DEFAULT_TASKS = _ROOT / "bench_tasks.json"
_DEFAULT_OUT = _ROOT / "carrier_bench.json"

_BUILTIN_TASKS = [
    {"id": "plan-simple", "stage": "plan",
     "prompt": "列出实现一个命令行 todo 应用的 3 个核心模块，每项一行。"},
    {"id": "implement-simple", "stage": "implement",
     "prompt": "用 Python 写一个函数 add(a, b) 返回两数之和，附带一行 docstring。"},
    {"id": "verify-simple", "stage": "verify",
     "prompt": "以下代码有无 bug？def div(a,b): return a/b  请简短回答。"},
]


def load_tasks(tasks_path: "str | None" = None) -> list:
    """从 bench_tasks.json 读取任务列表，文件不存在时返回内置默认任务。"""
    path = pathlib.Path(tasks_path) if tasks_path else _DEFAULT_TASKS
    if path.exists():
        try:
            return json.loads(path.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            pass
    return list(_BUILTIN_TASKS)


def _probe_request(carrier: str, prompt: str, base_url: str, api_key: str,
                   timeout: int) -> dict:
    """向 LiteLLM 发送单条请求，返回 {ok, latency_ms, cost}。"""
    payload = json.dumps({
        "model": carrier,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": 50,
    }).encode("utf-8")
    url = base_url.rstrip("/") + "/chat/completions"
    req = urllib.request.Request(
        url, data=payload,
        headers={"Content-Type": "application/json",
                 "Authorization": f"Bearer {api_key}"},
        method="POST",
    )
    t0 = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            latency_ms = (time.monotonic() - t0) * 1000
            d = json.loads(resp.read())
            cost_hdr = resp.headers.get("x-litellm-response-cost")
            cost = float(cost_hdr) if cost_hdr else 0.0
            return {"ok": True, "latency_ms": round(latency_ms, 1), "cost": cost}
    except Exception:  # noqa: BLE001
        return {"ok": False, "latency_ms": 0.0, "cost": 0.0}


def run_bench(
    carriers: list,
    base_url: str,
    api_key: str,
    tasks: "list | None" = None,
    timeout: int = 60,
) -> dict:
    """对每个 carrier 跑所有 benchmark 任务，返回按 carrier → stage → 统计的嵌套 dict。"""
    if not carriers:
        return {}
    if tasks is None:
        tasks = load_tasks()

    results: dict = {}
    for carrier in carriers:
        stage_buckets: dict[str, list] = {}
        for t in tasks:
            stage = t.get("stage", "unknown")
            prompt = t.get("prompt", "hi")
            r = _probe_request(carrier, prompt, base_url, api_key, timeout)
            stage_buckets.setdefault(stage, []).append(r)

        carrier_result: dict = {}
        for stage, rows in stage_buckets.items():
            ok_count = sum(1 for r in rows if r["ok"])
            carrier_result[stage] = {
                "ok_rate": ok_count / len(rows) if rows else 0.0,
                "avg_latency_ms": (sum(r["latency_ms"] for r in rows) / len(rows)
                                   if rows else 0.0),
                "avg_cost": (sum(r["cost"] for r in rows) / len(rows) if rows else 0.0),
                "runs": len(rows),
            }
        results[carrier] = carrier_result
    return results


def save_results(results: dict, out_path: "str | None" = None) -> None:
    """将结果带时间戳写入缓存文件。"""
    path = pathlib.Path(out_path) if out_path else _DEFAULT_OUT
    ts = datetime.now(tz=timezone.utc).isoformat()
    path.write_text(
        json.dumps({"ts": ts, "results": results}, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )


def load_results(out_path: "str | None" = None) -> dict:
    """读取上次 bench 结果，文件不存在返回 {}。"""
    path = pathlib.Path(out_path) if out_path else _DEFAULT_OUT
    if not path.exists():
        return {}
    try:
        d = json.loads(path.read_text(encoding="utf-8"))
        return d.get("results", d)
    except (json.JSONDecodeError, OSError):
        return {}


def bench_to_history_rows(results: dict) -> list:
    """将 carrier_bench.json 结果转成 carrier_router 可消费的 history_rows 格式。

    输入: {carrier: {stage: {ok_rate, avg_latency_ms, avg_cost, runs}}}
    输出: [{carrier, stage, ok_rate, avg_cost, runs}, ...]
    """
    rows = []
    for carrier, stage_data in results.items():
        for stage, sd in stage_data.items():
            rows.append({
                "carrier": carrier,
                "stage": stage,
                "ok_rate": sd.get("ok_rate", 0.5),
                "avg_cost": sd.get("avg_cost", 0.0),
                "runs": sd.get("runs", 0),
            })
    return rows


def print_table(results: dict) -> None:
    """打印各 carrier 在各 stage 上的 ok_rate/latency/cost 对比表。"""
    if not results:
        return
    stages = sorted({s for v in results.values() for s in v})
    header = f"{'carrier':<20}" + "".join(f"  {s:<28}" for s in stages)
    print(header)
    print("─" * len(header))
    for carrier, stage_data in sorted(results.items()):
        row = f"{carrier:<20}"
        for stage in stages:
            sd = stage_data.get(stage)
            if sd:
                cell = f"{sd['ok_rate']*100:.0f}%/{sd['avg_latency_ms']/1000:.1f}s/${sd['avg_cost']:.4f}"
            else:
                cell = "—"
            row += f"  {cell:<28}"
        print(row)
