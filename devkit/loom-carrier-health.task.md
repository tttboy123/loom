# Task: carrier_health.py — Carrier 健康探针

## 背景
Loom 负载均衡需要知道哪些 carrier 当前可达、响应延迟是多少。
本模块实现一个轻量探针：向 LiteLLM 网关发送最小探测请求，记录结果并缓存到本地文件。

## 要实现的文件
`devkit/carrier_health.py`

## 函数签名（必须完全符合）

```python
def probe(carrier: str, base_url: str, api_key: str, timeout: int = 5) -> dict:
    """向 LiteLLM 网关探测单个 carrier 的健康状态。

    发送 POST /chat/completions，model=carrier，content="ping"，max_tokens=1。
    返回 {ok: bool, latency_ms: float, error: str}。
    - ok=True：请求在 timeout 内收到有效响应
    - ok=False：超时或 HTTP 错误，error 填错误信息，latency_ms=0.0
    """

def probe_all(carriers: list[str], base_url: str, api_key: str, timeout: int = 5) -> dict:
    """批量探测多个 carriers，串行执行。

    返回 {carrier_name: {ok, latency_ms, error}, ...}
    若 carriers 为空返回 {}
    """

def load_cache(cache_path: str | None = None) -> dict:
    """从 devkit/carrier_health.json 读取上次探测结果，不存在返回 {}。

    cache_path=None 时自动使用 devkit/carrier_health.json
    返回 {carrier_name: {ok, latency_ms, error, ts}}
    """

def save_cache(results: dict, cache_path: str | None = None) -> None:
    """将探测结果（带时间戳）写入缓存文件。

    自动添加 ts 字段（ISO 格式 UTC 时间）
    """

def healthy_carriers(results: dict) -> list[str]:
    """从 probe_all 结果中筛出 ok=True 的 carrier 名列表，按 latency_ms 升序。"""
```

## 测试 golden 案例（必须全部通过）

1. `probe_all([], 'http://x', 'k')` → `{}`
2. `healthy_carriers({'glm': {'ok': True, 'latency_ms': 50.0, 'error': ''}, 'bad': {'ok': False, 'latency_ms': 0.0, 'error': 'timeout'}})` → `['glm']`
3. `healthy_carriers({'slow': {'ok': True, 'latency_ms': 200.0, 'error': ''}, 'fast': {'ok': True, 'latency_ms': 30.0, 'error': ''}})` → `['fast', 'slow']`（latency 升序）
4. `healthy_carriers({})` → `[]`
5. `probe('unreachable-carrier', 'http://127.0.0.1:19999', 'fake-key', timeout=1)['ok']` → `False`
6. `probe('unreachable-carrier', 'http://127.0.0.1:19999', 'fake-key', timeout=1)['latency_ms']` → `0.0`
7. `load_cache('/tmp/nonexistent_health_xyz.json')` → `{}`
8. save_cache + load_cache 往返：写入后能读回，且有 ts 字段
9. `healthy_carriers({'a': {'ok': True, 'latency_ms': 100.0, 'error': ''}})` → `['a']`
10. probe 返回的 dict 必须包含 ok / latency_ms / error 三个字段

## 约束
- 纯标准库（urllib），不依赖第三方包
- probe 函数不应依赖 carrier_router 或其他 devkit 模块（standalone）
- 只输出 `carrier_health.py` 文件内容，不需要测试文件
