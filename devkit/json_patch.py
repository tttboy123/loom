"""JSON Patch operations (RFC 6902 subset) for flat dicts.

Supports add / remove / replace operations on single-level keys only
(e.g. "/key"). Pure standard library, no external dependencies.
"""

from __future__ import annotations

from typing import Any

class JSONPatchError(ValueError):
    """Raised when a patch operation is malformed or cannot be applied."""

def _strip_key(path: str) -> str:
    """Validate a single-level RFC 6901 path and return the bare key.

    Only top-level keys are supported: the path must start with '/'
    followed by one non-empty segment. JSON Pointer escaping (RFC 6901
    ~1 / ~0) is intentionally not supported to keep the contract small.
    """
    if not isinstance(path, str):
        raise JSONPatchError(f"path must be a string, got {type(path).__name__}")
    if not path.startswith("/"):
        raise JSONPatchError(f"path must start with '/': {path!r}")
    key = path[1:]
    if key == "":
        raise JSONPatchError(f"path must reference a key: {path!r}")
    if "/" in key:
        raise JSONPatchError(f"only single-level paths supported, got {path!r}")
    return key

def apply_patch(doc: dict, patch: list[dict]) -> dict:
    """Apply a list of patch operations to a flat dict, returning a new dict.

    The input ``doc`` is never mutated. Unknown / malformed operations
    raise ``JSONPatchError``.
    """
    if not isinstance(doc, dict):
        raise JSONPatchError(f"doc must be a dict, got {type(doc).__name__}")
    if not isinstance(patch, list):
        raise JSONPatchError(f"patch must be a list, got {type(patch).__name__}")

    result = dict(doc)
    for i, op_obj in enumerate(patch):
        if not isinstance(op_obj, dict):
            raise JSONPatchError(f"patch[{i}] must be a dict, got {type(op_obj).__name__}")
        op = op_obj.get("op")
        if op not in ("add", "remove", "replace"):
            raise JSONPatchError(f"patch[{i}].op must be add/remove/replace, got {op!r}")
        key = _strip_key(op_obj.get("path", ""))

        if op in ("add", "replace"):
            if "value" not in op_obj:
                raise JSONPatchError(f"patch[{i}] ({op}) missing 'value'")
            result[key] = op_obj["value"]
        else:  # remove
            if key not in result:
                raise JSONPatchError(f"patch[{i}] remove: key {key!r} not in doc")
            del result[key]
    return result

def make_patch(old: dict, new: dict) -> list[dict]:
    """Compute a patch describing the diff between two flat dicts.

    Adds keys present only in ``new``, removes keys present only in
    ``old``, and replaces keys whose value changed.
    """
    if not isinstance(old, dict):
        raise JSONPatchError(f"old must be a dict, got {type(old).__name__}")
    if not isinstance(new, dict):
        raise JSONPatchError(f"new must be a dict, got {type(new).__name__}")

    patch: list[dict] = []
    for key in sorted(old.keys()):
        if key not in new:
            patch.append({"op": "remove", "path": f"/{key}"})
        elif old[key] != new[key]:
            patch.append({"op": "replace", "path": f"/{key}", "value": new[key]})
    for key in sorted(new.keys() - old.keys()):
        patch.append({"op": "add", "path": f"/{key}", "value": new[key]})
    return patch

def patch_summary(patch: list[dict]) -> dict:
    """Return counts of ops in a patch list.

    Unknown ops are counted in ``total`` but not in any bucket, so the
    invariant is ``total == adds + removes + replaces + other``.
    """
    if not isinstance(patch, list):
        raise JSONPatchError(f"patch must be a list, got {type(patch).__name__}")

    adds = removes = replaces = other = 0
    for i, op_obj in enumerate(patch):
        if not isinstance(op_obj, dict):
            raise JSONPatchError(f"patch[{i}] must be a dict, got {type(op_obj).__name__}")
        op = op_obj.get("op")
        if op == "add":
            adds += 1
        elif op == "remove":
            removes += 1
        elif op == "replace":
            replaces += 1
        else:
            other += 1
    return {
        "total": len(patch),
        "adds": adds,
        "removes": removes,
        "replaces": replaces,
    }
