# evidence.py
"""长时自治 Agent 的默认失败契约 / 物理证据门。

默认一律 NO-GO，只有拿出真实证据（测试通过 / codex 判 GO）才准翻 GO。
"""

def gate(record):
    """证据门判定。

    Args:
        record: dict，含以下键——
            has_test_output (bool): 是否真的产生过测试输出。
            tests_passed (bool|None): 测试是否通过。
            has_codex_verdict (bool): 是否有 codex 验证结论。
            codex_verdict (str|None): codex 结论，"GO"/"NO-GO"/None。

    Returns:
        dict: {"verdict": "GO"|"NO-GO", "reason": <一句话说明>}
    """
    has_test_output = record.get("has_test_output", False)
    tests_passed = record.get("tests_passed")
    has_codex_verdict = record.get("has_codex_verdict", False)
    codex_verdict = record.get("codex_verdict")

    # 规则1：有真实测试输出且通过 → GO
    if has_test_output is True and tests_passed is True:
        return {"verdict": "GO", "reason": "tests passed"}

    # 规则2：有 codex 结论且判 GO → GO
    if has_codex_verdict is True and codex_verdict == "GO":
        return {"verdict": "GO", "reason": "codex GO"}

    # 以下一律 NO-GO，给出尽量具体的原因
    if has_test_output is True and tests_passed is False:
        return {"verdict": "NO-GO", "reason": "tests failed"}
    if has_test_output is True and tests_passed is None:
        return {"verdict": "NO-GO", "reason": "tests pending"}
    if has_codex_verdict is True and codex_verdict == "NO-GO":
        return {"verdict": "NO-GO", "reason": "codex NO-GO"}
    if not has_test_output and not has_codex_verdict:
        return {"verdict": "NO-GO", "reason": "no evidence"}

    # 兜底：有部分证据但不足以翻 GO
    return {"verdict": "NO-GO", "reason": "insufficient evidence"}
