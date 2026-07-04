import pytest
import sys
sys.path.insert(0, "scripts")
from debug.env_gate import evaluate_gate

def _fs(files):
    """files: dict[path -> str]"""
    return lambda p: files.get(p)

def test_both_missing_fails():
    fs = _fs({})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["status"] == "FAIL"
    assert r["inspected_path"] is None
    assert "none of the env files exist" in r["reason"].lower()

def test_first_path_exists_passes_when_two_assignments_and_clean():
    body = "OPENAI_API_KEY=sk-abc\nANTHROPIC_API_KEY=sk-xyz\n"
    fs = _fs({"devkit/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["status"] == "PASS"
    assert r["inspected_path"] == "devkit/.env"
    assert r["assignment_count"] == 2
    assert r["violations"] == {"empty": [], "placeholder": []}

def test_second_path_used_when_first_missing():
    body = "AWS_ACCESS_KEY_ID=AKIA\nAWS_SECRET_ACCESS_KEY=secret\n"
    fs = _fs({"agent-platform/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["status"] == "PASS"
    assert r["inspected_path"] == "agent-platform/.env"
    assert r["assignment_count"] == 2

def test_too_few_assignments_fails():
    body = "OPENAI_API_KEY=sk-abc\n# comment only line\n"
    fs = _fs({"devkit/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["status"] == "FAIL"
    assert r["assignment_count"] == 1

def test_empty_value_violation_detected():
    body = "OPENAI_API_KEY=sk-abc\nANTHROPIC_API_KEY=\n"
    fs = _fs({"devkit/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert 2 in r["violations"]["empty"]
    assert r["status"] == "FAIL"

def test_placeholder_value_violation_detected():
    body = "OPENAI_API_KEY=your-key-here\nANTHROPIC_API_KEY=sk-xyz\n"
    fs = _fs({"devkit/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert 1 in r["violations"]["placeholder"]
    assert r["status"] == "FAIL"

def test_lowercase_key_does_not_count_as_assignment():
    body = "openai_api_key=sk-abc\nANTHROPIC_API_KEY=sk-xyz\n"
    fs = _fs({"devkit/.env": body})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["assignment_count"] == 1  # 只大写键计入
    assert r["status"] == "FAIL"

def test_unix_devkit_and_windows_agent_platform_precedence():
    body = "A=1\nB=2\n"
    fs = _fs({"devkit/.env": body, "agent-platform/.env": "C=3\n"})
    r = evaluate_gate(["devkit/.env", "agent-platform/.env"], fs)
    assert r["inspected_path"] == "devkit/.env"  # 第一个存在的优先
