# P7-HT2 installed Build 261 verification

Status: `PARTIAL / LOCAL INSTALL ACCEPTED / PI ASSET AND REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Bundle identity

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.6`
- Build: `261`
- Architecture: `arm64`
- Signing: ad-hoc, `codesign --verify --deep --strict` passed
- Rollback: `/Users/lune/Applications/Loom.app.previous`, Loom `0.5.6`
  Build `260`

Candidate and installed executable bytes are identical:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `431483e93513f96b6a801c4bf0e22a2c61aa9b0cef7246253f06ca1981ba7914` |
| `Contents/Library/Helpers/loomd` | `2bfd8c58f958b99d31b518c7f7b68bf927ce752c59d17ceb3562d85e18d42d19` |
| `Contents/Info.plist` | `9c58d311f7221ef1292b18b6d2f0a0fff5abd6d74c80cf143bdb10bcb03958f9` |

## Build and install gates

```text
scripts/build-loom-local-app.sh --output .../.dist-build261/Loom.app
PASS

scripts/test-build-loom-local-app.sh
native app build fixture PASS

scripts/test-install-loom-local-app.sh
native app installer fixture PASS
```

The deterministic fixture completed two clean release builds in 223.00 and
215.19 seconds and matched Mach-O UUID, executable, plist, every bundle file
and permission-aware manifest. Strict signing and the native launch smoke gate
passed. The transactional installer preserved Build 260 as the rollback bundle.

## Managed startup and restart

Build 261 was installed, launched, stopped completely and launched again. On
both starts `LoomLocalApp` ran from the installed bundle and the bundled `loomd`
ran as its managed child. Candidate and installed digests remained equal.

The first and second read-only private UDS `setup_snapshot` calls completed in
1.117 seconds and 243 milliseconds. Both returned:

- 24 Provider descriptors;
- seven online Runtime instances;
- seven executable Conversation Profiles;
- five bounded CC Switch import candidates;
- verified DeepSeek revision 2 and MiniMax revision 23 accounts;
- separate Codex, Claude Code, OpenCode, Pi and Loom Native Runtime rows.

The machine does not contain
`~/Library/Application Support/Loom/phase1-live`. The canonical daemon argv
therefore contained no local-model flags and Setup published no Pi Conversation
Profile. This is the accepted fail-closed result: an online Pi Harness does not
become a selectable Route until the locked llama.cpp executable and Qwen model
pass path, identity, size and SHA-256 inspection. All other Routes remained
usable.

## Remaining gate

No Provider request, credential mutation, model download or paid model call was
performed during this local installed verification. Phase 7 remains `PARTIAL`
until explicit authorization permits the pinned Pi asset download and the real
installed matrix proves model-selected reads and Proposals, visible
confirmation/cancellation, governed execution, expiry, replay rejection and
restart restoration across Codex, OpenCode, Claude Code, Pi and Loom Native.
