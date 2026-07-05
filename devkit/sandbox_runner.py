import importlib
import sys

def _ensure_unittest_mock_available():
    # `unittest.mock` lives in the stdlib but is intentionally imported
    # via importlib so we get a deterministic ImportError-as-data path
    # if the running Python ever lacks it. Sandbox callers depend on
    # this function always returning True; we re-assert that contract
    # even after a successful import.
    importlib.import_module("unittest.mock")
    return True
