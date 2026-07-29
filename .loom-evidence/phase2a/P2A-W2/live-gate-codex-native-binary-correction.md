# P2A-W2 Live Gate Codex Native Binary Correction

**Date**: 2026-07-29
**Status**: REVIEWED — fresh independent Contract Review PASS
**Parent**: reviewed P2A-W2 Live Gate and Deterministic Checkpoint Commit
Amendment

## 1. Pre-live diagnosis

The exact post-checkpoint preflight ran before creating an attempt root,
starting a daemon or app, executing Codex, touching Keychain, making a network
request, or writing a Journal Event.

The frozen live manifest named:

```text
/Users/lune/Documents/Codex/devtools/npm/bin/codex
```

That path is a symbolic link to the npm JavaScript wrapper. Production
`SystemCodexStatusRunner` deliberately:

1. uses `os.Lstat` and accepts only a regular executable file;
2. rejects symbolic links;
3. executes with an empty environment;
4. permits only the exact arguments `login status`.

The frozen symlink/JavaScript wrapper therefore cannot satisfy the production
identity boundary. An outer `PATH`, wrapper script, shell, environment
injection, or product-code relaxation would violate the reviewed contract.

The same reviewed official `@openai/codex` installation contains its platform
native arm64 executable. It is a regular executable file and requires no
JavaScript wrapper or ambient `PATH`.

## 2. Exact correction

This correction changes exactly one manifest field:

```text
codex_executable =
/Users/lune/Documents/Codex/devtools/npm/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex
```

Locked observations:

```text
sha256         = 29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a
file_type      = Mach-O 64-bit executable arm64
owner          = lune
mode           = 0755 regular file
bytes          = 260405808
codesign_id    = codex
team_id        = 2DC432GLL2
cdhash         = d03151872f950e955737aa42334e5bc513dcabc2
```

The executable and its parent directory must retain the same path, device,
inode, owner, type and mode from immediately before `NewCodexNativeAuthObserver`
construction through process completion. The existing production code performs
the before/launch/after identity checks.

The live daemon invocation's final argument is therefore corrected to:

```text
--codex-executable /Users/lune/Documents/Codex/devtools/npm/lib/node_modules/@openai/codex/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex
```

All other source locks, paths, commands, timeouts, one-request limit, credential
rules, TeamDefinition-only authority, cleanup, stop rules and no-retry behavior
remain byte-for-byte unchanged.

## 3. Preserved resident observer

Read-only process inspection found the accepted resident Runtime observer still
running from:

```text
/Users/lune/Library/Application Support/Loom/demo-resident/bin/loomd
```

Its arguments contain no `--socket`; it is not a product IPC daemon and does
not own the frozen default product socket, which remains absent. It uses its own
accepted SQLite and isolation root. The controlled W2 canary must leave this
process, LaunchAgent, SQLite, Runtime state and arguments untouched.

The W2 daemon uses the fresh attempt state/isolation root and the one private
default product socket. Any collision, resident-process mutation, or second
product socket stops before canary consumption.

## 4. Authority and consumption

- This is a correction inside the existing W2 live gate, not W3 or W4.
- It adds no product source change and relaxes no executable identity check.
- No canary action has occurred.
- The attempt root and default product socket remain absent.
- No Codex process, Keychain operation, Provider request, daemon, app, Journal
  write, install, staging, commit, push or merge occurred.
- The live allowance remains unconsumed.

Fresh independent Contract Review `PASS` is required before this corrected
native executable may be invoked.
