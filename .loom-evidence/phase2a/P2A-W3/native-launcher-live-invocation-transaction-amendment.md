# P2A-W3 Native Launcher Live Invocation Transaction Amendment

**Date**: 2026-08-02  
**Status**: `FROZEN / PENDING INDEPENDENT AMENDMENT REVIEW`  
**Risk**: `STRICT / LIVE GOVERNANCE ONLY`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Parent**: unique `P2A-W3 Controlled Execution Experience`  
**Parent closure contract SHA-256**:
`4493858a9be2411ba6cac1594df7bfdc0e4e7be062f5ec528b7c3746c7d30b18`  
**Implementation source lock SHA-256**:
`f6bb68656cd70b2c9d75861bc5a77325ea488bd78dcfa49fc0c1cc41dd64d772`  
**Result lock SHA-256**:
`0a66ec718634a5f809abbabf14ccf3cad65fa347625bfbc41ab4c9fc74f1fbdd`  
**Result Review SHA-256**:
`1e60ec960ff52878f0cdcc280d219028d8ee0e1d369d11f5c6784802291ec422`  
**New WorkItem**: none; `P2A-W4` does not exist

## 1. Causal basis

MiniMax-004 passed its exact setup-only product claim. Pi-004 did not exercise
the product: the Controller inserted one extra positional `daemon` token before
the frozen flag array. loomd rejected that non-manifest vector during argument
parsing with exit `2` and `invalid input`; SQLite remained byte-identical and
no daemon, socket, Pi, local model, preflight, Start or Provider action began.

The fresh independent Result Reviewer accepted the evidence and classified
this as `controller_invocation_error`, not a product defect. Pi-004 remains
consumed and cannot be retried. The parent contract authorizes no additional
live attempt, so a replacement may exist only through this reviewed amendment.

## 2. Frozen scope

This amendment changes only the live launch transaction and allowance. It
changes no production/test source, Event schema, Journal, Projection,
StateWriter, Rules, Work, Grant, Evidence, Supervisor, Runtime adapter, Pi RPC,
Provider, credential, setup, IPC, native/TUI product surface, scheduler, queue,
Team, Mission or execution authority.

It authorizes, after independent Amendment Review PASS only, exactly one wholly
new Pi replacement lineage. It does not authorize another MiniMax action,
reuse of Pi-004, a third product implementation repair, or P2A-W4.

## 3. Exact argv-attested launch transaction

The replacement manifest must freeze all of:

- one new attempt ID and non-existing user-owned non-symlink root mode `0700`;
- one pre-created regular SQLite mode `0600`, byte-identical to the retained
  six-fact initial state;
- the exact loomd executable path and SHA-256;
- the exact ordered 32-element `daemon_args` JSON array;
- canonical invocation SHA-256 over the UTF-8 bytes, including the final LF,
  produced by:

```text
jq -cS --arg executable "$EXACT_LOOMD" \
  '{executable:$executable,args:.daemon_args}' manifest.json
```

Before creating the one start marker or invoking the binary, the Controller
must independently reproduce that digest, prove array length `32`, prove the
first element is `--state`, prove every non-value flag is one of the frozen
allowlist below, and prove there is no positional argument:

```text
--state
--isolation-root
--runtime-dir
--probe-id
--instance-id
--device-id
--display-name
--interval
--process-timeout
--max-cycles
--socket
--codex-executable
--local-model-private-root
--local-model-executable
--local-model-path
```

`--runtime-dir` occurs exactly twice; every other flag occurs exactly once.
The Controller must pass the frozen array directly after the loomd executable.
No subcommand, wrapper-added positional token, PATH lookup or reconstructed
free-form string is permitted. The start marker is created only after these
checks and immediately before the single binary invocation.

Any mismatch, parse rejection or early exit consumes the replacement lineage.
There is no hidden retry, fallback, second invocation or in-place manifest
edit.

## 4. Replacement Pi boundary

The one replacement lineage retains the parent Pi claim and limits:

- exact installed Pi `0.82.1`, locked local llama-server/model, official
  user-level npm Codex launcher and passing implementation source lock;
- one daemon invocation/start, one saved-Team read-only preflight, at most one
  explicit Start, and zero network Provider requests;
- no retry or compaction;
- exact Supervisor, generation, Grant, authorized Frame, Source Evidence,
  Verifier Evidence and canonical Team terminal closure;
- complete raw stdout/stderr/exit capture, SQLite facts/hash/integrity,
  socket/IPC-lock/isolation/process cleanup and bounded non-disclosure scan;
- the unrelated `demo-resident` observer remains untouched.

The initial saved Team remains the retained exact Main plus dormant SubAgent
binding. Preflight and Start must be performed through the ordinary product
surface; no direct authority command or SQLite mutation is permitted.

## 5. Review and exit gates

In order:

1. immutable amendment hash and fresh independent Amendment Review PASS;
2. exact implementation source-lock and external-artifact revalidation with no
   source drift;
3. one fresh manifest/root preflight including the canonical argv digest;
4. one and only one replacement Pi invocation;
5. complete postflight and fresh independent Result Review over retained
   MiniMax-004 plus the replacement Pi lineage;
6. only if both exact claims PASS, the parent final no-terminal walkthrough,
   final source/evidence lock, final independent review and atomic W3 commit.

If the amendment Review fails, source/artifacts drift, argv attestation fails,
the binary exits early, the product claim fails, or cleanup is incomplete,
stop `HUMAN_REQUIRED`. Do not improvise another attempt.

Until Amendment Review PASS:

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / LIVE INVOCATION TRANSACTION AMENDMENT REVIEW PENDING / NO LIVE / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```
