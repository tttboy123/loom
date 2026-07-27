# Final Live Gate Pi 0.82.1 Transcript Compatibility Closure — Contract Review

Date: 2026-07-27
Baseline: `c7cabee40682f6e670d67b2553d85d6f8544b5c8`
Contract: `PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1`
Scope: read-only contract review; no product/test edit and no Pi/model/live
execution

## Review lineage

### Contract Review 1

Verdict: `FAIL`

Blocking findings:

1. pre-existing modified and untracked governance/user state was not
   sufficiently separated from Candidate scope;
2. the mandatory opt-in component could appear green by skipping when private
   Pi bindings were absent; and
3. no inspectable non-disclosing observation point proved that locked Pi
   actually produced forward-partial snapshots.

### Contract Repair 1 Review

Verdict: `FAIL`

Repair 1 froze exact dirty-worktree hashes, exact opt-in environment and
hard-fail rules, a mandatory proof sentinel, and a sanitized transcript audit
value.

The remaining Important finding was operational: an arbitrary no-return
interface observer could still block, panic, race, or mutate shared state and
therefore could affect execution despite sanitized data.

### Contract Repair 2 Review

Verdict: `PASS`

Critical findings: none.

Important findings: none.

The fresh independent Reviewer accepted:

- the unique-closure rule and prohibition on further point Amendments;
- freeze-time dirty-worktree quarantine and staged allowlisting;
- exact opt-in private binding variables, hard failure under opt-in, and
  mandatory verbose proof sentinel;
- the post-result, bool/int-only, best-effort send-only audit channel;
- non-blocking `select/default`, closed-channel panic containment, no retry,
  no goroutine, and adversarial nil/full/unbuffered/closed/concurrent tests;
- genuine hermetic, real locked-Pi component, and live-harness RED;
- the full Pi RPC lifecycle and multi-chunk forward-partial invariants;
- delta-only Frame/Evidence authority;
- real locked Pi `0.82.1` plus deterministic loopback SSE;
- read-only Supervisor/Grant/Frame/Evidence/Journal/Projection exercise; and
- the explicit `HUMAN_REQUIRED` stop if an excluded authority must change.

The Reviewer ran no test, Pi process, model process, or live canary. This review
authorizes RED only.

## Status-only finalization

After the PASS, the Controller changed only the contract status from
`CONTRACT REPAIR 2 REVIEW PENDING` to `CONTRACT REVIEW PASS — RED GATE` and its
terminal verdict from `PENDING` to `PASS`. No contract requirement, owned path,
acceptance predicate, check, trust boundary, or authorization changed.

## Implementation Review 1 and in-place Contract Repair 3

Implementation Review 1 returned `FAIL` before commit or live execution.

Important findings:

1. the final-RED test hashes drifted during bounded test-fixture correction
   despite the Repair-2 gofmt-only freeze;
2. the component did not assert the complete required
   Frame/Evidence/Journal/capacity/duplicate/failure closure; and
3. fixed `250 ms` cross-record pacing proved only a paced real-Pi path.

The Controller disclosed the exact test drift and current hashes in the same
Closure Contract. In-place Repair 3 now proposes:

- a fresh Repair-3 RED before Production Repair 1;
- prefix-comparable top-level/nested text witnesses with exact non-text
  equality, dual `D` prefix checks, monotonic longest witness, and exact
  terminal closure;
- an unpaced flush-per-record real-Pi success component;
- exact Frame/Evidence/artifact/receipt/capture/Journal/Projection assertions;
- a separate real-Pi observer-failure component with one failed Evidence
  lineage, revocation, release, and no retry; and
- no new Amendment, WorkItem, production file, dependency, or live
  authorization.

### Repair-3 Contract Review Attempt 1

Verdict: `INVALID / MIS-SCOPED`

The Reviewer reported that the current pre-Repair-3 implementation/tests did
not yet implement:

- prefix-comparable `M/P/W`;
- the new Repair-3 hermetic RED;
- unpaced real-Pi execution;
- the separate observer-failure component; and
- the expanded vertical assertions.

Those observations are accurate pre-RED state, but they are the work the
proposed contract explicitly forbids starting until fresh Contract Reviewer
`PASS`. They therefore do not assess whether the Repair-3 contract is safe,
bounded, complete, or verifiable. No contract defect was identified and this
attempt cannot authorize RED or implementation.

Fresh contract-only Repair-3 review is pending. Product/tests must not change
for Repair 3 and live remains locked until that review passes.

### Repair-3 Contract-only Review Attempt 2

Verdict: `FAIL`

The Reviewer found one valid Important contract inconsistency: the proposed
Repair-3 sequence correctly required a new RED against the current Candidate,
but the original real-Pi/audit subsection still required compilation to fail
because the audit type/configuration did not exist. That compile RED is
historical; the audit surface already exists after Repair 2.

The same contract now labels the old compile RED as immutable historical
lineage and defines the genuine Repair-3 RED against the current production
Candidate:

- prefix-comparable top/nested text skew in both directions;
- current exact-equality product fails only with the closed
  `message_partial_mismatch` reason;
- divergent controls remain rejected;
- real-Pi success/failure components and full vertical assertions are added
  at the same test-first checkpoint; and
- exact Repair-3 test hashes and unchanged production hash are recorded before
  Production Repair 1.

Fresh contract-only Repair-3 review remains pending. No Repair-3 product/test
implementation or live execution is authorized yet.

### Repair-3 Contract-only Review Attempt 3

Verdict: `FAIL`

The Reviewer found one remaining valid stale predicate: the original
live-harness RED still said the current Repair-3 baseline must fail on the old
rejection-diagnostic manifest/prefix, although the disclosed pre-Repair-3
harness already uses the closure manifest/prefix.

The contract now labels that harness failure as historical original RED. It is
not recreated. Repair 3 keeps the current harness GREEN and defines exactly
one mandatory new product RED command:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPCTranscriptClosurePrefixSkew$' \
  -count=1
```

That test targets the current exact-equality product and must fail only with
the closed `message_partial_mismatch` reason before Production Repair 1.

### Repair-3 Final Contract-only Review

Verdict: `PASS`

Critical findings: none.

Important findings: none.

The fresh Reviewer accepted:

- exact separation of historical original RED from the new Repair-3 RED;
- a credible deterministic prefix-skew RED against the current exact-equality
  product before Production Repair 1;
- exact disclosure and re-freezing of test-fixture drift;
- non-text-equal and text-prefix-comparable `M/P/W` safety with both-`D`
  prefix checks, monotonic witness, exact terminal closure, and delta-only
  authority;
- unpaced locked-Pi success plus separate observer-failure components;
- complete Frame/Evidence/artifact/receipt/capture/Journal/Projection and
  duplicate assertions;
- unchanged product/test ownership and read-only authority surfaces; and
- unchanged one-live-canary, no-retry, no-push/merge/release, and final user
  sign-off gates.

No tests, Pi process, model process, live canary, or private action ran during
this contract-only review.

Status-only finalization changed only the contract status and terminal verdict
from pending to Repair-3 Contract Review `PASS`. Repair-3 RED is now
authorized; Production Repair 1 and live execution remain locked until their
respective gates.

## In-place locked-Pi usage-skew repair review

The mandatory unpaced locked-Pi race component exposed a bounded Pi `0.82.1`
serialization mode during Production Repair 1: only nested partial usage had
advanced while top-level usage remained earlier; text and every other field
remained equal. The same unique Closure Contract now proposes a non-terminal,
top-level-lagging, component-wise usage progression rule, a deterministic
hermetic RED/rejection matrix, exact terminal equality, and unchanged
delta-only output authority.

The fresh contract-only Reviewer found no Critical or Important issue and
accepted:

- the exception is bounded to non-terminal `text_start` / `text_delta`;
- both messages remain independently schema-valid and all fields other than
  text/usage remain semantically equal;
- top-level usage may only precede nested usage component-wise, with no
  reverse, crossed, disappearing-optional, `cacheWrite1h`, malformed, or
  extra-key relation;
- nested partial remains the existing identity/usage progression input;
- `text_end` and later terminal records retain exact equality;
- usage cannot contribute output bytes, Frames, digests, Evidence, authority,
  or synthesis;
- the deterministic positive/rejection RED matrix precedes implementation;
  and
- the full component/race/repository matrix and one-live authorization remain
  unchanged.

No file was edited and no Pi, model, or live canary ran during review.

Verdict: `PASS`

Status-only finalization changed only this ledger and the unique contract's
status/terminal verdict. The usage-skew RED is authorized; implementation and
live execution remain locked behind their respective gates.

## Implementation Review 2 in-place repair review

Implementation Review 2 found a contract contradiction between the blanket
private-Evidence non-disclosure language and the existing read-only Evidence
authority's canonical authorized-Frame capture, plus a missing current-user
ownership assertion in the component.

The same unique contract proposes to preserve the authority boundary, permit
only private canonical Bridge/Grant-authorized Frames, retain strict
non-disclosure everywhere else, prove exact success/failure artifact Frame
content, and add UID checks only in already owned test/helper files.

The fresh Reviewer found no Critical or Important issue and accepted:

- private attempt Evidence retains only exact Bridge-validated,
  Grant-authorized canonical Frames;
- observer failure may retain Ack plus the rejected authorized Event and no
  later Frame;
- the existing capture is bounded, attempt-bound, Grant-sanitized, and
  digest-finalized;
- owned component tests can decode and compare artifact Frames without
  changing Evidence, TeamExecution, Supervisor, Grant, Journal, or Projection;
  and
- the resolved-file UID check is bounded to owned test/helper files and
  preserves the Windows compile-only gate.

No file was edited and no Pi, model, or live canary ran during review.

Verdict: `PASS`

Status-only finalization changed only this ledger and the unique contract's
status/terminal verdict. The bounded test-first repair is authorized; commit
and live execution remain locked.

VERDICT: PASS
## Reopen 1 RED discovery

The first locked-Pi RED invalidated the reviewed `maxTokens: 1024` proposal
before any product modification:

```text
observed request output budget = 1
phase=assistant_update event=text_start reason=terminal_stop_reason
```

Locked `simple-options.js` proves Pi reserves `4096` context tokens and clamps
the effective output budget to at least `1`. With the current declared
`contextWindow: 4096`, that reserve guarantees the observed one-token request.

An independent read-only parse of the exact installed GGUF v3 metadata proved
`qwen2.context_length=32768`. The in-place contract now proposes only:

```text
contextWindow: 4096 → 32768
maxTokens: unchanged at 256
```

This materially replaces the previously reviewed proposal. Contract Review 1
remains historical evidence for the earlier hypothesis but does not authorize
the corrected GREEN. Fresh Contract Review 2 is required before product
implementation.

This was a deterministic component RED. It is distinct from the first
controlled live canary, which failed at:

```text
phase=assistant_update event=text_end reason=terminal_stop_reason
```

VERDICT: PENDING

## Reopen 1 Contract Review 1 — historical, superseded

Date: `2026-07-28`

The fresh read-only Reviewer inspected only the in-place Reopen 1 diff against
baseline:

```text
143163c4f61481cd9359e44ee52c5347923ff9f4
```

It edited, staged, and committed nothing and ran no test, Pi, llama.cpp,
model, or live canary.

Critical findings: none.

Important findings: none.

The Reviewer confirmed:

- Reopen 1 remains inside the existing unique
  `PHASE1-FINAL-LIVE-PI-0821-TRANSCRIPT-CLOSURE-1` lineage and is not a new
  point Amendment;
- the consumed first canary remains historical and immutable;
- exactly one new isolated canary is gated behind Implementation Reviewer
  `PASS`, with no unchanged rerun and the three-repair ceiling retained;
- owned files, live-only additions, dependencies, acceptance, checks, trust
  boundary, quarantine, and authorization accounting are explicit;
- only terminal `stop` remains successful while `length`, `toolUse`, errors,
  missing, empty, and unknown reasons remain rejected;
- current product code still declares `MaxTokens: 256`, giving RED an exact
  pre-change target;
- locked Pi source sends `options.maxTokens` as the provider output-token
  field and maps `stop/end`, `length`, tool, content/network, and unknown
  finish reasons as stated by the contract; and
- `256 → 1024` is bounded below the declared `4096` context window and within
  the existing `16384` assistant-byte cap, changing execution budget without
  changing parser or authority.

No Critical or Important issue blocked the proposal as it then stood.
This `PASS` became historical when the first genuine RED disproved the
`maxTokens: 1024` hypothesis. It does not authorize the corrected contract.

VERDICT: PASS

## Reopen 1 Contract Review 2

Date: `2026-07-28`

The fresh read-only Reviewer edited, staged, and committed nothing and ran no
test, Pi, llama.cpp, model, or live canary.

Verdict: `FAIL`

Important findings:

1. changing only Pi's declared `contextWindow` to `32768` would leave the
   controlled local llama-server at `--ctx-size 4096`, creating an
   advertised/executed capability mismatch;
2. the root-cause proof depended on locked `simple-options.js`, but the
   mandatory component and pre-live bindings covered only `event-stream.js`
   and `openai-completions.js`; and
3. the prior Review-1 ledger still described the disproved `256 → 1024`
   proposal and therefore could not authorize the corrected contract.

The unique contract was repaired in place without a new Amendment, WorkItem,
Candidate, live authorization, parser exception, dependency, or authority:

- one shared context constant must align Pi's declaration and
  llama-server's `--ctx-size` at the exact GGUF `32768`;
- both output surfaces remain exactly `256`;
- `simple-options.js` joins the exact component and pre-live file/digest/mode/
  current-user ownership lock;
- `local_model_server.go` and its test are added to exact ownership; and
- Review 1 remains immutable historical evidence, explicitly superseded.

Fresh Contract Review 3 is required before RED. Product code remains
unchanged and live execution remains locked.

VERDICT: PENDING

## Reopen 1 Contract Review 3

Date: `2026-07-28`

The fresh independent read-only Reviewer inspected the repaired contract,
review ledger, and only the relevant current source facts. It edited, staged,
and committed nothing and ran no test, Pi, llama.cpp, model, or live canary.

Critical findings: none.

Important findings: none.

The Reviewer confirmed:

- one shared context constant must align Pi `modelsJSON` and llama-server
  `--ctx-size` at `32768`, while both output surfaces remain `256`;
- `local_model_server.go` and its test are explicitly owned;
- all three causal Pi source files, including exact `simple-options.js`, are
  mandatory component and pre-live bindings without changing quarantined
  `source-lock.json`;
- Review 1 is historical and superseded;
- the live `text_end` failure and deterministic component-RED `text_start`
  failure are explicitly distinct;
- exact ownership, TDD sequence, verification, trust/authority,
  no-parser-widening, one-live accounting, no unchanged rerun, and the
  three-repair ceiling are complete; and
- current `4096` Pi declaration/server argument and absent
  `simple-options.js` executable-test binding are expected pre-RED state.

Status-only finalization changed only the unique contract status and terminal
verdict. RED is authorized. Product implementation and live execution remain
locked behind their respective gates.

VERDICT: PASS

## Reopen 1 Repair 2 Contract Review

Date: `2026-07-28`

Baseline:

```text
67b251cae0e3a2086163998b309b4ebb5beadca5
```

The same unique Closure Contract proposes a harness-only authoritative-clock
binding repair after reviewed Reopen-1 result evidence.

Review must verify:

- no new point Amendment or WorkItem;
- Repair count `2 of 3`;
- exact harness-only ownership;
- preservation of `CommitTeamNodeAcceptance` and its exact-time authority
  predicate;
- one captured UTC snapshot shared by every authoritative time consumer;
- real context/deadline/process cleanup remains wall-clock-driven;
- genuine test-first RED and deterministic locked-Pi component closure;
- new manifest/attempt isolation from the consumed Reopen-1 invocation;
- complete checks, quarantine, and no-parser/no-authority expansion; and
- one new invocation only after Implementation Reviewer `PASS`, with no
  unchanged rerun.

Product/test implementation and live execution remain locked.

VERDICT: PENDING

### Repair 2 Contract Review Attempt 1

Verdict: `FAIL`

The fresh Reviewer found one Important lineage defect: the frozen
`67b251cd...` baseline/result SHA was not a Git object. The exact committed
result-evidence baseline is:

```text
67b251cae0e3a2086163998b309b4ebb5beadca5
```

The Reviewer also conditionally noted the repository's five pre-existing
modified quarantine files. They are not Candidate scope; the contract now
repeats their exact frozen hashes and requires them to remain unstaged and
uncommitted.

No test, Pi, llama.cpp, model, network, or live canary ran. Product/test
implementation remains unchanged and locked.

Fresh Repair-2 Contract Review 2 is required before RED.

VERDICT: PENDING

### Repair 2 Contract Review 2

Verdict: `PASS`

Critical findings: none.

Important findings: none.

The fresh independent Reviewer verified:

- exact baseline/result evidence Git commit
  `67b251cae0e3a2086163998b309b4ebb5beadca5`;
- same unique Closure Contract and Repair count `2 of 3`;
- harness-only ownership with product/parser/authority read-only;
- all five dirty-worktree quarantine hashes and empty staging;
- the unchanged exact-time predicate in `CommitTeamNodeAcceptance`;
- the current pre-RED dynamic-clock mismatch and consumed Reopen-1
  manifest/prefix;
- test-first fixed authoritative fact time while context/deadline/process
  cleanup remains wall-clock;
- complete component/full/race/vet/format/Windows checks; and
- fresh Implementation Review before exactly one isolated Repair-2 canary,
  with no unchanged rerun.

No test, Pi, llama.cpp, model, network, staging, commit, or file edit occurred
during review.

Status-only finalization changed only the unique contract status and terminal
verdict. RED is authorized. Implementation and live remain locked.

VERDICT: PASS
