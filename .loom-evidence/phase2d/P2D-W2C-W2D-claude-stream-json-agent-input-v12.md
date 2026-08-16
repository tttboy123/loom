# P2D-W2C/W2D Claude stream-json Agent Input continuation v12

Date: 2026-08-14  
Status: `SOURCE VERIFIED / SAME-PROCESS ONLY / INSTALLED LIVE OPEN`

## Boundary

Claude Code Agent Attempts with a durable Agent Input source may now retain one
version-locked `claude --print` process and one emitted Claude `session_id`
across multiple Loom Queue/Steer/Inject rounds. The process runs with
`--input-format stream-json`, `--output-format stream-json`,
`--replay-user-messages`, and `--no-session-persistence`.

This is in-memory same-process continuity. It does not use Claude's persisted
session store, does not resume after daemon restart, and does not run the real
Claude binary, Provider network, Loom App or installed-live matrix.

## Local protocol lock

Read-only inspection identified Claude Code `2.1.196` and bound continuation
conformance to the exact executable digest:

`sha256:6fc6e61ab7582c2bf241225ff90d9f79e91d69380cb9589fc9dedd3a30070f5a`

Local binary evidence confirms the stream-json input/output flags,
`--replay-user-messages`, `--no-session-persistence`, `session_id`,
`tool_use`/`tool_result`, and `system/compact_boundary` protocol vocabulary.

## Implementation

- The shared bounded system session runner supplies the process, process-group,
  executable-identity, cancellation and JSONL privacy boundary.
- The first stream event must be exact `system/init` for model
  `claude-sonnet-5`, Claude Code `2.1.196`, and one bounded session ID. Every
  later event must retain that session ID.
- Each Loom input must be replayed exactly once as the first user message in its
  round. Internal Claude tool loops are accepted only as typed
  `assistant/tool_use -> user/tool_result` pairs with unique matching IDs. All
  tool uses must close before the final result.
- `system/compact_boundary` may occur only after initialization and the current
  replayed user input. It neither changes the frozen Loom binding nor creates an
  Agent Input checkpoint.
- Only a successful final `result` creates the output checkpoint. Harness usage
  and reported USD cost are validated and accumulated across all Loom rounds.
- The Adapter advertises Agent Input support only when the persistent runner and
  exact executable conformance lock both pass. Drift fails before credential
  access; ordinary requests retain the existing one-shot JSON command.
- Prompt, input JSON and Inbox bytes are cleared after bounded use. Raw stderr,
  tool result content, Prompt and Provider body are not projected to
  diagnostics, Journal or Evidence.

## Verification

Mandatory RED failed on the missing session-runner injection, missing Adapter
continuation contract, session substitution, and a normal internal tool loop
that the initial text-only parser rejected. GREEN covers two Loom rounds, one
stable session, exact replayed user input, typed tool pairing, compaction,
session substitution, accounting aggregation and lifecycle cleanup.

The same Harness and affected product commands recorded in the V11 evidence
pass. The earlier parallel fixture timeout and subsequent ten-run focused plus
complete serial pass are preserved there rather than rewritten.

## Open gates

Encrypted ExternalSessionHandle persistence, daemon-restart reattachment,
installed Claude 2.1.196 conformance, real Anthropic response, CV6, ATL9
mixed-Team acceptance, COMP2-E and final UI/accounting/fallback governance
remain open under the sole Phase 2D Goal.
