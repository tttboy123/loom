# P7-HT2 installed Build 260 verification

Status: `PARTIAL / LOCAL INSTALL ACCEPTED / REAL MODEL MATRIX PENDING`

Date: 2026-08-29

## Bundle identity

- App: `/Users/lune/Applications/Loom.app`
- Version: `0.5.6`
- Build: `260`
- Architecture: `arm64`
- Signing: ad-hoc, `codesign --verify --deep --strict` passed
- Rollback: `/Users/lune/Applications/Loom.app.previous`, Loom `0.5.5`
  Build `259`

Candidate and installed executable bytes are identical:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `56c9089d7a10e9bd10758e7edbaa3c852d105bb37c50c043ba2424790d6fc3e6` |
| `Contents/Library/Helpers/loomd` | `bbfc1d6ecc57b4d6f2a73358c38632d4c43863ee453467730ed623fcdcf83e1c` |
| `Contents/Info.plist` | `b30da09b9c16d2c0bd7779374b7de2d22b6c9987a893cd2199ac98b8c46944e5` |

## Build and install gates

```text
scripts/build-loom-local-app.sh --output .../.dist-build260/Loom.app
PASS

scripts/test-build-loom-local-app.sh
native app build fixture PASS

scripts/test-install-loom-local-app.sh
native app installer fixture PASS
```

The deterministic build fixture performed two clean release builds and matched
their Mach-O UUID, executable, plist, every bundle file and permission-aware
manifest. Its native launch smoke survived the required window without a new
macOS crash report.

The transactional installer fixture passed dry-run non-mutation, initial
install, replacement, injected failure rollback, signal rollback, explicit
rollback and symlink destination rejection before the real user-level install.

## Managed startup and restart

On first launch and again after a full App/daemon stop and restart:

- `LoomLocalApp` started from the Build 260 bundle.
- The bundled `loomd` started as its managed child from the same bundle.
- The daemon's canonical OS argv carried exact state, isolation root, Socket,
  Runtime directories, Codex/OpenCode/Claude executables and managed parent PID.
- The App required no separately started daemon.
- The Conversation composer was focused and the local service reported ready.

The second read-only private UDS snapshot returned in 1.007 seconds with:

- 24 Provider descriptors;
- 7 online Runtime instances;
- 7 executable Conversation Profiles;
- 5 bounded CC Switch import candidates;
- verified DeepSeek revision 2 and MiniMax revision 23 accounts;
- Codex, Claude Code, OpenCode, Pi, Loom Native DeepSeek, Kimi and MiniMax
  preserved as separate Runtime rows.

No Provider was presented as an `opencode-*` duplicate. OpenCode remained a
Runtime combined with the exact underlying Provider Account only in Route
profiles.

## UI evidence

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-installed-build260-restart.jpeg` | `20dde174000f14d0194d047479e28add14cd6d8879bf25c53b76f8c3be6b62cc` |
| `P7-HT2-installed-build260-runtime-provider.jpeg` | `6d4efdc2223d61aa00a22b0ed6e9222b2269b79ef438ecfb179e81e14d6ace5d` |

The first artifact shows the conversation-first shell after cold restart with
the composer, Route/Model controls and all seven Runtime rows. The second shows
the unlocked Loom Credential Vault, seven Agent Runtimes and Provider Account
states without exposing credential values.

## Remaining live gate

No chat message, Provider request, credential mutation or paid model operation
was performed during this installed verification. Build 260 is locally
installable and restart-safe, but Phase 7 remains `PARTIAL` until the user
explicitly authorizes a real-network matrix proving model-selected reads and
Proposals, visible confirmation/cancellation, governed execution, expiry,
replay rejection and restart restoration across Codex, OpenCode, Claude Code,
Pi and Loom Native.
