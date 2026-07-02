"""
Sparse vector operations - pure standard library.

Each sparse vector is represented as {"data": {index: value, ...}} where only
non-zero values are stored in the inner dict.
"""

import math

def create(data: dict) -> dict:
    """Create a sparse vector from a {index: value} dict, dropping zero values."""
    return {"data": {k: v for k, v in data.items() if v != 0}}

def dot(a: dict, b: dict) -> float:
    """Compute the dot product of two sparse vectors."""
    a_data = a["data"]
    b_data = b["data"]
    # Iterate over the smaller dict for efficiency
    if len(a_data) > len(b_data):
        a_data, b_data = b_data, a_data
    total = 0.0
    for k, v in a_data.items():
        if k in b_data:
            total += v * b_data[k]
    return total

def add(a: dict, b: dict) -> dict:
    """Element-wise addition of two sparse vectors; zero results are not stored."""
    result = {}
    for k, v in a["data"].items():
        result[k] = v
    for k, v in b["data"].items():
        result[k] = result.get(k, 0) + v
    return {"data": {k: v for k, v in result.items() if v != 0}}

def scale(v: dict, factor: float) -> dict:
    """Multiply every element by factor; zero results are not stored."""
    if factor == 0:
        return {"data": {}}
    return {"data": {k: v * factor for k, v in v["data"].items()}}

def magnitude(v: dict) -> float:
    """Euclidean norm: sqrt(sum of squares of stored values)."""
    return math.sqrt(sum(x * x for x in v["data"].values()))
