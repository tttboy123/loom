# P2A-W2 Locked Pi Component Preflight RED

**Date**: 2026-07-30
**Status**: `RED — PRE-PROCESS MANIFEST GATE`
**Lineage**: `p2a-w2-pi-component-20260730-001`

The exact enabled component-test command stopped before Factory or process
construction with:

```text
locked Pi component gate closed: component_node_identity
```

The Go test command returned `PASS` only because the frozen contract requires
unsafe or mismatched manifest input to skip before process construction. It is
not component GREEN.

Read-only inspection showed:

```text
frozen Node path:
/Users/lune/Documents/Codex/devtools/node/bin/node

symlink component:
/Users/lune/Documents/Codex/devtools/node

canonical target directory:
/Users/lune/Documents/Codex/devtools/node-v24.16.0-darwin-arm64
```

The canonical Node executable remains a regular uid-501 `0755` file of
`120573328` bytes with SHA-256:

```text
1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8
```

No installed Pi process was constructed. Zero `--version` and zero
`--list-models` commands ran. No daemon, app, llama-server, Provider, Keychain,
network client, Journal write, socket, or retry was used. The isolation root
remained empty, so component execution A is not consumed.

Contract Repair 3 freezes the canonical Node executable and search-directory
paths without relaxing the rule that the installed Pi `.bin/pi` leaf is the
only permitted symlink.

VERDICT: RED
