# devkit/learn.py — Learning Sidecar (read-only analysis)
from __future__ import annotations

import pathlib
import re
import statistics
from collections import Counter
from typing import Any, Optional

_DEFAULT_RUNS_DIR = pathlib.Path(__file__).resolve().parent.parent / "devkit" / "runs"

Suggestion = dict[str, Any]


def _confidence(n: int) -> float:
    if n >= 10: return 0.9
    if n >= 5:  return 0.7
    if n >= 2:  return 0.5
    return 0.3


def _load_runs(runs_dir: Optional[pathlib.Path]) -> list[dict]:
    from devkit import insight
    d = runs_dir or _DEFAULT_RUNS_DIR
    try:
        return insight.runs_list(d)
    except Exception:
        return []


def _load_fitness(runs_dir: Optional[pathlib.Path]) -> list[dict]:
    from devkit import insight
    d = runs_dir or _DEFAULT_RUNS_DIR
    try:
        return insight.model_fitness(d).get("rows", [])
    except Exception:
        return []


def _is_go(gate: str | None) -> bool:
    if gate is None:
        return False
    g = gate.upper()
    return "GO" in g and "NO-GO" not in g


def _has_safety_nogo(run_dir_path: pathlib.Path) -> bool:
    try:
        log = (run_dir_path / "run-log.md").read_text(encoding="utf-8")
        return "Safety NO-GO" in log or "🚫 Safety NO-GO" in log
    except Exception:
        return False


def analyze(runs_dir: Optional[pathlib.Path] = None) -> dict:
    suggestions: list[Suggestion] = []

    runs = _load_runs(runs_dir)
    base = runs_dir or _DEFAULT_RUNS_DIR

    # --- summary ---
    costs = [r["cost"] for r in runs if r.get("cost") is not None]
    total_runs = len(runs)
    go_count = sum(1 for r in runs if _is_go(r.get("gate")))
    go_rate = go_count / total_runs if total_runs else 0.0
    task_types = [r["task_type"] for r in runs if r.get("task_type")]
    top_task = Counter(task_types).most_common(1)
    summary = {
        "total_runs": total_runs,
        "go_rate": round(go_rate, 4),
        "top_task_type": top_task[0][0] if top_task else None,
        "avg_cost_usd": round(sum(costs) / len(costs), 6) if costs else 0.0,
        "total_cost_usd": round(sum(costs), 6),
    }

    # --- 1. carrier suggestions from model_fitness rows ---
    rows = _load_fitness(runs_dir)
    by_type: dict[str, list[dict]] = {}
    for row in rows:
        tt = row.get("task_type")
        if tt:
            by_type.setdefault(tt, []).append(row)

    for tt, type_rows in by_type.items():
        if len(type_rows) < 2:
            continue
        type_rows_sorted = sorted(type_rows, key=lambda r: r.get("ok_rate") or 0, reverse=True)
        best = type_rows_sorted[0]
        worst = type_rows_sorted[-1]
        best_rate = best.get("ok_rate") or 0.0
        worst_rate = worst.get("ok_rate") or 0.0
        if best_rate - worst_rate >= 20:  # ok_rate is 0–100
            n = (best.get("uses") or 0) + (worst.get("uses") or 0)
            suggestions.append({
                "type": "carrier",
                "confidence": _confidence(n),
                "reason": (f"{tt}: {best['backend']} 成功率 {best_rate:.0f}%，"
                           f"{worst['backend']} 仅 {worst_rate:.0f}%，差距 {best_rate-worst_rate:.0f}pp"),
                "action": f"将 {tt} 类任务固定到 {best['backend']}，停用 {worst['backend']}",
                "data": {
                    "task_type": tt,
                    "best_backend": best["backend"],
                    "worst_backend": worst["backend"],
                    "best_ok_rate": best_rate,
                    "worst_ok_rate": worst_rate,
                    "count": n,
                },
            })

    # --- 2. quota trend warning ---
    if len(costs) >= 10:
        recent5 = costs[-5:]
        all20 = costs[-20:]
        avg_recent = statistics.mean(recent5)
        avg_all = statistics.mean(all20)
        if avg_recent > avg_all * 1.5:
            suggestions.append({
                "type": "quota",
                "confidence": _confidence(len(recent5)),
                "reason": f"最近 5 次均成本 ${avg_recent:.5f}，是近 20 次均值 ${avg_all:.5f} 的 {avg_recent/avg_all:.1f}x",
                "action": "检查是否有昂贵 carrier 被意外使用，或考虑加 --budget 上限",
                "data": {
                    "recent_5_avg": round(avg_recent, 6),
                    "rolling_20_avg": round(avg_all, 6),
                    "ratio": round(avg_recent / avg_all, 2),
                },
            })

    # --- 3. safety hotspot ---
    if base.is_dir():
        safety_hits = []
        for r in runs:
            run_dir_path = base / r["run_id"]
            if _has_safety_nogo(run_dir_path):
                safety_hits.append(r)
        if safety_hits:
            top_viol = Counter(r.get("task_type") for r in safety_hits if r.get("task_type")).most_common(1)
            suggestions.append({
                "type": "safety",
                "confidence": _confidence(len(safety_hits)),
                "reason": f"发现 {len(safety_hits)} 次 Safety NO-GO，主要在 {top_viol[0][0] if top_viol else '未知'} 类任务",
                "action": "审查相关 prompt，对高风险任务加 --safety 扫描",
                "data": {
                    "violation_count": len(safety_hits),
                    "top_task_type": top_viol[0][0] if top_viol else None,
                },
            })

    return {"suggestions": suggestions, "summary": summary}


def suggest_carrier(task_type: str, runs_dir: Optional[pathlib.Path] = None) -> Suggestion | None:
    rows = _load_fitness(runs_dir)
    type_rows = [r for r in rows if r.get("task_type") == task_type]
    if not type_rows:
        return None
    type_rows_sorted = sorted(type_rows, key=lambda r: r.get("ok_rate") or 0, reverse=True)
    best = type_rows_sorted[0]
    best_rate = best.get("ok_rate") or 0.0
    n = best.get("uses") or 0
    if len(type_rows) == 1 and best_rate >= 50:  # ok_rate is 0–100
        return None
    if len(type_rows) >= 2:
        worst = type_rows_sorted[-1]
        worst_rate = worst.get("ok_rate") or 0.0
        if best_rate - worst_rate < 20:  # ok_rate is 0–100
            return None
    return {
        "type": "carrier",
        "confidence": _confidence(n),
        "reason": f"{task_type} 最优 carrier: {best['backend']}（成功率 {best_rate:.0f}%，{n} 次样本）",
        "action": f"--carrier implement={best['backend']}",
        "data": {
            "task_type": task_type,
            "best_backend": best["backend"],
            "ok_rate": best_rate,
            "uses": n,
        },
    }


_RE_GOLDEN_FAIL = re.compile(
    r"^\|\s*(?P<name>\S+)\s*\|\s*❌\s*\|\s*(?P<detail>[^\|]+)", re.MULTILINE
)


def suggest_goldens(runs_dir: Optional[pathlib.Path] = None) -> list[Suggestion]:
    """
    Analyze NO-GO runs' golden failures and return actionable suggestions.

    Returns a list of Suggestion dicts (type="golden") grouping recurring
    failure patterns (import errors, wrong values, etc.) with counts.
    """
    base = runs_dir or _DEFAULT_RUNS_DIR
    if not isinstance(base, pathlib.Path):
        base = pathlib.Path(base)
    if not base.is_dir():
        return []

    runs = _load_runs(runs_dir)
    nogo_runs = [r for r in runs if r.get("gate") and "NO-GO" in r["gate"].upper()]

    # { failure_name: [(run_id, detail), ...] }
    fail_map: dict[str, list[tuple[str, str]]] = {}
    error_class_map: dict[str, int] = Counter()  # type: ignore[assignment]

    for r in nogo_runs:
        log_path = base / r["run_id"] / "run-log.md"
        try:
            text = log_path.read_text(encoding="utf-8")
        except Exception:
            continue
        for m in _RE_GOLDEN_FAIL.finditer(text):
            name = m.group("name").strip()
            detail = m.group("detail").strip()
            fail_map.setdefault(name, []).append((r["run_id"], detail))
            # Classify error type
            if "ModuleNotFoundError" in detail or "ImportError" in detail:
                error_class_map["import_error"] += 1
            elif "got=" in detail and "want=" in detail:
                error_class_map["value_mismatch"] += 1
            elif "Exception" in detail or "Error" in detail:
                error_class_map["runtime_error"] += 1
            else:
                error_class_map["other"] += 1

    if not fail_map:
        return []

    suggestions: list[Suggestion] = []

    # Group by most common error class
    top_class = error_class_map.most_common(1)[0][0] if error_class_map else "other"
    total_fails = sum(len(v) for v in fail_map.values())
    recurring = [(name, hits) for name, hits in fail_map.items() if len(hits) >= 2]

    if top_class == "import_error":
        suggestions.append({
            "type": "golden",
            "confidence": _confidence(total_fails),
            "reason": f"发现 {error_class_map['import_error']} 次 ImportError/ModuleNotFoundError — golden 在沙箱里跑，但 import 路径不对",
            "action": "检查 golden 的 import 行，确认沙箱里有对应模块；或在 import 里加 sys.path.insert(0,'.')",
            "data": {
                "error_class": "import_error",
                "count": error_class_map["import_error"],
                "affected_tests": list(fail_map.keys())[:10],
            },
        })

    if top_class == "value_mismatch":
        suggestions.append({
            "type": "golden",
            "confidence": _confidence(total_fails),
            "reason": f"发现 {error_class_map['value_mismatch']} 次值不匹配（got vs want），golden 期望值可能已过期",
            "action": "对比最新实现输出，更新 golden 的 expect 字段；或用 `devkit runs <run-id>` 查看详情",
            "data": {
                "error_class": "value_mismatch",
                "count": error_class_map["value_mismatch"],
                "affected_tests": list(fail_map.keys())[:10],
            },
        })

    if recurring:
        suggestions.append({
            "type": "golden",
            "confidence": _confidence(len(recurring)),
            "reason": f"{len(recurring)} 个 golden 用例在多次 run 中重复失败，可能需要重写",
            "action": "考虑删除或重写这些 golden：" + ", ".join(n for n, _ in recurring[:5]),
            "data": {
                "recurring_tests": [{"name": n, "fail_count": len(h)} for n, h in recurring],
            },
        })

    return suggestions


def quota_trend(runs_dir: Optional[pathlib.Path] = None) -> dict:
    empty = {"recent_10_cost": 0.0, "avg_cost": 0.0, "max_cost": 0.0, "trend": "stable"}
    runs = _load_runs(runs_dir)
    costs = [r["cost"] for r in runs if r.get("cost") is not None]
    if not costs:
        return empty
    recent = costs[-10:]
    total = sum(recent)
    avg = total / len(recent)
    half = len(recent) // 2
    if half >= 2:
        older = statistics.mean(recent[:half])
        newer = statistics.mean(recent[half:])
        trend = "rising" if newer > older * 1.1 else ("falling" if newer < older * 0.9 else "stable")
    else:
        trend = "stable"
    return {
        "recent_10_cost": round(total, 6),
        "avg_cost": round(avg, 6),
        "max_cost": round(max(recent), 6),
        "trend": trend,
    }
