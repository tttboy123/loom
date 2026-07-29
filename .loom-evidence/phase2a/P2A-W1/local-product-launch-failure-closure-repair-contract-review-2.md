# P2A-W1 Local Product Launch Failure Closure Contract Re-review 2

**Date**: 2026-07-29  
**Review type**: fresh independent, read-only  
**Contract reviewed SHA-256**:
`6e2061f32b6e31cdfc086195468f18b9c9470a9e7a060f32c3c3e2609cdc46b5`  
**Verdict**: `PASS`

The first independent pass reviewed SHA-256
`7bc6d9caf6276cee6d42019446ab3df5eeff68eb7981153f253c4c3861889d15`.
A follow-up read-only hash-drift closure check reproduced that digest by
reversing only the contract Status line. The current digest above differs only
because the Status now records this PASS; no semantic contract content changed.

## Findings

- P0: none
- P1: none
- P2: none

## Review 1 repairs reproduced

- The contract owns one exact fresh repaired source-lock path and one exact
  fresh repaired Candidate-manifest path.
- The retained failed Candidate is limited to the exact-path preflight and is
  explicitly ineligible for replacement live activation.
- The failure-reason record has one exact owned path and a fixed closed schema.
- The reason record prohibits paths, process identifiers, timestamps,
  environment names or values, raw stderr, and free text; it requires atomic
  no-replace creation, symlink refusal, uid `501`, and mode `0600`.

## Key gates reproduced

- The scope is one vertical repair inside existing P2A-W1; no new WorkItem is
  created.
- Owned files are sufficient and bounded. `internal/localipc` remains
  read-only unless a causal RED proves otherwise.
- Every section 2 source-input hash matches the current file.
- Mandatory RED covers closed daemon classification, product-daemon lifecycle
  classification, transaction ordering, exact-path preflight, reason-record
  creation, and one-bootstrap/no-retry accounting.
- Exact-path preflight is direct-run, private-state, bounded, and cannot
  replace or restart the resident observer or grant live allowance.
- The switch transaction requires original service absence before install,
  one bootstrap, no alternate socket, no retry, and immediate rollback on a
  pre-ready failure.
- The identity chain is acyclic: source lock is upstream; Candidate manifest
  binds the source lock; transaction binds source lock, manifest, and
  artifacts; activation audit binds source lock, manifest, transaction,
  fixture, and artifacts.
- Implementation Review PASS alone grants no production launch. A separate
  post-Review activation audit is required.
- Any replacement-canary failure consumes its allowance and stops
  `HUMAN_REQUIRED`.
- P2A-W2 remains locked until every P2A-W1 exit gate passes.

No file, installed product, App, service, Journal, Provider, Runtime, or live
state was mutated by the Reviewer.

## Contract Re-review 3: bounded preflight-harness reopen

**Contract reviewed SHA-256**:
`f3640d716ee9d36fa77faedf6a4b597843bf564392a97bf4dc8bd27937ddb1d2`  
**Preflight evidence SHA-256**:
`fae2bede1563d04a8198c0c5dbc5e3ed30ea431410d88c8dd14dc0549fd605fc`  
**Verdict**: `PASS`

Findings were P0 none, P1 none, and P2 none.

The fresh independent read-only Reviewer confirmed:

- the first exact-path execution is honestly classified as
  `INVALID EVIDENCE HARNESS - NO PRODUCT VERDICT`;
- `onlineStatusOutput` flattens the embedded `LocalProductSnapshot`, proving
  the nested-object predicate was invalid;
- post-cleanup resident PID/state/runs, absent run root and processes, exact
  Journal hash/integrity/Event count, and empty staging are recorded;
- exactly one replacement preflight is permitted;
- the replacement is not a production bootstrap, live canary, retry of the
  consumed transaction, or live allowance;
- the corrected predicate is limited to the closed top-level status fields;
- bounded outputs remain available until exit codes, byte counts, process
  count, and typed-status validity are recorded;
- all prior scope, no-retry, allowance `0`, P2A-W1, and P2A-W2 locks remain.

A final read-only hash-drift closure verified current contract SHA-256
`b04bde826380088f83caab5fcf4ed4f3d9a73de9cd796c0cbbc30b0a1575998b`.
Reversing only the Status line reproduces the Re-review 3 hash above; the PASS
covers the current exact contract.

## Contract Re-review 4: resident continuity rebaseline

**Contract reviewed SHA-256**:
`386877ed0ddf564918e3158949ba49ce8f46715d9a904ca07fa72663a58c3a49`  
**Verdict**: `PASS`

Findings were P0 none, P1 none, and P2 none.

The fresh independent read-only Reviewer confirmed:

- every historical PID/run observation remains immutable and disclosed;
- PID/run count is classified as process observation, not state authority;
- immutable continuity remains exact across installed hashes/modes,
  label/program, Journal, crash inventory, absence predicates, and staging;
- the pre-activation audit requires three stable samples spanning at least
  sixteen seconds after resource-heavy verification ends;
- exactly one `Resident PID` and one `Resident Runs` field must be frozen;
- the transaction must strictly parse and match both fields immediately before
  bootout, otherwise fail with zero bootstrap and zero mutation;
- rollback correctly restores the logical resident service and immutable state
  without claiming to restore an historical OS PID or launchd run count;
- no live allowance or new WorkItem is created, and P2A-W2 remains locked.

No file, service, Candidate, App, Journal, Provider, Runtime, credential,
staging, or live state was mutated by the Reviewer.

A final read-only hash-drift closure verified current contract SHA-256
`047f4bc691bc7304f0ba2e8082081ba1a91a176bae4aa4519ce4eff01b618fe3`.
Reverting only the Status line reproduces the reviewed Re-review 4 hash; no
semantic contract content changed.

## Contract Re-review 5: complete pre-READY attribution

**Contract reviewed SHA-256**:
`c4a73065183ec056e9199f70985379995b9366af716f7bf5a06d52585eea7149`  
**Verdict**: `PASS`

Findings were P0 none, P1 none, and P2 none.

The fresh independent read-only Reviewer confirmed:

- the consumed `FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`
  result and allowance `0` remain immutable;
- the missing predicate is not inferred;
- every consumed pre-READY boundary is covered by one closed phase enum;
- reason-writer failure cannot hide the primary phase;
- an uncovered consumed exit fails closed as `unclassified`;
- the ten-key JSON and closed stdout line prohibit raw or sensitive data;
- fixtures must cover every phase, invalid reason targets, no retry, the
  catch-all trap, and successful READY;
- original-service settling is bounded at sixty seconds and deterministically
  testable with an injected fast sequence;
- no new WorkItem, live authority, retry, Provider/Runtime action, commit, or
  P2A-W2 unlock is granted.

No file or live state was mutated by the Reviewer.

A final read-only hash-drift closure verified current contract SHA-256
`4713e56acd1a463e0081b57bade31118a8c1b68926dc5b5d7640aedfdefc2f5b`.
Reverting only the Status line reproduces the reviewed Re-review 5 hash; no
semantic contract content changed.
