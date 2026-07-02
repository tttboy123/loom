# devkit/bloom_filter.py
"""Simple Bloom Filter (standard library only).

Used for membership tests. May yield false positives, never false negatives.
"""

import math
import hashlib
from typing import Dict, List, Any

def _validate_params(capacity: int, error_rate: float) -> None:
    if not isinstance(capacity, int) or isinstance(capacity, bool):
        raise TypeError("capacity must be int")
    if capacity <= 0:
        raise ValueError("capacity must be > 0")
    if not isinstance(error_rate, (int, float)) or isinstance(error_rate, bool):
        raise TypeError("error_rate must be numeric")
    if not (0 < error_rate < 1):
        raise ValueError("error_rate must be in (0, 1)")

def _bit_size(capacity: int, error_rate: float) -> int:
    # m = -n * ln(p) / (ln 2)^2
    return int(math.ceil(-capacity * math.log(error_rate) / (math.log(2) ** 2)))

def _hash_count(bit_size: int, capacity: int) -> int:
    # k = (m/n) * ln 2
    k = int(round((bit_size / capacity) * math.log(2)))
    return max(k, 1)

def _hashes(item: str, k: int, m: int) -> List[int]:
    """Generate k distinct bit indices in [0, m) for an item.

    使用两个 SHA-256 摘要做 double-hashing，模拟 k 个不同哈希函数。
    """
    data = item.encode("utf-8")
    digest1 = int.from_bytes(hashlib.sha256(data).digest(), "big")
    digest2 = int.from_bytes(hashlib.sha256(digest1.to_bytes(32, "big")).digest(), "big")
    return [(digest1 + i * digest2) % m for i in range(k)]

def create(capacity: int = 1000, error_rate: float = 0.01) -> Dict[str, Any]:
    _validate_params(capacity, error_rate)
    m = _bit_size(capacity, error_rate)
    k = _hash_count(m, capacity)
    return {
        "capacity": capacity,
        "error_rate": float(error_rate),
        "bits": [0] * m,
        "hash_count": k,
        "added": 0,
    }

def add(bf: Dict[str, Any], item: str) -> Dict[str, Any]:
    new_bf = {
        "capacity": bf["capacity"],
        "error_rate": bf["error_rate"],
        "bits": list(bf["bits"]),
        "hash_count": bf["hash_count"],
        "added": bf["added"] + 1,
    }
    for idx in _hashes(str(item), new_bf["hash_count"], len(new_bf["bits"])):
        new_bf["bits"][idx] = 1
    return new_bf

def contains(bf: Dict[str, Any], item: str) -> bool:
    return all(
        bf["bits"][idx] == 1
        for idx in _hashes(str(item), bf["hash_count"], len(bf["bits"]))
    )

def bf_summary(bf: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "capacity": bf["capacity"],
        "added": bf["added"],
        "hash_count": bf["hash_count"],
        "bit_size": len(bf["bits"]),
    }
