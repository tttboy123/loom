# Final Live Gate Progressive Assistant Identity Controlled Live Canary

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-PROGRESSIVE-IDENTITY-1`
- Compatibility commit: `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`
- Contract Review: `PASS`
- Implementation Review: `PASS`
- Execution date: `2026-07-27`
- Approval decision: `approved`
- Execution authorization: `controlled local`
- Authorized invocations: `1`
- Invocations used: `1`
- Retry, fallback, compaction, or alternate model used: `NO`

## Pre-live gate

Immediately before invocation, the Controller revalidated:

- exact committed Candidate
  `a4e6f73cdbcf2dcdc986a5c04eadf9b5c32360ca`;
- installed Pi CLI SHA-256
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`;
- all seven locked installed Pi `0.82.1` source hashes in
  `source-lock.json`;
- installed llama.cpp SHA-256
  `a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b`,
  regular non-symlink mode `0700`, uid `501`;
- model SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`,
  size `1117320768`, regular non-symlink mode `0600`, uid `501`, and exact
  `GGUF` little-endian v3 header;
- source-lock SHA-256
  `e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5`;
- the private root as current-user-owned, non-symlink mode `0700`;
- all prior final-live evidence and both prior canary SQLite hashes;
- absence of the new manifest and progressive-identity attempt root;
- no listener on loopback port `18427`; and
- no final-live Pi, llama-server, or Loom daemon process.

Two preliminary process-filter commands returned false positives before the
invocation because their unanchored patterns matched an unrelated Playwright
`loomdemo` CLI daemon substring and the inspection shell's own command line.
They started no Runtime or model and created no manifest or attempt. Exact
anchored executable checks then passed. This did not consume the one live
authorization.

Every pre-live binding check passed before the controlled invocation.

## Single invocation

The Controller ran the contract-frozen opt-in test exactly once:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

The invocation exited `1` after `4.994s`; the live test failed after `4.25s`.
The bounded diagnostic was:

```text
final live Team execution failed
Team attempt evidence commit main/1: invalid Team node recovery
runtime adapter failure
Pi RPC protocol failed
response=true agent=true turns=1 message=true done=false settled=false
```

The installed Runtime discovery and local model path reached Pi RPC. Loom
accepted the RPC response, Agent start, one turn, user-message binding, and
assistant `message_start`, but again rejected the transcript before accepted
assistant `message_end`, `agent_end`, and `agent_settled`.

No accepted successful `AdapterResult`, output Frame, or output payload was
produced. The authorization was consumed by this failure. The Controller
performed no rerun, hidden retry, fallback, compaction recovery, parser
widening, alternate model, Provider switch, or network fallback.

This result proves the progressive identity Candidate did not close the live
Pi `0.82.1` transcript boundary. It does not prove the next exact rejection
field because no raw live transcript was captured or persisted.

## Authoritative failure outcome

Read-only SQLite inspection found `21` Events with:

- `21` distinct Event IDs;
- `21` distinct `(stream_id, seq)` pairs;
- one `RunTerminalCommitted` with status `failed` and reason
  `runtime_process_failed`;
- one `TeamNodeAttemptTerminal` with status `failed`, output classification
  `invalid`, and Evidence digest
  `b5bf37c277da0e3a14b244879ffa580a3fbdfb76fc10f360fab63062920dce1e`;
- one `WorkItemTerminal` with status `failed`;
- one `AgentGrantRevoked` with reason `terminal`;
- one `RuntimeCapacityReleased`; and
- one `EvidenceSubmitted`.

Grant facts contain a token hash and no raw token. The failure artifact and
receipt prove:

- `child_result_observed=false`;
- exactly one authorized Ack Frame;
- `output_frame_count=0`;
- `output_payload_bytes=0`;
- `result_observed=false`; and
- terminal status `failed`.

Private evidence digests:

```text
canary.sqlite
9f6ff64d8ceaf09dc7bf4b4747f141cffc942a9a4bd5113d5302789b58422a7f

bounded source
bfc5aca5df67c1244e4d28031f973404e5a118d5587288609a625906d3d1a54f

failure artifact
b5bf37c277da0e3a14b244879ffa580a3fbdfb76fc10f360fab63062920dce1e

Evidence capture
15d5da321fa994276c84d6ee893c785ddb0d344a58653beda1733d9713edb603

Evidence receipt
ca328b8ad7fe9e05da1768eb3f8bcf5bf94a8e65e03544079a515f665749af74
```

## Isolation, manifest, sanitization, and cleanup

Exactly one new direct-child
`controlled-canary-progressive-identity-*` attempt root exists. It is a
current-user-owned non-symlink directory at mode `0700`. Every descendant
directory is `0700`; SQLite, bounded source, artifact, capture, and receipt are
regular files at mode `0600`.

The independent progressive-identity manifest is a regular uid `501` file at
mode `0600`, size `1590`, with SHA-256:

```text
da04a07f47e84e278b29a841f9495849ffd923f27fe00acd4f8fc2d84e83f16c
```

It contains the frozen Runtime/model/server digests, bounded authorization, and
digest-only prompt binding. It contains no private absolute path, mirror URL,
prompt text, raw Grant, credential, hidden reasoning, or model output.

The new private Evidence scan also found no private absolute path, mirror URL,
private-key marker, raw-Grant marker, prompt text, hidden reasoning, or model
output.

Post-failure cleanup and revalidation proved:

- loopback port `18427` has no listener;
- no final-live Pi, llama-server, or Loom daemon process remains;
- Pi, llama.cpp, model, and source-lock digests remain unchanged;
- the old canary SQLite, additional canary SQLite, and every prior final-live
  evidence hash remain unchanged; and
- no prior attempt or manifest was modified.

## Fresh result-evidence review

Fresh independent read-only Review 1 verified:

- exactly one new attempt and one invocation, with no retry or fallback;
- manifest, SQLite, source, artifact, capture, receipt, historical evidence,
  and prior SQLite digests;
- uid `501` and exact `0700`/`0600` modes;
- `21` unique Events and the single failed
  terminal/revocation/release/Evidence lineage;
- `child_result_observed=false`, one authorized Ack, zero output Frames/bytes,
  and no accepted result;
- no raw Grant value;
- manifest and private-Evidence sanitization;
- empty port `18427` and no final-live Pi/llama/loomd process; and
- private materialization status and `HUMAN_REQUIRED — NO RETRY`.

Review 1 returned `FAIL` only because the Controller requested an early stop
before the Reviewer independently completed the installed Pi CLI and seven
locked source-file hashes. This was an evidence-completeness gap, not a product
repair or permission for another invocation.

Bounded Repair Review 2 independently hashed those exact eight files. Every
value matched `source-lock.json`:

```text
Pi CLI
af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

rpc-types.d.ts
122dec2245c472e15b71b6114c760eaf67937266482548e3bbafe2f13936aacc

pi-ai types.d.ts
95fddce61009f9ed0e97eb402e5438dfc980b76fc155edd4d6fc0ed3b71a0496

openai-completions.js
0d50250fe2931e66e2078279a397814202e1ecddee58faf4b8bc04c278da177a

pi-agent-core types.d.ts
849dfc75410c3651425b773db68ecb55da98e9fb15a8c6dc8fe17a845c966b07

agent-loop.js
d3d20bc773ccc8d5f7cfe0eabf8b421ff3da8685617445b142d89a44457741bc

agent-session.d.ts
4ab4df46f07f2a13dc3e289c1926432acae7e6df8acfa2b13699c799d6876b47

rpc-mode.js
c87945c1b3e81db3d7672735e9846a64b892e7184bc3d796d1affcd187b76ec2
```

Repair Review 2 returned `PASS` with no findings. This `PASS` applies only to
evidence accuracy and completeness. It cannot change the failed product result
or authorize another canary.

## Gate result

The progressive assistant identity controlled canary failed closed. Phase 1
does not reach `READY_FOR_FINAL_USER_SIGNOFF`. No further live attempt is
authorized by this amendment.

The current state remains:

```text
HUMAN_REQUIRED — NO RETRY
```

The result-evidence review is `PASS`. It validates only evidence and cleanup;
it does not change the failed product result or authorize another invocation.

VERDICT: FAIL
