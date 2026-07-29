# P2A-W1 Reopen 4 Repair A RED Evidence

**Date**: 2026-07-28
**Status**: `RED CAPTURED`
**Live action**: none

Command:

```text
go test ./internal/localipc \
  -run '^TestFileIdentityIncludesFileKind$' -count=1
```

Result: exit `1`.

Exact relevant failure:

```text
equal device/inode socket and regular identities collided:
localipc.fileIdentity{device:0x7, inode:0xb}
```

The deterministic fake `FileInfo` instances have the same device and inode but
different `Lstat` kinds. The current implementation treats them as identical,
reproducing the product condition behind the real replacement-cleanup flake.

No timeout, retry, live service, socket, SQLite, credential, Provider, Runtime,
staging, or installed state was changed by RED.
