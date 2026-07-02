# artifact.py
"""Loom 的结构化产物总线 v1"""

import re

PROTECTED = ("contract", "failure", "patch_targets")

def make(stage, role, title, body, fields=None, *,
         carrier=None, carrier_selected=None, task_type=None,
         tokens=None, cost=None,
         verdict=None, tests_passed=None,
         window_used=None, budget_report=None,
         failure_code=None, response_diag=None, materialization=None,
         output_protocol=None):
    if fields is None:
        fields = {}
    else:
        fields = dict(fields)
    art = {
        "stage": stage,
        "role": role,
        "title": title,
        "body": body,
        "fields": fields,
        "carrier": carrier,
        "task_type": task_type,
        "tokens": tokens,
        "cost": cost,
        "verdict": verdict,
        "tests_passed": tests_passed,
        "window_used": window_used,
        "budget_report": budget_report,
        "failure_code": failure_code,
        "response_diag": response_diag,
        "materialization": materialization,
        "output_protocol": output_protocol,
    }
    if carrier_selected is not None:
        art["carrier_selected"] = carrier_selected
    return art

def extract_fields(stage, body):
    if stage == "implement":
        try:
            from devkit import apply as _apply
            targets = sorted(_apply.extract_materialization_map(body).keys())
            if targets:
                return {"patch_targets": targets}
        except Exception:
            pass
        pattern = re.compile(r"^#\s*(\S+\.\S+)\s*$")
        targets = []
        for line in body.splitlines():
            match = pattern.match(line.strip())
            if match:
                targets.append(match.group(1))
        return {"patch_targets": targets}
    if stage == "review":
        for line in body.splitlines():
            stripped = line.strip()
            if "NO-GO" in stripped or "需要修改" in stripped:
                return {"failure": stripped}
        return {}
    return {}

def protected(artifact):
    source_fields = artifact.get("fields", {})
    result = {}
    for key in PROTECTED:
        if key in source_fields:
            result[key] = source_fields[key]
    return result
