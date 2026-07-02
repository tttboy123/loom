def create(capacity):
    return {"capacity": capacity, "cache": {}, "order": []}

def get(lru, key):
    if key not in lru["cache"]:
        return (None, lru)
    new_order = [k for k in lru["order"] if k != key] + [key]
    new_lru = {"capacity": lru["capacity"], "cache": dict(lru["cache"]), "order": new_order}
    return (new_lru["cache"][key], new_lru)

def put(lru, key, value):
    new_cache = dict(lru["cache"])
    new_order = [k for k in lru["order"] if k != key]
    new_cache[key] = value
    new_order.append(key)
    if len(new_cache) > lru["capacity"]:
        evict = new_order[0]
        new_order = new_order[1:]
        del new_cache[evict]
    return {"capacity": lru["capacity"], "cache": new_cache, "order": new_order}

def lru_summary(lru):
    return {"capacity": lru["capacity"], "size": len(lru["cache"]), "keys": list(lru["cache"].keys())}
