# Final Live Gate Compatibility Implementation Review

Date: 2026-07-27

## Review 1

Reviewer: fresh independent read-only implementation Reviewer

Verdict: `FAIL`

The first implementation review found five blocking gaps:

1. stdout stopped being inspected at `agent_settled`, so a trailing record could
   escape protocol validation;
2. nested Pi message objects were role-checked but did not have frozen
   Pi 0.82.1 field/content/usage allowlists;
3. metadata path lines could lose trailing control whitespace during trimming;
4. the live harness did not create and validate a sanitized resolved manifest
   before any live operation; and
5. local-model evidence lacked direct tests for binding drift, occupied port,
   invalid health status/body, cancellation, unsafe symlink/mode, and inherited
   credential/proxy removal.

No external asset was materialized and no live canary ran.

## Bounded Repair 1

Status: `IMPLEMENTED — REVERIFICATION AND FRESH REVIEW PENDING`

The same Candidate lineage now:

- closes stdin at `agent_settled`, drains stdout to EOF, and rejects every
  post-settled record;
- binds the installed Pi 0.82.1 user/assistant event sequence and strict nested
  message, text, usage, cost, completion, and agent-end schemas;
- rejects CR and path-line controls before metadata trimming;
- creates a digest-validated, path/prompt-sanitized, atomic 0600 resolved
  manifest before metadata discovery, model startup, or Provider invocation;
  and
- adds the missing local-model adversarial lifecycle and binding tests.

This file deliberately preserves Review 1 `FAIL`. A fresh independent Review 2
verdict will be appended only after the mandatory hermetic matrix passes again.

## Review 2

Reviewer: fresh independent read-only implementation Reviewer

Verdict: `FAIL`

Review 2 found two blockers:

1. the locked `pi-agent-core` source maps Provider `start`/`done` to top-level
   assistant `message_start`/`message_end`, while the corrected implementation
   and fixture still invented nested `message_update` start/done events; and
2. local-model binding did not reject parent symlinks, public intermediate
   directories, or realpath escape below the inferred lexical common root.

No external asset was materialized and no live canary ran.

## Fresh problem analysis

The required repeated-failure analyst confirmed that package drift, JSONL
framing, cleanup, network behavior, and llama.cpp output are not the root cause.
The root causes are a synthetic transcript inconsistent with the locked Agent
loop and an inferred rather than explicit private-root security boundary.

The analyst recommended bounded technical Contract Correction 2: encode the
actual one-prompt event sequence, add explicit `PrivateRoot`, share one read-only
full-chain binding inspector with the manifest/start paths, use no-follow leaf
opens plus owner/mode/identity/digest revalidation, and disclose the bounded
same-user pathname-reopen residual. Contract Review 3 must pass before product
Repair 2.

VERDICT: FAIL

## Bounded Product Repair 2

Status: `IMPLEMENTED AND VERIFIED — FRESH REVIEW 3 PENDING`

After Contract Review 8 `PASS`, the same Candidate lineage:

- replaced the synthetic nested Pi start/done transcript with the locked
  top-level assistant start/end lifecycle;
- enforces semantic equality and stable identity across each top-level
  assistant message and nested text partial;
- enforces the exact Pi 0.82.1 three-physical-line metadata diagnostic;
- replaces inferred common-root binding with explicit `PrivateRoot` and a
  shared read-only full-chain inspector;
- enforces exact owner/mode/symlink/identity/digest invariants twice before
  process start; and
- makes the pre-live resolved manifest consume that same inspector.

The focused gate, mandatory matrix, repeated race/stability gates, fuzz run,
Windows compile, live-skip proof, and source-lock verification all pass. No
external asset was materialized and no live canary ran.

Fresh independent Implementation Review 3 must return `PASS` before a local
compatibility commit, materialization, or live canary.

## Review 3

Reviewer: fresh independent read-only implementation Reviewer

Verdict: `FAIL`

Review 3 found one concurrency/security blocker: local-model startup returned
immediately after the first exact health response without a final bounded
readiness hold and process-exit check. A child that served health and exited,
or a post-free-check port race whose unrelated listener served health while
the bound child failed, could therefore be reported as ready.

No external asset was materialized and no live canary ran.

## Bounded Product Repair 3

Status: `IMPLEMENTED AND VERIFIED — FRESH REVIEW 4 PENDING`

Repair 3 is limited to local-model readiness:

- add behavioral fixtures for health-then-exit and post-free-check port theft;
- require consecutive exact health observations across a bounded readiness
  hold while continuously watching the child `wait` channel;
- perform one final nonblocking process-exit check before returning the server;
  and
- preserve process-group cleanup and typed failure on every rejected startup.

No other API, authority, Runtime, RPC, metadata, or live scope may change.
Fresh Implementation Review 4 must return `PASS` before the local compatibility
commit, materialization, or live canary.

## Review 4

Reviewer: fresh independent read-only implementation Reviewer

Verdict: `FAIL`

Review 4 reproduced a focused timing flake in the final readiness repair. Under
load, the health-then-exit fixture's one-second startup context expired before
the child wait channel was observed:

```text
Pi local model health failed
context deadline exceeded
```

The startup remained fail closed, but the fixture required deterministic
`ErrPiLocalModelProcess` classification and the frozen focused command was not
stable. Isolated reruns passed, confirming a race at the timeout/exit
classification boundary rather than a constant failure.

The bounded repair budget is exhausted. Further production or fixture changes
require explicit user authorization for a narrowly scoped Repair 4. External
materialization, local compatibility commit, and live canary remain prohibited.

VERDICT: FAIL — HUMAN_REQUIRED

## Authorized Bounded Product Repair 4

Status: `IMPLEMENTED AND VERIFIED — FRESH REVIEW 5 PENDING`

The user explicitly authorized one additional narrow repair. Exact scope:

- arbitrate `ctx.Done()` against an already-imminent child wait result before
  classifying startup as a health timeout;
- raise only the health-then-exit fixture startup budget from one second to the
  existing successful fixture's ten-second test budget; and
- rerun the full matrix followed by fresh independent Implementation Review 5.

No public API, authority, Runtime/RPC/metadata behavior, source version,
materialization, or live scope may change. Review 5 `PASS` remains mandatory
before the local compatibility commit, materialization, or live canary.

## Review 5

Reviewer: fresh independent read-only implementation Reviewer

Verdict: `PASS`

Review 5 found no defect requiring change. It independently confirmed that
Repair 4 deterministically closes Review 4's timeout/child-exit classification
flake:

```text
go test ./internal/runtime/piadapter \
  -run 'TestPiLocalModelHealthContextClassification|TestPiLocalModelServerFailsClosed/health_then_early_exit' \
  -count=20
ok loom-pi-rebuild/internal/runtime/piadapter 11.725s
```

It also reran the complete focused Pi 0.82.1 metadata, Pi RPC Bridge, and local
model suite; the opt-in live harness tests; `git diff --check`; and the
ordinary live-skip proof. All passed, and the live canary remained skipped.

The Reviewer confirmed the strict RPC-to-Bridge mapping, Supervisor-owned
FrameSink authority, explicit-root local-model safety boundary, stable health
and process-exit checks, opt-in live gate, and exact amendment ownership.

The reviewed local compatibility commit may proceed. External materialization
remains prohibited until that commit exists; controlled live execution remains
prohibited until materialization and digest preflight pass.

No external asset was materialized and no live canary ran.

VERDICT: PASS
