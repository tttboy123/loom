"""
Sprint Contract（借鉴 Anthropic harness）：编码前，评判者(Evaluator)先和构建者约定
「什么叫做完」—— 产出一份**可机器验证的验收合同**（Loom golden 格式的 JSON 用例）。

合同随后既注入构建者(implement)的提示（让它照着接口与验收点写），又作为 Eval Gate
跑在产物上、并喂进迭代反馈（`--iterate`）。评判者用独立载体（默认 reviewer），呼应 GAN 对抗。
"""
from __future__ import annotations

import json
import re
from typing import List, Tuple

from devkit import rdloop

_SYS = (
    "你是研发流程里的【评判者 / Evaluator】。在构建者动手编码之前，你的职责是先约定"
    "「什么叫做完」——产出一份**可机器自动验证**的验收合同。"
)

_FMT = (
    "只输出一个 JSON 数组，不要任何解释或 markdown 围栏。每个元素是一条验收用例，二选一格式：\n"
    "1) 纯函数返回值：{{\"name\":\"简述\",\"import\":\"from 模块名 import 函数名\","
    "\"expr\":\"函数名(参数)\",\"expect\":期望值}}\n"
    "2) 期望抛异常（非法输入）：{{\"name\":\"简述\",\"import\":\"from 模块名 import 函数名\","
    "\"expr\":\"函数名(非法参数)\",\"raises\":\"ValueError\"}}  ← 错误用例**必须**用 raises，不要把 expect 写成 \"ValueError\"\n"
    "3) 命令行：{{\"name\":\"简述\",\"cmd\":[\"python\",\"x.py\"],\"stdin\":\"\","
    "\"expect_contains\":\"输出应包含的子串\"}}\n"
    "要求：\n"
    "- 给 {n} 条左右，覆盖**旗舰用例 + 边界 + 至少一个非 happy-path（用 raises 表达）**。\n"
    "- **模块文件名要明确且统一**（构建者会照此命名，例如统一用 app.py / calc.py）。\n"
    "- expect 用 JSON 字面量（数字/字符串/布尔/数组/null），不要写 Python 代码。\n"
    "- 别假设未约定的行为；只测你能明确定义对错的点。"
)


def _extract_json_array(text: str):
    """从模型输出里稳健地抠出 JSON 数组。"""
    if not text:
        return None
    # 去掉可能的 ```json 围栏
    text = re.sub(r"```(?:json)?", "", text)
    start = text.find("[")
    end = text.rfind("]")
    if start < 0 or end <= start:
        return None
    try:
        data = json.loads(text[start:end + 1])
        return data if isinstance(data, list) else None
    except Exception:  # noqa: BLE001
        return None


def _valid_case(c) -> bool:
    return isinstance(c, dict) and ("expr" in c or "cmd" in c)


def negotiate(task: str, plan_text: str, base_url: str, api_key: str,
              carrier: str, n: int = 8, max_tokens: int = 1100
              ) -> Tuple[List[dict], int, float, str]:
    """生成验收合同。返回 (cases, tokens, cost, 原始文本)。失败则 cases=[]。"""
    user = (f"## 开发任务\n{task}\n\n## 已有计划（接口/范围参考）\n{plan_text[:3000]}\n\n"
            f"请产出验收合同。\n\n## 输出格式\n{_FMT.format(n=n)}")
    ok, content, _served, tokens, cost = rdloop.gateway_chat(
        base_url, api_key, carrier, _SYS, user, max_tokens,
        tags=["contract", "evaluator"])
    if not ok:
        return [], tokens, cost, content
    cases = _extract_json_array(content) or []
    cases = [c for c in cases if _valid_case(c)]
    return cases, tokens, cost, content


_REFINE_SYS = (
    "你是【开发/构建者】。评判者在编码前拟了一份验收合同（golden 用例）。"
    "你可以在动手前**收紧或修正**它：改掉明显写错的期望值、补一个被漏掉的边界/错误用例、"
    "去掉真正有歧义无法判定的用例。"
    "**不许把合同改弱**——不要删掉错误用例（raises），最终条数不得少于原合同。"
)
_REFINE_FMT = ("只输出最终的 JSON 数组（同样格式：expr/expect、或 expr/raises、或 cmd/expect_contains），"
               "不要解释。若没有异议，原样返回这份数组。")


def _refine(task: str, cases: List[dict], base_url: str, api_key: str,
            build_carrier: str, max_tokens: int = 1000) -> Tuple[List[dict], int, float]:
    """构建者审阅评判者拟的验收点 → 返回 (refined_cases, tokens, cost)。
    fail-safe：网关失败 / 非 JSON / 空 / 全无效 → 原样返回入参 cases。"""
    user = (f"## 开发任务\n{task}\n\n## 评判者拟的验收合同\n"
            f"{json.dumps(cases, ensure_ascii=False)}\n\n## 输出格式\n{_REFINE_FMT}")
    ok, content, _served, tokens, cost = rdloop.gateway_chat(
        base_url, api_key, build_carrier, _REFINE_SYS, user, max_tokens,
        tags=["contract", "builder"])
    if not ok:
        return cases, tokens, cost
    refined = [c for c in (_extract_json_array(content) or []) if _valid_case(c)]
    return (refined or cases), tokens, cost


def negotiate_rounds(task: str, plan_text: str, base_url: str, api_key: str,
                     eval_carrier: str, build_carrier: str, n: int = 8,
                     rounds: int = 1, max_tokens: int = 1100
                     ) -> Tuple[List[dict], int, float, str]:
    """评判者拟合同 → 构建者最多 rounds 轮收紧/修正（带反削弱地板）→ 议定。
    返回 (cases, tokens, cost, raw)，与 negotiate 同形。"""
    cases, tot_tk, tot_co, raw = negotiate(task, plan_text, base_url, api_key, eval_carrier, n, max_tokens)
    if not cases:
        return cases, tot_tk, tot_co, raw
    floor = len(cases)                                  # 反削弱：最终条数 >= 评判者原始条数
    floor_raises = sum(1 for c in cases if "raises" in c)  # raises 用例数不得减少
    for _ in range(max(rounds, 0)):
        refined, tk, co = _refine(task, cases, base_url, api_key, build_carrier, max_tokens)
        tot_tk += tk
        tot_co += co
        # 评判者有最终否决权：低于地板（条数或 raises 数）→ 拒绝本轮、保留原集
        if len(refined) < floor or sum(1 for c in refined if "raises" in c) < floor_raises:
            refined = cases
        if refined == cases:                            # 无异议/已收敛 → 提前停
            break
        cases = refined
    return cases, tot_tk, tot_co, raw


def to_block(cases: List[dict]) -> str:
    """把合同渲染成给构建者看的清单（注入 implement 提示）。"""
    lines = ["## 验收合同（必须全部满足；评判者已先行约定，按此实现接口与文件名）"]
    for i, c in enumerate(cases, 1):
        if "expr" in c:
            lines.append(f"{i}. `{c.get('import','')}` → `{c['expr']}` 应得 `{c.get('expect')!r}`"
                         f"  （{c.get('name','')}）")
        else:
            lines.append(f"{i}. 运行 `{' '.join(c.get('cmd', []))}` 输出应含 "
                         f"`{c.get('expect_contains','')}`  （{c.get('name','')}）")
    return "\n".join(lines)
