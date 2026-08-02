# P2A-W3 Codex Native-Auth Live Canary Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-codex-001`  
**Manifest SHA-256**:
`38ec546bb46e6153f7e4e4b3c74f71747faf8538f31563cbe33666705a2a6ba1`  
**Allowance**: consumed once; no retry  
**Verdict**: `FAIL — CODEX AVAILABLE / PRODUCT PREFLIGHT NOT REACHED`

## Preflight

- private attempt root and directories were user-owned `0700`;
- empty initial SQLite and manifest were regular user-owned `0600`;
- source lock `03d213...3ce` and Implementation Review 4 `PASS` matched;
- attempt `loomd`, `loom`, native bundle, Codex, Pi, Node, llama-server and
  exact GGUF SHA-256 identities matched the frozen manifest;
- native bundle was arm64, strict ad-hoc signature valid, contained no symlink
  and had bundle ID `com.earendilworks.loom.local`;
- default product socket and lock were absent; and
- the daemon was started with a credential-clean controller environment.

## Observed product result

The exact native app connected to the one attempt daemon. Through the ordinary
`Runtime & Providers` product sheet, Computer Use observed:

```text
Codex   Available
MiniMax Unconfigured
```

This validates the bounded Codex claim: the fixed regular Codex executable's
status-only path recognized existing native authentication. Loom did not invoke
login/logout, read credential storage or expose OAuth material.

The New Mission sheet was then opened. It truthfully reported no confirmed Team
available. The visible `Teams` rail action is currently a no-op in
`MissionWorkbench`; it could not create the first saved Team. The Controller
prepared to use the product TUI Team Builder, which shares the same typed daemon
API and requires no internal identifiers.

Before that Builder could connect, the one daemon exited with the exact closed
reason:

```text
daemon failed: observer_models_timeout
```

The Pi version/model metadata cycle exceeded the frozen `10s` process budget.
No second daemon start, alternate timeout, direct IPC/SQLite mutation, hidden
retry, Codex execution, MiniMax request or Pi execution occurred.

## Authoritative postflight

The final database is private, integrity-valid and SHA-256:

```text
09139a4b3241841be850438e3829627f1d99c5b08352b1c624acacad53666eec
```

Its complete facts are only the two accepted initialization facts:

```text
AgentGrantIdentityIndexInitialized | 1
WorkRunIdentityIndexInitialized    | 1
```

There is no Runtime discovery, TeamDefinition, TeamInstance, AgentInstance,
WorkItem, Run, Grant, Evidence, dispatch or execution fact. The product socket
and lock are absent, isolation is empty, and the attempt daemon, TUI, native app,
Pi and llama-server processes are absent. A non-database attempt-tree scan found
no raw credential, token or API-key material.

## Claim limit and continuation

The canary proves `Codex · Available` in the real product window but does not
prove the required exact product preflight. This lineage is consumed and will
not be retried. The independent MiniMax and Pi manifests remain unconsumed; a
governed decision is required before consuming them because the same `10s`
metadata budget would predictably reproduce this stop.

**VERDICT**: `FAIL — CODEX AVAILABLE / PRODUCT PREFLIGHT NOT REACHED`
