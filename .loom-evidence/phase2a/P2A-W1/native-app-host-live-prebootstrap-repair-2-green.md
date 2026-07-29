# P2A-W1 Native App Host Live Pre-Bootstrap Repair 2 GREEN

Date: 2026-07-28  
Status: `GREEN — PENDING FRESH INDEPENDENT IMPLEMENTATION REVIEW`  
Live allowance: `1`, unconsumed

## Repair

The native bundle builder now invokes SwiftPM release linking with:

```text
-Xlinker -no_uuid
-Xlinker -no_adhoc_codesign
```

It copies the unsigned executable into the private bundle, removes non-runtime
debug/N_OSO symbols with Apple `/usr/bin/strip -S`, then performs the existing
single final ad-hoc bundle signature.

The builder test resets the Swift package before each of two complete builds
and compares one canonical full relative-path/mode/file-digest manifest. This
proves clean build reproducibility instead of reusing a cached linked
executable.

The canonical manifest bytes are exactly:

```text
for each regular bundle file sorted by raw relative path:
relative_path NUL stat_percent_Sp_mode NUL lowercase_sha256 LF
```

The complete manifest digest is lowercase SHA-256 over the concatenated bytes.
The exact implementation used by the fixture and evidence is:

```sh
find "$bundle" -type f -print0 |
  sort -z |
  while IFS= read -r -d '' file
  do
    relative_path=${file#"$bundle/"}
    mode=$(stat -f '%Sp' "$file")
    digest=$(shasum -a 256 "$file" | awk '{print $1}')
    printf '%s\000%s\000%s\n' "$relative_path" "$mode" "$digest"
  done |
  shasum -a 256 |
  awk '{print $1}'
```

No Swift source, app behavior, IPC, daemon, Journal, Projection, StateWriter,
Runtime, Provider, authorization, or scheduler code changed.

## Deterministic proof

- pre-repair two-clean-build fixture: `RED`;
- UUID-free and unsigned but unstripped comparison: six N_OSO timestamp bytes
  still differed;
- `strip -S` diagnostic comparison: byte-for-byte equal;
- repaired two-clean-build signed bundle comparison: `PASS`;
- both canonical NUL-delimited manifest digests: identical;
- Mach-O `LC_UUID`: absent;
- final code signature: strict `PASS`;
- arm64, bundle ID, owner/mode, no-symlink, static-exclusion and
  secret-negative gates: `PASS`;
- Swift tests: `17 passed`;
- Swift release build: `PASS`;
- native app installer fixture: `PASS`;
- local-product installer including legacy and signal repair: `PASS`;
- `go test -count=1 ./...`: `PASS`;
- `go test -race -count=1 ./...`: `PASS`;
- `go vet ./...`: `PASS`;
- `go mod verify`: `all modules verified`;
- shell syntax and Git diff checks: `PASS`;
- staging: empty.

## Reproducible final Candidate identity

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

LoomLocalApp
6e9f03c064a1af09e27446a194bcbf5f5e14c3cd7e69d4a959af73229f2ebea8

Info.plist
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

complete bundle manifest
221e9878db56b451dd5b724ec0b2e3ceacf349269da9fea3cf571690bc33f67a
```

Both exact live destination dry-runs pass without mutation.

## Live state

No install, bootout, bootstrap, app launch, Computer Use, Journal write,
Runtime execution, Team creation, Provider action, or authority transition
occurred. The original resident service and signed hashes remain unchanged.
Candidate bootstrap count remains `0`; the native allowance remains `1`,
unconsumed.

Fresh independent Implementation Review is required. No prior activation may
authorize the repaired builder output.
