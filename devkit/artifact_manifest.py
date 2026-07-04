"""Build a single ``ArtifactManifest`` dict aligned with the runtime schema.

The schema lives at ``devkit/protocol_schemas/artifact_manifest.schema.json``
and is enforced at runtime by the rdloop evidence stage. This module is the
writer-side counterpart: it produces the dict that the verifier stage then
passes through jsonschema validation.

Schema requirements (relevant subset)
-------------------------------------
* ``api_version`` const ``loom.dev/v1`` (filled here).
* ``kind`` const ``ArtifactManifest``.
* ``metadata.id`` required.
* ``spec.entries`` required (list of artifact paths).
* ``spec.workspace_path`` / ``candidate_path`` / ``spec.lineage`` optional.

Why a separate module
---------------------
The Phase 1 schema was decorative — the rdloop evidence block wrote the
manifest on its own, format drift wasn't bounded. Phase B promoted this
helper to a first-class module so the producer path has a single writer and
verifier path (a single reader). Both load the same JSON schema.

Public API
----------
``build_manifest(*, manifest_id, entries, run_id=None, workspace_path=None,
candidate_path=None, lineage=None, source="loom_runtime")``

Returns a dict conforming to ``artifact_manifest.schema.json``. Raises
``ValueError`` on invalid input (caller's responsibility to ensure the
manifest id is non-empty and entries is a list).
"""
from __future__ import annotations

import json
import pathlib
from typing import Any

from jsonschema import Draft202012Validator

from .protocol_schemas import artifact_manifest_schema

PROTOCOL_VERSION = "loom.dev/v1"
MANIFEST_KIND = "ArtifactManifest"
ALLOWED_SOURCES = {
    "loom_runtime",
    "inner_sandbox",
    "materialized_repo",
    "external_handoff",
    "user_supplied",
}


class ManifestBuildError(ValueError):
    """Raised when build_manifest input is structurally invalid."""


def _validate_entries(entries: Any) -> list[dict]:
    if entries is None:
        raise ManifestBuildError("entries is required (spec.entries)")
    if not isinstance(entries, list):
        raise ManifestBuildError(f"entries must be a list, got {type(entries).__name__}")
    out: list[dict] = []
    for idx, raw in enumerate(entries):
        if isinstance(raw, str):
            out.append({"path": raw})
            continue
        if isinstance(raw, dict) and raw.get("path"):
            out.append(dict(raw))
            continue
        raise ManifestBuildError(
            f"entries[{idx}] must be a string path or dict with 'path', got {type(raw).__name__}"
        )
    return out


def _validate_lineage(lineage: Any) -> dict | None:
    if lineage is None:
        return None
    if not isinstance(lineage, dict):
        raise ManifestBuildError(f"lineage must be a dict, got {type(lineage).__name__}")
    return dict(lineage)


def build_manifest(
    *,
    manifest_id: str,
    entries: list[dict] | list[str],
    run_id: str | None = None,
    workspace_path: str | None = None,
    candidate_path: str | None = None,
    lineage: dict | None = None,
    source: str = "loom_runtime",
    api_version: str = PROTOCOL_VERSION,
) -> dict:
    """Return a dict conforming to ``artifact_manifest.schema.json``.

    Required
    --------
    * ``manifest_id``: written into ``metadata.id``; non-empty.
    * ``entries``: iterable of str paths or ``{"path": str, ...}`` dicts.

    Optional
    --------
    * ``run_id``: stamped on ``metadata.run_id`` when present.
    * ``workspace_path`` / ``candidate_path``: written under ``spec``.
    * ``lineage``: provenance dict; written under ``spec.lineage``.
    * ``source``: one of ``ALLOWED_SOURCES`` (default ``loom_runtime``).
    * ``api_version``: must equal ``PROTOCOL_VERSION`` for the schema const.
    """
    mid = str(manifest_id or "").strip()
    if not mid:
        raise ManifestBuildError("manifest_id is required (metadata.id)")
    if api_version != PROTOCOL_VERSION:
        raise ManifestBuildError(
            f"api_version must be {PROTOCOL_VERSION!r}, got {api_version!r}"
        )
    if source not in ALLOWED_SOURCES:
        raise ManifestBuildError(
            f"source must be one of {sorted(ALLOWED_SOURCES)}, got {source!r}"
        )

    metadata: dict[str, Any] = {"id": mid}
    if run_id:
        metadata["run_id"] = run_id

    spec: dict[str, Any] = {"entries": _validate_entries(entries)}
    if workspace_path:
        spec["workspace_path"] = workspace_path
    if candidate_path:
        spec["candidate_path"] = candidate_path
    normalized_lineage = _validate_lineage(lineage)
    if normalized_lineage is not None:
        spec["lineage"] = normalized_lineage

    return {
        "api_version": api_version,
        "kind": MANIFEST_KIND,
        "metadata": metadata,
        "spec": spec,
        "source": source,
    }


def validate_manifest(manifest: dict) -> None:
    """Raise ``jsonschema.ValidationError`` if the manifest violates the schema."""
    Draft202012Validator(artifact_manifest_schema.SCHEMA).validate(manifest)


def write_manifest(manifest: dict, path: pathlib.Path | str) -> pathlib.Path:
    """Validate then write the manifest as JSON. Returns the abs path."""
    validate_manifest(manifest)
    out = pathlib.Path(path)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return out
