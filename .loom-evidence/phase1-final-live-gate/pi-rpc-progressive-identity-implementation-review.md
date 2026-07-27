# Final Live Gate Pi RPC Progressive Assistant Identity Implementation Review

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-PROGRESSIVE-IDENTITY-1`
- Baseline: `e32f65035ec7c477bbee9e60cffa7c0989b028ce`
- Review date: `2026-07-27`
- Reviewer: fresh independent read-only Implementation Reviewer

## Findings

No Critical or Important findings.

The Reviewer confirmed:

- initial assistant identity binds immutable fields and rejects initial
  `responseId`, `responseModel`, `cacheWrite1h`, and `reasoning`;
- first `text_start` is the only response-ID binding point;
- every later partial requires exact response-ID stability and rejects
  `responseModel` and `cacheWrite1h`;
- `reasoning` presence may only progress monotonically on an assistant update;
- terminal validation requires an already-bound response ID and cannot
  introduce or remove `reasoning`;
- tests encode a genuine progressive happy path and adversarial prepopulated,
  missing, removed, terminal-removed, mutated, empty, oversized, and late
  response IDs, response-model substitution, timestamp drift, partial
  mismatch, cache-write presence, and reasoning monotonicity;
- manifest and attempt lineage are independent from all prior final-live
  evidence;
- no retry, fallback, parser widening, Bridge v1 change, authority change, or
  post-review gate bypass was introduced; and
- the complete Candidate is limited to the three owned Go files plus new
  progressive-identity governance evidence.

The only Minor note is that the worktree retains pre-existing out-of-scope
dirty and untracked files. They do not block this Candidate, but the Controller
must stage and commit only the exact owned files.

## Independent commands

The Reviewer independently ran:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821ProgressiveAssistantIdentity|ProgressiveAssistantIdentityRejections)$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/runtime/piadapter 7.683s

go test ./internal/app \
  -run '^TestFinalLiveProgressiveIdentityCanaryIsolation$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/app 0.546s

go test ./internal/runtime/piadapter ./internal/app -count=1
PASS:
  internal/runtime/piadapter 29.155s
  internal/app 8.347s

git diff --check e32f65035ec7c477bbee9e60cffa7c0989b028ce -- <owned files/evidence>
PASS: empty output
```

The Reviewer also confirmed:

- no progressive-identity manifest exists;
- no `controlled-canary-progressive-identity-*` attempt exists;
- loopback port `18427` has no listener; and
- no final-live llama-server, Pi Agent, or Loom daemon process is running.

The Reviewer performed no edit, stage, commit, network request, Runtime/model
invocation, live canary, credential access, or private-attempt mutation.

## Assessment

The Candidate satisfies the frozen progressive Pi RPC identity amendment.
Live remains blocked until:

1. one local atomic commit contains exactly the owned Candidate files;
2. installed Pi, llama.cpp, model, source-lock, private root, historical
   hashes, process, and port bindings are freshly revalidated; and
3. the independent sanitized manifest is created immediately before the one
   authorized invocation.

VERDICT: PASS
