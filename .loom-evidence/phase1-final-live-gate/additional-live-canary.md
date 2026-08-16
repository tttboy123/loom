# Final Live Gate Additional Controlled Live Canary

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-FRESH-ATTEMPT-1`
- Compatibility commit: `e32f650`
- Contract Review: `PASS`
- Implementation Review: `PASS`
- Execution date: `2026-07-27`
- Approval decision: `approved`
- Execution authorization: `controlled local`
- Authorized invocations: `1`
- Invocations used: `1`
- Retry or fallback used: `NO`

## Pre-live gate

Immediately before invocation, the Controller revalidated:

- installed Pi version `0.82.1` and CLI SHA-256
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`;
- installed llama.cpp SHA-256
  `a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`,
  regular non-symlink mode `0700`, uid `501`;
- model SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`,
  size `1117320768`, regular non-symlink mode `0600`, uid `501`, and exact
  `GGUF` little-endian v3 header;
- source-lock SHA-256
  `e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5`;
- private root as current-user-owned, non-symlink mode `0700`;
- old canary SQLite SHA-256
  `914f4d872246cfb124be17136d6c3077c08dd3bb2697f9b517d8ee14e65e76ae`;
- no pre-existing additional manifest or additional attempt root;
- no listener on loopback port `18427`; and
- no residual Pi or llama process.

Every pre-live check passed.

## Single invocation

The Controller ran the contract-frozen opt-in test exactly once:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

The invocation exited `1` after `5.520s`. The live test failed after `4.98s`.
The bounded diagnostic was:

```text
final live Team execution failed
Team attempt evidence commit main/1: invalid Team node recovery
runtime adapter failure
Pi RPC protocol failed
response=true agent=true turns=1 message=true done=false settled=false
```

The adapter therefore observed the RPC response, Agent start, one turn, the
user-message binding, and an assistant-message start, but it rejected the
transcript before an accepted assistant `message_end`, `agent_end`, and
`agent_settled` terminal sequence. It emitted no accepted successful
`AdapterResult`.

The authorization was consumed by this failure. The Controller performed no
rerun, retry, compaction recovery, alternate parser change, alternate model,
Provider switch, network fallback, or second additional canary.

## Authoritative failure outcome

Read-only SQLite inspection found `21` Events with:

- `21` distinct Event IDs;
- `21` distinct `(stream_id, seq)` pairs;
- one `RunTerminalCommitted` with status `failed` and reason
  `runtime_process_failed`;
- one `TeamNodeAttemptTerminal` with status `failed`, output classification
  `invalid`, and Evidence digest
  `80251826f53b3de12e039f1da58a8e2f5b6612637943c3f477671c544f89970b`;
- one `WorkItemTerminal` with status `failed`;
- one `AgentGrantRevoked` with reason `terminal`;
- one `RuntimeCapacityReleased`; and
- one `EvidenceSubmitted`.

The failure artifact records `child_result_observed=false` and one authorized
frame without persisting raw Grant material. Grant facts contain a token hash,
not a raw Grant value.

Private evidence digests:

```text
canary.sqlite
ba1ef877ccd4ebacafe492eeefca88e10602c3133770b201a624ad996a5fb1a3

bounded source
bfc5aca5df67c1244e4d28031f973404e5a118d5587288609a625906d3d1a54f

failure artifact
80251826f53b3de12e039f1da58a8e2f5b6612637943c3f477671c544f89970b

Evidence capture
cb3da6b54c245ed2778386389552a3eb6e6caffdd2878c57202f69d16bc01768

Evidence receipt
9e4571f8d5b35af9315523890419336ee561d35a084c7dc80722addf06dd3e2b
```

## Isolation, manifest, and cleanup

Exactly one new direct-child `controlled-canary-additional-*` attempt root
exists. It is a current-user-owned non-symlink directory at mode `0700`.
Every descendant directory is `0700`; SQLite, bounded source, artifact,
capture, and receipt are regular files at mode `0600`.

The independent additional manifest is a regular uid `501` file at mode
`0600`, size `1590`, with SHA-256:

```text
222d6004fc06a8afc3e1c9a8369216d9b6f3f97fa116a386c039df27362c3c32
```

It contains the frozen Runtime/model/server digests, bounded authorization,
and digest-only prompt binding. It contains no private absolute path, mirror
URL, prompt text, raw Grant, credential, hidden reasoning, or model output.
The prior manifest remains byte-for-byte unchanged.

Post-failure cleanup and revalidation proved:

- loopback port `18427` has no listener;
- no Pi or llama process remains;
- Pi, llama.cpp, model, and source-lock digests remain unchanged;
- the old canary SQLite and all prior final-live evidence hashes remain
  unchanged; and
- the new private evidence scan found no private absolute path, mirror URL,
  private-key marker, or token-shaped credential.

## Fresh result-evidence review

A fresh independent read-only Reviewer returned `PASS` with no findings.

The Reviewer independently confirmed:

- exactly one additional attempt and no retry or fallback;
- the SQLite, manifest, artifact, capture, receipt, source, installed binding,
  source-lock, prior evidence, and old-attempt digests;
- uid `501` and exact `0700`/`0600` modes;
- `21` unique Events and the single failed terminal/revocation/release/
  Evidence lineage;
- `child_result_observed=false`, one authorized frame, and no output payload;
- manifest and private-evidence sanitization;
- empty port `18427` and no residual Pi/llama process; and
- `HUMAN_REQUIRED — NO RETRY`, with no final sign-off or further-run claim.

This `PASS` applies only to evidence accuracy and completeness. It does not
change the controlled canary's failed product result.

## Gate result

The additional controlled live canary failed closed. Phase 1 does not reach
`READY_FOR_FINAL_USER_SIGNOFF`. No further live attempt is authorized by this
contract. The result-evidence review passed, but it cannot change the failed
product result or authorize a rerun.

VERDICT: FAIL
