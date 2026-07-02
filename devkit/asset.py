# devkit/asset.py
import json
import pathlib
import tomllib
from typing import Optional

ASSET_FILENAMES = ("loom.assets.toml", "loom.assets.json")

def _locate_root() -> pathlib.Path:
    """返回项目根目录（复用 devkit.rdloop.ROOT）。"""
    try:
        from devkit.rdloop import ROOT
        return pathlib.Path(ROOT)
    except (ImportError, AttributeError):
        # 若 rdloop 未定义，回退到当前工作目录上溯查找 .git 的父目录
        cwd = pathlib.Path.cwd()
        for parent in [cwd, *cwd.parents]:
            if (parent / ".git").exists():
                return parent
        return cwd

def find_assets_file(cwd: Optional[pathlib.Path] = None) -> Optional[pathlib.Path]:
    """按 cwd → ROOT → ~/.loom/ 顺序查找资产文件，找不到返回 None。"""
    cwd = pathlib.Path(cwd) if cwd else pathlib.Path.cwd()
    root = _locate_root()
    search_dirs = [cwd]
    # 若 cwd 在 root 之下，逐级向上到 root
    try:
        cwd.relative_to(root)
        for p in cwd.parents:
            if p == root.parent:  # 超过 root 则停止
                break
            search_dirs.append(p)
            if p == root:
                break
    except ValueError:
        # cwd 不在 root 之下，仅搜索 cwd 后直接跳到 root 和 home
        pass
    if root not in search_dirs:
        search_dirs.append(root)
    search_dirs.append(pathlib.Path.home() / ".loom")

    for base in search_dirs:
        for fname in ASSET_FILENAMES:
            candidate = base / fname
            if candidate.is_file():
                return candidate
    return None

def load_assets(path: Optional[pathlib.Path] = None) -> list[dict]:
    """加载资产列表。找不到文件返回 []，不抛异常。"""
    if path is None:
        path = find_assets_file()
        if path is None:
            return []
    else:
        path = pathlib.Path(path)
        if not path.is_file():
            return []

    try:
        if path.suffix == ".json":
            with path.open("r", encoding="utf-8") as fh:
                data = json.load(fh)
            assets = data.get("assets", data) if isinstance(data, dict) else data
        else:
            with path.open("rb") as fh:
                data = tomllib.load(fh)
            assets = data.get("assets", [])
    except Exception:
        return []

    # 标准化每一项
    result = []
    for item in assets:
        if not isinstance(item, dict):
            continue
        name = item.get("name", "")
        asset_type = item.get("type", "prompt")
        tags = item.get("tags", [])
        if not isinstance(tags, list):
            tags = []
        content = item.get("content", "").strip()
        trust_level = int(item.get("trust_level", 0))
        result.append({
            "name": name,
            "type": asset_type,
            "tags": tags,
            "content": content,
            "trust_level": trust_level,
        })
    return result

_TRUST_LABELS = {
    0: "untrusted",
    1: "reviewed",
    2: "reviewed+tested",
    3: "trusted",
    4: "verified",
    5: "pinned",
    6: "system",
}

def trust_label(level: int) -> str:
    return _TRUST_LABELS.get(level, f"level-{level}")


def _format_toml_asset(asset: dict) -> str:
    """将单个资产 dict 渲染为 TOML 片段。"""
    lines = [f'[[assets]]']
    lines.append(f'name = "{asset["name"]}"')
    lines.append(f'type = "{asset["type"]}"')
    trust = int(asset.get("trust_level", 0))
    lines.append(f'trust_level = {trust}')
    # 序列化 tags
    tags = asset.get("tags", [])
    tag_items = ", ".join(f'"{t}"' for t in tags)
    lines.append(f'tags = [{tag_items}]')
    # content 使用多行字符串，对内部三个双引号进行转义
    content = asset.get("content", "")
    escaped = content.replace('"""', '\\"""')
    lines.append('content = """')
    lines.append(escaped)
    lines.append('"""')
    return "\n".join(lines)

def save_assets(assets: list[dict], path: Optional[pathlib.Path] = None) -> pathlib.Path:
    """写入资产文件（默认 cwd/loom.assets.toml）。返回写入路径。"""
    if path is None:
        path = pathlib.Path.cwd() / "loom.assets.toml"
    else:
        path = pathlib.Path(path)

    # 确保目录存在
    path.parent.mkdir(parents=True, exist_ok=True)

    if path.suffix == ".json":
        with path.open("w", encoding="utf-8") as fh:
            json.dump({"assets": assets}, fh, ensure_ascii=False, indent=2)
    else:
        parts = []
        for asset in assets:
            parts.append(_format_toml_asset(asset))
        content = "\n\n".join(parts) + "\n"
        with path.open("w", encoding="utf-8") as fh:
            fh.write(content)

    return path

def add_asset(name: str, asset_type: str, content: str,
              tags: Optional[list[str]] = None, path: Optional[pathlib.Path] = None,
              trust_level: int = 0) -> dict:
    """添加或覆盖一个资产。返回被添加的 asset dict。name 重复则覆盖。"""
    assets = load_assets(path)
    new_asset = {
        "name": name,
        "type": asset_type,
        "tags": tags or [],
        "content": content,
        "trust_level": int(trust_level),
    }
    replaced = False
    for i, a in enumerate(assets):
        if a["name"] == name:
            assets[i] = new_asset
            replaced = True
            break
    if not replaced:
        assets.append(new_asset)
    save_assets(assets, path)
    return new_asset


def set_trust(name: str, level: int, path: Optional[pathlib.Path] = None) -> dict:
    """设置资产信任等级（0–6）。找不到资产抛 KeyError。"""
    if not (0 <= level <= 6):
        raise ValueError(f"trust_level 必须在 0–6 之间，收到 {level}")
    assets = load_assets(path)
    for a in assets:
        if a["name"] == name:
            a["trust_level"] = level
            save_assets(assets, path)
            return a
    raise KeyError(f"找不到资产：{name}")

def remove_asset(name: str, path: Optional[pathlib.Path] = None) -> bool:
    """删除指定名称的资产。找到并删了返回 True，未找到返回 False。"""
    assets = load_assets(path)
    original_len = len(assets)
    filtered = [a for a in assets if a["name"] != name]
    if len(filtered) == original_len:
        return False
    save_assets(filtered, path)
    return True

def get_asset(name: str, path: Optional[pathlib.Path] = None) -> Optional[dict]:
    """按 name 查找单个资产，找不到返回 None。"""
    assets = load_assets(path)
    for a in assets:
        if a["name"] == name:
            return a
    return None
