# devkit/work_pool.py
"""Work pool manager — pure standard library."""

def create(capacity: int) -> dict:
    """Create a new work pool with given capacity."""
    return {"capacity": capacity, "items": [], "active": []}

def submit(pool: dict, item: dict) -> tuple:
    """Submit an item to the pool. Returns (ok, pool)."""
    if len(pool["items"]) < pool["capacity"]:
        new_pool = {
            "capacity": pool["capacity"],
            "items": pool["items"] + [item],
            "active": pool["active"][:],
        }
        return (True, new_pool)
    return (False, pool)

def checkout(pool: dict, item_id: str) -> tuple:
    """Check out an item by id. Returns (item, pool)."""
    for i, item in enumerate(pool["items"]):
        if item.get("id") == item_id:
            # remove from items, add to active
            new_items = pool["items"][:i] + pool["items"][i+1:]
            new_active = pool["active"] + [item]
            new_pool = {
                "capacity": pool["capacity"],
                "items": new_items,
                "active": new_active,
            }
            return (item, new_pool)
    return (None, pool)

def complete(pool: dict, item_id: str) -> dict:
    """Complete a checked-out item. Returns new pool."""
    new_active = [item for item in pool["active"] if item.get("id") != item_id]
    return {
        "capacity": pool["capacity"],
        "items": pool["items"][:],
        "active": new_active,
    }

def pool_summary(pool: dict) -> dict:
    """Return summary of pool state."""
    return {
        "capacity": pool["capacity"],
        "pending": len(pool["items"]),
        "active": len(pool["active"]),
        "available": pool["capacity"] - len(pool["items"]) - len(pool["active"]),
    }
