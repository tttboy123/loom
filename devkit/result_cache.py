# devkit/result_cache.py
"""In-memory LRU result cache — pure stdlib.

Public API:
    new_cache(maxsize=128) -> dict
    put(cache, key, value) -> None
    get_cached(cache, key, default=None)
    cache_stats(cache) -> dict
"""
from collections import OrderedDict
from typing import Any

def new_cache(maxsize: int = 128) -> dict:
    """Create a new LRU result cache object."""
    return {
        'maxsize': maxsize,
        'store': OrderedDict(),
        'hits': 0,
        'misses': 0,
    }

def put(cache: dict, key: str, value: Any) -> None:
    """Store key→value; evict least-recently-used entry when over maxsize."""
    store = cache['store']
    if key in store:
        # overwrite value and refresh recency
        store[key] = value
        store.move_to_end(key)
    else:
        store[key] = value
        if len(store) > cache['maxsize']:
            store.popitem(last=False)  # drop LRU

def get_cached(cache: dict, key: str, default: Any = None) -> Any:
    """Return cached value (promoting LRU order), or default on miss."""
    store = cache['store']
    if key in store:
        cache['hits'] += 1
        store.move_to_end(key)  # mark as most-recently-used
        return store[key]
    cache['misses'] += 1
    return default

def cache_stats(cache: dict) -> dict:
    """Return a snapshot of cache stats: size, maxsize, hits, misses, hit_rate."""
    hits = cache['hits']
    misses = cache['misses']
    total = hits + misses
    hit_rate = hits / total if total > 0 else 0.0
    return {
        'size': len(cache['store']),
        'maxsize': cache['maxsize'],
        'hits': hits,
        'misses': misses,
        'hit_rate': hit_rate,
    }
