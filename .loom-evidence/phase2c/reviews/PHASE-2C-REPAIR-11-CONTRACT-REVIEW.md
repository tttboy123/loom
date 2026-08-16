# Phase 2C Repair 11 Contract Review

**Review type**: independent read-only contract and boundary review  
**Verdict**: FAIL  
**Counts**: P0=0, P1=1, P2=1

## Findings

### P1 - active boundary status is internally stale

The Repair Amendment header still named Repair 8, while the Candidate Boundary
status named Repair 10 and its Purpose described the Candidate only through
Repair 10. Those headers conflicted with Section 18 and `docs/CURRENT.md`, which
correctly made Repair 11 active. The detailed Repair 11 body was substantively
sound, but the exact bytes did not unambiguously authorize Repair 11 as the next
contract-reviewed boundary.

**Required correction**: update both active status headers and the Candidate
Purpose before any RED or source-lock action.

### P2 - Attempt 009 Runtime proof is imprecisely summarized

Section 18 said Attempt 009 proved `authoritative Runtime offline-to-online
recovery`. The exact proof was a controlled authoritative `online -> offline`
fact followed by ordinary-observer `offline -> online` recovery. J7 still failed
as one coherent client journey because the Attention page remained stale.

**Required correction**: state both directions and preserve the failed J7
adjudication in the opening summary.

## Positive Adjudication

The substantive Repair 11 scope causally covers all Attempt 009 findings:

- Attention refreshes both rendered sources.
- Recent labels reuse safe `action_required` context.
- `LOOM_JOURNEY_ID` is explicit and subfixtures are documented.
- J9 requires live native AX/keyboard traversal without Computer Use.
- J10 is recaptured after the new source lock.
- Product source changes remain limited to the four declared TUI/Swift paths.

The frozen Exit Contract and Journey Manifest are compatible with the stricter
evidence path. A re-review of the corrected exact bytes is required.

