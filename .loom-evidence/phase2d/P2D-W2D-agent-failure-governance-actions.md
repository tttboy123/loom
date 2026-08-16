# P2D-W2D Agent Failure Governance Actions

Date: 2026-08-12

Status: `CURRENT / SOURCE VERIFIED / BUILD 46 PACKAGED / NOT INSTALLED`

## User-visible result

The Team Pulse inspector now gives an affected Agent three bounded actions at
the point of failure:

- review a retry in the existing Attention governance surface;
- preview privacy-safe operational diagnostics;
- copy the correlated Incident ID.

A healthy peer Agent and Provider Account accounting rows do not inherit those
actions. Loom therefore presents a Provider or credential failure as an
Agent-local condition rather than a Team-wide offline state.

## Governance boundary

- `Review retry` navigates to Attention. It does not dispatch, mutate an
  Attempt, approve a fallback, or create execution authority.
- `View Agent diagnostics` uses the existing allowlisted diagnostic preview.
  Operational diagnostics remain separate from the Event Journal and cannot
  authorize execution.
- Actions are projected only when the exact Agent row has a current safe
  diagnostic. Retry review additionally requires that diagnostic to be marked
  retryable.
- Incident copy remains scoped to the exact projected Agent row.
- Missing arrays, old wire data, and out-of-range rows fail closed with no
  action.
- No credential reference, endpoint fingerprint, secret, Prompt, transcript,
  Provider response, ciphertext, nonce, wrapped key, or hidden reasoning is
  added to the UI contract.

## Verification

The focused fixture contains one retryable Anthropic Agent failure, one
successful OpenAI/Codex peer, and one Provider Account accounting row. It proves
that only the failed Agent receives diagnostic and retry-review actions while
the peer remains visible as `Succeeded` and action-free.

The following gates pass on branch `codex/loom-platform-slice2` from baseline
HEAD `651f156afda37a8e703cbc0396f9f38b7912600b`:

- focused Team inspector projection and isolation test;
- visible-action source contract, including non-inert action checks;
- complete Swift suite: 181 XCTest cases, one intentional visual-preview skip,
  and eight Swift Testing contracts, with zero failures;
- native rendering coverage for wide and compact windows, light and dark
  appearance, and large Dynamic Type;
- `git diff --check`.

## Boundary

The frozen, uninstalled v0.5.2 build 45 Candidate predates this increment and
does not contain these Agent-row actions. They are packaged in the unlaunched,
uninstalled v0.5.2 build 46 Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build46-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, action-string,
wire-contract, and ZIP byte-equivalence gates pass.

Installed Loom remains v0.5.2 build 39. Installed mixed-Team failure isolation,
real Provider Account accounting, approved fallback execution, and the
counter-backed no-Keychain live matrix remain open. Phase 2D stays
`ACTIVE / PARTIAL`.
