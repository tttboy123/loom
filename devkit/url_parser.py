# devkit/url_parser.py
"""URL 解析与构建工具模块"""

from urllib.parse import urlparse, urlencode, urlunparse, parse_qs

def parse(url: str) -> dict:
    """解析 URL 字符串为部件字典
    
    Args:
        url: URL 字符串
        
    Returns:
        dict: 包含 scheme, host, path, query (dict), fragment 的字典
    """
    parsed = urlparse(url)
    
    # 解析查询字符串为字典（保持每个参数第一个值）
    raw_query = parsed.query
    if raw_query:
        query_dict = {k: v[0] if len(v) == 1 else v 
                      for k, v in parse_qs(raw_query, keep_blank_values=True).items()}
    else:
        query_dict = {}
    
    return {
        "scheme": parsed.scheme,
        "host": parsed.hostname or "",
        "path": parsed.path or "",
        "query": query_dict,
        "fragment": parsed.fragment or "",
    }

def build(parts: dict) -> str:
    """从部件字典构建 URL 字符串
    
    Args:
        parts: 包含 scheme, host, path, query (dict), fragment 的字典
        
    Returns:
        str: 构建的 URL 字符串
    """
    scheme = parts.get("scheme", "")
    host = parts.get("host", "")
    path = parts.get("path", "")
    query = parts.get("query", {})
    fragment = parts.get("fragment", "")
    
    # 构建查询字符串（对键和值进行编码）
    query_str = urlencode(query) if query else ""
    
    # 构建 netloc
    netloc = host
    
    # 使用 urlunparse 组合
    return urlunparse((scheme, netloc, path, "", query_str, fragment))

def add_param(url: str, key: str, value: str) -> str:
    """向 URL 添加查询参数，返回新 URL 字符串
    
    会自动对参数值进行 URL 编码，包括空格、& 等特殊字符。
    
    Args:
        url: 原始 URL 字符串
        key: 参数名
        value: 参数值（会自动编码）
        
    Returns:
        str: 添加参数后的新 URL 字符串
    """
    parts = parse(url)
    parts["query"][key] = value
    return build(parts)
