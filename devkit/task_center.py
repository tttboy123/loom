# devkit/task_center.py
import json
import pathlib
import re
import datetime

try:
    from devkit.rdloop import ROOT as _ROOT
    TASKS_FILE: pathlib.Path = _ROOT / "devkit" / "tasks.json"
except Exception:  # noqa: BLE001
    TASKS_FILE = pathlib.Path(__file__).resolve().parent.parent / "devkit" / "tasks.json"

def _slugify(title: str) -> str:
    """生成任务 id 的 slug：小写，空格→'-'，去掉特殊字符，取前20字符。"""
    slug = title.lower().strip()
    slug = re.sub(r'\s+', '-', slug)
    slug = re.sub(r'[^a-z0-9\u4e00-\u9fff\-]', '', slug)  # 保留中文和连字符
    slug = re.sub(r'-+', '-', slug)  # 合并连续连字符
    slug = slug.strip('-')
    return slug[:20]

def load_tasks() -> list[dict]:
    """读取账本，返回 tasks 列表。文件不存在或解析失败返回 []。"""
    try:
        with open(TASKS_FILE, 'r', encoding='utf-8') as f:
            data = json.load(f)
            if isinstance(data, dict) and "tasks" in data:
                return data["tasks"]
    except (FileNotFoundError, json.JSONDecodeError, KeyError):
        pass
    return []

def save_tasks(tasks: list[dict]) -> None:
    """覆盖写入账本。自动创建父目录。"""
    TASKS_FILE.parent.mkdir(parents=True, exist_ok=True)
    with open(TASKS_FILE, 'w', encoding='utf-8') as f:
        json.dump({"tasks": tasks}, f, ensure_ascii=False, indent=2)

def new_task(title: str, task_id: str | None = None) -> dict:
    """新建任务。task_id 为空则从标题生成 slug。若 id 已存在抛 ValueError。"""
    tasks = load_tasks()
    if task_id is None:
        task_id = _slugify(title)
        if not task_id:
            raise ValueError("无法从标题生成有效 id，请手动指定 --id")
    # 检查重复
    if any(t["id"] == task_id for t in tasks):
        raise ValueError(f"任务 id '{task_id}' 已存在")
    task = {
        "id": task_id,
        "title": title,
        "created": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "runs": [],
        "status": "open"
    }
    tasks.append(task)
    save_tasks(tasks)
    return task

def link_run(task_id: str, run_id: str) -> dict:
    """把 run_id 关联到 task_id（幂等）。找不到任务抛 KeyError。"""
    tasks = load_tasks()
    for t in tasks:
        if t["id"] == task_id:
            if run_id not in t["runs"]:
                t["runs"].append(run_id)
                save_tasks(tasks)
            return t
    raise KeyError(f"找不到任务：{task_id}")

def get_task(task_id: str) -> dict | None:
    tasks = load_tasks()
    for t in tasks:
        if t["id"] == task_id:
            return t
    return None

def list_tasks() -> list[dict]:
    """返回按创建时间逆序的任务列表。"""
    tasks = load_tasks()
    # 按 created 逆序（最新在前）
    tasks.sort(key=lambda x: x.get("created", ""), reverse=True)
    return tasks

def close_task(task_id: str) -> bool:
    tasks = load_tasks()
    for t in tasks:
        if t["id"] == task_id:
            t["status"] = "closed"
            save_tasks(tasks)
            return True
    return False
