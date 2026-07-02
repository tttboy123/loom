# Task: P2 Loom Asset Importer — `devkit/asset.py` + `devkit asset` 子命令

## 背景
Loom 用户在反复运行中会积累常用的系统提示片段、规则约束、MCP 配置。
目标：新建 `devkit/asset.py` 模块，支持把这些可复用片段保存为「资产」，
并在 `devkit/__main__.py` 新增 `devkit asset` 子命令（list / show / add / remove）。

## 文件 A：新建 `devkit/asset.py`（纯新文件，只用标准库）

资产格式（存于 `loom.assets.toml`，查找顺序同 roles：cwd → 项目根 → ~/.loom/）：

```toml
[[assets]]
name = "anti-overengineering"
type = "rule"
tags = ["review", "quality"]
content = """
不增加新依赖，不引入新抽象，只改任务要求的内容。
"""

[[assets]]
name = "deepseek-system"
type = "skill"
tags = ["implement"]
content = """
你是资深 Python 工程师，优先用标准库，不过度封装。
"""
```

### 数据类型

```python
# type ∈ "rule" | "skill" | "mcp" | "prompt"
# tags: list[str]，可选
# content: str（多行 system prompt 片段）
```

### 主要函数

```python
ASSET_FILENAMES = ("loom.assets.toml", "loom.assets.json")

def find_assets_file(cwd=None) -> pathlib.Path | None:
    """按 cwd → ROOT → ~/.loom/ 顺序查找资产文件，找不到返回 None。"""

def load_assets(path=None) -> list[dict]:
    """加载资产列表。找不到文件返回 []，不抛异常。
    每项包含：name, type, tags（list，默认[]）, content。"""

def save_assets(assets: list[dict], path=None) -> pathlib.Path:
    """写入资产文件（默认 cwd/loom.assets.toml）。返回写入路径。"""

def add_asset(name: str, asset_type: str, content: str,
              tags: list[str] | None = None, path=None) -> dict:
    """添加或覆盖一个资产。返回被添加的 asset dict。
    name 重复则覆盖。"""

def remove_asset(name: str, path=None) -> bool:
    """删除指定名称的资产。找到并删了返回 True，未找到返回 False。"""

def get_asset(name: str, path=None) -> dict | None:
    """按 name 查找单个资产，找不到返回 None。"""
```

实现要点：
- TOML 格式用 `tomllib`（Python 3.11+ 标准库）读；写时用简单字符串拼接生成 TOML（无需第三方库）
- JSON 格式 fallback：若文件扩展名 .json，用 json 模块读写
- `save_assets` 写 TOML 时，多行 content 用 `"""..."""` 包裹，自动转义内部的 `"""`
- ROOT 从 `devkit.rdloop` 导入

## 文件 B：修改 `devkit/__main__.py`

### 1. 在 `main()` 里加路由（在 `if argv and argv[0] == "recommend":` 之前）：
```python
if argv and argv[0] == "asset":
    return _cmd_asset(argv[1:])
```

### 2. 新增 `_cmd_asset(argv) -> int` 函数（加在 `_cmd_recommend` 之前）：

```python
def _cmd_asset(argv) -> int:
    """管理可复用资产（system prompt 片段 / 规则 / 技能）。"""
    from devkit import asset as A
    p = argparse.ArgumentParser(prog="devkit asset",
                                description="管理可复用资产（prompt / rule / skill / mcp）")
    sub = p.add_subparsers(dest="action")

    # list
    ls = sub.add_parser("list", help="列出所有资产")
    ls.add_argument("--type", dest="filter_type", metavar="TYPE",
                    help="只显示某类型（rule/skill/mcp/prompt）")
    ls.add_argument("--tag", dest="filter_tag", metavar="TAG",
                    help="只显示含某 tag 的资产")

    # show
    sh = sub.add_parser("show", help="显示一个资产的完整内容")
    sh.add_argument("name", help="资产名称")

    # add
    ad = sub.add_parser("add", help="添加或覆盖一个资产")
    ad.add_argument("name", help="资产名称（唯一标识）")
    ad.add_argument("--type", dest="asset_type", default="prompt",
                    choices=["rule", "skill", "mcp", "prompt"], help="资产类型（默认 prompt）")
    ad.add_argument("--tags", default="", help="逗号分隔的 tag 列表")
    ad.add_argument("--content", required=True, help="资产内容（system prompt 片段）")

    # remove
    rm = sub.add_parser("remove", help="删除一个资产")
    rm.add_argument("name", help="资产名称")

    a = p.parse_args(argv)
    if not a.action:
        p.print_help()
        return 0

    if a.action == "list":
        assets = A.load_assets()
        if not assets:
            print("暂无资产 —— 用 `devkit asset add` 添加。")
            return 0
        if a.filter_type:
            assets = [x for x in assets if x.get("type") == a.filter_type]
        if a.filter_tag:
            assets = [x for x in assets if a.filter_tag in x.get("tags", [])]
        print(f"{'名称':<24}{'类型':<10}{'tags'}")
        print("-" * 60)
        for x in assets:
            tags = ", ".join(x.get("tags", []))
            print(f"  {x['name']:<22}{x.get('type','?'):<10}{tags}")
        print(f"\n共 {len(assets)} 个资产。")
        return 0

    if a.action == "show":
        x = A.get_asset(a.name)
        if x is None:
            print(f"找不到资产：{a.name}")
            return 1
        print(f"name:    {x['name']}\ntype:    {x.get('type','?')}\n"
              f"tags:    {', '.join(x.get('tags', []))}\n\n{x['content']}")
        return 0

    if a.action == "add":
        tags = [t.strip() for t in a.tags.split(",") if t.strip()]
        x = A.add_asset(a.name, a.asset_type, a.content, tags)
        print(f"✓ 已添加资产：{x['name']}（{x['type']}）")
        return 0

    if a.action == "remove":
        ok = A.remove_asset(a.name)
        if ok:
            print(f"✓ 已删除：{a.name}")
            return 0
        print(f"找不到资产：{a.name}")
        return 1

    return 0
```

## 约束
- 新建 `devkit/asset.py`（只用标准库：tomllib / json / pathlib）
- 只修改 `devkit/__main__.py`（不改其他文件）
- 所有文件读写异常在 asset.py 内部捕获，不抛出
- `save_assets` 写 TOML 时 name/type/tags 写单行，content 写多行 `"""`
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/asset.py` 和 `# devkit/__main__.py` 开头，产出完整文件
- 网关：http://localhost:4000
- 级别：L1 / report-only
