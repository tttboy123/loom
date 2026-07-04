import re
_NAME_PATTERN = re.compile(r"^[A-Za-z0-9_\-]{1,32}$")

def verify_placeholder(name: str) -> bool:
    if not isinstance(name, str) or not name:
        raise ValueError("name must be non-empty string")
    if not _NAME_PATTERN.fullmatch(name):
        raise ValueError(f"unsafe name: {name!r}")
    return True
