# S2-W37 Candidate Deliverable

- WorkItem: `S2-W37`
- Title: One-Shot Triggered Prepared Runtime Observation
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `38891c3`
- Contract SHA-256:
  `999d9fba2022fc2b059b9394b26b04e9208ffe4b0f018707a327e937ff5f9159`
- Contract Review 1 SHA-256:
  `9fa35ddc67ade9984c7c9490f3c95b2114542a491a37a08d1f26d357a4c3ca8f`
- Amendment 1 SHA-256:
  `f9ceb9118feca26cac8ab5ece6b753305ff9840d7a65a6eaf81c676ed0c6baf7`
- Amendment Review 2 SHA-256:
  `b32104b1a30fdcabb6fc4292a300c40ccfdd272e0e9546`

## Contract, RED, and Candidate

Contract Review 1 returned `FAIL` before implementation because the typed-nil
trigger requirement appeared incompatible with the new-file import whitelist.
Amendment 1 requires the accepted same-package `nilAppInterface(trigger)`
helper, without a `reflect` import, helper/API change, or new authority. Fresh
Amendment Review 2 returned `PASS` with no findings.

Mandatory RED added only `runtime_observation_trigger_test.go`; focused
compilation failed solely because the frozen trigger interface, error, and
function symbols were missing. The minimal Candidate validates context,
trigger, and prepared observer; awaits the injected trigger exactly once; then
delegates exactly once to the accepted S2-W36 observer. Trigger, context, and
downstream errors return five zero outputs with no fallback or retry.

```text
47baf06b4e5a032d22c68696649d39717b078093b30772294537e6f4d8e31be6  internal/app/runtime_observation_trigger.go
a1389a846a6ea94ca460504dad29356212cd8cf4d3ad6d6dc976e01cf78d91e8  internal/app/runtime_observation_trigger_test.go
```

## Controller verification

The complete strict matrix passes: focused app tests, app package, impacted
app/runtime/discoveryscan/state/projection/journal packages, focused race at
50 repetitions, repository tests, repository race, vet, formatting, and diff
checks.

Focused proof covers nil and typed-nil inputs, pre-canceled and deadline
contexts, trigger error and trigger-induced cancellation, exact
trigger-before-observer ordering, discovery-priority and status paths,
mutation isolation, and the complete direct downstream error matrix. Every
failure proves five zero outputs, exact selected/opposite call counts, and no
fallback or retry.

The real SQLite chain performs two explicit triggered discovery calls and two
explicit triggered status calls. It proves exact discovery/discovery/status
Event types with sequences 1/2/3, idempotent retries, opposite-writer
exclusion, and a final rebuilt Runtime projection with the changed display
name, online status, discovery sequence 2, and status sequence 3.

Static proof permits only the frozen imports; requires exactly one
`nilAppInterface(trigger)`, one trigger await, and one observer `RunOnce` in
that order; and rejects time, timer, ticker, channel, signal, loop, goroutine,
configuration, daemon, direct lower-layer, activation, and Slice 3 authority.

## Trust boundary

The trigger is injected and caller-driven. The Candidate adds no time source,
scheduler, recurrence, goroutine, channel, process signal, configuration,
daemon lifecycle, retry, projection rebuild, direct Journal/SQLite/Event
metadata access, Runtime activation, or execution authority. It only gates one
accepted S2-W36 call.

## Review gate

Fresh independent Implementation Review 1 returned `PASS` with no findings.
The Reviewer independently verified the complete matrix, direct error surface,
real SQLite chain, static boundary, and evidence. The Candidate is accepted
The fresh pre-commit focused, app, impact, focused-race-50, repository,
repository-race, vet, formatting, and diff matrix also passes. It may receive
its one exact-scope local atomic commit after the staged-scope audit.

VERDICT: PASS
