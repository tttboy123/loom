"""
把阶段产出的代码「物化」成文件 → 在沙箱里跑测试 → （人类门下）apply 到目标。

- materialize(text, dest): 从 markdown 文本抽出代码文件（按文件名标记），写进 dest。
- run_tests(dir): dest 里有 test_*.py 就用 unittest 跑，返回 (passed|None, summary)。
- apply_files(src, target): 把沙箱文件复制到 target —— **仅当用户显式 --apply 才调用（人类门）**。

对齐 Constitution：默认 report-only；测试不绿不建议 apply；apply 是人类显式动作。
"""
from __future__ import annotations

import os
import pathlib
import ast
import re
import shutil
import subprocess
import sys
from typing import List, Optional, Tuple, Mapping, Iterable


class MaterializeAstError(Exception):
    """Python 产物在写盘前/后未通过 AST 校验。"""

    def __init__(self, failures: list[dict]):
        super().__init__("materialized python failed AST validation")
        self.failures = failures

_LANG = r"(?:python|py|ts|tsx|js|jsx|javascript|go|rust|rs|bash|sh|toml|text|txt|md|markdown|json|ya?ml)?"
_FILE_MARKER_RE = (
    r"(?:\*\*文件\s*`(?P<file1>[^`]+)`\*\*|"
    r"^FILE:\s*`?(?P<file5>[\w./-]+\.[A-Za-z]+)`?|"
    r"文件[:：]\s*`?(?P<file2>[\w./-]+\.[A-Za-z]+)`?|"
    r"^#{2,4}\s*FILE:\s*`?(?P<file6>[\w./-]+\.[A-Za-z]+)`?|"
    r"^#{2,4}[^\n`]*?`(?P<file3>[\w./-]+\.[A-Za-z]+)`[^\n]*|"
    r"^#{2,4}\s*(?P<file7>[\w./-]+\.[A-Za-z]+)\s*$|"
    r"^[^\n`]*`(?P<file4>[\w./-]+\.[A-Za-z]+)`[^\n]*[:：]?\s*)"
)
# pytest 等测试依赖的共享缓存：装一次、所有 run_tests 复用 → 测试判定确定、不每次重装
_SHARED_DEPS = pathlib.Path(os.environ.get("LOOM_PYDEPS", str(pathlib.Path.home() / ".loom" / "pydeps")))
_FENCE_RE = re.compile(r"`{3,}\s*([\w+-]*)\n(.*?)(?:\n`{3,}|\Z)", re.S)
_VERIFY_CMD_RE = re.compile(r"^\s*(python3?|pytest)\b.*$", re.M)
_REQUIREMENT_SPEC_RE = re.compile(r"^[A-Za-z0-9_.-]+\s*(?:==|>=|<=|~=|>|<)\s*[^#\s].*$")
_PYTEST_REQUIREMENT_CMD_RE = re.compile(r"^python(?:3)?\s+-m\s+pytest\s*(?:==|>=|<=|~=|>|<)\s*.+$")
_VERIFY_TAIL_CUTOFFS = ("```", "## ", "\n## ")
_KNOWN_TEST_IMPORT_PACKAGES = {
    "pytest": "pytest",
    "yaml": "PyYAML",
}


def _discover_repo_root(start: pathlib.Path) -> Optional[pathlib.Path]:
    """尽量推断仓库根，用于把真实包目录注入 PYTHONPATH。"""
    cur = pathlib.Path(start).resolve()
    for cand in [cur, *cur.parents]:
        if (cand / "devkit" / "__init__.py").is_file():
            return cand
    return None


def _install_requirement_file(req: pathlib.Path, deps: pathlib.Path, note: str) -> str:
    if not req.exists():
        return note
    try:
        _pip_target(["-r", str(req)], deps)
        return note + f"（已尝试安装 {req.name} → _deps）\n"
    except Exception:  # noqa: BLE001
        return note + f"（{req.name} 安装失败，按无依赖跑）\n"


def _bootstrap_known_test_imports(
    env: dict,
    deps: pathlib.Path,
    shared: pathlib.Path,
    missing_modules: Iterable[str],
) -> str:
    note = ""
    installed: list[str] = []
    seen: set[str] = set()
    for module in missing_modules:
        top = str(module or "").split(".", 1)[0].strip()
        package = _KNOWN_TEST_IMPORT_PACKAGES.get(top)
        if not package or package in seen:
            continue
        seen.add(package)
        try:
            shared.mkdir(parents=True, exist_ok=True)
            _pip_target([package], shared)
            installed.append(package)
        except Exception:  # noqa: BLE001
            note += f"（测试依赖自举失败：{package}）\n"
    if installed:
        cur = env.get("PYTHONPATH", "")
        parts = [x for x in cur.split(os.pathsep) if x]
        for p in (deps, shared):
            sp = str(p)
            if pathlib.Path(sp).is_dir() and sp not in parts:
                parts.insert(0, sp)
        env["PYTHONPATH"] = os.pathsep.join(parts)
        note += f"（已自举测试依赖：{', '.join(installed)}）\n"
    return note


def _safe_relpath(path: str) -> Optional[str]:
    """保留子目录（多文件项目），但拒绝 `..` / 绝对路径（防穿越）。"""
    raw = path.strip().strip("`").replace("\\", "/")
    p = pathlib.PurePosixPath(raw)
    if p.is_absolute() or ".." in p.parts or not p.parts:
        return None
    parts = list(p.parts)
    if parts and parts[0] == "build":
        parts = parts[1:]
    if not parts:
        return None
    return str(pathlib.PurePosixPath(*parts))


def _fenced_blocks(text: str) -> list[tuple[str, str]]:
    """提取 fenced code block，允许末尾被截断时无闭合围栏。"""
    return [((lang or "").lower(), body) for lang, body in _FENCE_RE.findall(text or "")]


def materialize(text: str, dest: pathlib.Path) -> List[str]:
    """从文本抽出 {文件名 → 代码} 写入 dest。返回写入的文件名列表。"""
    dest = pathlib.Path(dest)
    dest.mkdir(parents=True, exist_ok=True)
    written = extract_materialization_map(text)
    failures = _validate_python_files(written)
    if failures:
        raise MaterializeAstError(failures)

    for name, body in written.items():
        f = dest / name
        f.parent.mkdir(parents=True, exist_ok=True)   # 多文件/子目录
        f.write_text(_sanitize_body_for_path(name, body) + "\n", encoding="utf-8")
    persisted_failures = _validate_python_files_from_disk(dest, list(written.keys()))
    if persisted_failures:
        for name in written:
            target = dest / name
            if target.exists():
                target.unlink()
        raise MaterializeAstError(persisted_failures)
    return list(written.keys())


def materialize_declared_artifact(text: str, dest: pathlib.Path, artifact_path: str | None) -> List[str]:
    """Materialize a single declared report artifact from fenced content.

    Used by artifact_json/report-only tasks where the model emits one fenced
    payload but omits explicit FILE markers.
    """
    target = _safe_relpath(artifact_path or "")
    if not target:
        return []
    file_map = _extract_declared_artifact_map(text, target)
    if not file_map:
        return []
    dest = pathlib.Path(dest)
    dest.mkdir(parents=True, exist_ok=True)
    for name, body in file_map.items():
        f = dest / name
        f.parent.mkdir(parents=True, exist_ok=True)
        f.write_text(_sanitize_body_for_path(name, body) + "\n", encoding="utf-8")
    return list(file_map.keys())


def extract_materialization_map(text: str) -> dict[str, str]:
    """从文本中抽出候选文件映射，但不写盘。"""
    written: dict[str, str] = {}

    # A) 文件名标记（**文件 `x`** / 文件：x / ### `x` / #### x）紧跟围栏（支持子目录路径）
    for m in re.finditer(
        _FILE_MARKER_RE + r"\s*\n+`{3,}" + _LANG + r"\n(?P<body>.*?)(?:\n`{3,}|\Z)",
        text, re.S | re.M,
    ):
        name = _safe_relpath(
            m.group("file1") or m.group("file2") or m.group("file3") or m.group("file4") or m.group("file5") or m.group("file6") or m.group("file7")
        )
        if name:
            written.setdefault(name, m.group("body"))

    # B) 围栏首行是 `# path/x.py` / `// x.ts` / `FILE: path/x.py` 元数据行
    for m in re.finditer(
        r"`{3,}" + _LANG + r"\n(?P<meta>(?://|#|FILE:)\s*(?P<path>[\w./-]+\.[A-Za-z]+)[^\n]*)\n(?P<body>.*?)(?:\n`{3,}|\Z)",
        text,
        re.S,
    ):
        name = _safe_relpath(m.group("path"))
        if name:
            written.setdefault(name, m.group("body"))

    # C) 兜底：A/B 都没识别出文件名，但确有 python 代码块 → 推断文件名（否则 0 文件白跑）
    if not written:
        for name, body in _salvage_unmarked(text).items():
            safe = _safe_relpath(name)
            if safe:
                written.setdefault(safe, body)
    if not written:
        for name, body in _salvage_report_block(text).items():
            safe = _safe_relpath(name)
            if safe:
                written.setdefault(safe, body)
    return written


def diagnose_materialization(text: str, files: List[str]) -> dict:
    """给 0 文件 / 格式失配提供稳定 failure code，便于自治链路排障。"""
    body = text or ""
    has_text = bool(body.strip())
    fenced_blocks = _fenced_blocks(body)
    has_code_fence = bool(fenced_blocks)
    has_python_block = any((lang or "").lower() in ("", "python", "py") for lang, _ in fenced_blocks)
    has_file_marker = bool(re.search(_FILE_MARKER_RE, body, re.M))
    if files:
        return {
            "status": "materialized",
            "failure_code": None,
            "file_count": len(files),
            "files": list(files),
            "has_code_fence": has_code_fence,
            "has_python_block": has_python_block,
            "has_file_marker": has_file_marker,
        }
    if not has_text:
        code = "MATERIALIZE_EMPTY_TEXT"
    elif has_code_fence and not has_file_marker:
        code = "FORMAT_MISMATCH_NO_FILE_MARKERS"
    elif has_python_block:
        code = "FORMAT_MISMATCH_UNRECOGNIZED_PYTHON"
    elif has_code_fence:
        code = "FORMAT_MISMATCH_UNSUPPORTED_BLOCKS"
    else:
        code = "MATERIALIZE_NO_CODE_BLOCKS"
    return {
        "status": "missing",
        "failure_code": code,
        "file_count": 0,
        "files": [],
        "has_code_fence": has_code_fence,
        "has_python_block": has_python_block,
        "has_file_marker": has_file_marker,
    }


def _infer_lang_from_path(path: str) -> str:
    ext = pathlib.Path(path).suffix.lower()
    return {
        ".py": "python",
        ".md": "markdown",
        ".markdown": "markdown",
        ".txt": "text",
        ".json": "json",
        ".yaml": "yaml",
        ".yml": "yaml",
        ".sh": "bash",
        ".bash": "bash",
        ".toml": "toml",
        ".js": "javascript",
        ".jsx": "javascript",
        ".ts": "typescript",
        ".tsx": "typescript",
    }.get(ext, ext.lstrip(".") or "text")


def _fences_balanced(text: str) -> bool:
    return (text or "").count("```") % 2 == 0


def _python_body_truncation_reason(body: str) -> Optional[str]:
    try:
        ast.parse(body)
        return None
    except SyntaxError as exc:
        msg = str(exc)
        if any(sig in msg for sig in (
            "unexpected EOF while parsing",
            "unterminated string literal",
            "was never closed",
            "expected an indented block",
        )):
            return msg
    return None


def _file_completion(path: str, body: str) -> tuple[bool, Optional[str]]:
    lang = _infer_lang_from_path(path)
    cleaned = _sanitize_body_for_path(path, body)
    if lang == "python":
        reason = _python_body_truncation_reason(cleaned)
        return (reason is None, reason)
    return (True, None)


def extract_verify_commands(text: str) -> list[str]:
    def _normalize_verify_command(line: str) -> str:
        stripped = line.strip()
        for marker in _VERIFY_TAIL_CUTOFFS:
            pos = stripped.find(marker)
            if pos > 0:
                stripped = stripped[:pos].rstrip()
        if stripped.startswith("pytest"):
            rest = stripped[len("pytest"):].lstrip()
            stripped = f"python -m pytest{(' ' + rest) if rest else ''}"
        return stripped

    def _is_verify_command(line: str) -> bool:
        stripped = _normalize_verify_command(line)
        if not stripped or stripped.startswith("#"):
            return False
        if _REQUIREMENT_SPEC_RE.match(stripped):
            return False
        if _PYTEST_REQUIREMENT_CMD_RE.match(stripped):
            return False
        return bool(re.match(r"^\s*python(?:3)?\b.*$", stripped))

    seen: set[str] = set()
    cmds: list[str] = []
    for lang, body in _fenced_blocks(text or ""):
        if lang not in {"bash", "sh", "shell"}:
            continue
        for line in body.splitlines():
            stripped = _normalize_verify_command(line)
            if _is_verify_command(stripped) and stripped not in seen:
                seen.add(stripped)
                cmds.append(stripped)
    for match in _VERIFY_CMD_RE.finditer(text or ""):
        stripped = _normalize_verify_command(match.group(0))
        if _is_verify_command(stripped) and stripped not in seen:
            seen.add(stripped)
            cmds.append(stripped)
    return cmds


def build_output_protocol(
    text: str,
    *,
    materialized_files: Optional[List[str]] = None,
    response_diag: Optional[Mapping] = None,
) -> dict:
    file_map = extract_materialization_map(text)
    ordered_paths = list(materialized_files or file_map.keys())
    files: list[dict] = []
    for path in ordered_paths:
        body = file_map.get(path, "")
        complete, reason = _file_completion(path, body)
        files.append({
            "path": path,
            "language": _infer_lang_from_path(path),
            "line_count": len(_sanitize_body_for_path(path, body).splitlines()) if body else None,
            "byte_count": len(_sanitize_body_for_path(path, body).encode("utf-8")) if body else None,
            "complete_guess": complete,
            "truncation_reason": reason,
        })
    fences_balanced = _fences_balanced(text or "")
    finish_reason = (response_diag or {}).get("finish_reason")
    suggested_continue = bool(
        (finish_reason == "length")
        or (not fences_balanced and bool(text or ""))
        or any(not f["complete_guess"] for f in files)
    )
    return {
        "schema_version": 1,
        "files": files,
        "verify_commands": extract_verify_commands(text),
        "fences_balanced": fences_balanced,
        "finish_reason": finish_reason,
        "suggested_continue": suggested_continue,
    }


def _looks_test(body: str) -> bool:
    return bool(re.search(r"\bdef test_|\bclass Test|import pytest|import unittest", body))


def _safe_name(name: str) -> str:
    """把推断出的模块名清成安全标识符（防穿越/怪字符）。"""
    s = re.sub(r"\W", "", str(name or ""))
    return (s[:40] or "solution")


def _salvage_unmarked(text: str) -> dict:
    """模型没标文件名时，从代码块推断 {文件名→代码}：实现名取测试的 `from X import` 以便能 import 到。"""
    blocks = _fenced_blocks(text)
    py = [(lang, body) for lang, body in blocks
          if lang in ("python", "py", "")
          and ("def " in body or "class " in body or "import " in body)]
    if not py:
        return {}
    tests = [b for _l, b in py if _looks_test(b)]
    impls = [b for _l, b in py if not _looks_test(b)]
    mod = "solution"
    for b in tests:                                  # 从测试 import 推断实现模块名（支持点路径）
        m = re.search(r"from\s+([\w.]+)\s+import", b)
        if m:
            mod = m.group(1)
            break
    segs = [_safe_name(s) for s in mod.split(".") if s.strip()] or ["solution"]
    impl_path = "/".join(segs) + ".py"               # pkg.reverse → pkg/reverse.py，让测试能 import 到
    test_base = segs[-1]
    out: dict = {}
    if impls:
        out[impl_path] = impls[0]
    elif not tests:                                  # 只有非测试块
        out[impl_path] = py[0][1]
    for i, b in enumerate(tests):
        out[f"tests/test_{test_base}{'' if i == 0 else i}.py"] = b
    return out


def _salvage_report_block(text: str) -> dict:
    """报告类任务兜底：有文件路径提示，但代码块前夹了说明文字。"""
    marker = re.search(_FILE_MARKER_RE, text or "", re.M)
    if not marker:
        return {}
    path = (
        marker.group("file1")
        or marker.group("file2")
        or marker.group("file3")
        or marker.group("file4")
        or marker.group("file5")
        or marker.group("file6")
        or marker.group("file7")
    )
    if not path:
        return {}
    blocks = _fenced_blocks(text or "")
    if not blocks:
        return {}
    if len(blocks) == 1:
        return {path: blocks[0][1]}
    for lang, body in blocks:
        if lang in {"md", "markdown", "text", "txt", "json", "yaml", "yml"}:
            return {path: body}
    return {}


def _extract_declared_artifact_map(text: str, artifact_path: str) -> dict[str, str]:
    blocks = _fenced_blocks(text or "")
    if not blocks:
        return {}
    wanted = _infer_lang_from_path(artifact_path)
    if len(blocks) == 1:
        lang, body = blocks[0]
        if wanted in {"json", "yaml", "markdown", "text"} or lang in {"", wanted}:
            return {artifact_path: body}
    for lang, body in blocks:
        if wanted == "json" and lang == "json":
            return {artifact_path: body}
        if wanted == "yaml" and lang in {"yaml", "yml"}:
            return {artifact_path: body}
        if wanted == "markdown" and lang in {"md", "markdown", "text", "txt"}:
            return {artifact_path: body}
    return {}


def _clean_body(body: str) -> str:
    """清掉漏进文件内容的 markdown 围栏行（``` / ```py），避免污染产物造成 SyntaxError。"""
    lines = [ln for ln in body.splitlines() if not re.match(r"^\s*`{3,}\s*[\w+-]*\s*$", ln)]
    return "\n".join(lines).rstrip()


def _sanitize_body_for_path(path: str, body: str) -> str:
    cleaned = _clean_body(body)
    lang = _infer_lang_from_path(path)
    if lang not in {"markdown", "text"} and "```" in cleaned:
        restart_match = re.search(r"```[\w+-]*\n(?://|#|FILE:)\s*[\w./-]+\.[A-Za-z]+", cleaned)
        if restart_match is not None:
            cleaned = cleaned[: restart_match.start()].rstrip()
        cleaned = re.sub(r"```[\w+-]*", "", cleaned).rstrip()
    return cleaned


def _validate_python_files(file_map: Mapping[str, str]) -> list[dict]:
    failures: list[dict] = []
    for path, body in file_map.items():
        if _infer_lang_from_path(path) != "python":
            continue
        sanitized = _sanitize_body_for_path(path, body)
        try:
            ast.parse(sanitized, filename=path)
        except SyntaxError as exc:
            lines = sanitized.splitlines()
            line_index = (exc.lineno or 1) - 1
            snippet = lines[line_index] if 0 <= line_index < len(lines) else ""
            failures.append({
                "path": path,
                "line": exc.lineno,
                "offset": exc.offset,
                "message": exc.msg,
                "snippet": snippet[:160],
            })
    return failures


def _validate_python_files_from_disk(dest: pathlib.Path, files: list[str]) -> list[dict]:
    failures: list[dict] = []
    for path in files:
        if _infer_lang_from_path(path) != "python":
            continue
        source = ""
        target = pathlib.Path(dest) / path
        try:
            source = target.read_text(encoding="utf-8")
            ast.parse(source, filename=path)
        except SyntaxError as exc:
            lines = source.splitlines()
            line_index = (exc.lineno or 1) - 1
            snippet = lines[line_index] if 0 <= line_index < len(lines) else ""
            failures.append({
                "path": path,
                "line": exc.lineno,
                "offset": exc.offset,
                "message": exc.msg,
                "snippet": snippet[:160],
            })
        except OSError as exc:
            failures.append({
                "path": path,
                "line": None,
                "offset": None,
                "message": f"{type(exc).__name__}: {exc}",
                "snippet": "",
            })
    return failures


def _pip_target(args: list, deps: pathlib.Path, timeout: int = 180) -> None:
    """pip install --target（绕过 PEP 668 system-managed 限制）。"""
    subprocess.run([sys.executable, "-m", "pip", "install", "--target", str(deps),
                    "-q", "--disable-pip-version-check"] + args,
                   capture_output=True, text=True, timeout=timeout)


def _has_pytest(env: dict) -> bool:
    try:
        return subprocess.run([sys.executable, "-m", "pytest", "--version"],
                              capture_output=True, env=env, timeout=30).returncode == 0
    except Exception:  # noqa: BLE001
        return False


def detect_stdlib_shadowing(dest: pathlib.Path) -> List[str]:
    """识别会遮蔽标准库模块的本地文件，如 pathlib.py / json.py。"""
    dest = pathlib.Path(dest)
    stdlib_names = set(getattr(sys, "stdlib_module_names", set()))
    hits: List[str] = []
    for f in sorted(dest.rglob("*.py")):
        rel = f.relative_to(dest)
        if (any(p.startswith("_") or p.startswith(".") or p == "__pycache__" for p in rel.parts)
                or rel.name == "__init__.py"):
            continue
        stem = f.stem
        if stem in stdlib_names:
            hits.append(str(rel))
    return hits


def _prepare_test_env(dest: pathlib.Path) -> tuple[dict, pathlib.Path, pathlib.Path, str]:
    """准备测试环境，供 collect/run 共享，避免两边逻辑漂移。"""
    env = dict(os.environ)
    deps = dest / "_deps"
    shared = _SHARED_DEPS
    note = ""
    test_dirs = sorted({
        str(path.parent)
        for path in list(dest.rglob("test_*.py")) + list(dest.rglob("*_test.py"))
    })

    def _addpath(*ps):
        cur = env.get("PYTHONPATH", "")
        parts = [x for x in cur.split(os.pathsep) if x]
        for p in ps:
            sp = str(p)
            if pathlib.Path(sp).is_dir() and sp not in parts:
                parts.insert(0, sp)
        env["PYTHONPATH"] = os.pathsep.join(parts)

    repo_root = _discover_repo_root(dest)
    _addpath(*test_dirs, deps, shared)
    if repo_root is not None:
        _addpath(repo_root)

    req = dest / "requirements.txt"
    note = _install_requirement_file(req, deps, note)
    if repo_root is not None:
        note = _install_requirement_file(repo_root / "requirements-dev.txt", deps, note)
        note = _install_requirement_file(repo_root / "devkit" / "requirements-dev.txt", deps, note)
    _addpath(deps, shared)

    return env, deps, shared, note


def _ensure_pytest(env: dict, deps: pathlib.Path, shared: pathlib.Path) -> tuple[Optional[str], str]:
    """返回 runner 名称与补充说明。"""
    note = ""
    if _has_pytest(env):
        return "pytest", note
    try:
        shared.mkdir(parents=True, exist_ok=True)
        _pip_target(["pytest"], shared)
        cur = env.get("PYTHONPATH", "")
        parts = [x for x in cur.split(os.pathsep) if x]
        for p in (deps, shared):
            sp = str(p)
            if pathlib.Path(sp).is_dir() and sp not in parts:
                parts.insert(0, sp)
        env["PYTHONPATH"] = os.pathsep.join(parts)
        if _has_pytest(env):
            return "pytest", "（pytest 已就绪 → 共享缓存）\n"
    except Exception:  # noqa: BLE001
        pass
    return None, note


def collect_tests(dest: pathlib.Path, timeout: int = 120) -> dict:
    """收集测试数量，给 0 collect / collect 异常输出稳定 failure code。"""
    dest = pathlib.Path(dest)
    tests = list(dest.rglob("test_*.py")) + list(dest.rglob("*_test.py"))
    if not tests:
        return {
            "ok": False,
            "runner": None,
            "collected": 0,
            "output": "（沙箱无 test_*.py，collect=0）",
            "failure_code": "TEST_COLLECT_NONE",
        }

    for test_path in tests:
        try:
            source = test_path.read_text(encoding="utf-8")
            ast.parse(source, filename=str(test_path))
        except SyntaxError as exc:
            return {
                "ok": False,
                "runner": None,
                "collected": 0,
                "output": f"（测试文件语法错误：{test_path.relative_to(dest)}: {exc}）",
                "failure_code": "TEST_SYNTAX_ERROR",
            }
        except OSError as exc:
            return {
                "ok": False,
                "runner": None,
                "collected": 0,
                "output": f"（读取测试文件失败：{test_path.relative_to(dest)}: {type(exc).__name__}: {exc}）",
                "failure_code": "TEST_COLLECT_ERROR",
            }

    shadowed = detect_stdlib_shadowing(dest)
    if shadowed:
        joined = ", ".join(shadowed)
        return {
            "ok": False,
            "runner": None,
            "collected": 0,
            "output": f"（标准库同名文件遮蔽：{joined}）",
            "failure_code": "STDLIB_SHADOWING",
        }

    env, deps, shared, note = _prepare_test_env(dest)
    preflight = _check_test_imports_preflight(dest)
    bootstrap_missing_modules = list(preflight.get("bootstrap_missing_modules", []))
    note += _bootstrap_known_test_imports(env, deps, shared, bootstrap_missing_modules)
    if bootstrap_missing_modules:
        preflight = _check_test_imports_preflight(dest, extra_paths=[deps, shared])
    if not preflight["ok"]:
        preflight["output"] = note + str(preflight.get("output", ""))
        return preflight
    runner, runner_note = _ensure_pytest(env, deps, shared)
    note += runner_note
    if runner == "pytest":
        cmd = [sys.executable, "-m", "pytest", "--collect-only", "-q", str(dest)]
        try:
            r = subprocess.run(cmd, cwd=str(dest), env=env, capture_output=True, text=True, timeout=timeout)
            raw = ((r.stdout or "") + (r.stderr or "")).strip()
            out = note + raw
            m = re.search(r"(\d+)\s+tests?\s+collected", raw)
            collected = int(m.group(1)) if m else 0
            failure_code = None
            if r.returncode != 0:
                failure_code = "TEST_COLLECT_ERROR"
            elif collected == 0:
                failure_code = "TEST_COLLECT_NONE"
            return {
                "ok": r.returncode == 0 and collected > 0,
                "runner": "pytest",
                "collected": collected,
                "output": out[-1800:],
                "failure_code": failure_code,
            }
        except subprocess.TimeoutExpired:
            return {
                "ok": False,
                "runner": "pytest",
                "collected": 0,
                "output": note + f"（collect 超时 > {timeout}s）",
                "failure_code": "TEST_COLLECT_TIMEOUT",
            }
        except Exception as e:  # noqa: BLE001
            return {
                "ok": False,
                "runner": "pytest",
                "collected": 0,
                "output": note + f"（collect 异常：{type(e).__name__}: {e}）",
                "failure_code": "TEST_COLLECT_ERROR",
            }

    script = (
        "import sys, unittest; "
        "suite = unittest.defaultTestLoader.discover(sys.argv[1], pattern='test_*.py', top_level_dir=sys.argv[1]); "
        "print(suite.countTestCases())"
    )
    try:
        r = subprocess.run(
            [sys.executable, "-c", script, str(dest)],
            cwd=str(dest), env=env, capture_output=True, text=True, timeout=timeout,
        )
        raw = ((r.stdout or "") + (r.stderr or "")).strip()
        first = raw.splitlines()[0].strip() if raw.splitlines() else "0"
        collected = int(first) if first.isdigit() else 0
        failure_code = None
        if r.returncode != 0:
            failure_code = "TEST_COLLECT_ERROR"
        elif collected == 0:
            failure_code = "TEST_COLLECT_NONE"
        return {
            "ok": r.returncode == 0 and collected > 0,
            "runner": "unittest",
            "collected": collected,
            "output": (note + raw)[-1800:],
            "failure_code": failure_code,
        }
    except subprocess.TimeoutExpired:
        return {
            "ok": False,
            "runner": "unittest",
            "collected": 0,
            "output": note + f"（collect 超时 > {timeout}s）",
            "failure_code": "TEST_COLLECT_TIMEOUT",
        }
    except Exception as e:  # noqa: BLE001
        return {
            "ok": False,
            "runner": "unittest",
            "collected": 0,
            "output": note + f"（collect 异常：{type(e).__name__}: {e}）",
            "failure_code": "TEST_COLLECT_ERROR",
        }


def run_tests(dest: pathlib.Path, timeout: int = 240) -> Tuple[Optional[bool], str]:
    """递归找测试运行（多文件/子目录）。优先 pytest（兼容 unittest 与 pytest 风格），
    缺则按需装到 _deps；再不行回退 unittest。有 requirements.txt 也尽力装。"""
    dest = pathlib.Path(dest)
    tests = list(dest.rglob("test_*.py")) + list(dest.rglob("*_test.py"))
    if not tests:
        return None, "（沙箱无 test_*.py，跳过测试）"
    env, deps, shared, note = _prepare_test_env(dest)
    preflight = _check_test_imports_preflight(dest)
    bootstrap_missing_modules = list(preflight.get("bootstrap_missing_modules", []))
    note += _bootstrap_known_test_imports(env, deps, shared, bootstrap_missing_modules)
    if bootstrap_missing_modules:
        preflight = _check_test_imports_preflight(dest, extra_paths=[deps, shared])
    if not preflight["ok"]:
        return False, note + str(preflight.get("output", ""))
    runner, runner_note = _ensure_pytest(env, deps, shared)
    note += runner_note

    if runner == "pytest":
        cmd = [sys.executable, "-m", "pytest", "-q", str(dest)]
    else:
        cmd = [sys.executable, "-m", "unittest", "discover", "-s", str(dest), "-t", str(dest),
               "-p", "test_*.py", "-v"]
        note += "（无 pytest，回退 unittest；pytest 风格用例可能 import 失败）\n"
    try:
        r = subprocess.run(cmd, cwd=str(dest), env=env, capture_output=True, text=True, timeout=timeout)
        out = note + ((r.stdout or "") + (r.stderr or "")).strip()
        return (r.returncode == 0), out[-1800:]
    except subprocess.TimeoutExpired:
        return False, note + f"（测试超时 > {timeout}s）"
    except Exception as e:  # noqa: BLE001
        return False, note + f"（测试执行异常：{type(e).__name__}: {e}）"


def _check_test_imports_preflight(
    dest: pathlib.Path,
    *,
    extra_paths: Iterable[pathlib.Path | str] = (),
) -> dict:
    from devkit import check_test_imports as _imports

    original_sys_path = list(sys.path)
    original_module_names = set(sys.modules)
    shadowed_modules: dict[str, object] = {}
    try:
        repo_root = _discover_repo_root(dest)
        test_dirs = sorted({
            str(path.parent)
            for path in list(pathlib.Path(dest).rglob("test_*.py")) + list(pathlib.Path(dest).rglob("*_test.py"))
        })
        extra = [str(path) for path in extra_paths if str(path)]
        preflight_paths = test_dirs + [str(dest)] + extra
        if repo_root is not None:
            preflight_paths.append(str(repo_root))
        sys.path[:] = preflight_paths + original_sys_path
        top_level = set()
        scan_roots = [pathlib.Path(dest)]
        if repo_root is not None:
            scan_roots.append(pathlib.Path(repo_root))
        for scan_root in scan_roots:
            for child in scan_root.iterdir():
                if child.is_dir() and (child / "__init__.py").exists():
                    top_level.add(child.name)
                elif child.is_file() and child.suffix == ".py":
                    top_level.add(child.stem)
        for name in list(sys.modules):
            if name.split(".", 1)[0] in top_level:
                shadowed_modules[name] = sys.modules.pop(name)
        report = _imports.check_directory(dest)
    finally:
        for name in list(sys.modules):
            if name not in original_module_names and name.split(".", 1)[0] in top_level:
                sys.modules.pop(name, None)
        sys.modules.update(shadowed_modules)
        sys.path[:] = original_sys_path

    if report.ok:
        return {"ok": True}
    local_roots: set[str] = set()
    scan_roots = [pathlib.Path(dest)]
    repo_root = _discover_repo_root(dest)
    if repo_root is not None:
        scan_roots.append(pathlib.Path(repo_root))
    for scan_root in scan_roots:
        for child in scan_root.iterdir():
            if child.name.startswith((".", "_deps", "__pycache__")):
                continue
            if child.is_dir():
                local_roots.add(child.name)
            elif child.is_file() and child.suffix == ".py":
                local_roots.add(child.stem)
    bootstrap_missing_modules = sorted({
        str(item.module or "").split(".", 1)[0].strip()
        for item in report.missing
        if (
            str(item.module or "").split(".", 1)[0].strip() in _KNOWN_TEST_IMPORT_PACKAGES
            and f"No module named '{str(item.module or '').split('.', 1)[0].strip()}'" in (item.error or "")
        )
    })
    if report.missing and len(bootstrap_missing_modules) == len(report.missing):
        # Defer known runner/test dependencies to bootstrap. Preflight should
        # stop on project import issues, not missing shared test packages.
        return {"ok": True, "bootstrap_missing_modules": bootstrap_missing_modules}
    symbol_only = bool(report.missing) and all("symbol missing" in (m.error or "") for m in report.missing)
    local_missing = bool(report.missing) and all(
        (
            "ModuleNotFoundError" in (m.error or "")
            and (m.module or "").split(".", 1)[0] in local_roots
        )
        for m in report.missing
    )
    missing_paths: list[str] = []
    if local_missing:
        seen_paths: set[str] = set()
        for item in report.missing:
            for rel in _imports.expected_module_paths(item.module):
                if rel not in seen_paths and not (pathlib.Path(dest) / rel).exists():
                    seen_paths.add(rel)
                    missing_paths.append(rel)
    if symbol_only:
        failure_code = "TEST_SYMBOL_ERROR"
    elif local_missing:
        failure_code = "TEST_LOCAL_MODULE_MISSING"
    else:
        failure_code = "TEST_IMPORT_ERROR"
    return {
        "ok": False,
        "runner": None,
        "collected": 0,
        "output": "（测试导入预检失败：%s）" % "; ".join(report.errors[:8]),
        "failure_code": failure_code,
        "missing_paths": missing_paths or None,
        "bootstrap_missing_modules": bootstrap_missing_modules,
    }


def _list_files(sandbox: pathlib.Path) -> List[str]:
    """沙箱里可交付的产物相对路径（排除 _* / __pycache__ / *.pyc）。"""
    out = []
    for f in sorted(sandbox.rglob("*")):
        if not f.is_file():
            continue
        rel = f.relative_to(sandbox)
        # 排除内部产物与缓存：_deps / __pycache__ / .pytest_cache / 任何 _ 或 . 开头的目录 / *.pyc
        if (any(p.startswith("_") or p.startswith(".") or p == "__pycache__" for p in rel.parts)
                or rel.name.startswith("_") or rel.suffix == ".pyc"):
            continue
        out.append(str(rel))
    return out


def apply_files(sandbox: pathlib.Path, target: str, *, files: Optional[List[str]] = None) -> List[str]:
    """把 manifest 选出的产物复制到 target。"""
    sandbox, tgt = pathlib.Path(sandbox), pathlib.Path(target)
    applied = []
    if files is None:
        raise ValueError("apply_files requires explicit files from ArtifactManifest")
    rel_files = list(files)
    for rel in rel_files:
        dst = tgt / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(sandbox / rel, dst)
        applied.append(rel)
    return applied


def apply_to_git(sandbox: pathlib.Path, repo: str, branch: str,
                 message: Optional[str] = None, timeout: int = 90,
                 files: Optional[List[str]] = None) -> dict:
    """人类门：在 repo 里新建 branch（用 git worktree，不动用户当前 checkout），
    把沙箱产物拷进去、git add+commit（**不 push**）。返回 {branch, commit, applied}。"""
    import tempfile
    sandbox, repo = pathlib.Path(sandbox), pathlib.Path(repo).expanduser()
    if not (repo / ".git").exists():
        return {"error": f"{repo} 不是 git 仓库"}
    rel_files = list(files) if files is not None else _list_files(sandbox)
    if not rel_files:
        return {"error": "没有可提交的产物文件"}
    wt = pathlib.Path(tempfile.mkdtemp(prefix="loom-wt-"))

    def g(args, cwd):
        return subprocess.run(["git", "-C", str(cwd)] + args, capture_output=True, text=True, timeout=timeout)

    try:
        r = g(["worktree", "add", "-b", branch, str(wt), "HEAD"], repo)
        if r.returncode != 0:
            return {"error": f"创建分支/worktree 失败：{(r.stderr or r.stdout)[:200]}"}
        for rel in rel_files:
            dst = wt / rel
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(sandbox / rel, dst)
        g(["add", "-A"], wt)
        msg = message or f"loom: apply build ({len(rel_files)} files)"
        c = g(["-c", "user.email=loom@local", "-c", "user.name=Loom", "commit", "-m", msg], wt)
        if c.returncode != 0:
            return {"error": f"commit 失败：{(c.stderr or c.stdout)[:200]}"}
        sha = g(["rev-parse", "--short", "HEAD"], wt).stdout.strip()
        return {"branch": branch, "commit": sha, "applied": rel_files, "repo": str(repo)}
    except Exception as e:  # noqa: BLE001
        return {"error": f"git apply 异常：{type(e).__name__}: {e}"}
    finally:
        g(["worktree", "remove", "--force", str(wt)], repo)
