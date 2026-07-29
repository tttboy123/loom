# P2A-W2 Locked Pi Component Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-pi-component-20260730-001`
**Execution**: `A`
**Status**: `GREEN`

## Contract gates

- Contract Repair 3 fresh independent Re-review: `PASS`
- pre-process skip-closed test without enable inputs: `PASS/SKIP`
- strict manifest duplicate/unknown/trailing/malformed tests: `PASS`
- pre-component implementation Review Repair 1: `PASS`
- component root: uid `501`, mode `0700`
- isolation root: uid `501`, mode `0700`, empty
- manifest: uid `501`, mode `0600`
- manifest SHA-256:
  `157f277e1d046bccb6e03ae9bf21339c2e8a5bc321deea6836b6ecc9619a6c28`

## Exact locked identities

```text
Pi cli.js
size   681
mode   0700
uid    501
sha256 af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

Node
size   120573328
mode   0755
uid    501
sha256 1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8

llama-server
size   33472
mode   0700
uid    501
sha256 a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b

GGUF
size   1117320768
mode   0600
uid    501
sha256 cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
```

The installed Pi `.bin/pi` leaf resolved only to the frozen `cli.js`. The
canonical Node executable and search directory contained no symlink component.

## Execution

Exact command:

```text
LOOM_P2A_W2_LOCKED_PI_COMPONENT='p2a-w2-pi-component-20260730-001' \
LOOM_P2A_W2_LOCKED_PI_MANIFEST='/Users/lune/Library/Application Support/Loom/p2a-w2-pi-component-20260730-001/manifest.json' \
go test ./internal/runtime/piadapter \
  -run '^TestLockedPiComponent$' \
  -count=1 -v
```

Result:

```text
=== RUN   TestLockedPiComponent
--- PASS: TestLockedPiComponent (12.43s)
PASS
ok   loom-pi-rebuild/internal/runtime/piadapter 12.939s
```

The test used the production
`NewPiLocalRuntimeProbeFactory → BuildProbe → DiscoverRuntime → Pi parser`
flow. That flow issued exactly one frozen `--version` request followed by
exactly one frozen offline `--list-models` request. It accepted version
`0.82.1` and exactly:

```text
loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
```

No raw Pi stdout or stderr was published by the test.

## Postconditions

- isolation root empty;
- component root contains only `isolation/` and `manifest.json`;
- no component Journal, WAL, SHM, rollback journal, socket or PID marker;
- no listener on `127.0.0.1:18427`;
- no component-root, attempt-004 or locked llama-server process;
- Pi/Node/llama/GGUF identity, size, mode, owner and digest unchanged;
- no daemon, native app, llama-server, Provider, Keychain, Journal write,
  inference or product canary started;
- execution B is forbidden because execution A is GREEN.

The previously recorded resident PID `44887` was absent during the post-run
read-only `ps` check. This component lineage did not signal, restart or mutate
that PID or any LaunchAgent.

VERDICT: PASS
