# codec.py
"""
codec.py — Base64 / Hex 编解码工具（纯标准库）

契约：
- 字符串入参，UTF-8 作为中间字节序列；
- 空串原样返回；
- 非法输入抛 ValueError（不静默吞错，也不幻觉成功）。
"""

import base64
import binascii

def b64_encode(data: str) -> str:
    """将字符串 UTF-8 编码后 Base64 编码，返回字符串。"""
    if not isinstance(data, str):
        raise TypeError(f"b64_encode expects str, got {type(data).__name__}")
    return base64.b64encode(data.encode("utf-8")).decode("ascii")

def b64_decode(data: str) -> str:
    """Base64 解码后返回 UTF-8 字符串。"""
    if not isinstance(data, str):
        raise TypeError(f"b64_decode expects str, got {type(data).__name__}")
    if data == "":
        return ""
    try:
        decoded = base64.b64decode(data, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise ValueError(f"invalid base64 input: {exc}") from exc
    try:
        return decoded.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError(f"decoded bytes are not valid UTF-8: {exc}") from exc

def hex_encode(data: str) -> str:
    """将字符串转为十六进制表示。"""
    if not isinstance(data, str):
        raise TypeError(f"hex_encode expects str, got {type(data).__name__}")
    return data.encode("utf-8").hex()

def hex_decode(data: str) -> str:
    """十六进制解码回字符串。"""
    if not isinstance(data, str):
        raise TypeError(f"hex_decode expects str, got {type(data).__name__}")
    if data == "":
        return ""
    try:
        decoded = bytes.fromhex(data)
    except ValueError as exc:
        raise ValueError(f"invalid hex input: {exc}") from exc
    try:
        return decoded.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError(f"decoded bytes are not valid UTF-8: {exc}") from exc
