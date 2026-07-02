# Task: devkit/setup.py — 一键设置向导

## 背景
Loom 现在需要让小白用户（不懂 YAML、不懂 LiteLLM）也能一键部署自治 Agent Team。
第一步是一个交互式设置向导，引导用户填写 API Key，启动 LiteLLM 网关，验证可用性。

## 要实现的文件
`devkit/setup.py`（纯标准库，不依赖第三方包）

## 函数签名（必须完全符合）

```python
def detect_docker() -> bool:
    """检测 Docker 是否可用（能跑 docker info）。返回 bool。"""

def detect_env_key(key_name: str) -> str:
    """从环境变量或 .env 文件读取 key，返回值或空字符串。"""

def write_env_file(keys: dict, env_path: str | None = None) -> None:
    """将 {KEY: value} 写入 agent-platform/.env 文件（追加或更新已有行）。"""

def start_litellm(compose_file: str | None = None) -> dict:
    """调用 docker compose up -d litellm，返回 {ok: bool, output: str}。"""

def check_gateway(base_url: str = "http://localhost:4000", timeout: int = 10) -> dict:
    """检查 LiteLLM 网关是否可达，返回 {ok: bool, latency_ms: float, error: str}。"""

def run_setup(interactive: bool = True, keys: dict | None = None) -> dict:
    """主设置流程，返回 {ok: bool, steps: list[{name, ok, message}]}。
    
    流程：
    1. 检测已有 key（detect_env_key）
    2. interactive=True 时对缺失 key 提示用户输入（input()）
    3. 写入 .env 文件（write_env_file）
    4. 检测 Docker 并启动 LiteLLM（start_litellm）
    5. 验证网关可达（check_gateway）
    每步结果追加到 steps 列表，任意步骤失败不中断（继续尝试下一步）。
    """
```

## Golden 案例（必须全部通过）

1. `detect_docker()` 返回 bool 类型
2. `detect_env_key("MINIMAX_API_KEY_NONEXISTENT_XYZ")` 返回 `""`（不存在的 key）
3. `write_env_file({"TEST_KEY_XYZ": "test_val"}, "/tmp/test_loom_env.env")` 后 `open("/tmp/test_loom_env.env").read()` 包含 `"TEST_KEY_XYZ=test_val"`
4. `check_gateway("http://127.0.0.1:19999", timeout=1)["ok"]` → `False`（不可达端口）
5. `check_gateway("http://127.0.0.1:19999", timeout=1)["latency_ms"]` → `0.0`
6. `run_setup(interactive=False, keys={})["steps"]` 是 list
7. `run_setup(interactive=False, keys={})` 的 steps 中每项有 name/ok/message 三个字段
8. `run_setup(interactive=False, keys={"MINIMAX_API_KEY": "test"})["steps"]` 长度 >= 3
9. `check_gateway("http://127.0.0.1:19999", timeout=1)` 返回包含 error 字段的 dict
10. `write_env_file` 多次写同一 key 不产生重复行（更新而非追加）

## 约束
- 纯标准库（subprocess, pathlib, urllib），不依赖第三方
- interactive=False 时不调用 input()
- Docker 不可用时 start_litellm 返回 {ok: False, output: "Docker 不可用"}，不抛异常
- 只输出 `setup.py` 文件内容，不需要测试文件
