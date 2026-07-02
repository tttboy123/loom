#!/usr/bin/env python3
"""Task queue status reporter for Loom backlog.

支持：
- 按 backlog 顺序给出当前任务编号（1-based）
- 标出 running/pending/failed/done 阶段
- 输出“当前正在开发任务”
- 记录每次检查到 `--log-file`（JSONL）
"""

from __future__ import annotations

import argparse
import json
from datetime import datetime
import re
from pathlib import Path


def parse_backlog(path: str) -> list[dict]:
    p = Path(path)
    if not p.exists():
        raise FileNotFoundError(f"未找到 backlog 文件: {path}")
    data = json.loads(p.read_text(encoding="utf-8"))
    if isinstance(data, dict):
        return [t for t in data.get("tasks", []) if isinstance(t, dict)]
    if isinstance(data, list):
        return [t for t in data if isinstance(t, dict)]
    return []


def safe_task_text(task: dict) -> str:
    return (task.get("task") or task.get("title") or "").strip()


def read_latest_run(root: str, max_dirs: int = 20) -> tuple[str | None, str | None]:
    root_path = Path(root)
    run_dirs = [root_path / "runs", root_path / "devkit" / "runs"]
    dirs: list[Path] = []
    for run_dir in run_dirs:
        if not run_dir.exists():
            continue
        dirs.extend([p for p in run_dir.iterdir() if p.is_dir() and p.name])

    if not dirs:
        return None, None

    dirs = sorted(dirs, key=lambda p: p.stat().st_mtime, reverse=True)
    if not dirs:
        return None, None
    dirs = dirs[: max_dirs]

    task_patterns = (
        re.compile(r"^(?:[-\*]\s*任务[:：]\s*)(.*)$"),
        re.compile(r"^(?:[-\*]\s*task[:：]\s*)(.*)$", re.IGNORECASE),
    )

    for d in dirs:
        run_log = d / "run-log.md"
        if not run_log.exists():
            continue
        text = run_log.read_text(encoding="utf-8", errors="ignore")
        for line in text.splitlines():
            for pattern in task_patterns:
                m = pattern.match(line.strip())
                if m:
                    return d.name, m.group(1).strip()
        return d.name, None
    return None, None


def collect_snapshot(tasks: list[dict]) -> tuple[list[dict], list[dict], list[dict], list[dict], dict]:
    snapshots = []
    running = []
    pending = []
    failed = []
    done = []

    totals = {
        "total": len(tasks),
        "running": 0,
        "pending": 0,
        "failed": 0,
        "done": 0,
        "stopped": 0,
        "ready": 0,
    }

    for idx, task in enumerate(tasks, start=1):
        status = str(task.get("status", "")).lower() or "unknown"
        item = {
            "index": idx,
            "id": str(task.get("id", f"no-id-{idx}")),
            "status": status,
            "priority": task.get("priority"),
            "delivery_mode": str(task.get("delivery_mode", "report-only") or "report-only"),
            "task_kind": str(task.get("task_kind", "")).strip() or None,
            "stop_reason": str(task.get("stop_reason", "")).strip() or None,
            "human_required_reason": str(task.get("human_required_reason", "")).strip() or None,
            "failure_code": str(task.get("failure_code", "")).strip() or None,
            "contract_blocked": str(task.get("failure_code", "")).strip() == "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN",
            "task": safe_task_text(task),
        }
        snapshots.append(item)
        if status == "running":
            totals["running"] += 1
            running.append(item)
        elif status == "pending":
            totals["pending"] += 1
            pending.append(item)
            if not task.get("deps"):
                totals["ready"] += 1
        elif status == "failed":
            totals["failed"] += 1
            failed.append(item)
        elif status == "done":
            totals["done"] += 1
            done.append(item)
        elif status == "stopped":
            totals["stopped"] += 1
    return snapshots, running, pending, failed, totals


def format_line(prefix: str, item: dict) -> str:
    task_preview = item.get("task", "")
    if task_preview:
        task_preview = task_preview.replace("\n", " ")[:80]
    else:
        task_preview = "(无任务文本)"
    meta = []
    if item.get("task_kind"):
        meta.append(str(item["task_kind"]))
    if item.get("failure_code"):
        meta.append(str(item["failure_code"]))
    if item.get("stop_reason"):
        meta.append(str(item["stop_reason"]))
    if item.get("contract_blocked"):
        meta.append("contract-blocked")
    meta_suffix = f" [{' | '.join(meta)}]" if meta else ""
    return (
        f"{prefix} #{item['index']} {item['status']} "
        f"{item['id']}({item.get('priority')}/{item.get('delivery_mode')}){meta_suffix} {task_preview}"
    )


def write_state(log_file: str | None, record: dict) -> None:
    if not log_file:
        return
    out = Path(log_file)
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("a", encoding="utf-8") as fp:
        fp.write(json.dumps(record, ensure_ascii=False) + "\n")


def render_snapshot(backlog_path: str, log_file: str | None, print_queue: int = 10, run_root: str | None = None):
    tasks = parse_backlog(backlog_path)
    snapshots, running, pending, failed, totals = collect_snapshot(tasks)
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    run_id = None
    run_task = None
    if run_root:
        run_id, run_task = read_latest_run(run_root)

    lines = [
        f"[{now}] 任务队列快照",
        f"backlog: {backlog_path}",
        f"总计 {totals['total']}，进行中 {totals['running']}，待办 {totals['pending']}，阻塞/失败 {totals['failed'] + totals['stopped']}，完成 {totals['done']}，可立即执行 {totals['ready']}",
    ]
    if running:
        lines.append("正在开发：")
        for item in running:
            lines.append("  " + format_line("TASK", item))
    else:
        lines.append("正在开发：无 running 任务")

    if pending:
        lines.append("待处理队列（按文件顺序前 10）：")
        for item in pending[:print_queue]:
            lines.append("  " + format_line("TASK", item))
    if failed:
        lines.append("失败队列（按文件顺序前 10）：")
        for item in failed[:print_queue]:
            lines.append("  " + format_line("TASK", item))

    if run_id:
        lines.append(f"最近 run: {run_id}")
    if run_task:
        lines.append(f"最近 run 任务: {run_task[:100]}")

    message = "\n".join(lines)
    print(message)

    record = {
        "ts": now,
        "backlog": backlog_path,
        "totals": {
            "total": totals["total"],
            "running": totals["running"],
            "pending": totals["pending"],
            "failed": totals["failed"],
            "done": totals["done"],
            "stopped": totals["stopped"],
            "ready": totals["ready"],
        },
        "all_tasks": snapshots,
        "running": running,
        "ready": pending,
        "failed": failed,
        "done": [item for item in snapshots if item["status"] == "done"],
        "ready_top": pending[:print_queue],
        "failed_top": failed[:print_queue],
        "latest_run": {"id": run_id, "task": run_task},
    }
    write_state(log_file, record)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="查看并记录 Loom 任务队列进度")
    parser.add_argument("--backlog", default="devkit/backlog.json", help="backlog 文件路径")
    parser.add_argument("--log-file", default=None, help="记录输出为 JSONL 的文件路径")
    parser.add_argument(
        "--run-root",
        default=".",
        help="用于读取 runs 日志的根路径（默认当前仓库）",
    )
    parser.add_argument("--print-queue", type=int, default=10, help="显示前 N 条待处理任务")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    render_snapshot(
        backlog_path=args.backlog,
        log_file=args.log_file,
        print_queue=args.print_queue,
        run_root=args.run_root,
    )


if __name__ == "__main__":
    main()
