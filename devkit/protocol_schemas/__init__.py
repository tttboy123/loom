"""JSON Schema descriptors for Loom protocol documents.

The actual ``.schema.json`` files live next to this ``__init__.py`` and are
loaded lazily so importing this package has no side effect beyond reading
the filesystem.

Each submodule exposes a module-level ``SCHEMA`` dict (the parsed JSON
schema) and a ``schema_path`` ``pathlib.Path`` for direct fs access. Schema
loading is cached at process scope.
"""
