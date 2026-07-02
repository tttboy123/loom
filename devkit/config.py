# devkit/config.py — project-level defaults from loom.toml (searched upward like .gitignore)
from __future__ import annotations

import pathlib
import sys

_FILENAME = "loom.toml"

_SCALAR_KEYS = frozenset([
    "stages", "cascade", "max_tokens", "recipe", "iterate",
    "budget", "compact_model", "base_url",
])
_BOOL_KEYS = frozenset(["safety", "auto_carrier", "ponytail", "no_compact", "no_cache"])


def find_config(start: pathlib.Path | None = None) -> pathlib.Path | None:
    cwd = start or pathlib.Path.cwd()
    for d in [cwd, *cwd.parents]:
        p = d / _FILENAME
        if p.is_file():
            return p
    return None


def load_config(path: pathlib.Path | None = None) -> dict:
    """Load loom.toml, return dict of recognized run defaults."""
    config_path = path or find_config()
    if config_path is None:
        return {}
    try:
        if sys.version_info >= (3, 11):
            import tomllib
            with open(config_path, "rb") as f:
                data = tomllib.load(f)
        else:
            try:
                import tomli as tomllib  # pip install tomli on <3.11
                with open(config_path, "rb") as f:
                    data = tomllib.load(f)
            except ImportError:
                # Fallback: minimal key=value TOML parser (covers 90% of cases)
                data = _minimal_toml(config_path.read_text(encoding="utf-8"))
    except Exception:
        return {}

    result: dict = {}
    defaults = data.get("defaults", {})
    for k, v in defaults.items():
        if k in _SCALAR_KEYS:
            result[k] = v
        elif k in _BOOL_KEYS:
            result[k] = bool(v)

    carrier_section = data.get("carrier", {})
    if carrier_section:
        result["_carrier"] = [f"{stage}={model}" for stage, model in carrier_section.items()]

    return result


def write_default_config(path: pathlib.Path) -> None:
    path.write_text(
        '# Loom project config — devkit reads these as defaults for every run\n'
        '# CLI flags always override.\n\n'
        '[defaults]\n'
        '# stages = "plan,implement,review"   # 要跑的阶段\n'
        '# cascade = "deepseek,glm"           # 级联升级载体\n'
        '# max_tokens = 8000\n'
        '# recipe = "cheap-dev"\n'
        '# safety = true\n'
        '# auto_carrier = true\n'
        '# iterate = 1\n'
        '# budget = 0.05\n\n'
        '# [carrier]\n'
        '# implement = "deepseek"\n'
        '# review = "claude"\n',
        encoding="utf-8",
    )


def _minimal_toml(text: str) -> dict:
    """Very minimal TOML parser: handles [section] + key = value lines."""
    result: dict = {}
    section: dict = result
    section_name: str | None = None
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("[") and line.endswith("]"):
            section_name = line[1:-1].strip()
            section = {}
            result[section_name] = section
            continue
        if "=" in line:
            k, _, v = line.partition("=")
            k = k.strip()
            v = v.strip().lstrip('"').rstrip('"').strip("'")
            if v.lower() == "true":
                v = True
            elif v.lower() == "false":
                v = False
            else:
                try:
                    v = int(v)
                except ValueError:
                    try:
                        v = float(v)
                    except ValueError:
                        pass
            section[k] = v
    return result
