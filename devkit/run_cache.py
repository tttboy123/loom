"""Caching layer for devkit run results.

Pure standard library. All functions are pure: they return a new dict
rather than mutating the input, so callers can treat cache objects
as values and compose them in expressions.
"""

def create(max_size: int) -> dict:
    """Create a fresh cache object.

    Args:
        max_size: Maximum number of entries before eviction kicks in.

    Returns:
        A dict with shape
        ``{"cache": {}, "max_size": int, "hits": 0, "misses": 0}``.
    """
    return {"cache": {}, "max_size": max_size, "hits": 0, "misses": 0}

def lookup(cache_obj: dict, key: str) -> tuple:
    """Look up ``key`` in ``cache_obj``.

    Returns ``(True, value)`` on a hit, otherwise ``(False, None)``.
    The returned ``cache_obj`` is a *new* dict; the input is not mutated.
    """
    new_obj = {
        "cache": dict(cache_obj["cache"]),
        "max_size": cache_obj["max_size"],
        "hits": cache_obj["hits"],
        "misses": cache_obj["misses"],
    }
    if key in new_obj["cache"]:
        new_obj["hits"] += 1
        return (True, new_obj["cache"][key])
    new_obj["misses"] += 1
    return (False, None)

def store(cache_obj: dict, key: str, value) -> dict:
    """Insert ``(key, value)`` into ``cache_obj``.

    If the cache is already at capacity (``len(cache) >= max_size``),
    the oldest insertion (the first key of the dict) is evicted first.
    Returns a new cache object; the input is not mutated.
    """
    new_obj = {
        "cache": dict(cache_obj["cache"]),
        "max_size": cache_obj["max_size"],
        "hits": cache_obj["hits"],
        "misses": cache_obj["misses"],
    }
    max_size = new_obj["max_size"]
    while len(new_obj["cache"]) >= max_size:
        oldest = next(iter(new_obj["cache"]))
        del new_obj["cache"][oldest]
    new_obj["cache"][key] = value
    return new_obj

def cache_stats(cache_obj: dict) -> dict:
    """Return ``{size, hits, misses, hit_rate}`` for ``cache_obj``."""
    hits = cache_obj["hits"]
    misses = cache_obj["misses"]
    total = hits + misses
    hit_rate = hits / total if total else 0.0
    return {
        "size": len(cache_obj["cache"]),
        "hits": hits,
        "misses": misses,
        "hit_rate": hit_rate,
    }
