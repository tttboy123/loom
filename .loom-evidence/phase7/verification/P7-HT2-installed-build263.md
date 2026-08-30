# P7-HT2 installed Build 263 verification

Status: `PARTIAL / LOCAL INSTALL ACCEPTED / PI ASSET AND REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Bundle identity

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.6`
- Build: `263`
- Architecture: `arm64`
- Signing: ad-hoc, `codesign --verify --deep --strict` passed
- Rollback: `/Users/lune/Applications/Loom.app.previous`, Loom `0.5.6`
  Build `262`

Candidate and installed executable bytes are identical:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `1df7c5a12d9208a966ba50656ed8a1289e5b0c4db676b7a70f12699c83783aeb` |
| `Contents/Library/Helpers/loomd` | `0570adefead5bd498fb87278985b34c8d71894e804d2313bc57bcc84b97c1fb3` |
| `Contents/Info.plist` | `c4c6f5222e3c105abdd437694b73f97cc7f4d99e3f078fba6be568675bd68cf0` |

## Build and install gates

```text
swift test --quiet
457 XCTest cases, 2 conditional skips, 0 failures
20 strict Swift Testing contract cases, 0 failures

scripts/build-loom-local-app.sh --output .../.dist-build263/Loom.app
PASS (93.59s)

scripts/test-build-loom-local-app.sh
native app build fixture PASS (clean builds: 154.23s and 247.91s)

scripts/test-install-loom-local-app.sh
native app installer fixture PASS
```

The deterministic fixture matched Mach-O UUID, executable, plist, every bundle
file and permission-aware manifest. Strict signing and the native launch smoke
gate passed. The transactional installer preserved Build 262 as the rollback
bundle.

The latest source-only UI change does not alter Go behavior. The complete Go
repository suite, `go vet` and focused race gates recorded in
`P7-HT2-source.md` remain the applicable Build 263 daemon boundary.

## Managed startup and restart

Build 263 was installed, launched, stopped completely and launched again. On
both starts `LoomLocalApp` ran from the installed bundle and the bundled `loomd`
ran as its managed child with canonical state, isolation and socket arguments.

The first and second read-only private UDS `setup_snapshot` calls completed in
570 and 234 milliseconds. Both returned:

- 24 Provider descriptors;
- seven online Runtime instances;
- seven executable Conversation Profiles;
- five bounded CC Switch import candidates;
- verified DeepSeek revision 2 and MiniMax revision 23 accounts;
- separate Codex, Claude Code, OpenCode, Pi and Loom Native Runtime rows.

No Provider request or credential mutation occurred. The Vault and existing
Provider metadata retained their exact revisions across both cold starts.

The source-ready five-Runtime control-tool runner remained gated off. It cannot
start unless the operator separately authorizes paid requests and managed App
restart and supplies this exact build number; a skipped run is not acceptance.

## Installed visual acceptance

The real installed Runtime inspector keeps all seven Runtime rows visible and
shows Pi 0.82.1 as:

```text
Online · Local model required for Conversation
```

This distinguishes Harness discovery from executable Conversation capacity.
The composer still exposes only the seven executable Profiles; no false Pi
Route appears. The Conversation remains centered, service readiness is visible,
and no controls or labels overlap.

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-installed-build263-runtime-inspector.png` | `a693ab32c25041c31add302b93466c1d4dcac5e8d0625f599812dafa3d13d140` |

## Remaining gate

The machine still has no integrity-checked Pi local-model assets. No real
Provider request, paid model call or credential operation was used. Phase 7
remains `PARTIAL` until explicit authorization permits the pinned asset install
and the installed five-Runtime matrix proves model tool selection, Proposal
review, user confirmation/cancellation, governed execution, expiry, replay
rejection and restart restoration.
