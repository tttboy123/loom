# P2A-W2 Live Gate Pi Node Search Path Correction Review

**Date**: 2026-07-29
**Status**: PASS
**Reviewer**: independent read-only Reviewer
`p2a_w1_implementation_review3`

## Verdict

`PASS`

- P0 findings: none.
- P1 findings: none.
- P2 findings: none.

## Material checks

The Reviewer confirmed that:

- the failed startup is zero-event, pre-credential, pre-Keychain, pre-network
  and pre-product-journey, so it does not consume the sole live allowance;
- the ordered correction is bounded to exactly two directories, with the
  reviewed Pi `.bin` directory first and the locked Node `bin` directory
  second;
- the Node identity, SHA-256, type, mode, owner, device and inode stop rules
  prevent ambient interpreter substitution;
- the correction preserves the Codex native-binary correction, credential
  boundary, one-request limit, no-hidden-retry rule, TeamDefinition-only
  authority, resident observer, W3 lock and no-W4 boundary;
- exactly one corrected startup is allowed, and another startup is prohibited
  if that correction fails or an Event exists before SecureField hand-off.

## Independence statement

The Reviewer read exactly the correction and its three named parent/review
files. It:

- modified, staged and committed no file;
- ran no test, Git, process, SQLite, binary or live verification;
- inspected no Keychain data;
- used no network or GUI;
- started or stopped no process;
- performed no allowance-consuming action.

The corrected two-directory Runtime search path may now enter its exact
preflight and one corrected startup. This Review does not itself consume the
canary or authorize an identity mismatch.

## Event-boundary wording re-review

Before startup, the Controller found that the original reviewed stop sentence
could incorrectly reject the Runtime discovery/status Events that a successful
observer must append before the UI can display Pi. The correction was repaired
to require:

- exactly zero Events immediately before corrected startup;
- only expected isolated Runtime discovery/status observation Events before
  SecureField hand-off;
- rejection of every credential, team, execution, dispatch, generation,
  Evidence or other unexpected Event.

The same independent Reviewer re-read only the repaired correction and returned
`PASS` with no P0, P1 or P2 findings. It edited or staged no file, ran no test,
Git, process, SQLite, binary, network, Keychain, GUI or live action, and
confirmed that only one corrected startup remains permitted.
