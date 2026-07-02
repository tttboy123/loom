# devkit/output_buffer.py

def create() -> dict:
    return {"chunks": [], "total_chars": 0, "flushed": False}

def append(buf: dict, chunk: str) -> dict:
    new_chunks = buf["chunks"] + [chunk]
    new_total_chars = buf["total_chars"] + len(chunk)
    return {"chunks": new_chunks, "total_chars": new_total_chars, "flushed": buf["flushed"]}

def flush(buf: dict) -> tuple[str, dict]:
    joined = "".join(buf["chunks"])
    return (joined, {"chunks": [], "total_chars": 0, "flushed": True})

def buffer_stats(buf: dict) -> dict:
    chunk_count = len(buf["chunks"])
    total_chars = buf["total_chars"]
    flushed = buf["flushed"]
    if chunk_count == 0:
        avg_chunk_size = 0.0
    else:
        avg_chunk_size = total_chars / chunk_count
    return {
        "chunk_count": chunk_count,
        "total_chars": total_chars,
        "flushed": flushed,
        "avg_chunk_size": avg_chunk_size,
    }
