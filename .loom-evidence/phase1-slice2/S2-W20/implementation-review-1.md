# S2-W20 Implementation Review 1

- Reviewer: fresh independent read-only implementation Reviewer
- Contract SHA-256:
  `d403da481f25754d144f3721e696d21ba67479626b2a91c1551f6f87a199e0b6`
- Product SHA-256:
  `46d3166c9cf3182693e08009cc4929b066b71ddf6cb18a7d7a06a0887c6ce6c4`
- Test SHA-256:
  `7e9bd9874cb4b74ba70de317fd05fd974995e51d00914ed4b849228b5b901f91`

## Finding

1. High: exact appender-result validation compared `EmittedAt` with
   `time.Time.Equal` but did not require both timestamp locations to be UTC.
   A custom appender could therefore return the same instant with a non-UTC
   `Location`, and the writer would accept that non-canonical Event into the
   commit Candidate. The accepted S2-W14 writer rejects this mismatch.
2. Tests covered length, order, sequence, and payload mutations but did not
   prove rejection of this timestamp-location mutation.

The Reviewer found no other blocking product, test, scope, state-authority, or
security issue. Independent focused, package, focused-race-30, repository,
repository-race, vet, format, and diff checks passed.

VERDICT: FAIL
