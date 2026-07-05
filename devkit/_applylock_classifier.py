import re, pathlib
from dataclasses import dataclass

@dataclass
class ApplylockConfig:
    test_prefix_allowlist: tuple = ()
    exact_path_allow: tuple = ()
    glob_blocklist: tuple = ()
    prefix_blocklist: tuple = ()
    hardcoded_control_paths: tuple = ()

def classify(path: str, cfg: ApplylockConfig) -> str:
    p = pathlib.PurePosixPath(path)
    name = p.name
    if cfg.test_prefix_allowlist and name.startswith(cfg.test_prefix_allowlist):
        return "test_prefix"
    if path in (cfg.exact_path_allow or ()):
        return "exact_allow"
    for pat in (cfg.glob_blocklist or ()):
        if pat and re.fullmatch(pat, name):
            return "glob_block"
    for pre in (cfg.prefix_blocklist or ()):
        if pre and path.startswith(pre):
            return "prefix_block"
    if path in (cfg.hardcoded_control_paths or ()):
        return "hardcoded_control"
    return "unknown"
