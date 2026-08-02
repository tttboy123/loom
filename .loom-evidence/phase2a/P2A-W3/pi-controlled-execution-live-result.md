# P2A-W3 Pi Controlled Execution Live Canary Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-pi-001`  
**Initial manifest SHA-256**: `34506c7e...8bd`  
**Active manifest SHA-256**: `5657c07a0ca1d71862ce676a80c06e370bfa2d6c706cd41c5c7619cc99752bc1`  
**Allowance**: consumed once; no retry  
**Verdict**: `FAIL — PRODUCT PREFLIGHT CONFLICT / NO START`

## Preflight and daemon

The unconsumed manifest was completed with the reviewed explicit `30s` Pi
metadata process budget; its prior bytes remain under the attempt manifest
directory. Source lock, Implementation Review, private modes, no-symlink tree,
attempt binaries, native bundle and external Pi/Node/llama/GGUF identities all
matched.

The private initial SQLite was empty, regular and user-owned `0600`. The single
daemon start succeeded and appended exactly one current online Pi Runtime
discovery. It completed twelve observation cycles with one discovery write,
zero status writes and eleven no-write cycles. No Provider credential or
network Provider request was used.

## Saved-Team product path

Through the ordinary TUI Team Builder, the Controller selected one Main and one
dormant SubAgent, reviewed the local Runtime/model binding and explicitly
confirmed `P2A W3 Controlled Team`. The product truthfully reported that the
saved Team was ready and that no work had started.

The authoritative product snapshot then showed exactly one confirmed,
executable, non-read-only saved Team:

```text
TeamInstance  team-instance-8f2f4f51416eac8a915927fde5da6420
Runtime       pi-0.82.1-p2a-w3-pi
Status        online
Capacity      1
Main          1 active materialized AgentInstance
SubAgent      1 dormant binding, 0 active materialized SubAgents
```

This is live proof that the reviewed Dormant Capacity Amendment closes its
bounded product claim without creating any WorkItem, Run, Grant or execution
fact.

## Mission preflight stop

The New Mission flow accepted one bounded objective and invoked the ordinary
read-only product preflight. It returned `conflict`. One explicit product
refresh succeeded, after which one second read-only preflight returned the same
`conflict`. The Controller did not Start the Mission, call a direct execution
command, mutate SQLite, replace Runtime metadata or retry the daemon.

A read-only source/Journal reconciliation identified the exact fail-closed
incompatibility:

```text
live Runtime model ID:
  loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m

mission binding authority expects:
  qwen2.5-coder-1.5b-instruct-q4-k-m
```

`ProjectionMissionExecutionBindingSource` rejects the namespaced live model ID
before compilation. The deterministic tests cover only the unnamespaced ID.
Changing this accepted mission-binding authority is outside the frozen
Saved-Team-only Amendment, so the canary stopped rather than expanding scope.

The TUI also displayed the internal TeamDefinition identifier in the New
Mission selector and collapsed spaces while entering the objective. These are
non-authoritative product-experience findings; neither was used to bypass or
reinterpret the preflight conflict.

## Shutdown and authoritative postflight

The TUI exited normally. One normal interrupt stopped the single daemon with
exit `0` and:

```json
{"completed_cycles":12,"discovery_events":1,"status_events":0,"no_write_cycles":11}
```

The final DB is private, integrity-valid and SHA-256:

```text
677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
```

Its complete facts are:

```text
AgentGrantIdentityIndexInitialized | 1
AgentInstanceCreated                | 1
RuntimeInstanceDiscovered           | 1
TeamDefinitionSaved                 | 1
TeamInstanceCreated                 | 1
WorkRunIdentityIndexInitialized     | 1
```

There is zero WorkItem, Run, Grant, Evidence, dispatch or TeamExecution fact.
The product socket and product lock are absent, the retained SQLite lock is
unheld, isolation is empty, and the daemon, TUI, native app, Pi, Codex and
llama-server processes are absent. A non-database attempt/evidence scan found
no raw credential, token or API-key material.

**VERDICT**: `FAIL — PRODUCT PREFLIGHT CONFLICT / NO START`
