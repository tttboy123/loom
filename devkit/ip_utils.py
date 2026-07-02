# devkit/ip_utils.py
import ipaddress

def is_valid_ipv4(ip: str) -> bool:
    """判断是否为合法 IPv4 地址（4个0-255的数字，点分隔）"""
    # 1. 拒绝空白前后空格、前导0（除了单0）、负数、字母、空段等
    # 2. 用 ipaddress 标准库验证，并结合严格格式检查
    try:
        ipaddress.IPv4Address(ip)
    except ipaddress.AddressValueError:
        return False

    # 额外检查：拒绝前导0（如 "1.2.3.04"），因为 IPv4Address 允许它
    parts = ip.split('.')
    if len(parts) != 4:
        return False
    for part in parts:
        if not part.isdigit():
            return False
        # 前导0检查：数字长度>1且以0开头
        if len(part) > 1 and part[0] == '0':
            return False
    return True

def ip_to_int(ip: str) -> int:
    """将 IPv4 地址转换为整数"""
    return int(ipaddress.IPv4Address(ip))

def int_to_ip(n: int) -> str:
    """将整数转换为 IPv4 地址字符串"""
    return str(ipaddress.IPv4Address(n))

def in_subnet(ip: str, subnet: str) -> bool:
    """判断 ip 是否在 CIDR 子网内（如 '192.168.1.0/24'）"""
    addr = ipaddress.IPv4Address(ip)
    net = ipaddress.IPv4Network(subnet, strict=False)
    return addr in net
