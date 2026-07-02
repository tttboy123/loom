"""
用户自定义角色 / 载体层 —— 让「每个 Agent 的配置」成为**数据**而非写死的代码。

研发流水线的角色（阶段）从一个用户可编辑的文件读取；找不到才回退到内置默认 5 角色。
支持 **TOML**（推荐，多行 system 提示友好，Python 3.11+ 标准库 tomllib 直接读）与 **JSON**。

最低接入成本：`carrier` 可以直接写网关**已知的任意后端名**
（deepseek / glm / minimax / codex-sub），这样**零额外网关配置、免重启**；
也可以写 `loom-*` 语义载体（享受热改厂商那一套）。

查找顺序：$LOOM_ROLES → 当前目录 → 项目根（agent-platform）→ ~/.loom/。
"""
from __future__ import annotations

import json
import os
import pathlib
from typing import List, Optional

from devkit.rdloop import ROOT, Stage
from devkit.rdloop import STAGES as _DEFAULT_STAGES

FILENAMES = ("loom.roles.toml", "loom.roles.json")
EXECUTORS = ("chat", "hermes", "openclaw", "codex", "opencode")
# 控制台读写的「共享角色文件」—— 放在 devkit/（容器内可写 + 与宿主机共享），
# 让控制台在线编辑与 CLI `devkit roles`/`run` 用同一份。
CONSOLE_ROLES_PATH = ROOT / "devkit" / "loom.roles.toml"


def _candidate_paths(cwd: Optional[pathlib.Path] = None) -> List[pathlib.Path]:
    cwd = cwd or pathlib.Path.cwd()
    out: List[pathlib.Path] = []
    env = os.environ.get("LOOM_ROLES")
    if env:
        out.append(pathlib.Path(env))
    for base in (cwd, ROOT, ROOT / "devkit", pathlib.Path.home() / ".loom"):
        for name in FILENAMES:
            out.append(base / name)
    return out


def find_roles_file(cwd: Optional[pathlib.Path] = None) -> Optional[pathlib.Path]:
    for p in _candidate_paths(cwd):
        if p.is_file():
            return p
    return None


def active_source(cwd: Optional[pathlib.Path] = None) -> str:
    p = find_roles_file(cwd)
    return str(p) if p else "(内置默认 5 角色)"


def _parse(path: pathlib.Path) -> dict:
    txt = path.read_text(encoding="utf-8")
    if path.suffix == ".json":
        return json.loads(txt)
    import tomllib                       # 标准库（Python 3.11+）
    return tomllib.loads(txt)


def _to_stage(d: dict, i: int) -> Stage:
    if not isinstance(d, dict):
        raise ValueError(f"第 {i} 个 stage 不是表/对象")
    key = str(d.get("key") or "").strip()
    if not key:
        raise ValueError(f"第 {i} 个 stage 缺少 key")
    system = str(d.get("system") or "").strip()
    if not system:
        raise ValueError(f"stage {key!r} 缺少 system（角色契约 / 系统提示）")
    carrier = str(d.get("carrier") or d.get("model") or "").strip()
    if not carrier:
        raise ValueError(f"stage {key!r} 缺少 carrier/model（指向哪个载体或后端）")
    role = str(d.get("role") or key).strip()
    title = str(d.get("title") or key).strip()
    executor = str(d.get("executor") or "chat").strip().lower()
    if executor not in EXECUTORS:
        raise ValueError(f"stage {key!r} 的 executor 非法：{executor!r}（可选 {list(EXECUTORS)}）")
    mt = d.get("max_tokens")
    if mt in (None, "", 0):
        mt = None
    else:
        try:
            mt = int(mt)
        except (TypeError, ValueError):
            raise ValueError(f"stage {key!r} 的 max_tokens 必须是正整数")
        if mt <= 0:
            raise ValueError(f"stage {key!r} 的 max_tokens 必须 > 0")
    return Stage(key, role, carrier, title, system, executor, mt)


def validate_stage_dicts(raw, src: str = "") -> List[Stage]:
    """把一串 dict 校验成 [Stage]（缺字段 / key 重复即抛 ValueError）。CLI 与控制台共用。"""
    where = f"{src}: " if src else ""
    if not isinstance(raw, list) or not raw:
        raise ValueError(f"{where}需要一个非空的 stages 列表")
    stages = [_to_stage(d, i) for i, d in enumerate(raw, 1)]
    keys = [s.key for s in stages]
    dup = sorted({k for k in keys if keys.count(k) > 1})
    if dup:
        raise ValueError(f"{where}stage key 重复：{dup}")
    return stages


def load_stages(path: Optional[str] = None) -> List[Stage]:
    """读取用户角色文件 → [Stage]；没有文件则回退内置默认。校验失败抛 ValueError。"""
    p = pathlib.Path(path) if path else find_roles_file()
    if not p:
        return list(_DEFAULT_STAGES)
    data = _parse(p)
    raw = data.get("stages") if isinstance(data, dict) else data
    return validate_stage_dicts(raw, str(p))


def save_stages(raw, path: Optional[pathlib.Path] = None) -> dict:
    """校验一串角色 dict 并写成 TOML（默认写控制台共享文件）。返回 {ok, path, n}。"""
    dest = pathlib.Path(path) if path else CONSOLE_ROLES_PATH
    stages = validate_stage_dicts(raw)            # 先校验，过不了不落盘
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(stages_to_toml(stages), encoding="utf-8")
    return {"ok": True, "path": str(dest), "n": len(stages)}


def delete_console_roles() -> bool:
    """删除控制台共享角色文件（恢复默认/其它来源）。返回是否删了。"""
    if CONSOLE_ROLES_PATH.is_file():
        CONSOLE_ROLES_PATH.unlink()
        return True
    return False


# --------------------------------------------------------------------------- #
# 脚手架渲染（devkit roles init 用）—— 以当前生效角色为蓝本，让用户从可跑的基线改起
# --------------------------------------------------------------------------- #
def _toml_ml(s: str) -> str:
    # 多行 basic string：先转义反斜杠，再断开三引号序列
    return s.replace("\\", "\\\\").replace('"""', '\\"\\"\\"')


def stages_to_toml(stages: List[Stage]) -> str:
    head = (
        "# ============================================================\n"
        "# Loom 角色定义 —— 你自己的研发流水线（这就是「每个 Agent 的配置」）。\n"
        "# 每个 [[stages]] = 一个角色 / 阶段，按从上到下的顺序执行。\n"
        "#\n"
        "# carrier：指向哪个模型。两种写法——\n"
        "#   1) 直接写网关已知的后端名：deepseek / glm / minimax / codex-sub\n"
        "#      → 零额外配置、免重启，最低接入成本。\n"
        "#   2) 写 loom-* 语义载体（在 litellm/config.full.yaml 里）→ 可在控制台热改厂商。\n"
        "#\n"
        "# executor：本阶段用哪个执行器 —— chat（默认，扁平对话）| hermes | openclaw（带工具的 agent）。\n"
        "# max_tokens：本阶段单独的 token 上限（可选；不写则用 run 级默认）。\n"
        "#\n"
        "# 改完直接 `devkit run \"任务\"` 即可生效（devkit roles list 可查当前生效）。\n"
        "# ============================================================\n\n"
    )
    blocks = []
    for s in stages:
        mt = (f"max_tokens = {s.max_tokens}\n" if s.max_tokens
              else "# max_tokens = 900        # 可选：本阶段单独的 token 上限\n")
        blocks.append(
            "[[stages]]\n"
            f'key = "{s.key}"\n'
            f'role = "{s.role}"\n'
            f'title = "{s.title}"\n'
            f'carrier = "{s.carrier}"\n'
            f'executor = "{s.executor}"        # chat | hermes | openclaw | codex\n'
            f"{mt}"
            f'system = """\n{_toml_ml(s.system)}\n"""\n'
        )
    return head + "\n".join(blocks)


def stages_to_json(stages: List[Stage]) -> str:
    return json.dumps(
        {"stages": [{"key": s.key, "role": s.role, "title": s.title,
                     "carrier": s.carrier, "executor": s.executor,
                     "max_tokens": s.max_tokens, "system": s.system} for s in stages]},
        ensure_ascii=False, indent=2)
