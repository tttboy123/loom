"""devkit/dashboard.py — 终端 dashboard，整合 backlog_stats 和 run_summary。"""
from __future__ import annotations

import json
import os
import pathlib

def render(
    backlog_path: str = 'devkit/backlog.json',
    runs_dir: str = 'devkit/runs',
    n: int = 5,
) -> str:
    """返回多行 dashboard 字符串。路径不存在时不报错，返回合理默认值。"""
    # backlog stats
    total = done = pending = failed = 0
    score = 0.5
    try:
        from devkit.backlog_stats import stats, health_score
        bl = json.loads(pathlib.Path(backlog_path).read_text(encoding='utf-8')) if os.path.exists(backlog_path) else []
        s = stats(bl)
        total, done, pending, failed = s['total'], s['done'], s['pending'], s['failed']
        score = health_score(bl)
    except Exception:
        pass

    pct = (done / total * 100.0) if total > 0 else 0.0

    # recent runs table
    table = '(no runs)'
    try:
        from devkit.run_summary import recent_runs, format_table
        rows = recent_runs(n=n, runs_dir=runs_dir)
        table = format_table(rows)
    except Exception:
        pass

    observability = None
    try:
        from devkit.agent_observability import collect
        observability = collect(backlog_path=backlog_path, runs_dir=runs_dir)
    except Exception:
        observability = None

    lines = [
        '=== Loom Dashboard ===',
        f'Backlog: {total} tasks | Done: {done} ({pct:.0f}%) | Pending: {pending} | Failed: {failed}',
        f'Health: {score:.2f} | Recent runs: {n}',
    ]
    if observability:
        q = observability['queue']['totals']
        lr = observability['latest_run']
        ap = observability['autopilot']
        running = observability['queue'].get('running', [])
        failed_top = observability['queue'].get('failed_top', [])
        lines += [
            f'Autopilot: tmux={"up" if ap["tmux_alive"] else "down"} | worker={"up" if ap["worker"]["alive"] else "down"} | Ready: {q["ready"]} | Running: {q["running"]} | Stopped: {q.get("stopped", 0)} | ContractBlocked: {q.get("contract_blocked", 0)}',
            f'Latest: {lr["id"] or "-"} | SilentZero: {"yes" if lr["silent_zero"] else "no"} | Gate: {lr["gate"] or "-"} | Failure: {lr.get("failure_code") or "-"}',
        ]
        if running:
            item = running[0]
            lines.append(f'Current: #{item["index"]} {item["id"]} [{item.get("task_kind") or "-"}] {item["task"]}')
        if failed_top:
            item = failed_top[0]
            reason = item.get("failure_code") or item.get("stop_reason") or "-"
            lines.append(f'FailedTop: #{item["index"]} {item["id"]} [{item.get("task_kind") or "-"}] {reason}')
    lines += [
        table,
    ]
    return '\n'.join(lines)
