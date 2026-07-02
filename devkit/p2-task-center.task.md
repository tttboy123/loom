# Task: P2 Task Center — `devkit/task_center.py` + `devkit task` 子命令

## 背景
用户反复针对同一个"特性/任务"跑多次 R&D loop（NO-GO → 迭代 → GO），
但目前没有办法把多次 runs 关联到同一个任务、看进度弧线。
目标：新建 `devkit/task_center.py`，提供 JSON 账本（`devkit/tasks.json`），
记录任务元信息和关联的 run_id 列表；
在 `devkit/__main__.py` 新增 `devkit task` 子命令（new / link / list / show）。

## 文件 A：新建 `devkit/task_center.py`（纯新文件，只用标准库）

账本文件：`{ROOT}/devkit/tasks.json`（不存在时自动创建）。

格式：
```json
{
  "tasks": [
    {
      "id": "word-count",
      "title": "实现 word_count 函数",
      "created": "2026-06-28T12:00:00",
      "runs": ["20260628-120000", "20260628-130000"],
      "status": "open"
    }
  ]
}
```

### 函数签名

```python
TASKS_FILE: pathlib.Path   # ROOT / "devkit" / "tasks.json"

def load_tasks() -> list[dict]:
    """读取账本，返回 tasks 列表。文件不存在或解析失败返回 []。"""

def save_tasks(tasks: list[dict]) -> None:
    """覆盖写入账本。自动创建父目录。"""

def new_task(title: str, task_id: str | None = None) -> dict:
    """新建任务。task_id 不填则自动从 title 取前 20 字符做 slug（小写，空格→'-'，去掉特殊字符）。
    若 id 已存在则抛 ValueError。
    返回新建的 task dict，status 默认 "open"，created 为当前 ISO 时间，runs=[]。"""

def link_run(task_id: str, run_id: str) -> dict:
    """把 run_id 关联到 task_id（若已关联则幂等）。
    找不到 task_id 则抛 KeyError。返回更新后的 task dict。"""

def get_task(task_id: str) -> dict | None:
    """按 id 查找，找不到返回 None。"""

def list_tasks() -> list[dict]:
    """返回所有任务列表（按创建时间逆序）。"""

def close_task(task_id: str) -> bool:
    """把 task status 置为 'closed'。找不到返回 False，成功返回 True。"""
```

辅助：`_slugify(title: str) -> str` — 私有函数，把标题变 slug

## 文件 B：修改 `devkit/__main__.py`

### 1. 在 `main()` 里加路由（在 `if argv and argv[0] == "asset":` 之前）：
```python
if argv and argv[0] == "task":
    return _cmd_task(argv[1:])
```

### 2. 新增 `_cmd_task(argv) -> int` 函数（加在 `_cmd_asset` 之前）：

```python
def _cmd_task(argv) -> int:
    """Task Center：跨 run 任务追踪（new / link / list / show / close）。"""
    from devkit import task_center as TC
    from devkit.rdloop import ROOT

    p = argparse.ArgumentParser(prog="devkit task", description="跨 run 任务追踪账本")
    sub = p.add_subparsers(dest="action")

    # new
    nw = sub.add_parser("new", help="新建任务")
    nw.add_argument("title", help="任务标题")
    nw.add_argument("--id", dest="task_id", help="指定任务 id（默认从标题自动生成）")

    # link
    lk = sub.add_parser("link", help="把 run 关联到任务")
    lk.add_argument("task_id", help="任务 id")
    lk.add_argument("run_id", help="run id（devkit/runs/<run-id>）")

    # list
    sub.add_parser("list", help="列出所有任务")

    # show
    sh = sub.add_parser("show", help="查看任务详情（含关联 runs）")
    sh.add_argument("task_id", help="任务 id")

    # close
    cl = sub.add_parser("close", help="关闭任务（标记为 closed）")
    cl.add_argument("task_id", help="任务 id")

    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0

    if a.action == "new":
        try:
            t = TC.new_task(a.title, getattr(a, "task_id", None))
            print(f"✓ 新建任务：{t['id']}  {t['title']}")
        except ValueError as e:
            print(f"✗ {e}")
            return 1
        return 0

    if a.action == "link":
        runs_dir = ROOT / "devkit" / "runs"
        if not (runs_dir / a.run_id).is_dir():
            print(f"找不到 run：{a.run_id}")
            return 1
        try:
            t = TC.link_run(a.task_id, a.run_id)
            print(f"✓ 已关联：{a.task_id} ← {a.run_id}  （共 {len(t['runs'])} 个 runs）")
        except KeyError:
            print(f"找不到任务：{a.task_id}")
            return 1
        return 0

    if a.action == "list":
        tasks = TC.list_tasks()
        if not tasks:
            print("暂无任务 —— 用 `devkit task new \"任务标题\"` 新建。")
            return 0
        print(f"{'id':<24}{'状态':<8}{'runs':>5}  标题")
        print("-" * 70)
        for t in tasks:
            print(f"  {t['id']:<22}{t.get('status','open'):<8}{len(t.get('runs',[]))+1:>4}  {t['title'][:40]}")
        return 0

    if a.action == "show":
        t = TC.get_task(a.task_id)
        if t is None:
            print(f"找不到任务：{a.task_id}")
            return 1
        print(f"id:      {t['id']}\n标题：   {t['title']}\n状态：   {t.get('status','open')}\n创建：   {t.get('created','')}")
        runs = t.get("runs", [])
        if runs:
            from devkit import insight
            items = {it["run_id"]: it for it in insight.runs_list()}
            print(f"\n关联 runs（{len(runs)} 个）：")
            for rid in runs:
                it = items.get(rid, {})
                gate = (it.get("gate") or "—")[:20]
                print(f"  {rid:<22}  {gate}")
        else:
            print("\n暂无关联 runs —— 用 `devkit task link` 关联。")
        return 0

    if a.action == "close":
        ok = TC.close_task(a.task_id)
        if ok:
            print(f"✓ 已关闭：{a.task_id}")
            return 0
        print(f"找不到任务：{a.task_id}")
        return 1

    return 0
```

## 约束
- 新建 `devkit/task_center.py`（只用标准库：json / pathlib / datetime / re）
- 只修改 `devkit/__main__.py`（不改其他文件）
- 所有文件读写异常在 task_center.py 内部捕获，不抛出（除 ValueError/KeyError 作为接口契约）
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/task_center.py` 和 `# devkit/__main__.py` 开头，产出完整文件
- 网关：http://localhost:4000
- 级别：L1 / report-only
