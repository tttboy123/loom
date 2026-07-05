from __future__ import annotations

import json
import os
import pathlib
import threading
import time
from datetime import datetime, timezone


DEFAULT_STAGE_HEARTBEAT_SECONDS = float(os.environ.get("LOOM_STAGE_HEARTBEAT_INTERVAL", "30"))


def event_path(root: pathlib.Path | str) -> pathlib.Path:
    root_path = pathlib.Path(root)
    return root_path / "runtime" / "stage-events.jsonl"


def append_event(root: pathlib.Path | str, stage: str, event: str, **fields) -> pathlib.Path:
    path = event_path(root)
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "stage": str(stage),
        "event": str(event),
    }
    payload.update({k: v for k, v in fields.items() if v is not None})
    with path.open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(payload, ensure_ascii=False) + "\n")
    return path


class StageHeartbeat:
    def __init__(
        self,
        root: pathlib.Path | str,
        stage: str,
        *,
        interval_seconds: float | None = None,
        **metadata,
    ) -> None:
        self.root = pathlib.Path(root)
        self.stage = str(stage)
        self.interval_seconds = interval_seconds if interval_seconds is not None else DEFAULT_STAGE_HEARTBEAT_SECONDS
        self.metadata = dict(metadata)
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, name=f"loom-stage-{self.stage}", daemon=True)
        self._started_at = time.monotonic()
        self._finished = False

    def __enter__(self) -> "StageHeartbeat":
        append_event(self.root, self.stage, "start", **self.metadata)
        if self.interval_seconds > 0:
            self._thread.start()
        return self

    def _run(self) -> None:
        while not self._stop.wait(self.interval_seconds):
            append_event(
                self.root,
                self.stage,
                "heartbeat",
                elapsed_seconds=round(time.monotonic() - self._started_at, 3),
                **self.metadata,
            )

    def finish(self, status: str, **fields) -> None:
        if self._finished:
            return
        self._finished = True
        self._stop.set()
        if self._thread.is_alive():
            self._thread.join(timeout=max(self.interval_seconds, 0.1) + 0.2)
        append_event(
            self.root,
            self.stage,
            "finish",
            status=str(status),
            elapsed_seconds=round(time.monotonic() - self._started_at, 3),
            **self.metadata,
            **fields,
        )

    def __exit__(self, exc_type, exc, _tb) -> None:
        status = "error" if exc_type else "ok"
        failure = str(exc)[:240] if exc else None
        self.finish(status, error=failure)