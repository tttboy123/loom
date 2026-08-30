# P7-HT2 installed Build 262 verification

Status: `PARTIAL / LOCAL INSTALL ACCEPTED / PI ASSET AND REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Bundle identity

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.6`
- Build: `262`
- Architecture: `arm64`
- Signing: ad-hoc, `codesign --verify --deep --strict` passed
- Rollback: `/Users/lune/Applications/Loom.app.previous`, Loom `0.5.6`
  Build `261`

Candidate and installed executable bytes are identical:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `0556ae27a469f710992458c4272d60ac6c7f532117294a9d4da3121491d869b7` |
| `Contents/Library/Helpers/loomd` | `0570adefead5bd498fb87278985b34c8d71894e804d2313bc57bcc84b97c1fb3` |
| `Contents/Info.plist` | `2c99788fce70d4aaf521a013a5fe19b4e8ca73b8fd540974bc81739e84074e8a` |

## Build and install gates

```text
go test ./... -count=1
PASS

go vet ./...
PASS

scripts/build-loom-local-app.sh --output .../.dist-build262/Loom.app
PASS

scripts/test-build-loom-local-app.sh
native app build fixture PASS

scripts/test-install-loom-local-app.sh
native app installer fixture PASS
```

The deterministic fixture completed two clean release builds in 304.57 and
295.89 seconds and matched Mach-O UUID, executable, plist, every bundle file
and permission-aware manifest. Strict signing and the native launch smoke gate
passed. The transactional installer preserved Build 261 as the rollback bundle.

## Pi integrity boundary

Build 262 pins both official local backend artifacts before discovery or start:

- llama.cpp b10107 `llama-server` SHA-256:
  `a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`
- Qwen 2.5 Coder 1.5B Q4_K_M GGUF SHA-256:
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`

Path ownership, modes, regular-file identity and both digests must match. Exact
file identities are checked again immediately before process start. A changed
binary or model fails closed rather than publishing an executable Pi Route.

## Managed startup and restart

Build 262 was installed, launched, stopped completely and launched again. On
both starts `LoomLocalApp` ran from the installed bundle and the bundled `loomd`
ran as its managed child. Candidate and installed digests remained equal.

The first and second read-only private UDS `setup_snapshot` calls completed in
773 and 543 milliseconds. Both returned:

- 24 Provider descriptors;
- seven online Runtime instances;
- seven executable Conversation Profiles;
- five bounded CC Switch import candidates;
- verified DeepSeek revision 2 and MiniMax revision 23 accounts;
- separate Codex, Claude Code, OpenCode, Pi and Loom Native Runtime rows.

The machine still has no `~/Library/Application Support/Loom/phase1-live`
assets. The canonical daemon argv therefore carries no local-model flags and
Setup publishes no Pi Conversation Profile. All other Routes remain available.

## Remaining gate

No Provider request, credential mutation, model download or paid model call was
performed during this local installed verification. Phase 7 remains `PARTIAL`
until explicit authorization permits the pinned Pi asset download and the real
installed matrix proves model-selected reads and Proposals, visible
confirmation/cancellation, governed execution, expiry, replay rejection and
restart restoration across Codex, OpenCode, Claude Code, Pi and Loom Native.
