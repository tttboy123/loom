# Task: P2 Safety Gate — `devkit/safety.py` + `--safety` flag

## 背景
`devkit` R&D loop 在 `build/` 目录物化代码后无安全扫描。目标：新建 `devkit/safety.py` 模块，
对 build 产物做轻量安全检查（硬编码密钥 + 危险 shell + SQL 注入模式），
并在 `devkit/__main__.py` 新增 `--safety` flag（跑完 loop 后自动扫 build/）和 `devkit safety <run-id>` 子命令。

## 文件 A：新建 `devkit/safety.py`（纯新文件）

```python
# devkit/safety.py
"""轻量安全扫描：对 build/ 产物检查硬编码密钥 / shell 注入 / SQL 注入模式。"""
```

### 扫描规则（4 条，按 rule_id 排序）

| rule_id | 名称 | 正则（Python re，IGNORECASE） | 说明 |
|---|---|---|---|
| S001 | hardcoded-secret | `[A-Z_]{3,}_(?:KEY\|TOKEN\|SECRET\|PASSWORD)\s*=\s*['"][A-Za-z0-9+/=_\-]{20,}['"]` | 疑似硬编码密钥/token |
| S002 | shell-injection | `subprocess\.(?:run\|call\|Popen\|check_output)\([^)]*shell\s*=\s*True` | shell=True 可注入 |
| S003 | os-system | `os\.system\s*\(` | os.system 直接执行 shell |
| S004 | sql-concat | `(?:execute\|executemany)\s*\([^)]*\+` | SQL 字符串拼接（注入风险） |

### 主函数

```python
def scan_build(build_dir: pathlib.Path) -> dict:
    """扫描 build_dir 下所有 .py 文件，返回：
    {
        "ok": bool,           # True = 无违规
        "violations": [
            {"file": str, "line": int, "rule": str, "snippet": str},
            ...
        ],
        "scanned_files": int,  # 扫描的 .py 文件数
        "rules_applied": int,  # 启用的规则数（固定为 4）
    }
    缺目录或空目录 → {"ok": True, "violations": [], "scanned_files": 0, "rules_applied": 4}
    所有文件读取异常在内部捕获，不抛出。
    """
```

实现要点：
- 用 `re` 模块（标准库），每条规则编译一次
- 递归扫 `.py` 文件（`build_dir.rglob("*.py")`）
- 跳过 `__pycache__` 目录
- 逐行扫描：匹配到就记录 `{"file": 相对路径, "line": 行号(1-based), "rule": rule_id, "snippet": 该行[:120].strip()}`
- 一行可触发多条规则（多条都记录）
- `ok = len(violations) == 0`

## 文件 B：修改 `devkit/__main__.py`

### 1. 在 `main()` 里加路由（在 `if argv and argv[0] == "runs":` 之前）：
```python
if argv and argv[0] == "safety":
    return _cmd_safety(argv[1:])
```

### 2. 新增 `_cmd_safety(argv) -> int` 函数（加在 `_cmd_recommend` 之前）：
```python
def _cmd_safety(argv) -> int:
    """扫描某次 run 的 build/ 产物，检查硬编码密钥 / shell 注入等安全问题。"""
    from devkit import safety
    from devkit.rdloop import ROOT
    p = argparse.ArgumentParser(prog="devkit safety",
                                description="安全扫描：对 build/ 产物检查密钥 / 注入等问题")
    p.add_argument("run_id", help="run id（devkit/runs/<run-id>）")
    p.add_argument("--runs-dir", default=str(ROOT / "devkit" / "runs"))
    a = p.parse_args(argv)
    build_dir = pathlib.Path(a.runs_dir) / a.run_id / "build"
    if not build_dir.is_dir():
        print(f"找不到 build 目录：{build_dir}")
        return 1
    r = safety.scan_build(build_dir)
    print(f"扫描 {r['scanned_files']} 个文件 · {r['rules_applied']} 条规则")
    if r["ok"]:
        print("✅ 无安全问题")
        return 0
    print(f"⚠️  发现 {len(r['violations'])} 处违规：\n")
    for v in r["violations"]:
        print(f"  [{v['rule']}] {v['file']}:{v['line']}  {v['snippet']}")
    return 1
```

### 3. 在 `_cmd_run()` 里，`run_loop` 调用之后、`return` 之前加：
```python
if getattr(args, "safety", False):
    from devkit import safety as _safety
    from devkit.rdloop import ROOT as _ROOT
    _runs_dir = _ROOT / "devkit" / "runs"
    _run_id = res.get("run_id") or ""
    if _run_id:
        _build = _runs_dir / _run_id / "build"
        if _build.is_dir():
            _sr = _safety.scan_build(_build)
            if not _sr["ok"]:
                print(f"\n⚠️  Safety: {len(_sr['violations'])} 处违规（devkit safety {_run_id} 查详情）")
```

### 4. 在 `_cmd_run()` 的 argparse 里加 `--safety` 参数（加在 `--ponytail` 之后）：
```python
p.add_argument("--safety", action="store_true",
               help="跑完后自动对 build/ 做安全扫描（检查硬编码密钥 / shell 注入等）")
```

## 约束
- 新建 `devkit/safety.py`（只用标准库 re / pathlib）
- 只修改 `devkit/__main__.py`（不改其他文件）
- 所有文件读取异常在 safety.py 内部捕获，不抛出
- 不写 unittest 块
- 输出两个代码块，分别以 `# devkit/safety.py` 和 `# devkit/__main__.py` 开头，产出完整文件
- 网关：http://localhost:4000
- 级别：L1 / report-only
