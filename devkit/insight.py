"""
模型洞察：① 额度薅羊毛（quota）② 模型评分（scores）。

数据来源（诚实）：
  - 实际使用：全部来自 Loom 真跑出来的数据 —— LiteLLM `/spend/logs`（按后端的花费/tokens）
    + `devkit/runs/*/run-log.md`（成功率/延迟/成本）+ 你的 👍/👎 评分。
  - 官网评测：**不杜撰** —— 来自你维护的 `loom.scores.toml`（按各家文档/公开榜自己折算填）。

配置文件（可选，TOML/JSON；查找：$LOOM_QUOTA/$LOOM_SCORES → 当前目录 → 项目根 → ~/.loom/）：
  - loom.quota.toml：每个后端的免费额度 / 是否订阅。
  - loom.scores.toml：每个后端的官网评测分（0-100）。
"""
from __future__ import annotations

import json
import pathlib
import re
import urllib.request
from datetime import datetime, timezone
from typing import Optional

from devkit.rdloop import ROOT

RATINGS = ROOT / "devkit" / "ratings.jsonl"
CONFIG = ROOT / "litellm" / "config.full.yaml"
BACKEND_ORDER = ("claude", "codex", "glm", "deepseek", "minimax")

# 把五花八门的模型名（loom-*、openai/xxx、降级后的别名）归一到 5 个后端
_MATCH = {
    "claude": ("claude",),
    "codex": ("codex", "gpt-5", "gpt5"),
    "glm": ("glm",),
    "minimax": ("minimax", "abab"),
    "deepseek": ("deepseek",),
}


def canonical_backend(model: str) -> str:
    s = (model or "").lower()
    for name, subs in _MATCH.items():
        if any(x in s for x in subs):
            return name
    return "other"


def _carrier_map() -> dict:
    """loom-* 语义载体 → 实际模型串（从 config.full.yaml 解析，和控制台一致）。"""
    out = {}
    if CONFIG.exists():
        txt = CONFIG.read_text(encoding="utf-8")
        for m in re.finditer(r"model_name:\s*(loom-[\w-]+)[\s\S]*?model:\s*([^\n#]+)", txt):
            out[m.group(1).strip()] = m.group(2).strip()
    return out


def resolve_backend(served: str, carrier: str = "", carrier_map: Optional[dict] = None) -> str:
    """把（实际模型 / 载体）解析到 5 个后端之一：先看实际模型，再载体名，最后查 loom-* 映射。"""
    b = canonical_backend(served)
    if b != "other":
        return b
    b = canonical_backend(carrier)
    if b != "other":
        return b
    cm = carrier_map if carrier_map is not None else _carrier_map()
    return canonical_backend(cm.get(carrier, ""))


# --------------------------------------------------------------------------- #
# 配置文件加载（与 roles 同风格）
# --------------------------------------------------------------------------- #
def _find(filenames, env_key) -> Optional[pathlib.Path]:
    import os
    cands = []
    if env_key and os.environ.get(env_key):
        cands.append(pathlib.Path(os.environ[env_key]))
    for base in (pathlib.Path.cwd(), ROOT, pathlib.Path.home() / ".loom"):
        for n in filenames:
            cands.append(base / n)
    for p in cands:
        if p.is_file():
            return p
    return None


def _parse(path: pathlib.Path) -> dict:
    txt = path.read_text(encoding="utf-8")
    if path.suffix == ".json":
        return json.loads(txt)
    import tomllib
    return tomllib.loads(txt)


def load_quota_config() -> dict:
    p = _find(("loom.quota.toml", "loom.quota.json"), "LOOM_QUOTA")
    return _parse(p) if p else {}


def load_scores_config() -> dict:
    p = _find(("loom.scores.toml", "loom.scores.json"), "LOOM_SCORES")
    return _parse(p) if p else {}


# --------------------------------------------------------------------------- #
# 实际使用数据
# --------------------------------------------------------------------------- #
def _get(url: str, key: str, timeout: int = 8):
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {key}"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.loads(r.read().decode())


BASE_BACKENDS = ("codex-sub", "glm", "minimax", "deepseek")


def probe_one(base_url: str, key: str, model: str, timeout: int = 10) -> dict:
    """对单个基础后端做最小推理探测，**关掉降级链**（LiteLLM 文档开关 disable_fallbacks）——
    挂了就如实报错，不被别的后端悄悄兜住。并核对实际服务模型确实是探测目标（双保险）。
    返回 {backend, ok, detail}。"""
    payload = json.dumps({"model": model, "messages": [{"role": "user", "content": "hi"}],
                          "max_tokens": 1, "num_retries": 0, "disable_fallbacks": True}).encode()
    req = urllib.request.Request(
        f"{base_url}/v1/chat/completions", data=payload, method="POST",
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            served = (json.loads(r.read().decode()) or {}).get("model", "")
        # 双保险：即使降级没关掉，服务模型若不是探测目标，也判为 DOWN（防假阳性）。
        # served 为空（200 但无 model 字段）则跳过核对、按 ok 计——disable_fallbacks 才是主保证。
        if served and canonical_backend(served) != canonical_backend(model):
            return {"backend": model, "ok": False,
                    "detail": f"served={served}（≠探测目标，疑似被降级兜住）"}
        return {"backend": model, "ok": True, "detail": f"served={served or '?'}"}
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", "replace")
        try:
            msg = json.loads(body).get("error", {}).get("message", "") or body
        except Exception:  # noqa: BLE001
            msg = body
        return {"backend": model, "ok": False, "detail": str(msg).strip()[:140]}
    except Exception as ex:  # noqa: BLE001
        return {"backend": model, "ok": False, "detail": f"{type(ex).__name__}: {ex}"[:140]}


def liveness(base_url: str, key: str, backends=BASE_BACKENDS, prober=None) -> list:
    """并发探测基础后端真活性（关降级）。prober 可注入，便于单测不打网络。
    用线程池避免 5 个后端串行、全挂时卡到 ~timeout×5。"""
    from concurrent.futures import ThreadPoolExecutor
    p = prober or probe_one
    backends = tuple(backends)
    with ThreadPoolExecutor(max_workers=min(len(backends) or 1, 5)) as ex:
        return list(ex.map(lambda b: p(base_url, key, b), backends))  # map 保序


# --------------------------------------------------------------------------- #
# 健康判定（K8s 探针式）：把"真活性(liveness)"和"凭据就绪(readiness=token 没过期)"
# 合成一个状态，区分「过期需重登」「真挂了」「在服务」。
# --------------------------------------------------------------------------- #
TOKEN_DIR_DEFAULT = pathlib.Path("~/.cli-proxy-api").expanduser()
_SUB_TYPE = {"codex-sub": "codex"}  # 订阅后端 → token type


def _parse_iso(s) -> "datetime | None":
    """解析 ISO8601（含尾 Z 或 +08:00 偏移）→ 带时区的 UTC。失败返回 None（不抛、不误判过期）。"""
    if not s:
        return None
    txt = str(s).strip()
    if txt.endswith("Z"):
        txt = txt[:-1] + "+00:00"
    try:
        dt = datetime.fromisoformat(txt)
    except Exception:  # noqa: BLE001
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def _cred_state(backend: str, token_dir, now) -> tuple:
    """凭据就绪态：valid / expired / disabled / unknown / unavailable，+ 一句说明。
    非订阅后端=unknown（无 token 文件）；订阅后端但读不到目录/文件=unavailable（如容器没挂载）。"""
    typ = _SUB_TYPE.get(backend)
    if not typ:
        return "unknown", ""                       # glm/minimax/deepseek：直连 API key，无过期文件
    td = pathlib.Path(token_dir)
    if not td.is_dir():
        return "unavailable", ""                   # 订阅后端但目录不可读（容器未挂载 ~/.cli-proxy-api）
    cands = []                                      # 同 type 可能有多份（陈旧+新）→ 选 expired 最新的那份
    for f in sorted(td.glob("*.json")):
        try:
            d = json.loads(f.read_text(encoding="utf-8"))
        except Exception:  # noqa: BLE001
            continue
        if d.get("type") == typ:
            cands.append((_parse_iso(d.get("expired")), d))
    if not cands:
        return "unavailable", ""                    # 是订阅后端却找不到对应 token 文件
    dated = [(e, d) for e, d in cands if e is not None]
    d = max(dated, key=lambda x: x[0])[1] if dated else cands[0][1]  # 优先最新；都不可解析取第一个
    if d.get("disabled"):
        return "disabled", "token 被禁用"
    exp = _parse_iso(d.get("expired"))
    if exp is None:
        return "unknown", ""                        # 无法解析 expired → 当未知，绝不误判为过期
    if exp < now:
        return "expired", f"token 过期 {max((now - exp).days, 0)} 天前"
    return "valid", ""


def health(base_url: str, key: str, token_dir=None, prober=None, now=None) -> list:
    """逐后端健康判定。返回 [{backend, ok, detail, state, action, token_state}]，
    state ∈ serving/expired/down；ok==(state=='serving') 保持与 liveness 向后兼容。"""
    token_dir = TOKEN_DIR_DEFAULT if token_dir is None else token_dir
    now = now or datetime.now(timezone.utc)
    out = []
    for r in liveness(base_url, key, prober=prober):
        b, live, detail = r["backend"], r["ok"], r["detail"]
        cred, note = _cred_state(b, token_dir, now)
        action = ""
        if cred == "disabled":                      # 禁用：永远要重登
            state, detail, action = "expired", note, "run ./loom login"
        elif cred == "expired" and not live:        # 用户那个 bug：过期且打不通 → 明确"过期"
            state, detail, action = "expired", note, "run ./loom login"
        elif cred == "expired" and live:            # 过期但还在服务 → 仍 serving，detail 带告警
            state = "serving"
            detail = f"{detail}（{note}但仍在服务）" if note else detail
        elif not live:                              # 打不通且凭据 valid/unknown/unavailable → 真挂
            state = "down"
        else:
            state = "serving"
        out.append({"backend": b, "ok": (state == "serving"), "detail": detail,
                    "state": state, "action": action, "token_state": cred})
    return out


def spend_by_backend(base_url: str, key: str) -> dict:
    """从 LiteLLM /spend/logs 聚合每个后端的 calls/tokens/spend（真实花费）。"""
    logs = _get(f"{base_url}/spend/logs", key) or []
    agg: dict = {}
    for r in logs:
        b = canonical_backend(r.get("model", ""))
        a = agg.setdefault(b, {"backend": b, "calls": 0, "tokens": 0, "spend": 0.0})
        a["calls"] += 1
        a["tokens"] += int(r.get("total_tokens", 0) or 0)
        a["spend"] += float(r.get("spend", 0) or 0)
    return agg


_ROW = re.compile(
    r"^\|\s*[^|]+?\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*(OK|BLOCKED)\s*\|"
    r"\s*([\d.]+)s\s*\|\s*(\d+)\s*\|\s*\$([\d.]+)\s*\|")

# 同一行格式，但额外捕获第一列（阶段 st.key）——给"按阶段"透视用。
# 7 组：(stage, carrier, served, status, dt, toks, cost)。表头(| 阶段 |)/分隔行(| --- |)
# 因缺少 OK|BLOCKED 与 [\d.]+s 单元，照样不匹配，自动被跳过。
_ROW_STAGE = re.compile(
    r"^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*(OK|BLOCKED)\s*\|"
    r"\s*([\d.]+)s\s*\|\s*(\d+)\s*\|\s*\$([\d.]+)\s*\|")


def run_stats_by_backend(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """解析 run-log.md 各阶段行，按实际服务后端聚合 uses/成功/延迟/tokens/成本。"""
    agg: dict = {}
    if not runs_dir.exists():
        return agg
    cmap = _carrier_map()                        # loom-* → 实际模型（解析一次复用）
    for log in runs_dir.glob("*/run-log.md"):
        try:
            lines = log.read_text(encoding="utf-8").splitlines()
        except Exception:  # noqa: BLE001
            continue
        for line in lines:
            m = _ROW.match(line.strip())
            if not m:
                continue
            carrier, served, status, dt, toks, cost = m.groups()
            b = resolve_backend(served, carrier, cmap)  # BLOCKED 行 served='-' → 用载体/映射解析
            a = agg.setdefault(b, {"backend": b, "uses": 0, "ok": 0,
                                   "lat": 0.0, "tokens": 0, "cost": 0.0})
            a["uses"] += 1
            a["ok"] += 1 if status == "OK" else 0
            a["lat"] += float(dt)
            a["tokens"] += int(toks)
            a["cost"] += float(cost)
    return agg


def run_stats_by_stage(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """解析 run-log.md 各行，按【阶段】（第一列 st.key）聚合 uses/成功/延迟/tokens/成本。
    与 run_stats_by_backend 同语义，只是聚合键换成阶段。BLOCKED 行照计 uses（ok+0）。
    缺目录→{}；不匹配行（含表头/分隔行）跳过；读不到的文件跳过，不抛异常。"""
    agg: dict = {}
    if not runs_dir.exists():
        return agg
    for log in runs_dir.glob("*/run-log.md"):
        try:
            lines = log.read_text(encoding="utf-8").splitlines()
        except Exception:  # noqa: BLE001
            continue
        for line in lines:
            m = _ROW_STAGE.match(line.strip())
            if not m:
                continue
            stage, _carrier, _served, status, dt, toks, cost = m.groups()
            a = agg.setdefault(stage, {"stage": stage, "uses": 0, "ok": 0,
                                       "lat": 0.0, "tokens": 0, "cost": 0.0})
            a["uses"] += 1
            a["ok"] += 1 if status == "OK" else 0
            a["lat"] += float(dt)
            a["tokens"] += int(toks)
            a["cost"] += float(cost)
    return agg


def stage_report(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """按阶段透视：每阶段 uses/成功率/均延迟/均tok/均$/总tok/总$/占总成本%。
    缺目录或空目录都→ {"rows": [], "totals": {"tokens": 0, "cost": 0.0, "uses": 0}}。
    pct_cost 在总成本为 0 时记 0.0（不除零）。rows 按总成本降序。"""
    stats = run_stats_by_stage(runs_dir)
    grand_cost = sum((s["cost"] for s in stats.values()), 0.0)
    grand_tokens = sum(s["tokens"] for s in stats.values())
    grand_uses = sum(s["uses"] for s in stats.values())
    rows = []
    for s in stats.values():
        uses = s["uses"]
        rows.append({
            "stage": s["stage"],
            "uses": uses,
            "ok_rate": round(s["ok"] / uses * 100) if uses else None,
            "avg_lat": round(s["lat"] / uses, 1) if uses else None,
            "avg_tokens": round(s["tokens"] / uses) if uses else None,
            "avg_cost": round(s["cost"] / uses, 5) if uses else None,
            "total_tokens": s["tokens"],
            "total_cost": round(s["cost"], 5),
            "pct_cost": round(s["cost"] / grand_cost * 100, 1) if grand_cost else 0.0,
        })
    rows.sort(key=lambda x: x["total_cost"], reverse=True)
    return {"rows": rows,
            "totals": {"tokens": grand_tokens, "cost": round(grand_cost, 5),
                       "uses": grand_uses}}


# --------------------------------------------------------------------------- #
# 用户评分（👍/👎）
# --------------------------------------------------------------------------- #
def add_rating(backend: str, value: int, note: str = "") -> str:
    b = canonical_backend(backend) if canonical_backend(backend) != "other" else backend
    RATINGS.parent.mkdir(parents=True, exist_ok=True)
    rec = {"backend": b, "value": 1 if value > 0 else -1, "note": note,
           "ts": datetime.now().strftime("%Y-%m-%d %H:%M:%S")}
    with RATINGS.open("a", encoding="utf-8") as f:
        f.write(json.dumps(rec, ensure_ascii=False) + "\n")
    return b


def ratings_by_backend() -> dict:
    agg: dict = {}
    if not RATINGS.exists():
        return agg
    for line in RATINGS.read_text(encoding="utf-8").splitlines():
        try:
            r = json.loads(line)
        except Exception:  # noqa: BLE001
            continue
        a = agg.setdefault(r.get("backend", "other"), {"up": 0, "down": 0})
        if int(r.get("value", 0)) > 0:
            a["up"] += 1
        else:
            a["down"] += 1
    return agg


# --------------------------------------------------------------------------- #
# ① 额度薅羊毛报告
# --------------------------------------------------------------------------- #
def provider_balance(backend: str, provider_key: Optional[str]) -> dict:
    """尝试从 provider 余额 API 拉取实时可用余额。

    支持：deepseek（GET https://api.deepseek.com/user/balance），CNY → USD（÷7.2）。
    其余 backend → source="unsupported"；无 key → source="no_key"；失败 → source="error"。
    """
    if not provider_key:
        return {"backend": backend, "available_usd": None, "available_cny": None, "source": "no_key"}
    if backend != "deepseek":
        return {"backend": backend, "available_usd": None, "available_cny": None, "source": "unsupported"}
    try:
        req = urllib.request.Request(
            "https://api.deepseek.com/user/balance",
            headers={"Authorization": f"Bearer {provider_key}"},
        )
        with urllib.request.urlopen(req, timeout=10) as r:
            data = json.loads(r.read().decode("utf-8"))
        cny_info = next(
            (x for x in data.get("balance_infos", []) if x.get("currency") == "CNY"), None
        )
        if cny_info is None:
            return {"backend": backend, "available_usd": None, "available_cny": None, "source": "error"}
        available_cny = float(cny_info.get("granted_balance", 0)) + float(cny_info.get("topped_up_balance", 0))
        available_usd = round(available_cny / 7.2, 5)
        return {"backend": backend, "available_usd": available_usd, "available_cny": available_cny, "source": "api"}
    except Exception:  # noqa: BLE001
        return {"backend": backend, "available_usd": None, "available_cny": None, "source": "error"}


def quota_report(base_url: str, key: str) -> dict:
    """每个后端：类型 / 已用$ / 免费额度 / 剩余 / 用量%，并按"薅价值"排序。"""
    cfg = load_quota_config()
    try:
        spend = spend_by_backend(base_url, key)
        gateway_ok = True
    except Exception:  # noqa: BLE001  网关不可达 → 只给配置 + 0 用量
        spend, gateway_ok = {}, False

    rows = []
    for b in BACKEND_ORDER:
        c = cfg.get(b, {}) if isinstance(cfg, dict) else {}
        used = float(spend.get(b, {}).get("spend", 0.0))
        toks = int(spend.get(b, {}).get("tokens", 0))
        note = c.get("note", "")
        # 默认：claude/codex 在 Loom 里走订阅（除非配置显式覆盖 subscription=false）
        if c.get("subscription", b in ("claude", "codex")):
            kind, free, remaining, pct = "订阅", None, None, None
        else:
            free = float(c.get("free_usd", 0) or 0)
            # 尝试实时余额（仅当配置了 provider_key）
            pb = None
            pk = c.get("provider_key")
            if pk:
                pb = provider_balance(b, pk)
            if pb is not None and pb.get("available_usd") is not None:
                kind = "免费额度"
                remaining = pb["available_usd"]
                free = remaining + used  # 估算 free_usd
                pct = round(min(used / free * 100, 100), 1) if free > 0 else 0.0
                note = (note + " [实时]").strip()
            elif free > 0:
                kind = "免费额度"
                remaining = max(free - used, 0.0)
                pct = round(min(used / free * 100, 100), 1)
            else:
                kind, remaining, pct = "付费", None, None
        rows.append({"backend": b, "kind": kind, "used_usd": round(used, 5),
                     "tokens": toks, "free_usd": free, "remaining_usd": remaining,
                     "pct_used": pct, "note": note})

    def rank(x):
        if x["kind"] == "订阅":
            return (0, 0.0)                       # 订阅最优先（不花钱）
        if x["kind"] == "免费额度":
            return (1, -(x["remaining_usd"] or 0))  # 剩余免费额度多的优先
        return (2, x["used_usd"])                 # 付费的放最后
    rows.sort(key=rank)
    top = next((r["backend"] for r in rows
                if r["kind"] == "订阅"
                or (r["kind"] == "免费额度" and (r["remaining_usd"] or 0) > 0)), None)
    return {"rows": rows, "recommend": top, "gateway_ok": gateway_ok,
            "configured": bool(cfg)}


# --------------------------------------------------------------------------- #
# ② 模型评分报告（实际使用 + 用户 + 官网，透明加权）
# --------------------------------------------------------------------------- #
_W = {"actual": 0.5, "user": 0.2, "official": 0.3}


def score_report(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    stats = run_stats_by_backend(runs_dir)
    rates = ratings_by_backend()
    official = load_scores_config()
    rows = []
    for b in BACKEND_ORDER:
        s = stats.get(b)
        uses = s["uses"] if s else 0
        ok_rate = round(s["ok"] / s["uses"] * 100) if s and s["uses"] else None
        avg_lat = round(s["lat"] / s["uses"], 1) if s and s["uses"] else None
        avg_cost = round(s["cost"] / s["uses"], 5) if s and s["uses"] else None
        r = rates.get(b, {"up": 0, "down": 0})
        up, down = r["up"], r["down"]
        user_score = round(up / (up + down) * 100) if (up + down) else None
        oc = official.get(b, {}) if isinstance(official, dict) else {}
        off = oc.get("official")
        off = float(off) if isinstance(off, (int, float)) and off > 0 else None

        comps = {"actual": ok_rate, "user": user_score, "official": off}
        present = {k: v for k, v in comps.items() if v is not None}
        if present:
            wsum = sum(_W[k] for k in present)
            composite = round(sum(_W[k] * v for k, v in present.items()) / wsum)
        else:
            composite = None
        rows.append({"backend": b, "uses": uses, "ok_rate": ok_rate,
                     "avg_lat": avg_lat, "avg_cost": avg_cost, "up": up, "down": down,
                     "user_score": user_score, "official": off, "composite": composite})
    rows.sort(key=lambda x: (x["composite"] is not None, x["composite"] or 0), reverse=True)
    return {"rows": rows, "weights": _W,
            "has_official": any(r["official"] is not None for r in rows)}


# --------------------------------------------------------------------------- #
# ③ 额度预飞（quota_simulate）
# --------------------------------------------------------------------------- #
def quota_simulate(stages: list, base_url: str, key: str,
                   runs_dir=None) -> dict:
    """预判运行给定 stages 是否有足够额度。"""
    stage_costs = {}
    missing_stages = []
    try:
        report = stage_report(runs_dir)
        # stage_report 返回 {"rows": [...], "totals": {...}}；按 stage 键索引
        stages_data = ({r["stage"]: r for r in report.get("rows", [])}
                       if isinstance(report, dict) else {})
    except Exception:  # noqa: BLE001
        stages_data = {}

    for s in stages:
        info = stages_data.get(s, {})
        cost = info.get("avg_cost") if isinstance(info, dict) else None
        stage_costs[s] = cost
        if cost is None:
            missing_stages.append(s)

    estimated_total = float(sum(c for c in stage_costs.values() if c is not None))

    remaining_usd = None
    subscription = False
    quota_ok = False
    try:
        qr = quota_report(base_url, key)
        qrows = qr.get("rows", [])
        # 有任意订阅后端 → 不计费
        subscription = any(r.get("kind") == "订阅" for r in qrows)
        # 汇总所有免费额度的剩余（非 None 才计）
        free_rows = [r for r in qrows
                     if r.get("kind") == "免费额度" and r.get("remaining_usd") is not None]
        remaining_usd = sum(r["remaining_usd"] for r in free_rows) if free_rows else None
        quota_ok = True
    except Exception:  # noqa: BLE001
        pass

    if not quota_ok or missing_stages:
        verdict = "Unknown"
    elif subscription:
        verdict = "Safe"
    elif remaining_usd is None:
        verdict = "Safe"
    else:
        if estimated_total == 0:
            verdict = "Safe"
        elif estimated_total <= remaining_usd * 0.5:
            verdict = "Safe"
        elif estimated_total <= remaining_usd:
            verdict = "Risky"
        else:
            verdict = "Insufficient"

    return {
        "stages": stages,
        "stage_costs": stage_costs,
        "estimated_total": estimated_total,
        "remaining_usd": remaining_usd,
        "verdict": verdict,
        "missing_stages": missing_stages,
    }


# --------------------------------------------------------------------------- #
# ④ Model Fitness by task type
# --------------------------------------------------------------------------- #
def model_fitness(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """按 (backend, task_type) 分桶聚合历史成功率/均成本。

    数据来源：
      - run-log.md 每行 → backend / status / cost
      - 00-task.md → infer_task_type → task_type

    返回：
    {
      "rows": [{"backend", "task_type", "uses", "ok", "ok_rate", "avg_cost"}],
      "task_types": [...]   # 所有出现过的 task_type（排序）
    }
    """
    from devkit.tasktype import infer_task_type

    agg: dict = {}     # (backend, task_type) -> {uses, ok, cost}
    if not runs_dir.exists():
        return {"rows": [], "task_types": []}

    cmap = _carrier_map()
    for run_dir in runs_dir.iterdir():
        if not run_dir.is_dir():
            continue
        task_file = run_dir / "00-task.md"
        log_file = run_dir / "run-log.md"
        if not task_file.exists() or not log_file.exists():
            continue
        try:
            task_text = task_file.read_text(encoding="utf-8")
            task_type = infer_task_type(task_text)
            lines = log_file.read_text(encoding="utf-8").splitlines()
        except Exception:  # noqa: BLE001
            continue
        for line in lines:
            m = _ROW.match(line.strip())
            if not m:
                continue
            carrier, served, status, _dt, _toks, cost = m.groups()
            backend = resolve_backend(served, carrier, cmap)
            key = (backend, task_type)
            a = agg.setdefault(key, {"uses": 0, "ok": 0, "cost": 0.0})
            a["uses"] += 1
            a["ok"] += 1 if status == "OK" else 0
            a["cost"] += float(cost)

    rows = []
    for (backend, task_type), a in agg.items():
        uses = a["uses"]
        rows.append({
            "backend": backend,
            "task_type": task_type,
            "uses": uses,
            "runs": uses,   # alias for discover.from_fitness() compatibility
            "ok": a["ok"],
            "ok_rate": round(a["ok"] / uses * 100) if uses else None,
            "avg_cost": round(a["cost"] / uses, 5) if uses else None,
        })
    rows.sort(key=lambda x: (x["task_type"], -(x["ok_rate"] or 0)))
    task_types = sorted({r["task_type"] for r in rows})
    return {"rows": rows, "task_types": task_types}


# --------------------------------------------------------------------------- #
# ⑤ Model Recommend（基于历史 fitness 推荐 backend）
# --------------------------------------------------------------------------- #
def recommend_model(task: str,
                    runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> dict:
    """根据历史 model_fitness 数据推荐最适合当前任务的 backend。"""
    try:
        task_type = "unknown"
        try:
            from devkit.tasktype import infer_task_type
            task_type = infer_task_type(task)
        except Exception:  # noqa: BLE001
            pass
        try:
            rows = model_fitness(runs_dir).get("rows", [])
        except Exception:  # noqa: BLE001
            return {"task_type": task_type, "backend": None, "ok_rate": None,
                    "avg_cost": None, "uses": 0, "reason": "无历史数据"}
        candidates = [r for r in rows
                      if r.get("task_type") == task_type and r.get("uses", 0) >= 1]
        if not candidates:
            return {"task_type": task_type, "backend": None, "ok_rate": None,
                    "avg_cost": None, "uses": 0, "reason": "无历史数据"}
        best = max(candidates, key=lambda r: r.get("ok_rate") or -1)
        ok_rate = best.get("ok_rate")
        avg_cost = best.get("avg_cost")
        uses = best.get("uses", 0)
        backend = best.get("backend")
        reason = (f"历史 {uses} 次，成功率 {ok_rate}%，均 ${avg_cost:.5f}"
                  if avg_cost is not None else f"历史 {uses} 次，成功率 {ok_rate}%")
        return {"task_type": task_type, "backend": backend, "ok_rate": ok_rate,
                "avg_cost": avg_cost, "uses": uses, "reason": reason}
    except Exception:  # noqa: BLE001
        return {"task_type": "unknown", "backend": None, "ok_rate": None,
                "avg_cost": None, "uses": 0, "reason": "无历史数据"}


# --------------------------------------------------------------------------- #
# ⑥ Runs List（任务证据链视图）
# --------------------------------------------------------------------------- #
def runs_list(runs_dir: pathlib.Path = ROOT / "devkit" / "runs") -> list:
    """返回 runs 列表（按时间逆序），每项：
    {"run_id", "gate", "tokens", "cost", "task_type", "task_snippet", "artifact_files"}
    缺文件/解析失败的项自动跳过，不抛异常。
    """
    _TOK_COST = re.compile(r"\*\*(\d+)\s+tokens\s*·\s*\$([0-9.]+)\*\*")
    _GATE = re.compile(r"^(建议\s*GO[^\n]*|NO-GO[^\n]*)", re.MULTILINE)

    if not runs_dir.exists():
        return []

    items = []
    for run_dir in runs_dir.iterdir():
        if not run_dir.is_dir():
            continue
        run_id = run_dir.name
        gate = tokens = cost = task_type = task_snippet = None
        artifact_files: list = []

        try:
            log_text = (run_dir / "run-log.md").read_text(encoding="utf-8")
            m = _TOK_COST.search(log_text)
            if m:
                tokens = int(m.group(1))
                cost = float(m.group(2))
            gm = _GATE.search(log_text)
            gate = gm.group(1).strip() if gm else None
        except Exception:  # noqa: BLE001
            pass

        try:
            task_text = (run_dir / "00-task.md").read_text(encoding="utf-8")
            lines = [l.strip() for l in task_text.splitlines() if l.strip()
                     and not l.strip().startswith("#")]
            snippet = " ".join(lines)[:50]
            task_snippet = snippet or None
        except Exception:  # noqa: BLE001
            pass

        for af in sorted(run_dir.glob("*.artifact.json")):
            artifact_files.append(str(af))
            if task_type is None:
                try:
                    d = json.loads(af.read_text(encoding="utf-8"))
                    task_type = d.get("task_type")
                except Exception:  # noqa: BLE001
                    pass

        items.append({
            "run_id": run_id,
            "gate": gate,
            "tokens": tokens,
            "cost": cost,
            "task_type": task_type,
            "task_snippet": task_snippet,
            "artifact_files": artifact_files,
        })

    items.sort(key=lambda x: x["run_id"], reverse=True)
    return items
