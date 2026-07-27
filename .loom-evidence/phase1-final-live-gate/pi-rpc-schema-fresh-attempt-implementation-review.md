# Final Live Gate Pi RPC Schema and Fresh Attempt Isolation Implementation Review

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-FRESH-ATTEMPT-1`
- Baseline: `b34d8da635c6037c6f3658d580c1dc75bf541f63`
- Review date: `2026-07-27`
- Reviewer: fresh independent read-only Implementation Reviewer

## Findings

No blocking findings.

The Reviewer confirmed:

- `materializePiRPCSettings` uses direct final-path
  `O_WRONLY|O_CREATE|O_EXCL`, `0600`, write, `fsync`, close, and exact-byte
  revalidation before process start;
- `piRPCUserMessage` accepts only the Pi `0.82.1` one-text-block array and
  rejects the legacy string form;
- JSON `null` is explicitly rejected by the shared non-negative-number
  validator;
- existing strict transcript checks bind the user message through
  `message_start`, `message_end`, and `agent_end.messages`;
- the rejection matrix covers malformed, duplicate, extra, other-block,
  prompt-drift, timestamp, and settings-collision cases;
- pre-existing settings regular files, symlinks, and directories are preserved
  and prevent process start;
- the additional manifest filename is distinct from prior evidence;
- the live harness creates a fresh direct-child
  `controlled-canary-additional-*` attempt root under a validated `0700`
  private root;
- SQLite is pre-created once with `O_CREATE|O_EXCL`, regular non-symlink mode
  `0600`, before `sql.Open`; and
- no Bridge v1, Journal authority, policy, credential, daemon, scheduler,
  dependency, source-lock, or Runtime/model-installation boundary changed.

The Reviewer independently reran:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=1
PASS

go test ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=1
PASS

go test ./internal/runtime/piadapter ./internal/app -count=1
PASS
```

## Non-blocking note

`writeFinalLiveResolvedManifest` retains its pre-existing temp-file-plus-rename
publication style for the new additional manifest path. The current contract
forbids overwrite semantics specifically for `settings.json` and requires the
additional manifest not overwrite or alias prior
`resolved-live-manifest.json`; the Candidate satisfies that boundary.

If future governance requires a pre-existing additional-manifest path to fail
closed, it should be frozen as a separate hardening requirement rather than
retroactively expanding this contract.

The Reviewer made no edit, stage, commit, network request, Runtime/model/
Provider invocation, live canary, credential access, or private-attempt
mutation.

VERDICT: PASS
