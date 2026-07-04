import pytest
# looper 模块在 v0.1 已迁到 devkit/looper.py；本测试保留对历史 root 位置
# scripts/debug/looper.py 的兼容性断言（不应再有 from looper import ...）。
import sys
sys.path.insert(0, "scripts")
from debug.looper import verify_placeholder, _NAME_PATTERN   # early import 触发点

def test_happy_path():
    assert verify_placeholder("user-01") is True

def test_name_pattern_blocks_unsafe():
    assert _NAME_PATTERN.fullmatch("a/b") is None
    with pytest.raises(ValueError):
        verify_placeholder("a/b")

def test_empty_string_rejected():
    with pytest.raises(ValueError):
        verify_placeholder("")
