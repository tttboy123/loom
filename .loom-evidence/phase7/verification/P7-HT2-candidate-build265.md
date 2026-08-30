# P7-HT2 candidate Build 265 verification

Status: `SOURCE AND CANDIDATE ACCEPTED / NOT INSTALLED / REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Candidate identity

- Candidate: `.dist-build265/Loom.app`
- Version: `0.5.6`
- Build: `265`
- Architecture: `arm64`
- Signing: ad-hoc; `codesign --verify --deep --strict` passed
- Installed App during verification: `/Users/lune/Applications/Loom.app`
  Build `264`

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `2e86a99d068c969ab0c38dab3da378836881d62715b1a2dcfd90203ce47f64d0` |
| `Contents/Library/Helpers/loomd` | `d267ff498c3a7083e3ccf390314358457dad8f0e720f151edd30fe301eae537c` |
| `Contents/Info.plist` | `54160f6650542231785b26014f3d1c1ac560544effad621ce37426bad9f04a46` |

## Pi Runtime authority repair

The previous production gate pinned the 33KB `llama-server` launcher and Qwen
GGUF but did not freeze the sibling dynamic libraries loaded by llama.cpp.
Build 265 makes the exact official b10107 macOS arm64 archive the trust root:

```text
archive SHA-256
b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32
```

Loom parses that bounded archive into a deterministic complete Runtime tree,
then validates every materialized file and directory with no extras. Traversal,
absolute or escaping links, duplicate entries, unsupported objects, link cycles,
decompression bounds, dependency drift, symlinks, hard links, wrong ownership
or modes and identity replacement fail closed. The App and daemon accept local
model configuration only as a complete private-root/archive/server/model tuple.

## Verification

```text
go test ./... -count=1
PASS

go vet ./...
PASS

focused piadapter and cmd/loomd race gates
PASS

swift test --package-path apps/macos
461 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS

scripts/test-install-loom-local-app.sh
PASS

scripts/install-loom-local-app.sh --dry-run \
  --app .../.dist-build265/Loom.app \
  --destination /Users/lune/Applications/Loom.app
PASS
```

No Pi asset, Provider credential or installed App state was changed. No
Provider request, paid model call, App replacement or restart occurred. Build
265 therefore remains candidate evidence and does not supersede installed
Build 264 acceptance.

## Remaining gate

The official archive and model still need an explicitly authorized private
materialization, followed by Build 265 installation/restart and the five-Runtime
real-model control-tool matrix. Until those gates pass, Phase 7 remains
`PARTIAL`.
