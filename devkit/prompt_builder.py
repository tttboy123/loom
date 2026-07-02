"""Structured LLM prompt builder (纯标准库)."""

def system(text: str) -> dict:
    """构造一条 system 消息。"""
    return {'role': 'system', 'content': text}

def user(text: str) -> dict:
    """构造一条 user 消息。"""
    return {'role': 'user', 'content': text}

def assistant(text: str) -> dict:
    """构造一条 assistant 消息。"""
    return {'role': 'assistant', 'content': text}

def build(messages: list[dict]) -> list[dict]:
    """过滤掉 content 为空字符串的消息，返回新列表（不修改入参）。"""
    return [m for m in messages if m.get('content') != '']

def render_text(messages: list[dict]) -> str:
    """将消息列表渲染为多行文本：每行格式 '[role]: content'；空列表返回 ''。"""
    if not messages:
        return ''
    return '\n'.join(f"[{m['role']}]: {m['content']}" for m in messages)
