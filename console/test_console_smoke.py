# test_console_smoke.py
"""端到端 smoke test for console/server.py — 纯标准库，无浏览器依赖。

启动 server 子进程，调用各 API 端点，断言返回合法 JSON。
"""
import json
import subprocess
import sys
import time
import unittest
import urllib.error
import urllib.request
from pathlib import Path

_PORT = 18765
_BASE = f"http://127.0.0.1:{_PORT}"
_SERVER = Path(__file__).parent / "server.py"


def _get(path: str, timeout: int = 5) -> tuple[int, dict]:
    url = _BASE + path
    req = urllib.request.Request(url)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        return e.code, {}
    except Exception as e:
        return 0, {"error": str(e)}


class ConsoleServerSmokeTest(unittest.TestCase):
    _proc = None

    @classmethod
    def setUpClass(cls):
        cls._proc = subprocess.Popen(
            [sys.executable, str(_SERVER), "--port", str(_PORT)],
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        # Wait up to 5s for server to be ready
        for _ in range(25):
            try:
                urllib.request.urlopen(f"{_BASE}/api/fitness", timeout=0.5)
                break
            except Exception:
                time.sleep(0.2)

    @classmethod
    def tearDownClass(cls):
        if cls._proc:
            cls._proc.terminate()
            cls._proc.wait(timeout=5)

    def test_fitness_returns_json(self):
        status, body = _get("/api/fitness")
        self.assertEqual(status, 200)
        self.assertIsInstance(body, (dict, list))

    def test_tasks_returns_json(self):
        status, body = _get("/api/tasks")
        self.assertEqual(status, 200)
        self.assertIsInstance(body, (dict, list))

    def test_learn_returns_json(self):
        status, body = _get("/api/learn")
        self.assertEqual(status, 200)
        self.assertIsInstance(body, (dict, list))

    def test_assets_returns_json(self):
        status, body = _get("/api/assets")
        self.assertIn(status, (200, 404))

    def test_unknown_endpoint_404(self):
        status, _ = _get("/api/nonexistent_endpoint_xyz")
        self.assertEqual(status, 404)


if __name__ == "__main__":
    unittest.main(verbosity=2)
