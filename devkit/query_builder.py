# devkit/query_builder.py
"""Chainable query builder over list[dict]. Stdlib only."""

OPS = {
    "==": lambda a, b: a == b,
    "!=": lambda a, b: a != b,
    ">":  lambda a, b: a >  b,
    "<":  lambda a, b: a <  b,
    ">=": lambda a, b: a >= b,
    "<=": lambda a, b: a <= b,
}

def create(data):
    return {"data": data, "filters": [], "order_by": None, "limit_n": None}

def where(q, key, op, value):
    if op not in OPS:
        raise ValueError(f"unsupported op: {op!r}")
    new_q = dict(q)
    new_q["filters"] = list(q["filters"]) + [(key, op, value)]
    return new_q

def order(q, key, ascending=True):
    new_q = dict(q)
    new_q["order_by"] = (key, ascending)
    return new_q

def limit(q, n):
    if not isinstance(n, int) or n < 0:
        raise ValueError(f"limit must be non-negative int, got {n!r}")
    new_q = dict(q)
    new_q["limit_n"] = n
    return new_q

def execute(q):
    result = list(q["data"])

    # 1) filters
    for key, op, value in q["filters"]:
        cmp = OPS[op]
        result = [row for row in result if cmp(row.get(key), value)]

    # 2) order_by
    if q["order_by"] is not None:
        key, ascending = q["order_by"]

        def sort_key(row):
            v = row.get(key)
            # 缺 key / None 一律排到末尾；asc 时小→大
            return (v is None, v if v is not None else 0)

        result = sorted(result, key=sort_key, reverse=not ascending)

    # 3) limit
    if q["limit_n"] is not None:
        result = result[: q["limit_n"]]

    return result
