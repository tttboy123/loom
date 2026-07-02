"""
Loom 研发流程套件（devkit）—— 用「角色模型载体」跑一遍 R&D Loop。

定位（重要）：这是【研发流程层】，与 Buddys【产品】解耦。
  - 它只负责"怎么开发"：产出 spec / plan / 代码草案 / 测试 / 独立审查。
  - 产品代码活在 Buddys 仓库；本套件不碰产品 runtime。
  - 默认 L2 / autonomous：可自治执行到工作树；如需只出报告，显式切到 report-only。

机制：每个阶段 = 一个角色 = 一个稳定载体名（loom-*），统一打 LiteLLM 网关 :4000。
换某阶段用的厂商：只改 litellm/config.full.yaml 的对应载体，本套件无需改动。
只依赖 Python 标准库（urllib），宿主机直接 `python -m devkit "任务"` 即可跑。
"""
from __future__ import annotations

import json
import os
import pathlib
import re
import socket
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from datetime import datetime
from typing import List, Optional, Tuple

from devkit.delivery_mode import display_label as _delivery_display_label
from devkit.delivery_mode import resolve_delivery_mode as _resolve_delivery_mode
from devkit.delivery_mode import resolved_targets as _resolved_delivery_targets
from devkit.model_aliases import normalize_model_name

ROOT = pathlib.Path(__file__).resolve().parent.parent  # agent-platform/


# --------------------------------------------------------------------------- #
# 阶段定义：角色 + 载体 + 角色契约（system prompt 直接编码 RD-LOOP 的 skill gate）
# --------------------------------------------------------------------------- #
# 关键：Claude Code / Codex 订阅经 CLIProxyAPI 暴露的是「带工具的编码 agent」人格，
# 直接当 chat 用时会吐出工具调用语法（<invoke ...>）当噪音。这里统一压制。
NO_TOOLS_PREAMBLE = (
    "你运行在一个**无工具**的纯文本编排里：没有任何工具 / 函数可调用，"
    "也不要假装去读文件或目录。**禁止输出任何工具调用 / 函数调用语法（如 <invoke>、function_calls 等）**。"
    "直接用 markdown 给出本阶段的最终产物（结论 + 必要代码块即可）。\n\n"
)


def _load_constitution() -> str:
    """读取全局宪章（Wayland 式平台层治理），注入每个角色的 system 提示。缺失则空。"""
    f = ROOT / "CONSTITUTION.md"
    if f.exists():
        return "【Loom 宪章 · 全局规则，每个角色必须遵守，与安全/诚实冲突时本宪章优先】\n" \
               + f.read_text() + "\n\n"
    return ""


CONSTITUTION = _load_constitution()


@dataclass(frozen=True)
class Stage:
    key: str        # 阶段键
    role: str       # 角色（文档用）
    carrier: str    # LiteLLM model_name（= config.full.yaml 的 loom-* 载体）
    title: str      # 产物标题
    system: str     # 角色契约
    executor: str = "chat"            # 本阶段执行器：chat | hermes | openclaw
    max_tokens: Optional[int] = None  # 本阶段单独的 token 上限（None=用 run 级默认）


STAGES: List[Stage] = [
    Stage(
        "brainstorm", "product", "loom-product", "产品判断 / Spec 方向",
        "你是 Loom 研发流程里的【产品逻辑】角色（brainstorming 阶段）。"
        "把需求转成清晰的产品判断：核心用户价值、范围与取舍、关键风险，"
        "并**显式列出需要人类确认的项（human gate）**。不要写代码。输出简洁 markdown。",
    ),
    Stage(
        "plan", "orchestrator", "loom-orchestrator", "实现计划 / 分派",
        "你是【编排】角色。基于上游产品判断，产出**实现计划**："
        "拆成最小可验证步骤；每步给 objective / inputs / output / done(验收) / boundaries(不做什么)；"
        "并指明该步该由哪个角色做。给出**测试先行**的 contract 清单。不要写实现代码。",
    ),
    Stage(
        "implement", "dev", "loom-dev", "TDD 实现草案",
        "你是【开发】角色，**必须 TDD**。先写**会失败的测试**（unit + contract，"
        "至少覆盖一个旗舰用例和一个非 happy-path），再写**最小实现**让其通过，最后给**验证命令**。"
        "用计划指定的语言/框架；测试与实现都用代码块。这是草案，不落真实仓库。",
    ),
    Stage(
        "verify", "tester", "loom-tester", "验证 / Eval",
        "你是【测试 / 验证】角色。基于上游实现设计并（以描述方式）执行 eval："
        "喂 ≥1 个**非 happy-path 的真实输入**；检查**未知输入是否被忠实回报**（不静默丢、不幻觉）；"
        "旗舰用例端到端是否真成立。逐条给 PASS/FAIL + 证据。",
    ),
    Stage(
        "review", "reviewer", "loom-reviewer", "独立审查",
        "你是【独立审查】角色，**不共享实现者的上下文与假设**。除了『安全 + 测试绿』，"
        "必须显式验证：旗舰用例对真人是否真成立、未知输入是否忠实回报、证据/trace 是否只含相关项。"
        "识别**假完成**（如永远报缺、静默丢弃）。最后给 **APPROVE** 或 **REQUEST-CHANGES** + 具体 findings。",
    ),
]
STAGES_BY_KEY = {s.key: s for s in STAGES}


# --------------------------------------------------------------------------- #
# 产出归一化：把「agentic 模型」（如 Claude Code 订阅人格）吐的工具调用语法
# 转成干净产物——write_file 的内容抽成代码块，read/list 这类"四处看"的噪音丢弃。
# --------------------------------------------------------------------------- #
def normalize(text: str) -> str:
    def _wf(m: "re.Match") -> str:
        path = (m.groupdict().get("path") or "").strip()
        body = (m.groupdict().get("body") or "").strip()
        head = f"**文件 `{path}`**\n" if path else ""
        return f"\n{head}````\n{body}\n````\n"

    # write_file：闭合的 <path>/<content> 形式
    text = re.sub(
        r"<write_file>\s*<path>(?P<path>.*?)</path>\s*<content>(?P<body>.*?)</content>\s*</write_file>",
        _wf, text, flags=re.DOTALL)
    # write_file：属性形式 <write_file path="X"> ... </write_file>
    text = re.sub(
        r'<write_file[^>]*path="(?P<path>[^"]*)"[^>]*>(?P<body>.*?)</write_file>',
        _wf, text, flags=re.DOTALL)
    # write_file：被 max_tokens 截断、未闭合的末尾块 —— 抓到结尾
    text = re.sub(
        r"<write_file>\s*(?:<path>(?P<path>.*?)</path>)?\s*<content>(?P<body>.*)\Z",
        _wf, text, flags=re.DOTALL)

    # 通用清理：agentic 模型会发明各种伪工具标签（list_files / recursive / read_file …）。
    # 只在【代码围栏之外】清理任意 <小写标签> 块/碎片，围栏内的真实代码原样保留。
    def _clean(seg: str) -> str:
        seg = re.sub(r"<([a-z][a-z0-9_]*)\b[^>]*>.*?</\1>", "", seg, flags=re.DOTALL)  # 成对伪标签块
        seg = re.sub(r"</?[a-z][a-z0-9_]*\b[^>]*>", "", seg)                            # 残留单标签
        return seg

    parts = re.split(r"(`{3,}[\s\S]*?`{3,})", text)  # 奇数段=代码围栏，保护不动
    text = "".join(s if i % 2 else _clean(s) for i, s in enumerate(parts))
    return re.sub(r"\n{3,}", "\n\n", text).strip()


# --------------------------------------------------------------------------- #
# 网关客户端（仅标准库）
# --------------------------------------------------------------------------- #
def load_master_key(env_path: pathlib.Path = ROOT / ".env") -> str:
    key = os.environ.get("LITELLM_MASTER_KEY", "")
    if not key and env_path.exists():
        for line in env_path.read_text().splitlines():
            if line.startswith("LITELLM_MASTER_KEY="):
                key = line.split("=", 1)[1].strip()
                break
    return key


def load_env_key(name: str, env_path: pathlib.Path = ROOT / ".env") -> str:
    value = os.environ.get(name, "")
    if not value and env_path.exists():
        for line in env_path.read_text().splitlines():
            if line.startswith(name + "="):
                value = line.split("=", 1)[1].strip()
                break
    return value


# --------------------------------------------------------------------------- #
# 精确响应缓存（借鉴调研报告 #1）：相同请求不重复计费。缓存逻辑在 gateway_chat 内，
# 真正 HTTP 在 _uncached_gateway_chat —— 这样单测仍可 monkeypatch rdloop.gateway_chat。
# --------------------------------------------------------------------------- #
import hashlib as _hashlib
import threading as _threading

CACHE_ENABLED = os.environ.get("LOOM_NO_CACHE") not in ("1", "true", "True")  # 进程级开关
CACHE_TTL_S = 7 * 86400              # 默认 7 天
CACHE_MAX_ROWS = 5000               # 硬上限，超了淘汰最旧
_CACHE_DB = ROOT / "devkit" / ".cache" / "gateway.db"
_cache_local = _threading.local()    # 每线程记 last hit，供 run_loop 标「缓存」
_MINIMAX_REASONING_ALIASES = {"minimax", "loom-dev", "loom-tester"}
_THINKING_TAG_RE = re.compile(r"(?is)<think(?:ing)?\b[^>]*>.*?</think(?:ing)?>")


def _config_mtime() -> str:
    """config.full.yaml 的 mtime —— 进键，remap 改了配置就自动失效缓存；缺文件返哨兵。"""
    try:
        return str(int((ROOT / "litellm" / "config.full.yaml").stat().st_mtime))
    except Exception:  # noqa: BLE001
        return "0"


def _cache_key(model: str, system: str, user: str, max_tokens: int) -> str:
    """键 = sha256(model, system, user, max_tokens, config-mtime, schema)。
    刻意不含 tags（带 run:<ts> 会让任何请求都不命中）、timeout、served（只在响应里）。"""
    from devkit.cache import SCHEMA
    h = _hashlib.sha256()
    for part in (model, system, user, str(max_tokens), _config_mtime(), SCHEMA):
        h.update(part.encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def _is_minimax_reasoning_model(model: str) -> bool:
    low = (model or "").strip().lower()
    if low in _MINIMAX_REASONING_ALIASES:
        return True
    return "minimax-m3" in low


def _apply_model_specific_request_fields(payload: dict, model: str) -> dict:
    if _is_minimax_reasoning_model(model):
        if "max_tokens" in payload and "max_completion_tokens" not in payload:
            payload["max_completion_tokens"] = payload.pop("max_tokens")
        payload["thinking"] = {"type": "disabled"}
        payload["reasoning_split"] = True
    return payload


def _resolve_chat_target(base_url: str, api_key: str, model: str) -> tuple[str, str, bool]:
    target_url = f"{base_url}/v1/chat/completions"
    target_key = api_key
    normalized_base = base_url.rstrip("/")
    if _is_minimax_reasoning_model(model) and normalized_base in {"http://localhost:4000", "http://127.0.0.1:4000"}:
        minimax_key = load_env_key("MINIMAX_API_KEY")
        if minimax_key:
            return "https://api.minimaxi.com/v1/chat/completions", minimax_key, True
    return target_url, target_key, False


def _resolve_request_model(model: str, *, is_direct_minimax: bool) -> str:
    if is_direct_minimax and _is_minimax_reasoning_model(model):
        return "MiniMax-M3"
    return model


def _request_json(url: str, api_key: str, payload: dict, timeout: int) -> tuple[dict, float]:
    body = json.dumps(payload).encode()
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        cost_hdr = r.headers.get("x-litellm-response-cost")
        data = json.loads(r.read())
    try:
        cost = float(cost_hdr) if cost_hdr else 0.0
    except (TypeError, ValueError):
        cost = 0.0
    return data, cost


_TRANSIENT_HTTP_CODES = {408, 409, 425, 429, 500, 502, 503, 504}


def _request_json_with_retry(url: str, api_key: str, payload: dict, timeout: int, attempts: int = 2) -> tuple[dict, float]:
    last_exc = None
    for idx in range(max(1, attempts)):
        try:
            return _request_json(url, api_key, payload, timeout)
        except urllib.error.HTTPError as exc:
            last_exc = exc
            if exc.code not in _TRANSIENT_HTTP_CODES or idx >= attempts - 1:
                raise
        except (urllib.error.URLError, TimeoutError, socket.timeout) as exc:
            last_exc = exc
            if idx >= attempts - 1:
                raise
        time.sleep(min(1.5, 0.25 * (idx + 1)))
    raise last_exc  # pragma: no cover


def _should_retry_final_text(extracted: dict) -> bool:
    return extracted.get("failure_code") in {"EMPTY_REASONING_ONLY", "EMPTY_REASONING_ONLY_LENGTH"}


def _build_final_text_retry_payload(payload: dict) -> dict:
    retried = json.loads(json.dumps(payload))
    msgs = retried.get("messages") or []
    if msgs and isinstance(msgs[0], dict) and msgs[0].get("role") == "system":
        msgs[0]["content"] = str(msgs[0].get("content", "")).rstrip() + (
            "\n\n只输出最终答案正文。不要输出思维过程、推理摘要、解释前言。"
        )
    else:
        msgs.insert(0, {
            "role": "system",
            "content": "只输出最终答案正文。不要输出思维过程、推理摘要、解释前言。",
        })
    if "max_completion_tokens" in retried:
        retried["max_completion_tokens"] = max(int(retried["max_completion_tokens"]), 256)
    elif "max_tokens" in retried:
        retried["max_tokens"] = max(int(retried["max_tokens"]), 256)
    retried.pop("metadata", None)
    return retried


def _should_continue_truncated_text(extracted: dict) -> bool:
    diag = extracted.get("diag") or {}
    if not extracted.get("normalized_text", "").strip():
        return False
    if diag.get("finish_reason") == "length":
        return True
    try:
        from devkit import apply as _apply
        proto = _apply.build_output_protocol(extracted["text"], response_diag=diag)
        return bool(proto.get("suggested_continue"))
    except Exception:
        return False


def _build_truncated_continuation_payload(payload: dict, partial_text: str) -> dict:
    retried = json.loads(json.dumps(payload))
    msgs = retried.get("messages") or []
    partial_tail = str(partial_text or "")[-1200:]
    msgs.append({
        "role": "user",
        "content": (
            "上一个回答因为长度被截断了。"
            "请从上次停止的位置继续输出，不要重写前文，不要解释，不要道歉。"
            "如果上一段正在输出文件内容，请先补完整个文件，再继续后续文件。\n\n"
            "上次回答末尾如下，请紧接着续写：\n"
            f"{partial_tail}"
        ),
    })
    retried["messages"] = msgs
    if "max_completion_tokens" in retried:
        retried["max_completion_tokens"] = max(int(retried["max_completion_tokens"]), 512)
    elif "max_tokens" in retried:
        retried["max_tokens"] = max(int(retried["max_tokens"]), 512)
    retried.pop("metadata", None)
    return retried


def _merge_continuation_text(existing: str, continuation: str) -> str:
    left = str(existing or "")
    right = str(continuation or "")
    if not left:
        return right
    if not right:
        return left
    max_overlap = min(len(left), len(right), 400)
    for size in range(max_overlap, 0, -1):
        if left[-size:] == right[:size]:
            return left + right[size:]
    if right in left:
        return left
    if left.endswith(("+", "-", "*", "/", "=", ":", ",")) and right[:1] and not right.startswith((" ", "\n", "\t", ".", ")", "]", "}")):
        return left + " " + right
    return left + right


_MAX_CONTINUATION_ROUNDS = 3


def _flatten_chat_content(value) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        out = []
        for item in value:
            if isinstance(item, str):
                out.append(item)
                continue
            if not isinstance(item, dict):
                continue
            text = item.get("text")
            if isinstance(text, str):
                out.append(text)
                continue
            if isinstance(text, dict):
                nested = text.get("value")
                if isinstance(nested, str):
                    out.append(nested)
                    continue
            if item.get("type") == "output_text":
                nested = item.get("content")
                if isinstance(nested, str):
                    out.append(nested)
                    continue
            nested = item.get("content")
            if isinstance(nested, str):
                out.append(nested)
        return "\n".join(s for s in out if s)
    if isinstance(value, dict):
        for key in ("content", "text", "value"):
            nested = value.get(key)
            if nested is not None:
                text = _flatten_chat_content(nested)
                if text.strip():
                    return text
    return ""


def _strip_thinking_tags(text: str) -> str:
    if not isinstance(text, str):
        return ""
    return _THINKING_TAG_RE.sub("", text).strip()


def _has_cacheable_text_content(text: str) -> bool:
    """仅缓存最终仍有可用文本的成功响应，避免空正文假成功被放大。"""
    return bool(normalize(_strip_thinking_tags(text)).strip())


def _reasoning_details_text(value) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        out = []
        for item in value:
            if isinstance(item, dict):
                txt = item.get("text")
                if isinstance(txt, str) and txt.strip():
                    out.append(txt)
        return "\n".join(out).strip()
    if isinstance(value, dict):
        txt = value.get("text")
        if isinstance(txt, str):
            return txt.strip()
    return ""


def _extract_responses_output_text(payload: dict) -> str:
    output = payload.get("output")
    if not isinstance(output, list):
        return ""
    parts = []
    for item in output:
        if not isinstance(item, dict):
            continue
        if item.get("type") in {"output_text", "text"}:
            text = _flatten_chat_content(item.get("text") or item.get("content") or item)
            if text.strip():
                parts.append(text)
            continue
        content = item.get("content")
        if isinstance(content, list):
            for block in content:
                if not isinstance(block, dict):
                    continue
                if block.get("type") in {"output_text", "text"}:
                    text = _flatten_chat_content(block.get("text") or block.get("content") or block)
                    if text.strip():
                        parts.append(text)
    return "\n".join(parts).strip()


def _extract_response_payload(payload: dict) -> dict:
    choices = payload.get("choices")
    choice = choices[0] if isinstance(choices, list) and choices and isinstance(choices[0], dict) else {}
    message = choice.get("message") if isinstance(choice.get("message"), dict) else {}
    provider_fields = message.get("provider_specific_fields") if isinstance(message.get("provider_specific_fields"), dict) else {}
    finish_reason = choice.get("finish_reason")
    response_message = message.get("response_message") if isinstance(message.get("response_message"), dict) else {}
    candidates = [
        message.get("content"),
        choice.get("response_message"),
        response_message,
        choice.get("output_text"),
        message.get("output_text"),
        payload.get("output_text"),
        _extract_responses_output_text(payload),
    ]
    raw_candidate_text = ""
    for candidate in candidates:
        text = _flatten_chat_content(candidate)
        if text.strip() and not raw_candidate_text:
            raw_candidate_text = text
        text = _strip_thinking_tags(text)
        if text:
            normalized = normalize(text)
            return {
                "text": text,
                "normalized_text": normalized,
                "failure_code": None if normalized.strip() else "EMPTY_TOOL_NOISE",
                "diag": {
                    "object": payload.get("object"),
                    "served_model": payload.get("model"),
                    "finish_reason": finish_reason,
                    "choices_count": len(choices) if isinstance(choices, list) else 0,
                    "has_message_content": bool(_flatten_chat_content(message.get("content")).strip()),
                    "has_response_message": bool(response_message),
                    "has_output_text": bool(_extract_responses_output_text(payload).strip() or _flatten_chat_content(payload.get("output_text")).strip()),
                    "has_reasoning_content": bool(_flatten_chat_content(message.get("reasoning_content")).strip()
                                                 or _flatten_chat_content(provider_fields.get("reasoning_content")).strip()),
                    "has_reasoning_details": bool(_reasoning_details_text(message.get("reasoning_details"))
                                                  or _reasoning_details_text(response_message.get("reasoning_details"))),
                    "raw_text_len": len(text),
                    "normalized_text_len": len(normalized.strip()),
                },
            }

    reasoning_text = "\n".join(filter(None, [
        _flatten_chat_content(message.get("reasoning_content")),
        _flatten_chat_content(provider_fields.get("reasoning_content")),
        _reasoning_details_text(message.get("reasoning_details")),
        _reasoning_details_text(response_message.get("reasoning_details")),
    ])).strip()
    has_choices = isinstance(choices, list) and bool(choices)
    has_responses_output = bool(_extract_responses_output_text(payload).strip())
    if not has_choices:
        failure_code = "EMPTY_NO_CHOICES"
    elif reasoning_text and not raw_candidate_text:
        failure_code = "EMPTY_REASONING_ONLY_LENGTH" if finish_reason == "length" else "EMPTY_REASONING_ONLY"
    elif payload.get("object") == "response" and not has_responses_output:
        failure_code = "EMPTY_RESPONSES_NO_OUTPUT"
    else:
        failure_code = "EMPTY_NO_TEXT_FIELDS"
    return {
        "text": "",
        "normalized_text": "",
        "failure_code": failure_code,
        "diag": {
            "object": payload.get("object"),
            "served_model": payload.get("model"),
            "finish_reason": finish_reason,
            "choices_count": len(choices) if isinstance(choices, list) else 0,
            "has_message_content": bool(_flatten_chat_content(message.get("content")).strip()),
            "has_response_message": bool(response_message),
            "has_output_text": bool(has_responses_output or _flatten_chat_content(payload.get("output_text")).strip()),
            "has_reasoning_content": bool(reasoning_text),
            "has_reasoning_details": bool(_reasoning_details_text(message.get("reasoning_details"))
                                          or _reasoning_details_text(response_message.get("reasoning_details"))),
            "raw_text_len": 0,
            "normalized_text_len": 0,
        },
    }


def _extract_chat_completion_text(payload: dict) -> str:
    return _extract_response_payload(payload)["text"]


def gateway_chat(
    base_url: str, api_key: str, model: str, system: str, user: str,
    max_tokens: int = 900, timeout: int = 180, tags: Optional[list] = None,
    _db=None,
) -> Tuple[bool, str, str, int, float]:
    """带精确缓存的网关调用。命中返 (True, content, served, 0, 0.0)（计费为 0，预算/台账依然正确）。
    缓存可用 CACHE_ENABLED=False 或 LOOM_NO_CACHE=1 关。_db 仅供测试注入。"""
    _cache_local.hit = False
    _cache_local.response_diag = None
    _cache_local.failure_code = None
    if not CACHE_ENABLED:
        return _uncached_gateway_chat(base_url, api_key, model, system, user, max_tokens, timeout, tags)
    from devkit import cache as _cache
    db = _CACHE_DB if _db is None else _db
    key = _cache_key(model, system, user, max_tokens)
    got = _cache.get(db, key, CACHE_TTL_S)
    if got is not None:                         # 命中：免费返回
        if not _has_cacheable_text_content(got.get("content", "")):
            _cache.delete(db, key)
        else:
            _cache_local.hit = True
            _cache_local.response_diag = {"cache_hit": True, "served_model": got.get("served", model)}
            _cache_local.failure_code = None
            return True, got.get("content", ""), got.get("served", model), 0, 0.0
    ok, content, served, tokens, cost = _uncached_gateway_chat(
        base_url, api_key, model, system, user, max_tokens, timeout, tags)
    if ok and _has_cacheable_text_content(content):   # 只缓存成功且最终有正文的结果
        _cache.put(db, key, {"content": content, "served": served}, CACHE_MAX_ROWS)
    return ok, content, served, tokens, cost


def _uncached_gateway_chat(
    base_url: str, api_key: str, model: str, system: str, user: str,
    max_tokens: int = 900, timeout: int = 180, tags: Optional[list] = None,
) -> Tuple[bool, str, str, int, float]:
    """返回 (ok, content_or_error, actual_model_served, total_tokens, cost_usd)。

    tokens 取响应体 usage；cost 取 LiteLLM 的 x-litellm-response-cost 响应头
    （订阅后端无单价→$0，API 后端为真实花费）。"""
    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": system},
            {"role": "user", "content": user},
        ],
        "max_tokens": max_tokens,
    }
    payload = _apply_model_specific_request_fields(payload, model)
    request_url, request_key, is_direct_minimax = _resolve_chat_target(base_url, api_key, model)
    payload["model"] = _resolve_request_model(model, is_direct_minimax=is_direct_minimax)
    if tags and not is_direct_minimax:
        payload["metadata"] = {"tags": tags}   # LiteLLM 日志按 run/stage 过滤
    try:
        d, cost = _request_json_with_retry(request_url, request_key, payload, timeout)
        tokens = int((d.get("usage") or {}).get("total_tokens", 0) or 0)
        extracted = _extract_response_payload(d)
        if _should_retry_final_text(extracted):
            retry_payload = _build_final_text_retry_payload(payload)
            d2, cost2 = _request_json_with_retry(request_url, request_key, retry_payload, timeout)
            extracted2 = _extract_response_payload(d2)
            if extracted2.get("normalized_text", "").strip():
                usage2 = int((d2.get("usage") or {}).get("total_tokens", 0) or 0)
                diag2 = dict(extracted2.get("diag") or {})
                diag2["retry_attempted"] = True
                diag2["retry_reason"] = extracted.get("failure_code")
                _cache_local.response_diag = diag2
                _cache_local.failure_code = extracted2.get("failure_code")
                return True, extracted2["text"], d2.get("model", model), tokens + usage2, cost + cost2
            diag = dict(extracted.get("diag") or {})
            diag["retry_attempted"] = True
            diag["retry_reason"] = extracted.get("failure_code")
            diag["retry_failed_code"] = extracted2.get("failure_code")
            extracted["diag"] = diag
            extracted["failure_code"] = f"{extracted.get('failure_code')}_RETRY_EMPTY"
        continuation_rounds = 0
        while _should_continue_truncated_text(extracted) and continuation_rounds < _MAX_CONTINUATION_ROUNDS:
            continuation_rounds += 1
            d2, cost2 = _request_json_with_retry(
                request_url,
                request_key,
                _build_truncated_continuation_payload(payload, extracted["text"]),
                timeout,
            )
            extracted2 = _extract_response_payload(d2)
            merged = _merge_continuation_text(extracted["text"], extracted2.get("text", ""))
            merged_normalized = normalize(merged)
            diag = dict(extracted.get("diag") or {})
            diag["continuation_attempted"] = True
            diag["continuation_rounds"] = continuation_rounds
            diag["continuation_finish_reason"] = (extracted2.get("diag") or {}).get("finish_reason")
            diag["continued_text_len"] = len(extracted2.get("text", ""))
            if not merged_normalized.strip():
                diag["continuation_failed_code"] = extracted2.get("failure_code")
                extracted["diag"] = diag
                break
            tokens += int((d2.get("usage") or {}).get("total_tokens", 0) or 0)
            cost += cost2
            extracted["text"] = merged
            extracted["normalized_text"] = merged_normalized
            diag["raw_text_len"] = len(merged)
            diag["normalized_text_len"] = len(merged_normalized.strip())
            next_diag = dict(extracted2.get("diag") or {})
            diag["finish_reason"] = next_diag.get("finish_reason")
            extracted["diag"] = diag
            if not _should_continue_truncated_text(extracted):
                _cache_local.response_diag = diag
                _cache_local.failure_code = None
                return True, merged, d2.get("model", d.get("model", model)), tokens, cost
        content = extracted["text"]
        _cache_local.response_diag = extracted.get("diag")
        _cache_local.failure_code = extracted.get("failure_code")
        return True, content, d.get("model", model), tokens, cost
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")[:300]
        _cache_local.response_diag = {"http_error": e.code, "served_model": model}
        _cache_local.failure_code = f"HTTP_{e.code}"
        return False, f"HTTP {e.code}: {detail}", model, 0, 0.0
    except Exception as e:  # noqa: BLE001
        _cache_local.response_diag = {"exception": type(e).__name__, "served_model": model}
        _cache_local.failure_code = f"EXC_{type(e).__name__.upper()}"
        return False, f"{type(e).__name__}: {e}", model, 0, 0.0


# --------------------------------------------------------------------------- #
# 上下文压缩（compact 指针，借鉴 Kode）：长上游产物先用便宜模型压成要点，
# 再喂给下游阶段——省 token、保关键信号，替代粗暴截断。每件产物只压一次。
# --------------------------------------------------------------------------- #
COMPACT_THRESHOLD = 2500   # 超过这个字符数才压缩
COMPACT_MAX_TOKENS = 480

_COMPACT_SYS = (
    "你是上下文压缩器。把下面的产物压成不超过 400 字的要点，"
    "保留：关键结论 / 接口与数据结构 / 约束与取舍 / 待确认项 / 已知风险。"
    "丢弃：寒暄、重复、过程描述。直接给要点，不要前言。"
)


def compact_text(text: str, base_url: str, api_key: str, model: str,
                 tags: Optional[List[str]] = None) -> Tuple[str, int, float]:
    """把一段长文压成要点。返回 (摘要, tokens, cost)。失败则回退截断，零成本。"""
    ok, content, _served, tokens, cost = gateway_chat(
        base_url, api_key, model, _COMPACT_SYS, text[:8000],
        COMPACT_MAX_TOKENS, tags=tags or ["devkit:compact"])
    if ok and content.strip():
        return content.strip(), tokens, cost
    return text[:3500], 0, 0.0


def _safe_write_text(path: pathlib.Path, content: str, encoding: str = "utf-8") -> None:
    """Best-effort file write for long-running loops.

    A concurrent archiver/sweeper may remove the run/build directory between
    stage execution and artifact persistence. Recreate the parent on demand so
    the loop degrades to a warning instead of aborting the whole run.
    """
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding=encoding)


# --------------------------------------------------------------------------- #
# 迭代循环辅助（借鉴 Anthropic「Planner→Generator→Evaluator」长跑 harness）：
# 评判者 NO-GO → 把失败 critique 回灌构建者重做 → 重测重评，直到通过或达上限。
# --------------------------------------------------------------------------- #
def _exec_stage(st: "Stage", user: str, carrier: str, executor: str, stage_max: int,
                base_url: str, api_key: str, ts: str, run_dir: pathlib.Path,
                os_sandbox: bool, suffix: str = ""):
    """执行一个阶段（chat 走网关 / 否则走 agentic 执行器），返回 (ok, content, served, tokens, cost, dt)。"""
    system = NO_TOOLS_PREAMBLE + CONSTITUTION + st.system
    t0 = time.time()
    cached = False
    if executor == "chat":
        ok, content, served, tokens, cost = gateway_chat(
            base_url, api_key, carrier, system, user, stage_max,
            tags=[f"run:{ts}", f"stage:{st.key}", "iterate"])
        cached = getattr(_cache_local, "hit", False)
        if ok:
            content = normalize(content)
    else:
        from devkit import executors
        sandbox = executors.sandbox_dir(run_dir, st.key + suffix)
        ok, content, _ex = executors.run(executor, system + "\n\n" + user, carrier,
                                         sandbox, base_url, api_key, os_sandbox=os_sandbox)
        served, tokens, cost = f"{executor}({carrier})", 0, 0.0
    return ok, content, served, tokens, cost, time.time() - t0


def _materialize_test(impl_text: str, build_dir: pathlib.Path, golden):
    """物化 implement 产物 + 硬 gate + 跑测试(+golden)。"""
    from devkit import apply as _apply
    files = _apply.materialize(impl_text, build_dir)
    _ensure_runtime_dirs(build_dir)
    materialization = _apply.diagnose_materialization(impl_text, files)
    collect = _apply.collect_tests(build_dir) if files else {
        "ok": False,
        "runner": None,
        "collected": 0,
        "output": "（未识别到代码文件，跳过 collect）",
        "failure_code": materialization.get("failure_code") or "MATERIALIZE_NO_FILES",
    }
    if not files:
        tpassed = False
        tout = collect["output"]
    elif collect.get("failure_code"):
        tpassed = False
        tout = collect.get("output", "")
    else:
        tpassed, tout = _apply.run_tests(build_dir)
    _safe_write_text(build_dir / "_test-output.txt", tout)
    tests_failed = (tpassed is False) or bool(collect.get("failure_code"))
    eval_sum = ""
    if golden and not tests_failed:
        from devkit import evals as _evals
        eval_ok, eval_sum = _evals.run_golden(build_dir, golden)
        tests_failed = tests_failed or (not eval_ok)
    return tests_failed, tout, files, eval_sum, materialization, collect


_REPORT_ONLY_EXTS = {".md", ".markdown", ".txt", ".json", ".html", ".csv", ".yaml", ".yml"}
_CODE_EXTS = {".py", ".js", ".jsx", ".ts", ".tsx", ".go", ".rs", ".sh", ".bash"}
_REPORT_SIGNALS = (
    "只输出", "纯 .md 输出", "纯.md输出", "审计报告", "诊断报告", "markdown", "report only", "report-only",
    "只读", "read-only", "run-log.md", "diagnostic", "diagnose", "定位为什么", "完整 traceback",
)


def _task_prefers_report_only(task: str) -> bool:
    lowered = (task or "").lower()
    return (
        any(sig in task for sig in _REPORT_SIGNALS if not sig.isascii())
        or any(sig in lowered for sig in _REPORT_SIGNALS if sig.isascii())
    )


def _is_report_only_artifact(task: str, files: List[str]) -> bool:
    """报告/诊断类任务只要求产物，不强制 pytest collect。"""
    if not files:
        return False
    exts = {pathlib.Path(f).suffix.lower() for f in files}
    if any(name.endswith("_test.py") or os.path.basename(name).startswith("test_") for name in files):
        return False
    has_code_files = any(ext in _CODE_EXTS for ext in exts)
    if exts and exts.issubset(_REPORT_ONLY_EXTS):
        return True
    code_files = [f for f in files if pathlib.Path(f).suffix.lower() in _CODE_EXTS]
    code_files_under_runs = bool(code_files) and all(
        pathlib.PurePosixPath(f.replace("\\", "/")).parts[:1] == ("runs",) for f in code_files
    )
    if has_code_files:
        return code_files_under_runs and _task_prefers_report_only(task)
    return _task_prefers_report_only(task)


def _applylock_blocks_success(task: str, report_only_artifact: bool, locked_files: List[str]) -> bool:
    """区分“需人工 apply”与“必须判 NO-GO”的锁文件。"""
    if not locked_files:
        return False
    for path in locked_files:
        base = os.path.basename(path)
        if base in {"rdloop.py", "evals.py", "evidence.py", "ratchet.py", "stopcheck.py", "applylock.py"}:
            return True
        if path.endswith(".golden.json"):
            return True
        if base.startswith("test_") and base.endswith(".py"):
            if report_only_artifact or _task_prefers_report_only(task):
                continue
            return True
    return False


def _blocked_retry_hint(reason: str) -> str:
    lowered = (reason or "").lower()
    if "api key" in lowered or "auth" in lowered or "401" in lowered:
        return "检查 provider key 或登录态"
    if "未知执行器" in reason or "unknown executor" in lowered:
        return "修正 executor 配置"
    if "命令未找到" in reason or "not found" in lowered or "未安装" in reason:
        return "检查本地依赖或命令安装"
    if "超时" in reason or "timeout" in lowered:
        return "检查命令/模型可用性或提高超时"
    if "空正文" in reason or "empty" in lowered:
        return "检查模型返回结构或空响应兜底"
    return ""


def _ensure_runtime_dirs(build_dir: pathlib.Path) -> None:
    """给诊断/探针脚本预建常见运行时输出目录。"""
    (build_dir / "runs").mkdir(parents=True, exist_ok=True)


def _cascade_rounds(cascade: list, iterate: int) -> int:
    """cascade 蕴含 iterate：默认轮数=阶梯长-1；用户 --iterate 可压低、不可拔高（无更高档）。"""
    if not cascade:
        return iterate
    d = len(cascade) - 1
    return min(iterate, d) if iterate else d


def _cascade_carrier(cascade: list, round_idx: int, fallback: str) -> str:
    """第 round_idx 轮的 generator 载体：cascade[min(idx, 末档)]；无 cascade 则用 fallback。"""
    return cascade[min(round_idx, len(cascade) - 1)] if cascade else fallback


def _wants_changes(review_text: str) -> bool:
    """评判者是否要求改（找 REQUEST-CHANGES；找不到判定不阻塞，只由测试 gate）。"""
    up = (review_text or "").upper()
    return "REQUEST-CHANGES" in up or "REQUESTCHANGES" in up.replace(" ", "").replace("-", "")


def _fail_detail(tout: str, eval_sum: str) -> str:
    """把失败信号拼给构建者看：golden 明细（含 want=）优先，单测输出其次。"""
    parts = []
    if eval_sum:
        parts.append("### Golden 评测明细（注意每条的 want= 期望值）\n" + eval_sum)
    if tout and "跳过测试" not in tout:
        parts.append(f"### 测试输出\n```\n{tout[:1200]}\n```")
    return "\n\n".join(parts)


def _build_feedback(fail_detail: str, review_text: str) -> str:
    fb = "## 上一轮未通过，请做最小修复\n\n"
    if fail_detail:
        fb += fail_detail + "\n\n"
    if review_text:
        fb += f"### 评判者意见（REQUEST-CHANGES）\n{review_text[:1500]}\n\n"
    fb += ("请基于上一版实现，仅做让**测试 / Eval 全部通过**且满足评判意见的最小改动；"
           "严格按 Golden 的 want= 对齐行为。给出修复后的完整文件（同样用代码块、文件名不变）。")
    return fb


# --------------------------------------------------------------------------- #
# Loop 运行器（轻量状态机）
# --------------------------------------------------------------------------- #
def run_loop(
    task: str,
    stages: Optional[List[Stage]] = None,
    base_url: Optional[str] = None,
    out_root: pathlib.Path = ROOT / "devkit" / "runs",
    max_tokens: int = 900,
    carrier_overrides: Optional[dict] = None,
    executor_map: Optional[dict] = None,
    task_kind: Optional[str] = None,
    allowed_artifact_paths: Optional[list[str]] = None,
    forbidden_artifact_paths: Optional[list[str]] = None,
    delivery_mode: Optional[str] = None,
    apply_target: Optional[str] = None,
    apply_git: Optional[str] = None,
    apply_branch: Optional[str] = None,
    run_id: Optional[str] = None,
    golden: Optional[str] = None,
    os_sandbox: bool = False,
    compact_model: Optional[str] = "deepseek",
    budget: Optional[float] = None,
    iterate: int = 0,
    contract: int = 0,
    contract_rounds: int = 0,
    cascade: Optional[list] = None,
    blind_review: bool = False,
    physical_verify: bool = False,
    health_probe: bool = False,
) -> dict:
    stages = stages or STAGES
    carrier_overrides = dict(carrier_overrides or {})   # 拷贝：cascade 会就地改 implement，别污染调用方
    executor_map = executor_map or {}  # stage_key -> chat|hermes|openclaw（默认 chat）
    cascade = [c.strip() for c in (cascade or []) if c.strip()]  # cheap→strong 升级阶梯
    if cascade:                                          # cascade：初始 implement 用最便宜档；蕴含 iterate
        carrier_overrides["implement"] = cascade[0]
        iterate = _cascade_rounds(cascade, iterate)
    base_url = base_url or os.environ.get("LITELLM_BASE_URL", "http://localhost:4000")
    api_key = load_master_key()
    if not api_key:
        raise SystemExit("找不到 LITELLM_MASTER_KEY（设环境变量或填 agent-platform/.env）")

    ts = run_id or datetime.now().strftime("%Y%m%d-%H%M%S")  # 控制台可指定 ts，便于实时跟踪
    run_dir = out_root / ts
    run_dir.mkdir(parents=True, exist_ok=True)
    _safe_write_text(run_dir / "00-task.md", f"# 任务\n\n{task}\n")

    # 断点续跑：跳过已完成阶段，从中断处继续
    from devkit import resume as _resume
    _all_stage_keys = [s.key for s in stages]
    _pending_keys = _resume.pending_stages(run_dir, _all_stage_keys)
    if len(_pending_keys) < len(_all_stage_keys):
        _skipped = [k for k in _all_stage_keys if k not in _pending_keys]
        print(f"  ⏭ 断点续跑：跳过已完成阶段 [{', '.join(_skipped)}]")
        stages = [s for s in stages if s.key in _pending_keys]

    artifacts: List[Tuple[Stage, str]] = []
    compacted: dict = {}      # stage.key -> 压缩后的要点（长产物只压一次，跨阶段复用）
    log_rows: List[str] = []
    stage_meta: List[Tuple[str, str, str, str]] = []  # (stage, carrier, 实际模型, 状态)
    blocked_details: dict[str, str] = {}
    tot_tokens, tot_cost = 0, 0.0
    over_budget = False
    contract_done = False         # Sprint Contract 只在 implement 前生成一次
    from devkit import memory as _memory
    from devkit.tasktype import infer_task_type as _infer_task_type
    _task_type = _infer_task_type(task)
    _delivery_mode = _resolve_delivery_mode(
        delivery_mode=delivery_mode,
        apply_target=apply_target,
        apply_git=apply_git,
    )
    apply_target, apply_git = _resolved_delivery_targets(
        mode=_delivery_mode,
        repo_root=ROOT,
        apply_target=apply_target,
        apply_git=apply_git,
    )
    from devkit import task_contract as _task_contract
    _contract = _task_contract.build_task_contract(
        task,
        delivery_mode=_delivery_mode,
        task_kind=task_kind,
        allowed_artifact_paths=allowed_artifact_paths,
        forbidden_artifact_paths=forbidden_artifact_paths,
    )
    _impl_artifact_path: Optional[pathlib.Path] = None  # 指向 implement 阶段的 artifact JSON，供事后回填
    _impl_response_diag = None
    _codex_verify_failed = False  # codex executor 明确 NO-GO → 传播到 gate（不被 build/test 重置覆盖）
    mem_ctx = _memory.recall()  # 过往教训，注入 brainstorm/plan
    print(f"▶ Loom R&D Loop  run={ts}  stages={'→'.join(s.key for s in stages)}"
          + f"  mode={_delivery_mode}"
          + (f"  预算=${budget:.4f}" if budget else "") + "\n")

    # 容量预检：当设定 --budget 时预估成本并提前警告
    if budget is not None:
        from devkit import capacity as _capacity
        _cap_cm = {s.key: carrier_overrides.get(s.key, s.carrier) for s in stages}
        _cap_r = _capacity.preflight_check([s.key for s in stages], _cap_cm, [], budget)
        if not _cap_r["ok"]:
            print(f"  ⚠ 容量预检：{_cap_r['warning']}")
            _cheaper = _capacity.suggest_cheaper([s.key for s in stages], _cap_cm, [], budget)
            if _cheaper:
                print(f"  💡 建议删减可选阶段：{', '.join(_cheaper)}")
        elif _cap_r.get("warning"):
            print(f"  ℹ 容量预检：{_cap_r['warning']}")

    # 加载 carrier 健康缓存 + bench 历史（run 级别加载一次）
    from devkit import carrier_health as _ch, carrier_router as _cr, carrier_bench as _cbench
    if health_probe:
        _all_carriers = list({
            carrier_overrides.get(s.key, s.carrier) if not isinstance(
                carrier_overrides.get(s.key, s.carrier), list)
            else carrier_overrides.get(s.key, s.carrier)[0]
            for s in stages
        })
        print(f"  🔍 健康探针：探测 {_all_carriers} …")
        _probe_results = _ch.probe_all(_all_carriers, base_url, api_key)
        _ch.save_cache(_probe_results)
        _health_cache = _probe_results
    else:
        _health_cache = _ch.load_cache()
    _healthy_set: set = set(_ch.healthy_carriers(_health_cache)) if _health_cache else set()
    _bench_rows: list = _cbench.bench_to_history_rows(_cbench.load_results())

    for i, st in enumerate(stages, 1):
        # 窗口感知打包（替换写死的 txt[:3500]，按 carrier 窗口比例分配；受保护字段不被截）
        from devkit import blocks as _blocks, budget as _budget
        _carrier_raw = carrier_overrides.get(st.key, st.carrier)
        if isinstance(_carrier_raw, list):
            _carrier_raw = [normalize_model_name(c, stage=st.key) for c in _carrier_raw]
        else:
            _carrier_raw = normalize_model_name(_carrier_raw, stage=st.key)
        # 多 carrier 列表 → 用 carrier_router 按历史 ok_rate/成本选最优
        if isinstance(_carrier_raw, list) and _carrier_raw:
            _candidates = ([c for c in _carrier_raw if c in _healthy_set]
                           if _healthy_set else _carrier_raw) or _carrier_raw
            _carrier_now = _cr.select(st.key, _candidates, _bench_rows, _task_type)
            if len(_candidates) > 1:
                print(f"  🔀 carrier 路由：{st.key} 候选={_candidates} → 选中 {_carrier_now}")
        else:
            _carrier_now = _carrier_raw
        _win = _budget.carrier_window(_carrier_now)
        _bud = _budget.budget_tokens(_win)
        _mem = mem_ctx if st.key in ("brainstorm", "plan") else ""
        _task_block = (_mem + "\n\n" if _mem else "") + f"## 开发任务\n{task}"
        if artifacts:
            _task_block += "\n\n## 已有上游产物"
        # blind_review（T15）：review 阶段只看任务 + 测试输出，不看实现（防止评审者顺着实现走）
        _blind = blind_review and st.key == "review"
        _ups = [
            (a.key, f"### 上游产物：{a.title}（{a.role}）\n{compacted.get(a.key, txt)}")
            for a, txt in artifacts
            if not (_blind and a.key == "implement")
        ]
        if _blind:
            _task_block += "\n\n[盲审模式：实现代码已屏蔽，请仅基于任务规格 + 测试输出作评判]"
        _blks = _blocks.build_blocks(task=_task_block, system="", upstreams=_ups)
        _pack = _budget.pack(_blks, _bud)
        user = _pack["text"].lstrip("\n") + "\n\n请产出本阶段产物。"
        if _pack["dropped"]:
            print(f"  📦 上下文打包：保留 [{', '.join(_pack['kept'])}]"
                  f" · 丢弃 [{', '.join(_pack['dropped'])}] · 估用 {_pack['used']} tok / 窗口 {_win} tok")

        # Sprint Contract：implement 前，评判者先约定可机器验证的验收点 → 注入构建者 + 作为 Eval Gate
        if contract and st.key == "implement" and not contract_done:
            contract_done = True
            from devkit import contract as _contract
            plan_txt = next((t for s, t in artifacts if s.key == "plan"), "") or task
            ccarrier = (carrier_overrides.get("review")
                        or next((s.carrier for s in stages if s.key == "review"), "loom-reviewer"))
            if contract_rounds > 0:
                # 构建者载体 = implement 阶段载体（与评判者跨载体，保 GAN 独立性）
                bcarrier = carrier_overrides.get("implement", st.carrier)
                if bcarrier == ccarrier:                 # 同载体 → 独立性退化，明确告警
                    print(f"  ⚠ 合同协商：构建者与评判者载体相同（{bcarrier}），独立性退化"
                          f"——建议给 review 用跨厂商载体")
                ccases, ctk, cco, _craw = _contract.negotiate_rounds(
                    task, plan_txt, base_url, api_key, ccarrier, bcarrier, contract, contract_rounds)
            else:
                ccases, ctk, cco, _craw = _contract.negotiate(
                    task, plan_txt, base_url, api_key, ccarrier, contract)
            tot_tokens += ctk
            tot_cost += cco
            if ccases:
                _safe_write_text(
                    run_dir / "contract.json",
                    json.dumps(ccases, ensure_ascii=False, indent=2),
                )
                golden = golden or str(run_dir / "contract.json")  # 用户没手填 golden 才用合同
                user += "\n\n" + _contract.to_block(ccases)
                _neg = f" · 构建者协商{contract_rounds}轮" if contract_rounds > 0 else ""
                print(f"  ▸ Sprint Contract：评判者({ccarrier}) 约定 {len(ccases)} 条验收点{_neg}  +{ctk}tok ${cco:.5f}")
            else:
                print("  ▸ Sprint Contract：未能解析出验收用例（跳过，本轮无自动 Eval Gate）")

        carrier = _carrier_now  # 已由 carrier_router 选好（或 override 直接指定）
        # 执行器：运行时 --executor 覆盖 > 角色文件里写的 > chat
        executor = executor_map.get(st.key) or getattr(st, "executor", None) or "chat"
        stage_max = getattr(st, "max_tokens", None) or _budget.carrier_max_tokens(carrier) or max_tokens  # 角色>per-carrier(推理模型)>run 级默认
        t0 = time.time()
        system = NO_TOOLS_PREAMBLE + CONSTITUTION + st.system  # 全局宪章 + 角色专属规则
        # 控制面 codex-sub 长尾较重：优先缩短首跳并尽快切 fallback；
        # 执行面 MiniMax 仍保留较长预算。
        _dispatch_timeout = 600 if "minimax" in carrier.lower() else 180
        cached = False
        if executor == "chat":
            if carrier in {"codex-sub", "loom-product", "loom-orchestrator", "loom-reviewer"}:
                from devkit.ask import ask_one_with_fallback
                resp = ask_one_with_fallback(
                    [carrier], user, base_url, api_key,
                    max_tokens=stage_max,
                    tag=f"stage:{st.key}",
                    timeout=None,
                    extra_tags=[f"run:{ts}", "rdloop"],
                )
                ok = bool(resp.get("ok"))
                content = resp.get("content") if ok else resp.get("error", "")
                served = resp.get("served", resp.get("model", carrier))
                tokens = int(resp.get("tokens", 0) or 0)
                cost = float(resp.get("cost", 0.0) or 0.0)
                cached = getattr(_cache_local, "hit", False)
                response_diag = {
                    "served_model": served,
                    "attempted_models": resp.get("attempted_models", [carrier]),
                    "fallback_used": len(resp.get("attempted_models", [])) > 1,
                }
                failure_code = resp.get("failure_code")
            else:
                ok, content, served, tokens, cost = gateway_chat(
                    base_url, api_key, carrier, system, user, stage_max,
                    timeout=_dispatch_timeout,
                    tags=[f"run:{ts}", f"stage:{st.key}"])
                cached = getattr(_cache_local, "hit", False)
                response_diag = getattr(_cache_local, "response_diag", None)
                failure_code = getattr(_cache_local, "failure_code", None)
        else:
            from devkit import executors  # 延迟导入，避免包初始化期循环
            sandbox = executors.sandbox_dir(run_dir, st.key)
            ok, content, _ex = executors.run(executor, system + "\n\n" + user, carrier, sandbox,
                                             base_url, api_key, os_sandbox=os_sandbox)
            served, tokens, cost = f"{executor}({carrier})", 0, 0.0
            response_diag = {"executor": executor, "served_model": served}
            failure_code = None
        dt = time.time() - t0
        tot_tokens += tokens
        tot_cost += cost
        if ok and executor == "chat":
            content = normalize(content)
            # 空正文对 loop 没有可执行价值；显式判 BLOCKED，避免 0tok/缓存 命中时出现假 OK。
            if not content.strip():
                ok = False
                if cached:
                    failure_code = "EMPTY_CACHE_HIT"
                elif not failure_code:
                    failure_code = "EMPTY_NORMALIZED_TEXT"
                reason = "缓存命中为空正文" if cached else "返回空正文"
                content = (f"[空内容兜底] 载体 {carrier} {reason}（served={served}, "
                           f"tokens={tokens}, cost=${cost:.5f}, code={failure_code}）")
        status = "OK" if ok else "BLOCKED"
        served_note = (f"  实际={served}  {tokens}tok ${cost:.5f}" + ("  (缓存)" if cached else "")) if ok else ""
        print(f"  [{i}/{len(stages)}] {st.key:10s} via {executor:8s} 载体={carrier:18s} {status} ({dt:.1f}s){served_note}")
        if not ok:
            blocked_details[st.key] = content.splitlines()[0][:240] if content else "未返回详细错误"
            print(f"       ↳ {content.splitlines()[0][:120]}")

        body = content if ok else f"> ⚠️ 本阶段未跑通。\n>\n> {content}"
        _safe_write_text(run_dir / f"{i:02d}-{st.key}.md", f"# {st.title}（{st.role} · {carrier}）\n\n{body}\n")
        # 结构化产物 JSON（artifact schema 接线：填 carrier/task_type/tokens/cost/window_used/budget_report）
        from devkit import artifact as _artifact_mod
        # codex executor：从 verify 报告里解析 GO/NO-GO，同步传播到 tests_failed
        _stage_verdict: Optional[str] = None
        _stage_tests_passed: Optional[bool] = None
        if ok and executor == "codex":
            from devkit.executors import _parse_verify_report as _pvr
            _vr = _pvr(content)
            _stage_verdict = _vr["verdict"]
            _stage_tests_passed = _vr["tests_passed"]
            if _stage_verdict == "NO-GO":
                _codex_verify_failed = True   # 事后 OR 进 tests_failed，不被 build/test 重置
        _art_path = run_dir / f"{i:02d}-{st.key}.artifact.json"
        _output_protocol = None
        if st.key == "implement" and ok:
            try:
                from devkit import apply as _apply
                _output_protocol = _apply.build_output_protocol(body, response_diag=response_diag)
            except Exception:
                _output_protocol = None
        _art_obj = _artifact_mod.make(
            st.key, st.role, st.title, body,
            fields=_artifact_mod.extract_fields(st.key, body),
            carrier=carrier,
            carrier_selected=_carrier_now if isinstance(_carrier_raw, list) else None,
            task_type=_task_type,
            tokens=tokens if ok else None,
            cost=cost if ok else None,
            verdict=_stage_verdict,
            tests_passed=_stage_tests_passed,
            window_used=_win,
            budget_report={"kept": _pack["kept"], "dropped": _pack["dropped"], "used": _pack["used"]},
            failure_code=failure_code if not ok else None,
            response_diag=response_diag,
            output_protocol=_output_protocol,
        )
        _safe_write_text(_art_path, json.dumps(_art_obj, ensure_ascii=False, indent=2))
        if st.key == "implement":
            _impl_artifact_path = _art_path
            _impl_response_diag = response_diag
        if ok:
            artifacts.append((st, content))
            # compact 指针：产物太长就立刻压成要点，供下游阶段复用（成本计入总账）
            if compact_model and len(content) > COMPACT_THRESHOLD:
                summ, ctk, cco = compact_text(
                    content, base_url, api_key, compact_model,
                    tags=[f"run:{ts}", f"stage:{st.key}", "compact"])
                compacted[st.key] = summ
                tot_tokens += ctk
                tot_cost += cco
                print(f"       ↳ compact {len(content)}→{len(summ)} 字 via {compact_model}"
                      f"  +{ctk}tok ${cco:.5f}")
        log_rows.append(f"| {st.key} | {carrier} | {served if ok else '-'} | {status} | {dt:.1f}s | {tokens} | ${cost:.5f} |")
        stage_meta.append((st.key, carrier, served if ok else "-", status))

        # 软预算（人类门的成本护栏）：超了就停掉剩余阶段，按 NO-GO 收尾
        if budget and tot_cost > budget:
            over_budget = True
            print(f"  ⛔ 超预算：已花 ${tot_cost:.5f} > 预算 ${budget:.5f}，停止剩余阶段")
            break

    # ---- 构建 & 测试：把 implement 产出物化、跑测试（apply 是人类门）----
    build_note, tests_failed, eval_sum = "", False, ""
    iter_rounds, iter_cost, iter_converged = 0, 0.0, None  # 迭代循环状态（默认不迭代）
    cascade_path = [cascade[0]] if cascade else []         # 各轮实际用的 generator 载体（round0=初始）
    final_review_text = next((txt for s, txt in artifacts if s.key == "review"), "")
    impl_text = next((txt for s, txt in artifacts if s.key == "implement"), None)
    if impl_text:
        from devkit import apply as _apply
        build_dir = run_dir / "build"
        files = _apply.materialize(impl_text, build_dir)
        _ensure_runtime_dirs(build_dir)
        if not files and _task_prefers_report_only(task) and impl_text.strip():
            _safe_write_text(build_dir / "run-log.md", impl_text.strip() + "\n")
            files = ["run-log.md"]
        materialization = _apply.diagnose_materialization(impl_text, files)
        output_protocol = _apply.build_output_protocol(
            impl_text,
            materialized_files=files,
            response_diag=_impl_response_diag,
        )
        report_only_artifact = _is_report_only_artifact(task, files)
        _contract_paths = _task_contract.validate_materialized_paths(_contract, files)
        if not _contract_paths["ok"]:
            blocked_files = list(_contract_paths["blocked"])
            report_only_artifact = True
            collect = {
                "ok": False,
                "runner": None,
                "collected": 0,
                "output": "（task contract 拦截：report-only 任务不得落 repo 级测试/保护路径）",
                "failure_code": "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN",
            }
            tpassed, tout = False, collect["output"]
            _safe_write_text(build_dir / "_test-output.txt", tout)
            verdict = "❌ 任务契约拦截"
            from devkit import evidence as _evidence
            _ev_verdict = _evidence.gate({
                "has_test_output": False,
                "tests_passed": False,
                "has_codex_verdict": _codex_verify_failed,
                "codex_verdict": "NO-GO" if _codex_verify_failed else None,
            })
            tests_failed = True
            hard_failure_code = "TASK_CONTRACT_ARTIFACT_PATH_FORBIDDEN"
            eval_sum = ""
            build_note = (
                "## 构建 & 测试（sandbox: build/）\n\n"
                f"- 物化文件：{', '.join(files)}\n"
                f"- task_kind：**{_contract.task_kind}** / delivery_mode：**{_contract.delivery_mode}**\n"
                f"- 契约拦截：**{', '.join(blocked_files)}**\n"
                f"- 测试：**{verdict}**\n\n"
                "> ⛔ stage contract：report-only / 诊断任务不得漂移为 repo 级测试产物；"
                "如确需测试，请显式声明允许测试路径或升级为 apply-required。\n"
            )
            if _impl_artifact_path and _impl_artifact_path.exists():
                try:
                    _impl_art = json.loads(_impl_artifact_path.read_text(encoding="utf-8"))
                    _impl_art["verdict"] = "NO-GO"
                    _impl_art["tests_passed"] = False
                    _impl_art["failure_code"] = hard_failure_code
                    _impl_art["test_collection"] = collect
                    _impl_art["task_contract"] = {
                        "task_kind": _contract.task_kind,
                        "delivery_mode": _contract.delivery_mode,
                        "blocked_paths": blocked_files,
                    }
                    _safe_write_text(_impl_artifact_path, json.dumps(_impl_art, ensure_ascii=False, indent=2))
                except Exception:
                    pass
            _safe_write_text(run_dir / "88-build-and-test.md", build_note)
            gate = "NO-GO（任务契约阻止阶段漂移）"
            _safe_write_text(run_dir / "99-gate.md", f"# Gate 建议\n\n{gate}\n")
            return {
                "run_dir": str(run_dir),
                "gate": gate,
                "tokens": tot_tokens,
                "cost": tot_cost,
                "blocked": blocked_details,
            }
        collect = _apply.collect_tests(build_dir) if files and not report_only_artifact else {
            "ok": False,
            "runner": None,
            "collected": 0,
            "output": ("（report-only 产物，跳过测试收集）" if report_only_artifact
                       else "（未识别到代码文件，跳过 collect）"),
            "failure_code": (None if report_only_artifact
                             else materialization.get("failure_code") or "MATERIALIZE_NO_FILES"),
        }
        # T16 物理验证（smoke import）：在子进程里导入每个模块，捕获导入期错误
        if physical_verify:
            from devkit import verify as _verify
            _smoke = _verify.smoke_import(build_dir)
            if not _smoke["ok"]:
                print(f"  ⚠️  smoke_import：{len(_smoke['errors'])} 个模块导入失败")
                for _se in _smoke["errors"]:
                    print(f"       ✗ {_se['file']}: {_se['error']}")
            else:
                print(f"  ✅ smoke_import：{len(_smoke['imported'])} 个模块可导入")
        if not files:
            tpassed, tout = False, collect["output"]
        elif report_only_artifact:
            tpassed, tout = True, collect.get("output", "")
        elif collect.get("failure_code"):
            tpassed, tout = False, collect.get("output", "")
        else:
            tpassed, tout = _apply.run_tests(build_dir)
        _safe_write_text(build_dir / "_test-output.txt", tout)
        verdict = "✅ report-only 产物已生成" if report_only_artifact else {
            True: "✅ 测试通过", False: "❌ 测试失败", None: "⚠️ 无测试文件"
        }[tpassed]
        # evidence gate：默认失败契约，必须拿出物理证据才能 GO
        from devkit import evidence as _evidence
        _ev_verdict = (
            {"verdict": "GO", "reason": "report-only artifact"}
            if report_only_artifact
            else _evidence.gate({
                "has_test_output": tpassed is not None and bool(files),
                "tests_passed": tpassed,
                "has_codex_verdict": _codex_verify_failed,
                "codex_verdict": "NO-GO" if _codex_verify_failed else None,
            })
        )
        tests_failed = (_ev_verdict["verdict"] == "NO-GO") or not files or bool(collect.get("failure_code"))
        hard_failure_code = materialization.get("failure_code") or collect.get("failure_code")
        # 回填 implement artifact：verdict + tests_passed（仅在 build/test 后才知道）
        if _impl_artifact_path and _impl_artifact_path.exists():
            try:
                _impl_art = json.loads(_impl_artifact_path.read_text(encoding="utf-8"))
                _impl_art["verdict"] = "NO-GO" if tests_failed else "GO"
                _impl_art["tests_passed"] = tpassed
                _impl_art["failure_code"] = hard_failure_code or ("TESTS_FAILED" if tpassed is False else None)
                _impl_art["materialization"] = materialization
                _impl_art["output_protocol"] = output_protocol
                _impl_art["test_collection"] = collect
                _safe_write_text(
                    _impl_artifact_path,
                    json.dumps(_impl_art, ensure_ascii=False, indent=2),
                )
            except Exception:  # noqa: BLE001
                pass
        print(f"  ▸ build&test: 物化 {len(files)} 文件 · {verdict}")
        build_note = (
            f"\n## 构建 & 测试（sandbox: build/）\n\n"
            f"- 物化文件：{', '.join(files) if files else '（未识别到代码文件）'}\n"
            + (f"- 物化失败码：**{materialization['failure_code']}**\n" if materialization.get("failure_code") else "")
            + (f"- 产物模式：**report-only**\n" if report_only_artifact else "")
            + (f"- 测试收集：**{collect.get('runner') or 'n/a'} / {collect.get('collected', 0)}**\n" if files else "")
            + (f"- 收集失败码：**{collect['failure_code']}**\n" if collect.get("failure_code") else "")
            + f"- 测试：**{verdict}**\n\n```\n{tout[:1200]}\n```\n"
        )
        if golden:
            from devkit import evals as _evals
            eval_ok, eval_sum = _evals.run_golden(build_dir, golden)
            tests_failed = tests_failed or (not eval_ok)
            build_note += f"\n## Eval Gate（golden 质量回归）\n\n{eval_sum}\n"
            print(f"  ▸ eval gate: {'✅ 全过' if eval_ok else '❌ 有失败'}")
            # T16 物理验证：用子进程再跑一遍 golden，与 evals.py 的 in-process 结果交叉比对
            if physical_verify:
                from devkit import verify as _verify
                _pvr = _verify.run_golden_subprocess(build_dir, golden)
                _pv_sum = _verify.summarize(_pvr)
                build_note += f"\n## 物理验证（subprocess golden 交叉比对）\n\n```\n{_pv_sum}\n```\n"
                if not _pvr["ok"] and eval_ok:
                    print(f"  ⚠️  物理验证与 eval gate 结果不一致（subprocess 失败但 in-process 通过）")
                    tests_failed = True
                elif _pvr["ok"]:
                    print(f"  ✅ 物理验证：{_pvr['passed']}/{_pvr['passed']+_pvr['failed']} subprocess 通过")

        # ---- 迭代循环（Planner→Generator→Evaluator）：评判 NO-GO 就回灌构建者修复，直到通过或达上限 ----
        impl_stage = next((s for s in stages if s.key == "implement"), None)
        review_stage = next((s for s in stages if s.key == "review"), None)
        iter_cost0 = tot_cost
        from devkit import stopcheck as _stopcheck
        _iter_error_sigs: list = []   # 每轮错误签名，用于死循环检测
        while (iterate and impl_stage and iter_rounds < iterate
               and (tests_failed or _wants_changes(final_review_text))):
            iter_rounds += 1
            # cascade：本轮升级到下一档载体（cheap→strong）；否则沿用同一载体
            ic = _cascade_carrier(cascade, iter_rounds,
                                  carrier_overrides.get("implement", impl_stage.carrier))
            if cascade:
                cascade_path.append(ic)
            print(f"  ↻ 迭代 {iter_rounds}/{iterate}（评判未过 → {'升级 ' + ic if cascade else '回灌构建者修复'}）")
            # T5 STEER：循环间隙读转向指令（人类写文件 STEER 到 run_dir 或项目根即可注入）
            _steer_text = ""
            for _steer_path in [run_dir / "STEER", ROOT / "STEER"]:
                if _steer_path.exists():
                    _steer_text = _steer_path.read_text(encoding="utf-8").strip()
                    _steer_path.rename(str(_steer_path) + ".consumed")
                    print(f"  📡 STEER：注入转向指令（{_steer_path.name}，已消费）")
                    break
            feedback = _build_feedback(_fail_detail(tout, eval_sum),
                                       final_review_text if _wants_changes(final_review_text) else "")
            # 重跑 Generator（implement）
            iex = executor_map.get("implement") or getattr(impl_stage, "executor", None) or "chat"
            ismax = getattr(impl_stage, "max_tokens", None) or max_tokens
            iuser = f"## 开发任务\n{task}\n\n## 上一版实现\n{impl_text[:3000]}\n\n{feedback}"
            if _steer_text:
                iuser += f"\n\n## 转向指令（STEER）\n{_steer_text}"
            ok, new_impl, _sv, tk, co, dt = _exec_stage(
                impl_stage, iuser, ic, iex, ismax, base_url, api_key, ts, run_dir,
                os_sandbox, suffix=f"-r{iter_rounds}")
            tot_tokens += tk
            tot_cost += co
            print(f"     ↳ implement(r{iter_rounds}) via {iex}/{ic} {'OK' if ok else 'BLOCKED'} ({dt:.1f}s) {tk}tok ${co:.5f}")
            if not ok:
                if cascade:               # 该档不可用（down/unknown）→ 升级下一档，不中断 cascade
                    print(f"     ↳ {ic} 档不可用，升下一档")
                    continue
                break
            impl_text = new_impl
            _safe_write_text(run_dir / f"90-implement-r{iter_rounds}.md", f"# 迭代实现 r{iter_rounds}\n\n{new_impl}\n")
            tests_failed, tout, files, eval_sum, _iter_materialization, _iter_collect = _materialize_test(impl_text, build_dir, golden)
            print(f"     ↳ build&test(r{iter_rounds}): {'✅ 通过' if not tests_failed else '❌ 失败'}")
            # stopcheck：同一错误签名连续重复 → 写 AGENT_STOP 挂起
            _iter_error_sigs.append(tout[:120].strip() if tests_failed else "")
            _sc = _stopcheck.should_stop(_iter_error_sigs)
            if _sc["stop"]:
                print(f"  🛑 AGENT_STOP：{_sc['reason']}（{iter_rounds} 轮相同错误，挂起）")
                _safe_write_text(
                    run_dir / "AGENT_STOP",
                    f"stopcheck: {_sc['reason']} after {iter_rounds} iterate rounds\n",
                )
                break
            # 重跑 Evaluator（review）
            if review_stage:
                rc = carrier_overrides.get("review", review_stage.carrier)
                rex = executor_map.get("review") or getattr(review_stage, "executor", None) or "chat"
                rsmax = getattr(review_stage, "max_tokens", None) or max_tokens
                if blind_review:
                    ruser = (f"## 开发任务\n{task}\n\n"
                             f"## 测试输出（盲审：实现已屏蔽）\n```\n{tout[:1200]}\n```\n\n"
                             f"请仅基于任务规格 + 测试结果按 APPROVE / REQUEST-CHANGES 评判并给具体 findings。")
                else:
                    ruser = (f"## 开发任务\n{task}\n\n## 当前实现\n{impl_text[:3000]}\n\n"
                             f"## 测试输出\n```\n{tout[:1200]}\n```\n\n请按 APPROVE / REQUEST-CHANGES 评判并给具体 findings。")
                _rok, final_review_text, _rsv, rtk, rco, _rdt = _exec_stage(
                    review_stage, ruser, rc, rex, rsmax, base_url, api_key, ts, run_dir,
                    os_sandbox, suffix=f"-r{iter_rounds}")
                tot_tokens += rtk
                tot_cost += rco
                _safe_write_text(run_dir / f"91-review-r{iter_rounds}.md", f"# 迭代评判 r{iter_rounds}\n\n{final_review_text}\n")
                print(f"     ↳ review(r{iter_rounds}): {'REQUEST-CHANGES' if _wants_changes(final_review_text) else 'APPROVE'}")
            if budget and tot_cost > budget:
                over_budget = True
                print("  ⛔ 超预算，停止迭代")
                break
        if iterate or cascade:        # 单档 cascade 也算迭代模式（记阶梯/进 gate）
            iter_cost = tot_cost - iter_cost0
            iter_converged = (not tests_failed) and (not _wants_changes(final_review_text))
            build_note += (
                f"\n## 迭代循环（Planner→Generator→Evaluator）\n\n"
                f"- 轮数：**{iter_rounds}/{iterate}**\n"
                f"- 结果：{'✅ 收敛（评判通过）' if iter_converged else '❌ 未收敛（达上限仍 NO-GO）'}\n"
                f"- 迭代花费：**${iter_cost:.5f}**"
                + (f" · 单位接受变更成本 ≈ ${iter_cost/max(iter_rounds,1):.5f}/轮（已收敛）\n"
                   if iter_converged else "（未接受，等于空烧）\n")
                + (f"- cascade 阶梯：{' → '.join(cascade)}；实走：{' → '.join(cascade_path)}\n" if cascade else "")
                + f"\n最终测试：```\n{tout[:800]}\n```\n")
            print(f"  ↻ 迭代结束：{iter_rounds} 轮 · {'收敛✅' if iter_converged else '未收敛❌'} · 迭代花费 ${iter_cost:.5f}"
                  + (f" · 实走 {' → '.join(cascade_path)}" if cascade else ""))

        # applylock：检查哪些文件需要人类 apply（harness 核心 / test_* / *.golden.json）
        from devkit import applylock as _applylock
        _allow_apply_mode_tests = bool(apply_target or apply_git)
        _lock = _applylock.ApplyLock(
            allowed_test_prefixes=(
                ("tests/",) if _allow_apply_mode_tests else _applylock.DEFAULT_ALLOWED_TEST_PREFIXES
            )
        )
        _lock_ctx = _applylock.RunContext.get(run_id=ts, runs_dir=ROOT / "devkit" / "runs")
        _locked_files = [f for f in (files or []) if _lock.is_protected(f, _lock_ctx)]
        _exempted_files = [
            f"{f} ({_lock.exemption_reason(f, _lock_ctx)})"
            for f in (files or [])
            if not _lock.is_protected(f, _lock_ctx) and _lock.exemption_reason(f, _lock_ctx)
        ]
        if _exempted_files:
            build_note += f"\n> 🟢 applylock 放行：{', '.join(_exempted_files)}\n"
            print(f"  🟢 applylock 放行：{', '.join(_exempted_files)}")
        if _locked_files:
            build_note += f"\n> 🔒 以下文件需人类 apply（applylock 保护，已跳过自动 apply）：{', '.join(_locked_files)}\n"
            print(f"  🔒 applylock：{', '.join(_locked_files)} 需人类 apply（跳过）")
            if _applylock_blocks_success(task, report_only_artifact, _locked_files):
                tests_failed = True
                hard_failure_code = hard_failure_code or "APPLYLOCK_HUMAN_REQUIRED"
                build_note += "> ⛔ applylock 命中关键验证文件，本轮直接判定 NO-GO，禁止继续报进展。\n"
                if _impl_artifact_path and _impl_artifact_path.exists():
                    try:
                        _impl_art = json.loads(_impl_artifact_path.read_text(encoding="utf-8"))
                        _impl_art["verdict"] = "NO-GO"
                        _impl_art["failure_code"] = hard_failure_code
                        _safe_write_text(
                            _impl_artifact_path,
                            json.dumps(_impl_art, ensure_ascii=False, indent=2),
                        )
                    except Exception:  # noqa: BLE001
                        pass
            # T8：自动升级 review 到 codex-sub（harness 文件改动风险高，需独立强审查）
            if "review" not in carrier_overrides:
                carrier_overrides["review"] = "codex-sub"
                print(f"  🔺 T8 强审查升级：review 载体自动提升至 codex-sub（harness 文件需强审查）")
        if apply_target and files and not tests_failed:
            applied = _apply.apply_files(build_dir, apply_target)
            build_note += f"\n> ✅ 已 apply 到 `{apply_target}`：{', '.join(applied)}\n"
            print(f"  ✅ apply → {apply_target}: {', '.join(applied)}")
        elif apply_target and tests_failed:
            build_note += "\n> ⛔ 测试未过，未 apply（人类门：先修绿再 --apply）\n"
            print("  ⛔ 测试未过，未 apply")
        if apply_git and files and not tests_failed:
            # ratchet 门：golden.json 只增不减（自我修改护栏）
            from devkit import ratchet as _ratchet
            _ratchet_blocked = False
            _repo_root = pathlib.Path(apply_git)
            for _f in files:
                if not _f.endswith(".golden.json"):
                    continue
                _new_path = build_dir / _f
                _old_path = _repo_root / _f
                if not _new_path.exists() or not _old_path.exists():
                    continue
                try:
                    _old_cases = json.loads(_old_path.read_text(encoding="utf-8"))
                    _new_cases = json.loads(_new_path.read_text(encoding="utf-8"))
                    _rc = _ratchet.check(_old_cases, _new_cases)
                    if _rc["weakened"]:
                        build_note += (f"\n> 🔒 ratchet 拒绝 apply {_f}：{_rc['reason']}"
                                       f"（{_rc['old_count']}→{_rc['new_count']} cases）\n")
                        print(f"  🔒 ratchet：{_f} 弱化（{_rc['reason']}），拒绝 git apply")
                        _ratchet_blocked = True
                except Exception:  # noqa: BLE001
                    pass
            if _ratchet_blocked:
                build_note += "\n> ⛔ ratchet 护栏触发，未 git apply（测试集被弱化）\n"
            else:
                br = apply_branch or f"loom/{ts}"
                gr = _apply.apply_to_git(build_dir, apply_git, br, message=f"loom: {str(task)[:60]}")
                if gr.get("error"):
                    build_note += f"\n> ⛔ git apply 失败：{gr['error']}\n"
                    print(f"  ⛔ git apply 失败：{gr['error']}")
                else:
                    build_note += (f"\n> ✅ git apply：`{gr['repo']}` 分支 `{gr['branch']}` "
                                   f"commit `{gr['commit']}`（{len(gr['applied'])} 文件，未 push）\n")
                    print(f"  ✅ git → 分支 {gr['branch']} commit {gr['commit']} ({len(gr['applied'])} files, 未 push)")
        elif apply_git and tests_failed:
            build_note += "\n> ⛔ 测试未过，未 git apply\n"

    # 汇总 + go/no-go（report-only：只给建议，真实合并仍是 human gate）
    tests_failed = tests_failed or _codex_verify_failed  # 确保无 impl_text 分支也能 OR 进来
    blocked = [r.split("|")[1].strip() for r in log_rows if "BLOCKED" in r]
    blocked_note = ""
    if blocked:
        blocked_lines = []
        for stage_key in blocked:
            reason = blocked_details.get(stage_key, "未返回详细错误")
            hint = _blocked_retry_hint(reason)
            blocked_lines.append(f"> - {stage_key}: {reason}" + (f"；{hint}" if hint else ""))
        blocked_note = "\n> 未跑通阶段：\n" + "\n".join(blocked_lines) + "\n"
    reasons = ((["有阶段未跑通"] if blocked else [])
               + (["构建测试/Eval 未过"] if tests_failed else [])
               + ([f"超预算 ${budget:.4f}"] if over_budget else [])
               # 迭代模式下，评判者仍 REQUEST-CHANGES 也算 NO-GO（让评判真正成为 gate）
               + (["评判者 REQUEST-CHANGES"] if ((iterate or cascade) and _wants_changes(final_review_text)) else []))
    gate = f"NO-GO（{' / '.join(reasons)}）" if reasons else "建议 GO（需人类最终确认）"
    run_log = (
        f"# R&D Loop Run {ts}\n\n"
        f"- 任务：{task}\n- 网关：{base_url}\n- 级别：{_delivery_display_label(_delivery_mode)}\n"
        f"- 用量合计：**{tot_tokens} tokens · ${tot_cost:.5f}**（订阅后端不计费，故 $0；API 后端为真实花费）\n\n"
        f"## 各阶段\n\n| 阶段 | 载体 | 实际模型 | 状态 | 用时 | tokens | 花费 |\n"
        f"| --- | --- | --- | --- | --- | --- | --- |\n"
        + "\n".join(log_rows)
        + build_note
        + f"\n\n## Gate 建议\n\n{gate}\n"
        + blocked_note
    )
    _safe_write_text(run_dir / "run-log.md", run_log)
    append_run_ledger(task, stage_meta, gate, run_dir, tot_tokens, tot_cost)
    _memory.record(task, gate, next((txt for s, txt in artifacts if s.key == "review"), ""))
    iter_note = (f"  迭代：{iter_rounds} 轮 {'收敛✅' if iter_converged else '未收敛❌'}" if iterate else "")
    print(f"\n✓ 完成。产物目录：{run_dir}\n  Gate：{gate}  用量：{tot_tokens} tok · ${tot_cost:.5f}{iter_note}")
    return {"run_dir": str(run_dir), "gate": gate, "blocked": blocked,
            "tokens": tot_tokens, "cost": round(tot_cost, 6),
            "iterations": iter_rounds, "converged": iter_converged,
            "iterate_cost": round(iter_cost, 6), "cascade_path": cascade_path}


def append_run_ledger(
    task: str,
    stage_meta: List[Tuple[str, str, str, str]],
    gate: str,
    run_dir: pathlib.Path,
    tot_tokens: int = 0,
    tot_cost: float = 0.0,
    ledger: pathlib.Path = ROOT / "devkit" / "RUNS.md",
) -> None:
    """把本次运行追加一行到总台账（report-only，fail-open：出错绝不影响 loop）。

    采纳了 cross-vendor reviewer(Codex) 在 dogfood 审查里提的健壮性要求：
    非字符串/缺字段回填、清洗 `|`/换行、产物目录强制相对路径、整体 try/except 兜底。
    """
    try:
        def clean(s: object) -> str:
            return str("" if s is None else s).replace("\n", " ").replace("|", "│").strip()

        brief = " ".join(clean(task).split())[:60] or "UNKNOWN"
        cells = [f"{clean(k)}:{clean(c)}→{clean(m) if st == 'OK' else 'BLOCKED'}"
                 for (k, c, m, st) in (stage_meta or [])] or ["UNKNOWN"]
        stages_cell = ", ".join(cells)
        go = "NO-GO" if str(gate).startswith("NO-GO") else "GO"
        try:
            out = str(pathlib.Path(run_dir).relative_to(ROOT))
        except ValueError:
            out = str(run_dir)
        ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        usage = f"{int(tot_tokens)}tok·${float(tot_cost):.4f}"
        line = f"| {ts} | {brief} | {stages_cell} | {go} | {usage} | {out} |\n"

        if not ledger.exists():
            ledger.parent.mkdir(parents=True, exist_ok=True)
            ledger.write_text(
                "# devkit 运行总台账\n\n"
                "| 时间戳 | 任务摘要 | 各阶段 载体→实际模型 | Gate | 用量 | 产物目录 |\n"
                "| --- | --- | --- | --- | --- | --- |\n"
            )
        with ledger.open("a", encoding="utf-8") as f:
            f.write(line)
    except Exception:  # noqa: BLE001  — 台账失败绝不阻断 loop
        return
