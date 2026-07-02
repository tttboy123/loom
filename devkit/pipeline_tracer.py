# devkit/pipeline_tracer.py
from datetime import datetime, timezone

def create() -> dict:
    return {"spans": [], "active": None}

def start_span(tracer: dict, name: str) -> dict:
    new_tracer = {
        "spans": tracer["spans"] + [
            {
                "name": name,
                "start": datetime.now(timezone.utc).isoformat(),
                "end": None,
                "duration_ms": None,
            }
        ],
        "active": name,
    }
    return new_tracer

def end_span(tracer: dict, name: str) -> dict:
    new_spans = []
    for s in tracer["spans"]:
        if s["name"] == name and s["end"] is None:
            end_time = datetime.now(timezone.utc)
            start_time = datetime.fromisoformat(s["start"])
            delta = end_time - start_time
            duration_ms = delta.total_seconds() * 1000
            new_spans.append({
                "name": s["name"],
                "start": s["start"],
                "end": end_time.isoformat(),
                "duration_ms": duration_ms,
            })
        else:
            new_spans.append(s)
    new_active = None if tracer.get("active") == name else tracer.get("active")
    return {"spans": new_spans, "active": new_active}

def trace_summary(tracer: dict) -> dict:
    completed = sum(1 for s in tracer["spans"] if s["end"] is not None)
    return {
        "total_spans": len(tracer["spans"]),
        "completed": completed,
        "active": tracer.get("active"),
    }
