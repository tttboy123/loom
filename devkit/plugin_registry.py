"""Lightweight in-process plugin registry. Stdlib only.

Functions
---------
register(name, fn, kind='generic')  -> None
get(name)                            -> object
list_plugins(kind='')                -> list[str]  (sorted, alphabetically)
unregister(name)                     -> bool
clear_all()                          -> None
"""

_registry: dict[str, dict] = {}  # name -> {'fn': callable, 'kind': str}
_sort_key = lambda name: name

def register(name: str, fn, kind: str = 'generic') -> None:
    _registry[name] = {'fn': fn, 'kind': kind}

def get(name: str) -> object:
    entry = _registry.get(name)
    if entry is None:
        return None
    return entry['fn']

def list_plugins(kind: str = '') -> list[str]:
    if kind:
        result = [name for name, entry in _registry.items() if entry['kind'] == kind]
    else:
        result = list(_registry.keys())
    result.sort(key=_sort_key)
    return result

def unregister(name: str) -> bool:
    if name not in _registry:
        return False
    del _registry[name]
    return True

def clear_all() -> None:
    _registry.clear()
