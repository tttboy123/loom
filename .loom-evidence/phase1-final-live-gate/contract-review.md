# Final Live Gate Compatibility Amendment — Contract Review

## Review 1

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Reviewer
- Candidate baseline: `f51bfd4`
- External materialization performed: no
- Live canary performed: no

Blocking findings:

1. The contract self-declared `VERDICT: PASS` before independent review.
2. The Pi RPC allowlist omitted documented valid assistant lifecycle records
   `start` and `done`, including exact `done.reason` handling.
3. External materialization timing conflicted between Contract Review,
   Implementation Review, and commit gates.
4. Exact llama.cpp flags lacked version-pinned compatibility evidence and the
   intended single-model versus Pi router choice was not explicit.

Repair disposition:

- removed the self-authored PASS;
- added exact `start -> text_start -> text_delta+ -> text_end -> done(stop)`
  ordering and fail-closed completion reasons;
- made Implementation Review PASS plus the local compatibility commit the
  single materialization boundary; and
- locked the official llama.cpp `b10107` server reference digest, documented
  every exact flag, and explicitly selected static single-model
  OpenAI-compatible mode rather than Pi's dynamic `/llama` router.

VERDICT: FAIL

## Review 2

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Reviewer 2
- Candidate baseline: `f51bfd4` plus bounded governance Repair 1
- External materialization performed: no
- Live canary performed: no

Review 1 closure:

1. The contract no longer self-authors PASS; verdicts live in this review
   artifact.
2. The exact valid Pi 0.82.1 assistant lifecycle includes `start`, one bounded
   text block, and `done(reason=stop)` with fail-closed alternatives.
3. External materialization has one gate after RED/GREEN, the full matrix,
   Implementation Review PASS, and the local compatibility commit.
4. The pinned llama.cpp `b10107` server reference digest proves the frozen
   single-model flags and the static Pi `models.json` choice is explicit.

No blocking findings.

Residual non-blocking risks:

- llama.cpp/model files remain subject to later materialization-time exact
  digest verification;
- the RPC decoder is deliberately Pi-0.82.1-specific and requires a new
  amendment for upgrades; and
- ADR-0010 acceptance changes only its status/index and does not broaden scope.

VERDICT: PASS

## Technical Correction 2 — pending Review 3

- Date: `2026-07-27`
- Trigger: Implementation Review 2 `FAIL` plus required fresh problem analysis
- External materialization performed: no
- Live canary performed: no

Correction 2 supersedes only two technically false/underspecified clauses from
Review 2:

1. Locked `pi-agent-core` maps Provider start/done to top-level assistant
   message start/end; they are not nested RPC update events.
2. A local-model security root must be explicit and every component/leaf must
   be containment-, owner-, mode-, symlink-, identity-, and digest-validated;
   two leaf paths cannot safely infer that boundary.

The scope, owned files, one-writer/one-authority boundary, source versions,
offline model, process lifetime, external gate order, and live exclusions do
not expand. Product Repair 2 remains closed until a fresh independent Contract
Review 3 returns `PASS`.

VERDICT: FAIL

## Review 3

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 3
- External materialization performed: no
- Live canary performed: no

Correction 2's event mapping, explicit private root, shared binding inspector,
containment/revalidation requirements, residual-race disclosure, scope, and
external gates were technically coherent. Review 3 nevertheless found the ADR
had remained prematurely `accepted` and did not yet record the corrected
private-root decision.

Governance Repair 3 returned ADR-0010 and its index row to `proposed`, added the
locked Agent-loop mapping, explicit-root/shared-inspector boundary, full-chain
validation, and disclosed pathname-reopen residual. Product Repair 2 remains
closed pending fresh Review 4.

VERDICT: FAIL

## Review 4

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 4
- External materialization performed: no
- Live canary performed: no

Review 3's ADR blockers were closed. Review 4 found one remaining
contract/test-coherence ambiguity: the metadata contract said exact bounded
three-line diagnostic while the parser's non-empty-line normalization accepted
inserted blank lines.

Governance Repair 4 now freezes exactly three physical LF-delimited lines with
at most one final LF, no boundary whitespace, and no leading, interstitial, or
appended blank/whitespace-only line. Product Repair 2 must add regression tests
and make the Pi 0.82.1 diagnostic branch enforce this shape without weakening
the existing table grammar.

VERDICT: FAIL

## Review 5

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 5
- External materialization performed: no
- Live canary performed: no

Review 5 repeated the current parser's known acceptance of blank/boundary
whitespace as a blocking finding. That product nonconformance is real and is
the exact regression Repair 2 must expose and fix, but Repair 2 was still closed
because no corrected-contract PASS existed.

The contract is clarified for Review 6: contract review evaluates the frozen
Correction 2 for precision, testability, authorization, and internal
coherence. It must not require the known pre-Repair-2 product RED to already be
GREEN. Product Repair 2 remains closed pending Review 6.

VERDICT: FAIL

## Review 6

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 6
- External materialization performed: no
- Live canary performed: no

Review 6 was asked to conclude before it had inspected the governance diff and
therefore correctly refused to infer a PASS without evidence. It identified no
new contract defect, but its evidence was insufficient to open Repair 2.

Fresh Review 7 must inspect the corrected files and return an evidence-backed
verdict. Product Repair 2 remains closed.

VERDICT: FAIL

## Review 7

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 7
- External materialization performed: no
- Live canary performed: no

Blocking findings:

1. The exact private-file modes were internally ambiguous: the contract used
   the phrase "frozen modes" for leaves while the final gate summarized all
   private directories/files as `0700/0600`, which did not unambiguously bind
   the executable to mode 0700 and non-executable files to mode 0600.
2. `source-lock.json` still declared
   `implementation_verified_not_materialized`, overstating the current state
   after Technical Correction 2 reopened bounded Product Repair 2.

Governance Repair 7 freezes exact mode 0700 for every private directory and the
llama.cpp executable, exact mode 0600 for the model, resolved manifest, and
other non-executable private files, and marks the source lock as Technical
Correction 2 frozen with Repair 2 pending and no materialization.

Positive evidence: ADR-0010 remains proposed; the corrected Pi Agent-loop event
mapping, explicit private-root/shared-inspector boundary, external gate order,
and locked source hashes are coherent.

VERDICT: FAIL

## Review 8

- Date: `2026-07-27`
- Reviewer: fresh independent read-only Contract Reviewer 8
- External materialization performed: no
- Live canary performed: no

No blocking findings.

Evidence checked:

1. The contract keeps external materialization, the compatibility commit, and
   live canary prohibited while opening only bounded Product Repair 2.
2. The exact Pi 0.82.1 metadata diagnostic is three physical LF-delimited
   lines, with no blank or boundary whitespace and validation-only paths.
3. The installed locked `agent-loop.js` confirms Provider start/done map to
   top-level assistant `message_start`/`message_end`, with text content alone
   carried by `message_update`.
4. The source lock no longer overstates implementation or materialization and
   the installed Pi CLI, Agent-loop, RPC-mode, and RPC-types digests match.
5. The explicit PrivateRoot/shared-inspector/full-chain boundary and exact
   directory, executable, and non-executable modes are coherent and testable.
6. The external sequence remains RED, implementation, full matrix,
   Implementation Review PASS, local compatibility commit, materialization,
   digest verification, then one opt-in live canary.
7. ADR-0010 remained proposed throughout review and matched the corrected
   contract.

Known pre-Repair-2 product RED, including the old nested `start`/`done` decoder
and missing explicit `PrivateRoot`, was correctly treated as bounded repair
work rather than a contract defect.

VERDICT: PASS
