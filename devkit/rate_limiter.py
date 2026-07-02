# devkit/rate_limiter.py
# Pure standard library token-bucket rate limiter.

import time

def create(rate: float, capacity: float) -> dict:
    """Create a new limiter with a full bucket.

    Args:
        rate: tokens added per second.
        capacity: maximum tokens in the bucket.

    Returns:
        A limiter dict with the given rate/capacity, a full bucket,
        and last_refill unset (None).
    """
    return {
        "rate": rate,
        "capacity": capacity,
        "tokens": capacity,
        "last_refill": None,
    }

def consume(limiter: dict, amount: float = 1.0) -> tuple[bool, dict]:
    """Try to consume `amount` tokens from the limiter.

    On the first call, the bucket starts full (tokens == capacity) because
    last_refill is None, so no time-based refill is applied.

    On subsequent calls, elapsed time (since last_refill) is used to add
    tokens at `rate` per second, capped at `capacity`.
    """
    rate = limiter["rate"]
    capacity = limiter["capacity"]
    tokens = limiter["tokens"]
    last_refill = limiter["last_refill"]

    now = time.time()
    if last_refill is None:
        # First call: bucket is already full per spec; just stamp the time.
        last_refill = now
    else:
        elapsed = now - last_refill
        if elapsed > 0:
            tokens = min(capacity, tokens + rate * elapsed)
        last_refill = now

    if tokens >= amount:
        tokens -= amount
        new_limiter = {
            "rate": rate,
            "capacity": capacity,
            "tokens": tokens,
            "last_refill": last_refill,
        }
        return True, new_limiter

    # Not enough tokens: don't mutate stored state, but still update
    # last_refill so the elapsed time so far counts toward refilling.
    updated = {
        "rate": rate,
        "capacity": capacity,
        "tokens": tokens,
        "last_refill": last_refill,
    }
    return False, updated

def available(limiter: dict) -> float:
    """Return the current number of tokens in the bucket (lazy refill)."""
    rate = limiter["rate"]
    capacity = limiter["capacity"]
    tokens = limiter["tokens"]
    last_refill = limiter["last_refill"]

    if last_refill is None:
        return tokens

    elapsed = time.time() - last_refill
    if elapsed > 0:
        tokens = min(capacity, tokens + rate * elapsed)
    return tokens

def limiter_summary(limiter: dict) -> dict:
    """Return a summary view of the limiter with current availability."""
    rate = limiter["rate"]
    capacity = limiter["capacity"]
    tokens = available(limiter)

    if capacity > 0:
        utilization = 1.0 - (tokens / capacity)
    else:
        utilization = 0.0

    # Clamp to [0, 1] for safety against floating point drift.
    if utilization < 0.0:
        utilization = 0.0
    elif utilization > 1.0:
        utilization = 1.0

    return {
        "rate": rate,
        "capacity": capacity,
        "available": float(tokens),
        "utilization": utilization,
    }
